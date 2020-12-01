package common

import (
	corev1 "k8s.io/api/core/v1"
)

// HostAliasesFromPodlist -
func HostAliasesFromPodlist(podList *corev1.PodList) []corev1.HostAlias {
	hostAliases := []corev1.HostAlias{}

	if len(podList.Items) > 0 {
		for _, pod := range podList.Items {
			hostAliases = append(hostAliases, corev1.HostAlias{
				IP:        pod.Status.PodIP,
				Hostnames: []string{pod.Spec.Hostname},
			})
		}
	}

	return hostAliases
}
