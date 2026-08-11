package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/famarting/fws/internal/cmux"
	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/git"
	"github.com/famarting/fws/internal/log"
	"github.com/famarting/fws/internal/state"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var (
		base        string
		repoFlag    string
		reuseBranch bool
		noFocus     bool
	)
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new feature workspace (git worktree + cmux session)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Trace = true
			name := args[0]
			if err := state.ValidateName(name); err != nil {
				return err
			}
			if err := validateBase(base); err != nil {
				return err
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			var repoPath, repoName string
			if repoFlag != "" {
				repoPath = repoFlag
				top, err := git.TopLevel(repoPath)
				if err != nil {
					return err
				}
				repoPath = top
				repoName = filepath.Base(top)
			} else {
				repoPath, repoName, err = ResolveParentRepo(cfg)
				if err != nil {
					return err
				}
			}

			worktreePath := state.WorktreePath(cfg.WorktreesDir, repoName, name)
			if _, err := os.Stat(worktreePath); err == nil {
				return fmt.Errorf("worktree path already exists: %s", worktreePath)
			}
			if _, err := os.Stat(state.SidecarPath(cfg.WorktreesDir, repoName, name)); err == nil {
				return fmt.Errorf("sidecar already exists for %s/%s", repoName, name)
			}
			branchExists := git.BranchExists(repoPath, name)
			switch {
			case reuseBranch && !branchExists:
				return fmt.Errorf("--reuse-branch passed but branch %q does not exist in %s (drop --reuse-branch to create it)", name, repoPath)
			case !reuseBranch && branchExists:
				return fmt.Errorf("branch %q already exists in %s (pass --reuse-branch to use it)", name, repoPath)
			}

			if err := git.Fetch(repoPath, remoteFromBase(base)); err != nil {
				return err
			}

			baseSHA, err := git.RevParse(repoPath, base)
			if err != nil {
				return fmt.Errorf("resolve base %q: %w", base, err)
			}

			if reuseBranch {
				if err := git.WorktreeAddExisting(repoPath, worktreePath, name); err != nil {
					return err
				}
			} else {
				if err := git.WorktreeAdd(repoPath, worktreePath, name, base); err != nil {
					return err
				}
			}

			var wsID string
			if cmux.Available() {
				wsID, err = cmux.NewWorkspace(worktreePath)
				if err != nil {
					// roll back the worktree so we don't leak it
					_ = git.WorktreeRemove(repoPath, worktreePath, true)
					return fmt.Errorf("create cmux workspace: %w", err)
				}
				title := fmt.Sprintf("%s: %s", repoName, name)
				if err := cmux.Rename(wsID, title); err != nil {
					log.Warn("rename workspace: %v", err)
				}
			} else {
				log.Info("cmux not available; skipping workspace (run `fws cd` inside cmux later to spawn one)")
			}

			sc := state.Sidecar{
				Name:            name,
				Repo:            repoName,
				RepoPath:        repoPath,
				WorktreePath:    worktreePath,
				Branch:          name,
				Base:            base,
				BaseSHA:         baseSHA,
				CmuxWorkspaceID: wsID,
				CreatedAt:       time.Now().UTC(),
			}
			if err := state.Write(cfg.WorktreesDir, sc); err != nil {
				return err
			}

			if hooks := cfg.PostCreateFor(repoName); len(hooks) > 0 {
				if err := runPostCreate(worktreePath, hooks); err != nil {
					log.Warn("%v", err)
				}
			}

			if !noFocus && wsID != "" {
				if err := cmux.Select(wsID); err != nil {
					log.Warn("focus workspace: %v", err)
				}
			}

			fmt.Printf("\nworktree:  %s\nbranch:    %s\nbase:      %s (%s)\n",
				worktreePath, name, base, short(baseSHA))
			if wsID != "" {
				fmt.Printf("cmux ws:   %s\n", wsID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "origin/main", "base ref to branch from")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "explicit parent repo path (default: detect from cwd)")
	cmd.Flags().BoolVar(&reuseBranch, "reuse-branch", false, "use existing branch instead of creating new one")
	cmd.Flags().BoolVar(&noFocus, "no-focus", false, "don't select the new cmux workspace")
	_ = cmd.RegisterFlagCompletionFunc("base", completeBaseRefs)
	_ = cmd.RegisterFlagCompletionFunc("repo", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
	return cmd
}

// runPostCreate executes each command via `sh -c` from worktreeDir, streaming
// stdout/stderr. Aborts on first failure so a broken hook doesn't cascade.
func runPostCreate(worktreeDir string, cmds []string) error {
	log.Info("running post-create commands in %s", worktreeDir)
	for _, c := range cmds {
		log.Shell(c)
		sh := exec.Command("sh", "-c", c)
		sh.Dir = worktreeDir
		sh.Stdout = os.Stdout
		sh.Stderr = os.Stderr
		if err := sh.Run(); err != nil {
			return fmt.Errorf("post-create command %q failed: %w", c, err)
		}
	}
	return nil
}

// remoteFromBase returns the remote portion of a base ref ("origin/main" -> "origin").
// Bare refs (no slash) fall back to "origin".
func remoteFromBase(base string) string {
	if remote, _, ok := strings.Cut(base, "/"); ok {
		return remote
	}
	return "origin"
}

// validateBase rejects values git would interpret as flags (leading '-'),
// since base flows into `git fetch`, `git rev-parse`, and `git worktree add`
// as a positional argument.
func validateBase(base string) error {
	if base == "" {
		return fmt.Errorf("--base must not be empty")
	}
	if strings.HasPrefix(base, "-") {
		return fmt.Errorf("--base %q starts with '-', which git would interpret as a flag", base)
	}
	return nil
}
