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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackPlaybookGeneratorSpec defines the desired state of OpenStackPlaybookGenerator
type OpenStackPlaybookGeneratorSpec struct {
	// Name of the image used to generate playbooks
	ImageURL string `json:"imageURL"`
	// Name of the associated OpenStackClient resource (FIXME: remove when we switch to git repos for syncing generated playbooks)
	OpenStackClientName string `json:"openstackClientName"`
	// Required. config map containing Heat env file customizations
	HeatEnvConfigMap string `json:"heatEnvConfigMap"`
	// Optional. name of any custom ROLESFILE in the configmap used to generate the roles map. If not specified the default t-h-t roles will be used.
	RolesFile string `json:"rolesFile,omitempty"`
	// Optional. config map containing custom Heat template tarball which will be extracted prior to playbook generation
	TarballConfigMap string `json:"tarballConfigMap,omitempty"`
	// Heat Settings
	EphemeralHeatSettings OpenStackEphemeralHeatSpec `json:"ephemeralHeatSettings,omitempty"`
}

// OpenStackPlaybookGeneratorStatus defines the observed state of OpenStackPlaybookGenerator
type OpenStackPlaybookGeneratorStatus struct {
	// PlaybookHash hash
	PlaybookHash string `json:"playbookHash"`

	// CurrentState
	CurrentState string `json:"currentState"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osplaybookgenerator;osplaybookgenerators
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Playbook Generator"
// +kubebuilder:printcolumn:name="CurrentState",type=string,JSONPath=`.status.currentState`

// OpenStackPlaybookGenerator is the Schema for the openstackplaybookgenerators API
type OpenStackPlaybookGenerator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackPlaybookGeneratorSpec   `json:"spec,omitempty"`
	Status OpenStackPlaybookGeneratorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackPlaybookGeneratorList contains a list of OpenStackPlaybookGenerator
type OpenStackPlaybookGeneratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackPlaybookGenerator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackPlaybookGenerator{}, &OpenStackPlaybookGeneratorList{})
}
