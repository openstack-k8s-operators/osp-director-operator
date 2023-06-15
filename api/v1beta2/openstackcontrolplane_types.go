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

package v1beta2

import (
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackControlPlaneSpec defines the desired state of OpenStackControlPlane
type OpenStackControlPlaneSpec struct {
	// List of VirtualMachine roles
	VirtualMachineRoles map[string]OpenStackVirtualMachineRoleSpec `json:"virtualMachineRoles"`
	// +kubebuilder:validation:Optional
	// OpenstackClient image. If missing will be set to the configured OPENSTACKCLIENT_IMAGE_URL_DEFAULT in the CSV for the OSP Director Operator.
	OpenStackClientImageURL string `json:"openStackClientImageURL,omitempty"`
	// OpenStackClientStorageClass storage class
	OpenStackClientStorageClass string `json:"openStackClientStorageClass,omitempty"`
	// PasswordSecret used to e.g specify root pwd
	PasswordSecret string `json:"passwordSecret,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={ctlplane,external}
	// OpenStackClientNetworks the name(s) of the OpenStackClientNetworks used to attach the openstackclient to
	OpenStackClientNetworks []string `json:"openStackClientNetworks"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum={"train","wallaby","16.2","17.0","17.1"}
	// OpenStackRelease to overwrite OSPrelease auto detection from tripleoclient container image
	OpenStackRelease string `json:"openStackRelease"`

	// Idm secret used to register openstack client in IPA
	IdmSecret string `json:"idmSecret,omitempty"`

	// Name of the config map containing custom CA certificates to trust
	CAConfigMap string `json:"caConfigMap,omitempty"`

	// +kubebuilder:validation:Optional
	// AdditionalServiceVIPs - Map of service name <-> subnet nameLower for which a IP should get reserved on
	// These are used to create the <Service>VirtualFixedIPs environment parameters starting wallaby/OSP17.0.
	// Default "Redis":  "internal_api",
	//         "OVNDBs": "internal_api",
	// https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/deployment/network_v2.html#service-virtual-ips
	// Note: OSP17 networkv2 only
	AdditionalServiceVIPs map[string]string `json:"additionalServiceVIPs,omitempty"`
}

// OpenStackVirtualMachineRoleSpec - defines the desired state of VMs
type OpenStackVirtualMachineRoleSpec struct {
	// Number of VMs for the role
	RoleCount int `json:"roleCount"`
	// number of Cores assigned to the VM
	Cores uint32 `json:"cores"`
	// amount of Memory in GB used by the VM
	Memory uint32 `json:"memory"`

	// +kubebuilder:validation:Optional
	// (deprecated) root Disc size in GB - use RootDisk.DiskSize instead
	DiskSize uint32 `json:"diskSize,omitempty"`
	// +kubebuilder:validation:Optional
	// (deprecated) StorageClass to be used for the controller disks - use RootDisk.
	StorageClass string `json:"storageClass,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteMany
	// (deprecated) StorageAccessMode - use RootDisk.StorageAccessMode instead
	// Virtual machines must have a persistent volume claim (PVC) with a shared ReadWriteMany (RWX) access mode to be live migrated.
	StorageAccessMode string `json:"storageAccessMode,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Block;Filesystem
	// (deprecated) StorageVolumeMode - use RootDisk.StorageVolumeMode instead
	// When using OpenShift Virtualization with OpenShift Container Platform Container Storage,
	// specify RBD block mode persistent volume claims (PVCs) when creating virtual machine disks. With virtual machine disks,
	// RBD block mode volumes are more efficient and provide better performance than Ceph FS or RBD filesystem-mode PVCs.
	// To specify RBD block mode PVCs, use the 'ocs-storagecluster-ceph-rbd' storage class and VolumeMode: Block.
	StorageVolumeMode string `json:"storageVolumeMode,omitempty"`
	// +kubebuilder:validation:Optional
	// (deprecated) BaseImageVolumeName used as the base volume for the VM  - use RootDisk.BaseImageVolumeName instead
	BaseImageVolumeName string `json:"baseImageVolumeName,omitempty"`

	// RootDisk specification of the VM
	RootDisk OpenStackVMSetDisk `json:"rootDisk"`
	// AdditionalDisks additional disks to add to the VM
	AdditionalDisks []OpenStackVMSetDisk `json:"additionalDisks,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=auto;shared
	// IOThreadsPolicy - IO thread policy for the domain. Currently valid policies are shared and auto.
	// However, if any disk requests a dedicated IOThread, ioThreadsPolicy will be enabled and default to shared.
	// When ioThreadsPolicy is set to auto IOThreads will also be "isolated" from the vCPUs and placed on the same physical CPU as the QEMU emulator thread.
	// An ioThreadsPolicy of shared indicates that KubeVirt should use one thread that will be shared by all disk devices.
	IOThreadsPolicy string `json:"ioThreadsPolicy,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Block Multi-Queue is a framework for the Linux block layer that maps Device I/O queries to multiple queues.
	// This splits I/O processing up across multiple threads, and therefor multiple CPUs. libvirt recommends that the
	// number of queues used should match the number of CPUs allocated for optimal performance.
	BlockMultiQueue bool `json:"blockMultiQueue"`

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

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this VMset
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// GrowvolsArgs - arguments to the growvols command to expand logical volumes after provisioning
	// Note requires the command to exist on the base image
	GrowvolsArgs []string `json:"growvolsArgs,omitempty"`
}

// OpenStackControlPlaneStatus defines the observed state of OpenStackControlPlane
type OpenStackControlPlaneStatus struct {
	VIPStatus          map[string]HostStatus                   `json:"vipStatus,omitempty"`
	Conditions         shared.ConditionList                    `json:"conditions,omitempty" optional:"true"`
	ProvisioningStatus OpenStackControlPlaneProvisioningStatus `json:"provisioningStatus,omitempty"`

	// OSPVersion the OpenStack version to render templates files
	OSPVersion shared.OSPVersion `json:"ospVersion"`
}

// OpenStackControlPlaneProvisioningStatus represents the overall provisioning state of
// the OpenStackControlPlane (with an optional explanatory message)
type OpenStackControlPlaneProvisioningStatus struct {
	State        shared.ProvisioningState `json:"state,omitempty"`
	Reason       string                   `json:"reason,omitempty"`
	DesiredCount int                      `json:"desiredCount,omitempty"`
	ReadyCount   int                      `json:"readyCount,omitempty"`
	ClientReady  bool                     `json:"clientReady,omitempty"`
}

// ControlPlaneProvisioningReason - the reason of the condition for this openstack ctlplane
type ControlPlaneProvisioningReason string

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackControlPlane) IsReady() bool {
	return instance.Status.ProvisioningStatus.State == shared.ProvisioningState(shared.ControlPlaneProvisioned)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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
func (instance OpenStackControlPlane) GetHostnames() map[string]string {
	ret := make(map[string]string)
	for _, val := range instance.Status.VIPStatus {
		ret[val.Hostname] = val.HostRef
	}
	return ret
}
