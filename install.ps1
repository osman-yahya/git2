# git2 installer for Windows
#
#   irm https://raw.githubusercontent.com/osman-yahya/git2/main/install.ps1 | iex
#
# Downloads the latest release binary to %LOCALAPPDATA%\Programs\git2 and adds
# that folder to your user PATH. No admin rights required.
$ErrorActionPreference = "Stop"

$repo = "osman-yahya/git2"
$dir  = Join-Path $env:LOCALAPPDATA "Programs\git2"
$exe  = Join-Path $dir "git2.exe"
$url  = "https://github.com/$repo/releases/latest/download/git2-windows-amd64.exe"

New-Item -ItemType Directory -Force -Path $dir | Out-Null

Write-Host "downloading git2 ..." -ForegroundColor Cyan
Invoke-WebRequest -Uri $url -OutFile $exe -UseBasicParsing

# add to user PATH if missing (takes effect in new terminals)
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $dir) {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
    Write-Host "added $dir to your user PATH" -ForegroundColor Yellow
    $pathNote = " (open a NEW terminal first)"
} else {
    $pathNote = ""
}

$version = & $exe --version
Write-Host "installed $exe ($version)" -ForegroundColor Green
Write-Host "run: git2$pathNote"
