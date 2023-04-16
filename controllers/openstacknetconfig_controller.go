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
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	ospdirectorv1beta2 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta2"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
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

	if instance.Status.Hosts == nil {
		instance.Status.Hosts = make(map[string]ospdirectorv1beta1.OpenStackHostStatus)
	}

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

	defer func(cond *shared.Condition) {
		//
		// Update object conditions
		//
		instance.Status.Conditions.UpdateCurrentCondition(
			cond.Type,
			cond.Reason,
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
		// then lets add the finalizer and update the object. ThnodeConfPolicy.Nameis is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, openstacknetconfig.FinalizerName) {
			controllerutil.AddFinalizer(instance, openstacknetconfig.FinalizerName)
			if err := r.Update(ctx, instance); err != nil {
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
					ctx,
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
				ctx,
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

		if err := r.GetClient().List(ctx, osNetAttList, listOpts...); err != nil {
			return ctrl.Result{}, err
		}

		if len(osNetAttList.Items) > 0 {
			instance.Status.ProvisioningStatus.AttachReadyCount = len(osNetAttList.Items)

			cond.Message = fmt.Sprintf("OpenStackNetConfig %s waiting for all OpenStackNetAttachments to be deleted", instance.Name)
			cond.Type = shared.NetConfigWaiting
			common.LogForObject(r, cond.Message, instance)

			return ctrl.Result{RequeueAfter: time.Second * 20}, nil

		}

		// TODO: osmacaddr cleanup

		// X. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, openstacknetconfig.FinalizerName)
		err = r.Update(ctx, instance)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
			cond.Reason = shared.CommonCondReasonRemoveFinalizerError
			cond.Type = shared.CommonCondTypeError

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		common.LogForObject(r, fmt.Sprintf("CR %s deleted", instance.Name), instance)

		return ctrl.Result{}, nil
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.IsReady())
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("%s %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Kind, instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	//
	// Add AppLabel
	//
	instance.SetLabels(labels.Merge(instance.GetLabels(), common.GetLabels(instance, openstacknetconfig.AppLabel, map[string]string{})))

	//
	// 1) create all OpenStackNetworkAttachments
	//
	instance.Status.ProvisioningStatus.AttachDesiredCount = len(instance.Spec.AttachConfigurations)
	instance.Status.ProvisioningStatus.AttachReadyCount = 0
	for name, attachConfig := range instance.Spec.AttachConfigurations {
		// TODO: (mschuppert) cleanup single removed netAttachment in list
		netAttachment, err := r.applyNetAttachmentConfig(
			ctx,
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
		err = r.getNetAttachmentStatus(
			instance,
			netAttachment,
			cond,
		)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	//
	// 2) create all OpenStackNetworks
	//
	instance.Status.ProvisioningStatus.NetDesiredCount = r.getNetDesiredCount(instance.Spec.Networks)
	instance.Status.ProvisioningStatus.NetReadyCount = 0

	ctlplaneReservations := map[string]int{}
	for _, net := range instance.Spec.Networks {

		// TODO: (mschuppert) cleanup single removed netConfig in list
		for _, subnet := range net.Subnets {
			osNet, err := r.applyNetConfig(
				ctx,
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
				cond.Type = shared.NetConfigWaiting

				return ctrlResult, nil
			}

			if net.IsControlPlane {
				ctlplaneReservations[osNet.Spec.NameLower] = osNet.Status.ReservedIPCount
			}

			instance.Status.ProvisioningStatus.NetReadyCount++
		}
	}

	// all nodes have a ctlplane network, if there are no reservations
	// in any of the ctlplane subnets, reset the osnetcfg host reservation status
	statusHostReservationCleanup := true
	for _, res := range ctlplaneReservations {
		if res > 0 {
			statusHostReservationCleanup = false
			break
		}
	}
	if statusHostReservationCleanup {
		instance.Status.Hosts = map[string]ospdirectorv1beta1.OpenStackHostStatus{}
	}

	//
	// 3) Create or update the MACAddress CR object
	//
	instance.Status.ProvisioningStatus.PhysNetDesiredCount = len(instance.Spec.OVNBridgeMacMappings.PhysNetworks)
	instance.Status.ProvisioningStatus.PhysNetReadyCount = 0
	macAddress, err := r.createOrUpdateOpenStackMACAddress(
		ctx,
		instance,
		cond,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// Update CR status from OSmac status
	//
	r.getMACStatus(
		instance,
		cond,
		macAddress,
	)

	if instance.IsReady() {
		instance.Status.ProvisioningStatus.State = shared.ProvisioningState(shared.NetConfigConfigured)
		instance.Status.ProvisioningStatus.Reason = fmt.Sprintf("%s %s all resources configured", instance.Kind, instance.Name)
	} else {
		instance.Status.ProvisioningStatus.State = shared.ProvisioningState(shared.NetConfigConfiguring)
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
		// verify object has OpenStackNetConfigReconcileLabel
		//
		reconcileCR, ok := labels[shared.OpenStackNetConfigReconcileLabel]
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
		Watches(&source.Kind{Type: &ospdirectorv1beta1.OpenStackIPSet{}}, LabelWatcher).
		Complete(r)
}

func (r *OpenStackNetConfigReconciler) applyNetAttachmentConfig(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	nodeConfName string,
	nodeConfPolicy *ospdirectorv1beta1.NodeConfigurationPolicy,
	cond *shared.Condition,
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
		shared.InitMap(&attachConfig.Labels)
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

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, attachConfig, apply)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", attachConfig.Kind, attachConfig.Name)
		cond.Reason = shared.NetAttachCondReasonCreateError
		cond.Type = shared.CommonCondTypeError
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
	cond.Reason = shared.NetAttachCondReasonCreated
	cond.Type = shared.CommonCondTypeProvisioned

	return attachConfig, nil
}

func (r *OpenStackNetConfigReconciler) getNetAttachmentStatus(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	netAttachment *ospdirectorv1beta1.OpenStackNetAttachment,
	cond *shared.Condition,
) error {

	cond.Message = fmt.Sprintf("OpenStackNetConfig %s is configuring OpenStackNetAttachment(s)", instance.Name)
	cond.Type = shared.NetConfigConfiguring

	//
	// sync latest status of the osnetattach object to the osnetconfig
	//
	if netAttachment.Status.Conditions != nil && len(netAttachment.Status.Conditions) > 0 {
		condition := netAttachment.Status.Conditions.GetCurrentCondition()

		if condition != nil {
			if condition.Type == shared.NetAttachConfigured {
				cond.Message = fmt.Sprintf("OpenStackNetConfig %s has successfully configured OpenStackNetAttachment %s", instance.Name, netAttachment.Name)
				cond.Type = shared.NetConfigConfigured
				common.LogForObject(r, cond.Message, instance)

				instance.Status.ProvisioningStatus.AttachReadyCount++

			} else if condition.Type == shared.NetAttachError {
				cond.Message = fmt.Sprintf("OpenStackNetAttachment error: %s", condition.Message)
				cond.Type = shared.NetConfigError

				return fmt.Errorf(cond.Message)
			}
		}
	}

	instance.Status.ProvisioningStatus.State = shared.ProvisioningState(cond.Type)
	instance.Status.ProvisioningStatus.Reason = cond.Message

	return nil
}

func (r *OpenStackNetConfigReconciler) attachCleanup(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	nodeConfName string,
	nodeConfPolicy *ospdirectorv1beta1.NodeConfigurationPolicy,
	cond *shared.Condition,
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
	cond.Type = shared.NetConfigConfiguring

	if err := r.Delete(ctx, attachConfig, &client.DeleteOptions{}); err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	common.LogForObject(r, cond.Message, instance)

	return nil
}

func (r *OpenStackNetConfigReconciler) applyNetConfig(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *shared.Condition,
	net *ospdirectorv1beta1.Network,
	subnet *ospdirectorv1beta1.Subnet,
) (*ospdirectorv1beta1.OpenStackNet, error) {
	osNet := &ospdirectorv1beta1.OpenStackNet{
		ObjectMeta: metav1.ObjectMeta{
			// _ is a not allowed char for an OCP object, lets remove it
			Name:      strings.ToLower(strings.Replace(subnet.Name, "_", "", -1)),
			Namespace: instance.Namespace,
		},
	}

	//
	// get OSNet object
	//
	err := r.Get(ctx, types.NamespacedName{Name: strings.ToLower(osNet.Name), Namespace: osNet.Namespace}, osNet)
	if err != nil && !k8s_errors.IsNotFound(err) {
		cond.Message = fmt.Sprintf("Failed to get %s %s ", osNet.Kind, osNet.Name)
		cond.Reason = shared.CommonCondReasonOSNetError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return osNet, err
	}

	// If network RoleReservations spec map is nil, create it
	if osNet.Spec.RoleReservations == nil {
		osNet.Spec.RoleReservations = map[string]ospdirectorv1beta1.OpenStackNetRoleReservation{}
	}

	if osNet.Status.Reservations == nil {
		osNet.Status.Reservations = map[string]ospdirectorv1beta1.NodeIPReservation{}
	}

	//
	// Create IPs for new nodes added to OSClient,OSVMset,OSBMset and OSCtlplane (VIPs)
	//
	reservations, err := r.ensureIPReservation(
		ctx,
		instance,
		cond,
		osNet,
	)
	if err != nil {
		return osNet, err
	}

	apply := func() error {
		shared.InitMap(&osNet.Labels)
		osNet.Labels[common.OwnerUIDLabelSelector] = string(instance.UID)
		osNet.Labels[common.OwnerNameLabelSelector] = instance.Name
		osNet.Labels[common.OwnerNameSpaceLabelSelector] = instance.Namespace
		osNet.Labels[common.OwnerControllerNameLabelSelector] = openstacknetconfig.AppLabel
		osNet.Labels[shared.NetworkNameLabelSelector] = net.Name
		osNet.Labels[shared.NetworkNameLowerLabelSelector] = net.NameLower
		osNet.Labels[shared.SubNetNameLabelSelector] = subnet.Name
		osNet.Labels[shared.ControlPlaneNetworkLabelSelector] = strconv.FormatBool(net.IsControlPlane)

		osNet.Spec.AttachConfiguration = subnet.AttachConfiguration
		osNet.Spec.MTU = net.MTU
		osNet.Spec.Name = net.Name
		osNet.Spec.NameLower = subnet.Name
		if net.IsControlPlane {
			osNet.Spec.DomainName = fmt.Sprintf("%s.%s", ospdirectorv1beta1.ControlPlaneNameLower, instance.Spec.DomainName)
			// TripleO does not support VLAN on ctlplane
		} else {
			osNet.Spec.DomainName = fmt.Sprintf("%s.%s", strings.ToLower(net.Name), instance.Spec.DomainName)
			osNet.Spec.Vlan = subnet.Vlan
		}
		osNet.Spec.VIP = net.VIP

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

		osNet.Spec.RoleReservations = reservations

		return controllerutil.SetControllerReference(instance, osNet, r.Scheme)
	}

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, osNet, apply)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", osNet.Kind, osNet.Name)
		cond.Reason = shared.NetCondReasonCreateError
		cond.Type = shared.CommonCondTypeError
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
	cond.Reason = shared.NetCondReasonCreated
	cond.Type = shared.CommonCondTypeProvisioned

	return osNet, nil
}

func (r *OpenStackNetConfigReconciler) getNetStatus(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	osNet *ospdirectorv1beta1.OpenStackNet,
	cond *shared.Condition,
) (ctrl.Result, error) {

	cond.Message = fmt.Sprintf("OpenStackNetConfig %s is configuring OpenStackNet(s)", instance.Name)
	cond.Type = shared.NetConfigConfiguring

	ctrlResult := ctrl.Result{
		RequeueAfter: time.Second * 20,
	}
	//
	// sync latest status of the osnet object to the osnetconfig
	//
	if osNet.Status.Conditions != nil && len(osNet.Status.Conditions) > 0 {
		condition := osNet.Status.Conditions.GetCurrentCondition()

		if condition != nil {
			if condition.Type == shared.NetAttachConfigured {
				cond.Message = fmt.Sprintf("OpenStackNetConfig %s has successfully configured OpenStackNet %s", instance.Name, osNet.Spec.NameLower)
				cond.Type = shared.NetConfigConfigured

				ctrlResult = ctrl.Result{}
			} else if condition.Type == shared.NetAttachError {
				cond.Message = fmt.Sprintf("OpenStackNet error: %s", condition.Message)
				cond.Type = shared.NetConfigError
				common.LogForObject(r, cond.Message, instance)

				return ctrl.Result{}, fmt.Errorf(cond.Message)
			}
		}
	}

	_, cidrSuffix, err := common.GetCidrParts(osNet.Spec.Cidr)
	if err != nil {
		// TODO set cond
		return ctrl.Result{}, err
	}

	for _, roleNetStatus := range osNet.Spec.RoleReservations {
		for _, roleReservation := range roleNetStatus.Reservations {
			//
			// Add net status to netcfg status if reservation exist in spec
			// and host is not deleted
			//
			if nodeReservation, ok := osNet.Status.Reservations[roleReservation.Hostname]; ok &&
				roleReservation.IP == nodeReservation.IP &&
				!nodeReservation.Deleted {

				// set/update host status

				hostStatus := ospdirectorv1beta1.OpenStackHostStatus{}
				if _, ok := instance.Status.Hosts[roleReservation.Hostname]; ok {
					hostStatus = instance.Status.Hosts[roleReservation.Hostname]
				}

				if hostStatus.IPAddresses == nil {
					hostStatus.IPAddresses = map[string]string{}
				}
				if hostStatus.OVNBridgeMacAdresses == nil {
					hostStatus.OVNBridgeMacAdresses = map[string]string{}
				}
				hostStatus.IPAddresses[osNet.Spec.NameLower] = fmt.Sprintf("%s/%d", nodeReservation.IP, cidrSuffix)
				instance.Status.Hosts[roleReservation.Hostname] = hostStatus
			} else {
				delete(instance.Status.Hosts, roleReservation.Hostname)
			}

		}
	}

	instance.Status.ProvisioningStatus.State = shared.ProvisioningState(cond.Type)
	instance.Status.ProvisioningStatus.Reason = cond.Message

	return ctrlResult, nil
}

func (r *OpenStackNetConfigReconciler) osnetCleanup(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	subnet *ospdirectorv1beta1.Subnet,
	cond *shared.Condition,
) error {
	osNet := &ospdirectorv1beta1.OpenStackNet{}

	osNet.Name = strings.ToLower(strings.Replace(subnet.Name, "_", "", -1))
	osNet.Namespace = instance.Namespace

	cond.Message = fmt.Sprintf("OpenStackNet %s delete started", osNet.Name)
	cond.Type = shared.NetConfigConfiguring

	if err := r.Delete(ctx, osNet, &client.DeleteOptions{}); err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	common.LogForObject(r, cond.Message, instance)

	return nil
}

// create or update the OpenStackMACAddress object
func (r *OpenStackNetConfigReconciler) createOrUpdateOpenStackMACAddress(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *shared.Condition,
) (*ospdirectorv1beta1.OpenStackMACAddress, error) {
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
	err := r.Get(ctx, types.NamespacedName{Name: strings.ToLower(instance.Name), Namespace: instance.Namespace}, macAddress)
	if err != nil && !k8s_errors.IsNotFound(err) {
		cond.Message = fmt.Sprintf("Failed to get %s %s ", macAddress.Kind, macAddress.Name)
		cond.Reason = shared.MACCondReasonError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return macAddress, err
	}

	// If network RoleReservations spec map is nil, create it
	if macAddress.Spec.RoleReservations == nil {
		macAddress.Spec.RoleReservations = map[string]ospdirectorv1beta1.OpenStackMACRoleReservation{}
	}

	if macAddress.Status.MACReservations == nil {
		macAddress.Status.MACReservations = map[string]ospdirectorv1beta1.OpenStackMACNodeReservation{}
	}

	//
	// get all IPsets
	//
	labelSelector := map[string]string{
		shared.OpenStackNetConfigReconcileLabel: instance.Name,
	}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelSelector),
	}

	ipSetList := &ospdirectorv1beta1.OpenStackIPSetList{}
	if err := r.GetClient().List(
		ctx,
		ipSetList,
		listOpts...,
	); err != nil && !k8s_errors.IsNotFound(err) {
		return macAddress, err
	}

	//
	// create map of ipSet referenced by owner UID
	//
	ipSetOwnerUIDRefMap := map[string]ospdirectorv1beta1.OpenStackIPSet{}
	for idx, ipSet := range ipSetList.Items {
		ipSetOwnerUIDRefMap[ipSet.Labels[common.OwnerUIDLabelSelector]] = ipSetList.Items[idx]
	}

	//
	// get all VMset
	//
	labelSelector = map[string]string{}
	listOpts = []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelSelector),
	}
	vmSetList := &ospdirectorv1beta2.OpenStackVMSetList{}
	if err := r.GetClient().List(
		ctx,
		vmSetList,
		listOpts...,
	); err != nil && !k8s_errors.IsNotFound(err) {
		return macAddress, err
	}

	//
	// get all BMset
	//
	bmSetList := &ospdirectorv1beta1.OpenStackBaremetalSetList{}
	if err := r.GetClient().List(
		ctx,
		bmSetList,
		listOpts...,
	); err != nil && !k8s_errors.IsNotFound(err) {
		return macAddress, err
	}

	//
	// Create list of all ipSets from VMset and BMset which are tripleo Roles
	//
	ipSetCreateMACList := []ospdirectorv1beta1.OpenStackIPSet{}
	for _, vmSet := range vmSetList.Items {
		if ipSet, ok := ipSetOwnerUIDRefMap[string(vmSet.GetUID())]; ok && vmSet.Spec.IsTripleoRole {
			ipSetCreateMACList = append(ipSetCreateMACList, ipSet)
		}
	}

	// Note: (mschuppert) bmSet does not yet have a IsTripleoRole parameter, but we discussed it
	for _, bmSet := range bmSetList.Items {
		if ipSet, ok := ipSetOwnerUIDRefMap[string(bmSet.GetUID())]; ok {
			ipSetCreateMACList = append(ipSetCreateMACList, ipSet)
		}
	}

	//
	// create MAC reservations for all entries in ipSetCreateMACList
	//
	reservations := map[string]ospdirectorv1beta1.OpenStackMACRoleReservation{}

	for _, ipSet := range ipSetCreateMACList {
		roleMACReservation := map[string]ospdirectorv1beta1.OpenStackMACNodeReservation{}

		// get current BMSet reservations corresbonding to the IPSet
		bmSetReservations := map[string]ospdirectorv1beta1.HostStatus{}
		for _, bmSet := range bmSetList.Items {
			if ipSet.Labels[common.OwnerUIDLabelSelector] == string(bmSet.GetUID()) {
				bmSetReservations = bmSet.Status.BaremetalHosts
			}
		}

		for _, host := range ipSet.Status.Hosts {

			roleMACReservation[host.Hostname], err = r.ensureMACReservation(
				instance,
				cond,
				macAddress,
				macAddress.Spec.RoleReservations,
				ipSet.Spec.RoleName,
				host.Hostname,
				bmSetReservations[host.Hostname].AnnotatedForDeletion,
			)
			if err != nil {
				return macAddress, err
			}
		}

		//
		// add reservations from deleted nodes if physnet.PreserveReservations
		//
		if *instance.Spec.PreserveReservations {
			r.preserveMACReservations(
				macAddress,
				&roleMACReservation,
				ipSet.Spec.RoleName,
			)
		}

		reservations[ipSet.Spec.RoleName] = ospdirectorv1beta1.OpenStackMACRoleReservation{
			Reservations: roleMACReservation,
		}

	}

	//
	// create/update OSMACAddress
	//
	op, err := controllerutil.CreateOrPatch(ctx, r.Client, macAddress, func() error {
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
			cond.Reason = shared.CommonCondReasonControllerReferenceError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return err
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", macAddress.Kind, macAddress.Name)
		cond.Reason = shared.MACCondReasonCreateMACError
		cond.Type = shared.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return macAddress, err
	}

	cond.Message = "OpenStackMACAddress CR successfully reconciled"

	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))
	}
	cond.Reason = shared.MACCondReasonAllMACAddressesCreated
	cond.Type = shared.CommonCondTypeProvisioned

	instance.Status.ProvisioningStatus.PhysNetReadyCount = len(macAddress.Spec.PhysNetworks)

	return macAddress, nil
}

