package main

import (
	"flag"
	"fmt"

	provisionserver "github.com/openstack-k8s-operators/osp-director-operator/pkg/provisionserver"
	"github.com/spf13/cobra"
)

var (
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of OSP-Director-Operator Provision-Server IP Discovery Agent",
		Long:  `All software has versions.  This is that of the OSP-Director-Operator Provision-Server IP Discovery Agent.`,
		Run:   runVersionCmd,
	}
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersionCmd(cmd *cobra.Command, args []string) {
	err := flag.Set("logtostderr", "true")
	if err != nil {
		panic(err.Error())
	}
	flag.Parse()

	program := "ProvisionServerIpDiscoveryAgent"
	version := "v" + provisionserver.Version.String()

	fmt.Println(program, version)
}
