package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RepoConfig holds per-repo overrides keyed by repo name (the basename of the
// parent repo's top-level dir, e.g. "myrepo").
type RepoConfig struct {
	// PostCreate runs after `fws new` finishes wiring up the worktree, sidecar,
	// and cmux session. Each entry is passed to `sh -c` from the worktree root,
	// so shell features (cd, &&, env vars) work as expected.
	PostCreate []string `json:"post_create,omitempty"`
}

type Config struct {
	WorktreesDir string                `json:"worktrees_dir,omitempty"`
	Repos        map[string]RepoConfig `json:"repos,omitempty"`
}

// PostCreateFor returns the post-create commands configured for repo, or nil.
func (c Config) PostCreateFor(repo string) []string {
	if c.Repos == nil {
		return nil
	}
	return c.Repos[repo].PostCreate
}

// Path returns the config file path, honoring FWS_CONFIG, falling back to
// ~/.fws/config.json.
func Path() (string, error) {
	if p := os.Getenv("FWS_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fws", "config.json"), nil
}

func Load() (Config, error) {
	cfg := Config{}

	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", path, err)
		}
	case errors.Is(err, os.ErrNotExist):
		// no config file — fine, fall through with zero value
	default:
		return Config{}, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	// Precedence for worktrees dir: FWS_WORKTREES_DIR > config file > default.
	if env := os.Getenv("FWS_WORKTREES_DIR"); env != "" {
		cfg.WorktreesDir = env
	}
	if cfg.WorktreesDir == "" {
		cfg.WorktreesDir = filepath.Join(home, "projects", "feature-worktrees")
	}
	if strings.HasPrefix(cfg.WorktreesDir, "~/") {
		cfg.WorktreesDir = filepath.Join(home, cfg.WorktreesDir[2:])
	}

	if err := os.MkdirAll(cfg.WorktreesDir, 0o755); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
