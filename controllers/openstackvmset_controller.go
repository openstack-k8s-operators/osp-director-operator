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
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/api/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	bindatautil "github.com/openstack-k8s-operators/osp-director-operator/pkg/bindata_util"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"

	sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	virtv1 "kubevirt.io/client-go/api/v1"
	//cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
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
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,namespace=openstack,resources=network-attachment-definitions,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=kubevirt.io,namespace=openstack,resources=virtualmachines,verbs=create;delete;get;list;patch;update;watch
// FIXME: Is there a way to scope the following RBAC annotation to just the "openshift-machine-api" namespace?
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=list;watch
// +kubebuilder:rbac:groups=nmstate.io,resources=nodenetworkconfigurationpolicies,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/finalizers,verbs=get;list;watch;create;update;patch;delete
// FIXME: Is there a way to scope the following RBAC annotation to just the "openshift-sriov-network-operator" namespace?
// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworknodepolicies;sriovnetworks,verbs=get;list;watch;create;update;patch;delete

// Reconcile - controller VMs
func (r *OpenStackVMSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
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
			if err := r.Update(context.Background(), instance); err != nil {
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
		// SRIOV resources
		err = r.sriovResourceCleanup(instance)
		if err != nil {
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

	// Create base image, controller disks get cloned from this
	if instance.Spec.BaseImageVolumeName == "" {
		err = r.cdiCreateBaseDisk(instance)
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
	baseImageName := fmt.Sprintf("osp-vmset-baseimage-%s", instance.UID[0:4])
	if instance.Spec.BaseImageVolumeName != "" {
		baseImageName = instance.Spec.BaseImageVolumeName
	}
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

	// Create network config
	err = r.createNetworkConfig(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create controller VM objects and finally set VMSet status in etcd
	if instance.Status.BaseImageDVReady {
		for _, ctl := range controllerDetails {
			err = r.vmCreateInstance(instance, envVars, &ctl)
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

func (r *OpenStackVMSetReconciler) sriovResourceCleanup(instance *ospdirectorv1beta1.OpenStackVMSet) error {
	labelSelectorMap := map[string]string{
		OwnerUIDLabelSelector:       string(instance.UID),
		OwnerNameSpaceLabelSelector: instance.Namespace,
		OwnerNameLabelSelector:      instance.Name,
	}

	// Delete sriovnetworks in openshift-sriov-network-operator namespace
	sriovNetworks, err := vmset.GetSriovNetworksWithLabel(r.Client, labelSelectorMap, "openshift-sriov-network-operator")

	if err != nil {
		return err
	}

	for _, sn := range sriovNetworks.Items {
		//sn := &sriovNetworks.Items[idx]

		err = r.Client.Delete(context.Background(), &sn, &client.DeleteOptions{})

		if err != nil {
			return err
		}

		log.Info(fmt.Sprintf("SriovNetwork deleted: name %s - %s", sn.Name, sn.UID))
	}

	// Delete sriovnetworknodepolicies in openshift-sriov-network-operator namespace
	sriovNetworkNodePolicies, err := vmset.GetSriovNetworkNodePoliciesWithLabel(r.Client, labelSelectorMap, "openshift-sriov-network-operator")

	if err != nil {
		return err
	}

	for _, snnp := range sriovNetworkNodePolicies.Items {
		//snnp := &sriovNetworkNodePolicies.Items[idx]

		err = r.Client.Delete(context.Background(), &snnp, &client.DeleteOptions{})

		if err != nil {
			return err
		}

		log.Info(fmt.Sprintf("SriovNetworkNodePolicy deleted: name %s - %s", snnp.Name, snnp.UID))
	}

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
	sriovNetworkFn := handler.ToRequestsFunc(func(o handler.MapObject) []reconcile.Request {
		result := []reconcile.Request{}
		label := o.Meta.GetLabels()
		// verify object has ownerUIDLabelSelector
		if uid, ok := label[OwnerUIDLabelSelector]; ok {
			r.Log.Info(fmt.Sprintf("SriovNetwork object %s marked with OSP owner ref: %s", o.Meta.GetName(), uid))
			// return namespace and Name of CR
			name := client.ObjectKey{
				Namespace: label[OwnerNameSpaceLabelSelector],
				Name:      label[OwnerNameLabelSelector],
			}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		if len(result) > 0 {
			return result
		}
		return nil
	})

	sriovNetworkNodePolicyFn := handler.ToRequestsFunc(func(o handler.MapObject) []reconcile.Request {
		result := []reconcile.Request{}
		label := o.Meta.GetLabels()
		// verify object has ownerUIDLabelSelector
		if uid, ok := label[OwnerUIDLabelSelector]; ok {
			r.Log.Info(fmt.Sprintf("SriovNetworkNodePolicy object %s marked with OSP owner ref: %s", o.Meta.GetName(), uid))
			// return namespace and Name of CR
			name := client.ObjectKey{
				Namespace: label[OwnerNameSpaceLabelSelector],
				Name:      label[OwnerNameLabelSelector],
			}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		if len(result) > 0 {
			return result
		}
		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackVMSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&virtv1.VirtualMachine{}).
		Watches(&source.Kind{Type: &sriovnetworkv1.SriovNetwork{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: sriovNetworkFn,
			}).
		Watches(&source.Kind{Type: &sriovnetworkv1.SriovNetworkNodePolicy{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: sriovNetworkNodePolicyFn,
			}).
		Complete(r)
}

func (r *OpenStackVMSetReconciler) getRenderData(instance *ospdirectorv1beta1.OpenStackVMSet) (*bindatautil.RenderData, error) {
	data := bindatautil.MakeRenderData()
	// Base image used to clone the controller VM images from
	// adding first 5 char from instance.UID as identifier
	if instance.Spec.BaseImageVolumeName == "" {
		data.Data["BaseImageVolumeName"] = fmt.Sprintf("osp-vmset-baseimage-%s", instance.UID[0:4])
	} else {
		data.Data["BaseImageVolumeName"] = instance.Spec.BaseImageVolumeName
	}
	data.Data["BaseImageURL"] = instance.Spec.BaseImageURL
	data.Data["DiskSize"] = instance.Spec.DiskSize
	data.Data["Namespace"] = instance.Namespace
	data.Data["Cores"] = instance.Spec.Cores
	data.Data["Memory"] = instance.Spec.Memory
	data.Data["StorageClass"] = ""
	if instance.Spec.StorageClass != "" {
		data.Data["StorageClass"] = instance.Spec.StorageClass
	}
	data.Data["Network"] = instance.Spec.OSPNetwork.Name
	data.Data["BridgeName"] = instance.Spec.OSPNetwork.BridgeName
	// TODO: mschuppert - create NodeNetworkConfigurationPolicy not using unstructured ?
	data.Data["NodeNetworkConfigurationPolicyNodeSelector"] = instance.Spec.OSPNetwork.NodeNetworkConfigurationPolicy.NodeSelector
	data.Data["NodeNetworkConfigurationPolicyDesiredState"] = instance.Spec.OSPNetwork.NodeNetworkConfigurationPolicy.DesiredState.String()
	// SRIOV config
	data.Data["NodeSriovConfigurationPolicyNodeSelector"] = instance.Spec.OSPNetwork.NodeSriovConfigurationPolicy.NodeSelector
	data.Data["SriovPort"] = instance.Spec.OSPNetwork.NodeSriovConfigurationPolicy.DesiredState.Port
	data.Data["SriovRootDevice"] = instance.Spec.OSPNetwork.NodeSriovConfigurationPolicy.DesiredState.RootDevice
	data.Data["SriovMtu"] = instance.Spec.OSPNetwork.NodeSriovConfigurationPolicy.DesiredState.Mtu
	data.Data["SriovNumVfs"] = instance.Spec.OSPNetwork.NodeSriovConfigurationPolicy.DesiredState.NumVfs
	data.Data["SriovDeviceType"] = instance.Spec.OSPNetwork.NodeSriovConfigurationPolicy.DesiredState.DeviceType

	// get deployment user ssh pub key from Spec.DeploymentSSHSecret
	secret, _, err := common.GetSecret(r, instance.Spec.DeploymentSSHSecret, instance.Namespace)
	if err != nil {
		r.Log.Info("DeploymentSSHSecret secret not found!!")
		return nil, err
	}
	data.Data["AuthorizedKeys"] = secret.Data["authorized_keys"]

	// get deployment userdata from secret
	userdataSecret := fmt.Sprintf("%s-cloudinit", instance.Name)
	secret, _, err = common.GetSecret(r, userdataSecret, instance.Namespace)
	if err != nil {
		r.Log.Info("CloudInit userdata secret not found!!")
		return nil, err
	}
	data.Data["UserDataSecret"] = secret.Name

	return &data, nil
}

func (r *OpenStackVMSetReconciler) createNetworkConfig(instance *ospdirectorv1beta1.OpenStackVMSet) error {
	data, err := r.getRenderData(instance)
	if err != nil {
		return err
	}

	objs := []*uns.Unstructured{}
	oref := metav1.NewControllerRef(instance, instance.GroupVersionKind())

	// Labels for all objects
	labelSelector := map[string]string{
		OwnerUIDLabelSelector:       string(instance.UID),
		OwnerNameSpaceLabelSelector: instance.Namespace,
		OwnerNameLabelSelector:      instance.Name,
	}
	for k, v := range common.GetLabels(instance.Name, vmset.AppLabel) {
		labelSelector[k] = v
	}

	// Generate the Network Definition or SRIOV config objects
	manifestType := "network"

	if instance.Spec.OSPNetwork.NodeSriovConfigurationPolicy.DesiredState.Port != "" {
		manifestType = "sriov"
	}

	manifests, err := bindatautil.RenderDir(filepath.Join(ManifestPath, manifestType), data)
	if err != nil {
		r.Log.Error(err, "Failed to render manifests : %v")
		return err
	}
	objs = append(objs, manifests...)

	// Apply the objects to the cluster
	for _, obj := range objs {
		// Set owner reference on objects in the same namespace as the operator
		if obj.GetNamespace() == instance.Namespace {
			obj.SetOwnerReferences([]metav1.OwnerReference{*oref})
		}
		// merge owner ref label into labels on the objects
		obj.SetLabels(labels.Merge(obj.GetLabels(), labelSelector))

		if err := bindatautil.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			r.Log.Error(err, "Failed to apply objects")
			return err
		}
	}

	return nil
}

func (r *OpenStackVMSetReconciler) cdiCreateBaseDisk(instance *ospdirectorv1beta1.OpenStackVMSet) error {
	data, err := r.getRenderData(instance)
	if err != nil {
		return err
	}

	objs := []*uns.Unstructured{}
	oref := metav1.NewControllerRef(instance, instance.GroupVersionKind())

	// Labels for all objects
	labelSelector := map[string]string{
		OwnerUIDLabelSelector:       string(instance.UID),
		OwnerNameSpaceLabelSelector: instance.Namespace,
		OwnerNameLabelSelector:      instance.Name,
	}
	for k, v := range common.GetLabels(instance.Name, vmset.AppLabel) {
		labelSelector[k] = v
	}

	// 01 - create root base volume on StorageClass
	manifests, err := bindatautil.RenderDir(filepath.Join(ManifestPath, "cdi"), data)
	if err != nil {
		return err
	}
	objs = append(objs, manifests...)

	// Apply the objects to the cluster
	for _, obj := range objs {
		// Set owner reference on objects in the same namespace as the operator
		if obj.GetNamespace() == instance.Namespace {
			obj.SetOwnerReferences([]metav1.OwnerReference{*oref})
		}
		// merge owner ref label into labels on the objects
		obj.SetLabels(labels.Merge(obj.GetLabels(), labelSelector))

		if err := bindatautil.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			r.Log.Error(err, "Failed to apply objects")
			return err
		}
	}

	return nil
}

func (r *OpenStackVMSetReconciler) vmCreateInstance(instance *ospdirectorv1beta1.OpenStackVMSet, envVars map[string]common.EnvSetter, ctl *ospdirectorv1beta1.Host) error {
	data, err := r.getRenderData(instance)
	if err != nil {
		return err
	}

	objs := []*uns.Unstructured{}
	oref := metav1.NewControllerRef(instance, instance.GroupVersionKind())

	// Labels for all objects
	labelSelector := map[string]string{
		OwnerUIDLabelSelector:       string(instance.UID),
		OwnerNameSpaceLabelSelector: instance.Namespace,
		OwnerNameLabelSelector:      instance.Name,
	}
	for k, v := range common.GetLabels(instance.Name, vmset.AppLabel) {
		labelSelector[k] = v
	}

	// Generate the Contoller VM object
	data.Data["DomainName"] = ctl.DomainName
	data.Data["DomainNameUniq"] = ctl.DomainNameUniq
	data.Data["NetworkDataSecret"] = ctl.NetworkDataSecret

	manifests, err := bindatautil.RenderDir(filepath.Join(ManifestPath, "virtualmachine"), data)
	if err != nil {
		r.Log.Error(err, "Failed to render virtualmachine manifests : %v")
		return err
	}
	objs = append(objs, manifests...)

	// Apply the objects to the cluster
	for _, obj := range objs {
		// Set owner reference on objects in the same namespace as the operator
		if obj.GetNamespace() == instance.Namespace {
			obj.SetOwnerReferences([]metav1.OwnerReference{*oref})
		}
		// merge owner ref label into labels on the objects
		obj.SetLabels(labels.Merge(obj.GetLabels(), labelSelector))

		// Set finalizer to prevent unsanctioned deletion of VirtualMachine
		finalizers := append(obj.GetFinalizers(), vmset.VirtualMachineFinalizerName)
		obj.SetFinalizers(finalizers)

		//Todo: mschuppert log vm definition when debug?
		r.Log.Info(fmt.Sprintf("/n%s/n", obj))
		if err := bindatautil.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			r.Log.Error(err, "Failed to apply objects")
			return err
		}
	}

	return nil
}
