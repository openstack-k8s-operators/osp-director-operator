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
	"reflect"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/tidwall/gjson"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nmstateapi "github.com/nmstate/kubernetes-nmstate/api/shared"
	nmstatev1alpha1 "github.com/nmstate/kubernetes-nmstate/api/v1alpha1"
	sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
)

// OpenStackNetReconciler reconciles a OpenStackNet object
type OpenStackNetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackNetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackNetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackNetReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackNetReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,namespace=openstack,resources=deployments/finalizers,verbs=update
// FIXME: Cluster-scope required below for now, as the operator watches openshift-machine-api namespace as well
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=nmstate.io,resources=nodenetworkconfigurationpolicies,verbs=create;delete;get;list;patch;update;watch

// Reconcile -
func (r *OpenStackNetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("overcloudnet", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackNet{}
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

	// examine DeletionTimestamp to determine if object is under deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, openstacknet.FinalizerName) {
			controllerutil.AddFinalizer(instance, openstacknet.FinalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return ctrl.Result{}, err
			}
			r.Log.Info(fmt.Sprintf("Finalizer %s added to CR %s", openstacknet.FinalizerName, instance.Name))
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, openstacknet.FinalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. Clean up resources used by the operator
		// NNCP resources
		err = r.nncpResourceCleanup(instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		// SRIOV resources
		err = r.sriovResourceCleanup(instance)
		if err != nil {
			return ctrl.Result{}, err
		}

		// 3. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, openstacknet.FinalizerName)
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// Variables for setting state, which are needed in some cases to properly
	// set status within the conditional flow below
	var netState ospdirectorv1beta1.NetState
	var netStateMsg string

	// If RoleReservations status map is nil, create it
	if instance.Status.RoleReservations == nil {
		instance.Status.RoleReservations = map[string]ospdirectorv1beta1.OpenStackNetRoleStatus{}
	}

	// Used in comparisons below to determine whether a status update is actually needed
	oldStatus := instance.Status.DeepCopy()

	// If we have an SRIOV definition on this network, use that.  Otherwise assume the
	// non-SRIOV configuration must be present
	if instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.Port != "" {
		if err := r.ensureSriov(instance); err != nil {
			instance.Status.CurrentState = ospdirectorv1beta1.NetError
			_ = r.setStatus(instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}
	} else {

		//
		// get the desired state
		//
		// the desired state is raw json, marshal -> unmarshal to parse it
		desiredStateByte, err := json.Marshal(instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("Error marshal NodeNetworkConfigurationPolicy desired state: %v", instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy))
			instance.Status.CurrentState = ospdirectorv1beta1.NetError
			_ = r.setStatus(instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}

		var desiredState map[string]json.RawMessage //interface{}
		err = json.Unmarshal(desiredStateByte, &desiredState)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("Error unmarshal NodeNetworkConfigurationPolicy desired state: %v", string(desiredStateByte)))
			instance.Status.CurrentState = ospdirectorv1beta1.NetError
			_ = r.setStatus(instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}

		// get bridgeName from desiredState
		bridgeName := gjson.Get(string(desiredState["desiredState"]), "interfaces.#.name").Array()[0]

		// Don't add a uniq  label here because the NCP might be related to multiple networks/NetworkAttachmentDefinitions
		labelSelector := map[string]string{
			common.OwnerNameSpaceLabelSelector:      instance.Namespace,
			common.OwnerControllerNameLabelSelector: openstacknet.AppLabel,
		}

		// generate NodeNetworkConfigurationPolicy for the bridge
		ncp := common.NetworkConfigurationPolicy{
			Name:                           bridgeName.String(),
			Labels:                         labelSelector,
			NodeNetworkConfigurationPolicy: instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy,
		}
		err = common.CreateOrUpdateNetworkConfigurationPolicy(r, instance, instance.Kind, &ncp)
		if err != nil {
			instance.Status.CurrentState = ospdirectorv1beta1.NetError
			_ = r.setStatus(instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}

		// TODO: add finalizer to the NCP to prevent the bridge from being deleted if a network gets deleted

		if !controllerutil.ContainsFinalizer(instance, openstacknet.FinalizerName) {
			controllerutil.AddFinalizer(instance, openstacknet.FinalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return ctrl.Result{}, err
			}
			r.Log.Info(fmt.Sprintf("Finalizer %s added to CR %s", openstacknet.FinalizerName, instance.Name))
		}

		// create NetworkAttachmentDefinition
		vlan := strconv.Itoa(instance.Spec.Vlan)
		nad := common.NetworkAttachmentDefinition{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    common.GetLabels(instance, openstacknet.AppLabel, map[string]string{}),
			Data: map[string]string{
				"Name":       instance.Name,
				"BridgeName": bridgeName.String(),
				"Vlan":       vlan,
			},
		}

		// create nad
		err = common.CreateOrUpdateNetworkAttachmentDefinition(r, instance, instance.Kind, metav1.NewControllerRef(instance, instance.GroupVersionKind()), &nad)
		if err != nil {
			instance.Status.CurrentState = ospdirectorv1beta1.NetError
			_ = r.setStatus(instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}

		// create static nad used for openstackclient
		name := fmt.Sprintf("%s-static", instance.Name)
		nad.Name = name
		nad.Data["Name"] = name
		nad.Data["Static"] = "true"

		err = common.CreateOrUpdateNetworkAttachmentDefinition(r, instance, instance.Kind, metav1.NewControllerRef(instance, instance.GroupVersionKind()), &nad)
		if err != nil {
			instance.Status.CurrentState = ospdirectorv1beta1.NetError
			_ = r.setStatus(instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}

		// If we get this far, we assume the NAD been successfully created (NAD does not
		// have a status block we can examine), so now examine the status of the NNCP
		nncpList := &nmstatev1alpha1.NodeNetworkConfigurationPolicyList{}

		// Can't use "r.Client.Get" here, as the NNCP is cluster-scoped and does not have a namespace.  We require the
		// NNCP resource struct object in order to examine its status, but the NNCP was created earlier via a server-side
		// apply.  Thus we don't already have the struct object in memory, and must acquire it by asking for a list of
		// NNCPs filtered by our label selector (which should return just the one resource we seek)
		err = r.Client.List(context.TODO(), nncpList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelSelector),
		})

		if err != nil {
			instance.Status.CurrentState = ospdirectorv1beta1.NetError
			_ = r.setStatus(instance, oldStatus, err.Error())
			return ctrl.Result{}, err
		}

		if len(nncpList.Items) < 1 {
			msg := fmt.Sprintf("OpenStackNet %s associated NodeNetworkConfigurationPolicy is still initializing, requeuing...", instance.Name)
			instance.Status.CurrentState = ospdirectorv1beta1.NetInitializing
			err = r.setStatus(instance, oldStatus, msg)
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}

		nncp := nncpList.Items[0]

		netState = ospdirectorv1beta1.NetConfiguring
		netStateMsg = fmt.Sprintf("OpenStackNet %s is configuring targeted node(s)", instance.Name)

		for _, condition := range nncp.Status.Conditions {
			if condition.Status == corev1.ConditionTrue {
				if condition.Type == nmstateapi.NodeNetworkConfigurationPolicyConditionAvailable {
					netState = ospdirectorv1beta1.NetConfigured
					netStateMsg = fmt.Sprintf("OpenStackNet %s has been successfully configured on targeted node(s)", instance.Name)
					break
				} else if condition.Type == nmstateapi.NodeNetworkConfigurationPolicyConditionDegraded {
					instance.Status.CurrentState = ospdirectorv1beta1.NetError
					_ = r.setStatus(instance, oldStatus, fmt.Sprintf("Underlying NNCP error: %s", condition.Message))
					return ctrl.Result{}, err
				}
			}
		}
	}

	// Count IP assignments and update status
	totalIps := 0

	for _, roleIps := range instance.Status.RoleReservations {
		totalIps += len(roleIps.Reservations)
	}

	instance.Status.ReservedIPCount = totalIps
	instance.Status.CurrentState = netState

	err = r.setStatus(instance, oldStatus, netStateMsg)
	return ctrl.Result{}, err
}

func (r *OpenStackNetReconciler) setStatus(instance *ospdirectorv1beta1.OpenStackNet, oldStatus *ospdirectorv1beta1.OpenStackNetStatus, msg string) error {

	if msg != "" {
		r.Log.Info(msg)
	}

	if !reflect.DeepEqual(instance.Status, oldStatus) {
		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
		// TODO: Using msg as reason and message for now
		instance.Status.Conditions.Set(ospdirectorv1beta1.ConditionType(instance.Status.CurrentState), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(msg), msg)
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			r.Log.Error(err, "OpenStackNet update status error: %v")
			return err
		}
	}
	return nil
}

// SetupWithManager -
func (r *OpenStackNetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	sriovNetworkFn := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		label := o.GetLabels()
		// verify object has ownerUIDLabelSelector
		if uid, ok := label[common.OwnerUIDLabelSelector]; ok {
			r.Log.Info(fmt.Sprintf("SriovNetwork object %s marked with OSP owner ref: %s", o.GetName(), uid))
			// return namespace and Name of CR
			name := client.ObjectKey{
				Namespace: label[common.OwnerNameSpaceLabelSelector],
				Name:      label[common.OwnerNameLabelSelector],
			}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		if len(result) > 0 {
			return result
		}
		return nil
	})

	sriovNetworkNodePolicyFn := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		label := o.GetLabels()
		// verify object has ownerUIDLabelSelector
		if uid, ok := label[common.OwnerUIDLabelSelector]; ok {
			r.Log.Info(fmt.Sprintf("SriovNetworkNodePolicy object %s marked with OSP owner ref: %s", o.GetName(), uid))
			// return namespace and Name of CR
			name := client.ObjectKey{
				Namespace: label[common.OwnerNameSpaceLabelSelector],
				Name:      label[common.OwnerNameLabelSelector],
			}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		if len(result) > 0 {
			return result
		}
		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackNet{}).
		Owns(&networkv1.NetworkAttachmentDefinition{}).
		Owns(&nmstatev1alpha1.NodeNetworkConfigurationPolicy{}).
		Watches(&source.Kind{Type: &sriovnetworkv1.SriovNetwork{}}, sriovNetworkFn).
		Watches(&source.Kind{Type: &sriovnetworkv1.SriovNetworkNodePolicy{}}, sriovNetworkNodePolicyFn).
		Complete(r)
}

func (r *OpenStackNetReconciler) ensureSriov(instance *ospdirectorv1beta1.OpenStackNet) error {
	// Labels for all SRIOV objects
	labelSelector := common.GetLabels(instance, openstacknet.AppLabel, map[string]string{})

	sriovNet := &sriovnetworkv1.SriovNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sriov-network", instance.Name),
			Namespace: "openshift-sriov-network-operator",
			Labels:    labelSelector,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, sriovNet, func() error {
		sriovNet.Labels = common.GetLabels(instance, openstacknet.AppLabel, map[string]string{})
		sriovNet.Spec = sriovnetworkv1.SriovNetworkSpec{
			SpoofChk:         instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.SpoofCheck,
			Trust:            instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.Trust,
			ResourceName:     fmt.Sprintf("%s_sriovnics", instance.Name),
			NetworkNamespace: instance.Namespace,
		}

		return nil
	})

	if err != nil {
		return err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("SriovNetwork %s successfully reconciled - operation: %s", sriovNet.Name, string(op)))
	}

	sriovPolicy := &sriovnetworkv1.SriovNetworkNodePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sriov-policy", instance.Name),
			Namespace: "openshift-sriov-network-operator",
			Labels:    labelSelector,
		},
	}

	if instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.RootDevice != "" {
		sriovPolicy.Spec.NicSelector.RootDevices = []string{instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.RootDevice}
	}

	op, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, sriovPolicy, func() error {
		sriovPolicy.Labels = common.GetLabels(instance, openstacknet.AppLabel, map[string]string{})
		sriovPolicy.Spec = sriovnetworkv1.SriovNetworkNodePolicySpec{
			DeviceType: instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.DeviceType,
			Mtu:        int(instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.Mtu),
			NicSelector: sriovnetworkv1.SriovNetworkNicSelector{
				PfNames: []string{instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.Port},
			},
			NodeSelector: instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.NodeSelector,
			NumVfs:       int(instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.NumVfs),
			Priority:     5,
			ResourceName: fmt.Sprintf("%s_sriovnics", instance.Name),
		}

		if instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.RootDevice != "" {
			sriovPolicy.Spec.NicSelector.RootDevices = []string{instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.RootDevice}
		}

		return nil
	})

	if err != nil {
		return err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("SriovNetworkNodePolicy %s successfully reconciled - operation: %s", sriovPolicy.Name, string(op)))
	}

	return nil
}

