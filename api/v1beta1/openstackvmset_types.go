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
	BaseImageDVReady bool `json:"baseImageDVReady,omitempty"`
	// VMsReady is the number of ready  kubevirt controller vm instances
	VMsReady int `json:"vmsReady,omitempty"`
	// VMpods are the names of the kubevirt controller vm pods
	VMpods  []string              `json:"vmpods,omitempty"`
	VMHosts map[string]HostStatus `json:"vmHosts,omitempty"`
}

// Host -
type Host struct {
	Hostname          string                                            `json:"hostname"`
	DomainName        string                                            `json:"domainName"`
	DomainNameUniq    string                                            `json:"domainNameUniq"`
	IPAddress         string                                            `json:"ipAddress"`
	NetworkDataSecret string                                            `json:"networkDataSecret"`
	Labels            map[string]string                                 `json:"labels"`
	NNCP              map[string]nmstate.NodeNetworkConfigurationPolicy `json:"nncp"`
	NAD               map[string]networkv1.NetworkAttachmentDefinition  `json:"nad"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osvmset;osvmsets
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack VMSet"

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
	for key, val := range vms.Status.VMHosts {
		ret[key] = val.Hostname
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
