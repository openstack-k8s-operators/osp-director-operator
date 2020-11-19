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

package provisionserver

import (
	corev1 "k8s.io/api/core/v1"
)

// GetVolumes - general provisioning service volumes
func GetVolumes(name string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: name + "-image-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: name + "-httpd-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name + "-httpd-config",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "httpd.conf",
							Path: "httpd.conf",
						},
						{
							Key:  "mime.types",
							Path: "mime.types",
						},
					},
				},
			},
		},
	}
}

// GetInitVolumeMounts - general init task VolumeMounts
func GetInitVolumeMounts(name string) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      name + "-image-data",
			MountPath: "/usr/local/apache2/htdocs",
		},
		{
			Name:      name + "-httpd-config",
			MountPath: "/usr/local/apache2/conf",
			ReadOnly:  false,
		},
	}
}

// GetVolumeMounts - general VolumeMounts
func GetVolumeMounts(name string) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      name + "-image-data",
			MountPath: "/usr/local/apache2/htdocs",
		},
		{
			Name:      name + "-httpd-config",
			MountPath: "/usr/local/apache2/conf",
			ReadOnly:  true,
		},
	}
}
