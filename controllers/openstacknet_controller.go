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
	"strconv"

	"github.com/go-logr/logr"
	"github.com/tidwall/gjson"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
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

	// If RoleReservations status map is nil, create it
	if instance.Status.RoleReservations == nil {
		instance.Status.RoleReservations = map[string]ospdirectorv1beta1.OpenStackNetRoleStatus{}
	}

	// If we have an SRIOV definition on this network, use that.  Otherwise assume the
	// non-SRIOV configuration must be present
	if instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.Port != "" {
		if err := r.ensureSriov(instance); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// generate NodeNetworkConfigurationPolicy
		ncp := common.NetworkConfigurationPolicy{
			Name:                           instance.Name,
			Labels:                         common.GetLabels(instance, openstacknet.AppLabel, map[string]string{}),
			NodeNetworkConfigurationPolicy: instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy,
		}
		err = common.CreateOrUpdateNetworkConfigurationPolicy(r, instance, instance.Kind, &ncp)
		if err != nil {
			return ctrl.Result{}, err
		}

		// create NetworkAttachmentDefinition
		// the desired state is raw json, marshal -> unmarshal to parse it
		desiredStateByte, err := json.Marshal(instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("Error marshal NodeNetworkConfigurationPolicy desired state: %v", instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy))
			return ctrl.Result{}, err
		}

		var desiredState map[string]json.RawMessage //interface{}
		err = json.Unmarshal(desiredStateByte, &desiredState)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("Error unmarshal NodeNetworkConfigurationPolicy desired state: %v", string(desiredStateByte)))
			return ctrl.Result{}, err
		}

		bridgeName := gjson.Get(string(desiredState["desiredState"]), "interfaces.#.name").Array()[0]
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
			return ctrl.Result{}, err
		}

		// create static nad used for openstackclient
		name := fmt.Sprintf("%s-static", instance.Name)
		nad.Name = name
		nad.Data["Name"] = name
		nad.Data["Static"] = "true"

		err = common.CreateOrUpdateNetworkAttachmentDefinition(r, instance, instance.Kind, metav1.NewControllerRef(instance, instance.GroupVersionKind()), &nad)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
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
