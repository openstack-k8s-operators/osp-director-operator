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

// OpenStackIPSetSpec defines the desired state of OpenStackIPSet
type OpenStackIPSetSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	// HostCount Host count
	HostCount int `json:"hostCount"`

	// +kubebuilder:validation:Optional
	// DeletedHosts List of hosts which got deleted
	DeletedHosts []string `json:"deletedHosts"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// VIP flag to indicate ipset is a request for a VIP
	VIP bool `json:"vip"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// ServiceVIP flag to indicate ipset is a request for a VirtualFixedIPs entry
	ServiceVIP bool `json:"serviceVIP"`

	// +kubebuilder:validation:Required
	// RoleName the name of the role this VM Spec is associated with. If it is a TripleO role, the name must match.
	RoleName string `json:"roleName"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={ctlplane}
	// Networks the name(s) of the OpenStackNetworks used to generate IPs
	Networks []string `json:"networks"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// AddToPredictableIPs add/ignore IPs to be added to Predictable IPs list
	AddToPredictableIPs bool `json:"addToPredictableIPs"`
}

// OpenStackIPSetStatus defines the observed state of OpenStackIPSet
type OpenStackIPSetStatus struct {
	Hosts    map[string]HostStatus `json:"hosts,omitempty"`
	Reserved int                   `json:"reserved,omitempty"`
	Networks int                   `json:"networks,omitempty"`

	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=osipset;osipsets;osips
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack IPSet"
//+kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.hostCount",description="Desired"
//+kubebuilder:printcolumn:name="Reserved",type="integer",JSONPath=".status.reserved",description="Reserved"
//+kubebuilder:printcolumn:name="Networks",type="integer",JSONPath=".status.networks",description="Networks"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.status==\"True\")].type",description="Status"
//+kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.status==\"True\")].message",description="Reason"

// OpenStackIPSet a resource to request a set of IPs for the given networks
type OpenStackIPSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackIPSetSpec   `json:"spec,omitempty"`
	Status OpenStackIPSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackIPSetList contains a list of OpenStackIPSet
type OpenStackIPSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackIPSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackIPSet{}, &OpenStackIPSetList{})
}

// GetHostnames -
func (instance OpenStackIPSet) GetHostnames() map[string]string {
	ret := make(map[string]string)
	for _, val := range instance.Status.Hosts {
		ret[val.Hostname] = val.HostRef
	}
	return ret
}