func (r *OpenStackNetReconciler) nncpResourceCleanup(instance *ospdirectorv1beta1.OpenStackNet) error {
	// First check if any other OpenStackNets are left using the bridge name in the attachConfiguration
	osNetBridgeNames, err := openstacknet.GetOpenStackNetsAttachConfigBridgeNames(r, instance.Namespace)

	if err != nil {
		return err
	}

	if len(osNetBridgeNames) > 0 {
		found := false

		// Get this OpenStackNet's bridge name first
		myBridgeName := osNetBridgeNames[instance.Name]
		delete(osNetBridgeNames, instance.Name)

		if myBridgeName == "" {
			// If this instance does not have an associated bridge name, there's really nothing we can do
			return nil
		}

		for _, bridgeName := range osNetBridgeNames {
			if bridgeName == myBridgeName {
				found = true
				break
			}
		}

		if !found {
			// If no other OpenStackNet contains this instance's bridge name, delete the underlying NNCP
			labelSelector := map[string]string{
				common.OwnerControllerNameLabelSelector: openstacknet.AppLabel,
				common.OwnerNameSpaceLabelSelector:      instance.Namespace,
			}

			networkConfigurationPolicy := nmstatev1alpha1.NodeNetworkConfigurationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:   myBridgeName,
					Labels: labelSelector,
				},
			}

			if err := r.Client.Delete(context.TODO(), &networkConfigurationPolicy); err != nil && !k8s_errors.IsNotFound(err) {
				return err
			}

			r.Log.Info(fmt.Sprintf("NodeNetworkConfigurationPolicy is no longer required and has been deleted: %s", myBridgeName))
		}
	}

	return nil
}

