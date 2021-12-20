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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackipset"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackControlPlaneReconciler reconciles an OpenStackControlPlane object
type OpenStackControlPlaneReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackControlPlaneReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackControlPlaneReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackControlPlaneReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackControlPlaneReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackcontrolplanes/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackvmsets/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackmacaddresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hco.kubevirt.io,namespace=openstack,resources="*",verbs="*"
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;list;patch;update;watch

// Reconcile - control plane
func (r *OpenStackControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("controlplane", req.NamespacedName)

	// Fetch the controlplane instance
	instance := &ospdirectorv1beta1.OpenStackControlPlane{}
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
	cond := &ospdirectorv1beta1.Condition{}

	if instance.Status.VIPStatus == nil {
		instance.Status.VIPStatus = map[string]ospdirectorv1beta1.HostStatus{}
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.Status.ProvisioningStatus.State == ospdirectorv1beta1.ControlPlaneProvisioned)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		err = common.WrapErrorForObject(
			fmt.Sprintf("OpenStackControlPlane %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name),
			instance,
			err,
		)

		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
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

		instance.Status.ProvisioningStatus.Reason = cond.Message
		instance.Status.ProvisioningStatus.State = ospdirectorv1beta1.ControlPlaneProvisioningState(cond.Type)

		if statusChanged() {
			if updateErr := r.Client.Status().Update(context.Background(), instance); updateErr != nil {
				if err == nil {
					err = common.WrapErrorForObject(
						"Update Status", instance, updateErr)
				} else {
					common.LogErrorForObject(r, updateErr, "Update status", instance)
				}
			} else {
				// log status changed messages also to operator log
				common.LogForObject(r, cond.Message, instance)
			}
		}

	}(cond)

	/*
		// Used in comparisons below to determine whether a status update is actually needed
		newProvStatus := ospdirectorv1beta1.OpenStackControlPlaneProvisioningStatus{}

	*/

	envVars := make(map[string]common.EnvSetter)

	//
	// Set the OSP version, the version is usually set in the ctlplane webhook,
	// so this is mostly for when running local with no webhooks and no OpenStackRelease is provided
	//
	var OSPVersion ospdirectorv1beta1.OSPVersion
	if instance.Spec.OpenStackRelease != "" {
		OSPVersion, err = ospdirectorv1beta1.GetOSPVersion(instance.Spec.OpenStackRelease)
	} else {
		OSPVersion = ospdirectorv1beta1.OSPVersion(ospdirectorv1beta1.TemplateVersion16_2)
	}
	if err != nil {
		cond.Message = err.Error()
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ControlPlaneReasonNotSupportedVersion)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}
	instance.Status.OSPVersion = OSPVersion

	//
	// create or get hash of "tripleo-passwords" controlplane.TripleoPasswordSecret secret
	//
	err = r.createOrGetTripleoPasswords(instance, cond, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// Secret - used for deployment to ssh into the overcloud nodes,
	//          gets added to the controller VMs cloud-admin user using cloud-init
	//
	deploymentSecret, err := r.createOrGetDeploymentSecret(instance, cond, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// check if specified PasswordSecret secret exists
	//
	ctrlResult, err := r.verifySecretExist(instance, cond, instance.Spec.PasswordSecret)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// check if specified IdmSecret secret exists
	//
	ctrlResult, err = r.verifySecretExist(instance, cond, instance.Spec.IdmSecret)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// check if specified CAConfigMap config map exists
	//
	ctrlResult, err = r.verifyConfigMapExist(instance, cond, instance.Spec.CAConfigMap)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// create hostnames for the overcloud VIP
	//
	_, err = r.createNewHostnames(
		instance,
		cond,
		controlplane.Role,
		1-len(instance.Status.VIPStatus),
		true,
	)

	//
	// Create VIPs for networks where VIP parameter is true
	//

	// create list of networks where Spec.VIP == True
	vipNetworksList, err := r.createVIPNetworkList(instance, cond)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	//   Create/Update IPSet CR for the controlplane VIPs
	//
	_, ctrlResult, err = openstackipset.CreateOrUpdateIPset(
		r,
		instance,
		cond,
		common.IPSet{
			Networks:            vipNetworksList,
			Role:                controlplane.Role,
			HostCount:           controlplane.Count,
			VIP:                 true,
			AddToPredictableIPs: true,
			HostNameRefs:        instance.GetHostnames(),
		},
	)
	if (err != nil) || (ctrlResult != ctrl.Result{}) {
		return ctrlResult, err
	}

	//
	// Create VMSets
	//
	vmSets, err := r.createOrUpdateVMSets(
		instance,
		cond,
		deploymentSecret,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO:
	// - check vm container status and update CR.Status.VMsReady
	// - change CR.Status.VMs to be struct with name + Pod IP of the controllers

	//
	// Create or update the MACAddress CR object
	//
	err = r.createOrUpdateOpenStackMACAddress(
		instance,
		cond,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// Create openstack client pod
	//
	osc, err := r.createOrUpdateOpenStackClient(
		instance,
		cond,
		deploymentSecret,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// Calculate overall status
	//
	//var ctlPlaneState ospdirectorv1beta1.ControlPlaneProvisioningState

	// 1) OpenStackClient pod status
	clientPod, err := r.Kclient.CoreV1().Pods(instance.Namespace).Get(context.TODO(), osc.Name, metav1.GetOptions{})
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			timeout := 30
			cond.Message = fmt.Sprintf("%s pod %s not found, next reconcile in %d s", osc.Kind, osc.Name, timeout)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPodMissing)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)

			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
		}

		cond.Message = fmt.Sprintf("%s pod %s error", osc.Kind, osc.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPodError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	instance.Status.ProvisioningStatus.ClientReady = (clientPod != nil && clientPod.Status.Phase == corev1.PodRunning)

	// 2) OpenStackVMSet status
	vmSetStateCounts := map[ospdirectorv1beta1.VMSetProvisioningState]int{}
	for _, vmSet := range vmSets {
		if vmSet.Status.ProvisioningStatus.State == ospdirectorv1beta1.VMSetCondTypeError {
			// An error overrides all aggregrate state considerations
			vmSetCondition := vmSet.Status.Conditions.GetCurrentCondition()
			cond.Message = fmt.Sprintf("Underlying OSVMSet %s hit an error: %s", vmSet.Name, vmSet.Status.ProvisioningStatus.Reason)
			cond.Reason = vmSetCondition.Reason
			cond.Type = vmSetCondition.Type

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		vmSetStateCounts[vmSet.Status.ProvisioningStatus.State]++
	}

	instance.Status.ProvisioningStatus.DesiredCount = len(instance.Spec.VirtualMachineRoles)
	instance.Status.ProvisioningStatus.ReadyCount =
		vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeProvisioned] +
			vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeEmpty]

	// TODO?: Currently considering states in an arbitrary order of priority here...
	if vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeProvisioning] > 0 {
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ControlPlaneProvisioning)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonProvisioning)
		cond.Message = "One or more OSVMSets are provisioning"
	} else if vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeDeprovisioning] > 0 {
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ControlPlaneDeprovisioning)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonDeprovisioning)
		cond.Message = "One or more OSVMSets are deprovisioning"
	} else if vmSetStateCounts[ospdirectorv1beta1.VMSetCondTypeWaiting] > 0 || vmSetStateCounts[""] > 0 {
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ControlPlaneWaiting)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonInitialize)
		cond.Message = "Waiting on one or more OSVMSets to initialize or continue"
	} else {
		// If we get here, the only states possible for the VMSets are provisioned or empty,
		// which both count as provisioned
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.ControlPlaneProvisioned)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonProvisioned)
		cond.Message = "All requested OSVMSets have been provisioned"
	}

	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *OpenStackControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// watch for objects in the same namespace as the controller CR
	podWatcher := handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// verify if pods label match any of:
		// osp-director.openstack.org/controller: osp-vmset
		// osp-director.openstack.org/controller: osp-openstackclient
		controllers := map[string]bool{
			vmset.AppLabel:           true,
			openstackclient.AppLabel: true,
		}
		labels := obj.GetLabels()
		controller, ok := labels[common.OwnerControllerNameLabelSelector]
		if ok || controllers[controller] {
			// get all CRs from the same namespace
			crs := &ospdirectorv1beta1.OpenStackControlPlaneList{}
			listOpts := []client.ListOption{
				client.InNamespace(obj.GetNamespace()),
			}
			if err := r.Client.List(context.Background(), crs, listOpts...); err != nil {
				r.Log.Error(err, "Unable to retrieve CRs %v")
				return nil
			}

			for _, cr := range crs.Items {
				if obj.GetNamespace() == cr.Namespace {
					// return namespace and Name of CR
					name := client.ObjectKey{
						Namespace: cr.Namespace,
						Name:      cr.Name,
					}
					result = append(result, reconcile.Request{NamespacedName: name})
				}
			}
		}
		return result
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackControlPlane{}).
		Owns(&corev1.Secret{}).
		Owns(&ospdirectorv1beta1.OpenStackIPSet{}).
		Owns(&ospdirectorv1beta1.OpenStackVMSet{}).
		Owns(&ospdirectorv1beta1.OpenStackClient{}).
		Owns(&ospdirectorv1beta1.OpenStackMACAddress{}).
		// watch vmset and openstackclient pods in the same namespace
		// as we want to reconcile if VMs or openstack client pods change
		Watches(&source.Kind{Type: &corev1.Pod{}}, podWatcher).
		Complete(r)
}

