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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	//networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
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
	virtualMachineList, err := common.GetAllVirtualMachinesWithLabel(r, map[string]string{
		OwnerNameLabelSelector: instance.Name,
	}, instance.Namespace)

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
		if err != nil && !errors.IsNotFound(err) {
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

	envVars := make(map[string]common.EnvSetter)

	// check for required secrets
	sshSecret, _, err := common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil && errors.IsNotFound(err) {
		return ctrl.Result{RequeueAfter: time.Second * 20}, fmt.Errorf("DeploymentSSHSecret secret does not exist: %v", err)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Create/update secrets from templates
	secretLabels := common.GetLabels(instance.Name, vmset.AppLabel)

	templateParameters := make(map[string]interface{})
	templateParameters["AuthorizedKeys"] = strings.TrimSuffix(string(sshSecret.Data["authorized_keys"]), "\n")

	if instance.Spec.PasswordSecret != "" {
		// check if specified password secret exists before creating the controlplane
		passwordSecret, _, err := common.GetSecret(r, instance.Spec.PasswordSecret, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("PasswordSecret %s not found but specified in CR, next reconcile in 30s", instance.Spec.PasswordSecret)
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		// use same NodeRootPassword paremater as tripleo have
		if len(passwordSecret.Data["NodeRootPassword"]) > 0 {
			templateParameters["NodeRootPassword"] = string(passwordSecret.Data["NodeRootPassword"])
		}
	}

	cloudinit := []common.Template{
		// CloudInitSecret
		{
			Name:           fmt.Sprintf("%s-cloudinit", instance.Name),
			Namespace:      instance.Namespace,
			Type:           common.TemplateTypeNone,
			InstanceType:   instance.Kind,
			AdditionalData: map[string]string{"userdata": "/vmset/cloudinit/userdata"},
			Labels:         secretLabels,
			ConfigOptions:  templateParameters,
		},
		// TODO: mschuppert should we have this?
		// CloudInitCustomSecret
		//{
		//	Name:      fmt.Sprintf("%s-cloudinit-custom", instance.Name),
		//	Namespace: instance.Namespace,
		//	Type:      common.TemplateTypeCustom,
		//	Labels:    secretLabels,
		//},
	}

	err = common.EnsureSecrets(r, instance, cloudinit, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Get mapping of OSNets to binding type for this namespace
	osNetBindings, err := openstacknet.GetOpenStackNetsBindingMap(r, instance.Namespace)

	if err != nil {
		return ctrl.Result{}, err
	}

	// Verify that NodeNetworkConfigurationPolicy and NetworkAttachmentDefinition for each non-SRIOV-configured network exists, and...
	nncMap, err := common.GetAllNetworkConfigurationPolicies(r, map[string]string{"owner": "osp-director"})
	if err != nil {
		return ctrl.Result{}, err
	}
	nadMap, err := common.GetAllNetworkAttachmentDefinitions(r, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// ...verify that SriovNetwork and SriovNetworkNodePolicy exists for each SRIOV-configured network
	sriovLabelSelectorMap := map[string]string{
		"app":                       openstacknet.AppLabel,
		OwnerNameSpaceLabelSelector: instance.Namespace,
	}
	snMap, err := openstacknet.GetSriovNetworksWithLabel(r, sriovLabelSelectorMap, "openshift-sriov-network-operator")
	if err != nil {
		return ctrl.Result{}, err
	}
	snnpMap, err := openstacknet.GetSriovNetworkNodePoliciesWithLabel(r, sriovLabelSelectorMap, "openshift-sriov-network-operator")
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, net := range instance.Spec.Networks {
		if osNetBindings[net] == "" {
			// Non-SRIOV networks should have a NetworkConfigurationPolicy and a NetworkAttachmentDefinition
			if _, ok := nncMap[net]; !ok {
				r.Log.Info(fmt.Sprintf("NetworkConfigurationPolicy for network %s does not yet exist.  Reconciling again in 10 seconds", net))
				return ctrl.Result{RequeueAfter: time.Second * 10}, nil
			}
			if _, ok := nadMap[net]; !ok {
				r.Log.Error(err, fmt.Sprintf("NetworkAttachmentDefinition for network %s does not yet exist.  Reconciling again in 10 seconds", net))
				return ctrl.Result{RequeueAfter: time.Second * 10}, nil
			}
		} else if osNetBindings[net] == "sriov" {
			// SRIOV networks should have a SriovNetwork and a SriovNetworkNodePolicy
			if _, ok := snMap[fmt.Sprintf("%s-sriov-network", net)]; !ok {
				r.Log.Info(fmt.Sprintf("SriovNetwork for network %s does not yet exist.  Reconciling again in 10 seconds", net))
				return ctrl.Result{RequeueAfter: time.Second * 10}, nil
			}
			if _, ok := snnpMap[fmt.Sprintf("%s-sriov-policy", net)]; !ok {
				r.Log.Error(err, fmt.Sprintf("SriovNetworkNodePolicy for network %s does not yet exist.  Reconciling again in 10 seconds", net))
				return ctrl.Result{RequeueAfter: time.Second * 10}, nil
			}
		}
	}

	ipsetDetails := common.IPSet{
		Networks:            instance.Spec.Networks,
		Role:                instance.Spec.RoleName,
		HostCount:           instance.Spec.VMCount,
		AddToPredictableIPs: instance.Spec.IsTripleoRole,
	}
	ipset, op, err := common.OvercloudipsetCreateOrUpdate(r, instance, ipsetDetails)
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("IPSet for %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	if len(ipset.Status.HostIPs) < instance.Spec.VMCount {
		r.Log.Info(fmt.Sprintf("IPSet has not yet reached the required replicas %d", instance.Spec.VMCount))
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	baseImageName := fmt.Sprintf("osp-vmset-baseimage-%s", instance.UID[0:4])
	if instance.Spec.BaseImageVolumeName != "" {
		baseImageName = instance.Spec.BaseImageVolumeName
	}

	// Create base image, controller disks get cloned from this
	if instance.Spec.BaseImageVolumeName == "" {
		err = r.cdiCreateBaseDisk(instance, baseImageName)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// wait for the base image conversion job to be finished before we create the VMs
	// we check the pvc for the base image:
	// - import in progress
	//   annotations -> cdi.kubevirt.io/storage.pod.phase: Running
	// - when import done
	//   annotations -> cdi.kubevirt.io/storage.pod.phase: Succeeded
	pvc := &corev1.PersistentVolumeClaim{}

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: baseImageName, Namespace: instance.Namespace}, pvc)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info(fmt.Sprintf("PersistentVolumeClaim %s not found reconcile again in 10 seconds", baseImageName))
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}
	// mschuppert - do we need to consider any other status
	switch pvc.Annotations["cdi.kubevirt.io/storage.pod.phase"] {
	case "Running":
		r.Log.Info(fmt.Sprintf("VM base image %s creation still in running state, reconcile in 2min", baseImageName))
		instance.Status.BaseImageDVReady = false
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	case "Succeeded":
		instance.Status.BaseImageDVReady = true
	}

	existingVirtualMachines := map[string]string{}
	removalAnnotatedVirtualMachines := []virtv1.VirtualMachine{}

	// Generate a map of existing VMs and also store those annotated for potential removal
	for _, virtualMachine := range virtualMachineList.Items {
		existingVirtualMachines[virtualMachine.ObjectMeta.Name] = virtualMachine.ObjectMeta.Name

		if val, ok := virtualMachine.Annotations[vmset.VirtualMachineRemovalAnnotation]; ok && (strings.ToLower(val) == "yes" || strings.ToLower(val) == "true") {
			removalAnnotatedVirtualMachines = append(removalAnnotatedVirtualMachines, virtualMachine)
		}
	}

	// How many VirtualMachine de-allocations do we need (if any)?
	oldVmsToRemoveCount := len(existingVirtualMachines) - instance.Spec.VMCount

	if oldVmsToRemoveCount > 0 {
		if len(removalAnnotatedVirtualMachines) > 0 && len(removalAnnotatedVirtualMachines) == oldVmsToRemoveCount {
			for i := 0; i < oldVmsToRemoveCount; i++ {
				// Choose VirtualMachines to remove from the prepared list of VirtualMachines
				// that have the "osp-director.openstack.org/vmset-delete-virtualmachine=yes" annotation
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
			r.Log.Info(fmt.Sprintf("WARNING: Unable to find sufficient amount of VirtualMachine replicas annotated for scale-down (%d found, %d requested)", len(removalAnnotatedVirtualMachines), oldVmsToRemoveCount))
		}
	}

	// We will fill this map will newly-added hosts as well as existing ones
	controllerDetails := map[string]ospdirectorv1beta1.Host{}

	// How many new VirtualMachine allocations do we need (if any)?
	newVmsNeededCount := instance.Spec.VMCount - len(existingVirtualMachines)

	// Func to help increase DRY below in NetworkData loops
	generateNetworkData := func(instance *ospdirectorv1beta1.OpenStackVMSet, hostKey string) error {
		// TODO: multi nic support with bindata template
		netName := "ctlplane"

		hostnameDetails := common.Hostname{
			IDKey:    hostKey, //fmt.Sprintf("%s-%d", strings.ToLower(instance.Spec.Role), i),
			Basename: instance.Spec.RoleName,
			VIP:      false,
		}

		err := common.CreateOrGetHostname(instance, &hostnameDetails)
		if err != nil {
			return err
		}
		r.Log.Info(fmt.Sprintf("VMSet %s hostname set to %s", hostnameDetails.IDKey, hostnameDetails.Hostname))

		controllerDetails[hostnameDetails.Hostname] = ospdirectorv1beta1.Host{
			Hostname:          hostnameDetails.Hostname,
			DomainName:        hostnameDetails.Hostname,
			DomainNameUniq:    fmt.Sprintf("%s-%s", hostnameDetails.Hostname, instance.UID[0:4]),
			IPAddress:         ipset.Status.HostIPs[hostnameDetails.IDKey].IPAddresses[netName],
			NetworkDataSecret: fmt.Sprintf("%s-%s-networkdata", instance.Name, hostnameDetails.Hostname),
			Labels:            secretLabels,
			NNCP:              nncMap,
			NAD:               nadMap,
		}

		err = r.generateVirtualMachineNetworkData(instance, ipset, &envVars, templateParameters, controllerDetails[hostnameDetails.Hostname])
		if err != nil {
			return err
		}

		// update VMSet network status
		for _, netName := range instance.Spec.Networks {
			r.setNetStatus(instance, &hostnameDetails, netName, ipset.Status.HostIPs[hostnameDetails.IDKey].IPAddresses[netName])
		}
		r.Log.Info(fmt.Sprintf("VMSet network status for Hostname: %s - %s", instance.Status.VMHosts[hostnameDetails.IDKey].Hostname, instance.Status.VMHosts[hostnameDetails.IDKey].IPAddresses))

		return nil
	}

	// Generate new host NetworkData first, if necessary
	for i := 0; i < newVmsNeededCount; i++ {
		if err := generateNetworkData(instance, ""); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Generate existing host NetworkData next
	for _, virtualMachine := range existingVirtualMachines {
		if err := generateNetworkData(instance, virtualMachine); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create controller VM objects and finally set VMSet status in etcd
	if instance.Status.BaseImageDVReady {
		for _, ctl := range controllerDetails {
			err = r.vmCreateInstance(instance, envVars, &ctl, osNetBindings, baseImageName)
			if err != nil {
				return ctrl.Result{}, err
			}
			r.Log.Info(fmt.Sprintf("VMSet VM hostname set to %s", ctl.Hostname))
		}

		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		r.Log.Info("BaseImageDV is not Ready!")
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackVMSetReconciler) generateVirtualMachineNetworkData(instance *ospdirectorv1beta1.OpenStackVMSet, ipset *ospdirectorv1beta1.OpenStackIPSet, envVars *map[string]common.EnvSetter, templateParameters map[string]interface{}, host ospdirectorv1beta1.Host) error {
	templateParameters["ControllerIP"] = host.IPAddress

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
			Name:           host.NetworkDataSecret,
			Namespace:      instance.Namespace,
			Type:           common.TemplateTypeNone,
			InstanceType:   instance.Kind,
			AdditionalData: map[string]string{"networkdata": "/vmset/cloudinit/networkdata"},
			Labels:         host.Labels,
			ConfigOptions:  templateParameters,
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
	if controllerutil.ContainsFinalizer(virtualMachine, vmset.VirtualMachineFinalizerName) {
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
	controllerutil.RemoveFinalizer(virtualMachine, vmset.VirtualMachineFinalizerName)
	err := r.Client.Update(context.TODO(), virtualMachine)

	if err != nil {
		return err
	}

	r.Log.Info(fmt.Sprintf("VirtualMachine finalizer removed: name %s", virtualMachine.Name))

	return nil
}

func (r *OpenStackVMSetReconciler) setNetStatus(instance *ospdirectorv1beta1.OpenStackVMSet, hostnameDetails *common.Hostname, netName string, ipaddress string) {

	// If VMSet status map is nil, create it
	if instance.Status.VMHosts == nil {
		instance.Status.VMHosts = map[string]ospdirectorv1beta1.HostStatus{}
	}

	// Set network information status
	if instance.Status.VMHosts[hostnameDetails.IDKey].IPAddresses == nil {
		instance.Status.VMHosts[hostnameDetails.IDKey] = ospdirectorv1beta1.HostStatus{
			Hostname: hostnameDetails.Hostname,
			IPAddresses: map[string]string{
				netName: ipaddress,
			},
		}
	} else {
		instance.Status.VMHosts[hostnameDetails.IDKey].IPAddresses[netName] = ipaddress
	}
}

// SetupWithManager -
func (r *OpenStackVMSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: Myabe use filtering functions here since some resource permissions
	// are now cluster-scoped?
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackVMSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&virtv1.VirtualMachine{}).
		Complete(r)
}

func (r *OpenStackVMSetReconciler) cdiCreateBaseDisk(instance *ospdirectorv1beta1.OpenStackVMSet, baseImageVolumeName string) error {

	fsMode := corev1.PersistentVolumeFilesystem

	cdi := cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      baseImageVolumeName,
			Namespace: instance.Namespace,
			Labels:    common.GetLabelSelector(instance, vmset.AppLabel),
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
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: instance.Spec.BaseImageURL,
				},
			},
		},
	}

	cdi.Kind = "DataVolume"
	cdi.APIVersion = "cdi.kubevirt.io/v1alpha1"

	if instance.Spec.StorageClass != "" {
		cdi.Spec.PVC.StorageClassName = &instance.Spec.StorageClass
	}

	b, err := json.Marshal(cdi.DeepCopyObject())
	if err != nil {
		return err
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	// apply object using server side apply
	if err := common.SSAApplyYaml(context.TODO(), cfg, instance.Kind, b); err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to apply object %s", cdi.GetName()))
		return err
	}

	return err
}

func (r *OpenStackVMSetReconciler) vmCreateInstance(instance *ospdirectorv1beta1.OpenStackVMSet, envVars map[string]common.EnvSetter, ctl *ospdirectorv1beta1.Host, osNetBindings map[string]string, baseImageVolumeName string) error {

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
					Name:      baseImageVolumeName,
					Namespace: instance.Namespace,
				},
			},
		},
	}
	// set StorageClasseName when specified in the CR
	if instance.Spec.StorageClass != "" {
		dvTemplateSpec.Spec.PVC.StorageClassName = &instance.Spec.StorageClass
	}

	rootDisk := virtv1.Disk{
		Name: "rootdisk",
		DiskDevice: virtv1.DiskDevice{
			Disk: &virtv1.DiskTarget{
				Bus: "virtio",
			},
		},
	}

	cloudinitdisk := virtv1.Disk{
		Name: "cloudinitdisk",
		DiskDevice: virtv1.DiskDevice{
			Disk: &virtv1.DiskTarget{
				Bus: "virtio",
			},
		},
	}

	vmTemplate := virtv1.VirtualMachineInstanceTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"kubevirt.io/vm": ctl.DomainName,
				"openstackvmsets.osp-director.openstack.org/ospcontroller": "True",
			},
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
					Disks: []virtv1.Disk{
						rootDisk,
						cloudinitdisk,
					},
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
			Volumes: []virtv1.Volume{
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
			},
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
	vm := &virtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ctl.DomainName,
			Namespace: instance.Namespace,
			Labels:    common.GetLabelSelector(instance, vmset.AppLabel),
		},
		Spec: virtv1.VirtualMachineSpec{
			Template: &vmTemplate,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, vm, func() error {

		vm.Labels = common.GetLabelSelector(instance, vmset.AppLabel)

		vm.Spec.DataVolumeTemplates = []virtv1.DataVolumeTemplateSpec{dvTemplateSpec}
		vm.Spec.Running = &trueValue

		// merge additional networks
		networks := instance.Spec.Networks
		// sort networks to get an expected ordering for easier ooo nic template creation
		sort.Strings(networks)
		for _, net := range networks {
			ok := false
			osNetBinding := ""

			if osNetBinding, ok = osNetBindings[net]; !ok {
				return fmt.Errorf("OpenStackVMSet vmCreateInstance: No binding type found for network %s", net)
			}

			vm.Spec.Template.Spec.Domain.Devices.Interfaces = vmset.MergeVMInterfaces(
				vm.Spec.Template.Spec.Domain.Devices.Interfaces,
				vmset.InterfaceSetterMap{
					net: vmset.Interface(net, osNetBinding),
				},
			)

			vm.Spec.Template.Spec.Networks = vmset.MergeVMNetworks(
				vm.Spec.Template.Spec.Networks,
				vmset.NetSetterMap{
					net: vmset.Network(net, osNetBinding),
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

	return nil
}
