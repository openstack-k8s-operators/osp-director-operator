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
	"bytes"
	"context"
	"fmt"
	"net"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
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

	if instance.Status.IPAddresses == nil {
		instance.Status.IPAddresses = make(map[string]string)
	}

	// iterate over the requested Networks
	for _, netName := range instance.Spec.Networks {

		if instance.Status.IPAddresses[netName] != "" {
			continue
		}

		network := &ospdirectorv1beta1.OvercloudNet{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: netName, Namespace: instance.Namespace}, network)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}

		_, cidr, _ := net.ParseCIDR(network.Spec.Cidr)
		start := net.ParseIP(network.Spec.AllocationStart)
		end := net.ParseIP(network.Spec.AllocationEnd)

		ip, reservation, err := common.AssignIP(*cidr, start, end, network.Status.Reservations, instance.Name)

		// record the reservation on the OvercloudNet
		network.Status.Reservations = reservation
		err = r.Client.Status().Update(context.TODO(), network)
		if err != nil {
			r.Log.Error(err, "Failed to update OvercloudNet status %v")
			return ctrl.Result{}, err
		}

		instance.Status.IPAddresses[netName] = ip.String()

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

	roleIPSets := map[string]map[string][]string{}

	for _, ipset := range overcloudIPList.Items {
		if roleIPSet, exists := roleIPSets[ipset.Spec.Role]; exists {
			for netName, addr := range ipset.Status.IPAddresses {
				//roleIPSet.IPList = append(roleIPSet.IPList, ipset.Status.IPAddresses[netName])
				roleIPSet[netName] = append(roleIPSet[netName], addr)
			}
		} else {
			// initialize the roleIPSet with new netIPLists
			for name, addr := range ipset.Status.IPAddresses {
				roleIPSets[instance.Spec.Role] = map[string][]string{name: {addr}}
			}

		}
	}

	r.Log.Info("RoleIPList:")
	r.Log.Info(fmt.Sprintf("%v", roleIPSets))

	// write it all to a configmap
	envVars := make(map[string]common.EnvSetter)
	cmLabels := common.GetLabels(instance.Name, "osp-overcloudipset")

	templateParameters := make(map[string]string)
	templateParameters["PredictableIps"] = roleIPSetToString(roleIPSets)

	cm := []common.Template{
		{
			Name:           "predictable-ips-heat-env",
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

// roleIPSetToString -
func roleIPSetToString(r map[string]map[string][]string) string {
	var b bytes.Buffer
	for role, roleIps := range r {
		b.WriteString(fmt.Sprintf("  %sIPs:\n", role))
		for netname, ips := range roleIps {
			b.WriteString(fmt.Sprintf("    %s:\n", netname))
			for _, ip := range ips {
				b.WriteString(fmt.Sprintf("    - %s\n", ip))
			}
		}
	}
	return b.String()
}
