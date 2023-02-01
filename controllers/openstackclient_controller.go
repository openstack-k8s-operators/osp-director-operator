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
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackipset"
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

	// Fetch the OpenStackClient instance
	instance := &ospdirectorv1beta1.OpenStackClient{}
	err := r.Get(ctx, req.NamespacedName, instance)
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

	//
	// initialize condition
	//
	cond := instance.Status.Conditions.InitCondition()

	if instance.Status.OpenStackClientNetStatus == nil {
		instance.Status.OpenStackClientNetStatus = map[string]ospdirectorv1beta1.HostStatus{}
	}

	//
	// Used in comparisons below to determine whether a status update is actually needed
	//
	currentStatus := instance.Status.DeepCopy()
	statusChanged := func() bool {
		return !equality.Semantic.DeepEqual(
			r.getNormalizedStatus(&instance.Status),
			r.getNormalizedStatus(currentStatus),
		)
	}

	defer func(cond *shared.Condition) {
		//
		// Update object conditions
		//
		instance.Status.Conditions.UpdateCurrentCondition(
			cond.Type,
			cond.Reason,
			cond.Message,
		)

		if statusChanged() {
			if updateErr := r.Status().Update(context.Background(), instance); updateErr != nil {
				common.LogErrorForObject(r, updateErr, "Update status", instance)
			}
		}

		// log current status message to operator log
		common.LogForObject(r, cond.Message, instance)
	}(cond)

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.IsReady())

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackClient %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	var ctrlResult ctrl.Result
	currentLabels := instance.DeepCopy().Labels

	//
	// Only kept for running local
	// add osnetcfg CR label reference which is used in the in the osnetcfg
	// controller to watch this resource and reconcile
	//
	if _, ok := currentLabels[shared.OpenStackNetConfigReconcileLabel]; !ok {
		common.LogForObject(r, "osnetcfg reference label not added by webhook, adding it!", instance)
		instance.Labels, err = ospdirectorv1beta1.AddOSNetConfigRefLabel(
			r.Client,
			instance.Namespace,
			instance.Spec.Networks[0],
			currentLabels,
		)
		if err != nil {
			return ctrlResult, err
		}
	}

	//
	// add labels of all networks used by this CR
	//
	instance.Labels = ospdirectorv1beta1.AddOSNetNameLowerLabels(r.GetLogger(), instance.Labels, instance.Spec.Networks)

	//
	// update instance to sync labels if changed
	//
	if !equality.Semantic.DeepEqual(
		currentLabels,
		instance.Labels,
	) {
		err = r.Update(ctx, instance)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
			cond.Reason = shared.CommonCondReasonAddOSNetLabelError
			cond.Type = shared.CommonCondTypeError

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
	}

	envVars := make(map[string]common.EnvSetter)
	labels := common.GetLabels(instance, openstackclient.AppLabel, map[string]string{})

	//
	// check for DeploymentSSHSecret is there
	//
	if instance.Spec.DeploymentSSHSecret != "" {
		_, ctrlResult, err = common.GetDataFromSecret(
			ctx,
			r,
			instance,
			cond,
			shared.ConditionDetails{
				ConditionNotFoundType:   shared.CommonCondTypeWaiting,
				ConditionNotFoundReason: shared.CommonCondReasonDeploymentSecretMissing,
				ConditionErrorType:      shared.CommonCondTypeError,
				ConditionErrordReason:   shared.CommonCondReasonDeploymentSecretError,
			},
			instance.Spec.DeploymentSSHSecret,
			20,
			"",
		)
		if (err != nil) || (ctrlResult != ctrl.Result{}) {
			return ctrlResult, err
		}
	}

	//
	// check CAConfigMap is there
	//
	if instance.Spec.CAConfigMap != "" {
		_, ctrlResult, err = common.GetConfigMap(
			ctx,
			r,
			instance,
			cond,
			shared.ConditionDetails{
				ConditionNotFoundType:   shared.CommonCondTypeWaiting,
				ConditionNotFoundReason: shared.CommonCondReasonCAConfigMapMissing,
				ConditionErrorType:      shared.CommonCondTypeError,
				ConditionErrordReason:   shared.CommonCondReasonCAConfigMapError,
			},
			instance.Spec.CAConfigMap,
			20,
		)
		if (err != nil) || (ctrlResult != ctrl.Result{}) {
			return ctrlResult, err
		}
	}

	//
	// check for IdmSecret is there
	//
	if instance.Spec.IdmSecret != "" {
		_, ctrlResult, err = common.GetDataFromSecret(
			ctx,
			r,
			instance,
			cond,
			shared.ConditionDetails{
				ConditionNotFoundType:   shared.CommonCondTypeWaiting,
				ConditionNotFoundReason: shared.CommonCondReasonIdmSecretMissing,
				ConditionErrorType:      shared.CommonCondTypeError,
				ConditionErrordReason:   shared.CommonCondReasonIdmSecretError,
			},
			instance.Spec.IdmSecret,
			20,
			"",
		)
		if (err != nil) || (ctrlResult != ctrl.Result{}) {
			return ctrlResult, err
		}
	}

	//
	// create openstackclient IPs for all networks
	//
	ipsetStatus, ctrlResult, err := openstackipset.EnsureIPs(
		ctx,
		r,
		instance,
		cond,
		instance.Name,
		instance.Spec.Networks,
		1,
		false,
		false,
		[]string{},
		false,
	)

	for _, status := range ipsetStatus {
		hostStatus := ospdirectorv1beta1.SyncIPsetStatus(cond, instance.Status.OpenStackClientNetStatus, status)
		instance.Status.OpenStackClientNetStatus[status.Hostname] = hostStatus
	}

	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// NetworkAttachmentDefinition
	//
	ctrlResult, err = r.verifyNetworkAttachments(
		ctx,
		instance,
		cond,
	)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// create cm holding deployment script and render deployment script.
	//
	cms := []common.Template{
		// ScriptsConfigMap
		{
			Name:               "openstackclient-sh",
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{},
			ConfigOptions:      make(map[string]interface{}),
			Labels:             labels,
		},
	}
	err = common.EnsureConfigMaps(ctx, r, instance, cms, &envVars)
	if err != nil && k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	//
	// PVCs
	//
	err = r.createPVCs(
		ctx,
		instance,
		cond,
		labels,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	for hostname := range instance.Status.OpenStackClientNetStatus {
		//
		// Create or update the pod object
		//
		err = r.podCreateOrUpdate(
			ctx,
			instance,
			cond,
			hostname,
			&envVars,
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		hostStatus := instance.Status.OpenStackClientNetStatus[hostname]
		hostStatus.ProvisioningState = shared.ProvisioningState(cond.Type)
		instance.Status.OpenStackClientNetStatus[hostname] = hostStatus
	}

	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *OpenStackClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackClient{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&ospdirectorv1beta1.OpenStackIPSet{}).
		Complete(r)
}

func (r *OpenStackClientReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackClientStatus) *ospdirectorv1beta1.OpenStackClientStatus {

	//
	// set LastHeartbeatTime and LastTransitionTime to a default value as those
	// need to be ignored to compare if conditions changed.
	//
	s := status.DeepCopy()
	for idx := range s.Conditions {
		s.Conditions[idx].LastHeartbeatTime = metav1.Time{}
		s.Conditions[idx].LastTransitionTime = metav1.Time{}
	}

	return s
}

func (r *OpenStackClientReconciler) podCreateOrUpdate(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackClient,
	cond *shared.Condition,
	hostname string,
	envVars *map[string]common.EnvSetter,
) error {
	var terminationGracePeriodSeconds int64 = 0

	runAsUser := int64(instance.Spec.RunUID)
	runAsGroup := int64(instance.Spec.RunGID)

	//
	//   Get domain name and dns servers from osNetCfg
	//
	osNetCfg, err := ospdirectorv1beta1.GetOsNetCfg(r.GetClient(), instance.GetNamespace(), instance.GetLabels()[shared.OpenStackNetConfigReconcileLabel])
	if err != nil {
		cond.Type = shared.CommonCondTypeError
		cond.Reason = shared.NetConfigCondReasonError
		cond.Message = fmt.Sprintf("error getting OpenStackNetConfig %s: %s",
			instance.GetLabels()[shared.OpenStackNetConfigReconcileLabel],
			err)

		return err
	}

	// Get volumes
	initVolumeMounts := openstackclient.GetInitVolumeMounts(instance)
	volumeMounts := openstackclient.GetVolumeMounts(instance)
	volumes := openstackclient.GetVolumes(instance)

	// Get env vars
	(*envVars)["KOLLA_CONFIG_STRATEGY"] = common.EnvValue("COPY_ALWAYS")

	if instance.Spec.CloudName != "" {
		(*envVars)["OS_CLOUD"] = common.EnvValue(instance.Spec.CloudName)
	}

	(*envVars)["FQDN"] = common.EnvValue(instance.Name + "." + osNetCfg.Spec.DomainName)

	initEnvVars := make(map[string]common.EnvSetter)
	for k, v := range *envVars {
		initEnvVars[k] = v
	}
	if instance.Spec.IdmSecret != "" {
		initEnvVars["IPA_SERVER"] = func(env *corev1.EnvVar) {
			env.Value = ""
			env.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.IdmSecret,
					},
					Key: "IdMServer",
				},
			}
		}
		initEnvVars["IPA_SERVER_USER"] = func(env *corev1.EnvVar) {
			env.Value = ""
			env.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.IdmSecret,
					},
					Key: "IdMAdminUser",
				},
			}
		}
		initEnvVars["IPA_SERVER_PASSWORD"] = func(env *corev1.EnvVar) {
			env.Value = ""
			env.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.IdmSecret,
					},
					Key: "IdMAdminPassword",
				},
			}
		}
		initEnvVars["IPA_REALM"] = func(env *corev1.EnvVar) {
			env.Value = ""
			env.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.IdmSecret,
					},
					Key: "IdMRealm",
				},
			}
		}
		initEnvVars["IPA_DOMAIN"] = common.EnvValue(osNetCfg.Spec.DomainName)
	}

	// create k8s.v1.cni.cncf.io/networks network annotation to attach OpenStackClient to networks set in instance.Spec.Networks
	annotations := ""
	for id, netNameLower := range instance.Spec.Networks {
		// get network with name_lower label
		labelSelector := map[string]string{
			shared.SubNetNameLabelSelector: netNameLower,
		}

		// get network with name_lower label
		network, err := ospdirectorv1beta1.GetOpenStackNetWithLabel(
			r.Client,
			instance.Namespace,
			labelSelector,
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				cond.Message = fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower)
				cond.Reason = shared.CommonCondReasonOSNetNotFound
				cond.Type = shared.CommonCondTypeWaiting
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error getting OSNet with labelSelector %v", labelSelector)
			cond.Reason = shared.CommonCondReasonOSNetError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		nad := fmt.Sprintf("%s-static", network.Name)
		annotations += fmt.Sprintf("{\"name\": \"%s\", \"namespace\": \"%s\", \"ips\": [\"%s\"]}",
			nad,
			instance.Namespace,
			instance.Status.OpenStackClientNetStatus[hostname].IPAddresses[netNameLower],
		)
		if id < len(instance.Spec.Networks)-1 {
			annotations += ", "
		}
	}
	annotation := fmt.Sprintf("[%s]", annotations)

	// Create a (mostly) empty pod spec
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Annotations: map[string]string{},
		},
	}
	shared.InitMap(&pod.Labels)
	pod.Spec = corev1.PodSpec{}
	pod.Spec.SecurityContext = &corev1.PodSecurityContext{}
	pod.Spec.DNSConfig = &corev1.PodDNSConfig{}
	pod.Spec.Containers = []corev1.Container{
		{
			Name: "openstackclient",
		},
	}
	pod.Spec.InitContainers = []corev1.Container{
		{
			Name: "init-0",
		},
	}

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, pod, func() error {
		// Note: initialize the pod structs above then only reconcile individual fields here
		// Can not add/replace new structs in the pod here as that will drop the defaults that
		// are set/added when the pod is created.

		isPodUpdate := !pod.ObjectMeta.CreationTimestamp.IsZero()

		pod.ObjectMeta.Annotations["k8s.v1.cni.cncf.io/networks"] = annotation
		for k, v := range common.GetLabels(instance, openstackclient.AppLabel, map[string]string{}) {
			pod.Labels[k] = v
		}

		pod.Spec.SecurityContext.RunAsUser = &runAsUser
		pod.Spec.SecurityContext.RunAsGroup = &runAsGroup
		pod.Spec.SecurityContext.FSGroup = &runAsGroup
		pod.Spec.ServiceAccountName = openstackclient.ServiceAccount
		pod.Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds
		pod.Spec.Volumes = common.MergeVolumes(pod.Spec.Volumes, volumes)
		pod.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
		pod.Spec.Containers[0].Env = common.MergeEnvs(pod.Spec.Containers[0].Env, *envVars)
		pod.Spec.Containers[0].VolumeMounts = common.MergeVolumeMounts(pod.Spec.Containers[0].VolumeMounts, volumeMounts)
		pod.Spec.InitContainers[0].ImagePullPolicy = corev1.PullAlways
		pod.Spec.InitContainers[0].Env = common.MergeEnvs(pod.Spec.InitContainers[0].Env, initEnvVars)
		imageUpdate := false
		if pod.Spec.InitContainers[0].Image != instance.Spec.ImageURL {
			pod.Spec.Containers[0].Image = instance.Spec.ImageURL
			pod.Spec.InitContainers[0].Image = instance.Spec.ImageURL
			imageUpdate = true
		}

		pod.Spec.InitContainers[0].VolumeMounts = common.MergeVolumeMounts(pod.Spec.InitContainers[0].VolumeMounts, initVolumeMounts)

		if len(osNetCfg.Spec.DNSServers) != 0 {
			pod.Spec.DNSPolicy = corev1.DNSNone
			pod.Spec.DNSConfig.Nameservers = osNetCfg.Spec.DNSServers
		} else {
			pod.Spec.DNSPolicy = corev1.DNSClusterFirst
			pod.Spec.DNSConfig.Nameservers = []string{}
		}
		if len(osNetCfg.Spec.DNSSearchDomains) != 0 {
			pod.Spec.DNSConfig.Searches = osNetCfg.Spec.DNSSearchDomains
		} else {
			pod.Spec.DNSConfig.Searches = []string{}
		}

		if imageUpdate && isPodUpdate {
			// init container image is mutable but does nothing so force a delete
			return &common.ForbiddenPodSpecChangeError{Field: "Spec.InitContainers[0].Image"}
		}

		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			cond.Message = fmt.Sprintf("Error set controller reference for %s", pod.Name)
			cond.Reason = shared.CommonCondReasonControllerReferenceError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		return nil
	})
	if err != nil {
		if common.IsForbiddenPodSpecChangeError(err) || k8s_errors.IsInvalid(err) {
			// Delete pod when an unsupported change was requested, like
			// e.g. additional controller VM got up. We just re-create the
			// openstackclient pod
			common.LogForObject(
				r,
				fmt.Sprintf("OpenStackClient pod deleted due to spec change %v", err),
				instance,
			)
			if err := r.Delete(ctx, pod); err != nil && !k8s_errors.IsNotFound(err) {

				// Error deleting the object
				cond.Message = fmt.Sprintf("Error deleting OpenStackClient pod %s", pod.Name)
				cond.Reason = shared.OsClientCondReasonPodDeleteError
				cond.Type = shared.CommonCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			cond.Message = fmt.Sprintf("%s %s pod deleted", instance.Kind, instance.Name)
			cond.Reason = shared.OsClientCondReasonPodDeleted
			cond.Type = shared.CommonCondTypeWaiting
			return nil
		}

		cond.Message = fmt.Sprintf("Failed to create or update pod %s ", instance.Name)
		cond.Reason = shared.OsClientCondReasonPodError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s %s %s", instance.Kind, instance.Name, op)
		cond.Reason = shared.OsClientCondReasonPodProvisioned
		cond.Type = shared.CommonCondTypeProvisioned
		return nil
	}

	cond.Message = fmt.Sprintf("%s %s provisioned", instance.Kind, instance.Name)
	cond.Reason = shared.OsClientCondReasonPodProvisioned
	cond.Type = shared.CommonCondTypeProvisioned

	return nil
}

