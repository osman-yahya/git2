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
  branches, remotes and tags, live search (`/`), full colorized patch in the details pane
- **Status view** — stage/unstage with `space`, per-file diffs, commit with `c`
- **Branches view** — local + remote branches with ahead/behind, checkout with `enter`
- **Repo picker** — launched outside a repo? Choose from recent repos or browse the
  filesystem; the last location is remembered
- **Controls that fit your hands** — arrows, WASD, or vim keys; full mouse support
  (click rows/tabs, scroll wheel); adapts to light and dark terminals

## Install

**Quick version** (macOS / Linux, needs Go 1.22+):

```sh
make install              # → /usr/local/bin/git2  (sudo if needed)
make install PREFIX=~/.local   # no-sudo alternative
```

Full platform guides, including Windows and PATH setup:

- [macOS](docs/install-macos.md)
- [Linux](docs/install-linux.md)
- [Windows](docs/install-windows.md)

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
| `← →` / `a d` / `tab` | switch pane |
| `1` `2` `3` | Commits · Status · Branches |
| `enter` | focus diff · checkout branch |
| `/` | search commits |
| `space` / `c` | stage/unstage · commit |
| `?` / `q` | help · quit |

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).
