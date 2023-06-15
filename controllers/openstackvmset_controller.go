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
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	ospdirectorv1beta2 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta2"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackipset"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	// cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	//	virtctl "kubevirt.io/kubevirt/pkg/virtctl/vm"
)

// OpenStackVMSetReconciler reconciles a VMSet object
type OpenStackVMSetReconciler struct {
	client.Client
	Kclient        kubernetes.Interface
	Log            logr.Logger
	Scheme         *runtime.Scheme
	KubevirtClient kubecli.KubevirtClient
}

// GetClient -
func (r *OpenStackVMSetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackVMSetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetVirtClient -
func (r *OpenStackVMSetReconciler) GetVirtClient() kubecli.KubevirtClient {
	return r.KubevirtClient
}

// GetLogger -
func (r *OpenStackVMSetReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackVMSetReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,namespace=openstack,resources=deployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=template.openshift.io,namespace=openstack,resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:groups=core,resources=pods;persistentvolumeclaims;events;configmaps;secrets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=core,namespace=openstack,resources=serviceaccounts,verbs=get
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=cdi.kubevirt.io,namespace=openstack,resources=datavolumes,verbs=create;delete;get;list;patch;update;watch
// FIXME: Cluster-scope required below for now, as the operator watches openshift-machine-api namespace as well
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list
// +kubebuilder:rbac:groups=kubevirt.io,namespace=openstack,resources=virtualmachines,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=subresources.kubevirt.io,namespace=openstack,resources=virtualmachines/start,verbs=update
// FIXME: Is there a way to scope the following RBAC annotation to just the "openshift-machine-api" namespace?
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=list;watch
// +kubebuilder:rbac:groups=nmstate.io,resources=nodenetworkconfigurationpolicies,verbs=get;list
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets,verbs=get;list
// FIXME: Is there a way to scope the following RBAC annotation to just the "openshift-sriov-network-operator" namespace?
// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworknodepolicies;sriovnetworks,verbs=get;list;watch;create;update;patch;delete

// Reconcile - controller VMs
func (r *OpenStackVMSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("vmset", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta2.OpenStackVMSet{}
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
	// verify API version
	//
	if instance.Spec.RootDisk.DiskSize == 0 {
		return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second},
			fmt.Errorf(fmt.Sprintf("waiting for runtime object %s %s to be migrated to new API version",
				instance.Kind,
				instance.Name,
			))
	}

	//
	// initialize condition
	//
	cond := instance.Status.Conditions.InitCondition()

	// If VmSet status map is nil, create it
	if instance.Status.VMHosts == nil {
		instance.Status.VMHosts = map[string]ospdirectorv1beta2.HostStatus{}
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

		instance.Status.ProvisioningStatus.Reason = cond.Message
		instance.Status.ProvisioningStatus.State = shared.ProvisioningState(cond.Type)

		if statusChanged() {
			if updateErr := r.Status().Update(context.Background(), instance); updateErr != nil {
				common.LogErrorForObject(r, updateErr, "Update status", instance)
			}
		}

		// log current status message to operator log
		common.LogForObject(r, cond.Message, instance)
	}(cond)

	// What VMs do we currently have for this OpenStackVMSet?
	virtualMachineList, err := common.GetVirtualMachines(ctx, r, instance.Namespace, map[string]string{
		common.OwnerNameLabelSelector: instance.Name,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, vmset.FinalizerName) {
			controllerutil.AddFinalizer(instance, vmset.FinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
			common.LogForObject(
				r,
				fmt.Sprintf("Finalizer %s added to CR %s", vmset.FinalizerName, instance.Name),
				instance,
			)
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, vmset.FinalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. Clean up resources used by the operator
		// VirtualMachine resources
		err := r.virtualMachineListFinalizerCleanup(ctx, instance, cond, virtualMachineList)
		if err != nil && !k8s_errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 3. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, vmset.FinalizerName)
		err = r.Update(ctx, instance)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
			cond.Reason = shared.CommonCondReasonRemoveFinalizerError
			cond.Type = shared.CommonCondTypeError

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.IsReady())

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackVMSet %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	envVars := make(map[string]common.EnvSetter)
	templateParameters := make(map[string]interface{})
	secretLabels := common.GetLabels(instance, vmset.AppLabel, map[string]string{})
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

	//
	// Generate fencing data potentially needed by all VMSets in this instance's namespace
	//
	err = r.generateNamespaceFencingData(
		ctx,
		instance,
		cond,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// check for DeploymentSSHSecret and add AuthorizedKeys from DeploymentSSHSecret
	//
	templateParameters["AuthorizedKeys"], ctrlResult, err = common.GetDataFromSecret(
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
		"authorized_keys",
	)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

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

		return ctrl.Result{}, err
	}

	//
	// add DomainName
	//
	templateParameters["DomainName"] = osNetCfg.Spec.DomainName

	if instance.Spec.GrowvolsArgs != nil && len(instance.Spec.GrowvolsArgs) > 0 {
		templateParameters["GrowvolsArgs"] = instance.Spec.GrowvolsArgs

	} else {
		// use default for the role name
		templateParameters["GrowvolsArgs"] = common.GetRoleGrowvolsArgs(instance.Spec.RoleName)
	}

	//
	// check if PasswordSecret got specified and if it exists before creating the controlplane
	//
	if instance.Spec.PasswordSecret != "" {
		templateParameters["NodeRootPassword"], ctrlResult, err = r.getPasswordSecret(
			ctx,
			instance,
			cond,
		)
		if (err != nil) || (ctrlResult != ctrl.Result{}) {
			return ctrlResult, err
		}
	}

	//
	// add IsTripleoRole
	//
	templateParameters["IsTripleoRole"] = instance.Spec.IsTripleoRole

	//
	// Create CloudInitSecret secret for the vmset
	//
	err = r.createCloudInitSecret(
		ctx,
		instance,
		cond,
		envVars,
		secretLabels,
		templateParameters,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// Get mapping of all OSNets with their binding type for this namespace
	//
	osNetBindings, err := openstacknet.GetOpenStackNetsBindingMap(ctx, r, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// NetworkAttachmentDefinition, SriovNetwork and SriovNetworkNodePolicy
	//
	nadMap, ctrlResult, err := r.verifyNetworkAttachments(
		ctx,
		instance,
		cond,
		osNetBindings,
	)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	//   check/update instance status for annotated for deletion marked VMs
	//
	err = r.checkVMsAnnotatedForDeletion(
		ctx,
		instance,
		cond,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	//   Handle VM removal from VMSet
	//
	deletedHosts, err := r.doVMDelete(
		ctx,
		instance,
		cond,
		virtualMachineList,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// create IPs for all networks
	//
	ipsetStatus, ctrlResult, err := openstackipset.EnsureIPs(
		ctx,
		r,
		instance,
		cond,
		instance.Spec.RoleName,
		instance.Spec.Networks,
		instance.Spec.VMCount,
		false,
		false,
		deletedHosts,
		instance.Spec.IsTripleoRole,
	)

	for _, hostname := range deletedHosts {
		delete(instance.Status.VMHosts, hostname)
	}

	for _, status := range ipsetStatus {
		hostStatus := ospdirectorv1beta2.SyncIPsetStatus(cond, instance.Status.VMHosts, status)
		instance.Status.VMHosts[status.Hostname] = hostStatus
	}

	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	//   Create BaseImage for the VMSet
	//
	baseImageName, ctrlResult, err := r.createBaseImage(
		ctx,
		instance,
		cond,
	)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	//   Create/Update NetworkData
	//
	vmDetails, err := r.createNetworkData(
		ctx,
		instance,
		cond,
		osNetCfg,
		nadMap,
		envVars,
		templateParameters,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	//   Create the VM objects
	//
	ctrlResult, err = r.createVMs(
		ctx,
		instance,
		cond,
		envVars,
		osNetBindings,
		baseImageName,
		vmDetails,
	)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackVMSetReconciler) getNormalizedStatus(status *ospdirectorv1beta2.OpenStackVMSetStatus) *ospdirectorv1beta2.OpenStackVMSetStatus {

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

func (r *OpenStackVMSetReconciler) generateNamespaceFencingData(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
) error {
	// Ensure that a namespace-scoped kubevirt fencing agent service account token secret has been
	// created for any OSVMSet instances in this instance's namespace (all OSVMSets in this namespace
	// will share this same secret)

	labels := map[string]string{
		common.OwnerNameSpaceLabelSelector:      instance.Namespace,
		common.OwnerControllerNameLabelSelector: vmset.AppLabel,
	}

	saSecretTemplate := []common.Template{
		{
			Name:               vmset.KubevirtFencingServiceAccountSecret,
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeCustom,
			SecretType:         corev1.SecretTypeServiceAccountToken,
			InstanceType:       instance.Kind,
			AdditionalTemplate: nil,
			Annotations: map[string]string{
				common.ServiceAccountAnnotationName: vmset.KubevirtFencingServiceAccount,
			},
			Labels:        labels,
			ConfigOptions: nil,
		},
	}

	if err := common.EnsureSecrets(ctx, r, instance, saSecretTemplate, nil); err != nil {
		cond.Message = "Error creating Kubevirt Fencing ServiceAccount Secret"
		cond.Reason = shared.VMSetCondReasonKubevirtFencingServiceAccountError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	// Use aforementioned namespace-scoped kubevirt fencing agent service account token secret to
	// generate a namespace-scoped kubevirt fencing kubeconfig (all OSVMSets in this namespace will
	// share this same secret)

	// Get the kubeconfig used by the operator to acquire the cluster's API address
	kubeconfig, err := config.GetConfig()
	if err != nil {
		cond.Message = "Error getting kubeconfig used by the operator"
		cond.Reason = shared.VMSetCondReasonKubeConfigError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	templateParameters := map[string]interface{}{}
	templateParameters["Server"] = kubeconfig.Host
	templateParameters["Namespace"] = instance.Namespace

	// Read-back the secret for the kubevirt agent service account token
	kubevirtAgentTokenSecret, err := r.Kclient.CoreV1().Secrets(instance.Namespace).Get(ctx, vmset.KubevirtFencingServiceAccountSecret, metav1.GetOptions{})
	if err != nil {
		cond.Message = "Error getting the Kubevirt Fencing ServiceAccount Secret"
		cond.Reason = shared.VMSetCondReasonKubevirtFencingServiceAccountError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	templateParameters["Token"] = string(kubevirtAgentTokenSecret.Data["token"])

	kubeconfigTemplate := []common.Template{
		{
			Name:               vmset.KubevirtFencingKubeconfigSecret,
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeNone,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{"kubeconfig": "/vmset/fencing-kubeconfig/fencing-kubeconfig"},
			Labels:             labels,
			ConfigOptions:      templateParameters,
			SkipSetOwner:       true,
		},
	}

	err = common.EnsureSecrets(ctx, r, instance, kubeconfigTemplate, nil)
	if err != nil {
		cond.Message = "Error creating secret holding the kubeconfig used by the pacemaker fencing agent"
		cond.Reason = shared.VMSetCondReasonNamespaceFencingDataError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	return nil
}

func (r *OpenStackVMSetReconciler) generateVirtualMachineNetworkData(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	osNetCfg *ospdirectorv1beta1.OpenStackNetConfig,
	envVars *map[string]common.EnvSetter,
	templateParameters map[string]interface{},
	host ospdirectorv1beta1.Host,
) error {
	templateParameters["ControllerIP"] = host.IPAddress
	templateParameters["CtlplaneInterface"] = instance.Spec.CtlplaneInterface

	if len(instance.Spec.BootstrapDNS) > 0 {
		templateParameters["CtlplaneDns"] = instance.Spec.BootstrapDNS
	} else {
		templateParameters["CtlplaneDns"] = osNetCfg.Spec.DNSServers
	}

	if len(instance.Spec.DNSSearchDomains) > 0 {
		templateParameters["CtlplaneDnsSearch"] = instance.Spec.DNSSearchDomains
	} else {
		templateParameters["CtlplaneDnsSearch"] = osNetCfg.Spec.DNSSearchDomains
	}

	labelSelector := map[string]string{
		shared.ControlPlaneNetworkLabelSelector: strconv.FormatBool(true),
	}

	ctlplaneNets, err := ospdirectorv1beta1.GetOpenStackNetsMapWithLabel(
		r.Client,
		instance.Namespace,
		labelSelector,
	)
	if err != nil {
		cond.Message = fmt.Sprintf("Error getting ctlplane OSNets with labelSelector %v", labelSelector)
		cond.Reason = shared.CommonCondReasonOSNetError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)
		return err
	}

	var netNameLower string
	var ctlPlaneNetwork ospdirectorv1beta1.OpenStackNet

outer:
	for netName, osNet := range ctlplaneNets {
		for _, myNet := range instance.Spec.Networks {
			if myNet == netName {
				netNameLower = netName
				ctlPlaneNetwork = osNet
				break outer
			}
		}
	}

	if netNameLower == "" {
		cond.Message = "Ctlplane network not found"
		cond.Reason = shared.CommonCondReasonOSNetError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)
		return err
	}

	gateway := ctlPlaneNetwork.Spec.Gateway

	if gateway != "" {
		if strings.Contains(gateway, ":") {
			templateParameters["Gateway"] = fmt.Sprintf("gateway6: %s", gateway)
		} else {
			templateParameters["Gateway"] = fmt.Sprintf("gateway4: %s", gateway)
		}
	}

	routes := []map[string]string{}
	for _, route := range ctlPlaneNetwork.Spec.Routes {
		routes = append(routes, map[string]string{"to": route.Destination, "via": route.Nexthop})
	}
	templateParameters["CtlplaneRoutes"] = routes

	networkdata := []common.Template{
		{
			Name:               host.NetworkDataSecret,
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeNone,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{"networkdata": "/vmset/cloudinit/networkdata"},
			Labels:             host.Labels,
			ConfigOptions:      templateParameters,
		},
	}

	err = common.EnsureSecrets(ctx, r, instance, networkdata, envVars)
	if err != nil {
		return err
	}

	return nil
}

func (r *OpenStackVMSetReconciler) virtualMachineListFinalizerCleanup(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	virtualMachineList *virtv1.VirtualMachineList,
) error {
	r.Log.Info(fmt.Sprintf("Removing finalizers from VirtualMachines in VMSet: %s", instance.Name))

	for _, virtualMachine := range virtualMachineList.Items {
		err := r.virtualMachineFinalizerCleanup(ctx, &virtualMachine, cond)

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *OpenStackVMSetReconciler) virtualMachineFinalizerCleanup(
	ctx context.Context,
	virtualMachine *virtv1.VirtualMachine,
	cond *shared.Condition,
) error {
	controllerutil.RemoveFinalizer(virtualMachine, vmset.FinalizerName)
	err := r.Update(ctx, virtualMachine)

	if err != nil {
		cond.Message = fmt.Sprintf("Failed to update %s %s", virtualMachine.Kind, virtualMachine.Name)
		cond.Reason = shared.CommonCondReasonRemoveFinalizerError
		cond.Type = shared.CommonCondTypeError

		err = common.WrapErrorForObject(cond.Message, virtualMachine, err)

		return err
	}

	r.Log.Info(fmt.Sprintf("VirtualMachine finalizer removed: name %s", virtualMachine.Name))

	return nil
}

// SetupWithManager -
func (r *OpenStackVMSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: Myabe use filtering functions here since some resource permissions
	// are now cluster-scoped?
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta2.OpenStackVMSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&virtv1.VirtualMachine{}).
		Owns(&ospdirectorv1beta1.OpenStackIPSet{}).
		Complete(r)
}

func (r *OpenStackVMSetReconciler) vmCreateInstance(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	envVars map[string]common.EnvSetter,
	ctl *ospdirectorv1beta1.Host,
	osNetBindings map[string]ospdirectorv1beta1.AttachType,
) error {

	evictionStrategy := virtv1.EvictionStrategyLiveMigrate
	trueValue := true
	terminationGracePeriodSeconds := int64(0)

	// VMs should only be started via the operator once right after creation.
	// after that its the responsibility if the end user, or pacemaker to manage
	// the run state. virtv1.RunStrategyOnce could be an option, but is not available
	// in kubevirt 0.49.0 api.
	// https://docs.openshift.com/container-platform/4.10/virt/virtual_machines/virt-create-vms.html
	// references https://kubevirt.io/api-reference/v0.49.0/definitions.html#_v1_virtualmachinespec
	runStrategy := virtv1.RunStrategyManual

	// get deployment userdata from secret
	userdataSecret := fmt.Sprintf("%s-cloudinit", instance.Name)
	secret, _, err := common.GetSecret(ctx, r, userdataSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			cond.Message = fmt.Sprintf("CloudInit userdata %s secret not found!!", userdataSecret)
		} else {
			cond.Message = fmt.Sprintf("Error CloudInit userdata %s secret", userdataSecret)
		}
		cond.Reason = shared.VMSetCondReasonCloudInitSecretError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	labels := common.GetLabels(instance, vmset.AppLabel, map[string]string{
		common.OSPHostnameLabelSelector: ctl.Hostname,
		"kubevirt.io/vm":                ctl.DomainName,
	})

	var vm *virtv1.VirtualMachine
	var vmTemplate *virtv1.VirtualMachineInstanceTemplateSpec
	if vm, err = r.KubevirtClient.VirtualMachine(instance.Namespace).Get(ctl.DomainName, &metav1.GetOptions{}); err != nil {
		// of not found, prepare the VirtualMachineInstanceTemplateSpec
		if k8s_errors.IsNotFound(err) {
			vmTemplate = &virtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ctl.DomainName,
					Namespace: instance.Namespace,
					Labels:    labels,
				},
				Spec: virtv1.VirtualMachineInstanceSpec{
					Hostname:                      ctl.DomainName,
					EvictionStrategy:              &evictionStrategy,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Domain: virtv1.DomainSpec{
						Devices: virtv1.Devices{
							Disks: []virtv1.Disk{},
							Interfaces: []virtv1.Interface{
								{
									Name:  "default",
									Model: "virtio",
									InterfaceBindingMethod: virtv1.InterfaceBindingMethod{
										Masquerade: &virtv1.InterfaceMasquerade{},
									},
								},
							},
							NetworkInterfaceMultiQueue: &trueValue,
							Rng:                        &virtv1.Rng{},
						},
						Machine: &virtv1.Machine{
							Type: "",
						},
						Features: &virtv1.Features{
							SMM: &virtv1.FeatureState{Enabled: &trueValue},
						},
						Firmware: &virtv1.Firmware{
							Bootloader: &virtv1.Bootloader{EFI: &virtv1.EFI{}},
						},
					},
					Volumes: []virtv1.Volume{},
					Networks: []virtv1.Network{
						{
							Name: "default",
							NetworkSource: virtv1.NetworkSource{
								Pod: &virtv1.PodNetwork{},
							},
						},
					},
				},
			}

			// VM
			vm = &virtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ctl.DomainName,
					Namespace: instance.Namespace,
				},
				Spec: virtv1.VirtualMachineSpec{
					Template: vmTemplate,
				},
			}

		} else {
			// Error reading the object - requeue the request.
			err = common.WrapErrorForObject("Get VirtualMachineInstanceTemplateSpec", vm, err)
			return err
		}
		// VM
		vm = &virtv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ctl.DomainName,
				Namespace: instance.Namespace,
				Labels:    common.GetLabels(instance, vmset.AppLabel, map[string]string{}),
			},
			Spec: virtv1.VirtualMachineSpec{
				Template: vmTemplate,
			},
		}
	}

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, vm, func() error {
		vm.Labels = shared.MergeStringMaps(
			vm.GetLabels(),
			common.GetLabels(instance, vmset.AppLabel, map[string]string{}),
		)
		vm.Spec.RunStrategy = &runStrategy
		vm.Spec.Template.Spec.NodeSelector = shared.MergeStringMaps(
			vm.Spec.Template.Spec.NodeSelector,
			instance.Spec.NodeSelector,
		)

		// If possible two VMs of the same roleshould not
		// run on the same worker node. This still allows to
		// manually migrate instances and they run on the same node.
		// On the next migration action they get again distributed.
		vm.Spec.Template.Spec.Affinity = common.DistributePods(
			common.OwnerNameLabelSelector,
			[]string{
				instance.Name,
			},
			corev1.LabelHostname,
		)

		if len(instance.Spec.BootstrapDNS) != 0 {
			vm.Spec.Template.Spec.DNSPolicy = corev1.DNSNone
			vm.Spec.Template.Spec.DNSConfig = &corev1.PodDNSConfig{
				Nameservers: instance.Spec.BootstrapDNS,
			}
			if len(instance.Spec.DNSSearchDomains) != 0 {
				vm.Spec.Template.Spec.DNSConfig.Searches = instance.Spec.DNSSearchDomains
			}
		}

		vm.Spec.Template.Spec.Domain.CPU = &virtv1.CPU{
			Cores: instance.Spec.Cores,
		}
		vm.Spec.Template.Spec.Domain.Resources = virtv1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", instance.Spec.Memory)),
			},
		}

		//
		// merge additional networks
		//
		networks := instance.Spec.Networks
		// sort networks to get an expected ordering for easier ooo nic template creation
		sort.Strings(networks)
		for _, netNameLower := range networks {

			// get network with name_lower label
			labelSelector := map[string]string{
				shared.SubNetNameLabelSelector: netNameLower,
			}
			network, err := ospdirectorv1beta1.GetOpenStackNetWithLabel(
				r.Client,
				instance.Namespace,
				labelSelector,
			)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					common.LogForObject(
						r,
						fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower),
						instance,
					)
					continue
				}
				// Error reading the object - requeue the request.
				cond.Message = fmt.Sprintf("Error getting OSNet with labelSelector %v", labelSelector)
				cond.Reason = shared.CommonCondReasonOSNetError
				cond.Type = shared.CommonCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			if _, ok := osNetBindings[netNameLower]; !ok {
				cond.Message = fmt.Sprintf("OpenStackVMSet vmCreateInstance: No binding type found %s - available bindings: %v",
					network.Name,
					osNetBindings)
				cond.Reason = shared.CommonCondReasonOSNetError
				cond.Type = shared.CommonCondTypeError

				return fmt.Errorf(cond.Message)
			}

			vm.Spec.Template.Spec.Domain.Devices.Interfaces = vmset.MergeVMInterfaces(
				vm.Spec.Template.Spec.Domain.Devices.Interfaces,
				vmset.InterfaceSetterMap{
					network.Name: vmset.Interface(network.Name, osNetBindings[netNameLower]),
				},
			)

			vm.Spec.Template.Spec.Networks = vmset.MergeVMNetworks(
				vm.Spec.Template.Spec.Networks,
				vmset.NetSetterMap{
					network.Name: vmset.Network(network.Name, osNetBindings[netNameLower]),
				},
			)
		}

		//
		// Disks and storage
		//
		if instance.Spec.BlockMultiQueue {
			vm.Spec.Template.Spec.Domain.Devices.BlockMultiQueue = &trueValue
		}
		if len(instance.Spec.IOThreadsPolicy) > 0 {
			ioThreadsPolicy := virtv1.IOThreadsPolicy(instance.Spec.IOThreadsPolicy)
			vm.Spec.Template.Spec.Domain.IOThreadsPolicy = &ioThreadsPolicy
		}

		// root, cloudinit and fencing disk
		vm.Spec.DataVolumeTemplates = vmset.MergeVMDataVolumes(
			vm.Spec.DataVolumeTemplates,
			vmset.DataVolumeSetterMap{
				ctl.DomainNameUniq: vmset.DataVolume(
					ctl.DomainNameUniq,
					instance.Namespace,
					instance.Spec.RootDisk.StorageAccessMode,
					instance.Spec.RootDisk.DiskSize,
					instance.Spec.RootDisk.StorageVolumeMode,
					instance.Spec.RootDisk.StorageClass,
					ctl.BaseImageName,
				),
			},
			instance.Namespace,
		)

		bootDevice := uint(1)
		vm.Spec.Template.Spec.Domain.Devices.Disks = vmset.MergeVMDisks(
			vm.Spec.Template.Spec.Domain.Devices.Disks,
			vmset.DiskSetterMap{
				"rootdisk": vmset.Disk(
					"rootdisk",
					virtv1.DiskBusVirtio,
					"",
					instance.Spec.RootDisk.DedicatedIOThread,
					&bootDevice,
				),
				"cloudinitdisk": vmset.Disk(
					"cloudinitdisk",
					virtv1.DiskBusVirtio,
					"",
					false,
					nil,
				),
				"fencingdisk": vmset.Disk(
					"fencingdisk",
					virtv1.DiskBusVirtio,
					"fencingdisk",
					false,
					nil,
				),
			},
		)

		vm.Spec.Template.Spec.Volumes = vmset.MergeVMVolumes(
			vm.Spec.Template.Spec.Volumes,
			vmset.VolumeSetterMap{
				"rootdisk": vmset.VolumeSourceDataVolume(
					"rootdisk",
					ctl.DomainNameUniq,
				),
				"cloudinitdisk": vmset.VolumeSourceCloudInitNoCloud(
					"cloudinitdisk",
					secret.Name,
					ctl.NetworkDataSecret,
				),
				"fencingdisk": vmset.VolumeSourceSecret(
					"fencingdisk",
					vmset.KubevirtFencingKubeconfigSecret,
				),
			},
		)

		// merge additional disks
		for _, disk := range instance.Spec.AdditionalDisks {
			name := strings.ToLower(fmt.Sprintf("%s-%s", ctl.DomainNameUniq, disk.Name))

			vm.Spec.DataVolumeTemplates = vmset.MergeVMDataVolumes(
				vm.Spec.DataVolumeTemplates,
				vmset.DataVolumeSetterMap{
					name: vmset.DataVolume(
						name,
						instance.Namespace,
						disk.StorageAccessMode,
						disk.DiskSize,
						disk.StorageVolumeMode,
						disk.StorageClass,
						"",
					),
				},
				instance.Namespace,
			)

			vm.Spec.Template.Spec.Domain.Devices.Disks = vmset.MergeVMDisks(
				vm.Spec.Template.Spec.Domain.Devices.Disks,
				vmset.DiskSetterMap{
					name: vmset.Disk(
						name,
						virtv1.DiskBusVirtio,
						"",
						disk.DedicatedIOThread,
						nil,
					),
				},
			)

			vm.Spec.Template.Spec.Volumes = vmset.MergeVMVolumes(
				vm.Spec.Template.Spec.Volumes,
				vmset.VolumeSetterMap{
					name: vmset.VolumeSourceDataVolume(
						name,
						name,
					),
				},
			)
		}

		err := controllerutil.SetControllerReference(instance, vm, r.Scheme)
		if err != nil {
			cond.Message = fmt.Sprintf("Error set controller reference for %s", vm.Name)
			cond.Reason = shared.CommonCondReasonControllerReferenceError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Error create/update %s VM %s", vm.Kind, vm.Name)
		cond.Reason = shared.VMSetCondReasonKubevirtError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)
		common.LogForObject(r, fmt.Sprintf("%s: %+v", cond.Message, vm), vm)

		return err
	}

	if op != controllerutil.OperationResultNone {
		if op == controllerutil.OperationResultCreated {
			// start VM once when it gets created
			if err := r.KubevirtClient.VirtualMachine(instance.Namespace).Start(vm.Name, &virtv1.StartOptions{}); err != nil {
				return common.WrapErrorForObject(
					fmt.Sprintf("VirtualMachine %s ERROR start after initial create: %s", vm.Name, err.Error()),
					instance,
					err,
				)
			}
		}
		common.LogForObject(
			r,
			fmt.Sprintf("VirtualMachine %s successfully reconciled - operation: %s", vm.Name, string(op)),
			instance,
		)
	}

	hostStatus := instance.Status.VMHosts[ctl.Hostname]

	if vm.Status.Ready {
		hostStatus.ProvisioningState = shared.ProvisioningState(shared.VMSetCondTypeProvisioned)
	} else if vm.Status.Created {
		hostStatus.ProvisioningState = shared.ProvisioningState(shared.VMSetCondTypeProvisioning)
	}

	instance.Status.VMHosts[ctl.Hostname] = hostStatus

	return nil
}

// check if specified password secret exists
func (r *OpenStackVMSetReconciler) getPasswordSecret(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
) (string, ctrl.Result, error) {

	passwordSecret, _, err := common.GetSecret(ctx, r, instance.Spec.PasswordSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			timeout := 30
			cond.Message = fmt.Sprintf("PasswordSecret %s not found but specified in CR, next reconcile in %d s", instance.Spec.PasswordSecret, timeout)
			cond.Reason = shared.VMSetCondReasonPasswordSecretMissing
			cond.Type = shared.CommonCondTypeWaiting

			common.LogForObject(r, cond.Message, instance)

			return "", ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
		}
		// Error reading the object - requeue the request.
		cond.Message = fmt.Sprintf("Error reading PasswordSecret: %s", instance.Spec.PasswordSecret)
		cond.Reason = shared.VMSetCondReasonPasswordSecretError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return "", ctrl.Result{}, err
	}

	if len(passwordSecret.Data["NodeRootPassword"]) == 0 {
		return "", ctrl.Result{}, k8s_errors.NewNotFound(corev1.Resource("NodeRootPassword"), "not found")
	}

	// use same NodeRootPassword paremater as tripleo have
	return string(passwordSecret.Data["NodeRootPassword"]), ctrl.Result{}, nil
}

