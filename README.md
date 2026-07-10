# git2

A beautiful terminal git client, inspired by [Fork](https://git-fork.com/). Point it at any
directory and it finds the enclosing repository and opens a fast, keyboard-driven UI with a
Fork-style commit graph.

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
  branches, remotes and tags, author/date/hash metadata, live search (`/`)
- **Details pane** — full commit info, diffstat and syntax-colored patch
- **Status view** — staged/unstaged/untracked files, per-file diff, stage/unstage with
  `space`, commit with `c`
- **Branches view** — local + remote branches sorted by recency with ahead/behind tracking,
  per-branch history, checkout with `enter`
- **Mouse support** — click rows and tabs, scroll with the wheel
- Auto-detects the repository from the current directory (or any path you pass)

## Install

```sh
go build -o git2 .
mv git2 /usr/local/bin/   # or anywhere on your PATH
```

## Usage

```sh
git2              # open the repo containing the current directory
git2 ~/code/app   # open a specific repo
git2 --print      # print the commit graph and exit (no TUI)
```

## Keys

| Key | Action |
| --- | --- |
| `1` `2` `3` | Commits · Status · Branches |
| `tab` / `←` `→` | switch pane focus |
| `j` `k` / `↑` `↓` | move / scroll |
| `ctrl+d` `ctrl+u` | half-page down / up |
| `g` / `G` | top / bottom |
| `enter` | focus diff · checkout branch |
| `/` | search commits |
| `space` | stage / unstage file |
| `c` | commit staged changes |
| `r` | refresh |
| `?` | help |
| `q` | quit |

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).
