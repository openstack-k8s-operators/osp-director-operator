module github.com/openstack-k8s-operators/osp-director-operator

go 1.13

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/go-logr/logr v0.2.1
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20201116105209-c72e2e0d8803
	github.com/nmstate/kubernetes-nmstate v0.33.0
	github.com/onsi/ginkgo v1.13.0
	github.com/onsi/gomega v1.10.1
	github.com/openshift/cluster-api v0.0.0-20191129101638-b09907ac6668
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.10.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.3
)

replace k8s.io/client-go => k8s.io/client-go v0.19.0
