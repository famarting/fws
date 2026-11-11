package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/famarting/fws/internal/cmux"
	"github.com/famarting/fws/internal/state"
)

// workspaceTitle is the cmux title fws gives a feature workspace.
func workspaceTitle(sc *state.Sidecar) string {
	return fmt.Sprintf("%s: %s", sc.Repo, sc.Name)
}

// shortenHome replaces a leading $HOME in p with "~" for display.
func shortenHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(os.PathSeparator)) {
		return "~" + p[len(home):]
	}
	return p
}

// aliveCmuxOrWarn returns the set of live cmux workspace IDs. If cmux is not
// installed or the list call fails, returns an empty map and surfaces the
// failure on stderr so commands can keep running with degraded info.
func aliveCmuxOrWarn() map[string]struct{} {
	if !cmux.Available() {
		return map[string]struct{}{}
	}
	alive, err := cmux.AliveIDs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cmux list failed: %v\n", err)
		return map[string]struct{}{}
	}
	return alive
}

// cmuxStatusFor renders a sidecar's cmux workspace state as one of
// "none" / "?" / "active" / "stale".
func cmuxStatusFor(workspaceID string, alive map[string]struct{}) string {
	if workspaceID == "" {
		return "none"
	}
	if !cmux.Available() {
		return "?"
	}
	if _, ok := alive[workspaceID]; ok {
		return "active"
	}
	return "stale"
}