func (r *OpenStackControlPlaneReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackControlPlaneStatus) *ospdirectorv1beta1.OpenStackControlPlaneStatus {

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

/*
func (r *OpenStackControlPlaneReconciler) setNetStatus(instance *ospdirectorv1beta1.OpenStackControlPlane, hostnameDetails *common.Hostname, netName string, ipaddress string) {

	// If OpenStackControlPlane status map is nil, create it
	if instance.Status.VIPStatus == nil {
		instance.Status.VIPStatus = map[string]ospdirectorv1beta1.HostStatus{}
	}

	// Set network information status
	if instance.Status.VIPStatus[hostnameDetails.Hostname].IPAddresses == nil {
		instance.Status.VIPStatus[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
			Hostname: hostnameDetails.Hostname,
			HostRef:  strings.ToLower(instance.Name),
			IPAddresses: map[string]string{
				netName: ipaddress,
			},
		}
	} else {
		status := instance.Status.VIPStatus[hostnameDetails.Hostname]
		status.HostRef = strings.ToLower(instance.Name)
		status.IPAddresses[netName] = ipaddress
		instance.Status.VIPStatus[hostnameDetails.Hostname] = status
	}

}


func (r *OpenStackControlPlaneReconciler) setProvisioningStatus(instance *ospdirectorv1beta1.OpenStackControlPlane, newProvStatus ospdirectorv1beta1.OpenStackControlPlaneProvisioningStatus) error {

	// if the current ProvisioningStatus is different from the actual, store the update
	// otherwise, just log the status again
	if !reflect.DeepEqual(instance.Status.ProvisioningStatus, newProvStatus) {
		r.Log.Info(fmt.Sprintf("%s - diff %s", instance.Status.ProvisioningStatus.Reason, diff.ObjectReflectDiff(instance.Status.ProvisioningStatus, newProvStatus)))
		instance.Status.ProvisioningStatus = newProvStatus

		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
		instance.Status.Conditions.Set(ospdirectorv1beta1.ConditionType(newProvStatus.State), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(newProvStatus.Reason), newProvStatus.Reason)

		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			r.Log.Error(err, "Failed to update CR status %v")
			return err
		}
	} else if newProvStatus.Reason != "" {
		r.Log.Info(newProvStatus.Reason)
	}

	return nil
}
*/

func (r *OpenStackControlPlaneReconciler) createVIPNetworkList(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
) ([]string, error) {

	// create uniq list networls of all VirtualMachineRoles
	networkList := make(map[string]bool)
	uniqNetworksList := []string{}

	for _, vmRole := range instance.Spec.VirtualMachineRoles {
		for _, netNameLower := range vmRole.Networks {
			// get network with name_lower label
			labelSelector := map[string]string{
				openstacknet.SubNetNameLabelSelector: netNameLower,
			}

			// get network with name_lower label to verify if VIP needs to be requested from Spec
			network, err := openstacknet.GetOpenStackNetWithLabel(
				r,
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
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonOSNetError)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return uniqNetworksList, err
			}

			if _, value := networkList[netNameLower]; !value && network.Spec.VIP {
				networkList[netNameLower] = true
				uniqNetworksList = append(uniqNetworksList, netNameLower)
			}
		}
	}

	return uniqNetworksList, nil
}

