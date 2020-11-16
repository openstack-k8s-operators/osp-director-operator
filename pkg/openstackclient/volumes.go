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

package openstackclient

import (
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumeMounts -
func GetVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "id-rsa",
			MountPath: "/root/.ssh",
			ReadOnly:  true,
		},
	}

}

// GetVolumes -
func GetVolumes(instance *ospdirectorv1beta1.OpenStackClient) []corev1.Volume {
	var config0600AccessMode int32 = 0600

	return []corev1.Volume{
		{
			Name: "id-rsa",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0600AccessMode,
					SecretName:  instance.Spec.DeploymentSSHSecret,
					Items: []corev1.KeyToPath{
						{
							Key:  "identity",
							Path: "id_rsa",
						},
					},
				},
			},
		},
	}

}
