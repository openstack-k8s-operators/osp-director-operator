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

// IPReservation contains an IP, Hostname, and a VIP flag
type IPReservation struct {
	//IP       net.IP `json:"ip"`
	IDKey               string `json:"idKey"`
	IP                  string `json:"ip"`
	Hostname            string `json:"hostname"`
	VIP                 bool   `json:"vip"`
	Role                string `json:"role"`
	AddToPredictableIPs bool   `json:"addToPredictableIPs"`
}

// OvercloudNetSpec defines the desired state of OvercloudNet
type OvercloudNetSpec struct {

	// +kubebuilder:validation:Required
	// Cidr the cidr to use for this network
	Cidr string `json:"cidr"`

	// +kubebuilder:validation:Required
	// AllocationStart a set of IPs that are reserved and will not be assigned
	AllocationStart string `json:"allocationStart"`

	// +kubebuilder:validation:Required
	// AllocationEnd a set of IPs that are reserved and will not be assigned
	AllocationEnd string `json:"allocationEnd"`

	// +kubebuilder:validation:Optional
	// Gateway optional gateway for the network
	Gateway string `json:"gateway"`
}

// OvercloudNetStatus defines the observed state of OvercloudNet
type OvercloudNetStatus struct {
	// Reservations IP address reservations
	Reservations []IPReservation `json:"reservations"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OvercloudNet represents the IPAM configuration for baremetal and VM hosts within OpenStack Overcloud deployment
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