func (r *OpenStackControlPlaneReconciler) createOrUpdateOpenStackMACAddress(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
) error {

	mac := &ospdirectorv1beta1.OpenStackMACAddress{
		ObjectMeta: metav1.ObjectMeta{
			// use the role name as the VM CR name
			Name:      strings.ToLower(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, mac, func() error {
		if len(instance.Spec.PhysNetworks) == 0 {
			mac.Spec.PhysNetworks = []ospdirectorv1beta1.Physnet{
				{
					Name:      controlplane.DefaultOVNChassisPhysNetName,
					MACPrefix: controlplane.DefaultOVNChassisPhysNetMACPrefix,
				},
			}
		} else {
			macPhysnets := []ospdirectorv1beta1.Physnet{}
			for _, physnet := range instance.Spec.PhysNetworks {
				macPrefix := physnet.MACPrefix
				// make sure if MACPrefix was not speficied to set the default prefix
				if macPrefix == "" {
					macPrefix = controlplane.DefaultOVNChassisPhysNetMACPrefix
				}
				macPhysnets = append(macPhysnets, ospdirectorv1beta1.Physnet{
					Name:      physnet.Name,
					MACPrefix: macPrefix,
				})
			}

			mac.Spec.PhysNetworks = macPhysnets
		}

		err := controllerutil.SetControllerReference(instance, mac, r.Scheme)
		if err != nil {
			cond.Message = fmt.Sprintf("Error set controller reference for %s", mac.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonControllerReferenceError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update OpenStackMACAddress %s ", instance.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonCreateMACError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}
	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("OpenStackMACAddress CR successfully reconciled - operation: %s", string(op))
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonPodProvisioned)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)
	}

	return nil
}

//
// create or get hash of "tripleo-passwords" controlplane.TripleoPasswordSecret secret
//
func (r *OpenStackControlPlaneReconciler) createOrGetTripleoPasswords(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
	envVars *map[string]common.EnvSetter,
) error {
	//
	// check if "tripleo-passwords" controlplane.TripleoPasswordSecret secret already exist
	//
	_, secretHash, err := common.GetSecret(r, controlplane.TripleoPasswordSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {

			common.LogForObject(
				r,
				fmt.Sprintf("Creating password secret: %s", controlplane.TripleoPasswordSecret),
				instance,
			)

			pwSecretLabel := common.GetLabels(instance, controlplane.AppLabel, map[string]string{})

			templateParameters := make(map[string]interface{})
			templateParameters["TripleoPasswords"] = common.GeneratePasswords()
			pwSecret := []common.Template{
				{
					Name:               controlplane.TripleoPasswordSecret,
					Namespace:          instance.Namespace,
					Type:               common.TemplateTypeConfig,
					InstanceType:       instance.Kind,
					AdditionalTemplate: map[string]string{},
					Labels:             pwSecretLabel,
					ConfigOptions:      templateParameters,
				},
			}

			err = common.EnsureSecrets(r, instance, pwSecret, envVars)
			if err != nil {
				cond.Message = fmt.Sprintf("Error creating TripleoPasswordsSecret %s", controlplane.TripleoPasswordSecret)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ControlPlaneReasonTripleoPasswordsSecretCreateError)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}
		} else if err != nil {
			cond.Message = fmt.Sprintf("Error get TripleoPasswordsSecret %s", controlplane.TripleoPasswordSecret)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ControlPlaneReasonTripleoPasswordsSecretError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}
	}

	(*envVars)[controlplane.TripleoPasswordSecret] = common.EnvValue(secretHash)

	return nil
}

//
// Secret - used for deployment to ssh into the overcloud nodes,
//          gets added to the controller VMs cloud-admin user using cloud-init
//
func (r *OpenStackControlPlaneReconciler) createOrGetDeploymentSecret(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
	envVars *map[string]common.EnvSetter,
) (*corev1.Secret, error) {
	deploymentSecretName := strings.ToLower(controlplane.AppLabel) + "-ssh-keys"

	deploymentSecret, secretHash, err := common.GetSecret(r, deploymentSecretName, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {

			var op controllerutil.OperationResult

			common.LogForObject(
				r,
				fmt.Sprintf("Creating deployment ssh secret: %s", deploymentSecretName),
				instance,
			)

			deploymentSecret, err = common.SSHKeySecret(
				deploymentSecretName,
				instance.Namespace,
				map[string]string{deploymentSecretName: ""},
			)
			if err != nil {
				cond.Message = fmt.Sprintf("Error creating ssh keys %s", deploymentSecretName)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ControlPlaneReasonDeploymentSSHKeysGenError)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return deploymentSecret, err
			}

			secretHash, op, err = common.CreateOrUpdateSecret(r, instance, deploymentSecret)
			if err != nil {
				cond.Message = fmt.Sprintf("Error create or update ssh keys secret %s", deploymentSecretName)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.ControlPlaneReasonDeploymentSSHKeysSecretCreateOrUpdateError)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return deploymentSecret, err
			}
			if op != controllerutil.OperationResultNone {
				common.LogForObject(
					r,
					fmt.Sprintf("Secret %s successfully reconciled - operation: %s", deploymentSecret.Name, string(op)),
					instance,
				)
			}
		} else {
			cond.Message = fmt.Sprintf("Error get secret %s", deploymentSecretName)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonDeploymentSecretError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return deploymentSecret, err
		}
	}
	(*envVars)[deploymentSecret.Name] = common.EnvValue(secretHash)

	return deploymentSecret, nil
}

