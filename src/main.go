package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vivekhiwarkar/secret-deployer/custom"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join("/home/infracloud", ".kube", "config")
		if envvar := os.Getenv("KUBECONFIG"); len(envvar) > 0 {
			kubeconfig = envvar
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			fmt.Println("Error in clientcmd.BuildConfigFromFlags()", err.Error())
			os.Exit(1)
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("error %s, creating clientset\n", err.Error())
	}

	ch := make(chan struct{})
	informers := informers.NewSharedInformerFactory(clientset, time.Second*30)
	controller := custom.InitController(clientset, informers.Apps().V1().Deployments())

	informers.Start(ch)
	controller.Run(ch)
}
