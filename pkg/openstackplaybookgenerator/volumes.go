/*
Copyright 2021 Red Hat

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

package openstackplaybookgenerator

import (
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumeMounts -
func GetVolumeMounts(instance *ospdirectorv1beta1.OpenStackPlaybookGenerator) []corev1.VolumeMount {
	retVolMounts := []corev1.VolumeMount{
		{
			Name:      fmt.Sprintf("%s-cloud-admin", instance.Spec.OpenStackClientName),
			MountPath: "/var/cloud-admin", // this is the cloud-admin mounted on openstackclient pod
			ReadOnly:  false,
		},
		{
			Name:      "tripleo-deploy-config",
			MountPath: "/home/cloud-admin/config",
			ReadOnly:  true,
		},
		{
			Name:      "tripleo-deploy-config-custom",
			MountPath: "/home/cloud-admin/config-custom",
			ReadOnly:  true,
		},
		{
			Name:      "openstackplaybook-scripts",
			MountPath: "/home/cloud-admin/create-playbooks.sh",
			SubPath:   "create-playbooks.sh",
			ReadOnly:  true,
		},
	}

	if instance.Spec.HeatTemplateTarball != "" {
		retVolMounts = append(retVolMounts,
			corev1.VolumeMount{
				Name:      "tripleo-deploy-tars",
				MountPath: "/home/cloud-admin/tht-tars",
				ReadOnly:  true,
			},
		)
	}
	return retVolMounts
}

// GetVolumes -
func GetVolumes(instance *ospdirectorv1beta1.OpenStackPlaybookGenerator) []corev1.Volume {
	//var config0600AccessMode int32 = 0600
	var config0644AccessMode int32 = 0644
	var config0755AccessMode int32 = 0755

	retVolumes := []corev1.Volume{
		{
			Name: "tripleo-deploy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0644AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "tripleo-deploy-config",
					},
				},
			},
		},
		{
			Name: "tripleo-deploy-config-custom",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0644AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.HeatEnvConfigMap,
					},
				},
			},
		},
		{
			Name: "openstackplaybook-scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0755AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackplaybook-script-" + instance.Name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "create-playbooks.sh",
							Path: "create-playbooks.sh",
						},
					},
				},
			},
		},
		{
			Name: fmt.Sprintf("%s-cloud-admin", instance.Spec.OpenStackClientName),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-cloud-admin", instance.Spec.OpenStackClientName),
				},
			},
		},
	}

	if instance.Spec.HeatTemplateTarball != "" {
		retVolumes = append(retVolumes,
			corev1.Volume{
				Name: "tripleo-deploy-tars",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						DefaultMode: &config0644AccessMode,
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.HeatTemplateTarball,
						},
					},
				},
			},
		)
	}
	return retVolumes
}