func (r *OpenStackControlPlaneReconciler) verifySecretExist(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
	secretName string,
) (ctrl.Result, error) {
	if secretName != "" {
		// check if specified secret exists before creating the controlplane
		_, _, err := common.GetSecret(r, secretName, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				timeout := 30
				cond.Message = fmt.Sprintf("Secret %s not found but specified in CR, next reconcile in %d s", secretName, timeout)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonSecretMissing)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)

				common.LogForObject(r, cond.Message, instance)

				return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error reading secret object: %s", secretName)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonSecretError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}

		common.LogForObject(
			r,
			fmt.Sprintf("Secret %s exists", secretName),
			instance,
		)
	}
	return ctrl.Result{}, nil
}

func (r *OpenStackControlPlaneReconciler) verifyConfigMapExist(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
	configMapName string,
) (ctrl.Result, error) {

	if configMapName != "" {
		_, _, err := common.GetConfigMapAndHashWithName(r, instance.Spec.CAConfigMap, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				timeout := 30
				cond.Message = fmt.Sprintf("ConfigMap %s not found but specified in CR, next reconcile in %d s", configMapName, timeout)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonConfigMapMissing)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeWaiting)

				common.LogForObject(r, cond.Message, instance)

				return ctrl.Result{RequeueAfter: time.Duration(timeout) * time.Second}, nil
			}
			// Error reading the object - requeue the request.
			cond.Message = fmt.Sprintf("Error reading config map object: %s", configMapName)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonConfigMapError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		common.LogForObject(
			r,
			fmt.Sprintf("Secret %s exists", configMapName),
			instance,
		)
	}
	return ctrl.Result{}, nil
}

