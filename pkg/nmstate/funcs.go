package nmstate

import (
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"
)

// GetDesiredStatedBridgeName - Get the bridge name associated with the desiredState of the attachConfiguration
func GetDesiredStatedBridgeName(desiredStateBytes []byte) (string, error) {
	bridge := ""

	if len(desiredStateBytes) > 0 {
		jsonBytes, err := yaml.YAMLToJSON(desiredStateBytes)

		if err != nil {
			return "", err
		}

		jsonStr := string(jsonBytes)

		if gjson.Get(jsonStr, "interfaces.#.name").Exists() {
			bridge = gjson.Get(jsonStr, "interfaces.#.name").Array()[0].String()
		}
	}

	return bridge, nil
}
