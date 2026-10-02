package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/famarting/fws/internal/claudemem"
	"github.com/famarting/fws/internal/cmux"
	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/git"
	"github.com/famarting/fws/internal/log"
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
			log.Trace = true
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
			return removeWorkspace(cfg, sc, force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "remove even if dirty / unpushed / unmerged")
	return cmd
}

// removeWorkspace removes a feature workspace (worktree + branch + sidecar +
// cmux session), given its sidecar. Shared by `fws rm` and `fws adopt --rm`.
func removeWorkspace(cfg config.Config, sc *state.Sidecar, force bool) error {
	worktreeExists := pathExists(sc.WorktreePath)

	// Safety net for the error paths: once the working tree is gone,
	// the sidecar would otherwise hang around and make `fws rm`
	// resolve to a ghost on later invocations. We also call
	// state.Delete explicitly on the happy path before cmux.Close,
	// because closing the cmux workspace can SIGKILL this process
	// (we may be inside it) — defers don't run in that case.
	// state.Delete is idempotent so double-calling is harmless.
	defer func() {
		if pathExists(sc.WorktreePath) {
			return
		}
		if err := state.Delete(cfg.WorktreesDir, sc.Repo, sc.Name); err != nil {
			log.Warn("delete sidecar: %v", err)
		}
	}()

	// Safety checks (skipped under --force, or when the worktree
	// directory is already missing — nothing to protect against).
	if !force && worktreeExists {
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
		log.Warn("%v", err)
	}

	// Always pass --force to git: fws ran its own dirty/ahead checks
	// above, and `git worktree remove` without --force leaves partial
	// state (admin entry gone, working dir half-deleted) when its
	// recursive rmdir hits ENOTEMPTY on ignored content.
	if worktreeExists {
		if err := git.WorktreeRemove(sc.RepoPath, sc.WorktreePath, true); err != nil {
			log.Warn("git worktree remove: %v", err)
		}
	}

	// If git couldn't (or wouldn't) remove the dir — e.g. the admin
	// entry was already pruned so git says "not a working tree", or
	// rmdir hit ENOTEMPTY — finish the job at the filesystem level.
	if pathExists(sc.WorktreePath) {
		log.Info("falling back to filesystem removal of %s", sc.WorktreePath)
		if err := removeAllForce(sc.WorktreePath); err != nil {
			return fmt.Errorf("remove worktree dir: %w", err)
		}
	}

	// Garbage-collect any `.git/worktrees/<id>` admin entry left
	// behind after a partial git removal or our filesystem fallback.
	if err := git.WorktreePrune(sc.RepoPath); err != nil {
		log.Warn("git worktree prune: %v", err)
	}

	// Try to delete the branch; only force if user passed --force.
	merged, err := git.IsMerged(sc.RepoPath, sc.Branch, sc.Base)
	if err != nil {
		log.Warn("merged check: %v", err)
	}
	if merged || force {
		if err := git.BranchDelete(sc.RepoPath, sc.Branch, force); err != nil {
			log.Warn("delete branch %s: %v", sc.Branch, err)
		}
	} else {
		log.Info("kept branch %s (not merged into %s; rerun with --force to delete)", sc.Branch, sc.Base)
	}

	// Delete sidecar before closing cmux. Close-workspace can SIGKILL
	// us if we're inside the cmux workspace, which would skip our
	// defers — so do the cleanup explicitly here. The defer above
	// still covers the error paths that return before this point.
	if err := state.Delete(cfg.WorktreesDir, sc.Repo, sc.Name); err != nil {
		log.Warn("delete sidecar: %v", err)
	}
	log.Info("removed sidecar for %s/%s", sc.Repo, sc.Name)

	// Close cmux workspace last. If we're inside it, this terminates our shell.
	if sc.CmuxWorkspaceID != "" && cmux.Available() {
		if err := cmux.Close(sc.CmuxWorkspaceID); err != nil {
			log.Warn("close cmux workspace: %v", err)
		}
	}
	return nil
}

// removeAllForce is os.RemoveAll that also copes with read-only
// directories (e.g. envtest's bin/k8s/<ver>, installed 0555): unlinking
// an entry needs write permission on its parent, so on EACCES it adds
// u+w to every directory in the tree and retries.
func removeAllForce(p string) error {
	err := os.RemoveAll(p)
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return err
	}
	_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && d.IsDir() {
			if info, err := d.Info(); err == nil {
				_ = os.Chmod(path, info.Mode().Perm()|0o700)
			}
		}
		return nil
	})
	return os.RemoveAll(p)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
