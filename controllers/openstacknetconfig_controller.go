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
	"strconv"
	"strings"
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
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	macaddress "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackmacaddress"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetattachment"
	openstacknetconfig "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetconfig"
)

// OpenStackNetConfigReconciler reconciles a OpenStackNetConfig object
type OpenStackNetConfigReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackNetConfigReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackNetConfigReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackNetConfigReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackNetConfigReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetconfigs/finalizers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknetattachments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets,verbs=get;list;watch;create;update;patch;delete

// Reconcile -
func (r *OpenStackNetConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstacknetconfig", req.NamespacedName)

	// Fetch the OpenStackNetConfig instance
	instance := &ospdirectorv1beta1.OpenStackNetConfig{}
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

	//
	// Used in comparisons below to determine whether a status update is actually needed
	//
	// Save the current status object
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
				if err == nil {
					err = common.WrapErrorForObject("Update Status", instance, updateErr)
				} else {
					common.LogErrorForObject(r, updateErr, "Update status", instance)
				}
			}
		}

		// log current status message to operator log
		common.LogForObject(r, cond.Message, instance)
	}(cond)

	// examine DeletionTimestamp to determine if object is under deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. ThnodeConfPolicy.Nameis is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, openstacknetconfig.FinalizerName) {
			controllerutil.AddFinalizer(instance, openstacknetconfig.FinalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return ctrl.Result{}, err
			}
			common.LogForObject(r, fmt.Sprintf("Finalizer %s added to CR %s", openstacknetconfig.FinalizerName, instance.Name), instance)
		}
	} else {
		//
		// 1. Check if finalizer is there
		//
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, openstacknetconfig.FinalizerName) {
			return ctrl.Result{}, nil
		}

		//
		// 2. Delete all OSNets
		//
		for _, net := range instance.Spec.Networks {

			// TODO: (mschuppert) cleanup single removed netConfig in list
			for _, subnet := range net.Subnets {
				if err := r.osnetCleanup(
					instance,
					&subnet,
					cond,
				); err != nil {
					return ctrl.Result{}, err
				}

			}
		}

		//
		// 3. Clean up all OpenStackNetworkAttachments
		//
		for name, attachConfig := range instance.Spec.AttachConfigurations {
			if err := r.attachCleanup(
				instance,
				name,
				&attachConfig,
				cond,
			); err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}

		//
		// 4. wait for OpenStackNetworkAttachments delete finished
		//
		osNetAttList := &ospdirectorv1beta1.OpenStackNetAttachmentList{}
		labelSelector := map[string]string{
			common.OwnerNameLabelSelector: instance.Name,
		}

		listOpts := []client.ListOption{
			client.InNamespace(instance.Namespace),
			client.MatchingLabels(labelSelector),
		}

		if err := r.GetClient().List(context.Background(), osNetAttList, listOpts...); err != nil {
			return ctrl.Result{}, err
		}

		if len(osNetAttList.Items) > 0 {
			instance.Status.ProvisioningStatus.AttachReadyCount = len(osNetAttList.Items)

			cond.Message = fmt.Sprintf("OpenStackNetConfig %s waiting for all OpenStackNetAttachments to be deleted", instance.Name)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigWaiting)
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 20}, nil

		}

		// TODO: osmacaddr cleanup

		// X. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, openstacknetconfig.FinalizerName)
		err = r.Client.Update(context.TODO(), instance)
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
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(
		r.Client,
		instance.Namespace,
		instance.Status.ProvisioningStatus.State == ospdirectorv1beta1.NetConfigConfigured,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("%s %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Kind, instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	//
	// 1) Create or update the MACAddress CR object
	//
	instance.Status.ProvisioningStatus.PhysNetDesiredCount = len(instance.Spec.OVNBridgeMacMappings.PhysNetworks)
	instance.Status.ProvisioningStatus.PhysNetReadyCount = 0
	err = r.createOrUpdateOpenStackMACAddress(
		instance,
		cond,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// 2) create all OpenStackNetworkAttachments
	//
	instance.Status.ProvisioningStatus.AttachDesiredCount = len(instance.Spec.AttachConfigurations)
	instance.Status.ProvisioningStatus.AttachReadyCount = 0
	for name, attachConfig := range instance.Spec.AttachConfigurations {
		// TODO: (mschuppert) cleanup single removed netAttachment in list
		netAttachment, err := r.applyNetAttachmentConfig(
			instance,
			name,
			&attachConfig,
			cond,
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		//
		// Set/update CR status from OSNetAttach status
		//
		ctrlResult, err := r.getNetAttachmentStatus(
			instance,
			netAttachment,
			cond,
		)
		if err != nil {
			return ctrl.Result{}, err
		} else if !reflect.DeepEqual(ctrlResult, ctrl.Result{}) {
			cond.Message = fmt.Sprintf("%s %s waiting for all OpenStackNetworkAttachments to be configured", instance.Kind, instance.Name)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigWaiting)

			return ctrlResult, nil
		}
		instance.Status.ProvisioningStatus.AttachReadyCount++
	}

	//
	// 3) create all OpenStackNetworks
	//
	instance.Status.ProvisioningStatus.NetDesiredCount = len(instance.Spec.Networks)
	instance.Status.ProvisioningStatus.NetReadyCount = 0
	for _, net := range instance.Spec.Networks {

		// TODO: (mschuppert) cleanup single removed netConfig in list
		for _, subnet := range net.Subnets {
			osNet, err := r.applyNetConfig(
				instance,
				cond,
				&net,
				&subnet,
			)
			if err != nil {
				return ctrl.Result{}, err
			}

			//
			// Update CR status from OSNet status
			//
			ctrlResult, err := r.getNetStatus(
				instance,
				osNet,
				cond,
			)
			if err != nil {
				return ctrl.Result{}, err
			} else if !reflect.DeepEqual(ctrlResult, ctrl.Result{}) {
				cond.Message = fmt.Sprintf("%s %s waiting for all OpenStackNetworks to be configured", instance.Kind, instance.Name)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigWaiting)

				return ctrlResult, nil
			}

			instance.Status.ProvisioningStatus.NetReadyCount++
		}
	}

	if (instance.Status.ProvisioningStatus.NetDesiredCount == instance.Status.ProvisioningStatus.NetReadyCount) &&
		(instance.Status.ProvisioningStatus.AttachDesiredCount == instance.Status.ProvisioningStatus.AttachReadyCount) &&
		(instance.Status.ProvisioningStatus.PhysNetDesiredCount == instance.Status.ProvisioningStatus.PhysNetReadyCount) {
		instance.Status.ProvisioningStatus.State = ospdirectorv1beta1.NetConfigConfigured
		instance.Status.ProvisioningStatus.Reason = fmt.Sprintf("%s %s all resources configured", instance.Kind, instance.Name)
	} else {
		instance.Status.ProvisioningStatus.State = ospdirectorv1beta1.NetConfigConfiguring
		instance.Status.ProvisioningStatus.Reason = fmt.Sprintf("%s %s waiting for all resources to be configured", instance.Kind, instance.Name)

	}

	return ctrl.Result{}, nil
}

func (r *OpenStackNetConfigReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackNetConfigStatus) *ospdirectorv1beta1.OpenStackNetConfigStatus {

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
func (r *OpenStackNetConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

	//
	// Schedule reconcile on OpenStackNetConfig if any of the objects change where
	// reconcile label openstacknetconfig.OpenStackNetConfigReconcileLabel
	//
	LabelWatcher := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		labels := o.GetLabels()
		//
		// verify object has ConfigGeneratorInputLabel
		//
		reconcileCR, ok := labels[openstacknetconfig.OpenStackNetConfigReconcileLabel]
		if !ok {
			return []reconcile.Request{}
		}

		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{
				Name:      reconcileCR,
				Namespace: o.GetNamespace(),
			}},
		}
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackNetConfig{}).
		Owns(&ospdirectorv1beta1.OpenStackNetAttachment{}).
		Owns(&ospdirectorv1beta1.OpenStackNet{}).
		Owns(&ospdirectorv1beta1.OpenStackMACAddress{}).
		Watches(&source.Kind{Type: &ospdirectorv1beta1.OpenStackBaremetalSet{}}, LabelWatcher).
		Watches(&source.Kind{Type: &ospdirectorv1beta1.OpenStackVMSet{}}, LabelWatcher).
		Complete(r)
}

