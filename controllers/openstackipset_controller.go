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
	"strings"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	overcloudipset "github.com/openstack-k8s-operators/osp-director-operator/pkg/overcloudipset"
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
func (r *OpenStackIPSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("openstackipset", req.NamespacedName)

	// Fetch the controller VM instance
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
		//instance.Status.IPAddresses = make(map[string]string)
		instance.Status.HostIPs = make(map[string]ospdirectorv1beta1.OpenStackIPHostsStatus)
	}

	instance.Status.Networks = make(map[string]ospdirectorv1beta1.OpenStackNetSpec)

	// iterate over the requested hostCount
	for count := 0; count < instance.Spec.HostCount; count++ {
		hostname := fmt.Sprintf("%s-%d", strings.ToLower(instance.Spec.Role), count)

		// iterate over the requested Networks
		for _, netName := range instance.Spec.Networks {

			network := &ospdirectorv1beta1.OpenStackNet{}
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: netName, Namespace: instance.Namespace}, network)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					r.Log.Info(fmt.Sprintf("OpenStackNet named %s not found!", netName))
				}
				// Error reading the object - requeue the request.
				return ctrl.Result{}, err
			}

			instance.Status.Networks[network.Name] = network.Spec

			_, cidr, err := net.ParseCIDR(network.Spec.Cidr)
			if err != nil {
				r.Log.Error(err, fmt.Sprintf("Failed to parse CIDR %s", network.Spec.Cidr))
				return ctrl.Result{}, err
			}

			r.Log.Info(fmt.Sprintf("Network %s, cidr %v", network.Name, cidr))
			// TODO: should we really skip here, or modify AssignIP to just return the ip of an already existing entry?
			if instance.Status.HostIPs[hostname].IPAddresses[netName] != "" {
				continue
			}

			start := net.ParseIP(network.Spec.AllocationStart)
			end := net.ParseIP(network.Spec.AllocationEnd)

			reservationIP := ""

			// Do we already have a reservation for this hostname on the network?
			for _, reservation := range network.Status.Reservations {
				if reservation.Hostname == hostname {
					// We also need the netmask (which is not stored in the OpenStackNet status),
					// so we acquire it from the OpenStackIPSet spec
					cidrPieces := strings.Split(network.Spec.Cidr, "/")
					reservationIP = fmt.Sprintf("%s/%s", reservation.IP, cidrPieces[len(cidrPieces)-1])
					break
				}
			}

			if reservationIP == "" {
				// No reservation found, so create a new one
				// TODO: we generate the hostname/host IDKey in different controllers, which needs to be changed
				//       when using Predictable IPs. The IDKey should remain when a node gets scaled down/deleted.
				//       Its entry should not be removed from the IP lists and instead needs to be marked deleted.
				//       For now set the role + count which will break things when removing a node in the middle.
				ip, reservation, err := common.AssignIP(common.AssignIPDetails{
					IPnet:               *cidr,
					RangeStart:          start,
					RangeEnd:            end,
					Reservelist:         network.Status.Reservations,
					ExcludeRanges:       []string{},
					IDKey:               fmt.Sprintf("%s-%d", strings.ToLower(instance.Spec.Role), count),
					Hostname:            hostname,
					VIP:                 instance.Spec.VIP,
					Role:                instance.Spec.Role,
					AddToPredictableIPs: instance.Spec.AddToPredictableIPs,
				})
				if err != nil {
					r.Log.Error(err, "Failed to do ip reservation %v")
					return ctrl.Result{}, err
				}

				reservationIP = ip.String()

				// record the reservation on the OpenStackNet
				network.Status.Reservations = reservation
				err = r.Client.Status().Update(context.TODO(), network)
				if err != nil {
					r.Log.Error(err, "Failed to update OpenStackNet status %v")
					return ctrl.Result{}, err
				}
			}

			if instance.Status.HostIPs[hostname].IPAddresses == nil {
				instance.Status.HostIPs[hostname] = ospdirectorv1beta1.OpenStackIPHostsStatus{IPAddresses: map[string]string{netName: reservationIP}}
			} else {
				instance.Status.HostIPs[hostname].IPAddresses[netName] = reservationIP
			}

		}
	}

	// update the IPs for each IPSet
	err = r.Client.Status().Update(context.TODO(), instance)
	if err != nil {
		r.Log.Error(err, "Failed to update OpenStackIPSet status %v")
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
		client.MatchingLabels{overcloudipset.AddToPredictableIPsLabel: "true"},
	}
	err = r.Client.List(context.TODO(), overcloudIPList, overcloudIPListOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	// write it all to a configmap
	envVars := make(map[string]common.EnvSetter)
	cmLabels := common.GetLabels(instance.Name, overcloudipset.AppLabel)

	//templateParameters, err := overcloudipset.CreateConfigMapParams(r, *overcloudIPList, ctlplaneCidr)
	templateParameters, err := overcloudipset.CreateConfigMapParams(*overcloudIPList, *overcloudNetList)
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
		//		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
