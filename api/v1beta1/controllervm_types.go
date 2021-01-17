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

// ControllerVMSpec defines the desired state of ControllerVM
type ControllerVMSpec struct {
	// Number of controllers to configure, 1 or 3
	ControllerCount int `json:"controllerCount"`
	// number of Cores assigned to the controller VMs
	Cores uint32 `json:"cores"`
	// amount of Memory in GB used by the controller VMs
	Memory uint32 `json:"memory"`
	// root Disc size in GB
	DiskSize uint32 `json:"diskSize"`
	// Name of the VM base image used to setup the controller VMs
	BaseImageURL string `json:"baseImageURL"`
	// StorageClass to be used for the controller disks
	StorageClass string `json:"storageClass"`
	// ImageImportStorageClass used to import base image into the cluster
	ImageImportStorageClass string `json:"imageImportStorageClass"`
	// name of secret holding the stack-admin ssh keys
	DeploymentSSHSecret string `json:"deploymentSSHSecret"`
	// OSPNetwork
	OSPNetwork Network `json:"ospNetwork"`
	// Networks the name(s) of the OvercloudNetworks used to generate IPs
	Networks []string `json:"networks"`
	// Role the name of the Overcloud role this IPset is associated with. Used to generate hostnames.
	Role string `json:"role"`
}

// ControllerHostStatus represents the hostname and IP info for a specific controller
type ControllerHostStatus struct {
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ipaddress"`
}

// ControllerVMStatus defines the observed state of ControllerVM
type ControllerVMStatus struct {
	// BaseImageDVReady is the status of the BaseImage DataVolume
	BaseImageDVReady bool `json:"baseImageDVReady"`
	// ControllersReady is the number of ready  kubevirt controller vm instances
	ControllersReady int32 `json:"controllersReady"`
	// Controllers are the names of the kubevirt controller vm pods
	Controllers     []string                        `json:"controllers"`
	ControllerHosts map[string]ControllerHostStatus `json:"controllerHosts"`
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

// ControllerVM is the Schema for the controllervms API
type ControllerVM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControllerVMSpec   `json:"spec,omitempty"`
	Status ControllerVMStatus `json:"status,omitempty"`
}

// GetHostnames -
func (cvm ControllerVM) GetHostnames() map[string]string {
	ret := make(map[string]string)
	for key, val := range cvm.Status.ControllerHosts {
		ret[key] = val.Hostname
	}
	return ret
}

// +kubebuilder:object:root=true

// ControllerVMList contains a list of ControllerVM
type ControllerVMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControllerVM `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ControllerVM{}, &ControllerVMList{})
}
