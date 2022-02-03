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
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	openstacknetconfig "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetconfig"
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

	defer func(cond *ospdirectorv1beta1.Condition) {
		//
		// Update object conditions
		//
		instance.Status.Conditions.UpdateCurrentCondition(
			cond.Type,
			cond.Reason,
			cond.Message,
		)

		if statusChanged() {
			if updateErr := r.Client.Status().Update(context.Background(), instance); updateErr != nil {
				common.LogErrorForObject(r, updateErr, "Update status", instance)
			}
		}

		// log current status message to operator log
		common.LogForObject(r, cond.Message, instance)
	}(cond)

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := common.OpenStackBackupOverridesReconcile(r.Client, instance)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackClient %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	//
	// add osnetcfg CR label reference which is used in the in the osnetcfg
	// controller to watch this resource and reconcile
	//
	var ctrlResult ctrl.Result
	ctrlResult, err = openstacknetconfig.AddOSNetConfigRefLabel(r, instance, cond, instance.Spec.Networks[0])
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// add labels of all networks used by this CR
	//
	err = openstacknet.AddOSNetNameLowerLabels(r, instance, cond, instance.Spec.Networks)
	if err != nil {
		return ctrl.Result{}, err
	}

	envVars := make(map[string]common.EnvSetter)
	labels := common.GetLabels(instance, openstackclient.AppLabel, map[string]string{})

	//
	// check for DeploymentSSHSecret is there
	//
	if instance.Spec.DeploymentSSHSecret != "" {
		_, ctrlResult, err = common.GetDataFromSecret(
			r,
			instance,
			cond,
			ospdirectorv1beta1.ConditionDetails{
				ConditionNotFoundType:   ospdirectorv1beta1.CommonCondTypeWaiting,
				ConditionNotFoundReason: ospdirectorv1beta1.CommonCondReasonDeploymentSecretMissing,
				ConditionErrorType:      ospdirectorv1beta1.CommonCondTypeError,
				ConditionErrordReason:   ospdirectorv1beta1.CommonCondReasonDeploymentSecretError,
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
			r,
			instance,
			cond,
			ospdirectorv1beta1.ConditionDetails{
				ConditionNotFoundType:   ospdirectorv1beta1.CommonCondTypeWaiting,
				ConditionNotFoundReason: ospdirectorv1beta1.CommonCondReasonCAConfigMapMissing,
				ConditionErrorType:      ospdirectorv1beta1.CommonCondTypeError,
				ConditionErrordReason:   ospdirectorv1beta1.CommonCondReasonCAConfigMapError,
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
			r,
			instance,
			cond,
			ospdirectorv1beta1.ConditionDetails{
				ConditionNotFoundType:   ospdirectorv1beta1.CommonCondTypeWaiting,
				ConditionNotFoundReason: ospdirectorv1beta1.CommonCondReasonIdmSecretMissing,
				ConditionErrorType:      ospdirectorv1beta1.CommonCondTypeError,
				ConditionErrordReason:   ospdirectorv1beta1.CommonCondReasonIdmSecretError,
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
	// create hostnames for the requested number of systems
	//
	_, err = r.createNewHostnames(
		instance,
		cond,
		// right now there can only be one
		1-len(instance.Status.OpenStackClientNetStatus),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// right now there can only be one
	hostnameValue := reflect.ValueOf(instance.GetHostnames()).MapKeys()[0]
	hostname := hostnameValue.String()

	//
	// get OSNetCfg object
	//
	osnetcfg := &ospdirectorv1beta1.OpenStackNetConfig{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      strings.ToLower(instance.Labels[openstacknetconfig.OpenStackNetConfigReconcileLabel]),
		Namespace: instance.Namespace},
		osnetcfg)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get %s %s ", osnetcfg.Kind, osnetcfg.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	//
	// Wait for IPs created on all configured networks
	//
	for hostname, hostStatus := range instance.Status.OpenStackClientNetStatus {
		err = openstacknetconfig.WaitOnIPsCreated(
			r,
			instance,
			cond,
			osnetcfg,
			instance.Spec.Networks,
			hostname,
			&hostStatus,
		)
		if err != nil {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		hostStatus.HostRef = hostname
		instance.Status.OpenStackClientNetStatus[hostname] = hostStatus
	}

	// Log network status if changed
	if !reflect.DeepEqual(currentStatus.OpenStackClientNetStatus, instance.Status.OpenStackClientNetStatus) {
		common.LogForObject(
			r,
			fmt.Sprintf("%s network status for Hostname: %s - %s",
				instance.Kind,
				instance.Status.OpenStackClientNetStatus[hostname].Hostname,
				diff.ObjectReflectDiff(
					currentStatus.OpenStackClientNetStatus,
					instance.Status.OpenStackClientNetStatus,
				),
			),
			instance,
		)
	}

	//
	// NetworkAttachmentDefinition
	//
	ctrlResult, err = r.verifyNetworkAttachments(
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
	err = common.EnsureConfigMaps(r, instance, cms, &envVars)
	if err != nil && k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	//
	// PVCs
	//
	err = r.createPVCs(
		instance,
		cond,
		labels,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	//
	// Create or update the pod object
	//
	err = r.podCreateOrUpdate(
		instance,
		cond,
		hostname,
		&envVars,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	hostStatus := instance.Status.OpenStackClientNetStatus[hostname]
	hostStatus.ProvisioningState = ospdirectorv1beta1.ProvisioningState(cond.Type)
	instance.Status.OpenStackClientNetStatus[hostname] = hostStatus

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
	instance *ospdirectorv1beta1.OpenStackClient,
	cond *ospdirectorv1beta1.Condition,
	hostname string,
	envVars *map[string]common.EnvSetter,
) error {
	var terminationGracePeriodSeconds int64 = 0

	runAsUser := int64(instance.Spec.RunUID)
	runAsGroup := int64(instance.Spec.RunGID)

	// Get volumes
	initVolumeMounts := openstackclient.GetInitVolumeMounts(instance)
	volumeMounts := openstackclient.GetVolumeMounts(instance)
	volumes := openstackclient.GetVolumes(instance)

	(*envVars)["KOLLA_CONFIG_STRATEGY"] = common.EnvValue("COPY_ALWAYS")

	if instance.Spec.CloudName != "" {
		(*envVars)["OS_CLOUD"] = common.EnvValue(instance.Spec.CloudName)
	}

	if instance.Spec.DomainName != "" {
		(*envVars)["FQDN"] = common.EnvValue(instance.Name + "." + instance.Spec.DomainName)

	}

	// create k8s.v1.cni.cncf.io/networks network annotation to attach OpenStackClient to networks set in instance.Spec.Networks
	annotations := ""
	for id, netNameLower := range instance.Spec.Networks {
		// get network with name_lower label
		labelSelector := map[string]string{
			openstacknet.SubNetNameLabelSelector: netNameLower,
		}

		// get network with name_lower label
		network, err := openstacknet.GetOpenStackNetWithLabel(
			r,
			instance.Namespace,
			labelSelector,
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				cond.Message = fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetNotFound)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error getting OSNet with labelSelector %v", labelSelector)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
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

	env := common.MergeEnvs([]corev1.EnvVar{}, *envVars)

	if instance.Spec.GitSecret != "" {
		env = common.MergeEnvs([]corev1.EnvVar{
			{
				Name: "GIT_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.GitSecret,
						},
						Key: "git_url",
					},
				},
			},
		}, *envVars)
	}

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
				Env:             env,
				VolumeMounts:    volumeMounts,
			},
		},
	}

	if len(instance.Spec.DNSServers) != 0 {
		pod.Spec.DNSPolicy = corev1.DNSNone
		pod.Spec.DNSConfig = &corev1.PodDNSConfig{
			Nameservers: instance.Spec.DNSServers,
		}
		if len(instance.Spec.DNSSearchDomains) != 0 {
			pod.Spec.DNSConfig.Searches = instance.Spec.DNSSearchDomains
		}
	}

	initEnv := env
	if instance.Spec.IdmSecret != "" {
		idmEnv := []corev1.EnvVar{
			{
				Name: "IPA_SERVER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.IdmSecret,
						},
						Key: "IdMServer",
					},
				},
			},
			{
				Name: "IPA_SERVER_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.IdmSecret,
						},
						Key: "IdMAdminUser",
					},
				},
			},
			{
				Name: "IPA_SERVER_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.IdmSecret,
						},
						Key: "IdMAdminPassword",
					},
				},
			},
			{
				Name: "IPA_REALM",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.IdmSecret,
						},
						Key: "IdMRealm",
					},
				},
			},
			{
				Name:  "IPA_DOMAIN",
				Value: instance.Spec.DomainName,
			},
		}
		initEnv = append(initEnv, idmEnv...)
	}

	initContainerDetails := []openstackclient.InitContainer{
		{
			ContainerImage: instance.Spec.ImageURL,
			Env:            initEnv,
			Privileged:     false,
			VolumeMounts:   initVolumeMounts,
		},
	}

	pod.Spec.InitContainers = openstackclient.GetInitContainers(initContainerDetails)

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, pod, func() error {
		pod.Spec.Containers[0].Env = common.MergeEnvs(pod.Spec.Containers[0].Env, *envVars)

		// labels
		common.InitMap(&pod.Labels)
		for k, v := range common.GetLabels(instance, openstackclient.AppLabel, map[string]string{}) {
			pod.Labels[k] = v
		}

		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			cond.Message = fmt.Sprintf("Error set controller reference for %s", pod.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonControllerReferenceError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		return nil
	})
	if err != nil {
		if k8s_errors.IsInvalid(err) {
			// Delete pod when an unsupported change was requested, like
			// e.g. additional controller VM got up. We just re-create the
			// openstackclient pod
			common.LogForObject(
				r,
				fmt.Sprintf("OpenStackClient pod deleted due to spec change %v", err),
				instance,
			)
			if err := r.Client.Delete(context.TODO(), pod); err != nil && !k8s_errors.IsNotFound(err) {

				// Error deleting the object
				cond.Message = fmt.Sprintf("Error deleting OpenStackClient pod %s", pod.Name)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPodDeleteError)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			r.Log.Info("openstackclient pod deleted due to spec change")
			return nil
		}

		cond.Message = fmt.Sprintf("Failed to create or update pod %s ", instance.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPodError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s %s %s", instance.Kind, instance.Name, op)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPodProvisioned)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)
	}

	cond.Message = fmt.Sprintf("%s %s provisioned", instance.Kind, instance.Name)
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPodProvisioned)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)

	return nil
}

