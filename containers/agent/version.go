package main

import (
	"flag"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of OSP-Director-Operator Agent",
		Long:  `All software has versions.  This is that of the OSP-Director-Operator Agent.`,
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

	program := "OSPDirectorOperatorAgent"
	version := "v1.0.0"

	fmt.Println(program, version)
}
