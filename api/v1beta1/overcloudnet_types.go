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

// NetworkConfig contains a Name, CIDR, and reservedIPs
type NetworkConfig struct {

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])(\/(3[0-2]|[1-2][0-9]|[0-9]))$`
	// CIDR the CIDR to use for this network
	CIDR string `json:"CIDR"`

	// +kubebuilder:validation:Required
	// Name the name of this Network
	Name string `json:"name"`

	// +kubebuilder:validation:Optional
	// +listType=set
	// ReservedIPs a set of IPs that are reserved and will not be assigned
	ReservedIPs []string `json:"reservedIPs,omitempty"`
}

// OvercloudNetSpec defines the desired state of OvercloudNet
type OvercloudNetSpec struct {

	// +kubebuilder:validation:Required
	// +listType=map
	// +listMapKey=CIDR
	// Networks a list of networks
	Networks []NetworkConfig `json:"networks,omitempty"`
}

// OvercloudNetStatus defines the observed state of OvercloudNet
type OvercloudNetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OvercloudNet is the Schema for the overcloudnets API
type OvercloudNet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OvercloudNetSpec   `json:"spec,omitempty"`
	Status OvercloudNetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OvercloudNetList contains a list of OvercloudNet
type OvercloudNetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OvercloudNet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OvercloudNet{}, &OvercloudNetList{})
}