// Create CloudInitSecret secret for the vmset
func (r *OpenStackVMSetReconciler) createCloudInitSecret(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	envVars map[string]common.EnvSetter,
	secretLabels map[string]string,
	templateParameters map[string]interface{},
) error {

	cloudinit := []common.Template{
		{
			Name:               fmt.Sprintf("%s-cloudinit", instance.Name),
			Namespace:          instance.Namespace,
			Type:               common.TemplateTypeNone,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{"userdata": "/vmset/cloudinit/userdata"},
			Labels:             secretLabels,
			ConfigOptions:      templateParameters,
		},
	}

	err := common.EnsureSecrets(ctx, r, instance, cloudinit, &envVars)
	if err != nil {
		cond.Message = fmt.Sprintf("Error creating CloudInitSecret secret for the vmset %s", instance.Name)
		cond.Reason = shared.VMSetCondReasonCloudInitSecretError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}
	return nil
}

// NetworkAttachmentDefinition, SriovNetwork and SriovNetworkNodePolicy
func (r *OpenStackVMSetReconciler) verifyNetworkAttachments(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	osNetBindings map[string]ospdirectorv1beta1.AttachType,
) (map[string]networkv1.NetworkAttachmentDefinition, ctrl.Result, error) {
	// Verify that NetworkAttachmentDefinition for each non-SRIOV-configured network exists, and...
	nadMap, err := common.GetAllNetworkAttachmentDefinitions(ctx, r, instance)
	if err != nil {
		return nadMap, ctrl.Result{}, err
	}

	// ...verify that SriovNetwork and SriovNetworkNodePolicy exists for each SRIOV-configured network
	sriovLabelSelectorMap := map[string]string{
		common.OwnerControllerNameLabelSelector: openstacknet.AppLabel,
		common.OwnerNameSpaceLabelSelector:      instance.Namespace,
	}
	snMap, err := openstacknet.GetSriovNetworksWithLabel(ctx, r, sriovLabelSelectorMap, "openshift-sriov-network-operator")
	if err != nil {
		return nadMap, ctrl.Result{}, err
	}
	snnpMap, err := openstacknet.GetSriovNetworkNodePoliciesWithLabel(ctx, r, sriovLabelSelectorMap, "openshift-sriov-network-operator")
	if err != nil {
		return nadMap, ctrl.Result{}, err
	}

	for _, netNameLower := range instance.Spec.Networks {
		timeout := 10

		cond.Type = shared.CommonCondTypeWaiting

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

				common.LogForObject(r, cond.Message, instance)

				continue
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error reading OpenStackNet with NameLower %s not found!", netNameLower)
			cond.Reason = shared.CommonCondReasonOSNetError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return nadMap, ctrl.Result{}, err
		}

		// We currently support SRIOV and bridge interfaces, with anything other than "sriov" indicating a bridge
		switch osNetBindings[network.Spec.NameLower] {
		case ospdirectorv1beta1.AttachTypeBridge:
			// Non-SRIOV networks should have a NetworkAttachmentDefinition
			if _, ok := nadMap[network.Name]; !ok {
				cond.Message = fmt.Sprintf("NetworkAttachmentDefinition %s does not yet exist.  Reconciling again in %d seconds", network.Name, timeout)
				return nadMap, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
			}
		case ospdirectorv1beta1.AttachTypeSriov:
			// SRIOV networks should have a SriovNetwork and a SriovNetworkNodePolicy
			if _, ok := snMap[fmt.Sprintf("%s-sriov-network", network.Spec.NameLower)]; !ok {
				cond.Message = fmt.Sprintf("SriovNetwork for network %s does not yet exist.  Reconciling again in %d seconds", network.Spec.NameLower, timeout)
				return nadMap, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
			} else if _, ok := snnpMap[fmt.Sprintf("%s-sriov-policy", network.Spec.NameLower)]; !ok {
				cond.Message = fmt.Sprintf("SriovNetworkNodePolicy for network %s does not yet exist.  Reconciling again in %d seconds", network.Spec.NameLower, timeout)
				return nadMap, ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
			}
		}
	}

	return nadMap, ctrl.Result{}, nil
}

