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
	nmstateapi "github.com/nmstate/kubernetes-nmstate/api/shared"
	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AttachType -
type AttachType string

const (
	// AttachTypeBridge -
	AttachTypeBridge AttachType = "bridge"
	// AttachTypeSriov -
	AttachTypeSriov AttachType = "sriov"
)

// NodeConfigurationPolicy - policy definition to create NodeNetworkConfigurationPolicy or NodeSriovConfigurationPolicy
type NodeConfigurationPolicy struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	NodeNetworkConfigurationPolicy nmstateapi.NodeNetworkConfigurationPolicySpec `json:"nodeNetworkConfigurationPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	NodeSriovConfigurationPolicy NodeSriovConfigurationPolicy `json:"nodeSriovConfigurationPolicy,omitempty"`
}

// NodeSriovConfigurationPolicy - Node selector and desired state for SRIOV network
type NodeSriovConfigurationPolicy struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	DesiredState SriovState        `json:"desiredState,omitempty"`
}

// SriovState - SRIOV-specific configuration details for an OSP network
type SriovState struct {
	// +kubebuilder:default=vfio-pci
	DeviceType string `json:"deviceType,omitempty"`
	// +kubebuilder:default=9000
	Mtu        uint32 `json:"mtu,omitempty"`
	NumVfs     uint32 `json:"numVfs"`
	Port       string `json:"port"`
	RootDevice string `json:"rootDevice,omitempty"`
	// +kubebuilder:validation:Enum={"on","off"}
	// +kubebuilder:default=on
	SpoofCheck string `json:"spoofCheck,omitempty"`
	// +kubebuilder:validation:Enum={"on","off"}
	// +kubebuilder:default=off
	Trust string `json:"trust,omitempty"`
}

// OpenStackNetAttachmentSpec defines the desired state of OpenStackNetAttachment
type OpenStackNetAttachmentSpec struct {

	// +kubebuilder:validation:Required
	// AttachConfiguration used for NodeNetworkConfigurationPolicy or NodeSriovConfigurationPolicy
	AttachConfiguration NodeConfigurationPolicy `json:"attachConfiguration"`
}

// OpenStackNetAttachmentStatus defines the observed state of OpenStackNetAttachment
type OpenStackNetAttachmentStatus struct {
	// CurrentState - the overall state of the network attachment
	CurrentState shared.ConditionType `json:"currentState"`

	// TODO: It would be simpler, perhaps, to just have Conditions and get rid of CurrentState,
	// but we are using the same approach in other CRDs for now anyhow
	// Conditions - conditions to display in the OpenShift GUI, which reflect CurrentState
	Conditions shared.ConditionList `json:"conditions,omitempty" optional:"true"`

	// AttachType of the OpenStackNetAttachment
	AttachType AttachType `json:"attachType"`

	// BridgeName of the OpenStackNetAttachment
	BridgeName string `json:"bridgeName"`
}

// IsReady - Is this resource in its fully-configured (quiesced) state?
func (instance *OpenStackNetAttachment) IsReady() bool {
	return instance.Status.CurrentState == shared.NetAttachConfigured
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=osnetattachment;osnetsattachment;osnetatt;osnetsatt
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack NetAttachment"
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.currentState`,description="Status"

// OpenStackNetAttachment are used to describe the node network configuration policy and configured as part of OSNetConfig resources
type OpenStackNetAttachment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackNetAttachmentSpec   `json:"spec,omitempty"`
	Status OpenStackNetAttachmentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackNetAttachmentList contains a list of OpenStackNetAttachment
type OpenStackNetAttachmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackNetAttachment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackNetAttachment{}, &OpenStackNetAttachmentList{})
}
