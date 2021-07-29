module github.com/openstack-k8s-operators/osp-director-operator

go 1.13

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/blang/semver v3.5.1+incompatible
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32 // indirect
	github.com/go-logr/logr v0.3.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20200626054723-37f83d1996bc
	github.com/metal3-io/baremetal-operator v0.0.0-20201116105209-c72e2e0d8803
	github.com/nmstate/kubernetes-nmstate v0.33.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/openshift/cluster-api v0.0.0-20191129101638-b09907ac6668
	github.com/openshift/sriov-network-operator v0.0.0-20201204053545-49045c36efb9
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.10.0
	github.com/spf13/cobra v1.1.1
	github.com/tidwall/gjson v1.6.5
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/tools v0.1.5 // indirect
	google.golang.org/grpc v1.31.0 // indirect
	google.golang.org/grpc/examples v0.0.0-20210512000516-62adda2ece5e // indirect
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/apiserver v0.19.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800
	kubevirt.io/client-go v0.34.2
	kubevirt.io/containerized-data-importer v1.23.5
	sigs.k8s.io/controller-runtime v0.7.2
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20200526144822-34f54f12813a
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200521150516-05eb9880269c
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.17.0
	google.golang.org/grpc => google.golang.org/grpc v1.26.0
	k8s.io/client-go => k8s.io/client-go v0.19.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.3
	kubevirt.io/client-go => kubevirt.io/client-go v0.34.2
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.1
)
