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

const (
	// DefaultOVNChassisPhysNetName - default physnet netname used for OVNStaticBridgeMacMappings
	DefaultOVNChassisPhysNetName = "datacentre"

	// DefaultOVNChassisPhysNetMACPrefix - default prefix used to create MAC addresses for OVNStaticBridgeMacMappings
	DefaultOVNChassisPhysNetMACPrefix = "fa:16:3a"
)

// Physnet - name and prefix to be used for the physnet
type Physnet struct {
	// +kubebuilder:default="datacentre"
	// Name - the name of the physnet
	Name string `json:"name"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="fa:16:3a"
	// MACPrefix - the MAC address prefix to use
	// Locally administered addresses are distinguished from universally administered addresses by setting
	// (assigning the value of 1 to) the second-least-significant bit of the first octet of the address.
	// https://en.wikipedia.org/wiki/MAC_address#Universal_vs._local_(U/L_bit)
	MACPrefix string `json:"macPrefix"`
}

// OpenStackMACAddressSpec defines the desired state of OpenStackMACAddress
type OpenStackMACAddressSpec struct {
	// +kubebuilder:validation:MinItems=1
	// PhysNetworks - physical networks list to create MAC addresses per physnet per node to create OVNStaticBridgeMacMappings
	PhysNetworks []Physnet `json:"physNetworks"`

	// +kubebuilder:validation:Optional
	// RoleReservations, MAC address reservations per role
	RoleReservations map[string]OpenStackMACRoleReservation `json:"roleReservations"`
}

// OpenStackMACRoleReservation -
type OpenStackMACRoleReservation struct {
	// Reservations IP address reservations per role
	Reservations map[string]OpenStackMACNodeReservation `json:"reservations"`
}

// OpenStackMACNodeReservation defines the observed state of the MAC addresses per PhysNetworks
type OpenStackMACNodeReservation struct {
	// Reservations MAC reservations per PhysNetwork
	Reservations map[string]string `json:"reservations"`

	// +kubebuilder:validation:Optional
	// Deleted - node and therefore MAC reservation are flagged as deleted
	Deleted bool `json:"deleted"`
}

// OpenStackMACAddressStatus defines the observed state of OpenStackMACAddress
type OpenStackMACAddressStatus struct {
	// Reservations MAC address reservations per node
	MACReservations map[string]OpenStackMACNodeReservation `json:"macReservations"`

	// ReservedMACCount - the count of all MAC addresses reserved
	ReservedMACCount int `json:"reservedMACCount"`

	// CurrentState - the overall state of the OSMAC cr
	CurrentState shared.ConditionType `json:"currentState"`

	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`
}

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackMACAddress) IsReady() bool {
	return instance.Status.CurrentState == shared.MACCondTypeConfigured
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osmacaddress;osmacaddresses;osmacaddr;osmacaddrs
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack MACAddress"
// +kubebuilder:printcolumn:name="Reserved MACs",type="integer",JSONPath=".status.reservedMACCount"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.currentState`,description="Status"

// OpenStackMACAddress represents Mac address reservations for static OVN bridge mappings
type OpenStackMACAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackMACAddressSpec   `json:"spec,omitempty"`
	Status OpenStackMACAddressStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackMACAddressList contains a list of OpenStackMACAddress
type OpenStackMACAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackMACAddress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackMACAddress{}, &OpenStackMACAddressList{})
}
