package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	WorktreesDir string
}

func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	dir := os.Getenv("FWS_WORKTREES_DIR")
	if dir == "" {
		dir = filepath.Join(home, "projects", "feature-worktrees")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Config{}, err
	}
	return Config{WorktreesDir: dir}, nil
}
