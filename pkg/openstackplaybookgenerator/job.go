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
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PlaybookJob -
func PlaybookJob(cr *ospdirectorv1beta1.OpenStackPlaybookGenerator, configHash string) *batchv1.Job {

	runAsUser := int64(openstackclient.CloudAdminUID)
	runAsGroup := int64(openstackclient.CloudAdminGID)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "generate-playbooks-" + cr.Name,
			Namespace: cr.Namespace,
		},
	}

	var terminationGracePeriodSeconds int64 = 0
	var backoffLimit int32 = 2

	// Get volumes
	volumeMounts := GetVolumeMounts(cr)
	volumes := GetVolumes(cr)

	cmd := []string{"/bin/bash", "/home/cloud-admin/create-playbooks.sh"}
	if cr.Spec.Debug {
		cmd = []string{"/bin/sleep", "infinity"}
	}

	job.Spec.BackoffLimit = &backoffLimit
	job.Spec.Template.Spec = corev1.PodSpec{
		RestartPolicy:      corev1.RestartPolicyOnFailure,
		ServiceAccountName: openstackclient.ServiceAccount,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:  &runAsUser,
			RunAsGroup: &runAsGroup,
			FSGroup:    &runAsGroup,
		},
		TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
		Volumes:                       volumes,
		Containers: []corev1.Container{
			{
				Name:            "generateplaybooks",
				Image:           cr.Spec.ImageURL,
				ImagePullPolicy: corev1.PullAlways,
				Command:         cmd,
				Env: []corev1.EnvVar{
					{
						Name:  "ConfigHash",
						Value: configHash,
					},
					{
						Name: "GIT_URL",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: cr.Spec.GitSecret,
								},
								Key: "git_url",
							},
						},
					},
				},
				VolumeMounts: volumeMounts,
			},
		},
	}

	return job
}