func (r *OpenStackNetConfigReconciler) applyNetAttachmentConfig(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	nodeConfName string,
	nodeConfPolicy *ospdirectorv1beta1.NodeConfigurationPolicy,
	cond *ospdirectorv1beta1.Condition,
) (*ospdirectorv1beta1.OpenStackNetAttachment, error) {
	attachConfig := &ospdirectorv1beta1.OpenStackNetAttachment{}

	//
	// default attach type is AttachTypeBridge
	//
	attachType := ospdirectorv1beta1.AttachTypeBridge
	if nodeConfPolicy.NodeSriovConfigurationPolicy.DesiredState.Port != "" {
		attachType = ospdirectorv1beta1.AttachTypeSriov
	}

	attachConfig.Name = fmt.Sprintf("%s-%s", nodeConfName, strings.ToLower(string(attachType)))
	attachConfig.Namespace = instance.Namespace

	apply := func() error {
		common.InitMap(&attachConfig.Labels)
		attachConfig.Labels[common.OwnerUIDLabelSelector] = string(instance.UID)
		attachConfig.Labels[common.OwnerNameLabelSelector] = instance.Name
		attachConfig.Labels[common.OwnerNameSpaceLabelSelector] = instance.Namespace
		attachConfig.Labels[common.OwnerControllerNameLabelSelector] = openstacknetconfig.AppLabel
		attachConfig.Labels[openstacknetattachment.AttachReference] = nodeConfName
		attachConfig.Labels[openstacknetattachment.AttachType] = string(attachType)
		attachConfig.Labels[string(attachType)] = nodeConfName

		switch attachType {
		case ospdirectorv1beta1.AttachTypeBridge:
			attachConfig.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy = nodeConfPolicy.NodeNetworkConfigurationPolicy
		case ospdirectorv1beta1.AttachTypeSriov:
			attachConfig.Spec.AttachConfiguration.NodeSriovConfigurationPolicy = nodeConfPolicy.NodeSriovConfigurationPolicy
		}

		return controllerutil.SetControllerReference(instance, attachConfig, r.Scheme)
	}

	op, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, attachConfig, apply)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", attachConfig.Kind, attachConfig.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.NetAttachCondReasonCreateError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, attachConfig, err)

		return attachConfig, err
	}

	cond.Message = fmt.Sprintf("%s %s %s %s CR successfully reconciled",
		instance.Kind,
		instance.Name,
		attachConfig.Kind,
		attachConfig.Name,
	)

	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))
	}
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.NetAttachCondReasonCreated)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)

	return attachConfig, nil
}

