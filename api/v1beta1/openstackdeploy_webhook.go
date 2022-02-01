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
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var openstackdeploylog = logf.Log.WithName("openstackdeploy-resource")

// OpenStackDeployDefaults -
type OpenStackDeployDefaults struct {
	AgentImageURL string
}

var openstackDeployDefaults OpenStackDeployDefaults

// SetupWebhookWithManager -
func (r *OpenStackDeploy) SetupWebhookWithManager(mgr ctrl.Manager, defaults OpenStackDeployDefaults) error {
	openstackDeployDefaults = defaults

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstackdeploy,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackdeploys,verbs=create;update,versions=v1beta1,name=mopenstackdeploy.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &OpenStackDeploy{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackDeploy) Default() {
	openstackdeploylog.Info("default", "name", r.Name)
	if r.Spec.ImageURL == "" {
		r.Spec.ImageURL = openstackDeployDefaults.AgentImageURL
	}
}
