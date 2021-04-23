package openstackephemeralheat

import (
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// HeatGetLabels -
func HeatGetLabels(name string) map[string]string {
	return map[string]string{"owner": "osp-director-operator", "cr": name, "app": "heat"}
}

// HeatAPIPod -
func HeatAPIPod(instance *ospdirectorv1beta1.OpenStackEphemeralHeat) *corev1.Pod {

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    HeatGetLabels(instance.Name),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "heat",
					Image: "quay.io/tripleomaster/openstack-heat-api:current-tripleo", //FIXME
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
					VolumeMounts: getHeatVolumeMounts(),
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:    "drop-heat",
					Image:   "quay.io/tripleomaster/openstack-mariadb:current-tripleo", //FIXME
					Command: []string{"sh", "-c", "mysql -h mariadb-" + instance.Name + " -u root -P 3306 -e \"DROP DATABASE IF EXISTS heat\";"},
					Env: []corev1.EnvVar{
						{
							Name:  "MYSQL_PWD",
							Value: "foobar123",
						},
					},
				},
				{
					Name:    "heat-db-create",
					Image:   "quay.io/tripleomaster/openstack-mariadb:current-tripleo", //FIXME
					Command: []string{"sh", "-c", "mysql -h mariadb-" + instance.Name + " -u root -P 3306 -e \"CREATE DATABASE IF NOT EXISTS heat; GRANT ALL PRIVILEGES ON heat.* TO 'heat'@'localhost' IDENTIFIED BY 'foobar123'; GRANT ALL PRIVILEGES ON heat.* TO 'heat'@'%' IDENTIFIED BY 'foobar123'; \""},
					Env: []corev1.EnvVar{
						{
							Name:  "MYSQL_PWD",
							Value: "foobar123",
						},
					},
				},
				{
					Name:    "heat-db-sync",
					Image:   "quay.io/tripleomaster/openstack-heat-api:current-tripleo", //FIXME
					Command: []string{"/usr/bin/heat-manage", "--config-file", "/var/lib/config-data/heat.conf", "db_sync"},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/var/lib/config-data",
							ReadOnly:  true,
							Name:      "config-data",
						},
					},
				},
			},
			Volumes: getHeatVolumes(instance.Name),
		},
	}
	return pod
}

// HeatAPIService func
func HeatAPIService(instance *ospdirectorv1beta1.OpenStackEphemeralHeat, scheme *runtime.Scheme) *corev1.Service {

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "heat-" + instance.Name,
			Namespace: instance.Namespace,
			Labels:    HeatGetLabels(instance.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "heat"},
			Ports: []corev1.ServicePort{
				{Name: "heat", Port: 8004, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	return svc
}

// HeatEngineReplicaSet -
func HeatEngineReplicaSet(instance *ospdirectorv1beta1.OpenStackEphemeralHeat, replicas int32) *appsv1.ReplicaSet {

	selectorLabels := map[string]string{
		"app":              "osp-director-operator-heat-engine",
		"heat-engine-name": instance.Name,
	}

	replicaset := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    HeatGetLabels(instance.Name),
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.Name,
					Namespace: instance.Namespace,
					Labels:    selectorLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "heat-engine",
							Image: "quay.io/tripleomaster/openstack-heat-engine:current-tripleo", //FIXME
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
							VolumeMounts: getHeatEngineVolumeMounts(),
						},
					},
					Volumes: getHeatVolumes(instance.Name),
				},
			},
		},
	}
	return replicaset
}

func getHeatVolumes(name string) []corev1.Volume {

	return []corev1.Volume{

		{
			Name: "kolla-config-api",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackephemeralheat-" + name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "heat_config.json",
							Path: "config.json",
						},
					},
				},
			},
		},
		{
			Name: "kolla-config-engine",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackephemeralheat-" + name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "heat_engine_config.json",
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
							Key:  "heat.conf",
							Path: "heat.conf",
						},
						{
							Key:  "heat_paste.ini",
							Path: "paste.ini",
						},
						{
							Key:  "heat_token.json",
							Path: "token.json",
						},
						{
							Key:  "heat_noauth.policy",
							Path: "noauth.policy",
						},
					},
				},
			},
		},
	}

}

func getHeatVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			MountPath: "/var/lib/config-data",
			ReadOnly:  true,
			Name:      "config-data",
		},
		{
			MountPath: "/var/lib/kolla/config_files",
			ReadOnly:  true,
			Name:      "kolla-config-api",
		},
	}

}

func getHeatEngineVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			MountPath: "/var/lib/config-data",
			ReadOnly:  true,
			Name:      "config-data",
		},
		{
			MountPath: "/var/lib/kolla/config_files",
			ReadOnly:  true,
			Name:      "kolla-config-engine",
		},
	}

}