func (r *OpenStackNetConfigReconciler) getNetAttachmentStatus(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	netAttachment *ospdirectorv1beta1.OpenStackNetAttachment,
	cond *ospdirectorv1beta1.Condition,
) (ctrl.Result, error) {

	cond.Message = fmt.Sprintf("OpenStackNetConfig %s is configuring OpenStackNetAttachment(s)", instance.Name)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigConfiguring)

	ctrlResult := ctrl.Result{
		RequeueAfter: time.Second * 20,
	}
	//
	// sync latest status of the osnetattach object to the osnetconfig
	//
	if netAttachment.Status.Conditions != nil && len(netAttachment.Status.Conditions) > 0 {
		condition := netAttachment.Status.Conditions.GetCurrentCondition()

		if condition != nil {
			if condition.Type == ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachConfigured) {
				cond.Message = fmt.Sprintf("OpenStackNetConfig %s has successfully configured OpenStackNetAttachment %s", instance.Name, netAttachment.Name)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigConfigured)
				common.LogForObject(r, cond.Message, instance)

				ctrlResult = ctrl.Result{}
			} else if condition.Type == ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError) {
				cond.Message = fmt.Sprintf("OpenStackNetAttachment error: %s", condition.Message)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigError)

				return ctrl.Result{}, fmt.Errorf(cond.Message)
			}
		}
	}

	instance.Status.ProvisioningStatus.State = ospdirectorv1beta1.ProvisioningState(cond.Type)
	instance.Status.ProvisioningStatus.Reason = cond.Message

	return ctrlResult, nil
}

