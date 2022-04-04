/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	//cni "github.com/containernetworking/cni/pkg/types"
	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nmstate "github.com/nmstate/kubernetes-nmstate/api/v1alpha1"
	virtv1 "kubevirt.io/api/core/v1"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	machinev1beta1 "github.com/openshift/cluster-api/pkg/apis/machine/v1beta1"
	sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/controllers"
	//cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	//templatev1 "github.com/openshift/api/template/v1"
	// +kubebuilder:scaffold:imports
)

const (
	// WebhookPort -
	WebhookPort = 4343
	// WebhookCertDir -
	WebhookCertDir = "/apiserver.local.config/certificates"
	// WebhookCertName -
	WebhookCertName = "apiserver.crt"
	// WebhookKeyName -
	WebhookKeyName = "apiserver.key"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ospdirectorv1beta1.AddToScheme(scheme))
	//utilruntime.Must(templatev1.AddToScheme(scheme))
	utilruntime.Must(virtv1.AddToScheme(scheme))
	utilruntime.Must(nmstate.AddToScheme(scheme))
	utilruntime.Must(networkv1.AddToScheme(scheme))
	//utilruntime.Must(cdiv1.AddToScheme(scheme))
	utilruntime.Must(metal3v1alpha1.AddToScheme(scheme))
	utilruntime.Must(machinev1beta1.AddToScheme(scheme))
	utilruntime.Must(sriovnetworkv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var enableWebhooks bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	namespace, err := getWatchNamespace()
	if err != nil {
		setupLog.Error(err, "failed to get WatchNamespace")
		os.Exit(1)

	}

	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "576d6738.openstack.org",
	}

	// create multi namespace cache if list of namespaces
	if strings.Contains(namespace, ",") {
		options.Namespace = ""
		options.NewCache = cache.MultiNamespacedCacheBuilder(strings.Split(namespace, ","))
		setupLog.Info(fmt.Sprintf("Namespaces added to the cache: %s", namespace))
	} else {
		options.Namespace = namespace
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}
	kclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if strings.ToLower(os.Getenv("ENABLE_WEBHOOKS")) != "false" {
		enableWebhooks = true

		// We're just getting a pointer here and overriding the default values
		srv := mgr.GetWebhookServer()
		srv.CertDir = WebhookCertDir
		srv.CertName = WebhookCertName
		srv.KeyName = WebhookKeyName
		srv.Port = WebhookPort
	}

	if err = (&controllers.OpenStackControlPlaneReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackControlPlane"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackControlPlane")
		os.Exit(1)
	}
	if err = (&controllers.OpenStackVMSetReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackVMSet"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackVMSet")
		os.Exit(1)
	}
	if err = (&controllers.OpenStackProvisionServerReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackProvisionServer"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackProvisionServer")
		os.Exit(1)
	}
	if err = (&controllers.OpenStackBaremetalSetReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackBaremetalSet"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackBaremetalSet")
		os.Exit(1)
	}
	if err = (&controllers.OpenStackClientReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackClient"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackClient")
		os.Exit(1)
	}
	if err = (&controllers.OpenStackNetReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackNet"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackNet")
		os.Exit(1)
	}
	if err = (&controllers.OpenStackConfigGeneratorReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackConfigGenerator"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackConfigGenerator")
		os.Exit(1)
	}
	if err = (&controllers.OpenStackEphemeralHeatReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackEphemeralHeat"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackEphemeralHeat")
		os.Exit(1)
	}

	if err = (&controllers.OpenStackConfigVersionReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackConfigVersion"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackConfigVersion")
		os.Exit(1)
	}

	if err = (&controllers.OpenStackMACAddressReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackMACAddress"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackMACAddress")
		os.Exit(1)
	}

	if err = (&controllers.OpenStackNetConfigReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackNetConfig"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackNetConfig")
		os.Exit(1)
	}

	if err = (&controllers.OpenStackNetAttachmentReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackNetAttachment"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackNetAttachment")
		os.Exit(1)
	}

	if err = (&controllers.OpenStackBackupReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackBackup"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackBackup")
		os.Exit(1)
	}

	if err = (&controllers.OpenStackBackupRequestReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackBackupRequest"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackBackupRequest")
		os.Exit(1)
	}

	if err = (&controllers.OpenStackDeployReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackDeploy"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackDeploy")
		os.Exit(1)
	}

	if err = (&controllers.OpenStackIPSetReconciler{
		Client:  mgr.GetClient(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackIPSet"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackIPSet")
		os.Exit(1)
	}

	if enableWebhooks {
		//
		// DEFAULTS
		//
		openstackControlPlaneDefaults := ospdirectorv1beta1.OpenStackControlPlaneDefaults{
			OpenStackRelease: os.Getenv("OPENSTACK_RELEASE_DEFAULT"),
		}

		openstackClientDefaults := ospdirectorv1beta1.OpenStackClientDefaults{
			ImageURL: os.Getenv("OPENSTACKCLIENT_IMAGE_URL_DEFAULT"),
		}

		ephemeralHeatDefaults := ospdirectorv1beta1.OpenStackEphemeralHeatDefaults{
			HeatAPIImageURL:    os.Getenv("HEAT_API_IMAGE_URL_DEFAULT"),
			HeatEngineImageURL: os.Getenv("HEAT_ENGINE_IMAGE_URL_DEFAULT"),
			MariaDBImageURL:    os.Getenv("MARIADB_IMAGE_URL_DEFAULT"),
			RabbitImageURL:     os.Getenv("RABBITMQ_IMAGE_URL_DEFAULT"),
		}

		provisionServerDefaults := ospdirectorv1beta1.OpenStackProvisionServerDefaults{
			DownloaderImageURL: os.Getenv("DOWNLOADER_IMAGE_URL_DEFAULT"),
			AgentImageURL:      os.Getenv("AGENT_IMAGE_URL_DEFAULT"),
			ApacheImageURL:     os.Getenv("APACHE_IMAGE_URL_DEFAULT"),
		}

		openstackDeployDefaults := ospdirectorv1beta1.OpenStackDeployDefaults{
			AgentImageURL: os.Getenv("AGENT_IMAGE_URL_DEFAULT"),
		}

		openstackConfigGeneratorDefaults := ospdirectorv1beta1.OpenStackConfigGeneratorDefaults{
			ImageURL: os.Getenv("OPENSTACKCLIENT_IMAGE_URL_DEFAULT"),
		}

		//
		// Register webhooks
		//
		if err = (&ospdirectorv1beta1.OpenStackBaremetalSet{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackBaremetalSet")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackEphemeralHeat{}).SetupWebhookWithManager(mgr, ephemeralHeatDefaults); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackEphemeralHeat")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackProvisionServer{}).SetupWebhookWithManager(mgr, provisionServerDefaults); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackProvisionServer")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackControlPlane{}).SetupWebhookWithManager(mgr, openstackControlPlaneDefaults); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackControlPlane")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackVMSet{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackVMSet")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackConfigGenerator{}).SetupWebhookWithManager(mgr, openstackConfigGeneratorDefaults); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackConfigGenerator")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackNetConfig{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackNetConfig")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackNetAttachment{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackNetAttachment")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackNet{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackNet")
			os.Exit(1)
		}
		if err = (&ospdirectorv1beta1.OpenStackBackup{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackBackup")
			os.Exit(1)
		}
		if err = (&ospdirectorv1beta1.OpenStackBackupRequest{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackBackupRequest")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackDeploy{}).SetupWebhookWithManager(mgr, openstackDeployDefaults); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackDeploy")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackClient{}).SetupWebhookWithManager(mgr, openstackClientDefaults); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackClient")
			os.Exit(1)
		}

		if err = (&ospdirectorv1beta1.OpenStackIPSet{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackIPSet")
			os.Exit(1)
		}
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// getWatchNamespace returns the Namespace the operator should be watching for changes
func getWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}
