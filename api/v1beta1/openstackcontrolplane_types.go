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

// OSPVersion - OSP template version
type OSPVersion string

const (
	// TemplateVersionTrain - upstream train template version
	TemplateVersionTrain OSPVersion = "train"
	// TemplateVersion16_2 - OSP 16.2 template version
	TemplateVersion16_2 OSPVersion = "16.2"
	// TemplateVersionWallaby - upstream wallaby template version
	TemplateVersionWallaby OSPVersion = "wallaby"
	// TemplateVersion17_0 - OSP 17.0 template version
	TemplateVersion17_0 OSPVersion = "17.0"
)

// OpenStackControlPlaneSpec defines the desired state of OpenStackControlPlane
type OpenStackControlPlaneSpec struct {
	// List of VirtualMachine roles
	VirtualMachineRoles map[string]OpenStackVirtualMachineRoleSpec `json:"virtualMachineRoles"`
	// OpenstackClient image. If missing will be set to the configured OPENSTACKCLIENT_IMAGE_URL_DEFAULT in the CSV for the OSP Director Operator.
	OpenStackClientImageURL string `json:"openStackClientImageURL,omitempty"`
	// OpenStackClientStorageClass storage class
	OpenStackClientStorageClass string `json:"openStackClientStorageClass,omitempty"`
	// PasswordSecret used to e.g specify root pwd
	PasswordSecret string `json:"passwordSecret,omitempty"`
	// GitSecret used to pull playbooks into the openstackclient pod
	GitSecret string `json:"gitSecret"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={ctlplane,external}
	// OpenStackClientNetworks the name(s) of the OpenStackClientNetworks used to attach the openstackclient to
	OpenStackClientNetworks []string `json:"openStackClientNetworks"`

	// +kubebuilder:validation:Optional
	// PhysNetworks - physical networks list with optional MAC address prefix, used to create static OVN Bridge MAC address mappings.
	// Unique OVN bridge mac address is dynamically allocated by creating OpenStackMACAddress resource and create a MAC per physnet per OpenStack node.
	// This information is used to create the OVNStaticBridgeMacMappings.
	// If PhysNetworks is not provided, the tripleo default physnet datacentre gets created
	// If the macPrefix is not specified for a physnet, the default macPrefix "fa:16:3a" is used.
	PhysNetworks []Physnet `json:"physNetworks"`

	// +kubebuilder:default=false
	// EnableFencing is provided so that users have the option to disable fencing if desired
	// FIXME: Defaulting to false until Kubevirt agent merged into RHEL overcloud image
	EnableFencing bool `json:"enableFencing"`

	// Domain name used to build fqdn
	DomainName string `json:"domainName,omitempty"`

	// Upstream DNS servers
	DNSServers       []string `json:"dnsServers,omitempty"`
	DNSSearchDomains []string `json:"dnsSearchDomains,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum={"train","wallaby","16.2","17.0"}
	// OpenStackRelease to overwrite OSPrelease auto detection from tripleoclient container image
	OpenStackRelease string `json:"openStackRelease"`
}

// OpenStackVirtualMachineRoleSpec - defines the desired state of VMs
type OpenStackVirtualMachineRoleSpec struct {
	// Number of VMs for the role
	RoleCount int `json:"roleCount"`
	// number of Cores assigned to the VM
	Cores uint32 `json:"cores"`
	// amount of Memory in GB used by the VM
	Memory uint32 `json:"memory"`
	// root Disc size in GB
	DiskSize uint32 `json:"diskSize"`
	// StorageClass to be used for the controller disks
	StorageClass string `json:"storageClass,omitempty"`
	// BaseImageVolumeName used as the base volume for the VM
	BaseImageVolumeName string `json:"baseImageVolumeName"`

	// +kubebuilder:default=enp2s0
	// Interface to use for ctlplane network
	CtlplaneInterface string `json:"ctlplaneInterface"`

	// +kubebuilder:default={ctlplane,external,internalapi,tenant,storage,storagemgmt}
	// Networks the name(s) of the OpenStackNetworks used to generate IPs
	Networks []string `json:"networks"`

	// RoleName the name of the TripleO role this VM Spec is associated with. If it is a TripleO role, the name must match.
	RoleName string `json:"roleName"`
	// in case of external functionality, like 3rd party network controllers, set to false to ignore role in rendered overcloud templates.
	// +kubebuilder:default=true
	IsTripleoRole bool `json:"isTripleoRole,omitempty"`
}

