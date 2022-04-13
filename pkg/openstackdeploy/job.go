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

package openstackdeploy

import (
	"strconv"
	"strings"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	openstackclient "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackclient"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployJob -
func DeployJob(
	cr *ospdirectorv1beta1.OpenStackDeploy,
	openstackClientPod string,
	configVersion string,
	gitSecret string,
	advancedSettings *ospdirectorv1beta1.OpenStackDeployAdvancedSettingsSpec,
	ospVersion shared.OSPVersion,
) *batchv1.Job {

	runAsUser := int64(openstackclient.CloudAdminUID)
	runAsGroup := int64(openstackclient.CloudAdminGID)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy-openstack-" + cr.Name,
			Namespace: cr.Namespace,
		},
	}

	var terminationGracePeriodSeconds int64 = 60
	var backoffLimit int32 = 0

	cmd := []string{"/osp-director-agent", "deploy"}
	restartPolicy := corev1.RestartPolicyNever

	job.Spec.BackoffLimit = &backoffLimit
	job.Spec.Template.Spec = corev1.PodSpec{
		RestartPolicy:      restartPolicy,
		ServiceAccountName: ServiceAccount,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:  &runAsUser,
			RunAsGroup: &runAsGroup,
			FSGroup:    &runAsGroup,
		},
		TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
		Containers: []corev1.Container{
			{
				Name:            "deploy-openstack",
				Image:           cr.Spec.ImageURL,
				ImagePullPolicy: corev1.PullAlways,
				Command:         cmd,
				Env: []corev1.EnvVar{
					// NOTE: CONFIG_VERSION must be the first ENV due to logic in openstackdeploy_controller
					{
						Name:  "CONFIG_VERSION",
						Value: configVersion,
					},
					{
						Name:  "DEPLOY_NAME",
						Value: cr.Name,
					},
					{
						Name:  "OSP_DIRECTOR_OPERATOR_NAMESPACE",
						Value: cr.Namespace,
					},
					{
						Name:  "OPENSTACKCLIENT_POD",
						Value: openstackClientPod,
					},
					{
						Name:  "OSP_VERSION",
						Value: string(ospVersion),
					},
					{
						Name: "GIT_URL",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: gitSecret,
								},
								Key: "git_url",
							},
						},
					},
					{
						Name: "GIT_ID_RSA",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: gitSecret,
								},
								Key: "git_ssh_identity",
							},
						},
					},
					{
						Name:  "PLAYBOOK",
						Value: advancedSettings.Playbook,
					},
					{
						Name:  "LIMIT",
						Value: advancedSettings.Limit,
					},
					{
						Name:  "TAGS",
						Value: strings.Join(advancedSettings.Tags, ","),
					},
					{
						Name:  "SKIP_TAGS",
						Value: strings.Join(advancedSettings.SkipTags, ","),
					},
					{
						Name:  "SKIP_DEPLOY_IDENTIFIER",
						Value: strconv.FormatBool(advancedSettings.SkipDeployIdentifier),
					},
				},
			},
		},
	}

	return job
}
