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

package vmset

import (
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	virtv1 "kubevirt.io/api/core/v1"
)

// NetSetter - net setter for virtv1.Network
type NetSetter func(*virtv1.Network)

// NetSetterMap -
type NetSetterMap map[string]NetSetter

// Network - create additional multus virtv1.Network
func Network(networkName string, attachType ospdirectorv1beta1.AttachType) NetSetter {
	return func(net *virtv1.Network) {
		net.Name = networkName
		actualNetworkName := networkName

		if attachType == ospdirectorv1beta1.AttachTypeSriov {
			// SRIOV networks use "<network>-sriov-network" format for the actual network name
			// FIXME?: We could just change the OpenStackNet controller so that it uses the instance
			//         name without the "-sriov-network" suffix, but it is currently doing this
			//         because all SRIOV resources are created in a shared namespace
			//         (openshift-sriov-network-operator), so we're trying to avoid possible naming
			//         conflicts by appending this suffix
			actualNetworkName = fmt.Sprintf("%s-sriov-network", networkName)
		}

		net.NetworkSource.Multus = &virtv1.MultusNetwork{
			NetworkName: actualNetworkName,
		}
	}
}

// MergeVMNetworks - merge new Network into existing []virtv1.Network
func MergeVMNetworks(networks []virtv1.Network, newNetworks NetSetterMap) []virtv1.Network {
	for name, f := range newNetworks {
		updated := false
		for i := 0; i < len(networks); i++ {
			if networks[i].Name == name {
				f(&networks[i])
				updated = true
				break
			}
		}

		if !updated {
			networks = append(networks, virtv1.Network{Name: name})
			f(&networks[len(networks)-1])
		}
	}

	return networks
}

// InterfaceSetter - interface setter for virtv1.Interface
type InterfaceSetter func(*virtv1.Interface)

// InterfaceSetterMap -
type InterfaceSetterMap map[string]InterfaceSetter

// Interface - create additional Intercface, ATM only bridge
func Interface(ifName string, attachType ospdirectorv1beta1.AttachType) InterfaceSetter {
	return func(iface *virtv1.Interface) {
		iface.Name = ifName

		model := "virtio"

		// We currently support SRIOV and bridge interfaces, with anything other than "sriov" indicating a bridge
		switch attachType {
		case ospdirectorv1beta1.AttachTypeBridge:
			iface.InterfaceBindingMethod = virtv1.InterfaceBindingMethod{
				Bridge: &virtv1.InterfaceBridge{},
			}
		case ospdirectorv1beta1.AttachTypeSriov:
			model = ""
			iface.InterfaceBindingMethod = virtv1.InterfaceBindingMethod{
				SRIOV: &virtv1.InterfaceSRIOV{},
			}
		}

		iface.Model = model
	}
}

// MergeVMInterfaces - merge new Interface into existing []virtv1.Interface
func MergeVMInterfaces(interfaces []virtv1.Interface, newInterfaces InterfaceSetterMap) []virtv1.Interface {
	for name, f := range newInterfaces {
		updated := false
		for i := 0; i < len(interfaces); i++ {
			if interfaces[i].Name == name {
				f(&interfaces[i])
				updated = true
				break
			}
		}

		if !updated {
			interfaces = append(interfaces, virtv1.Interface{Name: name})
			f(&interfaces[len(interfaces)-1])
		}
	}

	return interfaces
}
