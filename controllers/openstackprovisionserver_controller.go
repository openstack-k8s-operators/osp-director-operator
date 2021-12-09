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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
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

// OpenStackProvisionServerReconciler reconciles a ProvisionServer object
type OpenStackProvisionServerReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackProvisionServerReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackProvisionServerReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackProvisionServerReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackProvisionServerReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackprovisionservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackprovisionservers/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackprovisionservers/status,verbs=get;update;patch
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
func (r *OpenStackProvisionServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackprovisionserver", req.NamespacedName)

	// Fetch the ProvisionServer instance
	instance := &ospdirectorv1beta1.OpenStackProvisionServer{}
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

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.Status.ProvisioningStatus.State == ospdirectorv1beta1.ProvisionServerProvisioned)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackProvisionServer %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	// get a copy of the CR ProvisioningStatus
	actualProvisioningState := instance.Status.DeepCopy().ProvisioningStatus

	// config maps
	envVars := make(map[string]common.EnvSetter)
	cmLabels := common.GetLabels(instance, provisionserver.AppLabel, map[string]string{})

	templateParameters := make(map[string]interface{})
	templateParameters["Port"] = strconv.Itoa(instance.Spec.Port)

	cm := []common.Template{
		// Apache server config
		{
			Name:               fmt.Sprintf("%s-httpd-config", instance.Name),
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeConfig,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{},
			Labels:             cmLabels,
			ConfigOptions:      templateParameters,
		},
	}

	err = common.EnsureConfigMaps(r, instance, cm, &envVars)

	if err != nil {
		return ctrl.Result{}, err
	}

	// Get the provisioning interface of the cluster worker nodes from either Metal3
	// or from the instance spec itself if it was provided there
	if provInterfaceName == "" {
		if instance.Spec.Interface == "" {
			r.Log.Info("Provisioning interface name not yet discovered, checking Metal3...")

			provInterfaceName, err = r.getProvisioningInterface(instance)

			if err != nil {
				msg := fmt.Sprintf("Unable to acquire provisioning interface: %v", err)
				actualProvisioningState.State = ospdirectorv1beta1.ProvisionServerError
				actualProvisioningState.Reason = msg
				_ = r.setProvisioningStatus(instance, actualProvisioningState)
				return ctrl.Result{}, err
			}

			if provInterfaceName == "" {
				err := fmt.Errorf("Metal3 provisioning interface configuration not found")
				actualProvisioningState.State = ospdirectorv1beta1.ProvisionServerError
				actualProvisioningState.Reason = err.Error()
				_ = r.setProvisioningStatus(instance, actualProvisioningState)
				return ctrl.Result{}, err
			}
		} else {
			r.Log.Info(fmt.Sprintf("Provisioning interface supplied by %s spec", instance.Name))
			provInterfaceName = instance.Spec.Interface
		}
	}

	// Create or update the Deployment object
	op, err := r.deploymentCreateOrUpdate(instance, provInterfaceName)

	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Calculate overall provisioning status
	actualProvisioningState.State = ospdirectorv1beta1.ProvisionServerProvisioning
	actualProvisioningState.Reason = fmt.Sprintf("ProvisionServer %s is currently provisioning", instance.Name)

	// Provision IP Discovery Agent sets status' ProvisionIP
	if instance.Status.ProvisionIP != "" {

		// Get the current LocalImageURL IP (if any)
		curURL, err := url.Parse(instance.Status.LocalImageURL)

		if err != nil {
			r.Log.Error(err, "Failed to parse existing LocalImageURL for OpenStackProvisionServer %s: %s", instance.Name, instance.Status.LocalImageURL)
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

			r.Log.Info(fmt.Sprintf("OpenStackProvisionServer %s status' LocalImageURL updated: %s", instance.Name, instance.Status.LocalImageURL))
		}

		// Now check the associated pod's status
		podList, err := r.Kclient.CoreV1().Pods(instance.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("deployment=%s-provisionserver-deployment", instance.Name),
		})

		if err != nil && !k8s_errors.IsNotFound(err) {
			actualProvisioningState.State = ospdirectorv1beta1.ProvisionServerError
			actualProvisioningState.Reason = err.Error()
			_ = r.setProvisioningStatus(instance, actualProvisioningState)
			return ctrl.Result{}, err
		} else if err == nil {
			// There should only be one pod.  If there is more than one, we have other problems...
			for _, pod := range podList.Items {
				if pod.Status.Phase == corev1.PodRunning {
					actualProvisioningState.State = ospdirectorv1beta1.ProvisionServerProvisioned
					actualProvisioningState.Reason = fmt.Sprintf("ProvisionServer %s has been provisioned", instance.Name)
					break
				}
			}
		}
	}

	err = r.setProvisioningStatus(instance, actualProvisioningState)

	return ctrl.Result{}, err
}

