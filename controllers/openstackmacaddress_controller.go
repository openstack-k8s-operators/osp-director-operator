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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipset,verbs=get;list;watch

// Reconcile -
func (r *OpenStackMACAddressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackmacaddress", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackMACAddress{}
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

	if instance.Status.MACReservations == nil {
		instance.Status.MACReservations = map[string]ospdirectorv1beta1.OpenStackMACNodeStatus{}
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

		if statusChanged() {
			if updateErr := r.Client.Status().Update(context.Background(), instance); updateErr != nil {
				if err == nil {
					err = common.WrapErrorForObject(
						"Update Status", instance, updateErr)
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
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(instance, macaddress.FinalizerName) {
			controllerutil.AddFinalizer(instance, macaddress.FinalizerName)
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
			r.Log.Info(fmt.Sprintf("Finalizer %s added to CR %s", macaddress.FinalizerName, instance.Name))
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, macaddress.FinalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, macaddress.FinalizerName)
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, instance.Status.CurrentState == ospdirectorv1beta1.MACCondTypeConfigured)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackMACAddress %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	// get ctlplane osnet as every vm/bm host is connected to it
	ctlplaneNet := &ospdirectorv1beta1.OpenStackNet{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "ctlplane", Namespace: instance.Namespace}, ctlplaneNet)
	if err != nil {
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonNetNotFound)

		// if ctlplaneNet does not exist, wait for it and set Condition to waiting
		if k8s_errors.IsNotFound(err) {
			cond.Message = "Waiting for ctlplane network to be created"
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACCondTypeWaiting)
			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{RequeueAfter: 20 * time.Second}, nil
		}
		cond.Message = "Error fetching the ctlplane network"
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACCondTypeError)
		err = common.WrapErrorForObject(cond.Message, instance, err)

		return ctrl.Result{}, err
	}

	// create reservation for physnet and each node
	for _, roleStatus := range ctlplaneNet.Spec.RoleReservations {

		// if addToPredictableIPs == false the role can be ignored for creating MAC
		// this is e.g. the openstackclient pod
		if roleStatus.AddToPredictableIPs {
			for _, reservation := range roleStatus.Reservations {
				// do not create MAC reservation for VIP
				if !reservation.VIP {
					// if node does not yet have a reservation, create the entry and set deleted -> false
					if _, ok := instance.Status.MACReservations[reservation.Hostname]; !ok {
						instance.Status.MACReservations[reservation.Hostname] = ospdirectorv1beta1.OpenStackMACNodeStatus{
							Deleted:      false,
							Reservations: map[string]string{},
						}
					}

					macNodeStatus := instance.Status.MACReservations[reservation.Hostname]

					// ignore deleted nodes
					if !reservation.Deleted {
						macNodeStatus.Deleted = false
						// create reservation for every specified physnet
						for _, physnet := range instance.Spec.PhysNetworks {
							// if there is no reservation for the physnet, create one
							if _, ok := macNodeStatus.Reservations[physnet.Name]; !ok {
								// create MAC address and verify it is uniqe in the CR reservations
								var newMAC string
								for ok := true; ok; ok = !macaddress.IsUniqMAC(instance.Status.MACReservations, newMAC) {
									newMAC, err = macaddress.CreateMACWithPrefix(physnet.MACPrefix)
									if err != nil {
										cond.Message = "Waiting for all MAC addresses to get created"
										cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonCreateMACError)
										cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACCondTypeCreating)
										err = common.WrapErrorForObject(cond.Message, instance, err)

										return ctrl.Result{}, err
									}
									instance.Status.ReservedMACCount++

									common.LogForObject(
										r,
										fmt.Sprintf("New MAC created - node: %s, physnet: %s, mac: %s", reservation.Hostname, physnet, newMAC),
										instance,
									)
								}

								macNodeStatus.Reservations[physnet.Name] = newMAC
							}
						}

					} else {
						// If the node is flagged as deleted in the osnet, also mark the
						// MAC reservation as deleted. If the node gets recreated with the
						// same name, it reuses the MAC.
						macNodeStatus.Deleted = true
					}

					instance.Status.MACReservations[reservation.Hostname] = macNodeStatus
				}
			}
		}
	}

	// When a role get delted, the IPs get freed up in the osnet.
	// Verify if any node got removed from network status and remove
	// the MAC reservation.
	var remove bool
	for node, reservation := range instance.Status.MACReservations {
		remove = true
		for _, roleStatus := range ctlplaneNet.Spec.RoleReservations {
			for _, netReservation := range roleStatus.Reservations {
				if node == netReservation.Hostname {
					remove = false
				}
			}
		}

		if remove {
			r.Log.Info(fmt.Sprintf("Delete MAC reservation - node %s, %s", node, reservation.Reservations))
			delete(instance.Status.MACReservations, node)
		}
	}

	cond.Message = "All MAC addresses created"
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.MACCondReasonAllMACAddressesCreated)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACCondTypeConfigured)

	instance.Status.CurrentState = ospdirectorv1beta1.MACCondTypeConfigured

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
		if err := r.Client.List(context.Background(), crs, listOpts...); err != nil {
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
