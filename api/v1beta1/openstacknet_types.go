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

// IPReservation contains an IP, Hostname, and a VIP flag
type IPReservation struct {
	IP         string `json:"ip"`
	Hostname   string `json:"hostname"`
	VIP        bool   `json:"vip"`
	ServiceVIP bool   `json:"serviceVIP,omitempty"`
	Deleted    bool   `json:"deleted"`
}

// NodeIPReservation contains an IP and Deleted flag
type NodeIPReservation struct {
	IP      string `json:"ip"`
	Deleted bool   `json:"deleted"`
}

// Route definition
type Route struct {
	// +kubebuilder:validation:Required
	// Destination, network CIDR
	Destination string `json:"destination"`

	// +kubebuilder:validation:Required
	// Nexthop, gateway for the destination
	Nexthop string `json:"nexthop"`
}

// OpenStackNetSpec defines the desired state of OpenStackNet
type OpenStackNetSpec struct {

	// +kubebuilder:validation:Required
	// Name of the tripleo network this network belongs to, e.g. Control, External, InternalApi, ...
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// NameLower the name of the subnet, name_lower name of the tripleo subnet, e.g. ctlplane, external, internal_api, internal_api_leaf1
	NameLower string `json:"nameLower"`

	// +kubebuilder:validation:Required
	// Cidr the cidr to use for this network
	Cidr string `json:"cidr"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Maximum=4094
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

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1500
	// MTU of the network
	MTU int `json:"mtu"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// VIP create virtual ip on the network
	VIP bool `json:"vip"`

	// +kubebuilder:validation:Optional
	// Routes, list of networks that should be routed via network gateway.
	Routes []Route `json:"routes"`

	// +kubebuilder:validation:Required
	// AttachConfiguration, used for virtual machines to attach to this network
	AttachConfiguration string `json:"attachConfiguration"`

	// +kubebuilder:validation:Required
	// DomainName the name of the domain for this network, usually lower(Name)."OSNetConfig.Spec.DomainName"
	DomainName string `json:"domainName"`

	// +kubebuilder:validation:Optional
	// Reservations, IP address reservations per role
	RoleReservations map[string]OpenStackNetRoleReservation `json:"roleReservations"`
}

// OpenStackNetRoleStatus defines the observed state of the Role Net status
type OpenStackNetRoleStatus struct {
	// Reservations IP address reservations
	Reservations        []IPReservation `json:"reservations"`
	AddToPredictableIPs bool            `json:"addToPredictableIPs"`
}

// OpenStackNetRoleReservation defines the observed state of the Role Net reservation
type OpenStackNetRoleReservation struct {
	// Reservations IP address reservations
	Reservations        []IPReservation `json:"reservations"`
	AddToPredictableIPs bool            `json:"addToPredictableIPs"`
}

// OpenStackNetStatus defines the observed state of OpenStackNet
type OpenStackNetStatus struct {
	// Reservations MAC address reservations per node
	Reservations map[string]NodeIPReservation `json:"reservations"`

	// ReservedIPCount - the count of all IPs ever reserved on this network
	ReservedIPCount int `json:"reservedIpCount"`

	// CurrentState - the overall state of this network
	CurrentState shared.ConditionType `json:"currentState"`

	// TODO: It would be simpler, perhaps, to just have Conditions and get rid of CurrentState,
	// but we are using the same approach in other CRDs for now anyhow
	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`
}

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackNet) IsReady() bool {
	return instance.Status.CurrentState == shared.NetConfigured
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osnet;osnets
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Net"
// +kubebuilder:printcolumn:name="CIDR",type=string,JSONPath=`.spec.cidr`
// +kubebuilder:printcolumn:name="DOMAIN",type=string,JSONPath=`.spec.domainName`
// +kubebuilder:printcolumn:name="MTU",type=string,JSONPath=`.spec.mtu`
// +kubebuilder:printcolumn:name="VLAN",type=string,JSONPath=`.spec.vlan`
// +kubebuilder:printcolumn:name="VIP",type=boolean,JSONPath=`.spec.vip`
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.spec.gateway`
// +kubebuilder:printcolumn:name="Routes",type=string,JSONPath=`.spec.routes`
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
