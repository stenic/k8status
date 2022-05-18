package main

import (
	"context"
	"flag"
	"log"
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
)

var kubeconfig string
var namespace string
var prefix string
var svcReps []SvcRep
var interval int

func init() {
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.StringVar(&namespace, "namespace", "", "namespace")
	flag.StringVar(&prefix, "prefix", "/", "path prefix")
	flag.IntVar(&interval, "interval", 5, "readiness poll interval")
}

func main() {
	flag.Parse()
	logs.InitLogs()
	defer logs.FlushLogs()

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
	r.Run()
}

func loadServiceInfo(clientset *kubernetes.Clientset, ns string) []SvcRep {
	var result []SvcRep
	svcs, err := clientset.CoreV1().Services(ns).List(context.Background(), v1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, svc := range svcs.Items {
		if val, ok := svc.Annotations["k8status.stenic.io/exclude"]; ok && val == "true" {
			continue
		}
		pods, err := clientset.CoreV1().Pods(ns).List(context.Background(), v1.ListOptions{
			LabelSelector: labels.SelectorFromSet(svc.Spec.Selector).String(),
		})
		if err != nil {
			log.Fatal(err)
		}
		ready := false
		for _, pod := range pods.Items {
			for _, container := range pod.Status.ContainerStatuses {
				if container.Ready {
					ready = true
				}
			}
		}
		name := svc.GetObjectMeta().GetName()
		if val, ok := svc.Annotations["k8status.stenic.io/name"]; ok {
			name = val
		}
		result = append(result, SvcRep{
			Name:  name,
			Ready: ready,
		})
	}

	return result
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
	Name  string         `json:"name"`
	Ready bool           `json:"ready"`
	Raw   v1core.Service `json:"-"`
}
