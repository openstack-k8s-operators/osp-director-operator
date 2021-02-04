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

// VMSetSpec defines the desired state of VMSet
type VMSetSpec struct {
	// Number of VMs to configure, 1 or 3
	VMCount int `json:"vmCount"`
	// number of Cores assigned to the VMs
	Cores uint32 `json:"cores"`
	// amount of Memory in GB used by the VMs
	Memory uint32 `json:"memory"`
	// root Disc size in GB
	DiskSize uint32 `json:"diskSize"`
	// Name of the VM base image used to setup the VMs
	BaseImageURL string `json:"baseImageURL"`
	// StorageClass to be used for the disks
	StorageClass string `json:"storageClass"`
	// BaseImageVolumeName Optional. If supplied will be used as the base volume for the VM instead of BaseImageURL.
	BaseImageVolumeName string `json:"baseImageVolumeName,omitempty"`
	// name of secret holding the stack-admin ssh keys
	DeploymentSSHSecret string `json:"deploymentSSHSecret"`
	// OSPNetwork
	OSPNetwork Network `json:"ospNetwork"`
	// Networks the name(s) of the OvercloudNetworks used to generate IPs
	Networks []string `json:"networks"`
	// Role the name of the Overcloud role this IPset is associated with. Used to generate hostnames.
	Role string `json:"role"`
	// PasswordSecret the name of the secret used to optionally set the root pwd by adding
	// NodeRootPassword: <base64 enc pwd>
	// to the secret data
	PasswordSecret string `json:"passwordSecret,omitempty"`
}

// VMHostStatus represents the hostname and IP info for a specific VM
type VMHostStatus struct {
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ipaddress"`
}

// VMSetStatus defines the observed state of VMSet
type VMSetStatus struct {
	// BaseImageDVReady is the status of the BaseImage DataVolume
	BaseImageDVReady bool `json:"baseImageDVReady,omitempty"`
	// VMsReady is the number of ready  kubevirt controller vm instances
	VMsReady int `json:"vmsReady,omitempty"`
	// VMpods are the names of the kubevirt controller vm pods
	VMpods  []string                `json:"vmpods,omitempty"`
	VMHosts map[string]VMHostStatus `json:"vmHosts,omitempty"`
}

// Host -
type Host struct {
	Hostname          string `json:"hostname"`
	DomainName        string `json:"domainName"`
	DomainNameUniq    string `json:"domainNameUniq"`
	IPAddress         string `json:"ipAddress"`
	NetworkDataSecret string `json:"networkDataSecret"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// VMSet represents a set of virtual machines hosts for a specific role within the Overcloud deployment
type VMSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMSetSpec   `json:"spec,omitempty"`
	Status VMSetStatus `json:"status,omitempty"`
}

// GetHostnames -
func (vms VMSet) GetHostnames() map[string]string {
	ret := make(map[string]string)
	for key, val := range vms.Status.VMHosts {
		ret[key] = val.Hostname
	}
	return ret
}

// +kubebuilder:object:root=true

// VMSetList contains a list of VMSet
type VMSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VMSet{}, &VMSetList{})
}
