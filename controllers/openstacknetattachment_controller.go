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
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	nmstateshared "github.com/nmstate/kubernetes-nmstate/api/shared"
	nmstatev1alpha1 "github.com/nmstate/kubernetes-nmstate/api/v1alpha1"
	sriovnetworkv1 "github.com/openshift/sriov-network-operator/api/v1"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	nmstate "github.com/openstack-k8s-operators/osp-director-operator/pkg/nmstate"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	openstacknetattachment "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetattachment"
)

// OpenStackNetAttachmentReconciler reconciles a OpenStackNetAttachment object
type OpenStackNetAttachmentReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackNetAttachmentReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackNetAttachmentReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackNetAttachmentReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackNetAttachmentReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetattachments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetattachments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetattachments/finalizers,verbs=update
// FIXME: Cluster-scope required below for now, as the operator watches openshift-machine-api namespace as well
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=nmstate.io,resources=nodenetworkconfigurationpolicies,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworknodepolicies,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworks,verbs=create;delete;get;list;patch;update;watch

// Reconcile -
func (r *OpenStackNetAttachmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstacknetattachment", req.NamespacedName)

	// Fetch the OpenStackNetAttachment instance
	instance := &ospdirectorv1beta1.OpenStackNetAttachment{}
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
		instance.Status.CurrentState = ospdirectorv1beta1.ProvisioningState(cond.Type)

		// TODO, we should set some proper cond.Reason type
		instance.Status.Conditions.UpdateCurrentCondition(
			cond.Type,
			ospdirectorv1beta1.ConditionReason(cond.Message),
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

	// examine DeletionTimestamp to determine if object is under deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, openstacknetattachment.FinalizerName) {
			controllerutil.AddFinalizer(instance, openstacknetattachment.FinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}

			common.LogForObject(r, fmt.Sprintf("Finalizer %s added to CR %s", openstacknetattachment.FinalizerName, instance.Name), instance)
		}
	} else {
		//
		// 1. check if finalizer is there
		//
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, openstacknetattachment.FinalizerName) {
			return ctrl.Result{}, nil
		}

		//
		// 2. Clean up resources used by the operator
		///
		// NNCP resources
		ctrlResult, err := r.cleanupNodeNetworkConfigurationPolicy(ctx, instance, cond)
		if err != nil && !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		} else if !reflect.DeepEqual(ctrlResult, ctrl.Result{}) {
			return ctrlResult, nil
		}
		// SRIOV resources
		err = r.sriovResourceCleanup(ctx, instance)
		if err != nil && !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		//
		// 3. as last step remove the finalizer on the operator CR to finish delete
		//
		controllerutil.RemoveFinalizer(instance, openstacknetattachment.FinalizerName)
		err = r.Update(ctx, instance)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonRemoveFinalizerError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		common.LogForObject(r, fmt.Sprintf("CR %s deleted", instance.Name), instance)

		return ctrl.Result{}, nil
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := common.OpenStackBackupOverridesReconcile(r.Client, instance)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackNetAttachment %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	// TODO: mschuppert not tested yet sriov with new CRDs
	if instance.Spec.AttachConfiguration.NodeSriovConfigurationPolicy.DesiredState.Port != "" {
		//
		// SRIOV
		//
		if err := r.ensureSriov(ctx, instance); err != nil {
			cond.Message = fmt.Sprintf("OpenStackNetAttach %s encountered an error configuring NodeSriovConfigurationPolicy", instance.Name)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError)
			return ctrl.Result{}, err
		}

		instance.Status.AttachType = ospdirectorv1beta1.AttachTypeSriov
	} else {
		//
		// Set/update CR status from NNCP status
		//
		if err = r.getNodeNetworkConfigurationPolicyStatus(ctx, instance, cond); err != nil && !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		//
		// if no NNCP was found, or the NNCP is in SuccessfullyConfigured -> create or update nncp
		//
		if k8s_errors.IsNotFound(err) ||
			cond.Reason == ospdirectorv1beta1.ConditionReason(nmstateshared.NodeNetworkConfigurationPolicyConditionSuccessfullyConfigured) {
			//
			// Create/update Bridge
			//
			if err := r.createOrUpdateNodeNetworkConfigurationPolicy(ctx, instance, cond); err != nil {
				cond.Message = fmt.Sprintf("OpenStackNetAttach %s encountered an error configuring NodeNetworkConfigurationPolicy", instance.Name)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError)

				return ctrl.Result{}, err
			}

			instance.Status.AttachType = ospdirectorv1beta1.AttachTypeBridge
		} else {
			common.LogForObject(r, fmt.Sprintf("NNCP %s config in progress waiting to be in %s state",
				instance.Status.BridgeName,
				nmstateshared.NodeNetworkConfigurationPolicyConditionSuccessfullyConfigured),
				instance)

			return ctrl.Result{RequeueAfter: time.Second * 20}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *OpenStackNetAttachmentReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackNetAttachmentStatus) *ospdirectorv1beta1.OpenStackNetAttachmentStatus {

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

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackNetAttachmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//
	// Schedule reconcile on openstacknetattachment if any of the global cluster objects
	// (nncp/sriov) change
	//
	ownerLabelWatcher := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		labels := o.GetLabels()
		//
		// verify object has OwnerNameLabelSelector
		//
		owner, ok := labels[common.OwnerNameLabelSelector]
		if !ok {
			return []reconcile.Request{}
		}
		namespace := labels[common.OwnerNameSpaceLabelSelector]
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{
				Name:      owner,
				Namespace: namespace,
			}},
		}
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackNetAttachment{}).
		Watches(&source.Kind{Type: &nmstatev1alpha1.NodeNetworkConfigurationPolicy{}}, ownerLabelWatcher).
		Watches(&source.Kind{Type: &sriovnetworkv1.SriovNetwork{}}, ownerLabelWatcher).
		Watches(&source.Kind{Type: &sriovnetworkv1.SriovNetworkNodePolicy{}}, ownerLabelWatcher).
		Complete(r)
}

