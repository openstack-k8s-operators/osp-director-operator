package client

import (
	"os"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	BaremetalSetGVR = schema.GroupVersionResource{
		Group:    "osp-director.openstack.org",
		Version:  "v1beta1",
		Resource: "baremetalsets",
	}
)

func GetInClusterClient() (dynamic.Interface, error) {
	var err error
	var config *rest.Config

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		glog.Error("Failed to get k8s client config")
		return nil, err
	}

	return dynamic.NewForConfigOrDie(config), nil
}
