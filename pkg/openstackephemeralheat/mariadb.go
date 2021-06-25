package openstackephemeralheat

import (
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// MariadbGetLabels -
func MariadbGetLabels(name string) map[string]string {
	return map[string]string{"owner": "osp-director-operator", "cr": name, "app": "mariadb"}
}

// MariadbPod -
func MariadbPod(instance *ospdirectorv1beta1.OpenStackEphemeralHeat, password string) *corev1.Pod {
	var runAsUser = int64(MySQLUID)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-" + instance.Name,
			Namespace: instance.Namespace,
			Labels:    MariadbGetLabels(instance.Name),
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: ServiceAccount,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: &runAsUser,
			},
			Containers: []corev1.Container{
				{
					Name:  "mariadb",
					Image: instance.Spec.MariadbImageURL,
					Env: []corev1.EnvVar{
						{
							Name:  "KOLLA_CONFIG_STRATEGY",
							Value: "COPY_ALWAYS",
						},
						{
							Name:  "ConfigHash",
							Value: instance.Spec.ConfigHash,
						},
					},
					VolumeMounts: getMariadbVolumeMounts(),
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:  "mariadb-init",
					Image: instance.Spec.MariadbImageURL,
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
							Name:  "DB_MAX_TIMEOUT",
							Value: "60",
						},
						{
							Name:  "DB_ROOT_PASSWORD",
							Value: password,
						},
					},
					VolumeMounts: getMariadbInitVolumeMounts(),
				},
			},
			Volumes: getMariadbVolumes(instance.Name),
		},
	}
	return pod
}

// MariadbService func
func MariadbService(instance *ospdirectorv1beta1.OpenStackEphemeralHeat, scheme *runtime.Scheme) *corev1.Service {

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-" + instance.Name,
			Namespace: instance.Namespace,
			Labels:    MariadbGetLabels(instance.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "mariadb"},
			Ports: []corev1.ServicePort{
				{Name: "database", Port: 3306, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	return svc
}

func getMariadbVolumes(name string) []corev1.Volume {

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
							Key:  "mariadb_config.json",
							Path: "config.json",
						},
					},
				},
			},
		},
		{
			Name: "kolla-config-init",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackephemeralheat-" + name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "mariadb_init_config.json",
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
							Key:  "galera.cnf",
							Path: "galera.cnf",
						},
						{
							Key:  "mariadb_init.sh",
							Path: "mariadb_init.sh",
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

func getMariadbVolumeMounts() []corev1.VolumeMount {
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
			MountPath: "/var/lib/mysql",
			ReadOnly:  false,
			Name:      "lib-data",
		},
	}

}

func getMariadbInitVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			MountPath: "/var/lib/config-data",
			ReadOnly:  true,
			Name:      "config-data",
		},
		{
			MountPath: "/var/lib/kolla/config_files",
			ReadOnly:  true,
			Name:      "kolla-config-init",
		},
		{
			MountPath: "/var/lib/mysql",
			ReadOnly:  false,
			Name:      "lib-data",
		},
	}

}
