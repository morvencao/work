package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"

	"open-cluster-management.io/work/demo/maestrosimulator/pub"
	"open-cluster-management.io/work/demo/maestrosimulator/sub"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.AddFlags(pflag.CommandLine)
	logs.InitLogs()
	defer logs.FlushLogs()

	command := newWorkCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newWorkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "simulator",
		Short:   "Simulate the Maestro to pub/sub resource",
		Version: "0.1.0",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
			os.Exit(1)
		},
	}

	cmd.AddCommand(sub.NewSub())
	cmd.AddCommand(pub.NewPub())
	return cmd
}
