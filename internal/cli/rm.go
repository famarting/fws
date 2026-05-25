package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/famarting/fws/internal/claudemem"
	"github.com/famarting/fws/internal/cmux"
	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/git"
	"github.com/famarting/fws/internal/state"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:               "rm [name]",
		Short:             "Remove a feature workspace (worktree + cmux session)",
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

			// Safety checks (skipped under --force).
			if !force {
				dirty, err := git.IsDirty(sc.WorktreePath)
				if err != nil {
					return fmt.Errorf("check status: %w", err)
				}
				if dirty {
					return fmt.Errorf("worktree %s is dirty (commit/stash or pass --force)", sc.WorktreePath)
				}
				remote := remoteFromBase(sc.Base)
				ahead, _, err := git.AheadBehind(sc.WorktreePath, remote, sc.Branch)
				switch {
				case errors.Is(err, git.ErrNoRemoteRef):
					// Branch has never been pushed. Fall back to comparing
					// against the recorded base SHA so a brand-new branch
					// with no commits beyond base passes silently.
					if sc.BaseSHA != "" {
						n, cerr := git.CountCommits(sc.WorktreePath, sc.BaseSHA+"..HEAD")
						if cerr != nil {
							return fmt.Errorf("count commits since base: %w", cerr)
						}
						if n > 0 {
							return fmt.Errorf("branch %s has %d unpushed commit(s) since base (push or pass --force)", sc.Branch, n)
						}
					}
				case err != nil:
					return fmt.Errorf("check ahead/behind: %w", err)
				case ahead > 0:
					return fmt.Errorf("branch %s has %d unpushed commit(s) (push or pass --force)", sc.Branch, ahead)
				}
			}

			// Adopt claude-mem observations into the parent project before the
			// worktree disappears. claude-mem's auto-adopt only fires at worker
			// boot and needs `git worktree list` to still show the worktree, so
			// without this hook the observations stay siloed forever.
			if err := claudemem.Adopt(sc.RepoPath, sc.Branch); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}

			fmt.Printf("removing worktree %s...\n", sc.WorktreePath)
			if err := git.WorktreeRemove(sc.RepoPath, sc.WorktreePath, force); err != nil {
				return fmt.Errorf("git worktree remove: %w", err)
			}

			// Try to delete the branch; only force if user passed --force.
			merged, err := git.IsMerged(sc.RepoPath, sc.Branch, sc.Base)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: merged check: %v\n", err)
			}
			if merged || force {
				if err := git.BranchDelete(sc.RepoPath, sc.Branch, force); err != nil {
					fmt.Fprintf(os.Stderr, "warning: delete branch %s: %v\n", sc.Branch, err)
				} else {
					fmt.Printf("deleted branch %s\n", sc.Branch)
				}
			} else {
				fmt.Printf("kept branch %s (not merged into %s; rerun with --force to delete)\n", sc.Branch, sc.Base)
			}

			// Delete sidecar before closing cmux (in case close-workspace kills us).
			if err := state.Delete(cfg.WorktreesDir, sc.Repo, sc.Name); err != nil {
				fmt.Fprintf(os.Stderr, "warning: delete sidecar: %v\n", err)
			}

			// Close cmux workspace last. If we're inside it, this terminates our shell.
			if sc.CmuxWorkspaceID != "" && cmux.Available() {
				fmt.Printf("closing cmux workspace %s\n", sc.CmuxWorkspaceID)
				if err := cmux.Close(sc.CmuxWorkspaceID); err != nil {
					fmt.Fprintf(os.Stderr, "warning: close cmux workspace: %v\n", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "remove even if dirty / unpushed / unmerged")
	return cmd
}
