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

package controlplane

const (
	// AppLabel -
	AppLabel = "osp-controlplane"

	// FinalizerName -
	FinalizerName = "controlplane.osp-director.openstack.org"

	// Count - controlplane count is atm fixed to 1
	Count = 1

	// Role - controlplane has not tripleo role, set it as const
	Role = "ControlPlane"
)
