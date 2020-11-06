module github.com/abays/osp-director-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openstack-k8s-operators/cinder-operator v0.0.0-20201021080935-bea86fce1acd
	github.com/openstack-k8s-operators/lib-common v0.0.0-20201012132655-247b83b2fafa
	github.com/openstack-k8s-operators/osp-director-operator v0.0.0-20201103130023-3fcc0835e23e
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
)
