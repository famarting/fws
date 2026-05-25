# fws — feature workspaces

`fws` is a small CLI that helps working on multiple things at once without
stepping on your own toes. It defines a standardized way of creating git worktree, git branches, and keeps its internal state - 
making it easy to keep track of every thing you are working on.

If you have [cmux](https://github.com/get-cmux/cmux), each workspace
also gets a cmux session, and `fws` focuses it for you.

## Install

```sh
go install github.com/famarting/fws/cmd/fws@latest
```

## Example

From inside a checked-out repo:

```sh
fws new fix-auth      # creates a worktree on a new branch, drops you in
fws ls                # lists every workspace, with dirty / ahead-behind
fws cd add-metrics    # jump to another feature you started earlier
fws rm fix-auth       # tear it down when you're done
```

Worktrees live under `~/projects/feature-worktrees/` by default
(override with `FWS_WORKTREES_DIR`).

Run `fws --help` for the full command list.

## Optional integrations

- **cmux** — if installed, `fws` focuses/closes the cmux session that
  matches each workspace.
- **claude-mem** — if installed, `fws rm` promotes the worktree's
  observations into the parent repo before deleting the worktree, so
  nothing is lost.
