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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
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

	// initilize MACReservations
	if instance.Status.MACReservations == nil {
		instance.Status.MACReservations = map[string]ospdirectorv1beta1.OpenStackMACNodeStatus{}
	}

	if instance.Status.Conditions == nil {
		instance.Status.Conditions = ospdirectorv1beta1.ConditionList{}
	}

	// get a copy if the CR MAC Reservations
	actualStatus := instance.DeepCopy().Status

	// get ctlplane osnet as every vm/bm host is connected to it
	ctlplaneNet := &ospdirectorv1beta1.OpenStackNet{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "ctlplane", Namespace: instance.Namespace}, ctlplaneNet)
	if err != nil {
		// if ctlplaneNet does not exist, wait for it and set Condition to waiting
		if k8s_errors.IsNotFound(err) {
			msg := "Waiting for ctlplane network to be created"
			actualStatus.CurrentState = ospdirectorv1beta1.MACWaiting
			actualStatus.Conditions.Set(ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACWaiting), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(msg), msg)
			_ = r.setStatus(instance, actualStatus)
			return ctrl.Result{Requeue: true}, nil
		}
		msg := "Error fetching the ctlplane network"
		actualStatus.CurrentState = ospdirectorv1beta1.MACError
		actualStatus.Conditions.Set(ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACError), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(msg), msg)
		_ = r.setStatus(instance, actualStatus)
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
					if _, ok := actualStatus.MACReservations[reservation.Hostname]; !ok {
						actualStatus.MACReservations[reservation.Hostname] = ospdirectorv1beta1.OpenStackMACNodeStatus{
							Deleted:      false,
							Reservations: map[string]string{},
						}
					}

					macNodeStatus := actualStatus.MACReservations[reservation.Hostname]

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
										msg := "Waiting for all MAC addresses to get created"
										actualStatus.CurrentState = ospdirectorv1beta1.MACCreating
										actualStatus.Conditions.Set(ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACCreating), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(msg), msg)
										_ = r.setStatus(instance, actualStatus)
										return ctrl.Result{}, err
									}
									actualStatus.ReservedMACCount++
									r.Log.Info(fmt.Sprintf("New MAC created - node: %s, physnet: %s, mac: %s", reservation.Hostname, physnet, newMAC))
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

					actualStatus.MACReservations[reservation.Hostname] = macNodeStatus
				}
			}
		}
	}

	// When a role get delted, the IPs get freed up in the osnet.
	// Verify if any node got removed from network status and remove
	// the MAC reservation.
	var remove bool
	for node, reservation := range actualStatus.MACReservations {
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
			delete(actualStatus.MACReservations, node)
		}
	}

	msg := "All MAC addresses created"
	actualStatus.CurrentState = ospdirectorv1beta1.MACConfigured
	actualStatus.Conditions.Set(ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.MACConfigured), corev1.ConditionTrue, ospdirectorv1beta1.ConditionReason(msg), msg)
	err = r.setStatus(instance, actualStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

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

func (r *OpenStackMACAddressReconciler) setStatus(instance *ospdirectorv1beta1.OpenStackMACAddress, actualState ospdirectorv1beta1.OpenStackMACAddressStatus) error {

	if !reflect.DeepEqual(instance.Status, actualState) {
		instance.Status = actualState
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			r.Log.Error(err, "OpenStackMACAddress update status error: %v")
			return err
		}
	}
	return nil
}
