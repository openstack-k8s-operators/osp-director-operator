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
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknetconfig "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetconfig"
)

// OpenStackIPSetReconciler reconciles a OpenStackIPSet object
type OpenStackIPSetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackIPSetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackIPSetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackIPSetReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackIPSetReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/finalizers,verbs=update

// Reconcile - controller IPsets
func (r *OpenStackIPSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch the controller IPset instance
	instance := &ospdirectorv1beta1.OpenStackIPSet{}
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

	// If IPSet status map is nil, create it
	if instance.Status.Hosts == nil {
		instance.Status.Hosts = map[string]ospdirectorv1beta1.HostStatus{}
	}
	instance.Status.Reserved = 0
	instance.Status.Networks = len(instance.Spec.Networks)

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

		if statusChanged() {
			if updateErr := r.Status().Update(context.Background(), instance); updateErr != nil {
				common.LogErrorForObject(r, updateErr, "Update status", instance)
			}
		}

		// log current status message to operator log
		common.LogForObject(r, cond.Message, instance)
	}(cond)

	var ctrlResult ctrl.Result
	currentLabels := instance.DeepCopy().Labels

	//
	// Only kept for running local
	// add osnetcfg CR label reference which is used in the in the osnetcfg
	// controller to watch this resource and reconcile
	//
	if _, ok := currentLabels[ospdirectorv1beta1.OpenStackNetConfigReconcileLabel]; !ok {
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
	instance.Labels = ospdirectorv1beta1.AddOSNetNameLowerLabels(
		r.GetLogger(),
		instance.Labels,
		instance.Spec.Networks,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

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
	// delete hostnames if there are any in spec
	//
	for _, deletedHost := range instance.Spec.DeletedHosts {
		delete(instance.Status.Hosts, deletedHost)
	}

	//
	// create hostnames
	//
	_, err = r.createNewHostnames(
		instance,
		cond,
		instance.Spec.HostCount-len(instance.Status.Hosts),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// get OSNetCfg object
	//
	osNetCfg, err := ospdirectorv1beta1.GetOsNetCfg(r.GetClient(), instance.GetNamespace(), instance.GetLabels()[ospdirectorv1beta1.OpenStackNetConfigReconcileLabel])
	if err != nil {
		cond.Type = shared.CommonCondTypeError
		cond.Reason = shared.NetConfigCondReasonError
		cond.Message = fmt.Sprintf("error getting OpenStackNetConfig %s: %s",
			instance.GetLabels()[shared.OpenStackNetConfigReconcileLabel],
			err)

		return ctrl.Result{}, err
	}

	//
	// Wait for IPs created on all configured networks
	//
	for hostname, hostStatus := range instance.Status.Hosts {
		err = openstacknetconfig.WaitOnIPsCreated(
			r,
			instance,
			cond,
			osNetCfg,
			instance.Spec.Networks,
			hostname,
			&hostStatus,
		)
		if err != nil {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		//
		// check if owning object is OSBMS
		//
		osBms := &ospdirectorv1beta1.OpenStackBaremetalSet{}
		err = r.Get(ctx, types.NamespacedName{
			Name:      instance.Labels[common.OwnerNameLabelSelector],
			Namespace: instance.Namespace},
			osBms)
		if err != nil && !k8s_errors.IsNotFound(err) {
			cond.Message = fmt.Sprintf("Failed to get %s %s ", osBms.Kind, osBms.Name)
			cond.Reason = shared.BaremetalSetCondReasonError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}

		// Can not set (like in vmset, osclient, osctlplane) HostRef for BMS to hostname as it references the used BMH host
		// if owning object is not osbms
		if k8s_errors.IsNotFound(err) {
			hostStatus.HostRef = hostname
		} else {

			// Get openshift-machine-api BaremetalHosts with
			baremetalHostsList := &metal3v1alpha1.BareMetalHostList{}

			labelSelector := map[string]string{
				common.OSPHostnameLabelSelector: hostname,
			}

			listOpts := []client.ListOption{
				client.InNamespace("openshift-machine-api"),
				client.MatchingLabels(labelSelector),
			}

			err := r.GetClient().List(ctx, baremetalHostsList, listOpts...)
			if err != nil && !k8s_errors.IsNotFound(err) {
				cond.Message = "Failed to get list of all BareMetalHost(s)"
				cond.Reason = shared.BaremetalHostCondReasonListError
				cond.Type = shared.BaremetalSetCondTypeError

				return reconcile.Result{}, err
			}

			for _, bmh := range baremetalHostsList.Items {
				hostStatus.HostRef = bmh.Name
			}

		}

		instance.Status.Hosts[hostname] = hostStatus
		instance.Status.Reserved++
	}

	for _, hostStatus := range instance.Status.Hosts {
		// if there is not yet an assigned BMH host, schedule reconcile
		if hostStatus.HostRef == ospdirectorv1beta1.HostRefInitState {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	cond.Type = shared.CommonCondTypeProvisioned
	cond.Reason = shared.IPSetCondReasonProvisioned
	cond.Message = "All requested IPs have been reserved"

	return ctrl.Result{}, nil
}

func (r *OpenStackIPSetReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackIPSetStatus) *ospdirectorv1beta1.OpenStackIPSetStatus {

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
func (r *OpenStackIPSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackIPSet{}).
		Complete(r)
}

//
// create hostnames for the requested number of systems
//
func (r *OpenStackIPSetReconciler) createNewHostnames(
	instance *ospdirectorv1beta1.OpenStackIPSet,
	cond *shared.Condition,
	newCount int,
) ([]string, error) {
	newHostnames := []string{}

	// create hostnames with no index if it is a VIP or service VIP
	vip := false
	if instance.Spec.VIP || instance.Spec.ServiceVIP {
		vip = true
	}

	if instance.Status.Hosts == nil {
		instance.Status.Hosts = map[string]ospdirectorv1beta1.HostStatus{}
	}

	//
	//   create hostnames for the newCount
	//
	currentNetStatus := instance.Status.DeepCopy().Hosts
	for i := 0; i < newCount; i++ {
		hostnameDetails := common.Hostname{
			Basename: instance.Spec.RoleName,
			VIP:      vip,
		}

		err := common.CreateOrGetHostname(instance, &hostnameDetails)
		if err != nil {
			cond.Message = fmt.Sprintf("error creating new hostname %v", hostnameDetails)
			cond.Reason = shared.CommonCondReasonNewHostnameError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return newHostnames, err
		}

		if hostnameDetails.Hostname != "" {
			if _, ok := instance.Status.Hosts[hostnameDetails.Hostname]; !ok {
				instance.Status.Hosts[hostnameDetails.Hostname] = ospdirectorv1beta1.HostStatus{
					Hostname:             hostnameDetails.Hostname,
					HostRef:              hostnameDetails.HostRef,
					AnnotatedForDeletion: false,
					IPAddresses:          map[string]string{},
				}
				newHostnames = append(newHostnames, hostnameDetails.Hostname)
			}

			common.LogForObject(
				r,
				fmt.Sprintf("%s hostname created: %s", instance.Kind, hostnameDetails.Hostname),
				instance,
			)
		}
	}

	if !reflect.DeepEqual(currentNetStatus, instance.Status.Hosts) {
		common.LogForObject(
			r,
			fmt.Sprintf("Updating CR status with new hostname information, %d new - %s",
				len(newHostnames),
				diff.ObjectReflectDiff(currentNetStatus, instance.Status.Hosts),
			),
			instance,
		)

		err := r.Status().Update(context.Background(), instance)
		if err != nil {
			cond.Message = "Failed to update CR status for new hostnames"
			cond.Reason = shared.CommonCondReasonCRStatusUpdateError
			cond.Type = shared.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return newHostnames, err
		}
	}

	return newHostnames, nil
}
