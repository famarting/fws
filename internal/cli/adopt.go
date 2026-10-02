package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/git"
	"github.com/famarting/fws/internal/log"
	"github.com/famarting/fws/internal/state"
	"github.com/spf13/cobra"
)

func newAdoptCmd() *cobra.Command {
	var (
		name    string
		base    string
		rmAfter bool
		force   bool
	)
	cmd := &cobra.Command{
		Use:   "adopt [path]",
		Short: "Write an fws sidecar for an existing git worktree (default: cwd) so `fws rm`/`fws cd` can manage it, or pass --rm to clean it up right away",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Trace = true
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if info, err := os.Stat(absPath); err != nil {
				return fmt.Errorf("stat %s: %w", absPath, err)
			} else if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", absPath)
			}

			isWT, err := git.IsWorktree(absPath)
			if err != nil {
				return fmt.Errorf("%s doesn't look like a git worktree: %w", absPath, err)
			}
			if !isWT {
				return fmt.Errorf("%s is a main checkout, not a linked worktree", absPath)
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Refuse if some other sidecar already tracks this exact worktree.
			all, err := state.List(cfg.WorktreesDir)
			if err != nil {
				return err
			}
			resolvedPath := state.ResolvePath(absPath)
			for _, s := range all {
				if state.ResolvePath(s.WorktreePath) == resolvedPath {
					return fmt.Errorf("worktree already tracked as %s/%s", s.Repo, s.Name)
				}
			}

			repoPath, err := git.MainRepoPath(absPath)
			if err != nil {
				return fmt.Errorf("resolve main repo: %w", err)
			}
			repoName := filepath.Base(repoPath)

			branch, err := git.CurrentBranch(absPath)
			if err != nil {
				return fmt.Errorf("resolve branch: %w", err)
			}

			if name == "" {
				name = branch
			}
			name = state.NormalizeName(name)
			if err := state.ValidateName(name); err != nil {
				return err
			}

			if !cmd.Flags().Changed("base") {
				base = defaultBase(repoPath)
			}
			if err := validateBase(base); err != nil {
				return err
			}
			if _, err := os.Stat(state.SidecarPath(cfg.WorktreesDir, repoName, name)); err == nil {
				return fmt.Errorf("sidecar already exists for %s/%s", repoName, name)
			}
			if err := git.Fetch(repoPath, remoteFromBase(base)); err != nil {
				return err
			}
			baseSHA, err := git.RevParse(repoPath, base)
			if err != nil {
				return fmt.Errorf("resolve base %q: %w", base, err)
			}

			sc := state.Sidecar{
				Name:         name,
				Repo:         repoName,
				RepoPath:     repoPath,
				WorktreePath: absPath,
				Branch:       branch,
				Base:         base,
				BaseSHA:      baseSHA,
				CreatedAt:    time.Now().UTC(),
			}
			if err := state.Write(cfg.WorktreesDir, sc); err != nil {
				return err
			}

			fmt.Printf("adopted:   %s\nname:      %s\nrepo:      %s (%s)\nbranch:    %s\nbase:      %s (%s)\n",
				absPath, name, repoName, repoPath, branch, base, short(baseSHA))

			if rmAfter {
				return removeWorkspace(cfg, &sc, force)
			}
			fmt.Println("\nno cmux workspace bound yet; run `fws cd " + name + "` to spawn one")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "sidecar name (default: current branch name)")
	cmd.Flags().StringVar(&base, "base", "", "base ref to record for ahead/behind + merge checks (default: origin's default branch, e.g. origin/main)")
	cmd.Flags().BoolVar(&rmAfter, "rm", false, "immediately remove the workspace after adopting it (worktree + branch + sidecar)")
	cmd.Flags().BoolVar(&force, "force", false, "with --rm, remove even if dirty / unpushed / unmerged")
	_ = cmd.RegisterFlagCompletionFunc("base", completeBaseRefs)
	return cmd
}