// check/update instance status for annotated for deletion marked VMs
func (r *OpenStackVMSetReconciler) checkVMsAnnotatedForDeletion(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
) error {
	// check for deletion marked VMs
	currentVMHostsStatus := instance.Status.DeepCopy().VMHosts
	deletionAnnotatedVMs, err := r.getDeletedVMOSPHostnames(ctx, instance, cond)
	if err != nil {
		return err
	}

	for hostname, vmStatus := range instance.Status.VMHosts {

		if len(deletionAnnotatedVMs) > 0 && common.StringInSlice(hostname, deletionAnnotatedVMs) {
			// set annotatedForDeletion status of the VM to true, if not already
			if !vmStatus.AnnotatedForDeletion {
				vmStatus.AnnotatedForDeletion = true
				common.LogForObject(
					r,
					fmt.Sprintf("Host deletion annotation set on VM %s", hostname),
					instance,
				)
			}
		} else {
			// check if the vm was previously flagged as annotated and revert it
			if vmStatus.AnnotatedForDeletion {
				vmStatus.AnnotatedForDeletion = false
				common.LogForObject(
					r,
					fmt.Sprintf("Host deletion annotation removed on VM %s", hostname),
					instance,
				)
			}
		}
		actualStatus := instance.Status.VMHosts[hostname]
		if !reflect.DeepEqual(&actualStatus, vmStatus) {
			instance.Status.VMHosts[hostname] = vmStatus
		}
	}

	if !reflect.DeepEqual(currentVMHostsStatus, instance.Status.VMHosts) {
		common.LogForObject(
			r,
			fmt.Sprintf("Updating CR status with deletion annotation information - %s",
				diff.ObjectReflectDiff(currentVMHostsStatus, instance.Status.VMHosts)),
			instance,
		)

		err = r.Status().Update(context.Background(), instance)
		if err != nil {
			cond.Message = "Failed to update CR status for annotated for deletion marked VMs"
			cond.Reason = shared.CommonCondReasonCRStatusUpdateError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}
	}
	return nil

}

