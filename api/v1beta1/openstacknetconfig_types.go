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

	// Networks, list of all tripleo networks of the deployment
	Networks []Network `json:"networks"`
}

// OpenStackNetConfigStatus defines the observed state of OpenStackNetConfig
type OpenStackNetConfigStatus struct {
	// CurrentState - the overall state of this network
	//CurrentState NetConfigState `json:"currentState"`

	// TODO: It would be simpler, perhaps, to just have Conditions and get rid of CurrentState,
	// but we are using the same approach in other CRDs for now anyhow
	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions         ConditionList                        `json:"conditions,omitempty" optional:"true"`
	ProvisioningStatus OpenStackNetConfigProvisioningStatus `json:"provisioningStatus,omitempty"`
}

// OpenStackNetConfigProvisioningStatus represents the overall provisioning state of
// the OpenStackNetConfig (with an optional explanatory message)
type OpenStackNetConfigProvisioningStatus struct {
	State              NetConfigState `json:"state,omitempty"`
	Reason             string         `json:"reason,omitempty"`
	AttachDesiredCount int            `json:"attachDesiredCount,omitempty"`
	AttachReadyCount   int            `json:"attachReadyCount,omitempty"`
	NetDesiredCount    int            `json:"netDesiredCount,omitempty"`
	NetReadyCount      int            `json:"netReadyCount,omitempty"`
}

// NetConfigState - the state of this openstack network config
type NetConfigState string

const (
	// NetConfigWaiting - the network configuration is blocked by prerequisite objects
	NetConfigWaiting NetConfigState = "Waiting"
	// NetConfigInitializing - we are waiting for underlying OCP network resource(s) to appear
	NetConfigInitializing NetConfigState = "Initializing"
	// NetConfigConfiguring - the underlying network resources are configuring the nodes
	NetConfigConfiguring NetConfigState = "Configuring"
	// NetConfigConfigured - the nodes have been configured by the underlying network resources
	NetConfigConfigured NetConfigState = "Configured"
	// NetConfigError - the network configuration hit a generic error
	NetConfigError NetConfigState = "Error"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=osnetconfig;osnetsconfig;osnetcfg;osnetscfg
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack NetConfig"
//+kubebuilder:printcolumn:name="AttachConfig Desired",type="integer",JSONPath=".status.provisioningStatus.attachDesiredCount",description="AttachConfig Desired"
//+kubebuilder:printcolumn:name="AttachConfig Ready",type="integer",JSONPath=".status.provisioningStatus.attachReadyCount",description="AttachConfig Ready"
//+kubebuilder:printcolumn:name="Networks Desired",type="integer",JSONPath=".status.provisioningStatus.netDesiredCount",description="Networks Desired"
//+kubebuilder:printcolumn:name="Networks Ready",type="integer",JSONPath=".status.provisioningStatus.netReadyCount",description="Networks Ready"
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
