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
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumeMounts -
func GetVolumeMounts(instance *ospdirectorv1beta1.OpenStackClient) []corev1.VolumeMount {

	volumes := []corev1.VolumeMount{
		{
			Name:      fmt.Sprintf("%s-hosts", instance.Name),
			MountPath: "/etc/hosts",
			SubPath:   "hosts",
			ReadOnly:  false,
		},
		{
			Name:      fmt.Sprintf("%s-hosts", instance.Name),
			MountPath: "/etc/hostname",
			SubPath:   "hostname",
			ReadOnly:  false,
		},
		{
			Name:      fmt.Sprintf("%s-cloud-admin", instance.Name),
			MountPath: "/home/cloud-admin",
			ReadOnly:  false,
		},
		{
			Name:      "openstackclient-scripts",
			MountPath: "/usr/local/bin/tripleo-deploy.sh",
			SubPath:   "tripleo-deploy.sh",
			ReadOnly:  true,
		},
		{
			Name:      "openstackclient-scripts",
			MountPath: "/usr/local/bin/tripleo-deploy-term.sh",
			SubPath:   "tripleo-deploy-term.sh",
			ReadOnly:  true,
		},
		{
			Name:      "openstackclient-scripts",
			MountPath: "/usr/local/bin/tripleo-export-ceph",
			SubPath:   "tripleo-export-ceph",
			ReadOnly:  true,
		},
		{
			Name:      "openstackclient-scripts",
			MountPath: "/home/cloud-admin/ctlplane-ansible-inventory",
			SubPath:   "ansible-inventory",
			ReadOnly:  true,
		},
		{
			Name:      "kolla-config",
			MountPath: "/var/lib/kolla/config_files",
			ReadOnly:  true,
		},
		{
			Name:      fmt.Sprintf("%s-kolla-src", instance.Name),
			MountPath: "/var/lib/kolla/src",
			ReadOnly:  true,
		},
	}

	if instance.Spec.DeploymentSSHSecret != "" {
		volumes = append(volumes, corev1.VolumeMount{
			Name:      "root-ssh",
			MountPath: "/root/.ssh",
			ReadOnly:  true})
	}

	return volumes
}

// GetInitVolumeMounts -
func GetInitVolumeMounts(instance *ospdirectorv1beta1.OpenStackClient) []corev1.VolumeMount {

	volumes := []corev1.VolumeMount{
		{
			Name:      "openstackclient-scripts",
			MountPath: "/usr/local/bin/init.sh",
			SubPath:   "init.sh",
			ReadOnly:  true,
		},
		{
			Name:      fmt.Sprintf("%s-hosts", instance.Name),
			MountPath: "/mnt/etc",
			ReadOnly:  false,
		},
		{
			Name:      fmt.Sprintf("%s-cloud-admin", instance.Name),
			MountPath: "/home/cloud-admin",
			ReadOnly:  false,
		},
		{
			Name:      "root-ssh",
			MountPath: "/root/.ssh",
			ReadOnly:  false,
		},
		{
			Name:      fmt.Sprintf("%s-kolla-src", instance.Name),
			MountPath: "/var/lib/kolla/src",
			ReadOnly:  false,
		},
		{
			Name:      "kolla-config-init",
			MountPath: "/var/lib/kolla/config_files",
			ReadOnly:  true,
		},
	}

	if instance.Spec.DeploymentSSHSecret != "" {
		volumes = append(volumes, []corev1.VolumeMount{
			{
				Name:      "ssh-config",
				MountPath: "/mnt/ssh-config/id_rsa",
				SubPath:   "id_rsa",
				ReadOnly:  true,
			},
			{
				Name:      "ssh-config",
				MountPath: "/mnt/ssh-config/id_rsa.pub",
				SubPath:   "id_rsa.pub",
				ReadOnly:  true,
			},
			{
				Name:      "ssh-config",
				MountPath: "/mnt/ssh-config/config",
				SubPath:   "config",
				ReadOnly:  true,
			},
		}...)
	}

	if instance.Spec.CAConfigMap != "" {
		volumes = append(volumes, corev1.VolumeMount{
			Name:      "ca-certs",
			MountPath: "/mnt/ca-certs",
			ReadOnly:  true,
		})
	}

	return volumes
}

// GetVolumes -
func GetVolumes(instance *ospdirectorv1beta1.OpenStackClient) []corev1.Volume {
	var config0600AccessMode int32 = 0600
	var config0644AccessMode int32 = 0644
	var config0755AccessMode int32 = 0755

	volumes := []corev1.Volume{
		{
			Name: "root-ssh",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
		{
			Name: "openstackclient-scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0755AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackclient-sh",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "tripleo-deploy.sh",
							Path: "tripleo-deploy.sh",
						},
						{
							Key:  "tripleo-deploy-term.sh",
							Path: "tripleo-deploy-term.sh",
						},
						{
							Key:  "tripleo-export-ceph",
							Path: "tripleo-export-ceph",
						},
						{
							Key:  "init.sh",
							Path: "init.sh",
						},
						{
							Key:  "ansible-inventory",
							Path: "ansible-inventory",
						},
					},
				},
			},
		},
		{
			Name: fmt.Sprintf("%s-hosts", instance.Name),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-hosts", instance.Name),
				},
			},
		},
		{
			Name: fmt.Sprintf("%s-cloud-admin", instance.Name),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-cloud-admin", instance.Name),
				},
			},
		},
		{
			Name: fmt.Sprintf("%s-kolla-src", instance.Name),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-kolla-src", instance.Name),
				},
			},
		},
		{
			Name: "kolla-config-init",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0644AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackclient-sh",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "kolla_config_init.json",
							Path: "config.json",
						},
					},
				},
			},
		},
		{
			Name: "kolla-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0644AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackclient-sh",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "kolla_config.json",
							Path: "config.json",
						},
					},
				},
			},
		},
	}

	if instance.Spec.DeploymentSSHSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "ssh-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  instance.Spec.DeploymentSSHSecret,
					Items: []corev1.KeyToPath{
						{
							Key:  "identity",
							Path: "id_rsa",
							Mode: &config0600AccessMode,
						},
						{
							Key:  "authorized_keys",
							Path: "id_rsa.pub",
						},
						{
							Key:  "config",
							Path: "config",
						},
					},
				},
			},
		})
	}

	if instance.Spec.CAConfigMap != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "ca-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0644AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.CAConfigMap,
					},
				},
			},
		})
	}

	return volumes
}