func (r *OpenStackVMSetReconciler) getDeletedVMOSPHostnames(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
) ([]string, error) {

	var annotatedVMs []string

	labelSelector := map[string]string{
		common.OwnerUIDLabelSelector:       string(instance.UID),
		common.OwnerNameSpaceLabelSelector: instance.Namespace,
		common.OwnerNameLabelSelector:      instance.Name,
	}

	vmHostList, err := common.GetVirtualMachines(ctx, r, instance.Namespace, labelSelector)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get virtual machines with labelSelector %v ", labelSelector)
		cond.Reason = shared.VMSetCondReasonVirtualMachineGetError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return annotatedVMs, err
	}

	// Get list of OSP hostnames from HostRemovalAnnotation annotated VMs
	for _, vm := range vmHostList.Items {

		if val, ok := vm.Annotations[shared.HostRemovalAnnotation]; ok &&
			(strings.ToLower(val) == "yes" || strings.ToLower(val) == "true") {
			ospHostname := vm.Spec.Template.Spec.Hostname
			annotatedVMs = append(annotatedVMs, ospHostname)

			common.LogForObject(
				r,
				fmt.Sprintf("VM %s/%s annotated for deletion", vm.Name, ospHostname),
				instance,
			)
		}
	}

	return annotatedVMs, nil
}

