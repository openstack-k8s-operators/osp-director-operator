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

package v1beta1

import (

	//client "github.com/openstack-k8s-operators/osp-director-operator/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var baremetalsetlog = logf.Log.WithName("baremetalset-resource")

// Dynamic client
//var dClient dynamic.Interface
var client goClient.Client

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *BaremetalSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	// var err error

	// Get a dynamic k8s client
	// dClient, err = client.GetInClusterDynamicClient()

	// if err != nil {
	// 	return err
	// }

	client = mgr.GetClient()

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-osp-director-openstack-org-v1beta1-baremetalset,mutating=false,failurePolicy=fail,groups=osp-director.openstack.org,resources=baremetalsets,versions=v1beta1,name=vbaremetalset.kb.io

var _ webhook.Validator = &BaremetalSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *BaremetalSet) ValidateCreate() error {
	baremetalsetlog.Info("validate create", "name", r.Name)

	return CheckRoleNameExists(client, r.TypeMeta, r.ObjectMeta, r.Spec.Role)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *BaremetalSet) ValidateUpdate(old runtime.Object) error {
	baremetalsetlog.Info("validate update", "name", r.Name)

	return CheckRoleNameExists(client, r.TypeMeta, r.ObjectMeta, r.Spec.Role)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *BaremetalSet) ValidateDelete() error {
	baremetalsetlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
