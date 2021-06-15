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
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/diff"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstackipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/overcloudipset"
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

	if instance.Status.HostIPs == nil {
		instance.Status.HostIPs = make(map[string]ospdirectorv1beta1.OpenStackIPHostsStatus)
	}

	if instance.Status.Networks == nil {
		instance.Status.Networks = make(map[string]ospdirectorv1beta1.NetworkStatus)
	}

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
			r.Log.Info(fmt.Sprintf("Finalizer %s added to CR %s", common.FinalizerName, instance.Name))
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
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("CR %s deleted", instance.Name))
		return ctrl.Result{}, nil
	}

	// get a copy of the current CR status
	currentStatus := instance.Status.DeepCopy()

	//
	// Remove existing IPSet entry
	//
	// Are there any remove nodes ?
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

		r.removeIP(instance, removedHosts)
	}

	//
	// Add new IPSet entry
	//
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

		err := r.addIP(instance, newHostsSorted)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 10 * time.Second}, fmt.Errorf("IPSet failed adding IPs for hosts %v due to object not found. Reconcile in 10s", newHostsSorted)
			}
			return ctrl.Result{}, err
		}
	}

	// update the IPs for IPSet if status got updated
	actualStatus := instance.Status
	if !reflect.DeepEqual(currentStatus, &actualStatus) {
		r.Log.Info(fmt.Sprintf("Updating IPSet status CR: %s - %s", instance.Name, diff.ObjectReflectDiff(currentStatus, &actualStatus)))
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			r.Log.Error(err, "Failed to update OpenStackIPSet status %v")
			return ctrl.Result{}, err
		}
	}

	// sync deleted IPSet with OSNet
	err = r.syncDeletedIPs(instance)
	if err != nil {
		r.Log.Error(err, "Failed to sync deleted OpenStackIPSet with OpenStackNet status %v")
		return ctrl.Result{}, err
	}

	// generate pre assigned IPs environment file for Heat
	overcloudNetList := &ospdirectorv1beta1.OpenStackNetList{}
	overcloudNetListOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.Limit(1000),
	}
	err = r.Client.List(context.TODO(), overcloudNetList, overcloudNetListOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	overcloudIPList := &ospdirectorv1beta1.OpenStackIPSetList{}
	overcloudIPListOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.Limit(1000),
		client.MatchingLabels{openstackipset.AddToPredictableIPsLabel: "true"},
	}
	err = r.Client.List(context.TODO(), overcloudIPList, overcloudIPListOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	// write it all to a configmap
	envVars := make(map[string]common.EnvSetter)
	cmLabels := common.GetLabels(instance, openstackipset.AppLabel, map[string]string{})

	templateParameters, err := openstackipset.CreateConfigMapParams(*overcloudIPList, *overcloudNetList)
	if err != nil {
		return ctrl.Result{}, err
	}

	cm := []common.Template{
		{
			Name:           "tripleo-deploy-config",
			Namespace:      instance.Namespace,
			Type:           common.TemplateTypeConfig,
			InstanceType:   instance.Kind,
			AdditionalData: map[string]string{},
			Labels:         cmLabels,
			ConfigOptions:  templateParameters,
			SkipSetOwner:   true,
		},
	}

	err = common.EnsureConfigMaps(r, instance, cm, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *OpenStackIPSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackIPSet{}).
		Complete(r)
}

func (r *OpenStackIPSetReconciler) removeIP(instance *ospdirectorv1beta1.OpenStackIPSet, removedHosts []string) {

	// remove entry from OSIPSet status
	for _, hostname := range removedHosts {
		delete(instance.Status.HostIPs, hostname)
		r.Log.Info(fmt.Sprintf("OSP Hostname %s removed from OSIPSet %s, new status %v", hostname, instance.Name, instance.Status.HostIPs))
	}

}

func (r *OpenStackIPSetReconciler) syncDeletedIPs(instance *ospdirectorv1beta1.OpenStackIPSet) error {

	allActiveIPSetHosts := make([]string, len(instance.Status.HostIPs))
	for hostname := range instance.Status.HostIPs {
		allActiveIPSetHosts = append(allActiveIPSetHosts, hostname)
	}

	// mark entry in OSNet status as deleted
	for _, netName := range instance.Spec.Networks {
		network := &ospdirectorv1beta1.OpenStackNet{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: netName, Namespace: instance.Namespace}, network)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				r.Log.Info(fmt.Sprintf("OpenStackNet named %s not found!", netName))
				continue
			}
			// Error reading the object - requeue the request.
			return err
		}

		// mark the reservation for the hosts as deleted
		for index, reservation := range network.Status.RoleReservations[instance.Spec.RoleName].Reservations {

			// set OSNet deleted flag for non exiting nodes in the ipset
			reservationDeleted := !common.StringInSlice(reservation.Hostname, allActiveIPSetHosts)

			// mark OSNet host entry as deleted
			if network.Status.RoleReservations[instance.Spec.RoleName].Reservations[index].Deleted != reservationDeleted {
				network.Status.RoleReservations[instance.Spec.RoleName].Reservations[index].Deleted = reservationDeleted
				r.Log.Info(fmt.Sprintf("Update network status of host %s - net %s - %v", reservation.Hostname, network.Name, network.Status.RoleReservations[instance.Spec.RoleName].Reservations[index].Deleted))
			}
		}

		// update status of OSNet
		err = r.Client.Status().Update(context.TODO(), network)
		if err != nil {
			r.Log.Error(err, "Failed to update OpenStackNet status %v")
			return err
		}

	}

	return nil
}

