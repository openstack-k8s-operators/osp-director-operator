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

// OpenStackConfigGeneratorSpec defines the desired state of OpenStackConfigGenerator
type OpenStackConfigGeneratorSpec struct {
	// Name of the image used to generate configs. If missing will be set to the configured OPENSTACKCLIENT_IMAGE_URL_DEFAULT in the CSV for the OSP Director Operator.
	ImageURL string `json:"imageURL,omitempty"`
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
}

// OpenStackConfigGeneratorStatus defines the observed state of OpenStackConfigGenerator
type OpenStackConfigGeneratorStatus struct {
	// ConfigHash hash
	ConfigHash string `json:"configHash"`

	// CurrentState
	CurrentState ConfigGeneratorState `json:"currentState"`

	// CurrentReason
	CurrentReason ConfigGeneratorReason `json:"currentReason"`

	// Conditions
	Conditions ConditionList `json:"conditions,omitempty" optional:"true"`
}

// ConfigGeneratorState - the state of the execution of this config generator
type ConfigGeneratorState string

// ConfigGeneratorReason - the reason of the condition for this config generator
type ConfigGeneratorReason string

const (
	//
	// condition types
	//

	// ConfigGeneratorWaiting - the config generator is blocked by prerequisite objects
	ConfigGeneratorWaiting ConfigGeneratorState = "Waiting"
	// ConfigGeneratorInitializing - the config generator is preparing to execute
	ConfigGeneratorInitializing ConfigGeneratorState = "Initializing"
	// ConfigGeneratorGenerating - the config generator is executing
	ConfigGeneratorGenerating ConfigGeneratorState = "Generating"
	// ConfigGeneratorFinished - the config generation has finished executing
	ConfigGeneratorFinished ConfigGeneratorState = "Finished"
	// ConfigGeneratorError - the config generation hit a generic error
	ConfigGeneratorError ConfigGeneratorState = "Error"

	//
	// condition reasones
	//

	// ConfigGeneratorCondReasonCMUpdated - configmap updated
	ConfigGeneratorCondReasonCMUpdated ConfigGeneratorReason = "ConfigMapUpdated"
	// ConfigGeneratorCondReasonCMNotFound - configmap not found
	ConfigGeneratorCondReasonCMNotFound ConfigGeneratorReason = "ConfigMapNotFound"
	// ConfigGeneratorCondReasonCMCreateError - error creating/update CM
	ConfigGeneratorCondReasonCMCreateError ConfigGeneratorReason = "ConfigMapCreateError"
	// ConfigGeneratorCondReasonCMHashError - error creating/update CM
	ConfigGeneratorCondReasonCMHashError ConfigGeneratorReason = "ConfigMapHashError"
	// ConfigGeneratorCondReasonCMHashChanged - cm hash changed
	ConfigGeneratorCondReasonCMHashChanged ConfigGeneratorReason = "ConfigMapHashChanged"
	// ConfigGeneratorCondReasonCustomRolesNotFound - custom roles file not found
	ConfigGeneratorCondReasonCustomRolesNotFound ConfigGeneratorReason = "CustomRolesNotFound"
	// ConfigGeneratorCondReasonVMInstanceList - VM instance list error
	ConfigGeneratorCondReasonVMInstanceList ConfigGeneratorReason = "VMInstanceListError"
	// ConfigGeneratorCondReasonFencingTemplateError - Error creating fencing config parameters
	ConfigGeneratorCondReasonFencingTemplateError ConfigGeneratorReason = "FencingTemplateError"
	// ConfigGeneratorCondReasonEphemeralHeatUpdated - Ephemeral heat created/updated
	ConfigGeneratorCondReasonEphemeralHeatUpdated ConfigGeneratorReason = "EphemeralHeatUpdated"
	// ConfigGeneratorCondReasonEphemeralHeatLaunch - Ephemeral heat to launch
	ConfigGeneratorCondReasonEphemeralHeatLaunch ConfigGeneratorReason = "EphemeralHeatLaunch"
	// ConfigGeneratorCondReasonEphemeralHeatDelete - Ephemeral heat delete
	ConfigGeneratorCondReasonEphemeralHeatDelete ConfigGeneratorReason = "EphemeralHeatDelete"
	// ConfigGeneratorCondReasonJobCreated - created job
	ConfigGeneratorCondReasonJobCreated ConfigGeneratorReason = "JobCreated"
	// ConfigGeneratorCondReasonJobDelete - delete job
	ConfigGeneratorCondReasonJobDelete ConfigGeneratorReason = "JobDelete"
	// ConfigGeneratorCondReasonJobFailed - failed job
	ConfigGeneratorCondReasonJobFailed ConfigGeneratorReason = "JobFailed"
	// ConfigGeneratorCondReasonJobFinished - job finished
	ConfigGeneratorCondReasonJobFinished ConfigGeneratorReason = "JobFinished"
	// ConfigGeneratorCondReasonConfigCreate - config create in progress
	ConfigGeneratorCondReasonConfigCreate ConfigGeneratorReason = "ConfigCreate"
	// ConfigGeneratorCondReasonWaitingNodesProvisioned - waiting on nodes be provisioned
	ConfigGeneratorCondReasonWaitingNodesProvisioned ConfigGeneratorReason = "WaitingNodesProvisioned"
	// ConfigGeneratorCondReasonInputLabelError - error adding/update ConfigGeneratorInputLabel
	ConfigGeneratorCondReasonInputLabelError ConfigGeneratorReason = "ConfigGeneratorInputLabelError"
	// ConfigGeneratorCondReasonRenderEnvFilesError - error rendering environmane file
	ConfigGeneratorCondReasonRenderEnvFilesError ConfigGeneratorReason = "RenderEnvFilesError"
	// ConfigGeneratorCondReasonClusterServiceIPError - error rendering environmane file
	ConfigGeneratorCondReasonClusterServiceIPError ConfigGeneratorReason = "ClusterServiceIPError"
)

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
