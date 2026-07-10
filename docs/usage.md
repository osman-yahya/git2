# Using git2

## Launching

```sh
git2              # open the repo containing the current directory
git2 ~/code/app   # open a specific repo (any path inside it works)
git2 --print      # print the commit graph to stdout and exit
```

git2 walks up from the given path to find the enclosing repository, exactly like
git itself.

## The repo picker

If you launch git2 outside any repository, the picker opens instead of an error:

- **★ Recent** — repos you opened before, most recent first (up to 15).
  `enter` reopens one instantly.
- **⌂ Browse** — a directory browser. Git repositories are listed first and
  marked `⎇ … git repo`; `enter` on a repo opens it, `enter` on a plain folder
  descends into it, `←`/`backspace` goes up, `~` jumps home, `.` opens the
  current location (useful when you're already inside a repo's subfolder).

`tab` switches between the two. The last browsed folder and the recent-repos
list are cached, so next time the picker starts where you left off.

Cache location:

| OS | Path |
| --- | --- |
| macOS | `~/Library/Application Support/git2/state.json` |
| Linux | `~/.config/git2/state.json` |
| Windows | `%AppData%\git2\state.json` |

Delete the file to reset history.

## Views

The tab bar switches between three views (`1` / `2` / `3`, or click):

### ⌥ Commits
The commit tree across **all** branches: colored lanes with fork/merge
connectors, badges for HEAD (green), local branches (blue), remotes (gray) and
tags (amber). The right pane shows the selected commit's metadata, diffstat and
full colorized patch. `/` filters live by subject, author or hash — `esc` clears.

### ± Status
Your working tree. `●` marks staged entries, `○` unstaged/untracked; the right
pane previews each file's diff. `space` stages or unstages the selected file,
`c` prompts for a message and commits everything staged.

### ⎇ Branches
Local and remote branches sorted by last activity, with ahead/behind tracking
info. The right pane shows the selected branch's history. `enter` checks a
branch out (remote branches get a local tracking branch automatically).

## Controls

Navigation works three ways — arrows, WASD, or vim keys — pick your habit:

| Key | Action |
| --- | --- |
| `↑ ↓` / `w s` / `j k` | move selection · scroll |
| `← →` / `a d` / `h l` / `tab` | switch pane focus |
| `ctrl+d` `ctrl+u` / `pgdn` `pgup` | half-page jump |
| `g` / `G` | top / bottom |
| `enter` | focus the diff pane · checkout branch |
| `/` | search commits |
| `space` | stage / unstage file |
| `c` | commit staged changes |
| `1` `2` `3` | switch view |
| `r` | refresh |
| `?` | help overlay |
| `q` / `ctrl+c` | quit |

Mouse: click a row to select it, click a tab to switch views, scroll wheel
scrolls whichever pane the pointer is over.
