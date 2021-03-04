/*
Copyright 2020 Red Hat

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
)

// Hash - struct to add hashes to status
type Hash struct {
	// Name of hash referencing the parameter
	Name string `json:"name,omitempty"`
	// Hash
	Hash string `json:"hash,omitempty"`
}

// Network - OSP network to create NodeNetworkConfigurationPolicy and NetworkAttachmentDefinition, or NodeSriovConfigurationPolicy
// TODO: that might change depending on our outcome of network config
type Network struct {
	Name                           string                                        `json:"name"`
	BridgeName                     string                                        `json:"bridgeName,omitempty"`
	NodeNetworkConfigurationPolicy nmstateapi.NodeNetworkConfigurationPolicySpec `json:"nodeNetworkConfigurationPolicy,omitempty"`
	NodeSriovConfigurationPolicy   NodeSriovConfigurationPolicy                  `json:"nodeSriovConfigurationPolicy,omitempty"`
}

// NodeSriovConfigurationPolicy - Node selector and desired state for SRIOV network
type NodeSriovConfigurationPolicy struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	DesiredState SriovState        `json:"desiredState,omitempty"`
}

// SriovState - SRIOV-specific configuration details for an OSP network
type SriovState struct {
	Port       string `json:"port"`
	RootDevice string `json:"rootDevice,omitempty"`
	// +kubebuilder:default=9000
	Mtu    uint32 `json:"mtu,omitempty"`
	NumVfs uint32 `json:"numVfs"`
	// +kubebuilder:default=vfio-pci
	DeviceType string `json:"deviceType,omitempty"`
}

// HostStatus represents the hostname and IP info for a specific VM
type HostStatus struct {
	Hostname    string            `json:"hostname"`
	IPAddresses map[string]string `json:"ipaddresses"`
}
