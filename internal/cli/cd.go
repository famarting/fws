package cli

import (
	"fmt"

	"github.com/famarting/fws/internal/cmux"
	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/state"
	"github.com/spf13/cobra"
)

func newCdCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "cd <name>",
		Short:             "Focus the cmux session for a feature workspace (heals stale entries)",
		Args:              cobra.ExactArgs(1),
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
			if !cmux.Available() {
				return fmt.Errorf("cmux is not installed; `fws cd` requires cmux. worktree path: %s", sc.WorktreePath)
			}
			title := workspaceTitle(sc)
			live, err := cmux.Locate(sc.CmuxWorkspaceID, title, sc.WorktreePath)
			if err != nil {
				return fmt.Errorf("cmux list: %w", err)
			}
			if live != "" {
				if live != sc.CmuxWorkspaceID {
					// cmux re-keyed it (e.g. after an app restart): record the new id.
					sc.CmuxWorkspaceID = live
					if err := state.Write(cfg.WorktreesDir, *sc); err != nil {
						return err
					}
				}
				return cmux.Select(live)
			}
			// No live workspace: spawn one and patch the sidecar.
			if sc.CmuxWorkspaceID == "" {
				fmt.Printf("no cmux workspace for %s yet; creating one...\n", sc.Name)
			} else {
				fmt.Printf("cmux workspace %s is stale; respawning...\n", sc.CmuxWorkspaceID)
			}
			id, err := cmux.NewWorkspace(sc.WorktreePath)
			if err != nil {
				return err
			}
			if err := cmux.Rename(id, title); err != nil {
				fmt.Printf("warning: rename: %v\n", err)
			}
			sc.CmuxWorkspaceID = id
			if err := state.Write(cfg.WorktreesDir, *sc); err != nil {
				return err
			}
			return cmux.Select(id)
		},
	}
}
