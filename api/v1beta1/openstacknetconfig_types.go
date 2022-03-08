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

const (
	// ControlPlaneName -
	ControlPlaneName string = "Control"
	// ControlPlaneNameLower -
	ControlPlaneNameLower string = "ctlplane"
	// DefaultDomainName -
	DefaultDomainName string = "localdomain"
)

// NetDetails of a subnet
type NetDetails struct {
	// +kubebuilder:validation:Required
	// Cidr, network Cidr e.g. 192.168.24.0/24
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

	// +kubebuilder:validation:Optional
	// Routes, list of networks that should be routed via network gateway.
	Routes []Route `json:"routes"`
}

// Subnet defines the tripleo subnet
type Subnet struct {
	// +kubebuilder:validation:Required
	// Name the name of the subnet, for the default ip_subnet, use the same NameLower as the osnet
	Name string `json:"name"`

	// +kubebuilder:validation:Optional
	// IPv4 subnet details
	IPv4 NetDetails `json:"ipv4"`

	// +kubebuilder:validation:Optional
	// IPv6 subnet details
	IPv6 NetDetails `json:"ipv6"`

	// +kubebuilder:validation:Optional
	// Vlan ID of the network
	Vlan int `json:"vlan"`

	// +kubebuilder:validation:Required
	// AttachConfiguration, which attachConfigurations this OSNet uses
	AttachConfiguration string `json:"attachConfiguration"`
}

