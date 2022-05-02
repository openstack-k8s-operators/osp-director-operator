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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
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

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	macaddress "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackmacaddress"
)

// OpenStackMACAddressReconciler reconciles a OpenStackMACAddress object
type OpenStackMACAddressReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackMACAddressReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackMACAddressReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackMACAddressReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackMACAddressReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackmacaddresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackmacaddresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackmacaddresses/finalizers,verbs=update
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets,verbs=get;list;watch

// Reconcile -
func (r *OpenStackMACAddressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackmacaddress", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackMACAddress{}
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

	if instance.Status.MACReservations == nil {
		instance.Status.MACReservations = map[string]ospdirectorv1beta1.OpenStackMACNodeReservation{}
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
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, macaddress.FinalizerName) {
			controllerutil.AddFinalizer(instance, macaddress.FinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return reconcile.Result{}, err
			}
			common.LogForObject(r, fmt.Sprintf("Finalizer %s added to CR %s", macaddress.FinalizerName, instance.Name), instance)
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, macaddress.FinalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, macaddress.FinalizerName)
		err = r.Update(ctx, instance)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
			cond.Reason = shared.CommonCondReasonRemoveFinalizerError
			cond.Type = shared.CommonCondTypeError

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.IsReady())

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackMACAddress %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	//
	// Update status with reservation count and flattened reservations
	//
	reservedMACCount := 0
	reservations := map[string]ospdirectorv1beta1.OpenStackMACNodeReservation{}
	for _, roleReservation := range instance.Spec.RoleReservations {
		for nodeName, nodeReservation := range roleReservation.Reservations {
			reservedMACCount += len(nodeReservation.Reservations)
			reservations[nodeName] = nodeReservation
		}
	}

	instance.Status.ReservedMACCount = reservedMACCount
	instance.Status.MACReservations = reservations

	cond.Message = "All MAC addresses created"
	cond.Reason = shared.MACCondReasonAllMACAddressesCreated
	cond.Type = shared.MACCondTypeConfigured

	instance.Status.CurrentState = shared.MACCondTypeConfigured

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackMACAddressReconciler) SetupWithManager(mgr ctrl.Manager) error {

	namespacedFn := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all CRs from the same namespace, right now there should only be one
		crs := &ospdirectorv1beta1.OpenStackMACAddressList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.List(context.TODO(), crs, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve CRs %v")
			return nil
		}

		for _, cr := range crs.Items {
			if o.GetNamespace() == cr.Namespace {
				// return namespace and Name of CR
				name := client.ObjectKey{
					Namespace: cr.Namespace,
					Name:      cr.Name,
				}
				result = append(result, reconcile.Request{NamespacedName: name})
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackMACAddress{}).
		// Watch OpenStackNet to create/delete MAC
		Watches(&source.Kind{Type: &ospdirectorv1beta1.OpenStackNet{}}, namespacedFn).
		Complete(r)
}

func (r *OpenStackMACAddressReconciler) getNormalizedStatus(status *ospdirectorv1beta1.OpenStackMACAddressStatus) *ospdirectorv1beta1.OpenStackMACAddressStatus {

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
