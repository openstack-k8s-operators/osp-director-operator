package provisionserver

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

// GetInitContainers - init containers for ProvisionServers
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
			ImagePullPolicy: corev1.PullAlways,
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