// Create BaseImage for the VMSet
func (r *OpenStackVMSetReconciler) createBaseImage(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
) (string, ctrl.Result, error) {
	baseImageName := fmt.Sprintf("osp-vmset-baseimage-%s", instance.UID[0:4])
	if instance.Spec.RootDisk.BaseImageVolumeName != "" {
		baseImageName = instance.Spec.RootDisk.BaseImageVolumeName
	}

	// wait for the base image conversion job to be finished before we create the VMs
	// we check the pvc for the base image:
	// - import in progress
	//   annotations -> cdi.kubevirt.io/storage.pod.phase: Running
	// - when import done
	//   annotations -> cdi.kubevirt.io/storage.pod.phase: Succeeded
	pvc := &corev1.PersistentVolumeClaim{}

	err := r.Get(ctx, types.NamespacedName{Name: baseImageName, Namespace: instance.Namespace}, pvc)
	if err != nil && k8s_errors.IsNotFound(err) {
		cond.Message = fmt.Sprintf("PersistentVolumeClaim %s not found reconcile again in 10 seconds", baseImageName)
		cond.Reason = shared.VMSetCondReasonPersitentVolumeClaimNotFound
		cond.Type = shared.CommonCondTypeWaiting
		common.LogForObject(r, cond.Message, instance)

		return baseImageName, ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	} else if err != nil {
		cond.Message = fmt.Sprintf("Failed to get persitent volume claim %s ", baseImageName)
		cond.Reason = shared.VMSetCondReasonPersitentVolumeClaimError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return baseImageName, ctrl.Result{}, err
	}
	// mschuppert - do we need to consider any other status
	switch pvc.Annotations["cdi.kubevirt.io/storage.pod.phase"] {
	case "Running":
		instance.Status.BaseImageDVReady = false

		cond.Message = fmt.Sprintf("VM base image %s creation still in running state, reconcile in 2min", baseImageName)
		cond.Reason = shared.VMSetCondReasonPersitentVolumeClaimCreating
		cond.Type = shared.CommonCondTypeWaiting
		common.LogForObject(r, cond.Message, instance)

		return baseImageName, ctrl.Result{RequeueAfter: time.Duration(2) * time.Minute}, nil
	case "Succeeded":
		instance.Status.BaseImageDVReady = true
	}

	return baseImageName, ctrl.Result{}, nil
}

