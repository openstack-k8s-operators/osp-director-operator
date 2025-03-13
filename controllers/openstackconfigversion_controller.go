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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
)

// OpenStackConfigVersionReconciler reconciles a OpenStackConfigVersion object
type OpenStackConfigVersionReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackconfigversions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackconfigversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=osp-director.openstack.org,resources=openstackconfigversions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackConfigVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *OpenStackConfigVersionReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("openstackconfigversion", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackConfigVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OpenStackConfigVersion{}).
		Complete(r)
}
