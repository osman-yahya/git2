# Install on Linux

## Prerequisites

- **git** — `apt install git` / `dnf install git` / `pacman -S git`
- **Go 1.22+** to build from source — your package manager or [go.dev/dl](https://go.dev/dl/)

## Build & install

```sh
git clone <repo-url> git2 && cd git2   # or use your existing checkout
sudo make install                       # builds and copies to /usr/local/bin/git2
```

To install without sudo:

```sh
make install PREFIX=~/.local            # installs to ~/.local/bin/git2
```

`~/.local/bin` is already on PATH in most modern distros; if not, add to
`~/.bashrc` or `~/.zshrc`:

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

- Prebuilt binaries (from `make release`) come in `-linux-amd64` and `-linux-arm64`
  flavors; `chmod +x` and drop one into any PATH directory.
- Any modern terminal works (GNOME Terminal, Konsole, Alacritty, Kitty, WezTerm, foot).
  For best results use a font with box-drawing glyphs — practically all system
  monospace fonts qualify.
- Settings cache lives at `~/.config/git2/state.json` (respects `$XDG_CONFIG_HOME`).
