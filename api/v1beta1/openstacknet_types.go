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
	nmstateapi "github.com/nmstate/kubernetes-nmstate/api/shared"
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

// NetworkConfiguration - OSP network to create NodeNetworkConfigurationPolicy and NetworkAttachmentDefinition
type NetworkConfiguration struct {
	BridgeName                     string                                        `json:"bridgeName,omitempty"`
	NodeNetworkConfigurationPolicy nmstateapi.NodeNetworkConfigurationPolicySpec `json:"nodeNetworkConfigurationPolicy,omitempty"`
}

// OpenStackNetSpec defines the desired state of OpenStackNet
type OpenStackNetSpec struct {

	// +kubebuilder:validation:Required
	// Cidr the cidr to use for this network
	Cidr string `json:"cidr"`

	// +kubebuilder:validation:Optional
	// Vlan ID of the network
	Vlan int `json:"vlan"`

	// +kubebuilder:validation:Required
	// AllocationStart a set of IPs that are reserved and will not be assigned
	AllocationStart string `json:"allocationStart"`

	// +kubebuilder:validation:Required
	// AllocationEnd a set of IPs that are reserved and will not be assigned
	AllocationEnd string `json:"allocationEnd"`

	// +kubebuilder:validation:Optional
	// Gateway optional gateway for the network
	Gateway string `json:"gateway"`

	// +kubebuilder:validation:Required
	// AttachConfiguration used for NodeNetworkConfigurationPolicy and NetworkAttachmentDefinition
	AttachConfiguration NetworkConfiguration `json:"attachConfiguration"`
}

// OpenStackNetStatus defines the observed state of OpenStackNet
type OpenStackNetStatus struct {
	// Reservations IP address reservations
	Reservations []IPReservation `json:"reservations"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osnet;osnets
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Net"

// OpenStackNet represents the IPAM configuration for baremetal and VM hosts within OpenStack Overcloud deployment
type OpenStackNet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackNetSpec   `json:"spec,omitempty"`
	Status OpenStackNetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackNetList contains a list of OpenStackNet
type OpenStackNetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackNet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackNet{}, &OpenStackNetList{})
}
