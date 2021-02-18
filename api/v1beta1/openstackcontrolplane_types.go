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

// OpenStackControlPlaneSpec defines the desired state of OpenStackControlPlane
type OpenStackControlPlaneSpec struct {
	Controller OpenStackControllerSpec `json:"controller"`
	// OpenstackClient image
	OpenStackClientImageURL string `json:"openStackClientImageURL"`
	// PasswordSecret used to e.g specify root pwd
	PasswordSecret string `json:"passwordSecret,omitempty"`
}

// OpenStackControllerSpec - defines the desired state of VMs VMs
type OpenStackControllerSpec struct {
	// Number of controllers to configure, 1 or 3
	ControllerCount int `json:"controllerCount"`
	// number of Cores assigned to the controller VMs
	Cores uint32 `json:"cores"`
	// amount of Memory in GB used by the controller VMs
	Memory uint32 `json:"memory"`
	// root Disc size in GB
	DiskSize uint32 `json:"diskSize"`
	// Name of the VM base image used to setup the controller VMs
	BaseImageURL string `json:"baseImageURL,omitempty"`
	// StorageClass to be used for the controller disks
	StorageClass string `json:"storageClass"`
	// BaseImageVolumeName Optional. If supplied will be used as the base volume for the VM instead of BaseImageURL.
	BaseImageVolumeName string `json:"baseImageVolumeName,omitempty"`
	// OSPNetwork
	OSPNetwork Network `json:"ospNetwork"`
	// Networks the name(s) of the OpenStackNetworks used to generate IPs
	Networks []string `json:"networks"`
	// Role the name of the Overcloud role this IPset is associated with. Used to generate hostnames.
	Role string `json:"role"`
}

// OpenStackControlPlaneStatus defines the observed state of OpenStackControlPlane
type OpenStackControlPlaneStatus struct {
	VIPStatus map[string]HostStatus `json:"vipStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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
	for key, val := range vips.Status.VIPStatus {
		ret[key] = val.Hostname
	}
	return ret
}
