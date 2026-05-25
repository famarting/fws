package cli

import (
	"fmt"

	"github.com/famarting/fws/internal/config"
	"github.com/spf13/cobra"
)

func newResolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "resolve <name>",
		Short:             "Print the absolute path of a feature workspace's worktree",
		Long:              "Prints the absolute filesystem path of a feature workspace's git worktree, one line, no trailing fluff. Designed for shell substitution: `some-tool --folder $(fws resolve my-feature) ...`. Exits non-zero if not found or ambiguous across repos.",
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completeWorktreeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			sc, err := ResolveTarget(cfg, args[0])
			if err != nil {
				return err
			}
			fmt.Println(sc.WorktreePath)
			return nil
		},
	}
	return cmd
}