//
// NetworkAttachmentDefinition, SriovNetwork and SriovNetworkNodePolicy
//
func (r *OpenStackClientReconciler) verifyNetworkAttachments(
	instance *ospdirectorv1beta1.OpenStackClient,
	cond *ospdirectorv1beta1.Condition,
) (ctrl.Result, error) {
	// verify that NetworkAttachmentDefinition for each configured network exist
	nadMap, err := common.GetAllNetworkAttachmentDefinitions(r, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, netNameLower := range instance.Spec.Networks {
		timeout := 10

		// get network with name_lower label
		network, err := openstacknet.GetOpenStackNetWithLabel(
			r,
			instance.Namespace,
			map[string]string{
				openstacknet.SubNetNameLabelSelector: netNameLower,
			},
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				cond.Message = fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetNotFound)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)

				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error reading OpenStackNet with NameLower %s not found!", netNameLower)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}

		if _, ok := nadMap[network.Name]; !ok {
			cond.Message = fmt.Sprintf("NetworkAttachmentDefinition %s does not yet exist.  Reconciling again in %d seconds", network.Name, timeout)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetWaiting)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)

			return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
		}
	}

	cond.Message = "All NetworkAttachmentDefinitions available"
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetAvailable)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)

	return ctrl.Result{}, nil
}