func (r *OpenStackNetConfigReconciler) attachCleanup(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	nodeConfName string,
	nodeConfPolicy *ospdirectorv1beta1.NodeConfigurationPolicy,
	cond *ospdirectorv1beta1.Condition,
) error {
	attachConfig := &ospdirectorv1beta1.OpenStackNetAttachment{}

	// default attach type is AttachTypeBridge
	attachType := ospdirectorv1beta1.AttachTypeBridge
	if nodeConfPolicy.NodeSriovConfigurationPolicy.DesiredState.Port != "" {
		attachType = ospdirectorv1beta1.AttachTypeSriov
	}

	attachConfig.Name = fmt.Sprintf("%s-%s", nodeConfName, strings.ToLower(string(attachType)))
	attachConfig.Namespace = instance.Namespace

	cond.Message = fmt.Sprintf("OpenStackNetAttachment %s delete started", attachConfig.Name)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigConfiguring)

	if err := r.Client.Delete(context.TODO(), attachConfig, &client.DeleteOptions{}); err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	common.LogForObject(r, cond.Message, instance)

	return nil
}

func (r *OpenStackNetConfigReconciler) applyNetConfig(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *ospdirectorv1beta1.Condition,
	net *ospdirectorv1beta1.Network,
	subnet *ospdirectorv1beta1.Subnet,
) (*ospdirectorv1beta1.OpenStackNet, error) {
	osNet := &ospdirectorv1beta1.OpenStackNet{}

	//
	// _ is a not allowed char for an OCP object, lets remove it
	//
	osNet.Name = strings.ToLower(strings.Replace(subnet.Name, "_", "", -1))
	osNet.Namespace = instance.Namespace

	apply := func() error {
		common.InitMap(&osNet.Labels)
		osNet.Labels[common.OwnerUIDLabelSelector] = string(instance.UID)
		osNet.Labels[common.OwnerNameLabelSelector] = instance.Name
		osNet.Labels[common.OwnerNameSpaceLabelSelector] = instance.Namespace
		osNet.Labels[common.OwnerControllerNameLabelSelector] = openstacknetconfig.AppLabel
		osNet.Labels[openstacknet.NetworkNameLabelSelector] = net.Name
		osNet.Labels[openstacknet.NetworkNameLowerLabelSelector] = net.NameLower
		osNet.Labels[openstacknet.SubNetNameLabelSelector] = subnet.Name
		osNet.Labels[openstacknet.ControlPlaneNetworkLabelSelector] = strconv.FormatBool(net.IsControlPlane)

		osNet.Spec.AttachConfiguration = subnet.AttachConfiguration
		osNet.Spec.MTU = net.MTU
		osNet.Spec.Name = net.Name
		osNet.Spec.NameLower = subnet.Name
		if net.IsControlPlane {
			osNet.Spec.DomainName = fmt.Sprintf("%s.%s", ospdirectorv1beta1.ControlPlaneNameLower, instance.Spec.DomainName)
		} else {
			osNet.Spec.DomainName = fmt.Sprintf("%s.%s", strings.ToLower(net.Name), instance.Spec.DomainName)
		}
		osNet.Spec.VIP = net.VIP
		osNet.Spec.Vlan = subnet.Vlan

		if subnet.IPv4.Cidr != "" {
			osNet.Spec.AllocationEnd = subnet.IPv4.AllocationEnd
			osNet.Spec.AllocationStart = subnet.IPv4.AllocationStart
			osNet.Spec.Cidr = subnet.IPv4.Cidr
			osNet.Spec.Gateway = subnet.IPv4.Gateway
			osNet.Spec.Routes = subnet.IPv4.Routes
		} else {
			osNet.Spec.AllocationEnd = subnet.IPv6.AllocationEnd
			osNet.Spec.AllocationStart = subnet.IPv6.AllocationStart
			osNet.Spec.Cidr = subnet.IPv6.Cidr
			osNet.Spec.Gateway = subnet.IPv6.Gateway
			osNet.Spec.Routes = subnet.IPv6.Routes
		}

		return controllerutil.SetControllerReference(instance, osNet, r.Scheme)
	}

	op, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, osNet, apply)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", osNet.Kind, osNet.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.NetCondReasonCreateError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, osNet, err)

		return nil, err
	}

	cond.Message = fmt.Sprintf("%s %s %s %s CR successfully reconciled",
		instance.Kind,
		instance.Name,
		osNet.Kind,
		osNet.Name,
	)

	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))
	}
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.NetCondReasonCreated)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)

	return osNet, nil
}