// createOrUpdateNetworkConfigurationPolicy - create or update NetworkConfigurationPolicy
func (r *OpenStackNetAttachmentReconciler) createOrUpdateNodeNetworkConfigurationPolicy(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetAttachment,
	cond *ospdirectorv1beta1.Condition,
) error {
	//
	// get bridgeName from desiredState
	//
	bridgeName, err := nmstate.GetDesiredStateBridgeName(instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy.DesiredState.Raw)
	if err != nil {
		cond.Message = fmt.Sprintf("Error get bridge name from NetworkConfigurationPolicy desired state - %s", instance.Name)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError)

		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	networkConfigurationPolicy := &nmstatev1alpha1.NodeNetworkConfigurationPolicy{}
	networkConfigurationPolicy.Name = bridgeName

	// set bridgeName to instance status to be able to consume information from there
	instance.Status.BridgeName = bridgeName

	apply := func() error {
		ospdirectorv1beta1.InitMap(&networkConfigurationPolicy.Labels)
		networkConfigurationPolicy.Labels[common.OwnerUIDLabelSelector] = string(instance.UID)
		networkConfigurationPolicy.Labels[common.OwnerNameLabelSelector] = instance.Name
		networkConfigurationPolicy.Labels[common.OwnerNameSpaceLabelSelector] = instance.Namespace
		networkConfigurationPolicy.Labels[common.OwnerControllerNameLabelSelector] = openstacknetattachment.AppLabel
		networkConfigurationPolicy.Labels[openstacknetattachment.BridgeLabel] = bridgeName

		networkConfigurationPolicy.Spec = instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy

		return nil
	}

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, networkConfigurationPolicy, apply)
	if err != nil {
		cond.Message = fmt.Sprintf("Updating %s networkConfigurationPolicy", bridgeName)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError)

		err = common.WrapErrorForObject(cond.Message, networkConfigurationPolicy, err)

		return err
	}

	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("NodeNetworkConfigurationPolicy %s is %s", networkConfigurationPolicy.Name, string(op))
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachConfiguring)

		common.LogForObject(r, string(op), networkConfigurationPolicy)
		common.LogForObject(r, cond.Message, instance)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(networkConfigurationPolicy, openstacknetattachment.FinalizerName) {
			controllerutil.AddFinalizer(networkConfigurationPolicy, openstacknetattachment.FinalizerName)
			if err := r.Update(ctx, networkConfigurationPolicy); err != nil {
				return err
			}
			common.LogForObject(r, fmt.Sprintf("Finalizer %s added to %s", openstacknetattachment.FinalizerName, networkConfigurationPolicy.Name), instance)
		}
	}

	return nil
}

