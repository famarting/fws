package cmux

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"

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
	listOut, err := run("--id-format", "both", "list-workspaces")
	if err != nil {
		return "", fmt.Errorf("list-workspaces after create: %w", err)
	}
	for _, line := range strings.Split(listOut, "\n") {
		if extractRef(line) != ref {
			continue
		}
		if id := extractUUID(line); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("created workspace %s but couldn't find its UUID in list output", ref)
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

// Close closes a workspace.
func Close(workspaceID string) error {
	_, err := run("close-workspace", "--workspace", workspaceID)
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