//
// create hostnames for the requested number of systems
//
func (r *OpenStackClientReconciler) createNewHostnames(
	instance *ospdirectorv1beta1.OpenStackClient,
	cond *ospdirectorv1beta1.Condition,
	newCount int,
) ([]string, error) {
	newHostnames := []string{}

	//
	//   create hostnames for the newCount
	//
	currentNetStatus := instance.Status.DeepCopy().OpenStackClientNetStatus
	for i := 0; i < newCount; i++ {
		hostnameDetails := common.Hostname{
			Basename: instance.Name,
			VIP:      false,
		}

		err := common.CreateOrGetHostname(instance, &hostnameDetails)
		if err != nil {
			cond.Message = fmt.Sprintf("error creating new hostname %v", hostnameDetails)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonNewHostnameError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return newHostnames, err
		}

		if instance.Status.OpenStackClientNetStatus == nil {
			instance.Status.OpenStackClientNetStatus = map[string]ospdirectorv1beta1.HostStatus{}
		}

		if hostnameDetails.Hostname != "" {
			if _, ok := instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname]; !ok {
				instance.Status.OpenStackClientNetStatus[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
					Hostname:             hostnameDetails.Hostname,
					HostRef:              hostnameDetails.HostRef,
					AnnotatedForDeletion: false,
					IPAddresses:          map[string]string{},
				}
				newHostnames = append(newHostnames, hostnameDetails.Hostname)
			}

			common.LogForObject(
				r,
				fmt.Sprintf("%s hostname created: %s", instance.Kind, hostnameDetails.Hostname),
				instance,
			)
		}
	}

	if !reflect.DeepEqual(currentNetStatus, instance.Status.OpenStackClientNetStatus) {
		common.LogForObject(
			r,
			fmt.Sprintf("Updating CR status with new hostname information, %d new - %s",
				len(newHostnames),
				diff.ObjectReflectDiff(currentNetStatus, instance.Status.OpenStackClientNetStatus),
			),
			instance,
		)

		err := r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			cond.Message = "Failed to update CR status for new hostnames"
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonCRStatusUpdateError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return newHostnames, err
		}
	}

	return newHostnames, nil
}

//
// PVCs
//
func (r *OpenStackClientReconciler) createPVCs(
	instance *ospdirectorv1beta1.OpenStackClient,
	cond *ospdirectorv1beta1.Condition,
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

	pvc, op, err := common.CreateOrUpdatePvc(r, instance, &pvcDetails)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update pvc %s ", pvc.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPVCError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
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

	pvc, op, err = common.CreateOrUpdatePvc(r, instance, &pvcDetails)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update pvc %s ", pvc.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPVCError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
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

	pvc, op, err = common.CreateOrUpdatePvc(r, instance, &pvcDetails)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update pvc %s ", pvc.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPVCError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
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
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPVCProvisioned)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)

	return nil
}
