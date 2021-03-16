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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/tidwall/gjson"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	vmset "github.com/openstack-k8s-operators/osp-director-operator/pkg/vmset"
)

// OpenStackNetReconciler reconciles a OpenStackNet object
type OpenStackNetReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OpenStackNetReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackNetReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackNetReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackNetReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstacknets/finalizers,verbs=update
// +kubebuilder:rbac:groups=osp-director.openstack.org,namespace=openstack,resources=deployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,namespace=openstack,resources=network-attachment-definitions,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=nmstate.io,resources=nodenetworkconfigurationpolicies,verbs=create;delete;get;list;patch;update;watch

// Reconcile -
func (r *OpenStackNetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("overcloudnet", req.NamespacedName)

	// Fetch the controller VM instance
	instance := &ospdirectorv1beta1.OpenStackNet{}
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

	// generate NodeNetworkConfigurationPolicy
	ncp := common.NetworkConfigurationPolicy{
		Name:                           instance.Name,
		Labels:                         common.GetLabelSelector(instance, openstacknet.AppLabel),
		NodeNetworkConfigurationPolicy: instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy,
	}
	err = common.CreateOrUpdateNetworkConfigurationPolicy(r, instance, instance.Kind, &ncp)
	if err != nil {
		return ctrl.Result{}, err
	}

	// create NetworkAttachmentDefinition
	// the desired state is raw json, marshal -> unmarshal to parse it
	desiredStateByte, err := json.Marshal(instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Error marshal NodeNetworkConfigurationPolicy desired state: %v", instance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy))
		return ctrl.Result{}, err
	}

	var desiredState map[string]json.RawMessage //interface{}
	err = json.Unmarshal(desiredStateByte, &desiredState)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Error unmarshal NodeNetworkConfigurationPolicy desired state: %v", string(desiredStateByte)))
		return ctrl.Result{}, err
	}

	bridgeName := gjson.Get(string(desiredState["desiredState"]), "interfaces.#.name").Array()[0]
	vlan := strconv.Itoa(instance.Spec.Vlan)
	nad := common.NetworkAttachmentDefinition{
		Name:      instance.Name,
		Namespace: instance.Namespace,
		Labels:    common.GetLabelSelector(instance, vmset.AppLabel),
		Data: map[string]string{
			"NetName":    instance.Name,
			"BridgeName": bridgeName.String(),
			"Vlan":       vlan,
		},
	}

	// create nad
	err = common.CreateOrUpdateNetworkAttachmentDefinition(r, instance, instance.Kind, metav1.NewControllerRef(instance, instance.GroupVersionKind()), &nad)
	if err != nil {
		return ctrl.Result{}, err
	}

	// create static nad used for openstackclient
	nad.Name = fmt.Sprintf("%s-static", instance.Name)
	nad.Data["Static"] = "true"

	err = common.CreateOrUpdateNetworkAttachmentDefinition(r, instance, instance.Kind, metav1.NewControllerRef(instance, instance.GroupVersionKind()), &nad)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager -
func (r *OpenStackNetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackNet{}).
		Owns(&networkv1.NetworkAttachmentDefinition{}).
		Complete(r)
}
