package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/webhook"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Starts OSP Director Operator webhook clean-up",
		Long:  "",
		Run:   runStartCmd,
	}

	validationWebhookGVR = schema.GroupVersionResource{
		Group:    "admissionregistration.k8s.io",
		Version:  "v1beta1",
		Resource: "validatingwebhookconfigurations",
	}

	mutationWebhookGVR = schema.GroupVersionResource{
		Group:    "admissionregistration.k8s.io",
		Version:  "v1beta1",
		Resource: "mutatingwebhookconfigurations",
	}
)

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStartCmd(cmd *cobra.Command, args []string) {
	var err error
	err = flag.Set("logtostderr", "true")
	if err != nil {
		panic(err.Error())
	}

	flag.Parse()

	glog.V(0).Info("Starting OSP Director Operator webhook clean-up")

	var config *rest.Config
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		panic(err.Error())
	}

	dClient := dynamic.NewForConfigOrDie(config)

	// If we get errors below, there's nothing we can really do other than log them

	validationWebhookClient := dClient.Resource(validationWebhookGVR)

	if err := validationWebhookClient.DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true", webhook.OspDirectorOperatorWebhookLabel),
	}); err != nil {
		glog.V(0).Info(err)
	}

	mutationWebhookClient := dClient.Resource(mutationWebhookGVR)

	if err := mutationWebhookClient.DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true", webhook.OspDirectorOperatorWebhookLabel),
	}); err != nil {
		glog.V(0).Info(err)
	}

	glog.V(0).Info("Shutting down OSP Director Operator webhook clean-up")
}
