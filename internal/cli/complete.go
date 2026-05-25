package cli

import (
	"strings"

	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/git"
	"github.com/famarting/fws/internal/state"
	"github.com/spf13/cobra"
)

// completeWorktreeNames returns names of existing worktrees for shell completion.
// If the same name exists in multiple repos, the disambiguated <repo>/<name>
// form is offered instead of the bare name.
func completeWorktreeNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	all, err := state.List(cfg.WorktreesDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	nameCounts := map[string]int{}
	for _, s := range all {
		nameCounts[s.Name]++
	}

	out := make([]string, 0, len(all))
	for _, s := range all {
		if nameCounts[s.Name] > 1 {
			out = append(out, s.Repo+"/"+s.Name+"\t"+s.Branch)
			continue
		}
		out = append(out, s.Name+"\t"+s.Repo+":"+s.Branch)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeBaseRefs offers local + origin/* refs from the parent repo.
func completeBaseRefs(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	repoPath, _, err := ResolveParentRepo(cfg)
	if err != nil {
		return []string{"origin/main", "origin/master", "main"}, cobra.ShellCompDirectiveNoFileComp
	}
	lines, err := git.ForEachRef(repoPath, "%(refname:short)", "refs/remotes/origin", "refs/heads")
	if err != nil {
		return []string{"origin/main"}, cobra.ShellCompDirectiveNoFileComp
	}
	refs := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "/HEAD") {
			continue
		}
		refs = append(refs, line)
	}
	return refs, cobra.ShellCompDirectiveNoFileComp
}
