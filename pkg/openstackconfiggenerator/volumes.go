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

package openstackconfiggenerator

import (
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumeMounts -
func GetVolumeMounts(instance *ospdirectorv1beta1.OpenStackConfigGenerator) []corev1.VolumeMount {
	retVolMounts := []corev1.VolumeMount{
		{
			Name:      "tripleo-deploy-config-" + instance.Name,
			MountPath: "/home/cloud-admin/config",
			ReadOnly:  true,
		},
		{
			Name:      controlplane.TripleoPasswordSecret,
			MountPath: "/home/cloud-admin/config-passwords",
			ReadOnly:  true,
		},
		{
			Name:      "heat-env-config",
			MountPath: "/home/cloud-admin/config-custom",
			ReadOnly:  true,
		},
		{
			Name:      "openstackconfig-scripts",
			MountPath: "/home/cloud-admin/create-playbooks.sh",
			SubPath:   "create-playbooks.sh",
			ReadOnly:  true,
		},
		{
			Name:      "openstackconfig-scripts",
			MountPath: "/home/cloud-admin/process-heat-environment.py",
			SubPath:   "process-heat-environment.py",
			ReadOnly:  true,
		},
		{
			Name:      "openstackconfig-scripts",
			MountPath: "/home/cloud-admin/process-roles.py",
			SubPath:   "process-roles.py",
			ReadOnly:  true,
		},
		{
			Name:      "git-ssh-config",
			MountPath: "/mnt/ssh-config/git_id_rsa",
			SubPath:   "git_id_rsa",
			ReadOnly:  true,
		},
	}

	if instance.Spec.TarballConfigMap != "" {
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
func GetVolumes(instance *ospdirectorv1beta1.OpenStackConfigGenerator) []corev1.Volume {
	var config0600AccessMode int32 = 0600
	var config0644AccessMode int32 = 0644
	var config0755AccessMode int32 = 0755

	retVolumes := []corev1.Volume{
		{
			Name: "tripleo-deploy-config-" + instance.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0644AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "tripleo-deploy-config-" + instance.Name,
					},
				},
			},
		},
		{
			Name: "heat-env-config",
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
			Name: controlplane.TripleoPasswordSecret,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  controlplane.TripleoPasswordSecret,
				},
			},
		},
		{
			Name: "openstackconfig-scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0755AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackconfig-script-" + instance.Name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "create-playbooks.sh",
							Path: "create-playbooks.sh",
						},
						{
							Key:  "process-heat-environment.py",
							Path: "process-heat-environment.py",
						},
						{
							Key:  "process-roles.py",
							Path: "process-roles.py",
						},
					},
				},
			},
		},
		{
			Name: "git-ssh-config", //ssh key for git repo access
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  instance.Spec.GitSecret,
					Items: []corev1.KeyToPath{
						{
							Key:  "git_ssh_identity",
							Path: "git_id_rsa",
							Mode: &config0600AccessMode,
						},
					},
				},
			},
		},
	}

	if instance.Spec.TarballConfigMap != "" {
		retVolumes = append(retVolumes,
			corev1.Volume{
				Name: "tripleo-deploy-tars",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						DefaultMode: &config0644AccessMode,
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.Spec.TarballConfigMap,
						},
					},
				},
			},
		)
	}
	return retVolumes
}
