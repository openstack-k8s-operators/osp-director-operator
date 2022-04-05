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
	// RootDisk specification of the VM
	RootDisk OpenStackVMSetDisk `json:"rootDisk"`
	// +kubebuilder:validation:Optional
	// AdditionalDisks additional disks to add to the VM
	AdditionalDisks []OpenStackVMSetDisk `json:"additionalDisks,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=default;auto;shared
	// IOThreadsPolicy - IO thread policy for the domain. Currently valid policies are shared and auto.
	// If IOThreadsPolicy is default, use of IOThreads will be disabled. However, if any disk requests a dedicated IOThread, ioThreadsPolicy will be enabled and default to shared.
	// When ioThreadsPolicy is set to auto IOThreads will also be "isolated" from the vCPUs and placed on the same physical CPU as the QEMU emulator thread.
	// An ioThreadsPolicy of shared indicates that KubeVirt should use one thread that will be shared by all disk devices.
	IOThreadsPolicy string `json:"ioThreadsPolicy"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Block Multi-Queue is a framework for the Linux block layer that maps Device I/O queries to multiple queues.
	// This splits I/O processing up across multiple threads, and therefor multiple CPUs. libvirt recommends that the
	// number of queues used should match the number of CPUs allocated for optimal performance.
	BlockMultiQueue bool `json:"blockMultiQueue"`

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

	// BootstrapDNS - initial DNS nameserver values to set on the VM when they are provisioned.
	// Note that subsequent TripleO deployment will overwrite these values
	BootstrapDNS     []string `json:"bootstrapDns,omitempty"`
	DNSSearchDomains []string `json:"dnsSearchDomains,omitempty"`
}

// OpenStackVMSetDisk defines additional disk properties
type OpenStackVMSetDisk struct {
	// Name of the additional disk, e.g. used to do the PVC request
	Name string `json:"name"`
	// Disc size in GB
	DiskSize uint32 `json:"diskSize"`
	// StorageClass to be used for the disk
	StorageClass string `json:"storageClass,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=ReadWriteMany
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteMany
	// StorageAccessMode - Virtual machines must have a persistent volume claim (PVC)
	// with a shared ReadWriteMany (RWX) access mode to be live migrated.
	StorageAccessMode string `json:"storageAccessMode,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=Filesystem
	// +kubebuilder:validation:Enum=Block;Filesystem
	// StorageVolumeMode - When using OpenShift Virtualization with OpenShift Container Platform Container Storage,
	// specify RBD block mode persistent volume claims (PVCs) when creating virtual machine disks. With virtual machine disks,
	// RBD block mode volumes are more efficient and provide better performance than Ceph FS or RBD filesystem-mode PVCs.
	// To specify RBD block mode PVCs, use the 'ocs-storagecluster-ceph-rbd' storage class and VolumeMode: Block.
	StorageVolumeMode string `json:"storageVolumeMode"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// DedicatedIOThread - Disks with dedicatedIOThread set to true will be allocated an exclusive thread.
	// This is generally useful if a specific Disk is expected to have heavy I/O traffic, e.g. a database spindle.
	DedicatedIOThread bool `json:"dedicatedIOThread"`
	// BaseImageVolumeName used as the base volume for the rootdisk of the VM
	BaseImageVolumeName string `json:"baseImageVolumeName"`
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
	State      ProvisioningState `json:"state,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	ReadyCount int               `json:"readyCount,omitempty"`
}