func (r *OpenStackNetConfigReconciler) getNetStatus(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	net *ospdirectorv1beta1.OpenStackNet,
	cond *ospdirectorv1beta1.Condition,
) (ctrl.Result, error) {

	cond.Message = fmt.Sprintf("OpenStackNetConfig %s is configuring OpenStackNet(s)", instance.Name)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigConfiguring)

	ctrlResult := ctrl.Result{
		RequeueAfter: time.Second * 20,
	}
	//
	// sync latest status of the osnet object to the osnetconfig
	//
	if net.Status.Conditions != nil && len(net.Status.Conditions) > 0 {
		condition := net.Status.Conditions.GetCurrentCondition()

		if condition != nil {
			if condition.Type == ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachConfigured) {
				cond.Message = fmt.Sprintf("OpenStackNetConfig %s has successfully configured OpenStackNet %s", instance.Name, net.Spec.NameLower)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigConfigured)

				ctrlResult = ctrl.Result{}
			} else if condition.Type == ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetAttachError) {
				cond.Message = fmt.Sprintf("OpenStackNet error: %s", condition.Message)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigError)
				common.LogForObject(r, cond.Message, instance)

				return ctrl.Result{}, fmt.Errorf(cond.Message)
			}
		}
	}

	instance.Status.ProvisioningStatus.State = ospdirectorv1beta1.ProvisioningState(cond.Type)
	instance.Status.ProvisioningStatus.Reason = cond.Message

	return ctrlResult, nil
}

func (r *OpenStackNetConfigReconciler) osnetCleanup(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	subnet *ospdirectorv1beta1.Subnet,
	cond *ospdirectorv1beta1.Condition,
) error {
	osNet := &ospdirectorv1beta1.OpenStackNet{}

	osNet.Name = strings.ToLower(strings.Replace(subnet.Name, "_", "", -1))
	osNet.Namespace = instance.Namespace

	cond.Message = fmt.Sprintf("OpenStackNet %s delete started", osNet.Name)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.NetConfigConfiguring)

	if err := r.Client.Delete(context.TODO(), osNet, &client.DeleteOptions{}); err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	common.LogForObject(r, cond.Message, instance)

	return nil
}

