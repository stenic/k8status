package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"path/filepath"
	"time"

	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
	klog "k8s.io/klog/v2"

	//"k8s.io/client-go/pkg/api/v1"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var kubeconfig string
var namespace string
var prefix string
var svcReps []SvcRep
var interval int
var showDegraded bool
var mode string

var nsNameCache = map[string]string{}
var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "k8status_health_checks_total",
		Help: "The total number of health checks performed",
	})
	servicesReady = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "k8status_services_ready",
		Help: "Current number of service ready",
	})
	servicesNotReady = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "k8status_services_not_ready",
		Help: "Current number of service not ready",
	})
)

func init() {
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.StringVar(&namespace, "namespace", "", "namespace")
	flag.StringVar(&prefix, "prefix", "/", "path prefix")
	flag.StringVar(&mode, "mode", "inclusive", "mode: inclusive or exclusive")
	flag.IntVar(&interval, "interval", 5, "readiness poll interval")
	flag.BoolVar(&showDegraded, "show-degraded", false, "indicate degraded service")
}

func main() {
	flag.Parse()
	logs.InitLogs()
	defer logs.FlushLogs()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		klog.Infof("Exposing metrics on :2112/metrics")
		http.ListenAndServe(":2112", nil)
	}()

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	klog.Infof("Loading services from %s", namespace)
	namespaces := []string{}
	if namespace != "" {
		namespaces = append(namespaces, namespace)
	}
	loader := func() {
		if namespace == "" {
			namespaces = loadNamespaces(clientset)
		}
		// klog.Info("Reloading services")
		_svcReps := []SvcRep{}
		for _, ns := range namespaces {
			_svcReps = append(_svcReps, loadServiceInfo(clientset, ns)...)
		}
		svcReps = _svcReps
	}
	loader()

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				loader()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	klog.Infof("Starting webserver at %s", prefix)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(
		gin.LoggerWithWriter(gin.DefaultWriter, "/healthz"),
		gin.Recovery(),
	)
	r.Use(static.Serve(prefix, static.LocalFile("./static", true)))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, "ok")
	})
	r.GET(prefix+"status", func(c *gin.Context) {
		c.JSON(getSvcStatus(svcReps), svcReps)
	})
	r.GET(prefix+"status/:name", func(c *gin.Context) {
		for _, svc := range svcReps {
			if svc.Name == c.Param("name") {
				svcs := []SvcRep{svc}
				c.JSON(getSvcStatus(svcs), svc)
				return
			}
		}
		c.JSON(404, gin.H{"msg": "Not found"})
	})

	if err := r.Run(); err != nil {
		klog.Fatal(err)
	}
}

func loadNamespaces(clientset *kubernetes.Clientset) []string {
	ctx := context.Background()
	var result []string
	ns, err := clientset.CoreV1().Namespaces().List(ctx, v1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, n := range ns.Items {
		if val, ok := n.Annotations["k8status.stenic.io/include"]; ok && val == "true" {
			result = append(result, n.Name)
		}
		if val, ok := n.Annotations["k8status.stenic.io/name"]; ok {
			nsNameCache[namespace] = val
		}
	}
	return result
}

func loadServiceInfo(clientset *kubernetes.Clientset, ns string) []SvcRep {
	ctx := context.Background()
	var readyCnt, notReadyCnt int = 0, 0
	var result []SvcRep
	svcs, err := clientset.CoreV1().Services(ns).List(context.Background(), v1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, svc := range svcs.Items {
		if val, ok := svc.Annotations["k8status.stenic.io/exclude"]; ok && val == "true" {
			continue
		}
		if mode == "exclusive" {
			if val, ok := svc.Annotations["k8status.stenic.io/include"]; ok && val != "true" {
				continue
			}
		}
		pods, err := clientset.CoreV1().Pods(svc.Namespace).List(ctx, v1.ListOptions{
			LabelSelector: labels.SelectorFromSet(svc.Spec.Selector).String(),
		})
		if err != nil {
			log.Fatal(err)
		}
		healthy := 0
		for _, pod := range pods.Items {
			for _, container := range pod.Status.ContainerStatuses {
				if container.Ready {
					healthy++
				}
			}
		}
		name := svc.GetObjectMeta().GetName()
		if val, ok := svc.Annotations["k8status.stenic.io/name"]; ok {
			name = val
		}
		description := ""
		if val, ok := svc.Annotations["k8status.stenic.io/description"]; ok {
			description = val
		}
		var status string
		switch healthy {
		case 0:
			status = "down"
		case len(pods.Items):
			status = "ok"
		default:
			if showDegraded {
				status = "degraded"
			} else {
				status = "ok"
			}
		}
		result = append(result, SvcRep{
			Name:        name,
			Namespace:   getNsName(svc.Namespace),
			Description: description,
			Ready:       healthy > 0,
			Status:      status,
		})

		if healthy > 0 {
			readyCnt++
		} else {
			notReadyCnt++
		}
	}

	opsProcessed.Inc()
	servicesReady.Set(float64(readyCnt))
	servicesNotReady.Set(float64(notReadyCnt))

	return result
}

func getNsName(namespace string) string {
	if val, ok := nsNameCache[namespace]; ok {
		return val
	}

	return namespace
}

func getSvcStatus(svcs []SvcRep) int {
	for _, svc := range svcs {
		if !svc.Ready {
			return 503
		}
	}

	return 200
}

type SvcRep struct {
	Name        string         `json:"name"`
	Namespace   string         `json:"namespace"`
	Description string         `json:"description"`
	Ready       bool           `json:"ready"`
	Raw         v1core.Service `json:"-"`
	Status      string         `json:"status"`
}