const (
	//
	// condition types
	//

	// VMSetCondTypeEmpty - special state for 0 requested VMs and 0 already provisioned
	VMSetCondTypeEmpty ProvisioningState = "Empty"
	// VMSetCondTypeWaiting - something is causing the OpenStackVmSet to wait
	VMSetCondTypeWaiting ProvisioningState = "Waiting"
	// VMSetCondTypeProvisioning - one or more VMs are provisioning
	VMSetCondTypeProvisioning ProvisioningState = "Provisioning"
	// VMSetCondTypeProvisioned - the requested VM count has been satisfied
	VMSetCondTypeProvisioned ProvisioningState = "Provisioned"
	// VMSetCondTypeDeprovisioning - one or more VMs are deprovisioning
	VMSetCondTypeDeprovisioning ProvisioningState = "Deprovisioning"
	// VMSetCondTypeError - general catch-all for actual errors
	VMSetCondTypeError ProvisioningState = "Error"

	//
	// condition reasones
	//

	// VMSetCondReasonError - error creating osvmset
	VMSetCondReasonError ConditionReason = "OpenStackVMSetError"
	// VMSetCondReasonInitialize - vmset initialize
	VMSetCondReasonInitialize ConditionReason = "OpenStackVMSetInitialize"
	// VMSetCondReasonProvisioning - vmset provisioning
	VMSetCondReasonProvisioning ConditionReason = "OpenStackVMSetProvisioning"
	// VMSetCondReasonDeprovisioning - vmset deprovisioning
	VMSetCondReasonDeprovisioning ConditionReason = "OpenStackVMSetDeprovisioning"
	// VMSetCondReasonProvisioned - vmset provisioned
	VMSetCondReasonProvisioned ConditionReason = "OpenStackVMSetProvisioned"
	// VMSetCondReasonCreated - vmset created
	VMSetCondReasonCreated ConditionReason = "OpenStackVMSetCreated"

	// VMSetCondReasonNamespaceFencingDataError - error creating the namespace fencing data
	VMSetCondReasonNamespaceFencingDataError ConditionReason = "NamespaceFencingDataError"
	// VMSetCondReasonKubevirtFencingServiceAccountError - error creating/reading the KubevirtFencingServiceAccount secret
	VMSetCondReasonKubevirtFencingServiceAccountError ConditionReason = "KubevirtFencingServiceAccountError"
	// VMSetCondReasonKubeConfigError - error getting the KubeConfig used by the operator
	VMSetCondReasonKubeConfigError ConditionReason = "KubeConfigError"
	// VMSetCondReasonCloudInitSecretError - error creating the CloudInitSecret
	VMSetCondReasonCloudInitSecretError ConditionReason = "CloudInitSecretError"
	// VMSetCondReasonDeploymentSecretMissing - deployment secret does not exist
	VMSetCondReasonDeploymentSecretMissing ConditionReason = "DeploymentSecretMissing"
	// VMSetCondReasonDeploymentSecretError - deployment secret error
	VMSetCondReasonDeploymentSecretError ConditionReason = "DeploymentSecretError"
	// VMSetCondReasonPasswordSecretMissing - password secret does not exist
	VMSetCondReasonPasswordSecretMissing ConditionReason = "PasswordSecretMissing"
	// VMSetCondReasonPasswordSecretError - password secret error
	VMSetCondReasonPasswordSecretError ConditionReason = "PasswordSecretError"

	// VMSetCondReasonVirtualMachineGetError - failed to get virtual machine
	VMSetCondReasonVirtualMachineGetError ConditionReason = "VirtualMachineGetError"
	// VMSetCondReasonVirtualMachineAnnotationMissmatch - Unable to find sufficient amount of VirtualMachine replicas annotated for scale-down
	VMSetCondReasonVirtualMachineAnnotationMissmatch ConditionReason = "VirtualMachineAnnotationMissmatch"
	// VMSetCondReasonVirtualMachineNetworkDataError - Error creating VM NetworkData
	VMSetCondReasonVirtualMachineNetworkDataError ConditionReason = "VMSetCondReasonVirtualMachineNetworkDataError"
	// VMSetCondReasonVirtualMachineProvisioning - virtual machine provisioning in progress
	VMSetCondReasonVirtualMachineProvisioning ConditionReason = "VirtualMachineProvisioning"
	// VMSetCondReasonVirtualMachineDeprovisioning - virtual machine deprovisioning in progress
	VMSetCondReasonVirtualMachineDeprovisioning ConditionReason = "VirtualMachineDeprovisioning"
	// VMSetCondReasonVirtualMachineProvisioned - virtual machines provisioned
	VMSetCondReasonVirtualMachineProvisioned ConditionReason = "VirtualMachineProvisioned"
	// VMSetCondReasonVirtualMachineCountZero - no virtual machines requested
	VMSetCondReasonVirtualMachineCountZero ConditionReason = "VirtualMachineCountZero"

	// VMSetCondReasonPersitentVolumeClaimNotFound - Persitent Volume Claim Not Found
	VMSetCondReasonPersitentVolumeClaimNotFound ConditionReason = "PersitentVolumeClaimNotFound"
	// VMSetCondReasonPersitentVolumeClaimError - Persitent Volume Claim error
	VMSetCondReasonPersitentVolumeClaimError ConditionReason = "PersitentVolumeClaimError"
	// VMSetCondReasonPersitentVolumeClaimCreating - Persitent Volume Claim create in progress
	VMSetCondReasonPersitentVolumeClaimCreating ConditionReason = "PersitentVolumeClaimCreating"
	// VMSetCondReasonBaseImageNotReady - VM base image not ready
	VMSetCondReasonBaseImageNotReady ConditionReason = "BaseImageNotReady"
)

// Host -
type Host struct {
	Hostname          string                                           `json:"hostname"`
	HostRef           string                                           `json:"hostRef"`
	DomainName        string                                           `json:"domainName"`
	DomainNameUniq    string                                           `json:"domainNameUniq"`
	IPAddress         string                                           `json:"ipAddress"`
	NetworkDataSecret string                                           `json:"networkDataSecret"`
	BaseImageName     string                                           `json:"baseImageName"`
	Labels            map[string]string                                `json:"labels"`
	NAD               map[string]networkv1.NetworkAttachmentDefinition `json:"nad"`
}

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackVMSet) IsReady() bool {
	cond := instance.Status.Conditions.InitCondition()

	return cond.Reason == VMSetCondReasonProvisioned || cond.Reason == VMSetCondReasonVirtualMachineCountZero
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osvmset;osvmsets;osvms
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack VMSet"
// +kubebuilder:printcolumn:name="Cores",type="integer",JSONPath=".spec.cores",description="Cores"
// +kubebuilder:printcolumn:name="RAM",type="integer",JSONPath=".spec.memory",description="RAM in GB"
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
func (instance OpenStackVMSet) GetHostnames() map[string]string {
	ret := make(map[string]string)
	for _, val := range instance.Status.VMHosts {
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
