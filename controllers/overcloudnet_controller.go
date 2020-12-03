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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
)

// OvercloudNetReconciler reconciles a OvercloudNet object
type OvercloudNetReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudnets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=osp-director.openstack.org,resources=overcloudnets/status,verbs=get;update;patch

func (r *OvercloudNetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("overcloudnet", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *OvercloudNetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ospdirectorv1beta1.OvercloudNet{}).
		Complete(r)
}
