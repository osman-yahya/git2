# Install on macOS

## Prerequisites

- **git** — ships with Xcode Command Line Tools (`xcode-select --install` if missing)
- **Go 1.22+** to build from source — `brew install go` or [go.dev/dl](https://go.dev/dl/)

## Build & install

```sh
git clone <repo-url> git2 && cd git2   # or use your existing checkout
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