func (r *OpenStackNetAttachmentReconciler) getNodeNetworkConfigurationPolicyStatus(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetAttachment,
	cond *ospdirectorv1beta1.Condition,
) error {
	networkConfigurationPolicy := &nmstatev1alpha1.NodeNetworkConfigurationPolicy{}

	err := r.Get(ctx, types.NamespacedName{Name: instance.Status.BridgeName}, networkConfigurationPolicy)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get %s %s ", networkConfigurationPolicy.Kind, networkConfigurationPolicy.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonNNCPError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError)
		err = common.WrapErrorForObject(cond.Message, instance, err)
		return err
	}

	cond.Message = fmt.Sprintf("%s %s is configuring targeted node(s)", networkConfigurationPolicy.Kind, networkConfigurationPolicy.Name)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachConfiguring)

	//
	// sync latest status of the nncp object to the osnetattach
	//
	if networkConfigurationPolicy.Status.Conditions != nil && len(networkConfigurationPolicy.Status.Conditions) > 0 {
		condition := nmstate.GetCurrentCondition(networkConfigurationPolicy.Status.Conditions)
		if condition != nil {
			cond.Message = fmt.Sprintf("%s %s: %s", networkConfigurationPolicy.Kind, networkConfigurationPolicy.Name, condition.Message)
			cond.Reason = ospdirectorv1beta1.ConditionReason(condition.Reason)
			cond.Type = ospdirectorv1beta1.ConditionType(condition.Type)

			if condition.Type == nmstateshared.NodeNetworkConfigurationPolicyConditionAvailable &&
				condition.Reason == nmstateshared.NodeNetworkConfigurationPolicyConditionSuccessfullyConfigured {
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachConfigured)
			} else if condition.Type == nmstateshared.NodeNetworkConfigurationPolicyConditionDegraded {
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError)

				return common.WrapErrorForObject(cond.Message, instance, err)
			}
		}
	}

	return nil
}

func (r *OpenStackNetAttachmentReconciler) cleanupNodeNetworkConfigurationPolicy(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetAttachment,
	cond *ospdirectorv1beta1.Condition,
) (ctrl.Result, error) {
	networkConfigurationPolicy := &nmstatev1alpha1.NodeNetworkConfigurationPolicy{}

	err := r.Get(ctx, types.NamespacedName{Name: instance.Status.BridgeName}, networkConfigurationPolicy)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	//
	// Set/update CR status from NNCP status
	//
	err = r.getNodeNetworkConfigurationPolicyStatus(ctx, instance, cond)

	// in case of nncp cond.Reason == FailedToConfigure, still continue to try to
	// cleanup and delete the nncp
	if err != nil &&
		cond.Reason != ospdirectorv1beta1.ConditionReason(nmstateshared.NodeNetworkConfigurationPolicyConditionFailedToConfigure) {
		return ctrl.Result{}, err
	}

	bridgeState, err := nmstate.GetDesiredStateBridgeInterfaceState(networkConfigurationPolicy.Spec.DesiredState.Raw)
	if err != nil {
		cond.Message = fmt.Sprintf("Error getting interface state for bride %s from %s networkConfigurationPolicy", instance.Status.BridgeName, networkConfigurationPolicy.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(cond.Message)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError)

		err = common.WrapErrorForObject(cond.Message, networkConfigurationPolicy, err)
		return ctrl.Result{}, err
	}

	if bridgeState != "absent" && bridgeState != "down" {
		apply := func() error {
			desiredState, err := nmstate.GetDesiredStateAsString(networkConfigurationPolicy.Spec.DesiredState.Raw)
			if err != nil {
				return err
			}

			//
			// Update nncp desired state to absent of all interfaces from the NNCP to unconfigure the device on the worker nodes
			// https://docs.openshift.com/container-platform/4.9/networking/k8s_nmstate/k8s-nmstate-updating-node-network-config.html
			//
			re := regexp.MustCompile(`"state":"up"`)
			desiredStateAbsent := re.ReplaceAllString(desiredState, `"state":"absent"`)

			networkConfigurationPolicy.Spec.DesiredState = nmstateshared.State{
				Raw: nmstateshared.RawState(desiredStateAbsent),
			}

			return nil
		}

		//
		// 1) Update nncp desired state to down to unconfigure the device on the worker nodes
		//
		op, err := controllerutil.CreateOrPatch(ctx, r.Client, networkConfigurationPolicy, apply)
		if err != nil {
			cond.Message = fmt.Sprintf("Updating %s networkConfigurationPolicy", instance.Status.BridgeName)
			cond.Reason = ospdirectorv1beta1.ConditionReason(cond.Message)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError)

			err = common.WrapErrorForObject(cond.Message, networkConfigurationPolicy, err)
			return ctrl.Result{}, err
		}

		if op != controllerutil.OperationResultNone {
			common.LogForObject(r, string(op), networkConfigurationPolicy)
		}

		//
		// 2) Delete nncp that DeletionTimestamp get set
		//
		if err := r.Delete(ctx, networkConfigurationPolicy); err != nil && !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

	} else if bridgeState == "absent" && networkConfigurationPolicy.DeletionTimestamp != nil {
		deletionTime := networkConfigurationPolicy.GetDeletionTimestamp().Time
		condition := nmstate.GetCurrentCondition(networkConfigurationPolicy.Status.Conditions)
		if condition != nil {
			nncpStateChangeTime := condition.LastTransitionTime.Time

			//
			// 3) Remove finalizer if nncp update finished
			//
			if nncpStateChangeTime.Sub(deletionTime).Seconds() > 0 &&
				condition.Type == "Available" &&
				condition.Reason == "SuccessfullyConfigured" {

				controllerutil.RemoveFinalizer(networkConfigurationPolicy, openstacknetattachment.FinalizerName)
				if err := r.Update(ctx, networkConfigurationPolicy); err != nil && !k8s_errors.IsNotFound(err) {
					cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
					cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonRemoveFinalizerError)
					cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

					err = common.WrapErrorForObject(cond.Message, instance, err)

					return ctrl.Result{}, err
				}

				common.LogForObject(r, fmt.Sprintf("NodeNetworkConfigurationPolicy is no longer required and has been deleted: %s", networkConfigurationPolicy.Name), instance)

				return ctrl.Result{}, nil

			}
		}
	}
	//
	// RequeueAfter after 20s and get the nncp CR deleted when the device got removed from the worker
	//
	return ctrl.Result{RequeueAfter: time.Second * 20}, nil
}