//
// create or update the OpenStackMACAddress object
//
func (r *OpenStackNetConfigReconciler) createOrUpdateOpenStackMACAddress(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *ospdirectorv1beta1.Condition,
) error {
	macAddress := &ospdirectorv1beta1.OpenStackMACAddress{
		ObjectMeta: metav1.ObjectMeta{
			// use the role name as the VM CR name
			Name:      strings.ToLower(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	//
	// get OSMACAddress object
	//
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: strings.ToLower(instance.Name), Namespace: instance.Namespace}, macAddress)
	if err != nil && !k8s_errors.IsNotFound(err) {
		cond.Message = fmt.Sprintf("Failed to get %s %s ", macAddress.Kind, macAddress.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	// If network RoleReservations spec map is nil, create it
	if macAddress.Spec.RoleReservations == nil {
		macAddress.Spec.RoleReservations = map[string]ospdirectorv1beta1.OpenStackMACRoleReservation{}
	}

	//
	// get all VMset
	//
	labelSelector := map[string]string{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelSelector),
	}

	vmSetList := &ospdirectorv1beta1.OpenStackVMSetList{}
	if err := r.GetClient().List(
		context.Background(),
		vmSetList,
		listOpts...,
	); err != nil && !k8s_errors.IsNotFound(err) {
		return err
	}

	//
	// get all BMset
	//
	bmSetList := &ospdirectorv1beta1.OpenStackBaremetalSetList{}
	if err := r.GetClient().List(
		context.Background(),
		bmSetList,
		listOpts...,
	); err != nil && !k8s_errors.IsNotFound(err) {
		return err
	}

	//
	// create MAC reservations for all VMset/BMset
	//
	reservations := map[string]ospdirectorv1beta1.OpenStackMACRoleReservation{}

	// VMset
	for _, vmSet := range vmSetList.Items {
		roleMACReservation := map[string]ospdirectorv1beta1.OpenStackMACNodeReservation{}

		for _, vmHost := range vmSet.Status.VMHosts {

			roleMACReservation[vmHost.Hostname], err = r.ensureMACReservation(
				instance,
				cond,
				macAddress,
				macAddress.Spec.RoleReservations,
				vmSet.Spec.RoleName,
				vmHost.Hostname,
				vmHost.AnnotatedForDeletion,
			)
			if err != nil {
				return err
			}
		}

		//
		// add reservations from deleted nodes if physnet.PreserveReservations
		//
		if *instance.Spec.OVNBridgeMacMappings.PreserveReservations {
			r.preserveMACReservations(
				macAddress,
				&roleMACReservation,
				vmSet.Spec.RoleName,
			)
		}

		reservations[vmSet.Spec.RoleName] = ospdirectorv1beta1.OpenStackMACRoleReservation{
			Reservations: roleMACReservation,
		}
	}

	// bmset
	for _, bmSet := range bmSetList.Items {
		roleMACReservation := map[string]ospdirectorv1beta1.OpenStackMACNodeReservation{}

		for _, bmHost := range bmSet.Status.BaremetalHosts {

			roleMACReservation[bmHost.Hostname], err = r.ensureMACReservation(
				instance,
				cond,
				macAddress,
				macAddress.Spec.RoleReservations,
				bmSet.Spec.RoleName,
				bmHost.Hostname,
				bmHost.AnnotatedForDeletion,
			)
			if err != nil {
				return err
			}
		}

		//
		// add reservations from deleted nodes if physnet.PreserveReservations
		//
		if *instance.Spec.OVNBridgeMacMappings.PreserveReservations {
			r.preserveMACReservations(
				macAddress,
				&roleMACReservation,
				bmSet.Spec.RoleName,
			)
		}

		reservations[bmSet.Spec.RoleName] = ospdirectorv1beta1.OpenStackMACRoleReservation{
			Reservations: roleMACReservation,
		}
	}

	//
	// create/update OSMACAddress
	//
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, macAddress, func() error {
		if len(instance.Spec.OVNBridgeMacMappings.PhysNetworks) == 0 {
			macAddress.Spec.PhysNetworks = []ospdirectorv1beta1.Physnet{
				{
					Name:      ospdirectorv1beta1.DefaultOVNChassisPhysNetName,
					MACPrefix: ospdirectorv1beta1.DefaultOVNChassisPhysNetMACPrefix,
				},
			}
		} else {
			macPhysnets := []ospdirectorv1beta1.Physnet{}
			for _, physnet := range instance.Spec.OVNBridgeMacMappings.PhysNetworks {
				macPrefix := physnet.MACPrefix
				// make sure if MACPrefix was not speficied to set the default prefix
				if macPrefix == "" {
					macPrefix = ospdirectorv1beta1.DefaultOVNChassisPhysNetMACPrefix
				}
				macPhysnets = append(macPhysnets, ospdirectorv1beta1.Physnet{
					Name:      physnet.Name,
					MACPrefix: macPrefix,
				})
			}

			macAddress.Spec.PhysNetworks = macPhysnets
		}

		macAddress.Spec.RoleReservations = reservations

		err := controllerutil.SetControllerReference(instance, macAddress, r.Scheme)
		if err != nil {
			cond.Message = fmt.Sprintf("Error set controller reference for %s", macAddress.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonControllerReferenceError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", macAddress.Kind, macAddress.Name)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonCreateMACError)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	cond.Message = "OpenStackMACAddress CR successfully reconciled"

	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))
	}
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonAllMACAddressesCreated)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeProvisioned)

	instance.Status.ProvisioningStatus.PhysNetReadyCount = len(macAddress.Spec.PhysNetworks)

	return nil
}

