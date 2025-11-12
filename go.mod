module github.com/openstack-k8s-operators/osp-director-operator

go 1.24.4

exclude k8s.io/cluster-bootstrap v0.0.0

require (
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/blang/semver/v4 v4.0.0
	github.com/ghodss/yaml v1.0.1-0.20220118164431-d8423dcdf344
	github.com/go-git/go-git/v5 v5.16.3
	github.com/go-logr/logr v1.4.3
	github.com/golang/glog v1.2.4
	github.com/google/uuid v1.6.0
	github.com/gophercloud/gophercloud/v2 v2.8.0
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.7.7
	github.com/k8snetworkplumbingwg/sriov-network-operator v0.0.0-20250828124948-002ef793cc63
	github.com/metal3-io/baremetal-operator/apis v0.0.0-20250728004541-45c6255cafc5
	github.com/nmstate/kubernetes-nmstate/api v0.0.0-20251022131709-6b5f257fffcd
	github.com/onsi/ginkgo/v2 v2.26.0
	github.com/onsi/gomega v1.38.2
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.10.1
	github.com/tidwall/gjson v1.18.0
	golang.org/x/crypto v0.44.0
	k8s.io/api v0.31.13
	k8s.io/apimachinery v0.31.13
	k8s.io/client-go v0.31.13
	k8s.io/utils v0.0.0-20250820121507-0af2bda4dd1d
	kubevirt.io/api v1.4.0
	kubevirt.io/client-go v1.4.0
	kubevirt.io/containerized-data-importer-api v1.60.3
	sigs.k8s.io/controller-runtime v0.19.4
	sigs.k8s.io/yaml v1.6.0
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/ProtonMail/go-crypto v1.1.6 // indirect
	github.com/aws/aws-sdk-go v1.44.204 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/openshift/library-go v0.0.0-20231020125025-211b32f1a1f2 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.68.0 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	k8s.io/apiserver v0.31.13 // indirect
	k8s.io/component-base v0.31.13 // indirect
	k8s.io/kube-aggregator v0.27.4 // indirect
	k8s.io/kube-openapi v0.31.0 // indirect
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.0.0-20220329064328-f3cc58c6ed90 // indirect
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ajeddeloh/go-json v0.0.0-20200220154158-5ae607161559 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clarketm/json v1.17.1 // indirect
	github.com/coreos/fcct v0.5.0 // indirect
	github.com/coreos/go-json v0.0.0-20230131223807-18775e0fb4fb // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/coreos/ign-converter v0.0.0-20230417193809-cee89ea7d8ff // indirect
	github.com/coreos/ignition v0.35.0 // indirect
	github.com/coreos/ignition/v2 v2.15.0 // indirect
	github.com/coreos/vcontext v0.0.0-20230201181013-d72178a18687 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-kit/kit v0.13.0 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.5.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/spdystream v0.4.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/openshift/api v0.0.0
	github.com/openshift/client-go v0.0.0-20230607134213-3cd0021bbee3 // indirect
	github.com/openshift/custom-resource-status v1.1.2 // indirect
	github.com/openshift/machine-config-operator v0.0.1-0.20231024085435-7e1fb719c1ba // indirect
	github.com/prometheus/client_golang v1.19.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/vincent-petithory/dataurl v1.0.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0
	go4.org v0.0.0-20200104003542-c7e774b10ea0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/apiextensions-apiserver v0.31.13 // indirect
	k8s.io/klog/v2 v2.130.1
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/kube-storage-version-migrator v0.0.6-0.20230721195810-5c8923c5ff96
	sigs.k8s.io/structured-merge-diff/v4 v4.6.0 // indirect
)

replace (
	// Downgrade gnostic-models to avoid go.yaml.in/yaml/v3 vs gopkg.in/yaml.v3 conflict
	// v0.6.8 uses gopkg.in/yaml.v3 (correct path) instead of go.yaml.in/yaml/v3
	github.com/google/gnostic-models => github.com/google/gnostic-models v0.6.8
	github.com/irifrance/gini => github.com/go-air/gini v1.0.4

	// pin to 4.18 release versions
	github.com/k8snetworkplumbingwg/sriov-network-operator => github.com/openshift/sriov-network-operator v0.0.0-20250828124948-002ef793cc63 // release-4.18
	github.com/metal3-io/baremetal-operator/apis => github.com/openshift/baremetal-operator/apis v0.0.0-20250728004541-45c6255cafc5 // release-4.18
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils => github.com/openshift/baremetal-operator/pkg/hardwareutils v0.0.0-20250728004541-45c6255cafc5 // release-4.18
	github.com/nmstate/kubernetes-nmstate/api => github.com/openshift/kubernetes-nmstate/api v0.0.0-20251022131709-6b5f257fffcd // release-4.18

	// map to latest commit from release-4.18 tag
	github.com/openshift/api => github.com/openshift/api v0.0.0-20250711200046-c86d80652a9e // release-4.18
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20241107164952-923091dd2b1a // release-4.18

	// pin to v0.31.13 (matches OpenShift 4.18 / Kubernetes 1.31)
	k8s.io/api => k8s.io/api v0.31.13
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.31.13
	k8s.io/apimachinery => k8s.io/apimachinery v0.31.13
	k8s.io/apiserver => k8s.io/apiserver v0.31.13
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.31.13
	k8s.io/client-go => k8s.io/client-go v0.31.13
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.31.13
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.31.13
	k8s.io/code-generator => k8s.io/code-generator v0.31.13
	k8s.io/component-base => k8s.io/component-base v0.31.13
	k8s.io/cri-api => k8s.io/cri-api v0.31.13
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.31.13
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.31.13
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.31.13

	// Fix kube-openapi version issue with k8s v0.31.x
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.31.13
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.31.13
	k8s.io/kubectl => k8s.io/kubectl v0.31.13
	k8s.io/kubelet => k8s.io/kubelet v0.31.13
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.31.13
	k8s.io/metrics => k8s.io/metrics v0.31.13
	k8s.io/node-api => k8s.io/node-api v0.31.13
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.31.13
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.31.13
	k8s.io/sample-controller => k8s.io/sample-controller v0.31.13

	// Pin structured-merge-diff to match k8s.io/apimachinery v0.31.13 requirement
	sigs.k8s.io/structured-merge-diff/v4 => sigs.k8s.io/structured-merge-diff/v4 v4.4.1
)
