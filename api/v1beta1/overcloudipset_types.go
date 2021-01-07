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

// OvercloudIPSetSpec defines the desired state of OvercloudIPSet
type OvercloudIPSetSpec struct {

	// Networks the name(s) of the OvercloudNetworks used to generate IPs
	Networks []string `json:"networks"`

	// Role the name of the Overcloud role this IPset is associated with. Used to generate the required predictable IPs files.
	Role string `json:"role"`

	// HostCount Host count
	HostCount int `json:"hostCount"`
}

// OvercloudHostIPStatus set of hosts with IP information
type OvercloudHostIPStatus struct {
	HostIPs map[string]OvercloudIPSetStatus `json:"hosts"`
}

// OvercloudIPSetStatus per host IP set
type OvercloudIPSetStatus struct {
	IPAddresses map[string]string `json:"ipaddresses"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OvercloudIPSet is the Schema for the overcloudipsets API
type OvercloudIPSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OvercloudIPSetSpec    `json:"spec,omitempty"`
	Status OvercloudHostIPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OvercloudIPSetList contains a list of OvercloudIPSet
type OvercloudIPSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OvercloudIPSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OvercloudIPSet{}, &OvercloudIPSetList{})
}
