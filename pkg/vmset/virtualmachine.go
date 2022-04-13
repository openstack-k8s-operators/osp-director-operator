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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	virtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// DiskSetter - disk setter for virtv1.Disk
type DiskSetter func(*virtv1.Disk)

// DiskSetterMap -
type DiskSetterMap map[string]DiskSetter

// Disk - create additional Disk, ATM only virtio
func Disk(
	diskName string,
	bus string,
	serial string,
	dedicatedIOThread bool,
) DiskSetter {
	return func(disk *virtv1.Disk) {
		disk.Name = diskName
		if serial != "" {
			disk.Serial = serial
		}
		disk.DiskDevice.Disk = &virtv1.DiskTarget{}
		disk.DiskDevice.Disk.Bus = bus
		disk.DedicatedIOThread = &dedicatedIOThread
	}
}

// MergeVMDisks - merge new Disk into existing []virtv1.Disk
func MergeVMDisks(disks []virtv1.Disk, newDisks DiskSetterMap) []virtv1.Disk {
	for name, f := range newDisks {
		updated := false
		for i := 0; i < len(disks); i++ {
			if disks[i].Name == name {
				f(&disks[i])
				updated = true
				break
			}
		}

		if !updated {
			disks = append(disks, virtv1.Disk{Name: name})
			f(&disks[len(disks)-1])
		}
	}

	return disks
}

// VolumeSetter - volume setter for virtv1.Volume
type VolumeSetter func(*virtv1.Volume)

// VolumeSetterMap -
type VolumeSetterMap map[string]VolumeSetter

// VolumeSourceDataVolume - create additional VolumeSourceDataVolume
func VolumeSourceDataVolume(
	volumeName string,
	dataVolumeName string,
) VolumeSetter {
	return func(volume *virtv1.Volume) {
		volume.Name = volumeName
		volume.VolumeSource.DataVolume = &virtv1.DataVolumeSource{}
		volume.VolumeSource.DataVolume.Name = dataVolumeName
	}
}

// VolumeSourceCloudInitNoCloud - create additional VolumeSourceCloudInitNoCloud
func VolumeSourceCloudInitNoCloud(
	volumeName string,
	userDataSecretRefName string,
	networkDataSecretRefName string,
) VolumeSetter {
	return func(volume *virtv1.Volume) {
		volume.Name = volumeName
		volume.VolumeSource.CloudInitNoCloud = &virtv1.CloudInitNoCloudSource{}
		volume.VolumeSource.CloudInitNoCloud.UserDataSecretRef = &corev1.LocalObjectReference{
			Name: userDataSecretRefName,
		}
		volume.VolumeSource.CloudInitNoCloud.NetworkDataSecretRef = &corev1.LocalObjectReference{
			Name: networkDataSecretRefName,
		}
	}
}

// VolumeSourceSecret - create additional VolumeSourceSecret
func VolumeSourceSecret(
	volumeName string,
	secretName string,
) VolumeSetter {
	return func(volume *virtv1.Volume) {
		volume.Name = volumeName
		volume.VolumeSource.Secret = &virtv1.SecretVolumeSource{}
		volume.VolumeSource.Secret.SecretName = secretName
	}
}

// MergeVMVolumes - merge new Volume into existing []virtv1.Volume
func MergeVMVolumes(volumes []virtv1.Volume, newVolumes VolumeSetterMap) []virtv1.Volume {
	for name, f := range newVolumes {
		updated := false
		for i := 0; i < len(volumes); i++ {
			if volumes[i].Name == name {
				f(&volumes[i])
				updated = true
				break
			}
		}

		if !updated {
			volumes = append(volumes, virtv1.Volume{Name: name})
			f(&volumes[len(volumes)-1])
		}
	}

	return volumes
}

// DataVolumeSetter - volume setter for virtv1.DataVolumeTemplateSpec
type DataVolumeSetter func(*virtv1.DataVolumeTemplateSpec)

// DataVolumeSetterMap -
type DataVolumeSetterMap map[string]DataVolumeSetter

// DataVolume - create additional DataVolume
func DataVolume(
	dataVolumeName string,
	namespace string,
	pvAccessMode string,
	diskSize uint32,
	volumeMode string,
	storageClass string,
	baseImageName string,
) DataVolumeSetter {
	volMode := corev1.PersistentVolumeMode(volumeMode)

	return func(dataVolume *virtv1.DataVolumeTemplateSpec) {
		dataVolume.ObjectMeta.Name = dataVolumeName
		dataVolume.ObjectMeta.Namespace = namespace

		dataVolume.Spec.PVC = &corev1.PersistentVolumeClaimSpec{}
		dataVolume.Spec.PVC.AccessModes = []corev1.PersistentVolumeAccessMode{
			corev1.PersistentVolumeAccessMode(pvAccessMode),
		}

		dataVolume.Spec.PVC.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", diskSize)),
			},
		}
		dataVolume.Spec.PVC.VolumeMode = &volMode
		if baseImageName != "" {
			dataVolume.Spec.Source = &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      baseImageName,
					Namespace: namespace,
				},
			}
		} else {
			dataVolume.Spec.Source = &cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			}
		}
	}
}

// MergeVMDataVolumes - merge new DataVolume into existing []virtv1.DataVolumeTemplateSpec
func MergeVMDataVolumes(dataVolumes []virtv1.DataVolumeTemplateSpec, newDataVolumes DataVolumeSetterMap, namespace string) []virtv1.DataVolumeTemplateSpec {
	for name, f := range newDataVolumes {
		updated := false
		for i := 0; i < len(dataVolumes); i++ {
			if dataVolumes[i].GetObjectMeta().GetName() == name {
				f(&dataVolumes[i])
				updated = true
				break
			}
		}

		if !updated {
			dataVolumes = append(dataVolumes, virtv1.DataVolumeTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				}})
			f(&dataVolumes[len(dataVolumes)-1])
		}
	}

	return dataVolumes
}

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