//
// create hostnames for the requested number of systems
//
func (r *OpenStackControlPlaneReconciler) createNewHostnames(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
	baseName string,
	newCount int,
	vip bool,
) ([]string, error) {
	newVMs := []string{}

	//
	//   create hostnames for the newCount
	//
	currentNetStatus := instance.Status.DeepCopy().VIPStatus
	for i := 0; i < newCount; i++ {
		hostnameDetails := common.Hostname{
			Basename: baseName,
			VIP:      vip,
		}

		err := common.CreateOrGetHostname(instance, &hostnameDetails)
		if err != nil {
			cond.Message = fmt.Sprintf("error creating new hostname %v", hostnameDetails)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonNewHostnameError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return newVMs, err
		}

		if hostnameDetails.Hostname != "" {
			if _, ok := instance.Status.VIPStatus[hostnameDetails.Hostname]; !ok {
				instance.Status.VIPStatus[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
					Hostname:             hostnameDetails.Hostname,
					HostRef:              hostnameDetails.HostRef,
					AnnotatedForDeletion: false,
					IPAddresses:          map[string]string{},
				}
				newVMs = append(newVMs, hostnameDetails.Hostname)
			}

			common.LogForObject(
				r,
				fmt.Sprintf("%s hostname created: %s", instance.Kind, hostnameDetails.Hostname),
				instance,
			)
		}
	}

	if !reflect.DeepEqual(currentNetStatus, instance.Status.VIPStatus) {
		common.LogForObject(
			r,
			fmt.Sprintf("Updating CR status with new hostname information, %d new - %s",
				len(newVMs),
				diff.ObjectReflectDiff(currentNetStatus, instance.Status.VIPStatus),
			),
			instance,
		)

		err := r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			cond.Message = "Failed to update CR status for new hostnames"
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonCRStatusUpdateError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return newVMs, err
		}
	}

	return newVMs, nil
}

