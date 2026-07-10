# Remotes, authentication & syncing

## How authentication works

git2 shells out to your system `git` for every operation, so it authenticates
exactly the way your terminal git does — there is no separate login:

- **SSH remotes** (`git@github.com:user/repo.git`) — uses your SSH keys.
  Set up once: [GitHub docs](https://docs.github.com/en/authentication/connecting-to-github-with-ssh).
- **HTTPS remotes** — uses your git credential helper:
  - macOS: `osxkeychain` (default with Apple git)
  - Windows: Git Credential Manager (ships with Git for Windows)
  - Any OS: `gh auth login` (GitHub CLI) also wires up git credentials
- Works identically with GitLab, Bitbucket, Gitea, or any git host.

git2 runs network commands with `GIT_TERMINAL_PROMPT=0`, so git can never
freeze the UI waiting for a password. If credentials are missing you get a
clear error telling you to set up keys or a helper — do that once in a normal
terminal (e.g. `git fetch` and answer the prompts, or `gh auth login`), and
git2 works from then on.

## Adding a remote

Press **`o`** in any view:

- no origin yet → an input opens; paste the URL
  (`git@github.com:user/repo.git` or `https://…`)
- origin exists → the footer shows its URL

## Fetching

- **`f`** fetches all remotes with `--prune` and refreshes the view.
- **Autofetch** runs the same fetch silently every 3 minutes whenever a remote
  is configured (the header shows `⇣ fetching…` while it runs). Errors during
  autofetch are ignored; manual fetch reports them.

## Pull & push

- **`p`** pulls with `--ff-only` — it will never create a surprise merge
  commit. If your branch diverged, you get an error and can decide yourself.
- **`P`** pushes the current branch. If the branch has no upstream yet, git2
  pushes with `-u origin <branch>`, **creating the remote branch
  automatically** — the footer confirms with `pushed & created origin/<branch>`.
- **`F`** force-pushes after a `y/N` confirmation. git2 always uses
  `--force-with-lease`, which refuses to overwrite commits on the remote that
  you haven't seen — much safer than a raw `--force`.

Network operations time out after 90 seconds.

## Stashes (view `4`)

- **`S`** in the Status view stashes the working tree, untracked files
  included; the message is optional.
- In the Stashes view: **`enter`** applies (keeps the stash), **`p`** pops
  (applies and removes), **`x`** drops after a `y/N` confirmation. The right
  pane previews each stash's diff.

## Merging (view `3`)

Select any branch and press **`m`** to merge it into the branch you're on,
after a `y/N` confirmation. Fast-forwards happen automatically; real merges
create a merge commit.

If the merge hits conflicts, git reports them in the footer and the conflicted
files appear in the Status view. Resolve them in your editor, stage with
`space`, and commit with `c` — or run `git merge --abort` in a terminal to
back out.
