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
	svcReps = loadServiceInfo(clientset, namespace)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				// klog.Info("Reloading services")
				svcReps = loadServiceInfo(clientset, namespace)
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

func loadServiceInfo(clientset *kubernetes.Clientset, ns string) []SvcRep {
	ctx := context.Background()
	var readyCnt, notReadyCnt int = 0, 0
	var result []SvcRep
	svcs, err := clientset.CoreV1().Services(ns).List(context.Background(), v1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, svc := range svcs.Items {
		if !includeNamespace(ctx, clientset, svc.Namespace) {
			continue
		}
		if val, ok := svc.Annotations["k8status.stenic.io/exclude"]; ok && val == "true" {
			continue
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
			Namespace:   getNsName(ctx, clientset, svc.Namespace),
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

var nsNameCache = map[string]string{}

func getNsName(ctx context.Context, clientset *kubernetes.Clientset, namespace string) string {
	if val, ok := nsNameCache[namespace]; ok {
		return val
	}

	ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, v1.GetOptions{})
	if err != nil {
		return namespace
	}

	if val, ok := ns.Annotations["k8status.stenic.io/name"]; ok {
		nsNameCache[namespace] = val
		return val
	}

	nsNameCache[namespace] = namespace
	return namespace
}

var nsExcludeCache = map[string]bool{}

func includeNamespace(ctx context.Context, clientset *kubernetes.Clientset, name string) bool {
	// Single namespace mode is always included
	if namespace != "" {
		return true
	}

	// Check cache
	if val, ok := nsExcludeCache[name]; ok {
		return val
	}

	// Check annotation
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return false
	}

	nsExcludeCache[name] = false
	if val, ok := ns.Annotations["k8status.stenic.io/include"]; ok && val == "true" {
		nsExcludeCache[name] = true
	}

	return nsExcludeCache[name]
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
