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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DistributePods - returns rule to ensure that two replicas of the same selector
// should not run if possible on the same worker node
func DistributePods(
	selectorKey string,
	selectorValues []string,
	topologyKey string,
) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			// This rule ensures that two replicas of the same selector
			// should not run if possible on the same worker node
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      selectorKey,
									Operator: metav1.LabelSelectorOpIn,
									Values:   selectorValues,
								},
							},
						},
						// usually corev1.LabelHostname "kubernetes.io/hostname"
						// https://github.com/kubernetes/api/blob/master/core/v1/well_known_labels.go#L20
						TopologyKey: topologyKey,
					},
					Weight: 1,
				},
			},
		},
	}
}
