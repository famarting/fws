package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/famarting/fws/internal/cmux"
	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/state"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Detect (and optionally fix) drift between sidecars, git worktrees, and cmux",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			scs, err := state.List(cfg.WorktreesDir)
			if err != nil {
				return err
			}
			alive := aliveCmuxOrWarn()

			// 1. Orphan sidecars (worktree dir gone)
			for _, s := range scs {
				if _, err := os.Stat(s.WorktreePath); errors.Is(err, os.ErrNotExist) {
					fmt.Printf("orphan sidecar (worktree gone): %s\n", state.SidecarPath(cfg.WorktreesDir, s.Repo, s.Name))
					if fix {
						if err := state.Delete(cfg.WorktreesDir, s.Repo, s.Name); err != nil {
							fmt.Fprintf(os.Stderr, "  delete failed: %v\n", err)
						} else {
							fmt.Println("  -> deleted")
						}
					}
				}
			}

			// 2. Orphan worktree dirs (no sidecar)
			entries, err := os.ReadDir(cfg.WorktreesDir)
			if err != nil {
				return err
			}
			knownDirs := map[string]struct{}{}
			for _, s := range scs {
				knownDirs[filepath.Base(s.WorktreePath)] = struct{}{}
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if _, ok := knownDirs[e.Name()]; ok {
					continue
				}
				if !state.IsManagedDirName(e.Name()) {
					continue
				}
				orphanPath := filepath.Join(cfg.WorktreesDir, e.Name())
				fmt.Printf("orphan worktree (no sidecar): %s\n", orphanPath)
				if fix {
					fmt.Printf("  -> not auto-removed; run `fws adopt %s` then `fws rm` to bring it under management (or remove it manually)\n", orphanPath)
				}
			}

			// 3. Stale cmux IDs (skip entries without a bound workspace)
			if cmux.Available() {
				for _, s := range scs {
					if s.CmuxWorkspaceID == "" {
						continue
					}
					if _, ok := alive[s.CmuxWorkspaceID]; ok {
						continue
					}
					fmt.Printf("stale cmux workspace id for %s/%s: %s (use `fws cd %s` to respawn)\n",
						s.Repo, s.Name, s.CmuxWorkspaceID, s.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "auto-remove orphan sidecars")
	return cmd
}
