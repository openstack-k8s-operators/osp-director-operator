package openstackephemeralheat

import (
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// MariadbGetLabels -
func MariadbGetLabels(name string) map[string]string {
	return map[string]string{"owner": "mariadb-operator", "cr": name, "app": "mariadb"}
}

// Pod -
func MariadbPod(instance *ospdirectorv1beta1.OpenStackEphemeralHeat) *corev1.Pod {

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    MariadbGetLabels(instance.Name),
		},
		Spec: corev1.PodSpec{
			//ServiceAccountName: "mariadb",
			Containers: []corev1.Container{
				{
					Name:  "mariadb",
					Image: "docker.io/tripleomaster/centos-binary-mariadb:current-tripleo", //FIXME
					Env: []corev1.EnvVar{
						{
							Name:  "KOLLA_CONFIG_STRATEGY",
							Value: "COPY_ALWAYS",
						},
					},
					VolumeMounts: getMariadbVolumeMounts(),
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:  "mariadb-init",
					Image: "docker.io/tripleomaster/centos-binary-mariadb:current-tripleo", //FIXME
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
							Value: "foobar123", //FIXME
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

// Service func
func MariadbService(instance *ospdirectorv1beta1.OpenStackEphemeralHeat, scheme *runtime.Scheme) *corev1.Service {

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
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
	controllerutil.SetControllerReference(instance, svc, scheme)
	return svc
}

var config0755AccessMode int32 = 075

func getMariadbVolumes(name string) []corev1.Volume {

	return []corev1.Volume{

		{
			Name: "kolla-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackephemeralheat",
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
						Name: "openstackephemeralheat",
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
						Name: "openstackephemeralheat",
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
