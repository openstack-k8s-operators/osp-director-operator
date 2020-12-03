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

// IPAddress
type IPAddress struct {
	Address string `json:"address"`

	NetName string `json:"name"`
}

// OvercloudIPSetSpec defines the desired state of OvercloudIPSet
type OvercloudIPSetSpec struct {

	// NetworkName the name of the OvercloudNetwork to pull IPs from
	NetworkNames []string `json:"networkNames,omitempty"`
}

// OvercloudIPSetStatus defines the observed state of OvercloudIPSet
type OvercloudIPSetStatus struct {
	IPAddresses []IPAddress `json:"ipaddresses,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OvercloudIPSet is the Schema for the overcloudipsets API
type OvercloudIPSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OvercloudIPSetSpec   `json:"spec,omitempty"`
	Status OvercloudIPSetStatus `json:"status,omitempty"`
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
