package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const sep = "__"

type Sidecar struct {
	Name            string    `json:"name"`
	Repo            string    `json:"repo"`
	RepoPath        string    `json:"repo_path"`
	WorktreePath    string    `json:"worktree_path"`
	Branch          string    `json:"branch"`
	Base            string    `json:"base"`
	BaseSHA         string    `json:"base_sha"`
	CmuxWorkspaceID string    `json:"cmux_workspace_id"`
	CreatedAt       time.Time `json:"created_at"`
}

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9/-]{0,61}$`)

// NormalizeName lowercases a user-supplied name so that names differing only
// in case map to the same branch, worktree directory and sidecar. Callers
// should normalize before ValidateName.
func NormalizeName(name string) string {
	return strings.ToLower(name)
}

func ValidateName(name string) error {
	if !slugRE.MatchString(name) {
		return fmt.Errorf("invalid name %q: must be lowercase [a-z0-9/-], start with alphanumeric, max 62 chars", name)
	}
	if strings.HasSuffix(name, "/") || strings.Contains(name, "//") {
		return fmt.Errorf("invalid name %q: cannot end with or contain consecutive slashes", name)
	}
	return nil
}

// Composite returns the on-disk directory/sidecar basename for (repo, name).
// Slashes in name are escaped to '+' so storage stays flat under worktreesDir;
// the sidecar's Name field preserves the original slashes for the branch.
func Composite(repo, name string) string {
	return repo + sep + strings.ReplaceAll(name, "/", "+")
}

// IsManagedDirName reports whether name uses the "<repo>__<name>" form fws
// gives its managed worktree directories.
func IsManagedDirName(name string) bool {
	return strings.Contains(name, sep)
}

func WorktreePath(worktreesDir, repo, name string) string {
	return filepath.Join(worktreesDir, Composite(repo, name))
}

func SidecarPath(worktreesDir, repo, name string) string {
	return filepath.Join(worktreesDir, Composite(repo, name)+".json")
}

func Write(worktreesDir string, s Sidecar) error {
	p := SidecarPath(worktreesDir, s.Repo, s.Name)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func Read(path string) (Sidecar, error) {
	var s Sidecar
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.CreatedAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			s.CreatedAt = info.ModTime().UTC()
		}
	}
	return s, nil
}

func Delete(worktreesDir, repo, name string) error {
	err := os.Remove(SidecarPath(worktreesDir, repo, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// List returns all sidecars in worktreesDir, sorted by composite name.
func List(worktreesDir string) ([]Sidecar, error) {
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return nil, err
	}
	var out []Sidecar
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := Read(filepath.Join(worktreesDir, e.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "fws: warning: %v\n", err)
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// FindByCwd returns the sidecar whose worktree_path is an ancestor of cwd.
func FindByCwd(worktreesDir, cwd string) (*Sidecar, error) {
	all, err := List(worktreesDir)
	if err != nil {
		return nil, err
	}
	abs := ResolvePath(cwd)
	for i := range all {
		wp := ResolvePath(all[i].WorktreePath)
		if abs == wp || strings.HasPrefix(abs, wp+string(filepath.Separator)) {
			return &all[i], nil
		}
	}
	return nil, nil
}

// ResolvePath returns the absolute, symlink-resolved form of p. On failure at
// any step it falls back to the best result so far. Used for cwd-vs-sidecar
// comparisons, where macOS's /var -> /private/var symlink (and tmpdir
// equivalents) would otherwise cause false negatives.
func ResolvePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}
