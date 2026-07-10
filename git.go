package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Repo wraps a git working directory and shells out to the git CLI.
type Repo struct {
	Root string
}

func findRepo(path string) (*Repo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	out, err := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("not a git repository (or any of its parents): %s", abs)
	}
	return &Repo{Root: strings.TrimSpace(string(out))}, nil
}

func (r *Repo) git(args ...string) (string, error) {
	full := append([]string{"-C", r.Root, "-c", "color.ui=false"}, args...)
	cmd := exec.Command("git", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}

// ---- commits ----

type RefKind int

const (
	RefHead RefKind = iota // branch that HEAD points to
	RefBranch
	RefRemote
	RefTag
)

type Ref struct {
	Name string
	Kind RefKind
}

type Commit struct {
	Hash    string
	Subject string
	Author  string
	Date    time.Time
	Parents []string
	Refs    []Ref
}

func (c Commit) ShortHash() string {
	if len(c.Hash) >= 8 {
		return c.Hash[:8]
	}
	return c.Hash
}

func (c Commit) IsMerge() bool { return len(c.Parents) > 1 }

func parseRefs(decoration string) []Ref {
	var refs []Ref
	for _, part := range strings.Split(decoration, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		switch {
		case strings.HasPrefix(name, "HEAD -> "):
			refs = append(refs, Ref{Name: strings.TrimPrefix(name, "HEAD -> "), Kind: RefHead})
		case name == "HEAD":
			refs = append(refs, Ref{Name: "HEAD", Kind: RefHead})
		case strings.HasPrefix(name, "tag: "):
			refs = append(refs, Ref{Name: strings.TrimPrefix(name, "tag: "), Kind: RefTag})
		case strings.Contains(name, "/"):
			refs = append(refs, Ref{Name: name, Kind: RefRemote})
		default:
			refs = append(refs, Ref{Name: name, Kind: RefBranch})
		}
	}
	return refs
}

func (r *Repo) Commits(limit int) ([]Commit, error) {
	format := "%H\x1f%P\x1f%an\x1f%at\x1f%D\x1f%s\x1e"
	out, err := r.git("log", "--all", "--date-order", "-n", strconv.Itoa(limit), "--pretty=format:"+format)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") ||
			strings.Contains(err.Error(), "bad default revision") {
			return nil, nil
		}
		return nil, err
	}
	var commits []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x1f", 6)
		if len(parts) < 6 {
			continue
		}
		ts, _ := strconv.ParseInt(parts[3], 10, 64)
		c := Commit{
			Hash:    parts[0],
			Author:  parts[2],
			Date:    time.Unix(ts, 0),
			Subject: parts[5],
		}
		if parts[1] != "" {
			c.Parents = strings.Fields(parts[1])
		}
		if parts[4] != "" {
			c.Refs = parseRefs(parts[4])
		}
		commits = append(commits, c)
	}
	return commits, nil
}

func (r *Repo) CommitDetails(hash string) ([]string, error) {
	head, err := r.git("show", "-s",
		"--format=%H%x1f%an <%ae>%x1f%ad%x1f%s%x1f%b",
		"--date=format:%Y-%m-%d %H:%M:%S", hash)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(strings.TrimRight(head, "\n"), "\x1f", 5)
	var lines []string
	if len(parts) == 5 {
		lines = append(lines,
			"commit "+parts[0],
			"author "+parts[1],
			"date   "+parts[2],
			"",
			"  "+parts[3],
		)
		if body := strings.TrimSpace(parts[4]); body != "" {
			lines = append(lines, "")
			for _, l := range strings.Split(body, "\n") {
				lines = append(lines, "  "+l)
			}
		}
		lines = append(lines, "")
	}
	patch, err := r.git("show", "--stat", "--patch", "--format=", hash)
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(strings.TrimRight(patch, "\n"), "\n") {
		lines = append(lines, strings.ReplaceAll(l, "\t", "    "))
	}
	return lines, nil
}

// ---- head / status ----

type HeadInfo struct {
	Branch    string
	Detached  bool
	Ahead     int
	Behind    int
	Dirty     int
	HasRemote bool
}

