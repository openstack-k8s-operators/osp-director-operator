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
	"strings"
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
func (r *OpenStackClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	ipsetDetails := common.IPSet{
		Networks:            instance.Spec.Networks,
		Role:                openstackclient.Role,
		HostCount:           openstackclient.Count,
		AddToPredictableIPs: false,
	}
	ipset, op, err := common.OvercloudipsetCreateOrUpdate(r, instance, ipsetDetails)
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("IPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	if len(ipset.Status.HostIPs) != openstackclient.Count {
		r.Log.Info(fmt.Sprintf("IPSet has not yet reached the required replicas %d", openstackclient.Count))
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// for now there is only support for a single openstackclient container per cr
	hostnameDetails := common.Hostname{
		IDKey:    fmt.Sprintf("%s-%d", strings.ToLower(openstackclient.Role), 0),
		Basename: openstackclient.Role,
		VIP:      false,
	}
	err = common.CreateOrGetHostname(instance, &hostnameDetails)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.Log.Info(fmt.Sprintf("OpenStackClient %s hostname set to %s", hostnameDetails.IDKey, hostnameDetails.Hostname))

	// verify that NodeNetworkConfigurationPolicy and NetworkAttachmentDefinition for each configured network exists
	nncMap, err := common.GetAllNetworkConfigurationPolicies(r, map[string]string{"owner": "osp-director"})
	if err != nil {
		return ctrl.Result{}, err
	}
	nadMap, err := common.GetAllNetworkAttachmentDefinitions(r, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, net := range instance.Spec.Networks {
		if _, ok := nncMap[net]; !ok {
			r.Log.Info(fmt.Sprintf("NetworkConfigurationPolicy for network %s does not yet exist.  Reconciling again in 10 seconds", net))
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
		if _, ok := nadMap[net]; !ok {
			r.Log.Error(err, fmt.Sprintf("NetworkAttachmentDefinition for network %s does not yet exist.  Reconciling again in 10 seconds", net))
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
	}

	// update network status
	for _, netName := range instance.Spec.Networks {
		r.setNetStatus(instance, &hostnameDetails, netName, ipset.Status.HostIPs[hostnameDetails.IDKey].IPAddresses[netName])
	}
	r.Log.Info(fmt.Sprintf("OpenStackClient network status for Hostname: %s - %s", instance.Status.OpenStackClientNetStatus[hostnameDetails.IDKey].Hostname, instance.Status.OpenStackClientNetStatus[hostnameDetails.IDKey].IPAddresses))

	err = r.Client.Status().Update(context.TODO(), instance)
	if err != nil {
		r.Log.Error(err, "Failed to update CR status %v")
		return ctrl.Result{}, err
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

	// get tripleo-deploy-config-custom, created/rendered by openstackipset controller
	tripleoCustomDeployCM, _, err := common.GetConfigMapAndHashWithName(r, "tripleo-deploy-config-custom", instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	tripleoCustomDeployFiles := tripleoCustomDeployCM.Data
	templateParameters["TripleoCustomDeployFiles"] = tripleoCustomDeployFiles

	// get tripleo-deploy-config, created/rendered by openstackipset controller
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
		StorageClass: instance.Spec.StorageClass,
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
		StorageClass: instance.Spec.StorageClass,
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
	op, err = r.podCreateOrUpdate(instance, &hostnameDetails, envVars)
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Pod %s successfully reconciled - operation: %s", instance.Name, string(op)))
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

func (r *OpenStackClientReconciler) setNetStatus(instance *ospdirectorv1beta1.OpenStackClient, hostnameDetails *common.Hostname, netName string, ipaddress string) {

	// If OpenStackClient status map is nil, create it
	if instance.Status.OpenStackClientNetStatus == nil {
		instance.Status.OpenStackClientNetStatus = map[string]ospdirectorv1beta1.HostStatus{}
	}

	// Set network information status
	if instance.Status.OpenStackClientNetStatus[hostnameDetails.IDKey].IPAddresses == nil {
		instance.Status.OpenStackClientNetStatus[hostnameDetails.IDKey] = ospdirectorv1beta1.HostStatus{
			Hostname: hostnameDetails.Hostname,
			IPAddresses: map[string]string{
				netName: ipaddress,
			},
		}
	} else {
		instance.Status.OpenStackClientNetStatus[hostnameDetails.IDKey].IPAddresses[netName] = ipaddress
	}
}

func (r *OpenStackClientReconciler) podCreateOrUpdate(instance *ospdirectorv1beta1.OpenStackClient, hostnameDetails *common.Hostname, envVars map[string]common.EnvSetter) (controllerutil.OperationResult, error) {
	var terminationGracePeriodSeconds int64 = 0
	runAsUser := int64(openstackclient.CloudAdminUID)
	runAsGroup := int64(openstackclient.CloudAdminGID)

	// Get volumes
	initVolumeMounts := openstackclient.GetInitVolumeMounts(instance)
	volumeMounts := openstackclient.GetVolumeMounts(instance)
	volumes := openstackclient.GetVolumes(instance)

	envVars["OS_CLOUD"] = common.EnvValue(instance.Spec.CloudName)

	// create k8s.v1.cni.cncf.io/networks network annotation to attach OpenStackClient to networks set in instance.Spec.Networks
	annotation := "["
	for id, net := range instance.Spec.Networks {
		annotation += fmt.Sprintf("{\"name\": \"%s-static\", \"namespace\": \"%s\", \"ips\": [\"%s\"]}", net, instance.Namespace, instance.Status.OpenStackClientNetStatus[hostnameDetails.IDKey].IPAddresses[net])
		if id < len(instance.Spec.Networks)-1 {
			annotation += ", "
		}
	}
	annotation += "]"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/networks": annotation,
			},
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
				"/bin/bash",
				"-c",
				"/usr/local/bin/init.sh",
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
