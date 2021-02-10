package main

import (
	"context"
	"flag"
	"os"

	"github.com/golang/glog"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	virtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Starts OSP Director Operator finalizer clean-up",
		Long:  "",
		Run:   runStartCmd,
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

	glog.V(0).Info("Starting OSP Director Operator finalizer clean-up")

	var config *rest.Config
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		glog.V(0).Infoln(err.Error())
		os.Exit(1)
	}

	goClient, err := client.New(config, client.Options{})

	if err != nil {
		glog.V(0).Infoln(err.Error())
		os.Exit(1)
	}

	// First remove finalizers from VirtualMachines
	virtualMachineList := &virtv1.VirtualMachineList{}

	listOpts := []client.ListOption{
		// TODO: Get this from elsewhere?
		client.InNamespace("openstack"),
	}

	err = goClient.List(context.TODO(), virtualMachineList, listOpts...)

	if err != nil {
		glog.V(0).Infoln(err.Error())
		os.Exit(1)
	}

	for _, virtualMachine := range virtualMachineList.Items {
		controllerutil.RemoveFinalizer(&virtualMachine, vmset.VirtualMachineFinalizerName)
		err := goClient.Update(context.TODO(), &virtualMachine)

		if err != nil {
			glog.V(0).Infoln(err.Error())
			os.Exit(1)
		}
	}

	// Now remove finalizers from VMSets
	vmSetList := &ospdirectorv1beta1.VMSetList{}

	err = goClient.List(context.TODO(), vmSetList, listOpts...)

	if err != nil {
		glog.V(0).Infoln(err.Error())
		os.Exit(1)
	}

	for _, vmSet := range vmSetList.Items {
		controllerutil.RemoveFinalizer(&vmSet, vmset.FinalizerName)
		err := goClient.Update(context.TODO(), &vmSet)

		if err != nil {
			glog.V(0).Infoln(err.Error())
			os.Exit(1)
		}
	}

	glog.V(0).Info("Shutting down OSP Director Operator finalizer clean-up")
}