func (r *Repo) Head() HeadInfo {
	var h HeadInfo
	if out, err := r.git("symbolic-ref", "--short", "-q", "HEAD"); err == nil {
		h.Branch = strings.TrimSpace(out)
	} else if out, err := r.git("rev-parse", "--short", "HEAD"); err == nil {
		h.Branch = strings.TrimSpace(out)
		h.Detached = true
	}
	if out, err := r.git("rev-list", "--left-right", "--count", "@{upstream}...HEAD"); err == nil {
		fields := strings.Fields(out)
		if len(fields) == 2 {
			h.Behind, _ = strconv.Atoi(fields[0])
			h.Ahead, _ = strconv.Atoi(fields[1])
		}
	}
	if out, err := r.git("status", "--porcelain"); err == nil {
		for _, l := range strings.Split(out, "\n") {
			if strings.TrimSpace(l) != "" {
				h.Dirty++
			}
		}
	}
	if out, err := r.git("remote"); err == nil {
		h.HasRemote = strings.TrimSpace(out) != ""
	}
	return h
}

type FileStatus struct {
	Path      string
	Code      string // single git status letter: M A D R C U ?
	Staged    bool
	Untracked bool
}

// Status returns staged entries followed by unstaged/untracked entries.
func (r *Repo) Status() ([]FileStatus, error) {
	out, err := r.git("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var staged, unstaged []FileStatus
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		x, y, path := line[0], line[1], line[3:]
		if x == '?' {
			unstaged = append(unstaged, FileStatus{Path: path, Code: "?", Untracked: true})
			continue
		}
		if x != ' ' {
			staged = append(staged, FileStatus{Path: path, Code: string(x), Staged: true})
		}
		if y != ' ' {
			unstaged = append(unstaged, FileStatus{Path: path, Code: string(y)})
		}
	}
	return append(staged, unstaged...), nil
}

// statusTarget strips the "old -> new" rename notation down to the new path.
func statusTarget(path string) string {
	if i := strings.Index(path, " -> "); i >= 0 {
		return path[i+4:]
	}
	return path
}

func (r *Repo) FileDiff(f FileStatus) ([]string, error) {
	if f.Untracked {
		data, err := os.ReadFile(filepath.Join(r.Root, statusTarget(f.Path)))
		if err != nil {
			return nil, err
		}
		lines := []string{"new file: " + f.Path, ""}
		for _, l := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			lines = append(lines, "+"+strings.ReplaceAll(l, "\t", "    "))
		}
		return lines, nil
	}
	args := []string{"diff"}
	if f.Staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", statusTarget(f.Path))
	out, err := r.git(args...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return []string{"(no changes)"}, nil
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		lines = append(lines, strings.ReplaceAll(l, "\t", "    "))
	}
	return lines, nil
}

func (r *Repo) StageFile(f FileStatus) error {
	_, err := r.git("add", "--", statusTarget(f.Path))
	return err
}

func (r *Repo) UnstageFile(f FileStatus) error {
	_, err := r.git("restore", "--staged", "--", statusTarget(f.Path))
	return err
}

func (r *Repo) Commit(message string) error {
	_, err := r.git("commit", "-m", message)
	return err
}

// ---- branches ----

type Branch struct {
	Name    string
	Current bool
	Remote  bool
	Hash    string
	Track   string
	Date    string
	Subject string
}

