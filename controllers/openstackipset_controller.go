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
	"strings"
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

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	corev1 "k8s.io/api/core/v1"
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

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackipsets/status,verbs=get;update;patch

// Reconcile - reconcile OpenStackIPSet objects
func (r *OpenStackIPSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackipset", req.NamespacedName)

	// Fetch the controller IPSet instance
	instance := &ospdirectorv1beta1.OpenStackIPSet{}
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

	if instance.Status.HostIPs == nil {
		instance.Status.HostIPs = make(map[string]ospdirectorv1beta1.OpenStackIPHostsStatus)
	}

	if instance.Status.Networks == nil {
		instance.Status.Networks = make(map[string]ospdirectorv1beta1.NetworkStatus)
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
		if !controllerutil.ContainsFinalizer(instance, common.FinalizerName) {
			controllerutil.AddFinalizer(instance, common.FinalizerName)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
			common.LogForObject(r, fmt.Sprintf("Finalizer %s added to CR %s", common.FinalizerName, instance.Name), instance)
		}
	} else {
		// 1. check if finalizer is there
		// Reconcile if finalizer got already removed
		if !controllerutil.ContainsFinalizer(instance, common.FinalizerName) {
			return ctrl.Result{}, nil
		}

		// 2. Clean up resources
		// remove osnet status from references of this CR

		err := r.cleanupOSNetStatus(instance)
		if err != nil && !k8s_errors.IsNotFound(err) {
			// ignore not found errors if the object is already gone
			return ctrl.Result{}, err
		}

		// 3. as last step remove the finalizer on the operator CR to finish delete
		controllerutil.RemoveFinalizer(instance, common.FinalizerName)
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			cond.Message = fmt.Sprintf("Failed to update %s %s", instance.Kind, instance.Name)
			cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.CommonCondReasonRemoveFinalizerError)
			cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.CommonCondTypeError)

			err = common.WrapErrorForObject(cond.Message, instance, err)

			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// If we determine that a backup is overriding this reconcile, requeue after a longer delay
	overrideReconcile, err := ospdirectorv1beta1.OpenStackBackupOverridesReconcile(r.Client, instance.Namespace, false)

	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideReconcile {
		r.Log.Info(fmt.Sprintf("OpenStackIPSet %s reconcile overridden due to OpenStackBackupRequest(s) state; requeuing after 20 seconds", instance.Name))
		return ctrl.Result{RequeueAfter: time.Duration(20) * time.Second}, err
	}

	//
	// Verify if all required networks for the OSIPset exist
	//
	ctrlResult, err := r.verifyNetworksExist(instance, cond)
	if err != nil {
		return ctrlResult, err
	}

	//
	// Remove existing IPSet entries if nodes got removed from instance.Spec.HostNameRefs
	//
	r.removeIP(instance, cond)

	//
	// Add new IPSet entries if nodes got added to instance.Spec.HostNameRefs
	//
	ctrlResult, err = r.addIP(
		instance,
		cond,
	)
	if err != nil {
		return ctrlResult, err
	}

	//
	// sync deleted IPSet with OSNet
	//
	err = r.syncDeletedIPs(instance)
	if err != nil {
		// TODO: sync with spec
		err = common.WrapErrorForObject("Failed to sync deleted OpenStackIPSet with OpenStackNet spec", instance, err)

		return ctrl.Result{}, err
	}

	cond.Message = fmt.Sprintf("OpenStackIPSet %s has successfully configured all IPs for %v host(s)", instance.Name, instance.Spec.HostCount)
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.IPSetCondReasonSuccessfullyConfigured)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.IPSetCondTypeConfigured)

	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *OpenStackIPSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackIPSet{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
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

func (r *OpenStackIPSetReconciler) removeIP(
	instance *ospdirectorv1beta1.OpenStackIPSet,
	cond *ospdirectorv1beta1.Condition,
) {
	removedHosts := []string{}
	// create list of removed hosts from the requesting CR
	for hostname := range instance.Status.HostIPs {
		if _, ok := instance.Spec.HostNameRefs[hostname]; !ok {
			removedHosts = append(removedHosts, hostname)
		}
	}

	// if there are removed nodes, remove them from OSIPSet and OSNet
	if len(removedHosts) > 0 {
		// make sure removedHosts slice is sorted
		sort.Strings(removedHosts)

		// remove entry from OSIPSet status
		for _, hostname := range removedHosts {
			delete(instance.Status.HostIPs, hostname)
			common.LogForObject(
				r,
				fmt.Sprintf("Hostname %s removed, new host list %v", hostname, reflect.ValueOf(instance.Status.HostIPs).MapKeys()),
				instance,
			)
		}

		cond.Message = fmt.Sprintf("Removed hosts: %s", removedHosts)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.IPSetCondReasonRemovedIPs)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.IPSetCondTypeConfigured)
	}
}

