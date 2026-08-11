// Package log is fws's tiny stderr logger. It surfaces every external command
// fws shells out to (git, cmux, claude-mem, post-create hooks) plus high-level
// progress messages, so users can see exactly what fws is doing on their
// behalf. Output goes to stderr so command stdout (e.g. `fws resolve`) stays
// machine-readable.
package log

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	Out   io.Writer = os.Stderr
	Quiet           = false

	// Trace gates Cmd/Shell output. Off by default. State-mutating commands
	// (fws new, fws rm) flip it on so users see exactly which git/cmux/shell
	// calls fws shells out to. Read-only commands (ls, info, resolve, doctor)
	// leave it off to keep their output clean.
	Trace = false
)

// Cmd logs an external command that's about to run. Suppressed unless Trace
// is on. The first arg is the program name; remaining args are joined with
// spaces.
func Cmd(name string, args ...string) {
	if !Trace || Quiet {
		return
	}
	if len(args) == 0 {
		fmt.Fprintf(Out, "+ %s\n", name)
		return
	}
	fmt.Fprintf(Out, "+ %s %s\n", name, strings.Join(args, " "))
}

// Shell logs a `sh -c` invocation, formatted like an interactive shell prompt.
// Suppressed unless Trace is on.
func Shell(script string) {
	if !Trace || Quiet {
		return
	}
	fmt.Fprintf(Out, "$ %s\n", script)
}

// Info logs a high-level progress message ("creating cmux workspace…"). Not
// gated by Trace — these are user-facing status lines, not raw command traces.
func Info(format string, args ...any) {
	if Quiet {
		return
	}
	fmt.Fprintf(Out, format+"\n", args...)
}

// Warn always logs (Quiet does not suppress warnings).
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}