func (r *OpenStackNetConfigReconciler) getMACStatus(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *shared.Condition,
	macAddress *ospdirectorv1beta1.OpenStackMACAddress,
) {

	for hostname, reservation := range macAddress.Status.MACReservations {
		if !reservation.Deleted {
			hostStatus := ospdirectorv1beta1.OpenStackHostStatus{}
			if _, ok := instance.Status.Hosts[hostname]; ok {
				hostStatus = instance.Status.Hosts[hostname]
			}

			if hostStatus.IPAddresses == nil {
				hostStatus.IPAddresses = map[string]string{}
			}
			if hostStatus.OVNBridgeMacAdresses == nil {
				hostStatus.OVNBridgeMacAdresses = map[string]string{}
			}

			hostStatus.OVNBridgeMacAdresses = reservation.Reservations

			instance.Status.Hosts[hostname] = hostStatus
		}
	}

}

// create or update the OpenStackMACAddress object
func (r *OpenStackNetConfigReconciler) ensureMACReservation(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *shared.Condition,
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
			if res, ok := instance.Spec.Reservations[hostname]; ok && res.MACReservations[physnet.Name] != "" {
				nodeMACReservation.Reservations[physnet.Name] = res.MACReservations[physnet.Name]
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
						instance,
						macAddress,
					),
					newMAC,
				) {
					newMAC, err = macaddress.CreateMACWithPrefix(physnet.MACPrefix)
					if err != nil {
						cond.Message = err.Error()
						cond.Reason = shared.MACCondReasonCreateMACError
						cond.Type = shared.MACCondTypeCreating
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

// allMACReservations - get all reservations from static + dynamic created
func (r *OpenStackNetConfigReconciler) allMACReservations(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	macAddress *ospdirectorv1beta1.OpenStackMACAddress,
) map[string]ospdirectorv1beta1.OpenStackMACNodeReservation {
	reservations := macAddress.Status.MACReservations

	for node, res := range instance.Spec.Reservations {
		if currentRes, ok := reservations[node]; !ok {
			reservations[node] = ospdirectorv1beta1.OpenStackMACNodeReservation{
				Reservations: res.MACReservations,
			}
		} else {
			currentRes.Reservations = res.MACReservations
			reservations[node] = currentRes

		}
	}

	return reservations
}

// add reservations from deleted nodes if Spec.PreserveReservations
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

// create or update the OpenStackMACAddress object
func (r *OpenStackNetConfigReconciler) ensureIPReservation(
	ctx context.Context,
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *shared.Condition,
	osNet *ospdirectorv1beta1.OpenStackNet,
) (map[string]ospdirectorv1beta1.OpenStackNetRoleReservation, error) {

	//
	// * get objects which use the network
	// * verify for all nodes of the role if they have reservation on the network
	// * if a host got added to the network create IP and update the network
	//   * otherwise take existing reservation
	//   * TODO static reservation
	//

	// reduce object scope by limit to the added name_lower network label
	labelSelector := map[string]string{
		fmt.Sprintf("%s/%s", shared.SubNetNameLabelSelector, osNet.Spec.NameLower): strconv.FormatBool(true),
	}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelSelector),
	}

	allRoles := map[string]bool{}

	//
	// get all OSIPsets
	//
	osIPsetList := &ospdirectorv1beta1.OpenStackIPSetList{}
	if err := r.GetClient().List(
		ctx,
		osIPsetList,
		listOpts...,
	); err != nil && !k8s_errors.IsNotFound(err) {
		return nil, err
	}

	for _, osIPset := range osIPsetList.Items {

		roleName := osIPset.Spec.RoleName

		//
		// For backward compatability check if owning object is osClient and set Role to <openstackclient.Role><instance.Name>
		//
		osClient := &ospdirectorv1beta1.OpenStackClient{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      osIPset.Labels[common.OwnerNameLabelSelector],
			Namespace: osIPset.Namespace},
			osClient)
		if err != nil {
			if !k8s_errors.IsNotFound(err) {
				cond.Message = fmt.Sprintf("Failed to get %s %s ", osClient.Kind, osClient.Name)
				cond.Reason = shared.OsClientCondReasonError
				cond.Type = shared.CommonCondTypeError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return nil, err
			}
		} else {
			roleName = fmt.Sprintf("%s%s", openstackclient.Role, osIPset.Spec.RoleName)
		}

		allRoles[roleName] = true

		//
		// are there new networks added to the CR?
		//
		err = r.ensureIPs(
			instance,
			cond,
			osNet,
			roleName,
			common.SortMapByValue(osIPset.GetHostnames()),
			osIPset.Spec.VIP,
			osIPset.Spec.ServiceVIP,
			osIPset.Spec.AddToPredictableIPs,
		)
		if err != nil {
			return nil, err
		}
	}

	//
	// cleanup reservations from osnet RoleReservations if
	// * role is not in allRoles (ipset deleted)
	// * and PreserveReservations is false
	//
	// if PreserveReservations is true, the reservations in the osnet just get flipped to deleted = true
	for role := range osNet.Spec.RoleReservations {
		if _, ok := allRoles[role]; !ok {
			if !*instance.Spec.PreserveReservations {
				delete(osNet.Spec.RoleReservations, role)
				continue
			}

			// if a role got fully deleted, mark all reservations in the osnet spec as deleted
			for idx, reservation := range osNet.Spec.RoleReservations[role].Reservations {
				reservation.Deleted = true
				osNet.Spec.RoleReservations[role].Reservations[idx] = reservation
			}
		}
	}

	return osNet.Spec.RoleReservations, nil
}

