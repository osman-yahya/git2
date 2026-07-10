# CLAUDE.md

git2 is a Fork-inspired terminal git client (Go + Bubble Tea + Lip Gloss).
Repo: https://github.com/osman-yahya/git2 · module `github.com/osman-yahya/git2`.
Single `main` package, no internal packages. Unit tests in `git2_test.go`
(`go test ./...`) cover pure logic (graph layout, PR URLs); interactive behavior
is verified by driving the TUI through a PTY (see below).

## Build & check

```sh
go build -o git2-bin .     # dev binary (git2-bin and git2 are gitignored)
gofmt -w . && go vet ./...
./git2-bin --print <repo>  # renders the commit graph to stdout, no TUI — fastest smoke test
```

## File map

| File | Contents |
| --- | --- |
| `main.go` | arg parsing, repo detection → picker fallback, `--print` mode, version const |
| `git.go` | all git CLI calls. `git()` = local ops; `gitNet()` = network ops with `GIT_TERMINAL_PROMPT=0` + 90s timeout (never let git prompt — it deadlocks the TUI) |
| `graph.go` | commit-graph lane layout (`BuildGraph`). Input must be `git log --date-order` (children before parents) |
| `model.go` | Bubble Tea model: state, all key/mouse handling, async `tea.Cmd`s, autofetch tick |
| `views.go` | all rendering; also `truncate`/`stripANSI`/`relTime` helpers |
| `styles.go` | Lip Gloss styles, lane color palette (Tokyo Night-ish) |
| `picker.go` | repo-picker TUI shown when launched outside a repo |
| `update.go` | `git2 update` self-updater (downloads latest release, rename-swap) |
| `config.go` | persisted state (recent repos, last browse dir) in `os.UserConfigDir()/git2/state.json` |

## Conventions

- Every git call goes through `Repo.git()`/`Repo.gitNet()` — never `exec.Command` directly.
- Async work returns typed msgs (`commitsMsg`, `actionMsg{reload:true}` …); stale async
  results are guarded by comparing against the currently selected item (`detailFor`,
  `diffFor`, `brLogFor`, `stDiffFor` pattern) — keep that when adding loaders.
- Destructive actions (force push, merge/rebase/cherry-pick/revert, stash drop) go
  through the confirm modal (`m.confirmMsg` + `m.confirmCmd`); text entry through the
  prompt modal (`m.promptMode`); multi-option decisions (blocked checkout:
  cancel/stash/discard) through the choice popup (`m.choiceOptions`, opened via
  `checkoutBlockedMsg`). All checkouts go through `m.doCheckout` so the popup logic
  stays centralized.
- Key aliases everywhere: arrows / `w s a d` / `j k h l`. `s` means *down*, so it can't
  be a mnemonic (stage = `space`, stash = `S`). New keys must be added to the footer
  hints and the `?` help overlay in views.go, plus docs/usage.md and README.
- Force push is always `--force-with-lease`; pull is always `--ff-only`.
- Machine-readable git output uses `%x1f` field / `%x1e` record separators.

## Testing (no real terminal available)

Bubble Tea apps can't be driven by piping stdin. Use the PTY harness pattern in Python
(`pty.fork` + write keys + drain output). Two things it MUST do or the app hangs/stalls:

1. Answer terminal queries: reply to `\x1b]11;?` (background color) with
   `\x1b]11;rgb:1a1a/1b1b/2626\x1b\\` and to `\x1b[6n` with `\x1b[1;1R`.
2. Set the winsize ioctl on the PTY before draining.

Then strip ANSI and grep the capture for expected strings. Known-good driver scripts
lived in the session scratchpad (`drive2.py`, `drive_picker.py`) — recreate as needed.
For remote-feature tests, use a local bare repo (`git init --bare origin.git`) as origin.

## Gotchas learned the hard way

- `lipgloss.HasDarkBackground()` queries the terminal via stdin; it's called once in
  main() **before** `tea.NewProgram` so the query can't eat Bubble Tea's input.
- Go slices: `s[1:]` on a possibly-empty slice panics — graph.go guards `matches[1:]`
  and `c.Parents[1:]`; keep the guard style.
- macOS `script` does not forward piped stdin to the PTY; the Python harness is the way.
- The GitHub API may return a stale/empty asset list right after upload — re-query
  before concluding an upload failed.
- `%(refname:short)` of `refs/remotes/origin/HEAD` is just `origin` — filter symbolic
  remote HEADs by the FULL refname or a phantom "origin" branch appears whose checkout
  detaches HEAD.
- Never drive the `O` (open PR) key in PTY tests — it really opens the user's browser.

## Release process

Release assets are **version-less** (`git2-macos-arm64`, …) so `install.sh` /
`install.ps1` can always fetch `releases/latest/download/<asset>` — never add the
version back into asset names.

1. Bump `version` in main.go and `VERSION` in Makefile (keep in sync).
2. Commit, `git tag -a vX.Y.Z -m "…"`, `git push origin main vX.Y.Z`.
3. `make release` → builds `dist/` for macos/linux × arm64/amd64 + windows-amd64
   and generates `checksums.txt`.
4. Create the GitHub release for the tag and upload all 6 files from `dist/`.
   No `gh` CLI on this machine; the token in the macOS keychain works:
   `printf 'protocol=https\nhost=github.com\n\n' | git credential fill` → password field,
   then the REST API (`POST /repos/osman-yahya/git2/releases`, then
   `uploads.github.com/...:releases/<id>/assets?name=<file>`). Never print the token.
5. Verify like a user: run the install.sh one-liner with `BIN_DIR=` override, check the
   binary's SHA-256 against checksums.txt, and `go install github.com/osman-yahya/git2@latest`.

## Docs to keep in sync

README.md (features + keys table) · docs/usage.md (full keymap & views) ·
docs/install-{macos,linux,windows}.md · docs/remotes.md (auth/network semantics) ·
help overlay + footer hints in views.go.