//
// Create VMSets
//
func (r *OpenStackControlPlaneReconciler) createOrUpdateVMSets(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
	deploymentSecret *corev1.Secret,
) ([]*ospdirectorv1beta1.OpenStackVMSet, error) {
	vmSets := []*ospdirectorv1beta1.OpenStackVMSet{}

	for _, vmRole := range instance.Spec.VirtualMachineRoles {

		/*
			actualStatus := instance.Status
			if !reflect.DeepEqual(currentStatus, &actualStatus) {
				err := r.Client.Status().Update(context.TODO(), instance)
				if err != nil {
					r.Log.Error(err, "Failed to update CR status %v")
					return err
				}

				common.LogForObject(
					r,
					fmt.Sprintf("VIP network status for Hostname: %s - %s",
					 instance.Status.VIPStatus[hostnameDetails.Hostname].Hostname,
					 instance.Status.VIPStatus[hostnameDetails.Hostname].IPAddresses,
					),

					instance,
				)
			}
		*/

		//
		// Create or update the vmSet CR object
		//
		vmSet := &ospdirectorv1beta1.OpenStackVMSet{
			ObjectMeta: metav1.ObjectMeta{
				// use the role name as the VM CR name
				Name:      strings.ToLower(vmRole.RoleName),
				Namespace: instance.Namespace,
			},
		}

		op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, vmSet, func() error {
			vmSet.Spec.VMCount = vmRole.RoleCount
			vmSet.Spec.Cores = vmRole.Cores
			vmSet.Spec.Memory = vmRole.Memory
			vmSet.Spec.DiskSize = vmRole.DiskSize
			if instance.Spec.DomainName != "" {
				vmSet.Spec.DomainName = instance.Spec.DomainName
			}
			vmSet.Spec.BootstrapDNS = instance.Spec.DNSServers
			vmSet.Spec.DNSSearchDomains = instance.Spec.DNSSearchDomains
			if vmRole.StorageClass != "" {
				vmSet.Spec.StorageClass = vmRole.StorageClass
			}
			vmSet.Spec.BaseImageVolumeName = vmRole.DeepCopy().BaseImageVolumeName
			vmSet.Spec.DeploymentSSHSecret = deploymentSecret.Name
			vmSet.Spec.CtlplaneInterface = vmRole.CtlplaneInterface
			vmSet.Spec.Networks = vmRole.Networks
			vmSet.Spec.RoleName = vmRole.RoleName
			vmSet.Spec.IsTripleoRole = vmRole.IsTripleoRole
			if instance.Spec.PasswordSecret != "" {
				vmSet.Spec.PasswordSecret = instance.Spec.PasswordSecret
			}

			err := controllerutil.SetControllerReference(instance, vmSet, r.Scheme)
			if err != nil {
				cond.Message = fmt.Sprintf("Error set controller reference for %s %s", vmSet.Kind, vmSet.Name)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonControllerReferenceError)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			return nil
		})

		if err != nil {
			cond.Message = fmt.Sprintf("Failed to create or update %s %s ", vmSet.Kind, vmSet.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return vmSets, err
		}
		if op != controllerutil.OperationResultNone {
			cond.Message = fmt.Sprintf("%s %s operation: %s", vmSet.Kind, vmSet.Name, string(op))
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.VMSetCondReasonProvisioned)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)

			common.LogForObject(
				r,
				fmt.Sprintf("VMSet CR %s successfully reconciled - operation: %s", vmSet.Name, string(op)),
				instance,
			)

		}
		vmSets = append(vmSets, vmSet)
	}

	return vmSets, nil
}