func (r *OpenStackNetReconciler) sriovResourceCleanup(instance *ospdirectorv1beta1.OpenStackNet) error {
	labelSelectorMap := map[string]string{
		common.OwnerUIDLabelSelector:       string(instance.UID),
		common.OwnerNameSpaceLabelSelector: instance.Namespace,
		common.OwnerNameLabelSelector:      instance.Name,
	}

	// Delete sriovnetworks in openshift-sriov-network-operator namespace
	sriovNetworks, err := openstacknet.GetSriovNetworksWithLabel(r, labelSelectorMap, "openshift-sriov-network-operator")

	if err != nil {
		return err
	}

	for _, sn := range sriovNetworks {
		err = r.Client.Delete(context.Background(), &sn, &client.DeleteOptions{})

		if err != nil {
			return err
		}

		r.Log.Info(fmt.Sprintf("SriovNetwork deleted: name %s - %s", sn.Name, sn.UID))
	}

	// Delete sriovnetworknodepolicies in openshift-sriov-network-operator namespace
	sriovNetworkNodePolicies, err := openstacknet.GetSriovNetworkNodePoliciesWithLabel(r, labelSelectorMap, "openshift-sriov-network-operator")

	if err != nil {
		return err
	}

	for _, snnp := range sriovNetworkNodePolicies {
		err = r.Client.Delete(context.Background(), &snnp, &client.DeleteOptions{})

		if err != nil {
			return err
		}

		r.Log.Info(fmt.Sprintf("SriovNetworkNodePolicy deleted: name %s - %s", snnp.Name, snnp.UID))
	}

	return nil
}
