package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const repoSlug = "osman-yahya/git2"

// latestVersion asks GitHub for the newest release tag ("0.9.0", no v).
func latestVersion(timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get("https://api.github.com/repos/" + repoSlug + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

// selfUpdate replaces the running binary with the latest GitHub release.
// There is no background auto-update: users run `git2 update` (or re-run the
// install one-liner) when they want the new version.
func selfUpdate() error {
	client := &http.Client{Timeout: 60 * time.Second}

	latest, err := latestVersion(60 * time.Second)
	if err != nil {
		return fmt.Errorf("checking latest release: %w", err)
	}
	if latest == version {
		fmt.Println("git2 " + version + " is already the latest version ✓")
		return nil
	}
	fmt.Printf("updating git2 %s → %s …\n", version, latest)

	osName := map[string]string{"darwin": "macos", "linux": "linux", "windows": "windows"}[runtime.GOOS]
	if osName == "" {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	asset := "git2-" + osName + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}

	dl, err := client.Get("https://github.com/" + repoSlug + "/releases/latest/download/" + asset)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", asset, err)
	}
	defer dl.Body.Close()
	if dl.StatusCode != 200 {
		return fmt.Errorf("downloading %s: HTTP %d", asset, dl.StatusCode)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)

	tmp, err := os.CreateTemp(filepath.Dir(exe), ".git2-update-*")
	if err != nil {
		return fmt.Errorf("cannot write next to %s (try re-running with elevated permissions): %w", exe, err)
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, dl.Body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("downloading: %w", err)
	}
	tmp.Close()
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return err
	}

	// swap: rename the running binary aside (allowed even on Windows), move
	// the new one into place, then try to clean up.
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replacing %s: %w", exe, err)
	}
	if err := os.Rename(tmpName, exe); err != nil {
		_ = os.Rename(old, exe) // roll back
		os.Remove(tmpName)
		return fmt.Errorf("installing new binary: %w", err)
	}
	if err := os.Remove(old); err != nil {
		fmt.Println("note: leftover " + old + " can be deleted (Windows keeps the running file locked)")
	}
	fmt.Println("✓ updated to git2 " + latest + " at " + exe)
	return nil
}
