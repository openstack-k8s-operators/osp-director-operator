module github.com/openstack-k8s-operators/osp-director-operator

go 1.14

exclude k8s.io/cluster-bootstrap v0.0.0

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-git/go-git/v5 v5.3.0
	github.com/go-logr/logr v0.4.0
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
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
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/tidwall/gjson v1.9.3
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/tools v0.1.7 // indirect
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/apiserver v0.21.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	kubevirt.io/client-go v0.34.2
	kubevirt.io/containerized-data-importer v1.23.5
	sigs.k8s.io/controller-runtime v0.9.2
	sigs.k8s.io/yaml v1.2.0
)

replace (
	// dependabot fixes
	github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/irifrance/gini => github.com/go-air/gini v1.0.4

	//github.com/mikefarah/yaml/v2 => gopkg.in/yaml.v2 v2.4.0

	// controller runtime
	github.com/openshift/api => github.com/openshift/api v0.0.0-20200331152225-585af27e34fd // release-4.5
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200326155132-2a6cd50aedd0 // release-4.5

	// CDI
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a

	google.golang.org/grpc => google.golang.org/grpc v1.26.0

	// pin to v0.21.2
	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/apiserver => k8s.io/apiserver v0.21.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.2
	k8s.io/client-go => k8s.io/client-go v0.21.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.2
	k8s.io/code-generator => k8s.io/code-generator v0.21.2
	k8s.io/component-base => k8s.io/component-base v0.21.2
	k8s.io/cri-api => k8s.io/cri-api v0.21.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.2
	k8s.io/kubectl => k8s.io/kubectl v0.21.2
	k8s.io/kubelet => k8s.io/kubelet v0.21.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.2
	k8s.io/metrics => k8s.io/metrics v0.21.2
	k8s.io/node-api => k8s.io/node-api v0.21.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.2
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.21.2
	k8s.io/sample-controller => k8s.io/sample-controller v0.21.2

	// pinned because no tag supports 1.18 yet
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.1-0.20191108220359-b1b620dd3f06
)
