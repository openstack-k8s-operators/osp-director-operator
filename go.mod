module github.com/openstack-k8s-operators/osp-director-operator

go 1.13

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v3.9.0+incompatible
	github.com/openstack-k8s-operators/cinder-operator v0.0.0-20201021080935-bea86fce1acd
	github.com/openstack-k8s-operators/lib-common v0.0.0-20201012132655-247b83b2fafa
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.7.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
)
