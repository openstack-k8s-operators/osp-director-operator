package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"

	"github.com/golang/glog"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/openstackdeploy"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

var (
	deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Execute an OpenStack Deploy workflow",
		Long:  "",
		Run:   runDeployCmd,
	}

	deployOpts struct {
		kubeconfig     string
		namespace      string
		pod            string
		deployName     string
		configVersion  string
		gitURL         string
		gitSSHIdentity string
		playbook       string
		limit          string
		tags           string
		skipTags       string
		ospVersion     string
	}

	openstackConfigVersionGVR = schema.GroupVersionResource{
		Group:    "osp-director.openstack.org",
		Version:  "v1beta1",
		Resource: "openstackconfigversions",
	}

	filteredAllNodesConfig = []string{
		"oslo_messaging_notify_short_bootstrap_node_name",
		"oslo_messaging_notify_node_names",
		"oslo_messaging_rpc_node_names",
		"memcached_node_ips",
		"ovn_dbs_vip",
		"redis_vip",
	}
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.PersistentFlags().StringVar(&deployOpts.namespace, "namespace", "openstack", "Namespace to use for openstackclient pod.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.pod, "pod", "", "Pod to use for executing the deployment.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.configVersion, "configVersion", "", "Config version to use when executing the deployment.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.deployName, "deployName", "", "The name of the deployment being executed. Controls the name of the generated exports ConfigMap.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.gitURL, "gitURL", "", "Git URL to use when downloading playbooks.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.gitSSHIdentity, "gitSSHIdentity", "", "Git SSH Identity to use when downloading playbooks.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.playbook, "playbook", "", "Playbook to deploy")
	deployCmd.PersistentFlags().StringVar(&deployOpts.playbook, "limit", "", "Playbook inventory limit")
	deployCmd.PersistentFlags().StringVar(&deployOpts.playbook, "tags", "", "Playbook include tags")
	deployCmd.PersistentFlags().StringVar(&deployOpts.playbook, "skipTags", "", "Playbook exclude tags")
	deployCmd.PersistentFlags().StringVar(&deployOpts.ospVersion, "ospVersion", "", "OSP release version")
}

// GetConfigMap for our exports Heat environment
func GetConfigMap(namespace string, exports string, exportsFiltered string, cephExport string, deployName string) *corev1.ConfigMap {

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      openstackdeploy.ConfigMapBasename + deployName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"ctlplane-export.yaml":          exports,
			"ctlplane-export-filtered.yaml": exportsFiltered,
			"ceph-export.yaml":              cephExport,
		},
	}

	return cm
}

// CopyFromPod -
func CopyFromPod(kclient kubernetes.Clientset, pod corev1.Pod, containerName string, filename string) (string, error) {
	req := kclient.CoreV1().RESTClient().Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		Param("container", containerName).
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false").
		Param("command", "sh")

	cfg, err := config.GetConfig()

	if err != nil {
		return "", err
	}

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return "", err
	}

	argString := fmt.Sprintf("%s\n", "/usr/bin/cat "+filename)
	argReader := strings.NewReader(argString)

	reader, writer := io.Pipe()

	go func() {
		defer writer.Close()
		err := exec.Stream(remotecommand.StreamOptions{
			Stdin:  argReader,
			Stdout: writer,
			Stderr: os.Stderr,
			Tty:    false,
		})
		if err != nil {
			panic(err)
		}
	}()
	buf := new(strings.Builder)
	num, err := io.Copy(buf, reader)
	if err != nil {
		return "", err
	}
	if num <= 0 {
		return "", nil
	}
	return buf.String(), nil
}

// ExecPodCommand -
func ExecPodCommand(kclient kubernetes.Clientset, pod corev1.Pod, containerName string, command string) error {
	//FIXME: what to do when the openstackclient exists but isn't in a running state yet?
	req := kclient.CoreV1().RESTClient().Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		Param("container", containerName).
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false").
		Param("command", "sh")

	cfg, err := config.GetConfig()

	if err != nil {
		return err
	}

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return err
	}

	argString := fmt.Sprintf("%s\n", command)
	reader := strings.NewReader(argString)

	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  io.Reader(reader),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
}

