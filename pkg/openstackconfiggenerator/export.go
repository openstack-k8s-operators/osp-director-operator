/*
Copyright 2022 Red Hat

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

package openstackconfiggenerator

import (
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/orchestration/v1/stacks"
)

func exportNames() map[string]string {
	return map[string]string{
		"EndpointMap":  "EndpointMapOverride",
		"HostsEntry":   "ExtraHostFileEntries",
		"GlobalConfig": "GlobalConfigExtraMapData",
	}
}

// CtlplaneExports -
func CtlplaneExports(heatServiceName string, log logr.Logger) (string, error) {

	provider, err := openstack.NewClient("http://" + heatServiceName + ":8004/")
	if err != nil {
		log.Error(err, "Failed to create new HeatClient provider.")
		return "", err
	}
	// override the EndpointLocator as we are using noauth without a real Catalog
	provider.EndpointLocator = func(opts gophercloud.EndpointOpts) (string, error) {
		return "http://" + heatServiceName + ":8004/v1/admin/", nil
	}
	client, err := openstack.NewOrchestrationV1(provider, gophercloud.EndpointOpts{Region: "regionOne"})
	if err != nil {
		log.Error(err, "Failed to create new HeatClient.")
		return "", err
	}

	overcloudStack, err := stacks.Find(client, "overcloud").Extract()
	if err != nil {
		log.Error(err, "Failed to find overcloud stack.")
		return "", err
	}

	parameters := make(map[string]interface{})
	// array of {output_key, output_value, description}
	for _, m := range overcloudStack.Outputs {
		outputKey := m["output_key"].(string)
		if val, ok := exportNames()[outputKey]; ok {
			parameters[val] = m["output_value"]
		}
	}

	parameterDefaults := make(map[string]interface{})
	parameterDefaults["parameter_defaults"] = parameters
	outputValueData, err := yaml.Marshal(parameterDefaults)
	if err != nil {
		log.Error(err, "Failed to marshal stack outputs to yaml.")
		return "", err
	}
	return string(outputValueData), nil

}