func (r *OpenStackIPSetReconciler) addIP(instance *ospdirectorv1beta1.OpenStackIPSet, newHosts common.HostnameList) error {

	for _, k := range newHosts {
		//hostRef := k.HostRef
		hostname := k.Hostname

		hostNetworkIPs := map[string]string{}

		// iterate over the requested Networks
		for _, netName := range instance.Spec.Networks {

			network := &ospdirectorv1beta1.OpenStackNet{}
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: netName, Namespace: instance.Namespace}, network)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					r.Log.Info(fmt.Sprintf("OpenStackNet named %s not found!", netName))
				}
				// Error reading the object - requeue the request.
				return err
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
				return fmt.Errorf(fmt.Sprintf("Failed to parse CIDR %s: %v", network.Spec.Cidr, err))
			}

			start := net.ParseIP(network.Spec.AllocationStart)
			end := net.ParseIP(network.Spec.AllocationEnd)

			// If network RoleReservations status map is nil, create it
			if network.Status.RoleReservations == nil {
				network.Status.RoleReservations = map[string]ospdirectorv1beta1.OpenStackNetRoleStatus{}
			}
			// If network RoleReservations[instance.Spec.RoleName] status map is nil, create it
			if _, ok := network.Status.RoleReservations[instance.Spec.RoleName]; !ok {
				network.Status.RoleReservations[instance.Spec.RoleName] = ospdirectorv1beta1.OpenStackNetRoleStatus{}
			}

			reservationIP := ""

			// Do we already have a reservation for this hostname on the network?
			for _, reservation := range network.Status.RoleReservations[instance.Spec.RoleName].Reservations {
				if reservation.Hostname == hostname {
					// We also need the netmask (which is not stored in the OpenStackNet status),
					// so we acquire it from the OpenStackIPSet spec
					cidrPieces := strings.Split(network.Spec.Cidr, "/")
					reservationIP = fmt.Sprintf("%s/%s", reservation.IP, cidrPieces[len(cidrPieces)-1])
					r.Log.Info(fmt.Sprintf("Re-use existing reservation for host %s in network %s with IP %s", hostname, netName, reservationIP))
					break
				}
			}

			// get list of all reservations
			reservationList := []ospdirectorv1beta1.IPReservation{}
			for _, roleReservations := range network.Status.RoleReservations {
				reservationList = append(reservationList, roleReservations.Reservations...)
			}

			if reservationIP == "" {
				// No reservation found, so create a new one
				ip, reservation, err := common.AssignIP(common.AssignIPDetails{
					IPnet:           *cidr,
					RangeStart:      start,
					RangeEnd:        end,
					RoleReservelist: network.Status.RoleReservations[instance.Spec.RoleName].Reservations,
					Reservelist:     reservationList,
					ExcludeRanges:   []string{},
					Hostname:        hostname,
					VIP:             instance.Spec.VIP,
					Deleted:         false,
				})
				if err != nil {
					return fmt.Errorf(fmt.Sprintf("Failed to do ip reservation: %v", err))
				}

				reservationIP = ip.String()

				// record the reservation on the OpenStackNet
				network.Status.RoleReservations[instance.Spec.RoleName] = ospdirectorv1beta1.OpenStackNetRoleStatus{
					AddToPredictableIPs: instance.Spec.AddToPredictableIPs,
					Reservations:        reservation,
				}

				r.Log.Info(fmt.Sprintf("Created new reservation for host %s in network %s with IP %s", hostname, netName, reservationIP))

			}

			// add netName -> reservationIP to hostIPs map
			hostNetworkIPs[netName] = reservationIP

			err = r.Client.Status().Update(context.TODO(), network)
			if err != nil {
				return fmt.Errorf(fmt.Sprintf("Failed to update OpenStackNet status: %v", err))
			}

		}

		// set/update ipset host status
		instance.Status.HostIPs[hostname] = ospdirectorv1beta1.OpenStackIPHostsStatus{
			IPAddresses: hostNetworkIPs,
		}
	}

	return nil
}

func (r *OpenStackIPSetReconciler) cleanupOSNetStatus(instance *ospdirectorv1beta1.OpenStackIPSet) error {

	// mark entry in OSNet status as deleted
	for _, netName := range instance.Spec.Networks {
		network := &ospdirectorv1beta1.OpenStackNet{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: netName, Namespace: instance.Namespace}, network)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				r.Log.Info(fmt.Sprintf("OpenStackNet named %s not found!", netName))
				continue
			}
			// Error reading the object - requeue the request.
			return err
		}

		// delete all reservation entries from RoleName
		delete(network.Status.RoleReservations, instance.Spec.RoleName)

		// update status of OSNet
		err = r.Client.Status().Update(context.TODO(), network)
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("Failed to update OpenStackNet status: %v", err))
		}
	}

	return nil
}
