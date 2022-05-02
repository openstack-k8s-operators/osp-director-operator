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

// OpenStackConfigGeneratorSpec defines the desired state of OpenStackConfigGenerator
type OpenStackConfigGeneratorSpec struct {
	// +kubebuilder:validation:Optional
	// Name of the image used to generate configs. If missing will be set to the configured OPENSTACKCLIENT_IMAGE_URL_DEFAULT in the CSV for the OSP Director Operator.
	ImageURL string `json:"imageURL,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	// Optional. List of heat environment files to include from tripleo-heat-templates/environments
	HeatEnvs []string `json:"heatEnvs,omitempty"`
	// Required. the name of the config map containing Heat env file customizations
	HeatEnvConfigMap string `json:"heatEnvConfigMap"`
	// Optional. the name of the config map containing custom Heat template tarball which will be extracted prior to config generation
	TarballConfigMap string `json:"tarballConfigMap,omitempty"`
	// Advanced Heat Settings can be used to increase the Heat Engine replicas or customize container images used during config generation.
	EphemeralHeatSettings OpenStackEphemeralHeatSpec `json:"ephemeralHeatSettings,omitempty"`
	// +kubebuilder:default=false
	// Interactive enables the user to rsh into the config generator pod for interactive debugging with the ephemeral heat instance. If enabled manual execution of the script to generate playbooks will be required.
	Interactive bool `json:"interactive,omitempty"`
	// GitSecret the name of the secret used to configure the Git repository url and ssh private key credentials used to store generated Ansible playbooks. This secret should contain an entry for both 'git_url' and 'git_ssh_identity'.
	GitSecret string `json:"gitSecret"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	// Optional. List of Roles used to limit which roles have network information injected during playbook generation. By default the list is empty and all Roles are included for Baremetal/Vmsets within the project.
	Roles []string `json:"roles,omitempty"`
}

// OpenStackConfigGeneratorStatus defines the observed state of OpenStackConfigGenerator
type OpenStackConfigGeneratorStatus struct {
	// ConfigHash hash
	ConfigHash string `json:"configHash"`

	// CurrentState
	CurrentState shared.ConditionType `json:"currentState"`

	// CurrentReason
	CurrentReason shared.ConditionReason `json:"currentReason"`

	// Conditions
	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`
}

// ConfigGeneratorState - the state of the execution of this config generator
type ConfigGeneratorState string

// ConfigGeneratorReason - the reason of the condition for this config generator
type ConfigGeneratorReason string

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackConfigGenerator) IsReady() bool {
	cond := instance.Status.Conditions.InitCondition()

	return cond.Type == shared.ConfigGeneratorCondTypeFinished && cond.Reason == shared.ConfigGeneratorCondReasonJobFinished
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osconfiggenerator;osconfiggenerators
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Config Generator"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.currentState`,description="Status"

// OpenStackConfigGenerator Used to configure Heat environments and template customizations to generate Ansible playbooks for OpenStack Overcloud deployment
type OpenStackConfigGenerator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackConfigGeneratorSpec   `json:"spec,omitempty"`
	Status OpenStackConfigGeneratorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackConfigGeneratorList contains a list of OpenStackConfigGenerator
type OpenStackConfigGeneratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackConfigGenerator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackConfigGenerator{}, &OpenStackConfigGeneratorList{})
}