func (r *OpenStackVMSetReconciler) doVMDelete(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	virtualMachineList *virtv1.VirtualMachineList,
) ([]string, error) {
	existingVirtualMachines := map[string]string{}
	removalAnnotatedVirtualMachines := []virtv1.VirtualMachine{}
	deletedHosts := []string{}

	// Generate a map of existing VMs and also store those annotated for potential removal
	for _, virtualMachine := range virtualMachineList.Items {
		existingVirtualMachines[virtualMachine.ObjectMeta.Name] = virtualMachine.ObjectMeta.Name

		if val, ok := virtualMachine.Annotations[shared.HostRemovalAnnotation]; ok && (strings.ToLower(val) == "yes" || strings.ToLower(val) == "true") {
			removalAnnotatedVirtualMachines = append(removalAnnotatedVirtualMachines, virtualMachine)
		}
	}

	// How many VirtualMachine de-allocations do we need (if any)?
	oldVmsToRemoveCount := len(existingVirtualMachines) - instance.Spec.VMCount

	if oldVmsToRemoveCount > 0 {
		if len(removalAnnotatedVirtualMachines) > 0 && len(removalAnnotatedVirtualMachines) == oldVmsToRemoveCount {
			for i := 0; i < oldVmsToRemoveCount; i++ {
				// Choose VirtualMachines to remove from the prepared list of VirtualMachines
				// that have the common.HostRemovalAnnotation annotation
				deletedHost, err := r.virtualMachineDeprovision(
					ctx,
					instance,
					cond,
					&removalAnnotatedVirtualMachines[0],
				)

				if err != nil {
					return deletedHosts, err
				}
				deletedHosts = append(deletedHosts, deletedHost)

				// Remove the removal-annotated VirtualMachine from the existingVirtualMachines map
				delete(existingVirtualMachines, removalAnnotatedVirtualMachines[0].Name)

				// Remove the removal-annotated VirtualMachine from the removalAnnotatedVirtualMachines list
				if len(removalAnnotatedVirtualMachines) > 1 {
					removalAnnotatedVirtualMachines = removalAnnotatedVirtualMachines[1:]
				} else {
					removalAnnotatedVirtualMachines = []virtv1.VirtualMachine{}
				}
			}
		} else {
			cond.Message = fmt.Sprintf("Unable to find sufficient amount of VirtualMachine replicas annotated for scale-down (%d found, %d requested)",
				len(removalAnnotatedVirtualMachines),
				oldVmsToRemoveCount)
			cond.Reason = shared.VMSetCondReasonVirtualMachineAnnotationMissmatch
			cond.Type = shared.CommonCondTypeWaiting
			common.LogForObject(r, cond.Message, instance)
		}
	}

	sort.Strings(deletedHosts)

	return deletedHosts, nil
}