func (r *OpenStackIPSetReconciler) syncDeletedIPs(instance *ospdirectorv1beta1.OpenStackIPSet) error {

	allActiveIPSetHosts := make([]string, len(instance.Status.HostIPs))
	for hostname := range instance.Status.HostIPs {
		allActiveIPSetHosts = append(allActiveIPSetHosts, hostname)
	}

	// mark entry in OSNet status as deleted
	for _, netName := range instance.Spec.Networks {
		// get network with name_lower label
		network, err := openstacknet.GetOpenStackNetWithLabel(
			r,
			instance.Namespace,
			map[string]string{
				openstacknet.SubNetNameLabelSelector: netName,
			},
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				r.Log.Info(fmt.Sprintf("OpenStackNet with NameLower %s not found!", netName))
				continue
			}
			// Error reading the object - requeue the request.
			return err
		}

		// mark the reservation for the hosts as deleted
		for index, reservation := range network.Spec.RoleReservations[instance.Spec.RoleName].Reservations {

			// set OSNet deleted flag for non exiting nodes in the ipset
			reservationDeleted := !common.StringInSlice(reservation.Hostname, allActiveIPSetHosts)

			// mark OSNet host entry as deleted
			if network.Spec.RoleReservations[instance.Spec.RoleName].Reservations[index].Deleted != reservationDeleted {
				network.Spec.RoleReservations[instance.Spec.RoleName].Reservations[index].Deleted = reservationDeleted
				r.Log.Info(fmt.Sprintf("Update network status of host %s - net %s - %v", reservation.Hostname, network.Name, network.Spec.RoleReservations[instance.Spec.RoleName].Reservations[index].Deleted))
			}
		}

		// update status of OSNet
		// TODO: just patch?
		err = r.Client.Update(context.TODO(), network)
		if err != nil {
			r.Log.Error(err, "Failed to update OpenStackNet spec %v")
			return err
		}

	}

	return nil
}

func (r *OpenStackIPSetReconciler) addIP(
	instance *ospdirectorv1beta1.OpenStackIPSet,
	cond *ospdirectorv1beta1.Condition,
) (ctrl.Result, error) {
	// Are there any new nodes ?
	newHosts := map[string]string{}
	// create list of new hosts from the requesting CR
	for hostname, hostref := range instance.Spec.HostNameRefs {
		if _, ok := instance.Status.HostIPs[hostname]; !ok {
			newHosts[hostname] = hostref
		}
	}

	// if there are new nodes, create OSIPSet entry and update OSNet
	if len(newHosts) > 0 {
		newHostsSorted := common.SortMapByValue(newHosts)

		common.LogForObject(
			r,
			fmt.Sprintf("New hosts: %s", newHostsSorted),
			instance,
		)

		cond.Message = fmt.Sprintf("New hosts: %s", newHostsSorted)
		cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.IPSetCondReasonNewHosts)
		cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.IPSetCondTypeConfiguring)
	}

	// TODO: change order to loop over networks and then nested over the hostlist to limit osnet updates to full list of new reservations added
	for _, host := range common.SortMapByValue(newHosts) {
		hostname := host.Key

		hostNetworkIPs := map[string]string{}

		// iterate over the requested Networks
		for _, netName := range instance.Spec.Networks {
			// get network with name_lower label
			network, err := openstacknet.GetOpenStackNetWithLabel(
				r,
				instance.Namespace,
				map[string]string{
					openstacknet.SubNetNameLabelSelector: netName,
				},
			)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					cond.Message = fmt.Sprintf("OpenStackNet with NameLower %s not found!", netName)
					cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.IPSetCondReasonNetNotFound)
					cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.IPSetCondTypeError)
					err = common.WrapErrorForObject(cond.Message, instance, err)

					return ctrl.Result{RequeueAfter: 20 * time.Second}, err
				}
				// Error reading the object - requeue the request.
				return ctrl.Result{}, err
			}

			// set ipset NetworkStatus
			instance.Status.Networks[network.Name] = ospdirectorv1beta1.NetworkStatus{
				Cidr:            network.Spec.Cidr,
				Vlan:            network.Spec.Vlan,
				AllocationStart: network.Spec.AllocationStart,
				AllocationEnd:   network.Spec.AllocationEnd,
				Gateway:         network.Spec.Gateway,
			}

			_, cidr, err := net.ParseCIDR(network.Spec.Cidr)
			if err != nil {
				cond.Message = fmt.Sprintf("Failed to parse CIDR %s", network.Spec.Cidr)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.IPSetCondReasonNetNotFound)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.IPSetCondTypeError)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{}, err
			}

			start := net.ParseIP(network.Spec.AllocationStart)
			end := net.ParseIP(network.Spec.AllocationEnd)

			// If network RoleReservations spec map is nil, create it
			if network.Spec.RoleReservations == nil {
				network.Spec.RoleReservations = map[string]ospdirectorv1beta1.OpenStackNetRoleStatus{}

			}
			// If network RoleReservations[instance.Spec.RoleName] Spec map is nil, create it
			if _, ok := network.Spec.RoleReservations[instance.Spec.RoleName]; !ok {
				network.Spec.RoleReservations[instance.Spec.RoleName] = ospdirectorv1beta1.OpenStackNetRoleStatus{}
			}

			reservationIP := ""

			// Do we already have a reservation for this hostname on the network?
			for _, reservation := range network.Spec.RoleReservations[instance.Spec.RoleName].Reservations {
				if reservation.Hostname == hostname {
					// We also need the netmask (which is not stored in the OpenStackNet spec),
					// so we acquire it from the OpenStackIPSet spec
					cidrPieces := strings.Split(network.Spec.Cidr, "/")
					reservationIP = fmt.Sprintf("%s/%s", reservation.IP, cidrPieces[len(cidrPieces)-1])

					common.LogForObject(
						r,
						fmt.Sprintf("Re-use existing reservation for host %s in network %s with IP %s", hostname, netName, reservationIP),
						instance,
					)

					break
				}
			}

			// get list of all reservations
			reservationList := []ospdirectorv1beta1.IPReservation{}
			for _, roleReservations := range network.Spec.RoleReservations {
				reservationList = append(reservationList, roleReservations.Reservations...)
			}

			if reservationIP == "" {
				// No reservation found, so create a new one
				ip, reservation, err := common.AssignIP(common.AssignIPDetails{
					IPnet:           *cidr,
					RangeStart:      start,
					RangeEnd:        end,
					RoleReservelist: network.Spec.RoleReservations[instance.Spec.RoleName].Reservations,
					Reservelist:     reservationList,
					ExcludeRanges:   []string{},
					Hostname:        hostname,
					VIP:             instance.Spec.VIP,
					Deleted:         false,
				})
				if err != nil {
					cond.Message = fmt.Sprintf("Failed to do ip reservation: %s", hostname)
					cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.IPSetCondReasonIPReservation)
					cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.IPSetCondTypeError)
					err = common.WrapErrorForObject(cond.Message, instance, err)

					return ctrl.Result{}, err
				}

				reservationIP = ip.String()

				// record the reservation on the OpenStackNet
				network.Spec.RoleReservations[instance.Spec.RoleName] = ospdirectorv1beta1.OpenStackNetRoleStatus{
					AddToPredictableIPs: instance.Spec.AddToPredictableIPs,
					Reservations:        reservation,
				}

				common.LogForObject(
					r,
					fmt.Sprintf("Created new reservation for host %s in network %s with IP %s", hostname, netName, reservationIP),
					instance,
				)
			}

			// add netName -> reservationIP to hostIPs map
			hostNetworkIPs[netName] = reservationIP

			// update status of OSNet
			// TODO: just patch?
			err = r.Client.Update(context.TODO(), network)
			if err != nil {
				r.Log.Error(err, fmt.Sprintf("Failed to update OpenStackNet %s", network.Name))
				return ctrl.Result{}, err
			}
		}

		// set/update ipset host status
		instance.Status.HostIPs[hostname] = ospdirectorv1beta1.OpenStackIPHostsStatus{
			IPAddresses: hostNetworkIPs,
		}
	}

	cond.Message = fmt.Sprintf("Created new reservations for %s", newHosts)
	cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.IPSetCondReasonIPReservation)
	cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.IPSetCondTypeConfigured)

	return ctrl.Result{}, nil
}

