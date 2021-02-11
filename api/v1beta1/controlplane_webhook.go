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
var controlplanelog = logf.Log.WithName("controlplane-resource")

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *ControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-osp-director-openstack-org-v1beta1-controlplane,mutating=false,failurePolicy=fail,groups=osp-director.openstack.org,resources=controlplanes,versions=v1beta1,name=vcontrolplane.kb.io

var _ webhook.Validator = &ControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ControlPlane) ValidateCreate() error {
	controlplanelog.Info("validate create", "name", r.Name)

	return r.checkBaseImageReqs()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ControlPlane) ValidateUpdate(old runtime.Object) error {
	controlplanelog.Info("validate update", "name", r.Name)

	return r.checkBaseImageReqs()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ControlPlane) ValidateDelete() error {
	controlplanelog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *ControlPlane) checkBaseImageReqs() error {
	if r.Spec.Controller.BaseImageURL == "" && r.Spec.Controller.BaseImageVolumeName == "" {
		return fmt.Errorf("Either \"baseImageURL\" or \"baseImageVolumeName\" must be provided")
	}

	return nil
}
