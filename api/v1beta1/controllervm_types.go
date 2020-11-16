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
	// name of secret holding the stack-admin ssh keys
	DeploymentSSHSecret string `json:"deploymentSSHSecret"`
	// Networks - e.g. ctlplane, tenant, internalAPI, storage, storageMgmt, external
	Networks []Network `json:"networks"`
}

// ControllerVMStatus defines the observed state of ControllerVM
type ControllerVMStatus struct {
	// BaseImageDVReady is the status of the BaseImage DataVolume
	BaseImageDVReady bool `json:"baseImageDVReady"`
	// ControllersReady is the number of ready  kubevirt controller vm instances
	ControllersReady int32 `json:"controllersReady"`
	// Controllers are the names of the kubevirt controller vm pods
	Controllers []string `json:"controllers"`
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
