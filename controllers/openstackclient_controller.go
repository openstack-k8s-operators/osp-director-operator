/*
Copyright 2020 Red Hat

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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackClientReconciler reconciles a OpenStackClient object
type OpenStackClientReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackClientReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackClientReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackClientReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackClientReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=create;delete;get;list;patch;update;watch

// Reconcile - openstackclient
func (r *OpenStackClientReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("openstackclient", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackClient{}
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

	envVars := make(map[string]common.EnvSetter)

	// check for required secrets
	_, _, err = common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil && errors.IsNotFound(err) {
		return ctrl.Result{RequeueAfter: time.Second * 20}, fmt.Errorf("DeploymentSSHSecret secret does not exist: %v", err)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	ipset, op, err := r.overcloudipsetCreateOrUpdate(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("IPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	if len(ipset.Status.HostIPs) != instance.Spec.VMCount {
		r.Log.Info(fmt.Sprintf("IPSet has not yet reached the required replicas %d", instance.Spec.VMCount))
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	templateParameters := make(map[string]interface{})
	cmLabels := common.GetLabels(instance.Name, openstackclient.AppLabel)
	cms := []common.Template{
		// Custom CM holding Tripleo deployment environment parameter files
		{
			Name:      "tripleo-deploy-config-custom",
			Namespace: instance.Namespace,
			Type:      common.TemplateTypeCustom,
			Labels:    cmLabels,
		},
		// Custom CM holding Tripleo net-config files then used in parameter files
		{
			Name:      "tripleo-net-config",
			Namespace: instance.Namespace,
			Type:      common.TemplateTypeCustom,
			Labels:    cmLabels,
		},
	}
	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil {
		return ctrl.Result{}, nil
	}

	// get tripleo-deploy-config-custom, created/rendered by overcloudipset controller
	tripleoCustomDeployCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-deploy-config-custom", instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	tripleoCustomDeployFiles := tripleoCustomDeployCM.Data
	templateParameters["TripleoCustomDeployFiles"] = tripleoCustomDeployFiles

	// get tripleo-deploy-config, created/rendered by overcloudipset controller
	tripleoDeployCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-deploy-config", instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		return ctrl.Result{}, err
	}

	tripleoDeployFiles := tripleoDeployCM.Data
	templateParameters["TripleoDeployFiles"] = tripleoDeployFiles

	// create cm holding deployment script and render deployment script.
	// All yaml files from tripleo-deploy-config-custom and tripleo-deploy-config
	// are added as environment files.
	cms = []common.Template{
		// ScriptsConfigMap
		{
			Name:           "openstackclient-sh",
			Namespace:      instance.Namespace,
			Type:           common.TemplateTypeScripts,
			InstanceType:   instance.Kind,
			AdditionalData: map[string]string{},
			ConfigOptions:  templateParameters,
			Labels:         cmLabels,
		},
	}
	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil {
		return ctrl.Result{}, nil
	}

	// PVCs
	// volume to presistent store /etc/hosts where entries get added by tripleo deploy
	pvcDetails := common.Pvc{
		Name:         fmt.Sprintf("%s-hosts", instance.Name),
		Namespace:    instance.Namespace,
		Size:         openstackclient.HostsPersistentStorageSize,
		Labels:       common.GetLabels(instance.Name, openstackclient.AppLabel),
		StorageClass: openstackclient.PersistentStorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}

	pvc, op, err := common.CreateOrUpdatePvc(r, instance, &pvcDetails)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Openstackclient /etc/hosts PVC %s Updated", pvc.Name))
	}

	pvcDetails = common.Pvc{
		Name:         fmt.Sprintf("%s-cloud-admin", instance.Name),
		Namespace:    instance.Namespace,
		Size:         openstackclient.CloudAdminPersistentStorageSize,
		Labels:       common.GetLabels(instance.Name, openstackclient.AppLabel),
		StorageClass: openstackclient.PersistentStorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}

	pvc, op, err = common.CreateOrUpdatePvc(r, instance, &pvcDetails)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Openstackclient PVC /home/cloud-admin %s Updated", pvc.Name))
	}

	// Create or update the pod object
	op, err = r.podCreateOrUpdate(instance, envVars)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Pod %s successfully reconciled - operation: %s", instance.Name, string(op)))
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *OpenStackClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackClient{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *OpenStackClientReconciler) overcloudipsetCreateOrUpdate(instance *ospdirectorv1beta1.OpenStackClient) (*ospdirectorv1beta1.OvercloudIPSet, controllerutil.OperationResult, error) {
	overcloudIPSet := &ospdirectorv1beta1.OvercloudIPSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.ObjectMeta.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, overcloudIPSet, func() error {
		overcloudIPSet.Spec.Networks = instance.Spec.Networks
		overcloudIPSet.Spec.Role = "OpenstackClient"
		overcloudIPSet.Spec.HostCount = 1

		err := controllerutil.SetControllerReference(instance, overcloudIPSet, r.Scheme)

		if err != nil {
			return err
		}

		return nil
	})

	return overcloudIPSet, op, err
}

func (r *OpenStackClientReconciler) podCreateOrUpdate(instance *ospdirectorv1beta1.OpenStackClient, envVars map[string]common.EnvSetter) (controllerutil.OperationResult, error) {
	var terminationGracePeriodSeconds int64 = 0
	runAsUser := int64(openstackclient.CloudAdminUID)
	runAsGroup := int64(openstackclient.CloudAdminGID)

	// Get volumes
	initVolumeMounts := openstackclient.GetInitVolumeMounts(instance)
	volumeMounts := openstackclient.GetVolumeMounts(instance)
	volumes := openstackclient.GetVolumes(instance)

	envVars["OS_CLOUD"] = common.EnvValue(instance.Spec.CloudName)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			// TODO: remove hard coded IP
			Annotations: map[string]string{"k8s.v1.cni.cncf.io/networks": `[{"name": "osp-static", "namespace": "openstack", "ips": ["192.168.25.6/24"]}]`},
		},
	}
	pod.Spec = corev1.PodSpec{
		ServiceAccountName: openstackclient.ServiceAccount,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:  &runAsUser,
			RunAsGroup: &runAsGroup,
			FSGroup:    &runAsGroup,
		},
		TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
		Volumes:                       volumes,
		Containers: []corev1.Container{
			{
				Name:            "openstackclient",
				Image:           instance.Spec.ImageURL,
				ImagePullPolicy: corev1.PullAlways,
				Command: []string{
					"/bin/bash",
					"-c",
					"/bin/sleep infinity",
				},
				Env:          common.MergeEnvs([]corev1.EnvVar{}, envVars),
				VolumeMounts: volumeMounts,
			},
		},
	}

	initContainerDetails := []openstackclient.InitContainer{
		{
			ContainerImage: instance.Spec.ImageURL,
			Commands: []string{
				"/bin/bash", "-c", "/usr/local/bin/init.sh",
			},
			Env:          common.MergeEnvs([]corev1.EnvVar{}, envVars),
			Privileged:   false,
			VolumeMounts: initVolumeMounts,
		},
	}

	pod.Spec.InitContainers = openstackclient.GetInitContainers(initContainerDetails)

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, pod, func() error {
		// HostAliases
		pod.Spec.HostAliases = instance.Spec.HostAliases
		pod.Spec.Containers[0].Env = common.MergeEnvs(pod.Spec.Containers[0].Env, envVars)

		// labels
		common.InitMap(&pod.Labels)
		for k, v := range common.GetLabels(instance.Name, openstackclient.AppLabel) {
			pod.Labels[k] = v
		}

		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil && errors.IsInvalid(err) {
		// Delete pod when an unsupported change was requested, like
		// e.g. additional controller VM got up. We just re-create the
		// openstackclient pod
		r.Log.Info(fmt.Sprintf("openstackclient pod deleted due to spec change %v", err))
		if err := r.Client.Delete(context.TODO(), pod); err != nil {
			return op, err
		}

		r.Log.Info("openstackclient pod deleted due to spec change")
		return controllerutil.OperationResultUpdated, nil
	}
	return op, err
}