//
// create or update the OpenStackMACAddress object
//
func (r *OpenStackNetConfigReconciler) ensureMACReservation(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *ospdirectorv1beta1.Condition,
	macAddress *ospdirectorv1beta1.OpenStackMACAddress,
	reservations map[string]ospdirectorv1beta1.OpenStackMACRoleReservation,
	roleName string,
	hostname string,
	annotatedForDeletion bool,
) (ospdirectorv1beta1.OpenStackMACNodeReservation, error) {

	nodeMACReservation := ospdirectorv1beta1.OpenStackMACNodeReservation{}

	//
	// if node does not yet have a reservation, create one
	//
	if _, ok := reservations[roleName].Reservations[hostname]; !ok {
		nodeMACReservation = ospdirectorv1beta1.OpenStackMACNodeReservation{
			Deleted:      annotatedForDeletion,
			Reservations: map[string]string{},
		}

	} else {
		nodeMACReservation = ospdirectorv1beta1.OpenStackMACNodeReservation{
			Deleted:      reservations[roleName].Reservations[hostname].Deleted,
			Reservations: map[string]string{},
		}
		for _, physnet := range instance.Spec.OVNBridgeMacMappings.PhysNetworks {
			nodeMACReservation.Reservations[physnet.Name] = reservations[roleName].Reservations[hostname].Reservations[physnet.Name]
		}
	}

	if !annotatedForDeletion {
		nodeMACReservation.Deleted = annotatedForDeletion

		// create reservation for every specified physnet
		for _, physnet := range instance.Spec.OVNBridgeMacMappings.PhysNetworks {

			//
			// if there is a static reservation configured for the physnet AND is not "", use it
			//
			if res, ok := instance.Spec.OVNBridgeMacMappings.StaticReservations[hostname]; ok && res.Reservations[physnet.Name] != "" {
				nodeMACReservation.Reservations[physnet.Name] = res.Reservations[physnet.Name]
				continue
			}

			//
			// if there is no reservation for the physnet, OR the reservation is empty (e.g. new physnet), create one
			//
			if res, ok := nodeMACReservation.Reservations[physnet.Name]; !ok || res == "" {
				// create MAC address and verify it is uniqe in the CR reservations
				var newMAC string
				var err error
				for ok := true; ok; ok = !ospdirectorv1beta1.IsUniqMAC(
					r.allMACReservations(
						cond,
						instance,
						macAddress,
					),
					newMAC,
				) {
					newMAC, err = macaddress.CreateMACWithPrefix(physnet.MACPrefix)
					if err != nil {
						cond.Message = err.Error()
						cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonCreateMACError)
						cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACCondTypeCreating)
						err = common.WrapErrorForObject(cond.Message, instance, err)

						return nodeMACReservation, err
					}

					common.LogForObject(
						r,
						fmt.Sprintf("New MAC created - node: %s, physnet: %s, mac: %s", hostname, physnet.Name, newMAC),
						instance,
					)
				}

				nodeMACReservation.Reservations[physnet.Name] = newMAC
			}
		}

	} else {
		// If the node is flagged as deleted in the osnet, also mark the
		// MAC reservation as deleted. If the node gets recreated with the
		// same name, it reuses the MAC.
		nodeMACReservation.Deleted = annotatedForDeletion
	}

	return nodeMACReservation, nil
}

//
// allMACReservations - get all reservations from static + dynamic created
//
func (r *OpenStackNetConfigReconciler) allMACReservations(
	cond *ospdirectorv1beta1.Condition,
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	macAddress *ospdirectorv1beta1.OpenStackMACAddress,
) map[string]ospdirectorv1beta1.OpenStackMACNodeReservation {
	reservations := macAddress.Status.MACReservations

	for node, res := range instance.Spec.OVNBridgeMacMappings.StaticReservations {
		reservations[node] = res
	}

	return reservations
}

//
// add reservations from deleted nodes if physnet.PreserveReservations
//
func (r *OpenStackNetConfigReconciler) preserveMACReservations(
	macAddress *ospdirectorv1beta1.OpenStackMACAddress,
	roleMACReservation *map[string]ospdirectorv1beta1.OpenStackMACNodeReservation,
	roleName string,
) {
	for hostname := range macAddress.Spec.RoleReservations[roleName].Reservations {
		nodeMACReservation := ospdirectorv1beta1.OpenStackMACNodeReservation{
			Deleted:      true,
			Reservations: map[string]string{},
		}

		if _, ok := (*roleMACReservation)[hostname]; !ok {
			nodeMACReservation.Reservations =
				macAddress.Spec.RoleReservations[roleName].Reservations[hostname].Reservations
			(*roleMACReservation)[hostname] = nodeMACReservation
		}
	}

}
