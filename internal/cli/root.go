package cli

import "github.com/spf13/cobra"

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "fws",
		Short:         "fws — manage feature workspaces (git worktrees + cmux sessions)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newNewCmd(),
		newLsCmd(),
		newRmCmd(),
		newInfoCmd(),
		newCdCmd(),
		newDoctorCmd(),
		newResolveCmd(),
	)
	return root
}