// OpenStackControlPlaneStatus defines the observed state of OpenStackControlPlane
type OpenStackControlPlaneStatus struct {
	VIPStatus          map[string]HostStatus                   `json:"vipStatus,omitempty"`
	Conditions         ConditionList                           `json:"conditions,omitempty" optional:"true"`
	ProvisioningStatus OpenStackControlPlaneProvisioningStatus `json:"provisioningStatus,omitempty"`

	// OSPVersion the OpenStack version to render templates files
	OSPVersion OSPVersion `json:"ospVersion"`
}

// OpenStackControlPlaneProvisioningStatus represents the overall provisioning state of
// the OpenStackControlPlane (with an optional explanatory message)
type OpenStackControlPlaneProvisioningStatus struct {
	State        ControlPlaneProvisioningState `json:"state,omitempty"`
	Reason       string                        `json:"reason,omitempty"`
	DesiredCount int                           `json:"desiredCount,omitempty"`
	ReadyCount   int                           `json:"readyCount,omitempty"`
	ClientReady  bool                          `json:"clientReady,omitempty"`
}

// ControlPlaneProvisioningState - the overall state of this OpenStackControlPlane
type ControlPlaneProvisioningState string

const (
	// ControlPlaneEmpty - special state for 0 requested VMs and 0 already provisioned
	ControlPlaneEmpty ControlPlaneProvisioningState = "Empty"
	// ControlPlaneWaiting - something is causing the OpenStackBaremetalSet to wait
	ControlPlaneWaiting ControlPlaneProvisioningState = "Waiting"
	// ControlPlaneProvisioning - one or more VMs are provisioning
	ControlPlaneProvisioning ControlPlaneProvisioningState = "Provisioning"
	// ControlPlaneProvisioned - the requested VM count for all roles has been satisfied
	ControlPlaneProvisioned ControlPlaneProvisioningState = "Provisioned"
	// ControlPlaneDeprovisioning - one or more VMs are deprovisioning
	ControlPlaneDeprovisioning ControlPlaneProvisioningState = "Deprovisioning"
	// ControlPlaneError - general catch-all for actual errors
	ControlPlaneError ControlPlaneProvisioningState = "Error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osctlplane;osctlplanes
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack ControlPlane"
// +kubebuilder:printcolumn:name="VMSets Desired",type="integer",JSONPath=".status.provisioningStatus.desiredCount",description="VMSets Desired"
// +kubebuilder:printcolumn:name="VMSets Ready",type="integer",JSONPath=".status.provisioningStatus.readyCount",description="VMSets Ready"
// +kubebuilder:printcolumn:name="Client Ready",type="boolean",JSONPath=".status.provisioningStatus.clientReady",description="Client Ready"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.provisioningStatus.state",description="Status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.provisioningStatus.reason",description="Reason"

// OpenStackControlPlane represents a virtualized OpenStack control plane configuration
type OpenStackControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackControlPlaneSpec   `json:"spec,omitempty"`
	Status OpenStackControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackControlPlaneList contains a list of OpenStackControlPlane
type OpenStackControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackControlPlane{}, &OpenStackControlPlaneList{})
}

// GetHostnames -
func (vips OpenStackControlPlane) GetHostnames() map[string]string {
	ret := make(map[string]string)
	for _, val := range vips.Status.VIPStatus {
		ret[val.Hostname] = val.HostRef
	}
	return ret
}