// SetupWithManager - prepare controller for use with operator manager
func (r *OpenStackProvisionServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: Myabe use filtering functions here since some resource permissions
	// are now cluster-scoped?
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackProvisionServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *OpenStackProvisionServerReconciler) deploymentCreateOrUpdate(instance *ospdirectorv1beta1.OpenStackProvisionServer, provInterfaceName string) (controllerutil.OperationResult, error) {
	trueValue := true

	// Get volumes
	initVolumeMounts := provisionserver.GetInitVolumeMounts(instance.Name)
	volumeMounts := provisionserver.GetVolumeMounts(instance.Name)
	volumes := provisionserver.GetVolumes(instance.Name)

	labels := common.GetLabels(instance, provisionserver.AppLabel, map[string]string{
		"deployment": instance.Name + "-provisionserver-deployment",
	})

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
					Labels: labels,
				},
			},
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, deployment, func() error {

		replicas := int32(1)

		deployment.Spec.Replicas = &replicas
		deployment.Spec.Template.Spec = corev1.PodSpec{
			ServiceAccountName: provisionserver.ServiceAccount,
			HostNetwork:        true,
			Volumes:            volumes,
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOperator(corev1.NodeSelectorOpIn),
										Values:   []string{provisionserver.AppLabel},
									},
								},
							},
							Namespaces:  []string{instance.Namespace},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "osp-httpd",
					Image:           instance.Spec.ApacheImageURL,
					ImagePullPolicy: corev1.PullAlways,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &trueValue,
					},
					Env: []corev1.EnvVar{},
					// FIXME: Everything else I've tried has failed to inject the desired httpd.conf,
					//        so we're hacking it this way for now
					Command: []string{
						"/bin/bash",
						"-ec",
						"cp -f /usr/local/apache2/conf/httpd.conf /etc/httpd/conf/httpd.conf && /usr/bin/run-httpd",
					},
					VolumeMounts: volumeMounts,
				},
				{
					Name:            "osp-provision-ip-discovery-agent",
					Args:            []string{"start"},
					Command:         []string{"provision-ip-discovery-agent"},
					Image:           instance.Spec.ProvisioningAgentImageURL,
					ImagePullPolicy: corev1.PullAlways,
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
				ContainerImage: instance.Spec.DownloaderImageURL,
				Env: []corev1.EnvVar{
					{
						Name:  "RHEL_IMAGE_URL",
						Value: instance.Spec.BaseImageURL,
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

func (r *OpenStackProvisionServerReconciler) getLocalImageURL(instance *ospdirectorv1beta1.OpenStackProvisionServer) string {
	baseFilename := instance.Spec.BaseImageURL[strings.LastIndex(instance.Spec.BaseImageURL, "/")+1 : len(instance.Spec.BaseImageURL)]
	baseFilenameEnd := baseFilename[len(baseFilename)-3:]

	if baseFilenameEnd == ".gz" || baseFilenameEnd == ".xz" {
		baseFilename = baseFilename[0 : len(baseFilename)-3]
	}

	return fmt.Sprintf("http://%s:%d/images/%s/compressed-%s", instance.Status.ProvisionIP, instance.Spec.Port, baseFilename, baseFilename)
}

func (r *OpenStackProvisionServerReconciler) getProvisioningInterface(instance *ospdirectorv1beta1.OpenStackProvisionServer) (string, error) {
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

func (r *OpenStackProvisionServerReconciler) setProvisioningStatus(instance *ospdirectorv1beta1.OpenStackProvisionServer, actualState ospdirectorv1beta1.OpenStackProvisionServerProvisioningStatus) error {
	// get current ProvisioningStatus
	currentState := instance.Status.ProvisioningStatus

	// if the current ProvisioningStatus is different from the actual, store the update
	// otherwise, just log the status again
	if !reflect.DeepEqual(currentState, actualState) {
		r.Log.Info(fmt.Sprintf("%s - diff %s", actualState.Reason, diff.ObjectReflectDiff(currentState, actualState)))
		instance.Status.ProvisioningStatus = actualState

		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
		instance.Status.Conditions.Set(ospdirectorv1beta1.ConditionType(actualState.State), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(actualState.Reason), actualState.Reason)

		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			r.Log.Error(err, "Failed to update CR status %v")
			return err
		}
	} else if actualState.Reason != "" {
		r.Log.Info(actualState.Reason)
	}

	return nil
}
