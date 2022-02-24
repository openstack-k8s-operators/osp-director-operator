package main

import (
	"flag"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	componentName = "osp-director-agent"
)

var (
	rootCmd = &cobra.Command{
		Use:   componentName,
		Short: "Run OSP-Director-Operator Agent",
		Long:  "Runs the OSP-Director-Operator Agent",
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