// ensureIPs - create IP reservations for all nodes of a role.
// If there is already an existing IP its being reused
func (r *OpenStackNetConfigReconciler) ensureIPs(
	instance *ospdirectorv1beta1.OpenStackNetConfig,
	cond *shared.Condition,
	osNet *ospdirectorv1beta1.OpenStackNet,
	roleName string,
	allRoleHosts common.List,
	vip bool,
	serviceVIP bool,
	addToPredictableIPs bool,
) error {

	reservations := []ospdirectorv1beta1.IPReservation{}

	//
	// Get static IP reservations from CR for this osNet
	//
	currentReservations := osNet.Status.Reservations
	staticReservations := []ospdirectorv1beta1.IPReservation{}
	if len(instance.Spec.Reservations) > 0 {
		for nodeName, nodeReservations := range instance.Spec.Reservations {
			if nodeNetIPReservation, ok := nodeReservations.IPReservations[osNet.Spec.NameLower]; ok {
				currentReservations[nodeName] = ospdirectorv1beta1.NodeIPReservation{
					IP: nodeNetIPReservation,
				}
				staticReservations = append(
					staticReservations,
					ospdirectorv1beta1.IPReservation{
						IP:         nodeNetIPReservation,
						Hostname:   nodeName,
						ServiceVIP: serviceVIP,
						VIP:        vip,
					},
				)
			}
		}
	}

	_, cidr, err := net.ParseCIDR(osNet.Spec.Cidr)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to parse CIDR %s", osNet.Spec.Cidr)
		cond.Reason = shared.CommonCondReasonCIDRParseError
		cond.Type = shared.NetConfigError
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return err
	}

	start := net.ParseIP(osNet.Spec.AllocationStart)
	end := net.ParseIP(osNet.Spec.AllocationEnd)

	for _, host := range allRoleHosts {

		hostname := host.Key

		//
		// Do we already have a reservation for this hostname on the network?
		//
		if reservation, ok := currentReservations[hostname]; ok {

			nodeReservation := ospdirectorv1beta1.IPReservation{
				IP:         reservation.IP,
				Hostname:   hostname,
				VIP:        vip,
				ServiceVIP: serviceVIP,
				Deleted:    false,
			}

			found := false
			for _, res := range reservations {
				if res.Hostname == hostname {
					found = true
					break
				}
			}
			if !found {
				reservations = append(reservations, nodeReservation)
			}

			common.LogForObject(
				r,
				fmt.Sprintf("Re-use existing reservation for host %s in network %s with IP %s",
					hostname,
					osNet.Spec.NameLower,
					reservation.IP),
				instance,
			)

		} else {
			//
			// No reservation found, so create a new one
			//
			var ip net.IPNet
			ip, reservations, err = common.AssignIP(common.AssignIPDetails{
				IPnet:           *cidr,
				RangeStart:      start,
				RangeEnd:        end,
				RoleReservelist: reservations,
				Reservelist: openstacknet.GetAllIPReservations(
					osNet,
					reservations,
					staticReservations,
				),
				ExcludeRanges: []string{},
				Hostname:      hostname,
				VIP:           vip,
				Deleted:       false,
			})
			if err != nil {
				cond.Message = fmt.Sprintf("Failed to do ip reservation: %s", hostname)
				cond.Reason = shared.NetConfigCondReasonIPReservationError
				cond.Type = shared.NetConfigError
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return err
			}

			common.LogForObject(
				r,
				fmt.Sprintf("Created new reservation for host %s in network %s with IP %s",
					hostname,
					osNet.Spec.NameLower,
					ip.String()),
				instance,
			)
		}
	}

	//
	// Add reservations for delete nodes of the role if Spec.PreserveReservations
	//
	if *instance.Spec.PreserveReservations {
		for _, oldReservation := range osNet.Spec.RoleReservations[roleName].Reservations {
			found := false
			for _, newReservation := range reservations {
				if oldReservation.Hostname == newReservation.Hostname {
					found = true
					continue
				}
			}
			if !found {
				oldReservation.Deleted = true
				reservations = append(reservations, oldReservation)
			}
		}
	}

	//
	// If there are reservations for the role, store them on the OpenStackNet
	//
	if len(reservations) > 0 {
		sort.Slice(reservations[:], func(i, j int) bool {
			return reservations[i].Hostname < reservations[j].Hostname
		})

		osNet.Spec.RoleReservations[roleName] = ospdirectorv1beta1.OpenStackNetRoleReservation{
			AddToPredictableIPs: addToPredictableIPs,
			Reservations:        reservations,
		}
	}

	cond.Message = fmt.Sprintf("IP reservations for role %s created", roleName)
	cond.Reason = shared.NetConfigCondReasonIPReservation
	cond.Type = shared.NetConfigConfigured

	return nil
}

// getNetDesiredCount - get the total of all networks subnets
func (r *OpenStackNetConfigReconciler) getNetDesiredCount(
	networks []ospdirectorv1beta1.Network,
) int {
	desiredCount := 0
	for _, net := range networks {
		desiredCount += len(net.Subnets)
	}

	return desiredCount
}
