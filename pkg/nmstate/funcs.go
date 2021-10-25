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

// GetCurrentCondition - Get current condition with status == corev1.ConditionTrue
func GetCurrentCondition(conditions nmstateshared.ConditionList) *nmstateshared.Condition {
	for i, cond := range conditions {
		if cond.Status == corev1.ConditionTrue {
			return &conditions[i]
		}
	}

	return nil
}
