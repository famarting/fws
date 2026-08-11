// Package claudemem invokes the claude-mem `adopt` CLI so worktree
// observations get promoted to the parent project before fws removes
// the worktree.
//
// Without this, claude-mem's built-in adoption sweep — which only fires
// at worker boot and requires `git worktree list` to still show the
// worktree — never sees worktrees that fws has already torn down.
package claudemem

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/famarting/fws/internal/log"
)

// Adopt runs `bun worker-service.cjs adopt --branch <branch> --cwd <repoPath>`.
// Returns nil if claude-mem isn't installed (caller treats that as a no-op).
// Returns an error only for unexpected failures; the caller should warn and
// continue rather than abort the rm.
func Adopt(repoPath, branch string) error {
	script := findScript()
	if script == "" {
		return nil
	}
	bun := findBun()
	if bun == "" {
		return fmt.Errorf("claude-mem present at %s but bun not found on PATH or in known locations", script)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{script, "adopt", "--branch", branch, "--cwd", repoPath}
	log.Cmd(bun, args...)
	cmd := exec.CommandContext(ctx, bun, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude-mem adopt failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
		fmt.Fprintln(os.Stderr, trimmed)
	}
	return nil
}

func findScript() string {
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".claude")
	}
	cache := filepath.Join(configDir, "plugins", "cache", "thedotmack", "claude-mem")
	entries, err := os.ReadDir(cache)
	if err != nil {
		return ""
	}
	versions := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(strings.SplitN(e.Name(), ".", 2)[0]); err != nil {
			continue
		}
		versions = append(versions, e.Name())
	}
	if len(versions) == 0 {
		return ""
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) > 0
	})
	candidate := filepath.Join(cache, versions[0], "scripts", "worker-service.cjs")
	if _, err := os.Stat(candidate); err != nil {
		return ""
	}
	return candidate
}

func findBun() string {
	if path, err := exec.LookPath("bun"); err == nil {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	candidates := []string{
		filepath.Join(home, ".bun", "bin", "bun"),
		"/opt/homebrew/bin/bun",
		"/usr/local/bin/bun",
		"/home/linuxbrew/.linuxbrew/bin/bun",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func compareSemver(a, b string) int {
	ap, bp := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(ap) || i < len(bp); i++ {
		var ai, bi int
		if i < len(ap) {
			ai, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bi, _ = strconv.Atoi(bp[i])
		}
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
	}
	return 0
}