// Network describes a tripleo network
type Network struct {

	// +kubebuilder:validation:Required
	// Name of the tripleo network this network belongs to, e.g. External, InternalApi, ...
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// NameLower the name of the subnet, name_lower name of the tripleo subnet, e.g. external, internal_api, internal_api_leaf1
	NameLower string `json:"nameLower"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// IsControlPlane indicates if this network is the overcloud control plane network
	IsControlPlane bool `json:"isControlPlane"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1500
	// MTU of the network
	MTU int `json:"mtu"`

	// +kubebuilder:validation:Required
	// Subnets of the tripleo network
	Subnets []Subnet `json:"subnets"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// VIP create virtual ip on the network
	VIP bool `json:"vip"`
}

// OpenStackNetConfigSpec defines the desired state of OpenStackNetConfig
type OpenStackNetConfigSpec struct {

	// +kubebuilder:validation:Required
	// AttachConfigurations used for NodeNetworkConfigurationPolicy or NodeSriovConfigurationPolicy
	AttachConfigurations map[string]NodeConfigurationPolicy `json:"attachConfigurations"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=localdomain
	// DomainName the name of the dns domain for the OSP environment
	DomainName string `json:"domainName"`

	// +kubebuilder:validation:Optional
	// DNSServers, list of dns servers
	DNSServers []string `json:"dnsServers"`

	// +kubebuilder:validation:Optional
	// DNSSearchDomains, list of DNS search domains
	DNSSearchDomains []string `json:"dnsSearchDomains,omitempty"`

	// +kubebuilder:validation:Required
	// Networks, list of all tripleo networks of the deployment
	Networks []Network `json:"networks"`

	// +kubebuilder:validation:Optional
	// OVNBridgeMacMappings - configuration of the physical networks used to create to create static OVN Bridge MAC address mappings.
	// Unique OVN bridge mac address is dynamically allocated by creating OpenStackMACAddress resource and create a MAC per physnet per OpenStack node.
	// This information is used to create the OVNStaticBridgeMacMappings.
	// - If PhysNetworks is not provided, the tripleo default physnet datacentre gets created.
	// - If the macPrefix is not specified for a physnet, the default macPrefix "fa:16:3a" is used.
	// - If PreserveReservations is not specified, the default is true.
	OVNBridgeMacMappings OVNBridgeMacMappingConfig `json:"ovnBridgeMacMappings"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// PreserveReservations - preserve the MAC/IP reservation of the host within a node role when the node got deleted
	// but the role itself is not deleted. The reservations of all nodes in the role get deleted when the full node
	// role is being deleted. (default: true)
	PreserveReservations *bool `json:"preserveReservations"`

	// +kubebuilder:validation:Optional
	// Reservations, manual/static MAC/IP address reservations per node
	Reservations map[string]OpenStackNetStaticNodeReservations `json:"reservations"`
}

// OpenStackNetStaticNodeReservations defines the static reservations of the nodes
type OpenStackNetStaticNodeReservations struct {
	// +kubebuilder:validation:Optional
	// IPReservations, manual/static IP address reservations per network
	IPReservations map[string]string `json:"ipReservations"`

	// +kubebuilder:validation:Optional
	// MACReservations, manual/static MAC address reservations per physnet
	MACReservations map[string]string `json:"macReservations"`
}

// OVNBridgeMacMappingConfig defines the desired state of OpenStackMACAddress
type OVNBridgeMacMappingConfig struct {
	// +kubebuilder:validation:MinItems=1
	// PhysNetworks - physical networks list to create MAC addresses per physnet per node to create OVNStaticBridgeMacMappings
	PhysNetworks []Physnet `json:"physNetworks"`
}

// OpenStackNetConfigStatus defines the observed state of OpenStackNetConfig
type OpenStackNetConfigStatus struct {
	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions         ConditionList                        `json:"conditions,omitempty" optional:"true"`
	ProvisioningStatus OpenStackNetConfigProvisioningStatus `json:"provisioningStatus,omitempty"`

	Hosts map[string]OpenStackHostStatus `json:"hosts"`
}

// OpenStackHostStatus per host IP set
type OpenStackHostStatus struct {
	IPAddresses          map[string]string `json:"ipaddresses"`
	OVNBridgeMacAdresses map[string]string `json:"ovnBridgeMacAdresses"`
}

// OpenStackNetConfigProvisioningStatus represents the overall provisioning state of
// the OpenStackNetConfig (with an optional explanatory message)
type OpenStackNetConfigProvisioningStatus struct {
	State               ProvisioningState `json:"state,omitempty"`
	Reason              string            `json:"reason,omitempty"`
	AttachDesiredCount  int               `json:"attachDesiredCount,omitempty"`
	AttachReadyCount    int               `json:"attachReadyCount,omitempty"`
	NetDesiredCount     int               `json:"netDesiredCount,omitempty"`
	NetReadyCount       int               `json:"netReadyCount,omitempty"`
	PhysNetDesiredCount int               `json:"physNetDesiredCount,omitempty"`
	PhysNetReadyCount   int               `json:"physNetReadyCount,omitempty"`
}

const (
	// NetConfigWaiting - the network configuration is blocked by prerequisite objects
	NetConfigWaiting ProvisioningState = "Waiting"
	// NetConfigInitializing - we are waiting for underlying OCP network resource(s) to appear
	NetConfigInitializing ProvisioningState = "Initializing"
	// NetConfigConfiguring - the underlying network resources are configuring the nodes
	NetConfigConfiguring ProvisioningState = "Configuring"
	// NetConfigConfigured - the nodes have been configured by the underlying network resources
	NetConfigConfigured ProvisioningState = "Configured"
	// NetConfigError - the network configuration hit a generic error
	NetConfigError ProvisioningState = "Error"

	// NetConfigCondReasonnError - error creating osnetcfg
	NetConfigCondReasonnError ConditionReason = "OpenStackNetConfigError"
	// NetConfigCondReasonWaitingOnIPsForHost - waiting on IPs for all configured networks to be created
	NetConfigCondReasonWaitingOnIPsForHost ConditionReason = "WaitingOnIPsForHost"
	// NetConfigCondReasonWaitingOnHost - waiting on host to be added to osnetcfg
	NetConfigCondReasonWaitingOnHost ConditionReason = "WaitingOnHost"
	// NetConfigCondReasonIPReservationError - Failed to do ip reservation
	NetConfigCondReasonIPReservationError ConditionReason = "IPReservationError"
	// NetConfigCondReasonIPReservation - ip reservation created
	NetConfigCondReasonIPReservation ConditionReason = "IPReservationCreated"
)

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackNetConfig) IsReady() bool {
	return instance.Status.ProvisioningStatus.NetDesiredCount == instance.Status.ProvisioningStatus.NetReadyCount &&
		instance.Status.ProvisioningStatus.AttachDesiredCount == instance.Status.ProvisioningStatus.AttachReadyCount &&
		instance.Status.ProvisioningStatus.PhysNetDesiredCount == instance.Status.ProvisioningStatus.PhysNetReadyCount
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=osnetconfig;osnetsconfig;osnetcfg;osnetscfg
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack NetConfig"
//+kubebuilder:printcolumn:name="AttachConfig Desired",type="integer",JSONPath=".status.provisioningStatus.attachDesiredCount",description="AttachConfig Desired"
//+kubebuilder:printcolumn:name="AttachConfig Ready",type="integer",JSONPath=".status.provisioningStatus.attachReadyCount",description="AttachConfig Ready"
//+kubebuilder:printcolumn:name="Networks Desired",type="integer",JSONPath=".status.provisioningStatus.netDesiredCount",description="Networks Desired"
//+kubebuilder:printcolumn:name="Networks Ready",type="integer",JSONPath=".status.provisioningStatus.netReadyCount",description="Networks Ready"
//+kubebuilder:printcolumn:name="PhysNetworks Desired",type="integer",JSONPath=".status.provisioningStatus.physNetDesiredCount",description="PhysNetworks Desired"
//+kubebuilder:printcolumn:name="PhysNetworks Ready",type="integer",JSONPath=".status.provisioningStatus.physNetReadyCount",description="PhysNetworks Ready"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.provisioningStatus.state",description="Status"
//+kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.provisioningStatus.reason",description="Reason"

// OpenStackNetConfig is the Schema for the openstacknetconfigs API
type OpenStackNetConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackNetConfigSpec   `json:"spec,omitempty"`
	Status OpenStackNetConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackNetConfigList contains a list of OpenStackNetConfig
type OpenStackNetConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackNetConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackNetConfig{}, &OpenStackNetConfigList{})
}
