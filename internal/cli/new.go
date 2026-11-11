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
		Use:   "new <name> [-- <command> [args...]]",
		Short: "Create a new feature workspace (git worktree + cmux session)",
		Long: "Create a new feature workspace (git worktree + cmux session).\n\n" +
			"Anything after `--` is run in the new cmux workspace once setup finishes:\n\n" +
			`  fws new foobaz -- claude "do xyz"`,
		Args: func(cmd *cobra.Command, args []string) error {
			n := len(args)
			if d := cmd.ArgsLenAtDash(); d >= 0 {
				n = d
			}
			if n != 1 {
				return fmt.Errorf("accepts 1 arg before `--`, received %d", n)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Trace = true
			name := state.NormalizeName(args[0])
			var initCmd []string
			if d := cmd.ArgsLenAtDash(); d >= 0 {
				initCmd = args[d:]
			}
			if err := state.ValidateName(name); err != nil {
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

			if !cmd.Flags().Changed("base") {
				base = defaultBase(repoPath)
			}
			if err := validateBase(base); err != nil {
				return err
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

			// rollback undoes whatever this command has created so far, so a
			// failed create leaves no worktree, branch, or cmux workspace
			// behind. A reused branch predates us and is never deleted.
			var wsID string
			rollback := func() {
				if wsID != "" {
					if err := cmux.Close(wsID); err != nil {
						log.Warn("rollback: close cmux workspace: %v", err)
					}
				}
				if pathExists(worktreePath) {
					if err := git.WorktreeRemove(repoPath, worktreePath, true); err != nil {
						log.Warn("rollback: git worktree remove: %v", err)
					}
				}
				if err := git.WorktreePrune(repoPath); err != nil {
					log.Warn("rollback: git worktree prune: %v", err)
				}
				if !reuseBranch && git.BranchExists(repoPath, name) {
					if err := git.BranchDelete(repoPath, name, true); err != nil {
						log.Warn("rollback: delete branch %s: %v", name, err)
					}
				}
			}

			if reuseBranch {
				if err := git.WorktreeAddExisting(repoPath, worktreePath, name); err != nil {
					rollback()
					return err
				}
			} else {
				// `worktree add -b` creates the branch before checking it
				// out, so a failed checkout can still leave the branch.
				if err := git.WorktreeAdd(repoPath, worktreePath, name, base); err != nil {
					rollback()
					return err
				}
			}

			if cmux.Available() {
				wsID, err = cmux.NewWorkspace(worktreePath)
				if err != nil {
					rollback()
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
				rollback()
				return err
			}

			if hooks := cfg.PostCreateFor(repoName); len(hooks) > 0 {
				if err := runPostCreate(worktreePath, hooks); err != nil {
					log.Warn("%v", err)
				}
			}

			// Sent last so it lands after post-create hooks have finished
			// setting the worktree up.
			if len(initCmd) > 0 {
				if wsID == "" {
					log.Warn("command after `--` ignored: no cmux workspace was created")
				} else if err := cmux.Send(wsID, shellJoin(initCmd)+"\n"); err != nil {
					log.Warn("run %q: %v", initCmd[0], err)
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
	cmd.Flags().StringVar(&base, "base", "", "base ref to branch from (default: origin's default branch, e.g. origin/main)")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "explicit parent repo path (default: detect from cwd)")
	cmd.Flags().BoolVar(&reuseBranch, "reuse-branch", false, "use existing branch instead of creating new one")
	cmd.Flags().BoolVar(&noFocus, "no-focus", false, "don't select the new cmux workspace")
	_ = cmd.RegisterFlagCompletionFunc("base", completeBaseRefs)
	_ = cmd.RegisterFlagCompletionFunc("repo", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
	return cmd
}

// shellJoin renders argv as a single shell command line, single-quoting any
// argument that isn't plainly safe so the workspace's shell sees the same words
// this process was given.
func shellJoin(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-./:=@,+", r)
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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

// defaultBase returns the base ref to use when --base isn't given: whatever
// origin/HEAD points at, else origin/main or origin/master if present, else
// origin/main. Only local refs are consulted; the caller fetches afterwards.
func defaultBase(repoPath string) string {
	if ref, err := git.RemoteHead(repoPath, "origin"); err == nil && ref != "" {
		return ref
	}
	for _, b := range []string{"main", "master"} {
		if git.RemoteBranchExists(repoPath, "origin", b) {
			return "origin/" + b
		}
	}
	return "origin/main"
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
