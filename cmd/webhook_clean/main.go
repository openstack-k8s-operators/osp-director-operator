package main

import (
	"flag"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	componentName = "osp-director-operator-webhook-clean"
)

var (
	rootCmd = &cobra.Command{
		Use:   componentName,
		Short: "Run OSP Director Operator webhook clean-up",
		Long:  "Runs OSP Director Operator webhook clean-up to remove operator webhooks",
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
