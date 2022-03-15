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

package common

import (
	corev1 "k8s.io/api/core/v1"
)

// MergeVolumes - merge pod volumes in-place
func MergeVolumes(vols []corev1.Volume, newVols []corev1.Volume) []corev1.Volume {
	for _, f := range newVols {
		updated := false
		for i := 0; i < len(vols); i++ {
			if vols[i].Name == f.Name {
				vols[i] = f
				updated = true
				break
			}
		}

		if !updated {
			vols = append(vols, f)
		}
	}

	return vols
}

// MergeVolumeMounts - merge container volume mounts in-place
func MergeVolumeMounts(vols []corev1.VolumeMount, newVols []corev1.VolumeMount) []corev1.VolumeMount {
	for _, f := range newVols {
		updated := false
		for i := 0; i < len(vols); i++ {
			if vols[i].MountPath == f.MountPath {
				vols[i] = f
				updated = true
				break
			}
		}

		if !updated {
			vols = append(vols, f)
		}
	}

	return vols
}
