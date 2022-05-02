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

// OpenStackEphemeralHeatSpec defines the desired state of OpenStackEphemeralHeat
type OpenStackEphemeralHeatSpec struct {
	// ConfigHash hash which acts as a unique identifier for this ephemeral heat instance
	ConfigHash string `json:"configHash,omitempty"`
	// Container image URL for the MySQL container image used as part of this ephemeral heat instance
	MariadbImageURL string `json:"mariadbImageURL,omitempty"`
	// Container image URL for the RabbitMQ container image used as part of this ephemeral heat instance
	RabbitImageURL string `json:"rabbitImageURL,omitempty"`
	// Container image URL for the Heat API container image used as part of this ephemeral heat instance
	HeatAPIImageURL string `json:"heatAPIImageURL,omitempty"`
	// Container image URL for the Heat Engine container image used as part of this ephemeral heat instance
	HeatEngineImageURL string `json:"heatEngineImageURL,omitempty"`
	// Number of replicas for the Heat Engine service, defaults to 3 if unset
	// +kubebuilder:default=3
	HeatEngineReplicas int32 `json:"heatEngineReplicas,omitempty"`
}

// OpenStackEphemeralHeatStatus defines the observed state of OpenStackEphemeralHeat
type OpenStackEphemeralHeatStatus struct {
	// Active hash
	Active bool `json:"active"`

	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osephemeralheat;osephemeralheats
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Ephemeral Heat"
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`

// OpenStackEphemeralHeat an ephermeral OpenStack Heat deployment used internally by the OSConfigGenerator to generate Ansible playbooks
type OpenStackEphemeralHeat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackEphemeralHeatSpec   `json:"spec,omitempty"`
	Status OpenStackEphemeralHeatStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackEphemeralHeatList contains a list of OpenStackEphemeralHeat
type OpenStackEphemeralHeatList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackEphemeralHeat `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackEphemeralHeat{}, &OpenStackEphemeralHeatList{})
}
