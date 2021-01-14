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

// OvercloudIPSetReconciler reconciles a OvercloudIPSet object
type OvercloudIPSetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OvercloudIPSetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OvercloudIPSetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OvercloudIPSetReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OvercloudIPSetReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudnets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudnets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudipsets/status,verbs=get;update;patch

// Reconcile - reconcile OvercloudIpSet objects
func (r *OvercloudIPSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("overcloudipset", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OvercloudIPSet{}
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
		instance.Status.HostIPs = make(map[string]ospdirectorv1beta1.OvercloudIPHostsStatus)
	}

	ctlplaneCidr := ""
	instance.Status.Networks = make(map[string]ospdirectorv1beta1.OvercloudNetSpec)

	// iterate over the requested hostCount
	for count := 0; count < instance.Spec.HostCount; count++ {
		hostname := fmt.Sprintf("%s-%d", strings.ToLower(instance.Spec.Role), count)

		// iterate over the requested Networks
		for _, netName := range instance.Spec.Networks {

			network := &ospdirectorv1beta1.OvercloudNet{}
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: netName, Namespace: instance.Namespace}, network)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					r.Log.Info(fmt.Sprintf("OvercloudNet named %s not found!", netName))
				}
				// Error reading the object - requeue the request.
				return ctrl.Result{}, err
			}

			instance.Status.Networks[network.Name] = network.Spec

			_, cidr, _ := net.ParseCIDR(network.Spec.Cidr)
			if network.Name == "ctlplane" {
				ctlplaneCidr = network.Spec.Cidr
			}

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
					// We also need the netmask (which is not stored in the OvercloudNet status),
					// so we acquire it from the OvercloudIpSet spec
					cidrPieces := strings.Split(network.Spec.Cidr, "/")
					reservationIP = fmt.Sprintf("%s/%s", reservation.IP, cidrPieces[len(cidrPieces)-1])
					break
				}
			}

			if reservationIP == "" {
				// No reservation found, so create a new one
				ip, reservation, err := common.AssignIP(*cidr, start, end, network.Status.Reservations, hostname)
				if err != nil {
					r.Log.Error(err, "Failed to do ip reservation %v")
					return ctrl.Result{}, err
				}

				reservationIP = ip.String()

				// record the reservation on the OvercloudNet
				network.Status.Reservations = reservation
				err = r.Client.Status().Update(context.TODO(), network)
				if err != nil {
					r.Log.Error(err, "Failed to update OvercloudNet status %v")
					return ctrl.Result{}, err
				}
			}

			if instance.Status.HostIPs[hostname].IPAddresses == nil {
				instance.Status.HostIPs[hostname] = ospdirectorv1beta1.OvercloudIPHostsStatus{IPAddresses: map[string]string{netName: reservationIP}}
			} else {
				instance.Status.HostIPs[hostname].IPAddresses[netName] = reservationIP
			}

		}
	}

	// update the IPs for each IPSet
	err = r.Client.Status().Update(context.TODO(), instance)
	if err != nil {
		r.Log.Error(err, "Failed to update OvercloudIpSet status %v")
		return ctrl.Result{}, err
	}

	// generate pre assigned IPs environment file for Heat
	overcloudIPList := &ospdirectorv1beta1.OvercloudIPSetList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.Limit(1000),
	}
	err = r.Client.List(context.TODO(), overcloudIPList, listOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	// write it all to a configmap
	envVars := make(map[string]common.EnvSetter)
	cmLabels := common.GetLabels(instance.Name, "osp-overcloudipset")

	templateParameters := overcloudipset.CreateConfigMapParams(*overcloudIPList, ctlplaneCidr)

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
func (r *OvercloudIPSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OvercloudIPSet{}).
		Complete(r)
}
