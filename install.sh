#!/bin/sh
# git2 installer for macOS & Linux
#
#   curl -fsSL https://raw.githubusercontent.com/osman-yahya/git2/main/install.sh | sh
#
# Downloads the latest release binary for this machine and installs it as
# `git2` on your PATH. Override the target directory with BIN_DIR=… .
set -eu

REPO="osman-yahya/git2"

case "$(uname -s)" in
  Darwin) os="macos" ;;
  Linux)  os="linux" ;;
  *) echo "git2: unsupported OS: $(uname -s) — see https://github.com/$REPO#install" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64)  arch="amd64" ;;
  *) echo "git2: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

asset="git2-$os-$arch"
url="https://github.com/$REPO/releases/latest/download/$asset"

# pick an install dir: BIN_DIR override > /usr/local/bin if writable > ~/.local/bin
if [ -n "${BIN_DIR:-}" ]; then
  dir="$BIN_DIR"
elif [ -w /usr/local/bin ]; then
  dir="/usr/local/bin"
else
  dir="$HOME/.local/bin"
fi
mkdir -p "$dir"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
echo "⇣ downloading $asset …"
curl -fsSL "$url" -o "$tmp"
chmod +x "$tmp"

# macOS: clear the quarantine flag downloads get
if [ "$os" = "macos" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$tmp" 2>/dev/null || true
fi

mv "$tmp" "$dir/git2"
trap - EXIT
echo "✓ installed $dir/git2 ($("$dir/git2" --version))"

case ":$PATH:" in
  *":$dir:"*) echo "  run: git2" ;;
  *)
    echo "  ⚠ $dir is not on your PATH — add this to your shell profile:"
    echo "      export PATH=\"$dir:\$PATH\""
    ;;
esac