func (r *Repo) Branches() ([]Branch, error) {
	format := "%(HEAD)\x1f%(refname)\x1f%(refname:short)\x1f%(objectname:short)\x1f%(upstream:track)\x1f%(committerdate:relative)\x1f%(contents:subject)"
	out, err := r.git("for-each-ref", "refs/heads", "refs/remotes",
		"--sort=-committerdate", "--format="+format)
	if err != nil {
		return nil, err
	}
	var branches []Branch
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 7)
		if len(parts) < 7 {
			continue
		}
		b := Branch{
			Current: parts[0] == "*",
			Remote:  strings.HasPrefix(parts[1], "refs/remotes/"),
			Name:    parts[2],
			Hash:    parts[3],
			Track:   parts[4],
			Date:    parts[5],
			Subject: parts[6],
		}
		if b.Remote && strings.HasSuffix(b.Name, "/HEAD") {
			continue
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func (r *Repo) Checkout(name string) error {
	// checking out a remote branch creates a local tracking branch
	local := name
	if i := strings.Index(name, "/"); i >= 0 {
		local = name[i+1:]
	}
	if local != name {
		if _, err := r.git("checkout", local); err == nil {
			return nil
		}
		_, err := r.git("checkout", "-b", local, "--track", name)
		return err
	}
	_, err := r.git("checkout", name)
	return err
}

func (r *Repo) BranchLog(name string, limit int) ([]string, error) {
	out, err := r.git("log", name, "-n", strconv.Itoa(limit),
		"--pretty=format:%h\x1f%s\x1f%an\x1f%cr")
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// ---- remotes / network ----

// gitNet runs a network-touching git command with terminal prompts disabled
// (credentials must come from SSH keys or a credential helper — an interactive
// prompt would deadlock the TUI) and a hard timeout.
func (r *Repo) gitNet(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	full := append([]string{"-C", r.Root, "-c", "color.ui=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if ctx.Err() != nil {
			msg = "network operation timed out"
		}
		if strings.Contains(msg, "terminal prompts disabled") ||
			strings.Contains(msg, "Authentication failed") ||
			strings.Contains(msg, "could not read Username") {
			msg = "authentication required — set up SSH keys or a credential helper (see docs/remotes.md)"
		}
		if strings.Contains(msg, "'origin' does not appear") {
			msg = "no remote 'origin' — press o to add one"
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}

func (r *Repo) RemoteURL(name string) string {
	out, err := r.git("remote", "get-url", name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (r *Repo) AddRemote(name, url string) error {
	_, err := r.git("remote", "add", name, url)
	return err
}

func (r *Repo) Fetch() error {
	_, err := r.gitNet("fetch", "--all", "--prune")
	return err
}

// Pull fast-forwards only; a diverged branch reports an error instead of
// creating surprise merge commits.
func (r *Repo) Pull() error {
	_, err := r.gitNet("pull", "--ff-only")
	return err
}

// Push pushes the current branch. Without an upstream it pushes with -u to
// origin, creating the remote branch. force uses --force-with-lease, which
// refuses to overwrite commits you haven't seen.
func (r *Repo) Push(force bool) (string, error) {
	out, err := r.git("symbolic-ref", "--short", "-q", "HEAD")
	branch := strings.TrimSpace(out)
	if err != nil || branch == "" {
		return "", errors.New("cannot push: detached HEAD")
	}
	args := []string{"push"}
	if force {
		args = append(args, "--force-with-lease")
	}
	if _, uerr := r.git("rev-parse", "--abbrev-ref", branch+"@{upstream}"); uerr != nil {
		args = append(args, "-u", "origin", branch)
		if _, err := r.gitNet(args...); err != nil {
			return "", err
		}
		return "✓ pushed & created origin/" + branch, nil
	}
	if _, err := r.gitNet(args...); err != nil {
		return "", err
	}
	if force {
		return "✓ force-pushed " + branch + " (with lease)", nil
	}
	return "✓ pushed " + branch, nil
}

// ---- stashes ----

type Stash struct {
	Ref  string // stash@{0}
	Age  string
	Desc string
}

func (r *Repo) Stashes() ([]Stash, error) {
	out, err := r.git("stash", "list", "--format=%gd\x1f%cr\x1f%gs")
	if err != nil {
		return nil, err
	}
	var stashes []Stash
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) < 3 {
			continue
		}
		stashes = append(stashes, Stash{Ref: parts[0], Age: parts[1], Desc: parts[2]})
	}
	return stashes, nil
}

func (r *Repo) StashPush(message string) error {
	args := []string{"stash", "push", "--include-untracked"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := r.git(args...)
	return err
}

func (r *Repo) StashApply(ref string) error {
	_, err := r.git("stash", "apply", ref)
	return err
}

func (r *Repo) StashPop(ref string) error {
	_, err := r.git("stash", "pop", ref)
	return err
}

func (r *Repo) StashDrop(ref string) error {
	_, err := r.git("stash", "drop", ref)
	return err
}

func (r *Repo) StashDiff(ref string) ([]string, error) {
	out, err := r.git("stash", "show", "-p", "--include-untracked", ref)
	if err != nil {
		// older git can't combine show -p with --include-untracked
		out, err = r.git("stash", "show", "-p", ref)
		if err != nil {
			return nil, err
		}
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		lines = append(lines, strings.ReplaceAll(l, "\t", "    "))
	}
	return lines, nil
}

// ---- merge ----

func (r *Repo) Merge(name string) error {
	_, err := r.git("merge", "--no-edit", name)
	return err
}
