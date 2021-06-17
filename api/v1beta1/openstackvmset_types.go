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
	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nmstate "github.com/nmstate/kubernetes-nmstate/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackVMSetSpec defines the desired state of an OpenStackVMSet
type OpenStackVMSetSpec struct {
	// Number of VMs to configure, 1 or 3
	VMCount int `json:"vmCount"`
	// number of Cores assigned to the VMs
	Cores uint32 `json:"cores"`
	// amount of Memory in GB used by the VMs
	Memory uint32 `json:"memory"`
	// root Disc size in GB
	DiskSize uint32 `json:"diskSize"`
	// Name of the VM base image used to setup the VMs
	BaseImageURL string `json:"baseImageURL,omitempty"`
	// StorageClass to be used for the disks
	StorageClass string `json:"storageClass,omitempty"`
	// BaseImageVolumeName Optional. If supplied will be used as the base volume for the VM instead of BaseImageURL.
	BaseImageVolumeName string `json:"baseImageVolumeName,omitempty"`
	// name of secret holding the stack-admin ssh keys
	DeploymentSSHSecret string `json:"deploymentSSHSecret"`

	// +kubebuilder:default=enp2s0
	// Interface to use for ctlplane network
	CtlplaneInterface string `json:"ctlplaneInterface"`

	// +kubebuilder:default={ctlplane,external,internalapi,tenant,storage,storagemgmt}
	// Networks the name(s) of the OpenStackNetworks used to generate IPs
	Networks []string `json:"networks"`

	// RoleName the name of the TripleO role this VM Spec is associated with. If it is a TripleO role, the name must match.
	RoleName string `json:"roleName"`
	// in case of external functionality, like 3rd party network controllers, set to false to ignore role in rendered overcloud templates.
	IsTripleoRole bool `json:"isTripleoRole"`
	// PasswordSecret the name of the secret used to optionally set the root pwd by adding
	// NodeRootPassword: <base64 enc pwd>
	// to the secret data
	PasswordSecret string `json:"passwordSecret,omitempty"`
}

// OpenStackVMSetStatus defines the observed state of OpenStackVMSet
type OpenStackVMSetStatus struct {
	// BaseImageDVReady is the status of the BaseImage DataVolume
	BaseImageDVReady   bool                             `json:"baseImageDVReady,omitempty"`
	Conditions         ConditionList                    `json:"conditions,omitempty" optional:"true"`
	ProvisioningStatus OpenStackVMSetProvisioningStatus `json:"provisioningStatus,omitempty"`
	// VMpods are the names of the kubevirt controller vm pods
	VMpods  []string              `json:"vmpods,omitempty"`
	VMHosts map[string]HostStatus `json:"vmHosts,omitempty"`
}

// OpenStackVMSetProvisioningStatus represents the overall provisioning state of all VMs in
// the OpenStackVMSet (with an optional explanatory message)
type OpenStackVMSetProvisioningStatus struct {
	State      VMSetProvisioningState `json:"state,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	ReadyCount int                    `json:"readyCount,omitempty"`
}

// VMSetProvisioningState - the overall state of all VMs in this OpenStackVmSet
type VMSetProvisioningState string

const (
	// VMSetEmpty - special state for 0 requested VMs and 0 already provisioned
	VMSetEmpty VMSetProvisioningState = "Empty"
	// VMSetWaiting - something is causing the OpenStackVmSet to wait
	VMSetWaiting VMSetProvisioningState = "Waiting"
	// VMSetProvisioning - one or more VMs are provisioning
	VMSetProvisioning VMSetProvisioningState = "Provisioning"
	// VMSetProvisioned - the requested VM count has been satisfied
	VMSetProvisioned VMSetProvisioningState = "Provisioned"
	// VMSetDeprovisioning - one or more VMs are deprovisioning
	VMSetDeprovisioning VMSetProvisioningState = "Deprovisioning"
	// VMSetError - general catch-all for actual errors
	VMSetError VMSetProvisioningState = "Error"
)

// Host -
type Host struct {
	Hostname          string                                            `json:"hostname"`
	HostRef           string                                            `json:"hostRef"`
	DomainName        string                                            `json:"domainName"`
	DomainNameUniq    string                                            `json:"domainNameUniq"`
	IPAddress         string                                            `json:"ipAddress"`
	NetworkDataSecret string                                            `json:"networkDataSecret"`
	BaseImageName     string                                            `json:"baseImageName"`
	Labels            map[string]string                                 `json:"labels"`
	NNCP              map[string]nmstate.NodeNetworkConfigurationPolicy `json:"nncp"`
	NAD               map[string]networkv1.NetworkAttachmentDefinition  `json:"nad"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osvmset;osvmsets;osvms
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack VMSet"
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.vmCount",description="Desired"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.provisioningStatus.readyCount",description="Ready"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.provisioningStatus.state",description="Status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.provisioningStatus.reason",description="Reason"

// OpenStackVMSet represents a set of virtual machines hosts for a specific role within the Overcloud deployment
type OpenStackVMSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackVMSetSpec   `json:"spec,omitempty"`
	Status OpenStackVMSetStatus `json:"status,omitempty"`
}

// GetHostnames -
func (vms OpenStackVMSet) GetHostnames() map[string]string {
	ret := make(map[string]string)
	for _, val := range vms.Status.VMHosts {
		ret[val.Hostname] = val.HostRef
	}
	return ret
}

// +kubebuilder:object:root=true

// OpenStackVMSetList contains a list of OpenStackVMSet
type OpenStackVMSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackVMSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackVMSet{}, &OpenStackVMSetList{})
}
