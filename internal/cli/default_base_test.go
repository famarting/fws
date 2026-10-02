package cli

import (
	"os/exec"
	"testing"
)

// gitT runs git in dir, failing the test on error.
func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// upstreamWithBranch creates a repo whose only branch is the given name.
func upstreamWithBranch(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", branch)
	gitT(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

func TestDefaultBase(t *testing.T) {
	t.Run("clone follows origin/HEAD", func(t *testing.T) {
		up := upstreamWithBranch(t, "master")
		clone := t.TempDir()
		gitT(t, clone, "clone", "-q", up, ".")
		if got := defaultBase(clone); got != "origin/master" {
			t.Errorf("defaultBase = %q, want origin/master", got)
		}
	})

	t.Run("no origin/HEAD falls back to existing master", func(t *testing.T) {
		up := upstreamWithBranch(t, "master")
		repo := t.TempDir()
		gitT(t, repo, "init", "-q")
		gitT(t, repo, "remote", "add", "origin", up)
		gitT(t, repo, "fetch", "-q", "origin")
		if got := defaultBase(repo); got != "origin/master" {
			t.Errorf("defaultBase = %q, want origin/master", got)
		}
	})

	t.Run("no origin at all falls back to origin/main", func(t *testing.T) {
		repo := t.TempDir()
		gitT(t, repo, "init", "-q")
		if got := defaultBase(repo); got != "origin/main" {
			t.Errorf("defaultBase = %q, want origin/main", got)
		}
	})
}
