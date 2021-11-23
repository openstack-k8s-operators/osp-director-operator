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

// IPSetState - the state of this openstack ipset
type IPSetState string

// IPSetReason - the reason of the condition for this openstack ipset
type IPSetReason string

const (
	//
	// condition types
	//
	// IPSetCondTypeInitializing - we are waiting for underlying OCP network resource(s) to appear
	IPSetCondTypeInitializing IPSetState = "Initializing"
	// IPSetCondTypeConfiguring - the underlying network resources are configuring the nodes
	IPSetCondTypeConfiguring IPSetState = "Configuring"
	// IPSetCondTypeConfigured - the nodes have been configured by the underlying network resources
	IPSetCondTypeConfigured IPSetState = "Configured"
	// IPSetCondTypeError - the network configuration hit a generic error
	IPSetCondTypeError IPSetState = "Error"
	// IPSetCondTypeWaiting - something else is causing the OpenStackIPSet to wait
	IPSetCondTypeWaiting IPSetState = "Waiting"

	//
	// condition reasones
	//
	// IPSetCondReasonRemovedIPs - the ipset removed IP reservations
	IPSetCondReasonRemovedIPs IPSetReason = "RemovedIPs"
	// IPSetCondReasonNewHosts - new host to register
	IPSetCondReasonNewHosts IPSetReason = "NewHosts"
	// IPSetCondReasonAddIPs - adding IPs for new hosts
	IPSetCondReasonAddIPs IPSetReason = "AddingIPs"
	// IPSetCondReasonNetNotFound - osnet not found
	IPSetCondReasonNetNotFound IPSetReason = "NetNotFound"
	// IPSetCondReasonNetsNotFound - osnets not found
	IPSetCondReasonNetsNotFound IPSetReason = "NetsNotFound"
	// IPSetCondReasonMACsNotFound - osmacs not found
	IPSetCondReasonMACsNotFound IPSetReason = "MACsNotFound"
	// IPSetCondReasonServiceNotFound - service not found
	IPSetCondReasonServiceNotFound IPSetReason = "ServiceNotFound"
	// IPSetCondReasonConfigMap - configmap reason
	IPSetCondReasonConfigMap IPSetReason = "ConfigMap"
	// IPSetCondReasonParseCIDR - parsing cidr
	IPSetCondReasonParseCIDR IPSetReason = "ParseCIDR"
	// IPSetCondReasonIPReservation - parsing cidr
	IPSetCondReasonIPReservation IPSetReason = "IPReservation"
	// IPSetCondReasonSuccessfullyConfigured - parsing cidr
	IPSetCondReasonSuccessfullyConfigured IPSetReason = "SuccessfullyConfigured"
)

// OpenStackIPSetSpec defines the desired state of OpenStackIPSet
type OpenStackIPSetSpec struct {

	// Networks the name(s) of the OpenStackNetworks used to generate IPs
	Networks []string `json:"networks"`

	// RoleName the name of the TripleO role this VM Spec is associated with. If it is a TripleO role, the name must match.
	RoleName string `json:"roleName"`

	// HostCount Host count
	HostCount int `json:"hostCount"`

	// HostNameRefs Hostname references for the HostCount with [hostname]->hostRef
	HostNameRefs map[string]string `json:"hostNameRefs"`

	// VIP flag to indicate ipset is a request for a VIP
	VIP bool `json:"vip"`

	// AddToPredictableIPs add/ignore ipset to add entries to Predictable IPs list
	AddToPredictableIPs bool `json:"addToPredictableIPs"`
}

// OpenStackIPSetStatus set of hosts with IP information
type OpenStackIPSetStatus struct {
	HostIPs  map[string]OpenStackIPHostsStatus `json:"hosts"`
	Networks map[string]NetworkStatus          `json:"networks"`

	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions ConditionList `json:"conditions,omitempty" optional:"true"`
}

// OpenStackIPHostsStatus per host IP set
type OpenStackIPHostsStatus struct {
	IPAddresses map[string]string `json:"ipaddresses"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osipset;osipsets;osips
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack IpSet"

// OpenStackIPSet represents a group of IP addresses for a specific deployment role within the OpenStack Overcloud
type OpenStackIPSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackIPSetSpec   `json:"spec,omitempty"`
	Status OpenStackIPSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackIPSetList contains a list of OpenStackIPSet
type OpenStackIPSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackIPSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackIPSet{}, &OpenStackIPSetList{})
}