func allNodesToYaml(allNodesData string, filter bool) (string, error) {

	allNodesUnstructured := make(map[string]interface{})
	err := json.Unmarshal([]byte(allNodesData), &allNodesUnstructured)
	if err != nil {
		return "", err
	}

	filteredAllNodesUnstructured := make(map[string]interface{})
	if filter {
		for _, key := range filteredAllNodesConfig {
			if val, exists := allNodesUnstructured[key]; exists {
				filteredAllNodesUnstructured[key] = val
			}
		}
	} else {
		filteredAllNodesUnstructured = allNodesUnstructured
	}

	// convert to string
	yamlStr, err := yaml.Marshal(filteredAllNodesUnstructured)

	return string(yamlStr), err
}

func getCtlplaneExports(dClient dynamic.Interface) (string, error) {

	configVersion := dClient.Resource(openstackConfigVersionGVR)

	unstructuredConfigVersion, err := configVersion.Namespace(deployOpts.namespace).Get(context.Background(), os.Getenv("CONFIG_VERSION"), metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", unstructuredConfigVersion.Object["spec"].(map[string]interface{})["ctlplaneExports"]), nil

}

func processOvercloudJSON(kclient kubernetes.Clientset, config *rest.Config) error {
	var err error
	glog.V(0).Info("Running process Overcloud JSON.")

	// lookup the deployment pod (openstackclient)
	pod, err := kclient.CoreV1().Pods(deployOpts.namespace).Get(
		context.TODO(),
		deployOpts.pod,
		metav1.GetOptions{})
	if err != nil {
		return err
	}

	// obtain the allNodesData from the overcloud.json from the deployment pod
	// this is written by TripleO's Ansible after successful deployment
	allNodesDataString, err := CopyFromPod(kclient, *pod, "openstackclient", "/home/cloud-admin/work/"+deployOpts.configVersion+"/playbooks/tripleo-ansible/group_vars/overcloud.json")
	if err != nil {
		return err
	}

	// get the ceph export yaml
	// this is written by tripleo-deploy.sh after successful deployment
	cephExportDataString, err := CopyFromPod(kclient, *pod, "openstackclient", "/home/cloud-admin/work/"+deployOpts.configVersion+"/playbooks/ceph-export.yaml")
	if err != nil {
		return err
	}

	// get the existing CtlplaneExports from the ConfigVersion CR
	dClient := dynamic.NewForConfigOrDie(config)
	ctlplaneExport, err := getCtlplaneExports(dClient)
	if err != nil {
		return err
	}

	// we now have 2 YAML strings. Combine them into a buffer
	// noting to tab in the new data accordingly.
	var filterBuffer, buffer strings.Builder

	// filtered
	allNodesDataFiltered, err := allNodesToYaml(allNodesDataString, true)
	if err != nil {
		return err
	}
	filterBuffer.WriteString(ctlplaneExport)
	filterBuffer.WriteString("  AllNodesExtraMapData:\n")
	for _, line := range strings.Split(allNodesDataFiltered, "\n") {
		filterBuffer.WriteString(fmt.Sprintf("    %s\n", line))
	}

	// unfiltered
	allNodesData, err := allNodesToYaml(allNodesDataString, false)
	if err != nil {
		return err
	}
	buffer.WriteString(ctlplaneExport)
	buffer.WriteString("  AllNodesExtraMapData:\n")
	for _, line := range strings.Split(allNodesData, "\n") {
		buffer.WriteString(fmt.Sprintf("    %s\n", line))
	}

	// Create or Update the ConfigMap
	configMap := GetConfigMap(deployOpts.namespace, buffer.String(), filterBuffer.String(), cephExportDataString, deployOpts.deployName)
	foundConfigMap, err := kclient.CoreV1().ConfigMaps(deployOpts.namespace).Get(context.TODO(), openstackdeploy.ConfigMapBasename+deployOpts.deployName, metav1.GetOptions{})
	if err != nil && k8s_errors.IsNotFound(err) {
		_, err := kclient.CoreV1().ConfigMaps(deployOpts.namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if !reflect.DeepEqual(configMap.Data, foundConfigMap.Data) {
		foundConfigMap.Data = configMap.Data
		_, err := kclient.CoreV1().ConfigMaps(deployOpts.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil

}

func runDeployCmd(cmd *cobra.Command, args []string) {
	var err error
	err = flag.Set("logtostderr", "true")
	if err != nil {
		panic(err.Error())
	}

	flag.Parse()

	glog.V(0).Info("Running deploy command.")

	if deployOpts.namespace == "" {
		name, ok := os.LookupEnv("OSP_DIRECTOR_OPERATOR_NAMESPACE")
		if !ok || name == "" {
			glog.Fatalf("namespace is required")
		}
		deployOpts.namespace = name
	}
	if deployOpts.pod == "" {
		pod, ok := os.LookupEnv("OPENSTACKCLIENT_POD")
		if !ok || pod == "" {
			glog.Fatalf("pod is required")
		}
		deployOpts.pod = pod
	}
	if deployOpts.configVersion == "" {
		configVersion, ok := os.LookupEnv("CONFIG_VERSION")
		if !ok || configVersion == "" {
			glog.Fatalf("configVersion is required")
		}
		deployOpts.configVersion = configVersion
	}
	if deployOpts.deployName == "" {
		deployName, ok := os.LookupEnv("DEPLOY_NAME")
		if !ok || deployName == "" {
			glog.Fatalf("deployName is required")
		}
		deployOpts.deployName = deployName
	}
	if deployOpts.ospVersion == "" {
		ospVersion, ok := os.LookupEnv("OSP_VERSION")
		if !ok || ospVersion == "" {
			glog.Fatalf("ospVersion is required")
		}
		deployOpts.ospVersion = ospVersion
	}

	if deployOpts.gitURL == "" {
		gitURL, ok := os.LookupEnv("GIT_URL")
		if !ok || gitURL == "" {
			glog.Fatalf("gitURL is required")
		}
		deployOpts.gitURL = gitURL
	}

	if deployOpts.gitSSHIdentity == "" {
		gitSSHIdentity, ok := os.LookupEnv("GIT_ID_RSA")
		if !ok || gitSSHIdentity == "" {
			glog.Fatalf("gitSSHIdentity is required")
		}
		deployOpts.gitSSHIdentity = gitSSHIdentity
	}

	if deployOpts.playbook == "" {
		playbook, _ := os.LookupEnv("PLAYBOOK")
		deployOpts.playbook = playbook
	}

	if deployOpts.limit == "" {
		limit, _ := os.LookupEnv("LIMIT")
		deployOpts.limit = limit
	}

	if deployOpts.tags == "" {
		tags, _ := os.LookupEnv("TAGS")
		deployOpts.tags = tags
	}

	if deployOpts.skipTags == "" {
		skipTags, _ := os.LookupEnv("SKIP_TAGS")
		deployOpts.skipTags = skipTags
	}

	var config *rest.Config
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		panic(err.Error())
	}

	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	pod, err := kclient.CoreV1().Pods(deployOpts.namespace).Get(
		context.TODO(),
		deployOpts.pod,
		metav1.GetOptions{})

	if err != nil {
		panic(err.Error())
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM)

	go func() {
		sigTerm := <-signalChannel
		if sigTerm == syscall.SIGTERM {
			glog.V(0).Info("Terminating deploy agent")
			execErr := ExecPodCommand(
				*kclient,
				*pod,
				"openstackclient",
				"CONFIG_VERSION='"+deployOpts.configVersion+"' "+
					"GIT_ID_RSA='"+deployOpts.gitSSHIdentity+"' "+
					"GIT_URL='"+deployOpts.gitURL+"' "+
					"PLAYBOOK='"+deployOpts.playbook+"' "+
					"LIMIT='"+deployOpts.limit+"' "+
					"TAGS='"+deployOpts.tags+"' "+
					"SKIP_TAGS='"+deployOpts.skipTags+"' "+
					"OSP_VERSION='"+deployOpts.ospVersion+"' "+
					"/usr/local/bin/tripleo-deploy-term.sh")
			if execErr != nil {
				panic(execErr.Error())
			}
		}
	}()

	execErr := ExecPodCommand(
		*kclient,
		*pod,
		"openstackclient",
		"CONFIG_VERSION='"+deployOpts.configVersion+"' "+
			"GIT_ID_RSA='"+deployOpts.gitSSHIdentity+"' "+
			"GIT_URL='"+deployOpts.gitURL+"' "+
			"PLAYBOOK='"+deployOpts.playbook+"' "+
			"LIMIT='"+deployOpts.limit+"' "+
			"TAGS='"+deployOpts.tags+"' "+
			"SKIP_TAGS='"+deployOpts.skipTags+"' "+
			"OSP_VERSION='"+deployOpts.ospVersion+"' "+
			"/usr/local/bin/tripleo-deploy.sh")
	if execErr != nil {
		panic(execErr.Error())
	}

	err = processOvercloudJSON(*kclient, config)
	if err != nil {
		panic(err.Error())
	}

	glog.V(0).Info("Shutting down deploy agent")

}