func (r *OpenStackVMSetReconciler) virtualMachineDeprovision(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	virtualMachine *virtv1.VirtualMachine,
) (string, error) {
	r.Log.Info(fmt.Sprintf("Deallocating VirtualMachine: %s", virtualMachine.Name))

	// First check if the finalizer is still there and remove it if so
	if controllerutil.ContainsFinalizer(virtualMachine, vmset.FinalizerName) {
		err := r.virtualMachineFinalizerCleanup(ctx, virtualMachine, cond)

		if err != nil {
			return "", err
		}
	}

	// Delete the VirtualMachine
	err := r.Delete(ctx, virtualMachine, &client.DeleteOptions{})
	if err != nil {
		return virtualMachine.Name, err
	}
	r.Log.Info(fmt.Sprintf("VirtualMachine deleted: name %s", virtualMachine.Name))

	// Also remove networkdata secret
	secret := fmt.Sprintf("%s-%s-networkdata", instance.Name, virtualMachine.Name)
	err = common.DeleteSecretsWithName(
		ctx,
		r,
		cond,
		secret,
		instance.Namespace,
	)
	if err != nil {
		return virtualMachine.Name, err
	}
	r.Log.Info(fmt.Sprintf("Network data secret deleted: name %s", secret))

	// Set status (remove this VMHost entry)
	delete(instance.Status.VMHosts, virtualMachine.Name)

	return virtualMachine.Name, nil
}