func (r *OpenStackControlPlaneReconciler) createOrUpdateOpenStackClient(
	instance *ospdirectorv1beta1.OpenStackControlPlane,
	cond *ospdirectorv1beta1.Condition,
	deploymentSecret *corev1.Secret,
) (*ospdirectorv1beta1.OpenStackClient, error) {
	osc := &ospdirectorv1beta1.OpenStackClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstackclient",
			Namespace: instance.Namespace,
		},
	}
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, osc, func() error {
		osc.Spec.ImageURL = instance.Spec.OpenStackClientImageURL
		osc.Spec.DeploymentSSHSecret = deploymentSecret.Name
		osc.Spec.CloudName = instance.Name
		osc.Spec.StorageClass = instance.Spec.OpenStackClientStorageClass
		osc.Spec.GitSecret = instance.Spec.GitSecret
		osc.Spec.RunUID = openstackclient.CloudAdminUID
		osc.Spec.RunGID = openstackclient.CloudAdminGID
		if instance.Spec.DomainName != "" {
			osc.Spec.DomainName = instance.Spec.DomainName
		}
		osc.Spec.DNSServers = instance.Spec.DNSServers
		osc.Spec.DNSSearchDomains = instance.Spec.DNSSearchDomains
		if instance.Spec.IdmSecret != "" {
			osc.Spec.IdmSecret = instance.Spec.IdmSecret
		}
		if instance.Spec.CAConfigMap != "" {
			osc.Spec.CAConfigMap = instance.Spec.CAConfigMap
		}

		if len(instance.Spec.OpenStackClientNetworks) > 0 {
			osc.Spec.Networks = instance.Spec.OpenStackClientNetworks
		}

		err := controllerutil.SetControllerReference(instance, osc, r.Scheme)
		if err != nil {
			cond.Message = fmt.Sprintf("Error set controller reference for %s %s", osc.Kind, osc.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonControllerReferenceError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", osc.Kind, osc.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return osc, err
	}
	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s %s operation: %s", osc.Kind, osc.Name, string(op))
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.OsClientCondReasonProvisioned)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)
	}

	return osc, nil
}
