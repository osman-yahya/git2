# Install on macOS

## Quick install (recommended)

```sh
curl -fsSL https://raw.githubusercontent.com/osman-yahya/git2/main/install.sh | sh
```

Downloads the latest release binary for your CPU, installs it to
`/usr/local/bin` (or `~/.local/bin` without write access) and clears the
quarantine flag. No Go required. Prefer `go install`? Use
`go install github.com/osman-yahya/git2@latest`.

## Manual install — prerequisites

- **git** — ships with Xcode Command Line Tools (`xcode-select --install` if missing)
- **Go 1.22+** to build from source — `brew install go` or [go.dev/dl](https://go.dev/dl/)

## Build & install

```sh
git clone https://github.com/osman-yahya/git2.git && cd git2   # or use your existing checkout
make install                            # builds and copies to /usr/local/bin/git2
```

`/usr/local/bin` may need `sudo make install`. To install without sudo:

```sh
make install PREFIX=~/.local            # installs to ~/.local/bin/git2
```

Then make sure `~/.local/bin` is on your PATH — add to `~/.zshrc`:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

## Verify

Open a new terminal anywhere and run:

```sh
git2
```

Inside a git repo it opens immediately. Anywhere else you get the repo picker
(recent repos + directory browser).

## Notes

- **Apple Silicon vs Intel**: building from source always matches your machine.
  Prebuilt binaries (from `make release`) come in `-macos-arm64` and `-macos-intel` flavors.
- **Gatekeeper**: if you downloaded a prebuilt binary and macOS blocks it, clear the
  quarantine flag: `xattr -d com.apple.quarantine ./git2`
- Works best in a truecolor terminal: Terminal.app, iTerm2, Ghostty, WezTerm, Kitty
  all work out of the box.
- Settings cache lives at `~/Library/Application Support/git2/state.json`.
