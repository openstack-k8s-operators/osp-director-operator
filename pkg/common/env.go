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

import (
	corev1 "k8s.io/api/core/v1"
)

// Update a list of corev1.EnvVar in place

// EnvSetter -
type EnvSetter func(*corev1.EnvVar)

// EnvSetterMap -
type EnvSetterMap map[string]EnvSetter

// MergeEnvs - merge envs
func MergeEnvs(envs []corev1.EnvVar, newEnvs EnvSetterMap) []corev1.EnvVar {
	for name, f := range newEnvs {
		updated := false
		for i := 0; i < len(envs); i++ {
			if envs[i].Name == name {
				f(&envs[i])
				updated = true
				break
			}
		}

		if !updated {
			envs = append(envs, corev1.EnvVar{Name: name})
			f(&envs[len(envs)-1])
		}
	}

	return envs
}

// EnvDownwardAPI - set env from FieldRef->FieldPath, e.g. status.podIP
func EnvDownwardAPI(field string) EnvSetter {
	return func(env *corev1.EnvVar) {
		if env.ValueFrom == nil {
			env.ValueFrom = &corev1.EnvVarSource{}
		}
		env.Value = ""

		if env.ValueFrom.FieldRef == nil {
			env.ValueFrom.FieldRef = &corev1.ObjectFieldSelector{}
		}

		env.ValueFrom.FieldRef.FieldPath = field
	}
}

// EnvValue -
func EnvValue(value string) EnvSetter {
	return func(env *corev1.EnvVar) {
		env.Value = value
		env.ValueFrom = nil
	}
}
