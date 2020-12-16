package main

import (
	"flag"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	componentName = "provision-ip-discovery-agent"
)

var (
	rootCmd = &cobra.Command{
		Use:   componentName,
		Short: "Run OSP-Director-Operator Provision-Server IP Discovery Agent",
		Long:  "Runs the OSP-Director-Operator Provision-Server IP Discovery Agent to discover provisioning IP of the associated host",
	}
)

func init() {
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		glog.Exitf("Error executing mcd: %v", err)
	}
}
