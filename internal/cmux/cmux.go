package cmux

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/famarting/fws/internal/log"
)

var (
	availableOnce sync.Once
	availableVal  bool
)

// Available reports whether the cmux binary is on PATH. The result is cached
// for the lifetime of the process, which is fine for a short-lived CLI.
func Available() bool {
	availableOnce.Do(func() {
		_, err := exec.LookPath("cmux")
		availableVal = err == nil
	})
	return availableVal
}

func run(args ...string) (string, error) {
	log.Cmd("cmux", args...)
	cmd := exec.Command("cmux", args...)
	// Silence the legacy-verb deprecation notice so it doesn't pollute
	// the error messages we build from stderr.
	cmd.Env = append(os.Environ(), "CMUX_QUIET=1")
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cmux %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

var (
	uuidRE = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	refRE  = regexp.MustCompile(`workspace:\S+`)
)

func extractUUID(s string) string { return uuidRE.FindString(s) }
func extractRef(s string) string  { return refRE.FindString(s) }

// NewWorkspace creates a new workspace cwd'd into path and returns its UUID.
// `cmux new-workspace` replies with a short ref ("OK workspace:N"), not a UUID,
// so we correlate via `cmux --id-format both list-workspaces`.
func NewWorkspace(cwd string) (string, error) {
	out, err := run("new-workspace", "--cwd", cwd)
	if err != nil {
		return "", err
	}
	ref := extractRef(out)
	if ref == "" {
		return "", fmt.Errorf("could not parse workspace ref from cmux output: %q", out)
	}
	// A busy cmux can reply with the ref before list-workspaces shows it,
	// so poll for a few seconds before giving up.
	var lastErr error
	for _, wait := range []time.Duration{0, 100, 250, 500, 1000, 2000} {
		time.Sleep(wait * time.Millisecond)
		listOut, err := run("--id-format", "both", "list-workspaces")
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = nil
		if id := uuidForRef(listOut, ref); id != "" {
			return id, nil
		}
	}
	// The caller only learns a UUID, so close the workspace by ref here or
	// it outlives the failed create.
	if _, err := run("close-workspace", "--workspace", ref); err != nil {
		log.Warn("close unresolved workspace %s: %v", ref, err)
	}
	if lastErr != nil {
		return "", fmt.Errorf("list-workspaces after create: %w", lastErr)
	}
	return "", fmt.Errorf("created workspace %s but couldn't find its UUID in list output", ref)
}

// uuidForRef returns the UUID on the list-workspaces line for ref, or "".
func uuidForRef(listOut, ref string) string {
	for _, line := range strings.Split(listOut, "\n") {
		if extractRef(line) == ref {
			return extractUUID(line)
		}
	}
	return ""
}

// Rename sets a workspace's title.
func Rename(workspaceID, title string) error {
	_, err := run("rename-workspace", "--workspace", workspaceID, title)
	return err
}

// Select focuses a workspace.
func Select(workspaceID string) error {
	_, err := run("select-workspace", "--workspace", workspaceID)
	return err
}

// Close closes a workspace. --force skips cmux's confirmation when a process
// is running in it, which is always the case when fws runs from inside it.
func Close(workspaceID string) error {
	_, err := run("close-workspace", "--workspace", workspaceID, "--force")
	return err
}

// AliveIDs returns the set of workspace UUIDs currently known to cmux.
func AliveIDs() (map[string]struct{}, error) {
	out, err := run("--id-format", "uuids", "list-workspaces")
	if err != nil {
		return nil, err
	}
	m := map[string]struct{}{}
	for _, line := range strings.Split(out, "\n") {
		if id := extractUUID(line); id != "" {
			m[id] = struct{}{}
		}
	}
	return m, nil
}

// Workspace is a live cmux workspace as reported by `cmux --json workspace list`.
type Workspace struct {
	ID          string  `json:"id"`
	CustomTitle *string `json:"custom_title"`
	Dir         string  `json:"current_directory"`
}

// List returns the live workspaces.
func List() ([]Workspace, error) {
	out, err := run("--json", "workspace", "list")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse cmux workspace list: %w", err)
	}
	return resp.Workspaces, nil
}

// Locate returns the live workspace ID for a feature workspace. cmux assigns
// new UUIDs when the app restarts, so a recorded id goes stale; when it is no
// longer live, fall back to the title fws set on creation, then (only when an
// id was recorded) to a workspace whose cwd is inside dir. Returns "" if
// nothing matches.
func Locate(id, title, dir string) (string, error) {
	wss, err := List()
	if err != nil {
		return "", err
	}
	return locate(wss, id, title, dir), nil
}

func locate(wss []Workspace, id, title, dir string) string {
	for _, w := range wss {
		if id != "" && strings.EqualFold(w.ID, id) {
			return w.ID
		}
	}
	for _, w := range wss {
		if title != "" && w.CustomTitle != nil && *w.CustomTitle == title {
			return w.ID
		}
	}
	// The cwd fallback only recovers a workspace fws once created (id != "").
	// Without a recorded id, a cwd match is just some shell that happens to
	// sit in the worktree, e.g. the one `fws adopt` was run from.
	if id != "" && dir != "" {
		dir = filepath.Clean(dir)
		for _, w := range wss {
			if w.Dir == "" {
				continue
			}
			d := filepath.Clean(w.Dir)
			if d == dir || strings.HasPrefix(d, dir+string(filepath.Separator)) {
				return w.ID
			}
		}
	}
	return ""
}

// Send types text into a workspace's focused terminal surface. cmux interprets
// "\n" in the text as Enter, so a trailing newline runs the command.
func Send(workspaceID, text string) error {
	_, err := run("send", "--workspace", workspaceID, "--", text)
	return err
}
