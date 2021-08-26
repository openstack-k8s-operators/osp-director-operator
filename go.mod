module github.com/openstack-k8s-operators/osp-director-operator

go 1.14

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.1.2
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20200626054723-37f83d1996bc
	github.com/metal3-io/baremetal-operator v0.0.0-20201116105209-c72e2e0d8803
	github.com/nmstate/kubernetes-nmstate v0.33.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/openshift/cluster-api v0.0.0-20191129101638-b09907ac6668
	github.com/openshift/sriov-network-operator v0.0.0-20201204053545-49045c36efb9
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.26.0
	github.com/spf13/cobra v1.1.3
	github.com/tidwall/gjson v1.6.5
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/tools v0.1.5 // indirect
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/apiserver v0.22.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20210707171843-4b05e18ac7d9
	kubevirt.io/client-go v0.34.2
	kubevirt.io/containerized-data-importer v1.38.0
	sigs.k8s.io/controller-runtime v0.9.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.3
	github.com/openshift/api => github.com/openshift/api v0.0.0-20210817132244-67c28690af52
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200521150516-05eb9880269c
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.18.3
	github.com/operator-framework/operator-registry => github.com/operator-framework/operator-registry v1.18.0
	google.golang.org/grpc => google.golang.org/grpc v1.33.1
	k8s.io/client-go => k8s.io/client-go v0.22.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.1
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.2
)
