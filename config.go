package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State is persisted across runs: recently opened repos and the directory
// the browser was last in. Lives in the platform config dir:
//
//	macOS   ~/Library/Application Support/git2/state.json
//	Linux   ~/.config/git2/state.json
//	Windows %AppData%\git2\state.json
type State struct {
	Recent  []string `json:"recent"`
	LastDir string   `json:"last_dir"`
}

const maxRecent = 15

func statePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "git2", "state.json"), nil
}

func loadState() State {
	var s State
	path, err := statePath()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

func saveState(s State) {
	path, err := statePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// Touch moves (or inserts) a repo path at the front of the recent list.
func (s *State) Touch(repo string) {
	out := []string{repo}
	for _, r := range s.Recent {
		if r != repo && len(out) < maxRecent {
			out = append(out, r)
		}
	}
	s.Recent = out
}

// ExistingRecent filters the recent list down to paths that still exist.
func (s State) ExistingRecent() []string {
	var out []string
	for _, r := range s.Recent {
		if info, err := os.Stat(r); err == nil && info.IsDir() {
			out = append(out, r)
		}
	}
	return out
}