// NetworkAttachmentDefinition, SriovNetwork and SriovNetworkNodePolicy
func (r *OpenStackClientReconciler) verifyNetworkAttachments(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackClient,
	cond *shared.Condition,
) (ctrl.Result, error) {
	// verify that NetworkAttachmentDefinition for each configured network exist
	nadMap, err := common.GetAllNetworkAttachmentDefinitions(ctx, r, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, netNameLower := range instance.Spec.Networks {
		timeout := 10

		// get network with name_lower label
		network, err := ospdirectorv1beta1.GetOpenStackNetWithLabel(
			r.Client,
			instance.Namespace,
			map[string]string{
				shared.SubNetNameLabelSelector: netNameLower,
			},
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				cond.Message = fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower)
				cond.Reason = shared.CommonCondReasonOSNetNotFound
				cond.Type = shared.CommonCondTypeWaiting

				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error reading OpenStackNet with NameLower %s not found!", netNameLower)
			cond.Reason = shared.CommonCondReasonOSNetError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}

		if _, ok := nadMap[network.Name]; !ok {
			cond.Message = fmt.Sprintf("NetworkAttachmentDefinition %s does not yet exist.  Reconciling again in %d seconds", network.Name, timeout)
			cond.Reason = shared.CommonCondReasonOSNetWaiting
			cond.Type = shared.CommonCondTypeWaiting

			return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
		}
	}

	cond.Message = "All NetworkAttachmentDefinitions available"
	cond.Reason = shared.CommonCondReasonOSNetAvailable
	cond.Type = shared.CommonCondTypeProvisioned

	return ctrl.Result{}, nil
}

