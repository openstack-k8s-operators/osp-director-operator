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

package controllers

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	provisionserver "github.com/openstack-k8s-operators/osp-director-operator/pkg/provisionserver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	provInterfaceName = ""
	provisioningsGVR  = schema.GroupVersionResource{
		Group:    "metal3.io",
		Version:  "v1alpha1",
		Resource: "provisionings",
	}
)

// ProvisionServerReconciler reconciles a ProvisionServer object
type ProvisionServerReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *ProvisionServerReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *ProvisionServerReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *ProvisionServerReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *ProvisionServerReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=provisionservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;create;update;delete;watch;
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=get;list;create;update;delete;watch;
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;create;update;delete;watch;
// +kubebuilder:rbac:groups=core,resources=volumes,verbs=get;list;create;update;delete;watch;
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;update;watch;
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;update;watch;
// +kubebuilder:rbac:groups=machine.openshift.io,resources="*",verbs="*"
// +kubebuilder:rbac:groups=metal3.io,resources="*",verbs="*"
// +kubebuilder:rbac:groups=security.openshift.io,namespace=openstack,resources="securitycontextconstraints",resourceNames="privileged",verbs="use"
// +kubebuilder:rbac:groups=security.openshift.io,namespace=openstack,resources="securitycontextconstraints",resourceNames="anyuid",verbs="use"

// Reconcile - provision image servers
func (r *ProvisionServerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("provisionserver", req.NamespacedName)

	// Fetch the ProvisionServer instance
	instance := &ospdirectorv1beta1.ProvisionServer{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// config maps
	envVars := make(map[string]common.EnvSetter)
	cmLabels := common.GetLabels(instance.Name, provisionserver.AppLabel)

	templateParameters := make(map[string]interface{})
	templateParameters["Port"] = strconv.Itoa(instance.Spec.Port)

	cm := []common.Template{
		// Apache server config
		{
			Name:           fmt.Sprintf("%s-httpd-config", instance.Name),
			Namespace:      instance.Namespace,
			Type:           common.TemplateTypeConfig,
			InstanceType:   instance.Kind,
			AdditionalData: map[string]string{},
			Labels:         cmLabels,
			ConfigOptions:  templateParameters,
		},
	}

	err = common.EnsureConfigMaps(r, instance, cm, &envVars)

	if err != nil {
		return ctrl.Result{}, err
	}

	// Get the provisioning interface of the cluster worker nodes
	if provInterfaceName == "" {
		r.Log.Info("Provisioning interface name not yet discovered, checking Metal3...")

		provInterfaceName, err = r.getProvisioningInterface(instance)

		if err != nil {
			return ctrl.Result{}, err
		}

		if provInterfaceName == "" {
			return ctrl.Result{}, fmt.Errorf("metal3 provisioning interface configuration not found")
		}
	}

	// provisionserver
	// Create or update the Deployment object
	op, err := r.deploymentCreateOrUpdate(instance, provInterfaceName)

	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Provision IP Discovery Agent sets status' ProvisionIP
	if instance.Status.ProvisionIP != "" {

		// Get the current LocalImageURL IP (if any)
		curURL, err := url.Parse(instance.Status.LocalImageURL)

		if err != nil {
			r.Log.Error(err, "Failed to parse existing LocalImageURL for ProvisionServer %s: %s", instance.Name, instance.Status.LocalImageURL)
			return ctrl.Result{}, err
		}

		// If the current LocalImageURL is empty, or its embedded IP does not equal the ProvisionIP, the update the LocalImageURL
		if instance.Status.LocalImageURL == "" || curURL.Hostname() != instance.Status.ProvisionIP {
			// Update status with LocalImageURL, given ProvisionIP status value
			instance.Status.LocalImageURL = r.getLocalImageURL(instance)
			err = r.Client.Status().Update(context.TODO(), instance)

			if err != nil {
				r.Log.Error(err, "Failed to update CR status %v")
				return ctrl.Result{}, err
			}

			r.Log.Info(fmt.Sprintf("ProvisionServer %s status' LocalImageURL updated: %s", instance.Name, instance.Status.LocalImageURL))
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager - prepare controller for use with operator manager
func (r *ProvisionServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: Myabe use filtering functions here since some resource permissions
	// are now cluster-scoped?
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.ProvisionServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *ProvisionServerReconciler) deploymentCreateOrUpdate(instance *ospdirectorv1beta1.ProvisionServer, provInterfaceName string) (controllerutil.OperationResult, error) {

	// Get volumes
	initVolumeMounts := provisionserver.GetInitVolumeMounts(instance.Name)
	volumeMounts := provisionserver.GetVolumeMounts(instance.Name)
	volumes := provisionserver.GetVolumes(instance.Name)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"deployment": instance.Name + "-provisionserver-deployment"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"deployment": instance.Name + "-provisionserver-deployment"},
				},
			},
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, deployment, func() error {

		replicas := int32(1)

		deployment.Spec.Replicas = &replicas
		deployment.Spec.Template.Spec = corev1.PodSpec{
			HostNetwork: true,
			Volumes:     volumes,
			Containers: []corev1.Container{
				{
					Name:         "osp-httpd",
					Image:        "quay.io/abays/httpd:2.4-alpine",
					Env:          []corev1.EnvVar{},
					VolumeMounts: volumeMounts,
				},
				{
					Name:    "osp-provision-ip-discovery-agent",
					Args:    []string{"start"},
					Command: []string{"provision-ip-discovery-agent"},
					// TODO: Create an openstack-k8s-operators quay image/tag for this
					Image: "quay.io/abays/provision-ip-discovery-agent:0.0.11",
					Env: []corev1.EnvVar{
						{
							Name:  "PROV_INTF",
							Value: provInterfaceName,
						},
						{
							Name:  "PROV_SERVER_NAME",
							Value: instance.GetName(),
						},
						{
							Name:  "PROV_SERVER_NAMESPACE",
							Value: instance.GetNamespace(),
						},
					},
				},
			},
		}

		initContainerDetails := []provisionserver.InitContainer{
			{
				// TODO: Create an openstack-k8s-operators quay image/tag for this
				ContainerImage: "quay.io/abays/downloader:0.0.3",
				Env: []corev1.EnvVar{
					{
						Name:  "RHEL_IMAGE_URL",
						Value: instance.Spec.RhelImageURL,
					},
				},
				Privileged:   false,
				VolumeMounts: initVolumeMounts,
			},
		}

		deployment.Spec.Template.Spec.InitContainers = provisionserver.GetInitContainers(initContainerDetails)

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)

		if err != nil {
			return err
		}

		return nil
	})

	return op, err
}

