package provisionserver

import (
	corev1 "k8s.io/api/core/v1"
)

// InitContainer information
type InitContainer struct {
	Privileged     bool
	ContainerImage string
	RhelImageURL   string
	VolumeMounts   []corev1.VolumeMount
}

// GetInitContainer - init container for cinder services
func GetInitContainer(init InitContainer) []corev1.Container {
	trueVar := true

	securityContext := &corev1.SecurityContext{}

	if init.Privileged {
		securityContext.Privileged = &trueVar
	}

	return []corev1.Container{
		{
			Name:            "init",
			Image:           init.ContainerImage,
			SecurityContext: securityContext,
			VolumeMounts:    init.VolumeMounts,
			Env: []corev1.EnvVar{
				{
					Name:  "RHEL_IMAGE_URL",
					Value: init.RhelImageURL,
				},
			},
		},
	}
}
