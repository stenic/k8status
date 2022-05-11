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
	"k8s.io/client-go/util/homedir"
	klog "k8s.io/klog/v2"

	//"k8s.io/client-go/pkg/api/v1"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
)

var kubeconfig string
var namespace string
var svcReps []SvcRep

func init() {
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.StringVar(&namespace, "namespace", "", "namespace")
	// flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
}

func main() {
	flag.Parse()
	logs.InitLogs()
	defer logs.FlushLogs()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	klog.Info("Loading services")
	svcReps = loadServiceInfo(clientset, namespace)

	ticker := time.NewTicker(15 * time.Second)
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

	klog.Info("Starting webserver")
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/services", func(c *gin.Context) {
		c.JSON(getSvcStatus(svcReps), svcReps)
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
		pods, err := clientset.CoreV1().Pods(ns).List(context.Background(), v1.ListOptions{
			LabelSelector: labels.SelectorFromSet(svc.Spec.Selector).String(),
		})
		if err != nil {
			log.Fatal(err)
		}
		ready := true
		for _, pod := range pods.Items {
			for _, container := range pod.Status.ContainerStatuses {
				if !container.Ready {
					ready = false
				}
			}
		}
		result = append(result, SvcRep{
			Name:  svc.GetObjectMeta().GetName(),
			Ready: ready,
		})
	}

	return result
}

func getSvcStatus(svcs []SvcRep) int {
	for _, svc := range svcs {
		if svc.Ready == false {
			return 503
		}
	}

	return 200
}

type SvcRep struct {
	Name  string         `json:"name"`
	Ready bool           `json:"ready"`
	Pods  []PodRep       `json:"-"`
	Raw   v1core.Service `json:"-"`
}

type PodRep struct {
	Name  string
	Ready bool
	Raw   v1core.Pod `json:"-"`
}