func (r *ProvisionServerReconciler) getLocalImageURL(instance *ospdirectorv1beta1.ProvisionServer) string {
	baseFilename := instance.Spec.RhelImageURL[strings.LastIndex(instance.Spec.RhelImageURL, "/")+1 : len(instance.Spec.RhelImageURL)]
	baseFilenameEnd := baseFilename[len(baseFilename)-3:]

	if baseFilenameEnd == ".gz" || baseFilenameEnd == ".xz" {
		baseFilename = baseFilename[0 : len(baseFilename)-3]
	}

	return fmt.Sprintf("http://%s:%d/images/%s/compressed-%s", instance.Status.ProvisionIP, instance.Spec.Port, baseFilename, baseFilename)
}

func (r *ProvisionServerReconciler) getProvisioningInterface(instance *ospdirectorv1beta1.ProvisionServer) (string, error) {
	cfg, err := config.GetConfig()

	if err != nil {
		return "", err
	}

	dynClient, err := dynamic.NewForConfig(cfg)

	if err != nil {
		return "", err
	}

	provisioningsClient := dynClient.Resource(provisioningsGVR)

	provisioning, err := provisioningsClient.Get(context.TODO(), "provisioning-configuration", metav1.GetOptions{})

	if err != nil {
		return "", err
	}

	provisioningSpecIntf := provisioning.Object["spec"]

	if provisioningSpec, ok := provisioningSpecIntf.(map[string]interface{}); ok {
		interfaceIntf := provisioningSpec["provisioningInterface"]

		if provInterfaceName, ok := interfaceIntf.(string); ok {
			r.Log.Info(fmt.Sprintf("Found provisioning interface %s in Metal3 config", provInterfaceName))
			return provInterfaceName, nil
		}
	}

	return "", nil
}
