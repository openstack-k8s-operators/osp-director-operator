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

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// OpenStackClientDefaults -
type OpenStackClientDefaults struct {
	ImageURL string
}

var openstackClientDefaults OpenStackClientDefaults

// log is for logging in this package.
var openstackclientlog = logf.Log.WithName("openstackclient-resource")

// SetupWebhookWithManager -
func (r *OpenStackClient) SetupWebhookWithManager(mgr ctrl.Manager, defaults OpenStackClientDefaults) error {

	openstackClientDefaults = defaults

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstackclient,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackclients,verbs=create;update,versions=v1beta1,name=mopenstackclient.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &OpenStackClient{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackClient) Default() {
	openstackclientlog.Info("default", "name", r.Name)

	if r.Spec.ImageURL == "" {
		r.Spec.ImageURL = openstackClientDefaults.ImageURL
	}

	//
	// set OpenStackNetConfig reference label if not already there
	// Note, any rename of the osnetcfg won't be reflected
	//
	if _, ok := r.GetLabels()[OpenStackNetConfigReconcileLabel]; !ok {
		labels, err := AddOSNetConfigRefLabel(
			r.Namespace,
			r.Spec.Networks[0],
			r.DeepCopy().GetLabels(),
		)
		if err != nil {
			openstackclientlog.Error(err, fmt.Sprintf("error adding OpenStackNetConfig reference label on %s - %s: %s", r.Kind, r.Name, err))
		}

		r.SetLabels(labels)
		openstackclientlog.Info(fmt.Sprintf("%s %s labels set to %v", r.GetObjectKind().GroupVersionKind().Kind, r.Name, r.GetLabels()))
	}

	//
	// add labels of all networks used by this CR
	//
	labels := AddOSNetNameLowerLabels(
		openstackclientlog,
		r.DeepCopy().GetLabels(),
		r.Spec.Networks,
	)
	if !equality.Semantic.DeepEqual(
		labels,
		r.GetLabels(),
	) {
		r.SetLabels(labels)
		openstackclientlog.Info(fmt.Sprintf("%s %s labels set to %v", r.GetObjectKind().GroupVersionKind().Kind, r.Name, r.GetLabels()))
	}

}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-osp-director-openstack-org-v1beta1-openstackclient,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackclients,versions=v1beta1,name=vopenstackclient.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &OpenStackIPSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackClient) ValidateCreate() error {
	openstackipsetlog.Info("validate create", "name", r.Name)

	//
	// validate that for all configured subnets an osnet exists
	//
	if err := validateNetworks(r.GetNamespace(), r.Spec.Networks); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackClient) ValidateUpdate(old runtime.Object) error {
	openstackipsetlog.Info("validate update", "name", r.Name)

	//
	// validate that for all configured subnets an osnet exists
	//
	if err := validateNetworks(r.GetNamespace(), r.Spec.Networks); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackClient) ValidateDelete() error {
	openstackipsetlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