func (r *OpenStackNetAttachmentReconciler) ensureSriov(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetAttachment,
) error {
	// Labels for all SRIOV objects
	labelSelector := common.GetLabels(instance, openstacknetattachment.AppLabel, map[string]string{})

	sriovNet := &sriovnetworkv1.SriovNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sriov-network", instance.Name),
			Namespace: "openshift-sriov-network-operator",
			Labels:    labelSelector,
		},
	}

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, sriovNet, func() error {
		sriovNet.Labels = common.GetLabels(instance, openstacknetattachment.AppLabel, map[string]string{})
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
		common.LogForObject(r, fmt.Sprintf("SriovNetwork %s successfully reconciled - operation: %s", sriovNet.Name, string(op)), instance)
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

	op, err = controllerutil.CreateOrPatch(ctx, r.Client, sriovPolicy, func() error {
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
		common.LogForObject(r, fmt.Sprintf("SriovNetworkNodePolicy %s successfully reconciled - operation: %s", sriovPolicy.Name, string(op)), instance)
	}

	return nil
}

func (r *OpenStackNetAttachmentReconciler) sriovResourceCleanup(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetAttachment,
) error {
	labelSelectorMap := map[string]string{
		common.OwnerUIDLabelSelector:       string(instance.UID),
		common.OwnerNameSpaceLabelSelector: instance.Namespace,
		common.OwnerNameLabelSelector:      instance.Name,
	}

	// Delete sriovnetworks in openshift-sriov-network-operator namespace
	sriovNetworks, err := openstacknetattachment.GetSriovNetworksWithLabel(ctx, r, labelSelectorMap, "openshift-sriov-network-operator")
	if err != nil {
		return err
	}

	for _, sn := range sriovNetworks {
		err = r.Delete(ctx, &sn, &client.DeleteOptions{})

		if err != nil {
			return err
		}

		common.LogForObject(r, fmt.Sprintf("SriovNetwork deleted: name %s - %s", sn.Name, sn.UID), instance)
	}

	// Delete sriovnetworknodepolicies in openshift-sriov-network-operator namespace
	sriovNetworkNodePolicies, err := openstacknet.GetSriovNetworkNodePoliciesWithLabel(ctx, r, labelSelectorMap, "openshift-sriov-network-operator")
	if err != nil {
		return err
	}

	for _, snnp := range sriovNetworkNodePolicies {
		err = r.Delete(ctx, &snnp, &client.DeleteOptions{})

		if err != nil {
			return err
		}

		common.LogForObject(r, fmt.Sprintf("SriovNetworkNodePolicy deleted: name %s - %s", snnp.Name, snnp.UID), instance)
	}

	return nil
}
