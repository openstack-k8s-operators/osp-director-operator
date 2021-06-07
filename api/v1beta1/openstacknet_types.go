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
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	VIP      bool   `json:"vip"`
	Deleted  bool   `json:"deleted"`
}

// NetworkConfiguration - OSP network to create NodeNetworkConfigurationPolicy and NetworkAttachmentDefinition
type NetworkConfiguration struct {
	BridgeName string `json:"bridgeName,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	NodeNetworkConfigurationPolicy nmstateapi.NodeNetworkConfigurationPolicySpec `json:"nodeNetworkConfigurationPolicy,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	NodeSriovConfigurationPolicy NodeSriovConfigurationPolicy `json:"nodeSriovConfigurationPolicy,omitempty"`
}

// NodeSriovConfigurationPolicy - Node selector and desired state for SRIOV network
type NodeSriovConfigurationPolicy struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	DesiredState SriovState        `json:"desiredState,omitempty"`
}

// SriovState - SRIOV-specific configuration details for an OSP network
type SriovState struct {
	// +kubebuilder:default=vfio-pci
	DeviceType string `json:"deviceType,omitempty"`
	// +kubebuilder:default=9000
	Mtu        uint32 `json:"mtu,omitempty"`
	NumVfs     uint32 `json:"numVfs"`
	Port       string `json:"port"`
	RootDevice string `json:"rootDevice,omitempty"`
	// +kubebuilder:validation:Enum={"on","off"}
	// +kubebuilder:default=on
	SpoofCheck string `json:"spoofCheck,omitempty"`
	// +kubebuilder:validation:Enum={"on","off"}
	// +kubebuilder:default=off
	Trust string `json:"trust,omitempty"`
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

// OpenStackNetRoleStatus defines the observed state of the Role Net status
type OpenStackNetRoleStatus struct {
	// Reservations IP address reservations
	Reservations        []IPReservation `json:"reservations"`
	AddToPredictableIPs bool            `json:"addToPredictableIPs"`
}

// OpenStackNetStatus defines the observed state of OpenStackNet
type OpenStackNetStatus struct {
	// Reservations IP address reservations per role
	RoleReservations map[string]OpenStackNetRoleStatus `json:"roleReservations"`

	// ReservedIPCount - the count of all IPs ever reserved on this network
	ReservedIPCount int `json:"reservedIpCount"`

	// CurrentState - the overall state of this network
	CurrentState NetState `json:"currentState"`

	// TODO: It would be simpler, perhaps, to just have Conditions and get rid of CurrentState,
	// but we are using the same approach in other CRDs for now anyhow
	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions ConditionList `json:"conditions,omitempty" optional:"true"`
}

// NetState - the state of this openstack network
type NetState string

const (
	// NetWaiting - the network configuration is blocked by prerequisite objects
	NetWaiting NetState = "Waiting"
	// NetInitializing - we are waiting for underlying OCP network resource(s) to appear
	NetInitializing NetState = "Initializing"
	// NetConfiguring - the underlying network resources are configuring the nodes
	NetConfiguring NetState = "Configuring"
	// NetConfigured - the nodes have been configured by the underlying network resources
	NetConfigured NetState = "Configured"
	// NetError - the network configuration hit a generic error
	NetError NetState = "Error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osnet;osnets
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Net"
// +kubebuilder:printcolumn:name="CIDR",type=string,JSONPath=`.spec.cidr`
// +kubebuilder:printcolumn:name="VLAN",type=string,JSONPath=`.spec.vlan`
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.spec.gateway`
// +kubebuilder:printcolumn:name="Reserved IPs",type="integer",JSONPath=".status.reservedIpCount"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.currentState`,description="Status"

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
