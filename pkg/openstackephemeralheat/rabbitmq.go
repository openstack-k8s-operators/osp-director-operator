package openstackephemeralheat

import (
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RabbitmqGetLabels -
func RabbitmqGetLabels(name string) map[string]string {
	return map[string]string{"owner": "osp-director-operator", "cr": name, "app": "rabbitmq"}
}

// RabbitmqPod -
func RabbitmqPod(instance *ospdirectorv1beta1.OpenStackEphemeralHeat) *corev1.Pod {

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rabbitmq-" + instance.Name,
			Namespace: instance.Namespace,
			Labels:    RabbitmqGetLabels(instance.Name),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "rabbitmq",
					Image: instance.Spec.RabbitImageURL,
					Env: []corev1.EnvVar{
						// added to resolve "Failed to create thread" aborts
						{
							Name:  "RABBITMQ_IO_THREAD_POOL_SIZE",
							Value: "128",
						},
						{
							Name:  "KOLLA_CONFIG_STRATEGY",
							Value: "COPY_ALWAYS",
						},
						{
							Name:  "ConfigHash",
							Value: instance.Spec.ConfigHash,
						},
					},
					VolumeMounts: getRabbitmqVolumeMounts(),
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:  "rabbitmq-init",
					Image: instance.Spec.RabbitImageURL,
					Env: []corev1.EnvVar{
						{
							Name:  "KOLLA_CONFIG_STRATEGY",
							Value: "COPY_ALWAYS",
						},
						{
							Name:  "KOLLA_BOOTSTRAP",
							Value: "true",
						},
						{
							Name:  "RABBITMQ_CLUSTER_COOKIE",
							Value: "foobar123", //FIXME
						},
					},
					VolumeMounts: getRabbitmqVolumeMounts(),
				},
			},
			Volumes: getRabbitmqVolumes(instance.Name),
		},
	}
	return pod
}

// RabbitmqService func
func RabbitmqService(instance *ospdirectorv1beta1.OpenStackEphemeralHeat, scheme *runtime.Scheme) *corev1.Service {

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rabbitmq-" + instance.Name,
			Namespace: instance.Namespace,
			Labels:    RabbitmqGetLabels(instance.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "rabbitmq"},
			Ports: []corev1.ServicePort{
				{Name: "rabbitmq", Port: 5672, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	return svc
}

func getRabbitmqVolumes(name string) []corev1.Volume {

	return []corev1.Volume{

		{
			Name: "kolla-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackephemeralheat-" + name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "rabbitmq_config.json",
							Path: "config.json",
						},
					},
				},
			},
		},
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackephemeralheat-" + name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "rabbitmq.config",
							Path: "rabbitmq.config",
						},
						{
							Key:  "rabbitmq-env.conf",
							Path: "rabbitmq-env.conf",
						},
					},
				},
			},
		},
		{
			Name: "lib-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
	}

}

func getRabbitmqVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			MountPath: "/var/lib/config-data",
			ReadOnly:  true,
			Name:      "config-data",
		},
		{
			MountPath: "/var/lib/kolla/config_files",
			ReadOnly:  true,
			Name:      "kolla-config",
		},
		{
			MountPath: "/var/lib/rabbitmq",
			ReadOnly:  false,
			Name:      "lib-data",
		},
	}

}
