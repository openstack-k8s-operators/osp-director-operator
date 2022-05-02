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
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackDeployAdvancedSettingsSpec defines advanced deployment settings
type OpenStackDeployAdvancedSettingsSpec struct {
	// +kubebuilder:validation:Optional
	// Playbook to run from config-download
	Playbook string `json:"playbook,omitempty"`
	// +kubebuilder:validation:Optional
	// Ansible inventory limit
	Limit string `json:"limit,omitempty"`
	// +kubebuilder:validation:Optional
	// Ansible include tags
	Tags []string `json:"tags,omitempty"`
	// +kubebuilder:validation:Optional
	// Ansible exclude tags
	SkipTags []string `json:"skipTags,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Skip setting a unique deploy identifier
	SkipDeployIdentifier bool `json:"skipDeployIdentifier"`
}

// OpenStackDeploySpec defines the desired state of OpenStackDeploy
type OpenStackDeploySpec struct {
	// +kubebuilder:validation:Optional
	// Name of the image
	ImageURL string `json:"imageURL,omitempty"`

	// ConfigVersion the config version/git hash of the playbooks to deploy.
	ConfigVersion string `json:"configVersion,omitempty"`

	// ConfigGenerator name of the configGenerator
	ConfigGenerator string `json:"configGenerator"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=deploy
	// +kubebuilder:validation:Enum={"deploy","update","externalUpdate"}
	// Deployment mode
	Mode string `json:"mode,omitempty"`

	// +kubebuilder:validation:Optional
	// Advanced deployment settings
	AdvancedSettings OpenStackDeployAdvancedSettingsSpec `json:"advancedSettings,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Skip NNCP validation to proceed deployment even if one NNCP status returns not all worker nodes are configured
	SkipNNCPValidation bool `json:"skipNNCPValidation"`
}

// OpenStackDeployStatus defines the observed state of OpenStackDeploy
type OpenStackDeployStatus struct {
	// ConfigVersion hash that has been deployed
	ConfigVersion string `json:"configVersion"`

	// CurrentState
	CurrentState shared.ProvisioningState `json:"currentState"`

	// CurrentReason
	CurrentReason shared.ConditionReason `json:"currentReason"`

	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=osdeploy;osdeploys;osdepl
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Deploy"
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.currentState`,description="Status"

// OpenStackDeploy executes a set of Ansible playbooks for the supplied OSConfigVersion
type OpenStackDeploy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackDeploySpec   `json:"spec,omitempty"`
	Status OpenStackDeployStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackDeployList contains a list of OpenStackDeploy
type OpenStackDeployList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackDeploy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackDeploy{}, &OpenStackDeployList{})
}
