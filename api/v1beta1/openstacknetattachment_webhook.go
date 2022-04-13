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

	nmstateshared "github.com/nmstate/kubernetes-nmstate/api/shared"
	nmstate "github.com/openstack-k8s-operators/osp-director-operator/pkg/nmstate"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var openstacknetattachmentlog = logf.Log.WithName("openstacknetattachment-resource")

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *OpenStackNetAttachment) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstacknetattachment,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstacknetattachments,verbs=create;update,versions=v1beta1,name=mopenstacknetattachment.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackNetAttachment{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackNetAttachment) Default() {
	openstacknetattachmentlog.Info("default", "name", r.Name)

	// To avoid validation errors, we need to set the DesiredState to {} if it is nil.
	// This is the error that occurs otherwise...
	//
	// spec.attachConfiguration.nodeNetworkConfigurationPolicy.desiredState:
	// Invalid value: "null": spec.attachConfiguration.nodeNetworkConfigurationPolicy.desiredState in body must be of type object: "null"
	//
	if r.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy.DesiredState.Raw == nil {
		r.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy.DesiredState = nmstateshared.NewState("{}")
	}

}

//+kubebuilder:webhook:path=/validate-osp-director-openstack-org-v1beta1-openstacknetattachment,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstacknetattachments,verbs=create;update,versions=v1beta1,name=vopenstacknetattachment.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackNetAttachment{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetAttachment) ValidateCreate() error {
	openstacknetattachmentlog.Info("validate create", "name", r.Name)

	return checkBackupOperationBlocksAction(r.Namespace, shared.APIActionCreate)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetAttachment) ValidateUpdate(old runtime.Object) error {
	openstacknetattachmentlog.Info("validate update", "name", r.Name)

	return r.checkBridgeName(old)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetAttachment) ValidateDelete() error {
	openstacknetattachmentlog.Info("validate delete", "name", r.Name)

	return checkBackupOperationBlocksAction(r.Namespace, shared.APIActionDelete)
}

func (r *OpenStackNetAttachment) checkBridgeName(old runtime.Object) error {

	// Get the current (potentially new) bridge name, if any
	curBridge, err := nmstate.GetDesiredStateBridgeName(r.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy.DesiredState.Raw)

	if err != nil {
		return err
	}

	// Get the old bridge name, if any
	var ok bool
	var oldInstance *OpenStackNetAttachment

	if oldInstance, ok = old.(*OpenStackNetAttachment); !ok {
		return fmt.Errorf("runtime object is not an OpenStackNetAttachment")
	}

	oldBridge, err := nmstate.GetDesiredStateBridgeName(oldInstance.Spec.AttachConfiguration.NodeNetworkConfigurationPolicy.DesiredState.Raw)

	if err != nil {
		return err
	}

	if curBridge != oldBridge {
		return fmt.Errorf("bridge names may not be changed")
	}

	return nil
}