func (r *OpenStackIPSetReconciler) cleanupOSNetStatus(instance *ospdirectorv1beta1.OpenStackIPSet) error {

	// mark entry in OSNet status as deleted
	for _, netName := range instance.Spec.Networks {
		// get network with name_lower label
		network, err := openstacknet.GetOpenStackNetWithLabel(
			r,
			instance.Namespace,
			map[string]string{
				openstacknet.SubNetNameLabelSelector: netName,
			},
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				r.Log.Info(fmt.Sprintf("OpenStackNet with NameLower %s not found!", netName))
				continue
			}
			// Error reading the object - requeue the request.
			return err
		}

		// delete all reservation entries from RoleName
		delete(network.Spec.RoleReservations, instance.Spec.RoleName)

		// update status of OSNet
		// TODO: just patch?
		err = r.Client.Update(context.TODO(), network)
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("Failed to update OpenStackNet spec: %v", err))
		}
	}

	return nil
}

//
// Verify if all required networks for the OSIPset exist
//
func (r *OpenStackIPSetReconciler) verifyNetworksExist(
	instance *ospdirectorv1beta1.OpenStackIPSet,
	cond *ospdirectorv1beta1.Condition,
) (ctrl.Result, error) {
	for _, net := range instance.Spec.Networks {
		osNet := &ospdirectorv1beta1.OpenStackNet{}
		osNetName := strings.ToLower(strings.Replace(net, "_", "", -1))

		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: osNetName, Namespace: instance.Namespace}, osNet)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				cond.Message = fmt.Sprintf("Underlying OSNet for network %s requested in IPSet is missing", net)
				cond.Reason = ospdirectorv1beta1.ConditionReason(ospdirectorv1beta1.IPSetCondReasonNetNotFound)
				cond.Type = ospdirectorv1beta1.ConditionType(ospdirectorv1beta1.IPSetCondTypeWaiting)
				err = common.WrapErrorForObject(cond.Message, instance, err)

				return ctrl.Result{RequeueAfter: 20 * time.Second}, err
			}
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}
