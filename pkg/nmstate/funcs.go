package nmstate

import (
	nmstateshared "github.com/nmstate/kubernetes-nmstate/api/shared"
	"github.com/tidwall/gjson"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// GetDesiredStatedBridgeName - Get the bridge name associated with the desiredState of the attachConfiguration
func GetDesiredStatedBridgeName(desiredStateBytes []byte) (string, error) {
	bridge := ""

	jsonStr, err := GetDesiredStateAsString(desiredStateBytes)
	if err != nil {
		return "", err
	}

	if gjson.Get(jsonStr, "interfaces.#.name").Exists() {
		bridge = gjson.Get(jsonStr, "interfaces.#.name").Array()[0].String()
	}

	return bridge, nil
}

// GetCurrentCondition - Get current condition with status == corev1.ConditionTrue
func GetCurrentCondition(conditions nmstateshared.ConditionList) *nmstateshared.Condition {
	for i, cond := range conditions {
		if cond.Status == corev1.ConditionTrue {
			return &conditions[i]
		}
	}

	return nil
}

// GetDesiredStateInterfaceState - Get the state of the bridge associated interface of the attachConfiguration
func GetDesiredStateInterfaceState(desiredStateBytes []byte) (string, error) {
	state := ""

	jsonStr, err := GetDesiredStateAsString(desiredStateBytes)
	if err != nil {
		return "", err
	}

	if gjson.Get(jsonStr, "interfaces.#.state").Exists() {
		state = gjson.Get(jsonStr, "interfaces.#.state").Array()[0].String()
	}

	return state, nil
}

// GetDesiredStateAsString - Get the state as string
func GetDesiredStateAsString(desiredStateBytes []byte) (string, error) {
	jsonStr := ""

	if len(desiredStateBytes) > 0 {
		jsonBytes, err := yaml.YAMLToJSON(desiredStateBytes)

		if err != nil {
			return "", err
		}

		jsonStr = string(jsonBytes)
	}

	return jsonStr, nil
}
