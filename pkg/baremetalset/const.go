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

package baremetalset

const (
	// AppLabel -
	AppLabel = "osp-baremetalset"

	// BaremetalHostRemovalAnnotation - Annotation key placed on BaremetalHost resources to target them for OpenStackBaremetalSet scale-down
	BaremetalHostRemovalAnnotation = "osp-director.openstack.org/baremetalset-delete-baremetalhost"

	// CloudInitUserDataSecretName - Naming template used for generating BaremetalHost cloudinit userdata secrets
	CloudInitUserDataSecretName = "%s-cloudinit-userdata-%s"
	// CloudInitNetworkDataSecretName - Naming template used for generating BaremetalHost cloudinit networkdata secrets
	CloudInitNetworkDataSecretName = "%s-cloudinit-networkdata-%s"

	// FinalizerName -
	FinalizerName = "baremetalsets.osp-director.openstack.org"

	// OwnerUIDLabelSelector - Ownership label for UID
	OwnerUIDLabelSelector = "baremetalset.osp-director.openstack.org/uid"
	// OwnerNameSpaceLabelSelector - Ownership label for Namespace
	OwnerNameSpaceLabelSelector = "baremetalset.osp-director.openstack.org/namespace"
	// OwnerNameLabelSelector - Ownership label for Name
	OwnerNameLabelSelector = "baremetalset.osp-director.openstack.org/name"
)