// Create/Update NetworkData
func (r *OpenStackVMSetReconciler) createNetworkData(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	osNetCfg *ospdirectorv1beta1.OpenStackNetConfig,
	nadMap map[string]networkv1.NetworkAttachmentDefinition,
	envVars map[string]common.EnvSetter,
	templateParameters map[string]interface{},
) (map[string]ospdirectorv1beta1.Host, error) {
	// We will fill this map with newly-added hosts as well as existing ones
	vmDetails := map[string]ospdirectorv1beta1.Host{}

	// Flag the network data secret as safe to collect with must-gather
	secretLabelsWithMustGather := common.GetLabels(instance, vmset.AppLabel, map[string]string{
		common.MustGatherSecret: "yes",
	})

	// Func to help increase DRY below in NetworkData loops
	generateNetworkData := func(instance *ospdirectorv1beta2.OpenStackVMSet, vm *ospdirectorv1beta2.HostStatus) error {
		// TODO mschuppert: get ctlplane network name using ooo-ctlplane-network label
		netName := "ctlplane"

		networkDataSecret := fmt.Sprintf("%s-%s-networkdata", instance.Name, vm.Hostname)
		vmDetails[vm.Hostname] = ospdirectorv1beta1.Host{
			Hostname:          vm.Hostname,
			HostRef:           vm.Hostname,
			DomainName:        vm.Hostname,
			DomainNameUniq:    fmt.Sprintf("%s-%s", vm.Hostname, instance.UID[0:4]),
			IPAddress:         instance.Status.VMHosts[vm.Hostname].IPAddresses[netName],
			NetworkDataSecret: networkDataSecret,
			Labels:            secretLabelsWithMustGather,
			NAD:               nadMap,
		}

		err := r.generateVirtualMachineNetworkData(
			ctx,
			instance,
			cond,
			osNetCfg,
			&envVars,
			templateParameters,
			vmDetails[vm.Hostname],
		)
		if err != nil {
			cond.Message = fmt.Sprintf("Error creating VM NetworkData for %s ", vm.Hostname)
			cond.Reason = shared.VMSetCondReasonVirtualMachineNetworkDataError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		// update VM status NetworkDataSecret and UserDataSecret
		vm.NetworkDataSecretName = networkDataSecret
		vm.UserDataSecretName = fmt.Sprintf("%s-cloudinit", instance.Name)

		return nil
	}

	// Generate host NetworkData
	for hostname, actualStatus := range instance.Status.DeepCopy().VMHosts {

		if hostname != "" {
			if err := generateNetworkData(instance, &actualStatus); err != nil {
				return vmDetails, err
			}
			currentStatus := instance.Status.VMHosts[hostname]
			if !reflect.DeepEqual(currentStatus, actualStatus) {
				instance.Status.VMHosts[hostname] = actualStatus
				common.LogForObject(
					r,
					fmt.Sprintf("Changed Host NetStatus, updating CR status current - %s: %v / new: %v",
						hostname,
						actualStatus,
						diff.ObjectReflectDiff(currentStatus, actualStatus)),
					instance,
				)
			}
		}
	}

	return vmDetails, nil
}

// Create the VM objects
func (r *OpenStackVMSetReconciler) createVMs(
	ctx context.Context,
	instance *ospdirectorv1beta2.OpenStackVMSet,
	cond *shared.Condition,
	envVars map[string]common.EnvSetter,
	osNetBindings map[string]ospdirectorv1beta1.AttachType,
	baseImageName string,
	vmDetails map[string]ospdirectorv1beta1.Host,
) (ctrl.Result, error) {
	if instance.Status.BaseImageDVReady {
		for _, ctl := range vmDetails {
			// Add chosen baseImageName to controller details, then create the VM instance
			ctl.BaseImageName = baseImageName
			err := r.vmCreateInstance(ctx, instance, cond, envVars, &ctl, osNetBindings)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// Calculate provisioning status
		readyCount := 0
		for _, host := range instance.Status.VMHosts {
			if host.ProvisioningState == shared.ProvisioningState(shared.VMSetCondTypeProvisioned) {
				readyCount++
			}
		}
		instance.Status.ProvisioningStatus.ReadyCount = readyCount

		switch readyCount := instance.Status.ProvisioningStatus.ReadyCount; {
		case readyCount == instance.Spec.VMCount && readyCount == 0:
			cond.Type = shared.VMSetCondTypeEmpty
			cond.Reason = shared.VMSetCondReasonVirtualMachineCountZero
			cond.Message = "No VirtualMachines have been requested"
		case readyCount == instance.Spec.VMCount:
			cond.Type = shared.VMSetCondTypeProvisioned
			cond.Reason = shared.VMSetCondReasonVirtualMachineProvisioned
			cond.Message = "All requested VirtualMachines have been provisioned"
		case readyCount < instance.Spec.VMCount:
			cond.Type = shared.VMSetCondTypeProvisioning
			cond.Reason = shared.VMSetCondReasonVirtualMachineProvisioning
			cond.Message = "Provisioning of VirtualMachines in progress"
		default:
			cond.Type = shared.VMSetCondTypeDeprovisioning
			cond.Reason = shared.VMSetCondReasonVirtualMachineDeprovisioning
			cond.Message = "Deprovisioning of VirtualMachines in progress"
		}

	} else {
		cond.Message = fmt.Sprintf("BaseImageDV is not ready for OpenStackVMSet %s", instance.Name)
		cond.Reason = shared.VMSetCondReasonBaseImageNotReady
		cond.Type = shared.CommonCondTypeWaiting

		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	return ctrl.Result{}, nil
}
