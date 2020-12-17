package main

import (
	"context"
	"flag"
	"net"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/sriov-network-operator/pkg/version"
	"github.com/spf13/cobra"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Starts Machine Config Daemon",
		Long:  "",
		Run:   runStartCmd,
	}

	startOpts struct {
		kubeconfig          string
		provIntf            string
		provServerName      string
		provServerNamespace string
	}

	provisionServerGVR = schema.GroupVersionResource{
		Group:    "osp-director.openstack.org",
		Version:  "v1beta1",
		Resource: "provisionservers",
	}
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.PersistentFlags().StringVar(&startOpts.provIntf, "prov-intf", "", "Provisioning interface name on the associated host")
	startCmd.PersistentFlags().StringVar(&startOpts.provServerName, "prov-server-name", "", "Provisioning server resource name")
	startCmd.PersistentFlags().StringVar(&startOpts.provServerNamespace, "prov-server-namespace", "", "Provisioning server resource namespace")
}

func runStartCmd(cmd *cobra.Command, args []string) {
	flag.Set("logtostderr", "true")
	flag.Parse()

	glog.V(0).Info("Starting ProvisionIpDiscoveryAgent")

	// To help debugging, immediately log version
	glog.V(2).Infof("Version: %+v", version.Version)

	if startOpts.provIntf == "" {
		name, ok := os.LookupEnv("PROV_INTF")
		if !ok || name == "" {
			glog.Fatalf("prov-intf is required")
		}
		startOpts.provIntf = name
	}

	if startOpts.provServerName == "" {
		name, ok := os.LookupEnv("PROV_SERVER_NAME")
		if !ok || name == "" {
			glog.Fatalf("prov-server-name is required")
		}
		startOpts.provServerName = name
	}

	if startOpts.provServerNamespace == "" {
		name, ok := os.LookupEnv("PROV_SERVER_NAMESPACE")
		if !ok || name == "" {
			glog.Fatalf("prov-server-namespace is required")
		}
		startOpts.provServerNamespace = name
	}

	var config *rest.Config
	var err error
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

	provServerClient := dClient.Resource(provisionServerGVR)

	ip := ""

	for {
		unstructured, err := provServerClient.Namespace(startOpts.provServerNamespace).Get(context.Background(), startOpts.provServerName, metav1.GetOptions{})

		if k8s_errors.IsNotFound(err) {
			// Deleted somehow, so just break
			break
		}

		if err != nil {
			panic(err.Error())
		}

		ifaces, err := net.Interfaces()

		if err != nil {
			panic(err.Error())
		}

		curIP := ""

		for _, iface := range ifaces {
			if iface.Name == startOpts.provIntf {
				addrs, err := iface.Addrs()

				if err != nil {
					panic(err.Error())
				}

				if len(addrs) > 0 {
					curIP = addrs[0].String()
					curIP = strings.Split(curIP, "/")[0]
				}
				break
			}
		}

		if curIP == "" {
			glog.V(0).Infof("WARNING: Unable to find provisioning IP for ProvisionServer %s (namespace %s) on interface %s!\n", startOpts.provServerName, startOpts.provServerName, startOpts.provIntf)
		} else if ip != curIP {

			unstructured.Object["status"] = map[string]interface{}{
				"provisionIp": curIP,
			}

			_, err = provServerClient.Namespace(startOpts.provServerNamespace).UpdateStatus(context.Background(), unstructured, metav1.UpdateOptions{})

			if err != nil {
				glog.V(0).Infof("Error updating ProvisionServer %s (namespace %s) \"provisionIp\" status: %s\n", startOpts.provServerName, startOpts.provServerNamespace, err)
			} else {
				ip = curIP
				glog.V(0).Infof("Updated ProvisionServer %s (namespace %s) with status \"provisionIp\": %s\n", startOpts.provServerName, startOpts.provServerNamespace, ip)

			}
		}

		time.Sleep(time.Second * 5)
	}

	glog.V(0).Info("Shutting down ProvisionIpDiscoveryAgent")
}
