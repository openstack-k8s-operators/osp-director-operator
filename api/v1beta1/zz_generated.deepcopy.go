// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	k8s_cni_cncf_iov1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/nmstate/kubernetes-nmstate/api/v1alpha1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CPUCountReq) DeepCopyInto(out *CPUCountReq) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CPUCountReq.
func (in *CPUCountReq) DeepCopy() *CPUCountReq {
	if in == nil {
		return nil
	}
	out := new(CPUCountReq)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CPUMhzReq) DeepCopyInto(out *CPUMhzReq) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CPUMhzReq.
func (in *CPUMhzReq) DeepCopy() *CPUMhzReq {
	if in == nil {
		return nil
	}
	out := new(CPUMhzReq)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CPUReqs) DeepCopyInto(out *CPUReqs) {
	*out = *in
	out.CountReq = in.CountReq
	out.MhzReq = in.MhzReq
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CPUReqs.
func (in *CPUReqs) DeepCopy() *CPUReqs {
	if in == nil {
		return nil
	}
	out := new(CPUReqs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiskGbReq) DeepCopyInto(out *DiskGbReq) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiskGbReq.
func (in *DiskGbReq) DeepCopy() *DiskGbReq {
	if in == nil {
		return nil
	}
	out := new(DiskGbReq)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiskReqs) DeepCopyInto(out *DiskReqs) {
	*out = *in
	out.GbReq = in.GbReq
	out.SSDReq = in.SSDReq
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiskReqs.
func (in *DiskReqs) DeepCopy() *DiskReqs {
	if in == nil {
		return nil
	}
	out := new(DiskReqs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiskSSDReq) DeepCopyInto(out *DiskSSDReq) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiskSSDReq.
func (in *DiskSSDReq) DeepCopy() *DiskSSDReq {
	if in == nil {
		return nil
	}
	out := new(DiskSSDReq)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HardwareReqs) DeepCopyInto(out *HardwareReqs) {
	*out = *in
	out.CPUReqs = in.CPUReqs
	out.MemReqs = in.MemReqs
	out.DiskReqs = in.DiskReqs
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HardwareReqs.
func (in *HardwareReqs) DeepCopy() *HardwareReqs {
	if in == nil {
		return nil
	}
	out := new(HardwareReqs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Hash) DeepCopyInto(out *Hash) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Hash.
func (in *Hash) DeepCopy() *Hash {
	if in == nil {
		return nil
	}
	out := new(Hash)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Host) DeepCopyInto(out *Host) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NNCP != nil {
		in, out := &in.NNCP, &out.NNCP
		*out = make(map[string]v1alpha1.NodeNetworkConfigurationPolicy, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.NAD != nil {
		in, out := &in.NAD, &out.NAD
		*out = make(map[string]k8s_cni_cncf_iov1.NetworkAttachmentDefinition, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Host.
func (in *Host) DeepCopy() *Host {
	if in == nil {
		return nil
	}
	out := new(Host)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HostStatus) DeepCopyInto(out *HostStatus) {
	*out = *in
	if in.IPAddresses != nil {
		in, out := &in.IPAddresses, &out.IPAddresses
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HostStatus.
func (in *HostStatus) DeepCopy() *HostStatus {
	if in == nil {
		return nil
	}
	out := new(HostStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IPReservation) DeepCopyInto(out *IPReservation) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IPReservation.
func (in *IPReservation) DeepCopy() *IPReservation {
	if in == nil {
		return nil
	}
	out := new(IPReservation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MemGbReq) DeepCopyInto(out *MemGbReq) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MemGbReq.
func (in *MemGbReq) DeepCopy() *MemGbReq {
	if in == nil {
		return nil
	}
	out := new(MemGbReq)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MemReqs) DeepCopyInto(out *MemReqs) {
	*out = *in
	out.GbReq = in.GbReq
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MemReqs.
func (in *MemReqs) DeepCopy() *MemReqs {
	if in == nil {
		return nil
	}
	out := new(MemReqs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Network) DeepCopyInto(out *Network) {
	*out = *in
	in.NodeNetworkConfigurationPolicy.DeepCopyInto(&out.NodeNetworkConfigurationPolicy)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Network.
func (in *Network) DeepCopy() *Network {
	if in == nil {
		return nil
	}
	out := new(Network)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkConfiguration) DeepCopyInto(out *NetworkConfiguration) {
	*out = *in
	in.NodeNetworkConfigurationPolicy.DeepCopyInto(&out.NodeNetworkConfigurationPolicy)
	in.NodeSriovConfigurationPolicy.DeepCopyInto(&out.NodeSriovConfigurationPolicy)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkConfiguration.
func (in *NetworkConfiguration) DeepCopy() *NetworkConfiguration {
	if in == nil {
		return nil
	}
	out := new(NetworkConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeSriovConfigurationPolicy) DeepCopyInto(out *NodeSriovConfigurationPolicy) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.DesiredState = in.DesiredState
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeSriovConfigurationPolicy.
func (in *NodeSriovConfigurationPolicy) DeepCopy() *NodeSriovConfigurationPolicy {
	if in == nil {
		return nil
	}
	out := new(NodeSriovConfigurationPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackBaremetalHostStatus) DeepCopyInto(out *OpenStackBaremetalHostStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackBaremetalHostStatus.
func (in *OpenStackBaremetalHostStatus) DeepCopy() *OpenStackBaremetalHostStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackBaremetalHostStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackBaremetalSet) DeepCopyInto(out *OpenStackBaremetalSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackBaremetalSet.
func (in *OpenStackBaremetalSet) DeepCopy() *OpenStackBaremetalSet {
	if in == nil {
		return nil
	}
	out := new(OpenStackBaremetalSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackBaremetalSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackBaremetalSetList) DeepCopyInto(out *OpenStackBaremetalSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenStackBaremetalSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackBaremetalSetList.
func (in *OpenStackBaremetalSetList) DeepCopy() *OpenStackBaremetalSetList {
	if in == nil {
		return nil
	}
	out := new(OpenStackBaremetalSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackBaremetalSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackBaremetalSetSpec) DeepCopyInto(out *OpenStackBaremetalSetSpec) {
	*out = *in
	if in.BmhLabelSelector != nil {
		in, out := &in.BmhLabelSelector, &out.BmhLabelSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.HardwareReqs = in.HardwareReqs
	if in.Networks != nil {
		in, out := &in.Networks, &out.Networks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackBaremetalSetSpec.
func (in *OpenStackBaremetalSetSpec) DeepCopy() *OpenStackBaremetalSetSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackBaremetalSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackBaremetalSetStatus) DeepCopyInto(out *OpenStackBaremetalSetStatus) {
	*out = *in
	if in.BaremetalHosts != nil {
		in, out := &in.BaremetalHosts, &out.BaremetalHosts
		*out = make(map[string]OpenStackBaremetalHostStatus, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackBaremetalSetStatus.
func (in *OpenStackBaremetalSetStatus) DeepCopy() *OpenStackBaremetalSetStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackBaremetalSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackClient) DeepCopyInto(out *OpenStackClient) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackClient.
func (in *OpenStackClient) DeepCopy() *OpenStackClient {
	if in == nil {
		return nil
	}
	out := new(OpenStackClient)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackClient) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackClientList) DeepCopyInto(out *OpenStackClientList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenStackClient, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackClientList.
func (in *OpenStackClientList) DeepCopy() *OpenStackClientList {
	if in == nil {
		return nil
	}
	out := new(OpenStackClientList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackClientList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackClientSpec) DeepCopyInto(out *OpenStackClientSpec) {
	*out = *in
	if in.HostAliases != nil {
		in, out := &in.HostAliases, &out.HostAliases
		*out = make([]v1.HostAlias, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Networks != nil {
		in, out := &in.Networks, &out.Networks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackClientSpec.
func (in *OpenStackClientSpec) DeepCopy() *OpenStackClientSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackClientSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackClientStatus) DeepCopyInto(out *OpenStackClientStatus) {
	*out = *in
	if in.OpenStackClientNetStatus != nil {
		in, out := &in.OpenStackClientNetStatus, &out.OpenStackClientNetStatus
		*out = make(map[string]HostStatus, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackClientStatus.
func (in *OpenStackClientStatus) DeepCopy() *OpenStackClientStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackClientStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackControlPlane) DeepCopyInto(out *OpenStackControlPlane) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackControlPlane.
func (in *OpenStackControlPlane) DeepCopy() *OpenStackControlPlane {
	if in == nil {
		return nil
	}
	out := new(OpenStackControlPlane)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackControlPlane) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackControlPlaneList) DeepCopyInto(out *OpenStackControlPlaneList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenStackControlPlane, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackControlPlaneList.
func (in *OpenStackControlPlaneList) DeepCopy() *OpenStackControlPlaneList {
	if in == nil {
		return nil
	}
	out := new(OpenStackControlPlaneList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackControlPlaneList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackControlPlaneSpec) DeepCopyInto(out *OpenStackControlPlaneSpec) {
	*out = *in
	if in.VirtualMachineRoles != nil {
		in, out := &in.VirtualMachineRoles, &out.VirtualMachineRoles
		*out = make(map[string]OpenStackVirtualMachineRoleSpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.OpenStackClientNetworks != nil {
		in, out := &in.OpenStackClientNetworks, &out.OpenStackClientNetworks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackControlPlaneSpec.
func (in *OpenStackControlPlaneSpec) DeepCopy() *OpenStackControlPlaneSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackControlPlaneSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackControlPlaneStatus) DeepCopyInto(out *OpenStackControlPlaneStatus) {
	*out = *in
	if in.VIPStatus != nil {
		in, out := &in.VIPStatus, &out.VIPStatus
		*out = make(map[string]HostStatus, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackControlPlaneStatus.
func (in *OpenStackControlPlaneStatus) DeepCopy() *OpenStackControlPlaneStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackControlPlaneStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackIPHostsStatus) DeepCopyInto(out *OpenStackIPHostsStatus) {
	*out = *in
	if in.IPAddresses != nil {
		in, out := &in.IPAddresses, &out.IPAddresses
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackIPHostsStatus.
func (in *OpenStackIPHostsStatus) DeepCopy() *OpenStackIPHostsStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackIPHostsStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackIPSet) DeepCopyInto(out *OpenStackIPSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackIPSet.
func (in *OpenStackIPSet) DeepCopy() *OpenStackIPSet {
	if in == nil {
		return nil
	}
	out := new(OpenStackIPSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackIPSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackIPSetList) DeepCopyInto(out *OpenStackIPSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenStackIPSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackIPSetList.
func (in *OpenStackIPSetList) DeepCopy() *OpenStackIPSetList {
	if in == nil {
		return nil
	}
	out := new(OpenStackIPSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackIPSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackIPSetSpec) DeepCopyInto(out *OpenStackIPSetSpec) {
	*out = *in
	if in.Networks != nil {
		in, out := &in.Networks, &out.Networks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackIPSetSpec.
func (in *OpenStackIPSetSpec) DeepCopy() *OpenStackIPSetSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackIPSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackIPSetStatus) DeepCopyInto(out *OpenStackIPSetStatus) {
	*out = *in
	if in.HostIPs != nil {
		in, out := &in.HostIPs, &out.HostIPs
		*out = make(map[string]OpenStackIPHostsStatus, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.Networks != nil {
		in, out := &in.Networks, &out.Networks
		*out = make(map[string]OpenStackNetSpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackIPSetStatus.
func (in *OpenStackIPSetStatus) DeepCopy() *OpenStackIPSetStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackIPSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackNet) DeepCopyInto(out *OpenStackNet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackNet.
func (in *OpenStackNet) DeepCopy() *OpenStackNet {
	if in == nil {
		return nil
	}
	out := new(OpenStackNet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackNet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackNetList) DeepCopyInto(out *OpenStackNetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenStackNet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackNetList.
func (in *OpenStackNetList) DeepCopy() *OpenStackNetList {
	if in == nil {
		return nil
	}
	out := new(OpenStackNetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackNetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackNetSpec) DeepCopyInto(out *OpenStackNetSpec) {
	*out = *in
	in.AttachConfiguration.DeepCopyInto(&out.AttachConfiguration)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackNetSpec.
func (in *OpenStackNetSpec) DeepCopy() *OpenStackNetSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackNetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackNetStatus) DeepCopyInto(out *OpenStackNetStatus) {
	*out = *in
	if in.Reservations != nil {
		in, out := &in.Reservations, &out.Reservations
		*out = make([]IPReservation, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackNetStatus.
func (in *OpenStackNetStatus) DeepCopy() *OpenStackNetStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackNetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackPlaybookGenerator) DeepCopyInto(out *OpenStackPlaybookGenerator) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackPlaybookGenerator.
func (in *OpenStackPlaybookGenerator) DeepCopy() *OpenStackPlaybookGenerator {
	if in == nil {
		return nil
	}
	out := new(OpenStackPlaybookGenerator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackPlaybookGenerator) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackPlaybookGeneratorList) DeepCopyInto(out *OpenStackPlaybookGeneratorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenStackPlaybookGenerator, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackPlaybookGeneratorList.
func (in *OpenStackPlaybookGeneratorList) DeepCopy() *OpenStackPlaybookGeneratorList {
	if in == nil {
		return nil
	}
	out := new(OpenStackPlaybookGeneratorList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackPlaybookGeneratorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackPlaybookGeneratorSpec) DeepCopyInto(out *OpenStackPlaybookGeneratorSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackPlaybookGeneratorSpec.
func (in *OpenStackPlaybookGeneratorSpec) DeepCopy() *OpenStackPlaybookGeneratorSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackPlaybookGeneratorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackPlaybookGeneratorStatus) DeepCopyInto(out *OpenStackPlaybookGeneratorStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackPlaybookGeneratorStatus.
func (in *OpenStackPlaybookGeneratorStatus) DeepCopy() *OpenStackPlaybookGeneratorStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackPlaybookGeneratorStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackProvisionServer) DeepCopyInto(out *OpenStackProvisionServer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackProvisionServer.
func (in *OpenStackProvisionServer) DeepCopy() *OpenStackProvisionServer {
	if in == nil {
		return nil
	}
	out := new(OpenStackProvisionServer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackProvisionServer) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackProvisionServerList) DeepCopyInto(out *OpenStackProvisionServerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenStackProvisionServer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackProvisionServerList.
func (in *OpenStackProvisionServerList) DeepCopy() *OpenStackProvisionServerList {
	if in == nil {
		return nil
	}
	out := new(OpenStackProvisionServerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackProvisionServerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackProvisionServerSpec) DeepCopyInto(out *OpenStackProvisionServerSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackProvisionServerSpec.
func (in *OpenStackProvisionServerSpec) DeepCopy() *OpenStackProvisionServerSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackProvisionServerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackProvisionServerStatus) DeepCopyInto(out *OpenStackProvisionServerStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackProvisionServerStatus.
func (in *OpenStackProvisionServerStatus) DeepCopy() *OpenStackProvisionServerStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackProvisionServerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackVMSet) DeepCopyInto(out *OpenStackVMSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackVMSet.
func (in *OpenStackVMSet) DeepCopy() *OpenStackVMSet {
	if in == nil {
		return nil
	}
	out := new(OpenStackVMSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackVMSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackVMSetList) DeepCopyInto(out *OpenStackVMSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenStackVMSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackVMSetList.
func (in *OpenStackVMSetList) DeepCopy() *OpenStackVMSetList {
	if in == nil {
		return nil
	}
	out := new(OpenStackVMSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenStackVMSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackVMSetSpec) DeepCopyInto(out *OpenStackVMSetSpec) {
	*out = *in
	if in.Networks != nil {
		in, out := &in.Networks, &out.Networks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackVMSetSpec.
func (in *OpenStackVMSetSpec) DeepCopy() *OpenStackVMSetSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackVMSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackVMSetStatus) DeepCopyInto(out *OpenStackVMSetStatus) {
	*out = *in
	if in.VMpods != nil {
		in, out := &in.VMpods, &out.VMpods
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.VMHosts != nil {
		in, out := &in.VMHosts, &out.VMHosts
		*out = make(map[string]HostStatus, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackVMSetStatus.
func (in *OpenStackVMSetStatus) DeepCopy() *OpenStackVMSetStatus {
	if in == nil {
		return nil
	}
	out := new(OpenStackVMSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenStackVirtualMachineRoleSpec) DeepCopyInto(out *OpenStackVirtualMachineRoleSpec) {
	*out = *in
	if in.Networks != nil {
		in, out := &in.Networks, &out.Networks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenStackVirtualMachineRoleSpec.
func (in *OpenStackVirtualMachineRoleSpec) DeepCopy() *OpenStackVirtualMachineRoleSpec {
	if in == nil {
		return nil
	}
	out := new(OpenStackVirtualMachineRoleSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SriovState) DeepCopyInto(out *SriovState) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SriovState.
func (in *SriovState) DeepCopy() *SriovState {
	if in == nil {
		return nil
	}
	out := new(SriovState)
	in.DeepCopyInto(out)
	return out
}
