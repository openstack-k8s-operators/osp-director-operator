package provisionserver

import (
	ospdirectorv1 "github.com/abays/osp-director-operator/api/v1beta1"
	util "github.com/openstack-k8s-operators/lib-common/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type provisionServerConfigOptions struct {
	Port int
}

// HttpdConfigMap - custom httpd config map
func HttpdConfigMap(cr *ospdirectorv1.ProvisionServer) *corev1.ConfigMap {
	opts := provisionServerConfigOptions{cr.Spec.Port}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-httpd-config",
			Namespace: cr.Namespace,
		},
		Data: map[string]string{
			"httpd.conf": util.ExecuteTemplateFile("httpd/config/httpd.conf", &opts),
			"mime.types": util.ExecuteTemplateFile("httpd/config/mime.types", nil),
		},
	}

	return cm
}
