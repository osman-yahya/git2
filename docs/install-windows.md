# Install on Windows

## Quick install (recommended)

In PowerShell:

```powershell
irm https://raw.githubusercontent.com/osman-yahya/git2/main/install.ps1 | iex
```

Downloads the latest release binary to `%LOCALAPPDATA%\Programs\git2`, adds
that folder to your **user PATH** automatically, and verifies the install. No
admin rights or Go required — open a new terminal afterwards and run `git2`.

## Manual install — prerequisites

- **git** — [git-scm.com/download/win](https://git-scm.com/download/win) (Git for Windows)
- **Go 1.22+** to build from source — [go.dev/dl](https://go.dev/dl/)
- **Windows Terminal** recommended (Microsoft Store) — best colors and glyph rendering.
  PowerShell inside Windows Terminal is the ideal combo; classic `cmd.exe` works too.

## Build & install

In PowerShell:

```powershell
git clone https://github.com/osman-yahya/git2.git ; cd git2    # or use your existing checkout
go build -trimpath -ldflags "-s -w" -o git2.exe .
```

Put `git2.exe` somewhere on your PATH. A clean way that needs no admin rights:

```powershell
mkdir "$env:LOCALAPPDATA\Programs\git2" -Force
copy git2.exe "$env:LOCALAPPDATA\Programs\git2\"
```

Then add that folder to your user PATH (one-time):

```powershell
[Environment]::SetEnvironmentVariable(
  "Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";$env:LOCALAPPDATA\Programs\git2",
  "User")
```

Alternatively `go install .` from the project folder drops `git2.exe` into
`%USERPROFILE%\go\bin` — add that to PATH instead if you prefer.

## Verify

Open a **new** terminal window (PATH changes need a fresh session) and run:

```powershell
git2
```

Inside a git repo it opens immediately. Anywhere else you get the repo picker
(recent repos + directory browser).

## Notes

- Prebuilt binaries (from `make release`) are named `git2-<version>-windows-amd64.exe`;
  rename to `git2.exe` and place on PATH.
- Also works inside **WSL** — follow the [Linux guide](install-linux.md) there.
- Settings cache lives at `%AppData%\git2\state.json`.
