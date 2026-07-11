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
- **`c` clones** a URL into the folder you're browsing and opens it;
  **`i` creates** a fresh repository there.

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

The tab bar switches between four views (`1`–`4`, or click):

### ⌥ Commits
The commit tree across **all** branches: colored lanes with fork/merge
connectors, badges for HEAD (green), local branches (blue), remotes (gray) and
tags (amber). The right pane shows the selected commit's metadata, diffstat and
full colorized patch. `/` filters live by subject, author or hash — `esc` clears.

Act on the selected commit: `c` checks it out (jumping to a local branch when
one points at it, detached HEAD otherwise; double-click does the same), `m`
merges it into your branch, `y` cherry-picks it, `R` rebases your branch onto
it, `v` reverts it, `n` creates a new branch starting at it. All of these
confirm before running.

`b` opens a branch-switch popup without leaving the graph, `t` toggles
**branch focus** (only the current branch's history), and `T` creates a tag on
the selected commit — or, if it already has tags, offers create/delete.

**Blocked switches**: if a checkout would clobber local changes, a popup opens
instead of an error — choose *don't switch*, *stash → switch → re-apply*, or
*discard changes* (irreversible). If the re-applied stash conflicts, the stash
is kept and the conflicts appear in the Status view.

### ± Status
Your working tree as a file tree: *Conflicts*, *Staged* and *Changes* sections
with files grouped under their directories, and your *Stashes* collected at
the bottom for quick access. Click (or navigate to) any file to preview a
clean, noise-free diff; `space` (or double-click) stages/unstages, `c` commits
everything staged and jumps to the graph so you see the new commit land.
`enter` on a stash applies it; `x` drops it.

**Merge conflicts — the resolve panel**: a conflicting merge (or cherry-pick,
rebase, revert) jumps you straight to Status with a clear message; nothing is
left in a broken state. Conflicted files sit in a red *Conflicts* section with
the conflict markers highlighted in the diff. Per file you can:

- `enter` — resolve popup: **use ours** (your branch), **use theirs**
  (incoming), or **I fixed it in my editor — mark resolved**
- `u` / `t` — one-key ours / theirs
- `space` — mark resolved after editing by hand

Then `c` commits the merge (message prefilled) or `X` aborts the whole thing.

If a merge is blocked because you have **uncommitted changes**, git2 says so
explicitly and offers: go commit/stash them yourself, or stash → merge →
re-apply automatically.

More working-tree tools: `D` discards a file's changes (confirmed —
irreversible), `A` amends the last commit with the message prefilled, `H`
toggles the right pane between the file's diff and its **history**, and `B`
toggles **blame**.

**Hunk staging**: press `d` to focus the diff pane (its border turns thick and
bright), then `[` / `]` select hunks — the active hunk header is highlighted —
and `space` stages or unstages just that hunk. Perfect for splitting a messy
file into clean commits.

### ⎇ Branches
Local and remote branches sorted by last activity, with ahead/behind tracking
info. The right pane shows the selected branch's history. `enter` checks a
branch out — for remote branches git2 switches to (or creates) the local
tracking branch and says so explicitly. `n` creates a new branch from HEAD,
`e` renames the selected branch, `x` deletes it (unmerged branches get a
force-delete popup instead of a silent failure),
`m` merges the selected branch into the current one after confirmation, and
`O` opens the pull-request page for the selected branch in your browser
(GitHub, GitLab and Bitbucket URLs are recognized).

### ≡ Stashes
Every stash with its age and message; the right pane previews the diff.
`enter` applies, `p` pops, `x` drops (with confirmation). Create stashes from
the Status view with `S`.

## Remotes & syncing

`f` fetches (and git2 autofetches every 3 minutes), `p` pulls fast-forward
only, `P` pushes — creating the remote branch automatically when there's no
upstream — and `F` force-pushes with lease after confirmation. `o` adds or
shows the origin remote. Authentication uses your existing git setup (SSH
keys, credential helper, `gh auth`); see **[remotes.md](remotes.md)** for
details.

## Controls

Navigation works three ways — arrows, WASD, or vim keys — pick your habit.
The focused pane has a **thick bright border** with a `▶` in its title;
`d` moves focus to the details pane, `a` brings it back to the list (arrow
keys stay in the list so they never jump panes by surprise):

| Key | Action |
| --- | --- |
| `↑ ↓` / `w s` / `j k` | move selection · scroll |
| `tab` / `shift+tab` | next / previous view |
| `a` / `d` (or `h` / `l`) | focus the list ↔ the details pane |
| `ctrl+d` `ctrl+u` / `pgdn` `pgup` | half-page jump |
| `g` / `G` | top / bottom |
| `enter` | focus diff · checkout branch · resolve conflict |
| `u` / `t` | resolve conflict: ours / theirs (status view) |
| `[` `]` + `space` | select & stage/unstage hunks (diff pane) |
| `B` | blame ↔ diff (status view) |
| `c` | checkout commit (commits view) |
| `t` | branch focus ↔ all branches (commits view) |
| `b` | switch-branch popup (commits view) |
| `n` | new branch — from commit or HEAD |
| `y` / `R` / `v` | cherry-pick · rebase onto · revert commit |
| `X` | abort merge (status view) |
| `D` | discard file changes (status view) |
| `A` | amend last commit (status view) |
| `H` | file history ↔ diff (status view) |
| `T` | create / delete tag (commits view) |
| `e` / `x` | rename / delete branch (branches view) |
| `O` | open PR page in browser (branches view) |
| `/` | search commits |
| `space` | stage / unstage file |
| `c` | commit staged changes |
| `S` | stash working tree (status view) |
| `m` | merge selected branch (branches view) |
| `f` | fetch all remotes |
| `p` | pull ff-only · pop stash |
| `P` / `F` | push · force-push (with lease) |
| `o` | add / show origin |
| `x` | drop stash |
| `1` `2` `3` `4` | jump to view |
| `r` | refresh |
| `?` | help overlay |
| `q` / `ctrl+c` | quit |

The line above the key hints is the **message line**: confirmations, prompts
and results appear there (and clear after a few seconds), while the hints stay
visible. When idle it shows details about the current selection.

Mouse: click a row to select it, **double-click to act** (checkout a commit or
branch, stage/unstage a file, apply a stash), click a tab to switch views,
scroll wheel scrolls whichever pane the pointer is over.

## Updating git2

There is no background auto-update. Get the newest release any time with:

```sh
git2 update
```

It downloads the latest binary for your platform and replaces itself in place
(on Windows a `git2.exe.old` leftover may remain — safe to delete). Re-running
the install one-liner does the same thing.
