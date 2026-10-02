package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/famarting/fws/internal/log"
)

func run(dir string, args ...string) (string, error) {
	log.Cmd("git", args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// TopLevel returns the absolute path to the toplevel of the git working tree at dir.
func TopLevel(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Clean(out), nil
}

// IsWorktree returns true if the toplevel at dir is a linked worktree (not the main one).
// In the main checkout, --git-dir and --git-common-dir resolve to the same path.
func IsWorktree(dir string) (bool, error) {
	commonDir, err := run(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return false, err
	}
	gitDir, err := run(dir, "rev-parse", "--git-dir")
	if err != nil {
		return false, err
	}
	absCommon, err := filepath.Abs(commonDir)
	if err != nil {
		return false, err
	}
	absGit, err := filepath.Abs(gitDir)
	if err != nil {
		return false, err
	}
	return absCommon != absGit, nil
}

// MainRepoPath returns the main checkout's toplevel directory for a linked
// worktree at dir (the parent of its `git-common-dir`).
func MainRepoPath(dir string) (string, error) {
	commonDir, err := run(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	absCommon, err := filepath.Abs(commonDir)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absCommon), nil
}

// CurrentBranch returns the branch checked out at dir, or an error if dir is
// in a detached-HEAD state.
func CurrentBranch(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if out == "HEAD" {
		return "", errors.New("in detached HEAD state, not on a branch")
	}
	return out, nil
}

// Fetch fetches a remote.
func Fetch(dir, remote string) error {
	_, err := run(dir, "fetch", remote)
	return err
}

// RevParse returns the SHA for a ref.
func RevParse(dir, ref string) (string, error) {
	return run(dir, "rev-parse", ref)
}

// RemoteHead returns the default branch a remote's HEAD points at, as a
// short ref (e.g. "origin/master"). Errors if refs/remotes/<remote>/HEAD is
// not set, which happens for remotes added after clone.
func RemoteHead(dir, remote string) (string, error) {
	return run(dir, "symbolic-ref", "--short", "refs/remotes/"+remote+"/HEAD")
}

// RemoteBranchExists returns true if <remote>/<branch> exists as a
// remote-tracking ref.
func RemoteBranchExists(dir, remote, branch string) bool {
	_, err := run(dir, "show-ref", "--verify", "--quiet", "refs/remotes/"+remote+"/"+branch)
	return err == nil
}

// BranchExists returns true if the branch exists locally.
func BranchExists(dir, branch string) bool {
	_, err := run(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// WorktreeAdd creates a new worktree with a new branch from base.
func WorktreeAdd(repoDir, worktreePath, branch, base string) error {
	_, err := run(repoDir, "worktree", "add", "-b", branch, worktreePath, base)
	return err
}

// WorktreeAddExisting attaches an existing local branch to a new worktree path.
func WorktreeAddExisting(repoDir, worktreePath, branch string) error {
	_, err := run(repoDir, "worktree", "add", worktreePath, branch)
	return err
}

// ForEachRef returns the non-empty lines produced by `git for-each-ref` for the
// given format and ref patterns. Lines are not further filtered; callers can
// strip e.g. HEAD pointers.
func ForEachRef(dir, format string, refPatterns ...string) ([]string, error) {
	args := append([]string{"for-each-ref", "--format=" + format}, refPatterns...)
	out, err := run(dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// WorktreeRemove removes a worktree.
func WorktreeRemove(repoDir, worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)
	_, err := run(repoDir, args...)
	return err
}

// WorktreePrune garbage-collects `.git/worktrees/<id>` admin entries whose
// working trees no longer exist. Safe to run unconditionally.
func WorktreePrune(repoDir string) error {
	_, err := run(repoDir, "worktree", "prune")
	return err
}

// IsDirty returns true if the working tree at dir has any modifications.
func IsDirty(dir string) (bool, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// ErrNoRemoteRef is returned by AheadBehind when the namesake remote ref
// refs/remotes/<remote>/<branch> does not exist.
var ErrNoRemoteRef = errors.New("no namesake remote ref")

// AheadBehind returns commits ahead/behind the namesake remote ref
// refs/remotes/<remote>/<branch>. Returns ErrNoRemoteRef when that ref does
// not exist.
//
// We deliberately do NOT use @{u}: fws creates branches via `worktree add -b`
// from a remote-tracking start point, which leaves @{u} pointing at the base
// branch (e.g. origin/main), not the branch's own remote ref. Comparing
// against @{u} would mis-report pushed commits as unpushed.
func AheadBehind(dir, remote, branch string) (ahead, behind int, err error) {
	remoteRef := remote + "/" + branch
	if _, err := run(dir, "rev-parse", "--verify", "--quiet", "refs/remotes/"+remoteRef); err != nil {
		return 0, 0, ErrNoRemoteRef
	}
	out, err := run(dir, "rev-list", "--left-right", "--count", remoteRef+"...HEAD")
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %q", out)
	}
	behind, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse behind count %q: %w", parts[0], err)
	}
	ahead, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse ahead count %q: %w", parts[1], err)
	}
	return ahead, behind, nil
}

// CountCommits returns the number of commits in the given revision range
// (e.g. "<sha>..HEAD").
func CountCommits(dir, rng string) (int, error) {
	out, err := run(dir, "rev-list", "--count", rng)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// IsMerged returns true if branch is fully merged into base.
func IsMerged(dir, branch, base string) (bool, error) {
	_, err := run(dir, "merge-base", "--is-ancestor", branch, base)
	if err == nil {
		return true, nil
	}
	// merge-base --is-ancestor exits 1 when the commit is not an ancestor.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// BranchDelete deletes a local branch.
func BranchDelete(dir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := run(dir, "branch", flag, branch)
	return err
}
