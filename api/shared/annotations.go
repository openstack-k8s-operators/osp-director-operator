// Package shared provides shared constants and utilities used across the OSP Director Operator API.
package shared

const (
	// HostRemovalAnnotation - Annotation key placed on VM or BMH resources to target them for scale-down
	HostRemovalAnnotation = "osp-director.openstack.org/delete-host"
)
