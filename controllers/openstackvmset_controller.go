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
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	//networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackipset"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	virtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

// OpenStackVMSetReconciler reconciles a VMSet object
type OpenStackVMSetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackVMSetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackVMSetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
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
// FIXME: Is there a way to scope the following RBAC annotation to just the "openshift-machine-api" namespace?
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=list;watch
// +kubebuilder:rbac:groups=nmstate.io,resources=nodenetworkconfigurationpolicies,verbs=get;list
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets,verbs=get;list
// FIXME: Is there a way to scope the following RBAC annotation to just the "openshift-sriov-network-operator" namespace?
// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworknodepolicies;sriovnetworks,verbs=get;list;watch;create;update;patch;delete

// Reconcile - controller VMs
func (r *OpenStackVMSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("vmset", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackVMSet{}
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

	// What VMs do we currently have for this OpenStackVMSet?
	virtualMachineList, err := common.GetVirtualMachines(r, instance.Namespace, map[string]string{
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
			r.Log.Info(fmt.Sprintf("Finalizer %s added to CR %s", vmset.FinalizerName, instance.Name))
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, vmset.FinalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. Clean up resources used by the operator
		// VirtualMachine resources
		err := r.virtualMachineListFinalizerCleanup(instance, virtualMachineList)
		if err != nil && !k8s_errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 3. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, vmset.FinalizerName)
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.Status.ProvisioningStatus.State == ospdirectorv1beta1.VMSetProvisioned)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackVMSet %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	// Generate fencing data potentially needed by all VMSets in this instance's namespace
	err = r.generateNamespaceFencingData(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Initialize conditions list if not already set
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
	}

	envVars := make(map[string]common.EnvSetter)

	// get a copy if the CR ProvisioningStatus
	actualProvisioningState := instance.Status.DeepCopy().ProvisioningStatus

	// check for required secrets
	sshSecret, _, err := common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil && k8s_errors.IsNotFound(err) {
		timeout := 20
		msg := fmt.Sprintf("DeploymentSSHSecret secret does not exist: %v", err)
		actualProvisioningState.State = ospdirectorv1beta1.VMSetWaiting
		actualProvisioningState.Reason = msg

		err := r.setProvisioningStatus(instance, actualProvisioningState)

		return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Create/update secrets from templates
	secretLabels := common.GetLabels(instance, vmset.AppLabel, map[string]string{})

	templateParameters := make(map[string]interface{})
	templateParameters["AuthorizedKeys"] = strings.TrimSuffix(string(sshSecret.Data["authorized_keys"]), "\n")

	if instance.Spec.DomainName != "" {
		templateParameters["DomainName"] = instance.Spec.DomainName
	}

	if instance.Spec.PasswordSecret != "" {
		// check if specified password secret exists before creating the controlplane
		passwordSecret, _, err := common.GetSecret(r, instance.Spec.PasswordSecret, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				timeout := 30
				msg := fmt.Sprintf("PasswordSecret %s not found but specified in CR, next reconcile in %d s", instance.Spec.PasswordSecret, timeout)

				actualProvisioningState.State = ospdirectorv1beta1.VMSetWaiting
				actualProvisioningState.Reason = msg

				err := r.setProvisioningStatus(instance, actualProvisioningState)

				return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		// use same NodeRootPassword paremater as tripleo have
		if len(passwordSecret.Data["NodeRootPassword"]) > 0 {
			templateParameters["NodeRootPassword"] = string(passwordSecret.Data["NodeRootPassword"])
		}
	}
	templateParameters["IsTripleoRole"] = instance.Spec.IsTripleoRole

	cloudinit := []common.Template{
		// CloudInitSecret
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

	err = common.EnsureSecrets(r, instance, cloudinit, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Get mapping of all OSNets with their binding type for this namespace
	osNetBindings, err := openstacknet.GetOpenStackNetsBindingMap(r, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Verify that NetworkAttachmentDefinition for each non-SRIOV-configured network exists, and...
	nadMap, err := common.GetAllNetworkAttachmentDefinitions(r, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// ...verify that SriovNetwork and SriovNetworkNodePolicy exists for each SRIOV-configured network
	sriovLabelSelectorMap := map[string]string{
		common.OwnerControllerNameLabelSelector: openstacknet.AppLabel,
		common.OwnerNameSpaceLabelSelector:      instance.Namespace,
	}
	snMap, err := openstacknet.GetSriovNetworksWithLabel(r, sriovLabelSelectorMap, "openshift-sriov-network-operator")
	if err != nil {
		return ctrl.Result{}, err
	}
	snnpMap, err := openstacknet.GetSriovNetworkNodePoliciesWithLabel(r, sriovLabelSelectorMap, "openshift-sriov-network-operator")
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, netNameLower := range instance.Spec.Networks {
		timeout := 10
		var msg string
		actualProvisioningState.State = ospdirectorv1beta1.VMSetWaiting

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
				r.Log.Info(fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower))
				continue
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}

		// We currently support SRIOV and bridge interfaces, with anything other than "sriov" indicating a bridge
		switch osNetBindings[network.Spec.NameLower] {
		case ospdirectorv1beta1.AttachTypeBridge:
			// Non-SRIOV networks should have a NetworkAttachmentDefinition
			if _, ok := nadMap[network.Name]; !ok {
				msg = fmt.Sprintf("NetworkAttachmentDefinition %s does not yet exist.  Reconciling again in %d seconds", network.Name, timeout)
			}
		case ospdirectorv1beta1.AttachTypeSriov:
			// SRIOV networks should have a SriovNetwork and a SriovNetworkNodePolicy
			if _, ok := snMap[fmt.Sprintf("%s-sriov-network", network.Spec.NameLower)]; !ok {
				msg = fmt.Sprintf("SriovNetwork for network %s does not yet exist.  Reconciling again in %d seconds", network.Spec.NameLower, timeout)
			} else if _, ok := snnpMap[fmt.Sprintf("%s-sriov-policy", network.Spec.NameLower)]; !ok {
				msg = fmt.Sprintf("SriovNetworkNodePolicy for network %s does not yet exist.  Reconciling again in %d seconds", network.Spec.NameLower, timeout)
			}
		}

		if msg != "" {
			actualProvisioningState.Reason = msg

			err := r.setProvisioningStatus(instance, actualProvisioningState)

			return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
		}
	}

	//
	// create hostnames for the requested number of systems
	//
	//   create new host allocations count
	//
	newCount := instance.Spec.VMCount - len(instance.Status.VMHosts)
	newVMs := []string{}

	//
	//   create hostnames for the newHostnameCount
	//

	// If VmSet status map is nil, create it
	if instance.Status.VMHosts == nil {
		instance.Status.VMHosts = map[string]ospdirectorv1beta1.HostStatus{}
	}

	currentVMHostsStatus := instance.Status.DeepCopy().VMHosts
	for i := 0; i < newCount; i++ {
		hostnameDetails := common.Hostname{
			Basename: instance.Spec.RoleName,
			VIP:      false,
		}

		err := common.CreateOrGetHostname(instance, &hostnameDetails)
		if err != nil {
			return ctrl.Result{}, err
		}

		if hostnameDetails.Hostname != "" {
			if _, ok := instance.Status.VMHosts[hostnameDetails.Hostname]; !ok {
				instance.Status.VMHosts[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
					Hostname:             hostnameDetails.Hostname,
					HostRef:              hostnameDetails.HostRef,
					AnnotatedForDeletion: false,
					IPAddresses:          map[string]string{},
				}
				newVMs = append(newVMs, hostnameDetails.Hostname)
			}
			r.Log.Info(fmt.Sprintf("VMSet hostname created: %s", hostnameDetails.Hostname))
		}
	}
	actualStatus := instance.Status.VMHosts
	if !reflect.DeepEqual(currentVMHostsStatus, actualStatus) {
		r.Log.Info(fmt.Sprintf("Updating CR status with new hostname information, %d new VMs - %s", len(newVMs), diff.ObjectReflectDiff(currentVMHostsStatus, actualStatus)))
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			r.Log.Error(err, "Failed to update CR status %v")
			return ctrl.Result{}, err
		}
	}

	hostnameRefs := instance.GetHostnames()

	//
	//   check/update instance status for annotated for deletion marged VMs
	//

	// check for deletion marked VMs
	currentVMHostsStatus = instance.Status.VMHosts
	deletionAnnotatedVMs, err := r.getDeletedVMOSPHostnames(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	for hostname, vmStatus := range instance.Status.VMHosts {

		if len(deletionAnnotatedVMs) > 0 && common.StringInSlice(hostname, deletionAnnotatedVMs) {
			// set annotatedForDeletion status of the VM to true, if not already
			if !vmStatus.AnnotatedForDeletion {
				vmStatus.AnnotatedForDeletion = true
				r.Log.Info(fmt.Sprintf("Host deletion annotation set on VM %s", hostname))
			}
		} else {
			// check if the vm was previously flagged as annotated and revert it
			if vmStatus.AnnotatedForDeletion {
				vmStatus.AnnotatedForDeletion = false
				r.Log.Info(fmt.Sprintf("Host deletion annotation removed on VM %s", hostname))
			}
		}
		actualStatus := instance.Status.VMHosts[hostname]
		if !reflect.DeepEqual(&actualStatus, vmStatus) {
			instance.Status.VMHosts[hostname] = vmStatus
		}
	}
	actualStatus = instance.Status.VMHosts
	if !reflect.DeepEqual(currentVMHostsStatus, actualStatus) {
		r.Log.Info(fmt.Sprintf("Updating CR status with deletion annotation information - %s", diff.ObjectReflectDiff(currentVMHostsStatus, actualStatus)))
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	//
	//   Create/Update IPSet for the VMSet CR
	//

	ipsetDetails := common.IPSet{
		Networks:            instance.Spec.Networks,
		Role:                instance.Spec.RoleName,
		HostCount:           instance.Spec.VMCount,
		AddToPredictableIPs: instance.Spec.IsTripleoRole,
		HostNameRefs:        hostnameRefs,
	}
	ipset, op, err := openstackipset.OvercloudipsetCreateOrUpdate(r, instance, ipsetDetails)
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("OpenStackIPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	if len(ipset.Status.HostIPs) < instance.Spec.VMCount {
		timeout := 10
		msg := fmt.Sprintf("OpenStackIPSet has not yet reached the required count %d", instance.Spec.VMCount)
		actualProvisioningState.State = ospdirectorv1beta1.VMSetWaiting
		actualProvisioningState.Reason = msg

		err := r.setProvisioningStatus(instance, actualProvisioningState)

		return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
	}

	//
	//   Create BaseImage for the VMSet
	//

	baseImageName := fmt.Sprintf("osp-vmset-baseimage-%s", instance.UID[0:4])
	if instance.Spec.BaseImageVolumeName != "" {
		baseImageName = instance.Spec.BaseImageVolumeName
	}

	// wait for the base image conversion job to be finished before we create the VMs
	// we check the pvc for the base image:
	// - import in progress
	//   annotations -> cdi.kubevirt.io/storage.pod.phase: Running
	// - when import done
	//   annotations -> cdi.kubevirt.io/storage.pod.phase: Succeeded
	pvc := &corev1.PersistentVolumeClaim{}

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: baseImageName, Namespace: instance.Namespace}, pvc)
	if err != nil && k8s_errors.IsNotFound(err) {
		timeout := 10
		msg := fmt.Sprintf("PersistentVolumeClaim %s not found reconcile again in 10 seconds", baseImageName)
		actualProvisioningState.State = ospdirectorv1beta1.VMSetWaiting
		actualProvisioningState.Reason = msg

		err := r.setProvisioningStatus(instance, actualProvisioningState)

		return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, err
	} else if err != nil {
		return ctrl.Result{}, err
	}
	// mschuppert - do we need to consider any other status
	switch pvc.Annotations["cdi.kubevirt.io/storage.pod.phase"] {
	case "Running":
		instance.Status.BaseImageDVReady = false
		timeout := 2
		msg := fmt.Sprintf("VM base image %s creation still in running state, reconcile in 2min", baseImageName)
		actualProvisioningState.State = ospdirectorv1beta1.VMSetWaiting
		actualProvisioningState.Reason = msg
		err := r.setProvisioningStatus(instance, actualProvisioningState)

		return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Minute}, err
	case "Succeeded":
		instance.Status.BaseImageDVReady = true
	}

	// store current VMHosts status to verify updated NetworkData change
	currentVMHostsStatus = instance.Status.DeepCopy().VMHosts

	//
	//   Handle VM removal from VMSet
	//

	existingVirtualMachines := map[string]string{}
	removalAnnotatedVirtualMachines := []virtv1.VirtualMachine{}

	// Generate a map of existing VMs and also store those annotated for potential removal
	for _, virtualMachine := range virtualMachineList.Items {
		existingVirtualMachines[virtualMachine.ObjectMeta.Name] = virtualMachine.ObjectMeta.Name

		if val, ok := virtualMachine.Annotations[common.HostRemovalAnnotation]; ok && (strings.ToLower(val) == "yes" || strings.ToLower(val) == "true") {
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
				err := r.virtualMachineDeprovision(instance, &removalAnnotatedVirtualMachines[0])

				if err != nil {
					return ctrl.Result{}, err
				}

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
			msg := fmt.Sprintf("Unable to find sufficient amount of VirtualMachine replicas annotated for scale-down (%d found, %d requested)", len(removalAnnotatedVirtualMachines), oldVmsToRemoveCount)

			actualProvisioningState.State = ospdirectorv1beta1.VMSetWaiting
			actualProvisioningState.Reason = msg

			err := r.setProvisioningStatus(instance, actualProvisioningState)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	//
	//   Create/Update NetworkData
	//

	// We will fill this map with newly-added hosts as well as existing ones
	vmDetails := map[string]ospdirectorv1beta1.Host{}

	// Flag the network data secret as safe to collect with must-gather
	secretLabelsWithMustGather := common.GetLabels(instance, vmset.AppLabel, map[string]string{
		common.MustGatherSecret: "yes",
	})

	// Func to help increase DRY below in NetworkData loops
	generateNetworkData := func(instance *ospdirectorv1beta1.OpenStackVMSet, vm *ospdirectorv1beta1.HostStatus) error {
		// TODO: multi nic support with bindata template
		netName := "ctlplane"

		vmDetails[vm.Hostname] = ospdirectorv1beta1.Host{
			Hostname:          vm.Hostname,
			HostRef:           vm.Hostname,
			DomainName:        vm.Hostname,
			DomainNameUniq:    fmt.Sprintf("%s-%s", vm.Hostname, instance.UID[0:4]),
			IPAddress:         ipset.Status.HostIPs[vm.Hostname].IPAddresses[netName],
			NetworkDataSecret: fmt.Sprintf("%s-%s-networkdata", instance.Name, vm.Hostname),
			Labels:            secretLabelsWithMustGather,
			NAD:               nadMap,
		}

		err = r.generateVirtualMachineNetworkData(instance, ipset, &envVars, templateParameters, vmDetails[vm.Hostname])
		if err != nil {
			return err
		}

		// update VMSet network status
		for _, netName := range instance.Spec.Networks {
			vm.HostRef = vm.Hostname
			vm.IPAddresses[netName] = ipset.Status.HostIPs[vm.Hostname].IPAddresses[netName]
		}

		return nil
	}

	// Generate new host NetworkData first, if necessary
	for _, hostname := range newVMs {
		actualStatus := instance.Status.DeepCopy().VMHosts[hostname]

		if hostname != "" {
			if err := generateNetworkData(instance, &actualStatus); err != nil {
				return ctrl.Result{}, err
			}
			currentStatus := instance.Status.VMHosts[hostname]
			if !reflect.DeepEqual(currentStatus, actualStatus) {
				r.Log.Info(fmt.Sprintf("Changed Host NetStatus, updating CR status current - %s: %v / new: %v", hostname, actualStatus, diff.ObjectReflectDiff(currentStatus, actualStatus)))
				instance.Status.VMHosts[hostname] = actualStatus
			}
		}

	}

	// Generate existing host NetworkData next
	for hostname, actualStatus := range instance.Status.DeepCopy().VMHosts {

		if !common.StringInSlice(hostname, newVMs) && hostname != "" {
			if err := generateNetworkData(instance, &actualStatus); err != nil {
				return ctrl.Result{}, err
			}
			currentStatus := instance.Status.VMHosts[hostname]
			if !reflect.DeepEqual(currentStatus, actualStatus) {
				r.Log.Info(fmt.Sprintf("Changed Host NetStatus, updating CR status current - %s: %v / new: %v", hostname, actualStatus, diff.ObjectReflectDiff(currentStatus, actualStatus)))
				instance.Status.VMHosts[hostname] = actualStatus
			}
		}
	}
	actualStatus = instance.Status.VMHosts
	if !reflect.DeepEqual(currentVMHostsStatus, actualStatus) {
		r.Log.Info(fmt.Sprintf("Updating CR status information - %s", diff.ObjectReflectDiff(currentVMHostsStatus, actualStatus)))

		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	//
	//   Create the VM objects
	//
	if instance.Status.BaseImageDVReady {
		for _, ctl := range vmDetails {
			// Add chosen baseImageName to controller details, then create the VM instance
			ctl.BaseImageName = baseImageName
			err = r.vmCreateInstance(instance, envVars, &ctl, osNetBindings)
			if err != nil {
				if strings.Contains(err.Error(), registry.OptimisticLockErrorMsg) {
					return ctrl.Result{RequeueAfter: time.Second * 1}, nil
				}

				msg := err.Error()
				actualProvisioningState.State = ospdirectorv1beta1.VMSetError
				actualProvisioningState.Reason = msg

				_ = r.setProvisioningStatus(instance, actualProvisioningState)
				return ctrl.Result{}, err
			}
		}

		// Calculate provisioning status
		var msg string
		var readyCount int

		for _, host := range instance.Status.VMHosts {
			if host.ProvisioningState == ospdirectorv1beta1.VMSetProvisioned {
				readyCount++
			}
		}

		actualProvisioningState.ReadyCount = readyCount

		if readyCount == instance.Spec.VMCount {
			if readyCount == 0 {
				actualProvisioningState.State = ospdirectorv1beta1.VMSetEmpty
				msg = "No VirtualMachines have been requested"
			} else {
				actualProvisioningState.State = ospdirectorv1beta1.VMSetProvisioned
				msg = "All requested VirtualMachines have been provisioned"
			}
		} else if readyCount < instance.Spec.VMCount {
			actualProvisioningState.State = ospdirectorv1beta1.VMSetProvisioning
			msg = "Provisioning of VirtualMachines in progress"
		} else {
			actualProvisioningState.State = ospdirectorv1beta1.VMSetDeprovisioning
			msg = "Deprovisioning of VirtualMachines in progress"
		}

		actualProvisioningState.Reason = msg

		err := r.setProvisioningStatus(instance, actualProvisioningState)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		msg := fmt.Sprintf("BaseImageDV is not ready for OpenStackVMSet %s", instance.Name)
		actualProvisioningState.Reason = msg
		actualProvisioningState.State = ospdirectorv1beta1.VMSetWaiting

		err := r.setProvisioningStatus(instance, actualProvisioningState)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackVMSetReconciler) generateNamespaceFencingData(instance *ospdirectorv1beta1.OpenStackVMSet) error {
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

	if err := common.EnsureSecrets(r, instance, saSecretTemplate, nil); err != nil {
		return err
	}

	// Use aforementioned namespace-scoped kubevirt fencing agent service account token secret to
	// generate a namespace-scoped kubevirt fencing kubeconfig (all OSVMSets in this namespace will
	// share this same secret)

	// Get the kubeconfig used by the operator to acquire the cluster's API address
	kubeconfig, err := config.GetConfig()

	if err != nil {
		return err
	}

	templateParameters := map[string]interface{}{}
	templateParameters["Server"] = kubeconfig.Host
	templateParameters["Namespace"] = instance.Namespace

	// Read-back the secret for the kubevirt agent service account token
	kubevirtAgentTokenSecret, err := r.Kclient.CoreV1().Secrets(instance.Namespace).Get(context.TODO(), vmset.KubevirtFencingServiceAccountSecret, metav1.GetOptions{})

	if err != nil {
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

	err = common.EnsureSecrets(r, instance, kubeconfigTemplate, nil)

	return err
}

func (r *OpenStackVMSetReconciler) generateVirtualMachineNetworkData(instance *ospdirectorv1beta1.OpenStackVMSet, ipset *ospdirectorv1beta1.OpenStackIPSet, envVars *map[string]common.EnvSetter, templateParameters map[string]interface{}, host ospdirectorv1beta1.Host) error {
	templateParameters["ControllerIP"] = host.IPAddress
	templateParameters["CtlplaneInterface"] = instance.Spec.CtlplaneInterface
	templateParameters["CtlplaneDns"] = instance.Spec.BootstrapDNS
	templateParameters["CtlplaneDnsSearch"] = instance.Spec.DNSSearchDomains

	gateway := ipset.Status.Networks["ctlplane"].Gateway
	if gateway != "" {
		if strings.Contains(gateway, ":") {
			templateParameters["Gateway"] = fmt.Sprintf("gateway6: %s", gateway)
		} else {
			templateParameters["Gateway"] = fmt.Sprintf("gateway4: %s", gateway)
		}
	}
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

	err := common.EnsureSecrets(r, instance, networkdata, envVars)
	if err != nil {
		return err
	}

	return nil
}

func (r *OpenStackVMSetReconciler) virtualMachineDeprovision(instance *ospdirectorv1beta1.OpenStackVMSet, virtualMachine *virtv1.VirtualMachine) error {
	r.Log.Info(fmt.Sprintf("Deallocating VirtualMachine: %s", virtualMachine.Name))

	// First check if the finalizer is still there and remove it if so
	if controllerutil.ContainsFinalizer(virtualMachine, vmset.FinalizerName) {
		err := r.virtualMachineFinalizerCleanup(virtualMachine)

		if err != nil {
			return err
		}
	}

	// Delete the VirtualMachine
	err := r.Client.Delete(context.TODO(), virtualMachine, &client.DeleteOptions{})
	if err != nil {
		return err
	}
	r.Log.Info(fmt.Sprintf("VirtualMachine deleted: name %s", virtualMachine.Name))

	// Also remove networkdata secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-networkdata", instance.Name, virtualMachine.Name),
			Namespace: instance.Namespace,
		},
	}

	err = r.Client.Delete(context.TODO(), secret, &client.DeleteOptions{})
	if err != nil {
		return err
	}
	r.Log.Info(fmt.Sprintf("Network data secret deleted: name %s", secret.Name))

	// Set status (remove this VMHost entry)
	delete(instance.Status.VMHosts, virtualMachine.Name)

	return nil
}

func (r *OpenStackVMSetReconciler) virtualMachineListFinalizerCleanup(instance *ospdirectorv1beta1.OpenStackVMSet, virtualMachineList *virtv1.VirtualMachineList) error {
	r.Log.Info(fmt.Sprintf("Removing finalizers from VirtualMachines in VMSet: %s", instance.Name))

	for _, virtualMachine := range virtualMachineList.Items {
		err := r.virtualMachineFinalizerCleanup(&virtualMachine)

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *OpenStackVMSetReconciler) virtualMachineFinalizerCleanup(virtualMachine *virtv1.VirtualMachine) error {
	controllerutil.RemoveFinalizer(virtualMachine, vmset.FinalizerName)
	err := r.Client.Update(context.TODO(), virtualMachine)

	if err != nil {
		return err
	}

	r.Log.Info(fmt.Sprintf("VirtualMachine finalizer removed: name %s", virtualMachine.Name))

	return nil
}

// SetupWithManager -
func (r *OpenStackVMSetReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// index spec.isTripleoRole so that it can be used to
	// https://github.com/kubernetes-sigs/kubebuilder/issues/547#issuecomment-573870160
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &ospdirectorv1beta1.OpenStackVMSet{}, "spec.isTripleoRole", func(obj client.Object) []string {
		vmset := obj.(*ospdirectorv1beta1.OpenStackVMSet)
		return []string{strconv.FormatBool(vmset.Spec.IsTripleoRole)}
	}); err != nil {
		return err
	}

	// TODO: Myabe use filtering functions here since some resource permissions
	// are now cluster-scoped?
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackVMSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&virtv1.VirtualMachine{}).
		Owns(&ospdirectorv1beta1.OpenStackIPSet{}).
		Complete(r)
}

func (r *OpenStackVMSetReconciler) vmCreateInstance(
	instance *ospdirectorv1beta1.OpenStackVMSet,
	envVars map[string]common.EnvSetter,
	ctl *ospdirectorv1beta1.Host,
	osNetBindings map[string]ospdirectorv1beta1.AttachType,
) error {

	evictionStrategy := virtv1.EvictionStrategyLiveMigrate
	fsMode := corev1.PersistentVolumeFilesystem
	trueValue := true
	terminationGracePeriodSeconds := int64(0)

	// get deployment userdata from secret
	userdataSecret := fmt.Sprintf("%s-cloudinit", instance.Name)
	secret, _, err := common.GetSecret(r, userdataSecret, instance.Namespace)
	if err != nil {
		r.Log.Info("CloudInit userdata secret not found!!")
		return err
	}

	// dvTemplateSpec for the VM
	dvTemplateSpec := virtv1.DataVolumeTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ctl.DomainNameUniq,
			Namespace: instance.Namespace,
		},
		Spec: cdiv1.DataVolumeSpec{
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					"ReadWriteOnce",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", instance.Spec.DiskSize)),
					},
				},
				VolumeMode: &fsMode,
			},
			Source: cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      ctl.BaseImageName,
					Namespace: instance.Namespace,
				},
			},
		},
	}
	// set StorageClasseName when specified in the CR
	if instance.Spec.StorageClass != "" {
		dvTemplateSpec.Spec.PVC.StorageClassName = &instance.Spec.StorageClass
	}

	disks := []virtv1.Disk{
		{
			Name: "rootdisk",
			DiskDevice: virtv1.DiskDevice{
				Disk: &virtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
		{
			Name: "cloudinitdisk",
			DiskDevice: virtv1.DiskDevice{
				Disk: &virtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
		{
			Name:   "fencingdisk",
			Serial: "fencingdisk",
			DiskDevice: virtv1.DiskDevice{
				Disk: &virtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
	}

	volumes := []virtv1.Volume{
		{
			Name: "rootdisk",
			VolumeSource: virtv1.VolumeSource{
				DataVolume: &virtv1.DataVolumeSource{
					Name: ctl.DomainNameUniq,
				},
			},
		},
		{
			Name: "cloudinitdisk",
			VolumeSource: virtv1.VolumeSource{
				CloudInitNoCloud: &virtv1.CloudInitNoCloudSource{
					UserDataSecretRef: &corev1.LocalObjectReference{
						Name: secret.Name,
					},
					NetworkDataSecretRef: &corev1.LocalObjectReference{
						Name: ctl.NetworkDataSecret,
					},
				},
			},
		},
		{
			Name: "fencingdisk",
			VolumeSource: virtv1.VolumeSource{
				Secret: &virtv1.SecretVolumeSource{
					SecretName: vmset.KubevirtFencingKubeconfigSecret,
				},
			},
		},
	}

	labels := common.GetLabels(instance, vmset.AppLabel, map[string]string{
		common.OSPHostnameLabelSelector: ctl.Hostname,
		"kubevirt.io/vm":                ctl.DomainName,
	})

	vmTemplate := virtv1.VirtualMachineInstanceTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: virtv1.VirtualMachineInstanceSpec{
			Hostname:                      ctl.DomainName,
			EvictionStrategy:              &evictionStrategy,
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			Domain: virtv1.DomainSpec{
				CPU: &virtv1.CPU{
					Cores: instance.Spec.Cores,
				},
				Devices: virtv1.Devices{
					Disks: disks,
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
				Machine: virtv1.Machine{
					Type: "",
				},
				Resources: virtv1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", instance.Spec.Memory)),
					},
				},
			},
			Volumes: volumes,
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

	// This run strategy ensures that the VM boots upon creation and reboots upon
	// failure, but also allows us to issue manual power commands to the Kubevirt API.
	// The default strategy, "Always", disallows the direct power command API calls
	// that are required by the Kubevirt fencing agent
	runStrategy := virtv1.RunStrategyRerunOnFailure

	// VM
	vm := &virtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ctl.DomainName,
			Namespace: instance.Namespace,
			Labels:    common.GetLabels(instance, vmset.AppLabel, map[string]string{}),
		},
		Spec: virtv1.VirtualMachineSpec{
			RunStrategy: &runStrategy,
			Template:    &vmTemplate,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, vm, func() error {

		vm.Labels = common.GetLabels(instance, vmset.AppLabel, map[string]string{})

		vm.Spec.DataVolumeTemplates = []virtv1.DataVolumeTemplateSpec{dvTemplateSpec}

		// merge additional networks
		networks := instance.Spec.Networks
		// sort networks to get an expected ordering for easier ooo nic template creation
		sort.Strings(networks)
		for _, netNameLower := range networks {

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
					r.Log.Info(fmt.Sprintf("OpenStackNet with NameLower %s not found!", netNameLower))
					continue
				}
				// Error reading the object - requeue the request.
				return err
			}

			if _, ok := osNetBindings[netNameLower]; !ok {
				return fmt.Errorf("OpenStackVMSet vmCreateInstance: No binding type found %s - available bindings: %v", network.Name, osNetBindings)
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

		err := controllerutil.SetControllerReference(instance, vm, r.Scheme)

		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("VirtualMachine %s successfully reconciled - operation: %s", vm.Name, string(op)))
	}

	hostStatus := instance.Status.VMHosts[ctl.Hostname]

	if vm.Status.Ready {
		hostStatus.ProvisioningState = ospdirectorv1beta1.VMSetProvisioned
	} else if vm.Status.Created {
		hostStatus.ProvisioningState = ospdirectorv1beta1.VMSetProvisioning
	}

	instance.Status.VMHosts[ctl.Hostname] = hostStatus

	return nil
}

func (r *OpenStackVMSetReconciler) getDeletedVMOSPHostnames(instance *ospdirectorv1beta1.OpenStackVMSet) ([]string, error) {

	var annotatedVMs []string

	labelSelector := map[string]string{
		common.OwnerUIDLabelSelector:       string(instance.UID),
		common.OwnerNameSpaceLabelSelector: instance.Namespace,
		common.OwnerNameLabelSelector:      instance.Name,
	}

	vmHostList, err := common.GetVirtualMachines(r, instance.Namespace, labelSelector)
	if err != nil {
		return annotatedVMs, err
	}

	// Get list of OSP hostnames from HostRemovalAnnotation annotated VMs
	for _, vm := range vmHostList.Items {

		if val, ok := vm.Annotations[common.HostRemovalAnnotation]; ok && (strings.ToLower(val) == "yes" || strings.ToLower(val) == "true") {
			ospHostname := vm.Spec.Template.Spec.Hostname
			r.Log.Info(fmt.Sprintf("VM %s/%s annotated for deletion", vm.Name, ospHostname))
			annotatedVMs = append(annotatedVMs, ospHostname)
		}
	}

	return annotatedVMs, nil
}

func (r *OpenStackVMSetReconciler) setProvisioningStatus(instance *ospdirectorv1beta1.OpenStackVMSet, actualState ospdirectorv1beta1.OpenStackVMSetProvisioningStatus) error {
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
