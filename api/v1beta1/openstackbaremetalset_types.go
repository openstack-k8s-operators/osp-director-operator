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
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HardwareReqType is used to enumerate the various hardware requests that can be made for the set
type HardwareReqType string

// OpenStackBaremetalSetSpec defines the desired state of OpenStackBaremetalSet
type OpenStackBaremetalSetSpec struct {
	// Count The number of baremetalhosts to attempt to aquire
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	Count int `json:"count,omitempty"`
	// Remote URL pointing to desired RHEL qcow2 image
	BaseImageURL string `json:"baseImageUrl,omitempty"`
	// ProvisionServerName Optional. If supplied will be used as the base Image for the baremetalset instead of baseImageURL.
	ProvisionServerName string `json:"provisionServerName,omitempty"`
	// Name of secret holding the stack-admin ssh keys
	DeploymentSSHSecret string `json:"deploymentSSHSecret"`
	// Interface to use for ctlplane network
	CtlplaneInterface string `json:"ctlplaneInterface"`
	// BmhLabelSelector allows for a sub-selection of BaremetalHosts based on arbitrary labels
	BmhLabelSelector map[string]string `json:"bmhLabelSelector,omitempty"`
	// Hardware requests for sub-selection of BaremetalHosts with certain hardware specs
	HardwareReqs HardwareReqs `json:"hardwareReqs,omitempty"`
	// Networks the name(s) of the OpenStackNetworks used to generate IPs
	Networks []string `json:"networks"`
	// RoleName the name of the TripleO role this OpenStackBaremetalSet is associated with. If it is a TripleO role, the name must match.
	RoleName string `json:"roleName"`
	// PasswordSecret the name of the secret used to optionally set the root pwd by adding
	// NodeRootPassword: <base64 enc pwd>
	// to the secret data
	PasswordSecret string `json:"passwordSecret,omitempty"`
	// BootstrapDNS - initial DNS nameserver values to set on the BaremetalHosts when they are provisioned.
	// Note that subsequent TripleO deployment will overwrite these values
	BootstrapDNS     []string `json:"bootstrapDns,omitempty"`
	DNSSearchDomains []string `json:"dnsSearchDomains,omitempty"`
	// GrowvolsArgs - arguments to the growvols command to expand logical volumes after provisioning
	// Note requires the command to exist on the base image
	GrowvolsArgs []string `json:"growvolsArgs,omitempty"`
}

// OpenStackBaremetalSetStatus defines the observed state of OpenStackBaremetalSet
type OpenStackBaremetalSetStatus struct {
	Conditions         shared.ConditionList                    `json:"conditions,omitempty" optional:"true"`
	ProvisioningStatus OpenStackBaremetalSetProvisioningStatus `json:"provisioningStatus,omitempty"`
	BaremetalHosts     map[string]HostStatus                   `json:"baremetalHosts,omitempty"`
}

// OpenStackBaremetalSetProvisioningStatus represents the overall provisioning state of all BaremetalHosts in
// the OpenStackBaremetalSet (with an optional explanatory message)
type OpenStackBaremetalSetProvisioningStatus struct {
	State      shared.ProvisioningState `json:"state,omitempty"`
	Reason     string                   `json:"reason,omitempty"`
	ReadyCount int                      `json:"readyCount,omitempty"`
}

// GetHostnames -
func (instance OpenStackBaremetalSet) GetHostnames() map[string]string {
	ret := make(map[string]string)
	for _, val := range instance.Status.BaremetalHosts {
		ret[val.Hostname] = val.HostRef
	}
	return ret
}

// HardwareReqs defines request hardware attributes for the BaremetalHost replicas
type HardwareReqs struct {
	CPUReqs  CPUReqs  `json:"cpuReqs,omitempty"`
	MemReqs  MemReqs  `json:"memReqs,omitempty"`
	DiskReqs DiskReqs `json:"diskReqs,omitempty"`
}

// CPUReqs defines specific CPU hardware requests
type CPUReqs struct {
	// Arch is a scalar (string) because it wouldn't make sense to give it an "exact-match" option
	// Can be either "x86_64" or "ppc64le" if included
	// +kubebuilder:validation:Enum=x86_64;ppc64le
	Arch     string      `json:"arch,omitempty"`
	CountReq CPUCountReq `json:"countReq,omitempty"`
	MhzReq   CPUMhzReq   `json:"mhzReq,omitempty"`
}

// CPUCountReq defines a specific hardware request for CPU core count
type CPUCountReq struct {
	// +kubebuilder:validation:Minimum=1
	Count int `json:"count,omitempty"`
	// If ExactMatch == false, actual count > Count will match
	ExactMatch bool `json:"exactMatch,omitempty"`
}

// CPUMhzReq defines a specific hardware request for CPU clock speed
type CPUMhzReq struct {
	// +kubebuilder:validation:Minimum=1
	Mhz int `json:"mhz,omitempty"`
	// If ExactMatch == false, actual mhz > Mhz will match
	ExactMatch bool `json:"exactMatch,omitempty"`
}

// MemReqs defines specific memory hardware requests
type MemReqs struct {
	GbReq MemGbReq `json:"gbReq,omitempty"`
}

// MemGbReq defines a specific hardware request for memory size
type MemGbReq struct {
	// +kubebuilder:validation:Minimum=1
	Gb int `json:"gb,omitempty"`
	// If ExactMatch == false, actual GB > Gb will match
	ExactMatch bool `json:"exactMatch,omitempty"`
}

// DiskReqs defines specific disk hardware requests
type DiskReqs struct {
	GbReq DiskGbReq `json:"gbReq,omitempty"`
	// SSD is scalar (bool) because it wouldn't make sense to give it an "exact-match" option
	SSDReq DiskSSDReq `json:"ssdReq,omitempty"`
}

// DiskGbReq defines a specific hardware request for disk size
type DiskGbReq struct {
	// +kubebuilder:validation:Minimum=1
	Gb int `json:"gb,omitempty"`
	// If ExactMatch == false, actual GB > Gb will match
	ExactMatch bool `json:"exactMatch,omitempty"`
}

// DiskSSDReq defines a specific hardware request for disk of type SSD (true) or rotational (false)
type DiskSSDReq struct {
	SSD bool `json:"ssd,omitempty"`
	// We only actually care about SSD flag if it is true or ExactMatch is set to true.
	// This second flag is necessary as SSD's bool zero-value (false) is indistinguishable
	// from it being explicitly set to false
	ExactMatch bool `json:"exactMatch,omitempty"`
}

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackBaremetalSet) IsReady() bool {
	return instance.Status.ProvisioningStatus.State == shared.ProvisioningState(shared.BaremetalSetCondTypeProvisioned) ||
		instance.Status.ProvisioningStatus.State == shared.ProvisioningState(shared.BaremetalSetCondTypeEmpty)
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osbmset;osbmsets;osbms
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack BaremetalSet"
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.count",description="Desired"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.provisioningStatus.readyCount",description="Ready"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.provisioningStatus.state",description="Status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.provisioningStatus.reason",description="Reason"

// OpenStackBaremetalSet represent a set of baremetal hosts for a specific role within the Overcloud deployment
type OpenStackBaremetalSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackBaremetalSetSpec   `json:"spec,omitempty"`
	Status OpenStackBaremetalSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackBaremetalSetList contains a list of OpenStackBaremetalSet
type OpenStackBaremetalSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackBaremetalSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackBaremetalSet{}, &OpenStackBaremetalSetList{})
}
