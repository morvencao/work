package hub

import (
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"open-cluster-management.io/work/pkg/hub"
	"open-cluster-management.io/work/pkg/version"
)

// NewHubManager generates a command to start hub manager
func NewHubManager() *cobra.Command {
	o := hub.NewWorkHubManagerOptions()
	cmdConfig := controllercmd.
		NewControllerCommandConfig("work-manager", version.Get(), o.RunWorkHubManager)
	cmd := cmdConfig.NewCommand()
	cmd.Use = "manager"
	cmd.Short = "Start the Work Hub Manager"

	o.AddFlags(cmd)

	// add disable leader election flag
	flags := cmd.Flags()
	flags.BoolVar(&cmdConfig.DisableLeaderElection, "disable-leader-election", false, "Disable leader election for the manager.")

	return cmd
}
