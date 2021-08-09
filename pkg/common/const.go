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

package common

const (
	// OwnerLabel -
	OwnerLabel = "osp-director"
	// GroupLabel -
	GroupLabel = OwnerLabel + ".openstack.org"
	// OwnerUIDLabelSelector -
	OwnerUIDLabelSelector = GroupLabel + "/uid"
	// OwnerNameSpaceLabelSelector -
	OwnerNameSpaceLabelSelector = GroupLabel + "/namespace"
	// OwnerNameLabelSelector -
	OwnerNameLabelSelector = GroupLabel + "/name"
	// OwnerControllerNameLabelSelector -
	OwnerControllerNameLabelSelector = GroupLabel + "/controller"

	// OSPHostnameLabelSelector - Tripleo Hostname
	OSPHostnameLabelSelector = GroupLabel + "/osphostname"
	// HostRemovalAnnotation - Annotation key placed on VM or BMH resources to target them for scale-down
	HostRemovalAnnotation = GroupLabel + "/delete-host"

	// MustGatherSecret - Label placed on secrets that are safe to collect with must-gather
	MustGatherSecret = GroupLabel + "/must-gather-secret"

	// FinalizerName -
	FinalizerName = GroupLabel
)
