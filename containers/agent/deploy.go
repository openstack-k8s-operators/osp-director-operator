package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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
		configVersion  string
		gitURL         string
		gitSSHIdentity string
	}
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.PersistentFlags().StringVar(&deployOpts.namespace, "namespace", "openstack", "Namespace to use for openstackclient pod.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.pod, "pod", "", "Pod to use for executing the deployment.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.configVersion, "configVersion", "", "Config version to use when executing the deployment.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.gitURL, "gitURL", "", "Git URL to use when downloading playbooks.")
	deployCmd.PersistentFlags().StringVar(&deployOpts.gitSSHIdentity, "gitSSHIdentity", "", "Git SSH Identity to use when downloading playbooks.")
}

// ExecPodCommand -
func ExecPodCommand(kclient kubernetes.Clientset, pod corev1.Pod, containerName string, command string) error {
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

	execErr := ExecPodCommand(*kclient, *pod, "openstackclient", "CONFIG_VERSION="+deployOpts.configVersion+" GIT_ID_RSA='"+deployOpts.gitSSHIdentity+"' GIT_URL='"+deployOpts.gitURL+"' "+"/usr/bin/bash /usr/local/bin/tripleo-deploy.sh")
	if execErr != nil {
		panic(execErr.Error())
	}

	glog.V(0).Info("Shutting down deploy agent")
}
