# git2

A beautiful terminal git client, inspired by [Fork](https://git-fork.com/). Type `git2` in any
terminal: inside a repository it opens straight into a Fork-style commit graph; anywhere else
it offers your recent repos and a directory browser to pick one.

```
●     [main] start v1.1 work
○─╮   (v1.0) Merge branch 'feature/ui'
● │   post-merge cleanup
│ ●   [feature/ui] ui: new sidebar
○─┼─╮ Merge branch 'feature/login'
●─╯ │ main: fix typo
│   ● [feature/login] login: validation
│   ● login: add form
●───╯ add feature scaffolding
●     initial commit
```

## Features

- **Commit graph** — colored branch lanes with rounded connectors, ref badges for HEAD,
  branches, remotes and tags (slash names like `dev/main` handled correctly), live search
  (`/`); the right column splits into a metadata card and a tinted-background diff pane
- **Commit actions** — checkout, merge, cherry-pick, rebase, revert, branch right from the
  graph; blocked by local changes? A popup offers stash-and-reapply or discard. `b` switches
  branch in place, `t` focuses the graph on the current branch
- **Status view** — file tree with clean per-file diffs, stage/unstage whole files or
  **individual hunks**, discard (`D`), amend (`A`), per-file history (`H`) and blame (`B`),
  stashes gathered at the bottom
- **Conflict resolve panel** — a conflicting merge jumps straight to Status: per-file
  *ours / theirs / fixed manually* popup, highlighted markers, commit-merge or abort;
  merges blocked by uncommitted changes get a stash-and-retry popup
- **Branches view** — local + remote branches with ahead/behind, checkout with `enter`,
  create/rename/delete (`n`/`e`/`x`), merge with `m`, open a pull-request page with `O`
- **Tags** — create or delete tags right on the graph (`T`)
- **Remotes & syncing** — add origin from the TUI, fetch (`f`) with autofetch every 3 min,
  ff-only pull, push that auto-creates the remote branch, force-push with lease; auth rides
  on your existing SSH keys / credential helper ([details](docs/remotes.md))
- **Stashes** — stash with untracked files (`S`), preview diffs, apply / pop / drop
- **Repo picker** — launched outside a repo? Choose from recent repos, browse the
  filesystem, clone a URL (`c`) or init a new repo (`i`); the last location is remembered
- **Controls that fit your hands** — arrows, WASD, or vim keys; full mouse support
  (click rows/tabs, clickable popups, double-click actions, scroll wheel); GUI-style
  theme with filled tabs, section bands and tinted diffs; light & dark terminals

## Install

**One line, no Go required** — grabs the latest release binary and puts `git2` on your PATH:

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/osman-yahya/git2/main/install.sh | sh
```

```powershell
# Windows (PowerShell) — installs to %LOCALAPPDATA%\Programs\git2 and adds it to PATH
irm https://raw.githubusercontent.com/osman-yahya/git2/main/install.ps1 | iex
```

With Go 1.22+ you can also `go install github.com/osman-yahya/git2@latest`,
or build from a checkout with `make install` (`PREFIX=~/.local` for no sudo).
Update later with `git2 update`.

Full platform guides, including manual installs and PATH setup:

- [macOS](docs/install-macos.md)
- [Linux](docs/install-linux.md)
- [Windows](docs/install-windows.md)
- [Remotes & authentication](docs/remotes.md)

Cross-compiled binaries for all five targets: `make release` → `dist/`.

## Usage

```sh
git2              # open the repo containing the current directory
git2 ~/code/app   # open a specific repo
git2 --print      # print the commit graph and exit (no TUI)
```

See the **[usage guide](docs/usage.md)** for the picker, views, and every keybinding.

### Keys at a glance

| Key | Action |
| --- | --- |
| `↑ ↓` / `w s` / `j k` | move · scroll |
| `tab` | next view |
| `a` / `d` | focus list ↔ details pane |
| `1` `2` `3` `4` | Commits · Status · Branches · Stashes |
| `enter` | focus diff · checkout branch |
| `c` / `y` / `R` / `v` | checkout · pick · rebase · revert commit |
| `O` | open PR in browser |
| `/` | search commits |
| `space` / `c` / `S` | stage/unstage · commit · stash |
| `f` / `p` / `P` | fetch · pull · push |
| `?` / `q` | help · quit |

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).