// PVCs
func (r *OpenStackClientReconciler) createPVCs(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackClient,
	cond *shared.Condition,
	labels map[string]string,
) error {
	// volume to presistent store /etc/hosts where entries get added by tripleo deploy
	pvcDetails := common.Pvc{
		Name:         fmt.Sprintf("%s-hosts", instance.Name),
		Namespace:    instance.Namespace,
		Size:         openstackclient.HostsPersistentStorageSize,
		Labels:       labels,
		StorageClass: instance.Spec.StorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}

	pvc, op, err := common.CreateOrUpdatePvc(ctx, r, instance, &pvcDetails)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update pvc %s ", pvc.Name)
		cond.Reason = shared.OsClientCondReasonPVCError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}
	if op != controllerutil.OperationResultNone {
		common.LogForObject(
			r,
			fmt.Sprintf("%s %s /etc/hosts PVC %s - operation: %s", instance.Kind, instance.Name, pvc.Name, string(op)),
			instance,
		)
	}

	pvcDetails = common.Pvc{
		Name:         fmt.Sprintf("%s-cloud-admin", instance.Name),
		Namespace:    instance.Namespace,
		Size:         openstackclient.CloudAdminPersistentStorageSize,
		Labels:       labels,
		StorageClass: instance.Spec.StorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}

	pvc, op, err = common.CreateOrUpdatePvc(ctx, r, instance, &pvcDetails)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update pvc %s ", pvc.Name)
		cond.Reason = shared.OsClientCondReasonPVCError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}
	if op != controllerutil.OperationResultNone {
		common.LogForObject(
			r,
			fmt.Sprintf("%s %s  /home/cloud-admin PVC %s - operation: %s", instance.Kind, instance.Name, pvc.Name, string(op)),
			instance,
		)
	}

	pvcDetails = common.Pvc{
		Name:         fmt.Sprintf("%s-kolla-src", instance.Name),
		Namespace:    instance.Namespace,
		Size:         openstackclient.KollaSrcPersistentStorageSize,
		Labels:       labels,
		StorageClass: instance.Spec.StorageClass,
		AccessMode: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}

	pvc, op, err = common.CreateOrUpdatePvc(ctx, r, instance, &pvcDetails)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update pvc %s ", pvc.Name)
		cond.Reason = shared.OsClientCondReasonPVCError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}
	if op != controllerutil.OperationResultNone {
		common.LogForObject(
			r,
			fmt.Sprintf("%s %s kolla-src PVC %s - operation: %s", instance.Kind, instance.Name, pvc.Name, string(op)),
			instance,
		)
	}

	cond.Message = fmt.Sprintf("All PVCs for %s %s available", instance.Kind, instance.Name)
	cond.Reason = shared.OsClientCondReasonPVCProvisioned
	cond.Type = shared.CommonCondTypeProvisioned

	return nil
}
