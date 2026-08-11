package cli

import (
	"github.com/famarting/fws/internal/log"
	"github.com/spf13/cobra"
)

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "fws",
		Short:         "fws — manage feature workspaces (git worktrees + cmux sessions)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVarP(&log.Quiet, "quiet", "q", false, "suppress action logs (warnings still print)")
	root.AddCommand(
		newNewCmd(),
		newLsCmd(),
		newRmCmd(),
		newAdoptCmd(),
		newInfoCmd(),
		newCdCmd(),
		newDoctorCmd(),
		newResolveCmd(),
	)
	return root
}
