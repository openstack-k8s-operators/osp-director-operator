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

package shared

// APIAction - typedef to enumerate API verbs
type APIAction string

const (
	// APIActionCreate - "create" API verb
	APIActionCreate APIAction = "create"
	// APIActionUpdate - "update" API verb
	APIActionUpdate APIAction = "update"
	// APIActionDelete - "delete" API verb
	APIActionDelete APIAction = "delete"
)

// Hash - struct to add hashes to status
type Hash struct {
	// Name of hash referencing the parameter
	Name string `json:"name,omitempty"`
	// Hash
	Hash string `json:"hash,omitempty"`
}

// ProvisioningState - the overall state of all VMs in this OpenStackVmSet
type ProvisioningState string

const (
	// HostRefInitState - intial HostRef state of a new node which has not yet assigned
	HostRefInitState string = "unassigned"
)
