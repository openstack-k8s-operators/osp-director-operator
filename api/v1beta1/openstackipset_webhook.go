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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var openstackipsetlog = logf.Log.WithName("openstackipset-resource")

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *OpenStackIPSet) SetupWebhookWithManager(mgr ctrl.Manager) error {

	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstackipset,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackipsets,verbs=create;update,versions=v1beta1,name=mopenstackipset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &OpenStackIPSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackIPSet) Default() {
	openstackipsetlog.Info("default", "name", r.Name)

	//
	// set OpenStackNetConfig reference label if not already there
	// Note, any rename of the osnetcfg won't be reflected
	//
	if _, ok := r.GetLabels()[OpenStackNetConfigReconcileLabel]; !ok {
		labels, err := AddOSNetConfigRefLabel(
			r.Namespace,
			r.Spec.Networks[0],
			r.GetLabels(),
		)
		if err != nil {
			controlplanelog.Error(err, fmt.Sprintf("error adding OpenStackNetConfig reference label on %s - %s: %s", r.Kind, r.Name, err))
		}
		r.SetLabels(labels)
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-osp-director-openstack-org-v1beta1-openstackipset,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackipsets,verbs=create;update,versions=v1beta1,name=vopenstackipset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &OpenStackIPSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackIPSet) ValidateCreate() error {
	openstackipsetlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackIPSet) ValidateUpdate(old runtime.Object) error {
	openstackipsetlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackIPSet) ValidateDelete() error {
	openstackipsetlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
