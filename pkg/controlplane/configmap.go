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

package controlplane

import (
	"sort"

	virtv1 "kubevirt.io/api/core/v1"
)

// FencingConfig - stores the unique fencing configuration for a particular Kubevirt VM
type FencingConfig struct {
	HostMac   string
	Namespace string
	VMName    string
}

// CreateFencingConfigMapParams - creates a map of parameters for fencing data needed in tripleo-deploy-config config map
func CreateFencingConfigMapParams(virtualMachineInstanceLists []*virtv1.VirtualMachineInstanceList) (map[string]interface{}, error) {
	templateParameters := make(map[string]interface{})

	fencingConfigs := []FencingConfig{}

	for _, virtualMachineInstanceList := range virtualMachineInstanceLists {
		for _, virtualMachineInstance := range virtualMachineInstanceList.Items {
			fencingConfig := FencingConfig{
				// We just need the MAC of any interface on the VM
				HostMac:   virtualMachineInstance.Status.Interfaces[0].MAC,
				Namespace: virtualMachineInstance.Namespace,
				VMName:    virtualMachineInstance.Name,
			}

			fencingConfigs = append(fencingConfigs, fencingConfig)
		}
	}

	// Sort the config so that any generated hashes remain constant (we don't know the order
	// of the virtualMachineInstanceList)

	sort.Slice(fencingConfigs, func(i, j int) bool {
		return fencingConfigs[i].VMName < fencingConfigs[j].VMName
	})

	templateParameters["FencingConfigs"] = fencingConfigs

	return templateParameters, nil
}
