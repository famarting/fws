# fws — feature workspaces

`fws` is a small CLI that helps working on multiple things at once without
stepping on your own toes. It defines a standardized way of creating git worktrees and git branches, and tracks them internally -
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

Anything after `--` runs in the new cmux workspace once setup finishes:

```sh
fws new fix-auth -- claude "fix the token refresh race in auth/session.go"
```

Worktrees live under `~/projects/feature-worktrees/` by default
(override with `FWS_WORKTREES_DIR`).

Run `fws --help` for the full command list.

For state-mutating commands (`fws new`, `fws rm`), `fws` traces every
external command it shells out to (git, cmux, claude-mem, `post_create`
hooks) to stderr with a `+ ` prefix, so you can see what it's doing on
your behalf. Read-only commands (`ls`, `info`, `resolve`, `cd`, `doctor`)
stay quiet. Pass `-q` / `--quiet` to suppress.

## Config

`fws` reads an optional config file from `~/.fws/config.json`
(override with `$FWS_CONFIG`).

```json
{
  "worktrees_dir": "~/projects/feature-worktrees",
  "repos": {
    "myrepo": {
      "post_create": [
        "cd web && npm i"
      ]
    }
  }
}
```

- `worktrees_dir` — where worktrees live. `FWS_WORKTREES_DIR` wins if set.
- `repos.<name>.post_create` — commands run after `fws new` finishes
  setting up a workspace for that repo. The key matches the parent repo's
  directory name. Each command runs via `sh -c` from the worktree root, so
  `cd`, `&&`, and env vars all work. The first failure aborts the chain.

## Optional integrations

- **cmux** — if installed, `fws` focuses/closes the cmux session that
  matches each workspace.
- **claude-mem** — if installed, `fws rm` promotes the worktree's
  observations into the parent repo before deleting the worktree, so
  nothing is lost.
