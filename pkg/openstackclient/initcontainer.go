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

	corev1 "k8s.io/api/core/v1"
)

// InitContainer information
type InitContainer struct {
	Args           []string
	Commands       []string
	ContainerImage string
	Env            []corev1.EnvVar
	Privileged     bool
	VolumeMounts   []corev1.VolumeMount
}

// GetInitContainers - init containers
func GetInitContainers(inits []InitContainer) []corev1.Container {
	trueVar := true

	securityContext := &corev1.SecurityContext{}
	initContainers := []corev1.Container{}

	for index, init := range inits {
		if init.Privileged {
			securityContext.Privileged = &trueVar
		}

		container := corev1.Container{
			Name:            fmt.Sprintf("init-%d", index),
			Image:           init.ContainerImage,
			SecurityContext: securityContext,
			VolumeMounts:    init.VolumeMounts,
			Env:             init.Env,
		}

		if len(init.Args) != 0 {
			container.Args = init.Args
		}

		if len(init.Commands) != 0 {
			container.Command = init.Commands
		}

		initContainers = append(initContainers, container)
	}

	return initContainers
}
