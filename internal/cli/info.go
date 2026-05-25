package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/famarting/fws/internal/config"
	"github.com/spf13/cobra"
)

func newInfoCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:               "info [name]",
		Short:             "Show details for a feature workspace (default: current)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeWorktreeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			var name string
			if len(args) == 1 {
				name = args[0]
			}
			sc, err := ResolveTarget(cfg, name)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(sc)
			}
			fmt.Printf("name:        %s\n", sc.Name)
			fmt.Printf("repo:        %s (%s)\n", sc.Repo, sc.RepoPath)
			fmt.Printf("worktree:    %s\n", sc.WorktreePath)
			fmt.Printf("branch:      %s\n", sc.Branch)
			fmt.Printf("base:        %s (%s)\n", sc.Base, short(sc.BaseSHA))
			if sc.CmuxWorkspaceID == "" {
				fmt.Printf("cmux ws:     none\n")
			} else {
				status := cmuxStatusFor(sc.CmuxWorkspaceID, aliveCmuxOrWarn())
				fmt.Printf("cmux ws:     %s [%s]\n", sc.CmuxWorkspaceID, status)
			}
			fmt.Printf("created:     %s\n", sc.CreatedAt.Local().Format("2006-01-02 15:04:05"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print raw sidecar JSON")
	return cmd
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
