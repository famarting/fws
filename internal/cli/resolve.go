package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/git"
	"github.com/famarting/fws/internal/state"
)

// ResolveParentRepo returns the path + repo name of the parent (main checkout) for cwd.
// If cwd is itself one of our worktrees, dereferences via its sidecar.
func ResolveParentRepo(cfg config.Config) (repoPath, repoName string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	// First: are we inside a tracked worktree? Sidecar wins.
	sc, err := state.FindByCwd(cfg.WorktreesDir, cwd)
	if err != nil {
		return "", "", err
	}
	if sc != nil {
		return sc.RepoPath, sc.Repo, nil
	}
	// Otherwise: resolve cwd's git toplevel. Refuse if it's a worktree we don't know about.
	top, err := git.TopLevel(cwd)
	if err != nil {
		return "", "", fmt.Errorf("not inside a git repo and not inside a known worktree: %w", err)
	}
	isWT, err := git.IsWorktree(top)
	if err != nil {
		return "", "", err
	}
	if isWT {
		return "", "", errors.New("cwd is a linked worktree but not registered with fws. cd into the main checkout and try again")
	}
	return top, filepath.Base(top), nil
}

// ResolveCurrent returns the sidecar for the worktree implied by context:
//  1. cwd inside a worktree path
//  2. cmux workspace ID match
//  3. error
func ResolveCurrent(cfg config.Config) (*state.Sidecar, error) {
	all, err := state.List(cfg.WorktreesDir)
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	abs := state.ResolvePath(cwd)
	for i := range all {
		wp := state.ResolvePath(all[i].WorktreePath)
		if abs == wp || strings.HasPrefix(abs, wp+string(filepath.Separator)) {
			return &all[i], nil
		}
	}
	if id := os.Getenv("CMUX_WORKSPACE_ID"); id != "" {
		for i := range all {
			if all[i].CmuxWorkspaceID == id {
				return &all[i], nil
			}
		}
	}
	return nil, errors.New("not inside an fws-managed feature workspace (no cwd match, no CMUX_WORKSPACE_ID match)")
}

// ResolveTarget resolves the sidecar for a [name] argument: if name is empty,
// use ResolveCurrent. Otherwise name may be the bare workspace name, a
// "<repo>/<name>" qualifier, the worktree dir name ("<repo>__<name>"), or the
// worktree path.
func ResolveTarget(cfg config.Config, name string) (*state.Sidecar, error) {
	if name == "" {
		return ResolveCurrent(cfg)
	}
	all, err := state.List(cfg.WorktreesDir)
	if err != nil {
		return nil, err
	}
	// Exact identifiers first: a worktree path or its dir name is unambiguous.
	abs := state.ResolvePath(name)
	for i := range all {
		wp := all[i].WorktreePath
		if state.ResolvePath(wp) == abs || filepath.Base(wp) == name {
			return &all[i], nil
		}
	}
	var matches []state.Sidecar
	for _, s := range all {
		if s.Name == name {
			matches = append(matches, s)
		}
	}
	// No bare-name match: try "<repo>/<name>". Names may themselves contain
	// "/", so only the first segment is treated as the repo.
	if len(matches) == 0 {
		if repo, rest, ok := strings.Cut(name, "/"); ok {
			for _, s := range all {
				if s.Repo == repo && s.Name == rest {
					matches = append(matches, s)
				}
			}
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no worktree named %q", name)
	case 1:
		return &matches[0], nil
	default:
		var repos []string
		for _, m := range matches {
			repos = append(repos, m.Repo)
		}
		return nil, fmt.Errorf("ambiguous: worktree %q exists in multiple repos: %v (use <repo>/%s)", name, repos, name)
	}
}
