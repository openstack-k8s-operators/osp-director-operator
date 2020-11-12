module github.com/openstack-k8s-operators/osp-director-operator

go 1.13

require (
	github.com/go-logr/logr v0.2.1
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20201107165446-c65c1ac8ddad
	github.com/onsi/ginkgo v1.13.0
	github.com/onsi/gomega v1.10.1
	github.com/openshift/cluster-api v0.0.0-20191129101638-b09907ac6668
	github.com/prometheus/client_golang v1.7.1 // indirect
	github.com/stretchr/testify v1.6.1 // indirect
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111 // indirect
	sigs.k8s.io/controller-runtime v0.6.2
)

replace k8s.io/client-go => k8s.io/client-go v0.19.0
