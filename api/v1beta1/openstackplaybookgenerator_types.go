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
	// Name of the image used to generate playbooks. If missing will be set to the configured OPENSTACKCLIENT_IMAGE_URL_DEFAULT in the CSV for the OSP Director Operator.
	ImageURL string `json:"imageURL,omitempty"`
	// Required. the name of the config map containing Heat env file customizations
	HeatEnvConfigMap string `json:"heatEnvConfigMap"`
	// Optional. the name of the config map containing custom Heat template tarball which will be extracted prior to playbook generation
	TarballConfigMap string `json:"tarballConfigMap,omitempty"`
	// Advanced Heat Settings can be used to increase the Heat Engine replicas or customize container images used during playbook generation.
	EphemeralHeatSettings OpenStackEphemeralHeatSpec `json:"ephemeralHeatSettings,omitempty"`
	// +kubebuilder:default=false
	// Interactive enables the user to rsh into the playbook generator pod for interactive debugging with the ephemeral heat instance. If enabled manual execution of the script to generate playbooks will be required.
	Interactive bool `json:"interactive,omitempty"`
	// GitSecret the name of the secret used to configure the Git repository url and ssh private key credentials used to store generated Ansible playbooks. This secret should contain an entry for both 'git_url' and 'git_ssh_identity'.
	GitSecret string `json:"gitSecret"`
}

// OpenStackPlaybookGeneratorStatus defines the observed state of OpenStackPlaybookGenerator
type OpenStackPlaybookGeneratorStatus struct {
	// PlaybookHash hash
	PlaybookHash string `json:"playbookHash"`

	// CurrentState
	CurrentState PlaybookGeneratorState `json:"currentState"`

	// Conditions
	Conditions ConditionList `json:"conditions,omitempty" optional:"true"`
}

// PlaybookGeneratorState - the state of the execution of this playbook generator
type PlaybookGeneratorState string

const (
	// PlaybookGeneratorWaiting - the playbook generator is blocked by prerequisite objects
	PlaybookGeneratorWaiting PlaybookGeneratorState = "Waiting"
	// PlaybookGeneratorInitializing - the playbook generator is preparing to execute
	PlaybookGeneratorInitializing PlaybookGeneratorState = "Initializing"
	// PlaybookGeneratorGenerating - the playbook generator is executing
	PlaybookGeneratorGenerating PlaybookGeneratorState = "Generating"
	// PlaybookGeneratorFinished - the playbook generation has finished executing
	PlaybookGeneratorFinished PlaybookGeneratorState = "Finished"
	// PlaybookGeneratorError - the playbook generation hit a generic error
	PlaybookGeneratorError PlaybookGeneratorState = "Error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osplaybookgenerator;osplaybookgenerators
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Playbook Generator"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.currentState`,description="Status"

// OpenStackPlaybookGenerator Used to configure Heat environments and template customizations to generate Ansible playbooks for OpenStack Overcloud deployment
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
