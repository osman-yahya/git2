package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		// merge/cherry-pick conflicts report on stdout, most errors on stderr —
		// combine so callers can classify what actually happened
		msg := strings.TrimSpace(stderr.String())
		if out := strings.TrimSpace(stdout.String()); out != "" {
			if msg != "" {
				msg = out + "\n" + msg
			} else {
				msg = out
			}
		}
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

// isRemoteRef reports whether a short ref name ("origin/main") belongs to a
// remote. Local branches may contain slashes too (dev/main), so the check is
// against the actual remote names, never just "contains a slash".
func isRemoteRef(name string, remotes []string) bool {
	for _, r := range remotes {
		if strings.HasPrefix(name, r+"/") {
			return true
		}
	}
	return false
}

func parseRefs(decoration string, remotes []string) []Ref {
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
		case isRemoteRef(name, remotes):
			refs = append(refs, Ref{Name: name, Kind: RefRemote})
		default:
			refs = append(refs, Ref{Name: name, Kind: RefBranch})
		}
	}
	return refs
}

// RemoteNames lists the configured remotes ("origin", …).
func (r *Repo) RemoteNames() []string {
	out, err := r.git("remote")
	if err != nil {
		return nil
	}
	var names []string
	for _, n := range strings.Split(strings.TrimSpace(out), "\n") {
		if n != "" {
			names = append(names, n)
		}
	}
	return names
}

// Commits lists commits reachable from ref (branch/tag); ref "" means all refs.
func (r *Repo) Commits(limit int, ref string) ([]Commit, error) {
	format := "%H\x1f%P\x1f%an\x1f%at\x1f%D\x1f%s\x1e"
	args := []string{"log", "--date-order", "-n", strconv.Itoa(limit), "--pretty=format:" + format}
	if ref == "" {
		args = append(args, "--all")
	} else {
		args = append(args, ref, "--")
	}
	out, err := r.git(args...)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") ||
			strings.Contains(err.Error(), "bad default revision") {
			return nil, nil
		}
		return nil, err
	}
	remotes := r.RemoteNames()
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
			c.Refs = parseRefs(parts[4], remotes)
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// CommitDetails returns the metadata block and the patch separately so the
// UI can show them in two stacked panes.
func (r *Repo) CommitDetails(hash string) (meta, patch []string, err error) {
	head, err := r.git("show", "-s",
		"--format=%H%x1f%an <%ae>%x1f%ad%x1f%s%x1f%b%x1f%P",
		"--date=format:%Y-%m-%d %H:%M:%S", hash)
	if err != nil {
		return nil, nil, err
	}
	parts := strings.SplitN(strings.TrimRight(head, "\n"), "\x1f", 6)
	if len(parts) == 6 {
		meta = append(meta,
			"commit  "+parts[0],
			"author  "+parts[1],
			"date    "+parts[2],
		)
		if p := strings.TrimSpace(parts[5]); p != "" {
			short := []string{}
			for _, ph := range strings.Fields(p) {
				short = append(short, ph[:min(8, len(ph))])
			}
			meta = append(meta, "parents "+strings.Join(short, ", "))
		}
		meta = append(meta, "", "  "+parts[3])
		if body := strings.TrimSpace(parts[4]); body != "" {
			meta = append(meta, "")
			for _, l := range strings.Split(body, "\n") {
				meta = append(meta, "  "+l)
			}
		}
	}
	out, err := r.git("show", "--stat", "--patch", "--format=", hash)
	if err != nil {
		return nil, nil, err
	}
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		patch = append(patch, strings.ReplaceAll(l, "\t", "    "))
	}
	return meta, patch, nil
}

// ---- head / status ----

type HeadInfo struct {
	Branch    string
	Detached  bool
	Ahead     int
	Behind    int
	Dirty     int
	HasRemote bool
	Merging   bool
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
	if out, err := r.git("rev-parse", "--git-path", "MERGE_HEAD"); err == nil {
		p := strings.TrimSpace(out)
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.Root, p)
		}
		if _, err := os.Stat(p); err == nil {
			h.Merging = true
		}
	}
	return h
}

type FileStatus struct {
	Path      string
	Code      string // single git status letter: M A D R C U ?
	Staged    bool
	Untracked bool
	Conflict  bool
}

// Status returns conflicted entries, then staged, then unstaged/untracked.
func (r *Repo) Status() ([]FileStatus, error) {
	out, err := r.git("status", "--porcelain", "-uall")
	if err != nil {
		return nil, err
	}
	var conflicts, staged, unstaged []FileStatus
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		x, y, path := line[0], line[1], line[3:]
		if x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D') {
			conflicts = append(conflicts, FileStatus{Path: path, Code: "!", Conflict: true})
			continue
		}
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
	return append(conflicts, append(staged, unstaged...)...), nil
}

// statusTarget strips the "old -> new" rename notation down to the new path.
func statusTarget(path string) string {
	if i := strings.Index(path, " -> "); i >= 0 {
		return path[i+4:]
	}
	return path
}

// cleanDiff strips git's plumbing header noise from a patch, keeping a
// simple file header, hunk separators and the +/- lines.
func cleanDiff(raw string) []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(raw, "\n"), "\n") {
		l = strings.ReplaceAll(l, "\t", "    ")
		switch {
		case strings.HasPrefix(l, "diff --git "):
			// "diff --git a/x b/y" → file header on y
			if i := strings.LastIndex(l, " b/"); i >= 0 {
				if len(lines) > 0 {
					lines = append(lines, "")
				}
				lines = append(lines, "▸ "+l[i+3:])
			}
		case strings.HasPrefix(l, "index "), strings.HasPrefix(l, "--- "),
			strings.HasPrefix(l, "+++ "), strings.HasPrefix(l, "new file mode"),
			strings.HasPrefix(l, "deleted file mode"), strings.HasPrefix(l, "old mode"),
			strings.HasPrefix(l, "new mode"), strings.HasPrefix(l, "similarity index"),
			strings.HasPrefix(l, "rename from"), strings.HasPrefix(l, "rename to"):
			// plumbing noise — drop
		case strings.HasPrefix(l, "@@"):
			// "@@ -1,4 +1,5 @@ ctx" → "@@ +1  ctx"
			rest := strings.TrimPrefix(l, "@@")
			ctx := ""
			if i := strings.Index(rest, "@@"); i >= 0 {
				ctx = strings.TrimSpace(rest[i+2:])
				rest = rest[:i]
			}
			target := ""
			for _, fld := range strings.Fields(rest) {
				if strings.HasPrefix(fld, "+") {
					target = strings.SplitN(fld[1:], ",", 2)[0]
				}
			}
			sep := "@@ line " + target
			if ctx != "" {
				sep += "  · " + ctx
			}
			lines = append(lines, sep)
		default:
			lines = append(lines, l)
		}
	}
	return lines
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
	return cleanDiff(out), nil
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
		// skip the symbolic origin/HEAD ref (its short name is just "origin",
		// and checking it out would detach HEAD)
		if strings.HasSuffix(parts[1], "/HEAD") {
			continue
		}
		branches = append(branches, b)
	}
	return branches, nil
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
	return cleanDiff(out), nil
}

// ---- merge ----

func (r *Repo) Merge(name string) error {
	_, err := r.git("merge", "--no-edit", name)
	return err
}

// ---- commit-level operations ----

// isBlockedCheckout reports whether an error is git refusing to switch
// because local changes would be overwritten.
func isBlockedCheckout(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "would be overwritten") ||
		strings.Contains(s, "commit your changes or stash them")
}

// CheckoutBranch switches branches and returns a description of what
// actually happened. remote=true means name is "remote/branch": git2 switches
// to (or creates) the local tracking branch and the message says so. Local
// branch names may legitimately contain slashes (dev/main).
func (r *Repo) CheckoutBranch(name string, remote bool) (string, error) {
	if remote {
		local := name
		if i := strings.Index(name, "/"); i >= 0 {
			local = name[i+1:]
		}
		if _, err := r.git("show-ref", "--verify", "--quiet", "refs/heads/"+local); err == nil {
			if _, err := r.git("checkout", local); err != nil {
				return "", err
			}
			return "✓ switched to local " + local + " (tracking " + name + " — pull to sync)", nil
		}
		if _, err := r.git("checkout", "-b", local, "--track", name); err != nil {
			return "", err
		}
		return "✓ created " + local + " tracking " + name, nil
	}
	if _, err := r.git("checkout", name); err != nil {
		return "", err
	}
	return "✓ checked out " + name, nil
}

// CheckoutCommit moves HEAD to a commit (detached).
func (r *Repo) CheckoutCommit(hash string) (string, error) {
	if _, err := r.git("checkout", hash); err != nil {
		return "", err
	}
	return "✓ checked out " + hash[:8] + " (detached HEAD — use Branches to return)", nil
}

// DiscardAll throws away tracked working-tree changes.
func (r *Repo) DiscardAll() error {
	_, err := r.git("reset", "--hard")
	return err
}

func (r *Repo) CherryPick(hash string) error {
	_, err := r.git("cherry-pick", hash)
	return err
}

func (r *Repo) Rebase(onto string) error {
	_, err := r.git("rebase", onto)
	return err
}

func (r *Repo) Revert(hash string) error {
	_, err := r.git("revert", "--no-edit", hash)
	return err
}

// ---- pull-request URLs ----

// PRURL builds the "open a pull request" page for a branch on the origin
// host. Understands github / gitlab / bitbucket; anything else gets the
// repo page.
func prURL(remoteURL, branch string) string {
	u := strings.TrimSuffix(strings.TrimSpace(remoteURL), ".git")
	u = strings.TrimPrefix(u, "ssh://")
	if strings.HasPrefix(u, "git@") {
		u = strings.Replace(strings.TrimPrefix(u, "git@"), ":", "/", 1)
	}
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	host := u
	if i := strings.Index(u, "/"); i >= 0 {
		host = u[:i]
	}
	switch {
	case strings.Contains(host, "github"):
		return "https://" + u + "/compare/" + branch + "?expand=1"
	case strings.Contains(host, "gitlab"):
		return "https://" + u + "/-/merge_requests/new?merge_request%5Bsource_branch%5D=" + branch
	case strings.Contains(host, "bitbucket"):
		return "https://" + u + "/pull-requests/new?source=" + branch
	default:
		return "https://" + u
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// ---- branches & merge state ----

// CreateBranch creates and switches to a new branch; base "" means HEAD.
func (r *Repo) CreateBranch(name, base string) error {
	args := []string{"checkout", "-b", name}
	if base != "" {
		args = append(args, base)
	}
	_, err := r.git(args...)
	return err
}

func (r *Repo) MergeAbort() error {
	_, err := r.git("merge", "--abort")
	return err
}

// MergeMessage returns the prepared merge commit message (first line).
func (r *Repo) MergeMessage() string {
	out, err := r.git("rev-parse", "--git-path", "MERGE_MSG")
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(out)
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.Root, p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	if i := strings.IndexByte(string(data), '\n'); i > 0 {
		return string(data[:i])
	}
	return strings.TrimSpace(string(data))
}

// stashMeta splits a stash description like "WIP on main: 1a2b3c msg" or
// "On feature/x: msg" into the branch and the message part.
func stashMeta(desc string) (branch, msg string) {
	s := desc
	switch {
	case strings.HasPrefix(s, "WIP on "):
		s = strings.TrimPrefix(s, "WIP on ")
	case strings.HasPrefix(s, "On "):
		s = strings.TrimPrefix(s, "On ")
	default:
		return "", desc
	}
	if i := strings.Index(s, ": "); i >= 0 {
		return s[:i], s[i+2:]
	}
	return "", desc
}

// ---- v0.6: discard, amend, tags, branch admin, file history ----

// DiscardFile throws away changes to one file: untracked files are deleted,
// tracked files restored from HEAD (index and worktree).
func (r *Repo) DiscardFile(f FileStatus) error {
	target := statusTarget(f.Path)
	if f.Untracked {
		_, err := r.git("clean", "-f", "--", target)
		return err
	}
	_, err := r.git("restore", "--staged", "--worktree", "--source=HEAD", "--", target)
	return err
}

// LastCommitMessage returns the full message of HEAD (for amend prefill).
func (r *Repo) LastCommitMessage() string {
	out, err := r.git("log", "-1", "--pretty=%s")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (r *Repo) CommitAmend(message string) error {
	_, err := r.git("commit", "--amend", "-m", message)
	return err
}

func (r *Repo) CreateTag(name, hash string) error {
	_, err := r.git("tag", name, hash)
	return err
}

func (r *Repo) DeleteTag(name string) error {
	_, err := r.git("tag", "-d", name)
	return err
}

func (r *Repo) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.git("branch", flag, name)
	return err
}

func (r *Repo) RenameBranch(oldName, newName string) error {
	_, err := r.git("branch", "-m", oldName, newName)
	return err
}

// FileHistory returns a compact log of every commit touching a path.
func (r *Repo) FileHistory(path string, limit int) ([]string, error) {
	out, err := r.git("log", "--follow", "-n", strconv.Itoa(limit),
		"--pretty=format:%h\x1f%ad\x1f%an\x1f%s", "--date=format:%Y-%m-%d", "--", path)
	if err != nil {
		return nil, err
	}
	lines := []string{"history of " + path, ""}
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		parts := strings.SplitN(l, "\x1f", 4)
		if len(parts) == 4 {
			lines = append(lines, fmt.Sprintf("%s  %s  %-18s %s", parts[0], parts[1], truncate(parts[2], 18), parts[3]))
		}
	}
	if len(lines) == 2 {
		lines = append(lines, "(no commits touch this file)")
	}
	return lines, nil
}

// Clone clones a URL into dir (network op, no prompts).
func gitClone(url, dest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--", url, dest)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}

func gitInit(dir string) error {
	out, err := exec.Command("git", "init", dir).CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

// TagsAt lists tag names pointing at a commit.
func (r *Repo) TagsAt(hash string) []string {
	out, err := r.git("tag", "--points-at", hash)
	if err != nil {
		return nil
	}
	var tags []string
	for _, t := range strings.Split(strings.TrimSpace(out), "\n") {
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// ---- v0.7: conflict resolution, hunks, blame ----

// gitIn runs git with input piped to stdin (for apply).
func (r *Repo) gitIn(stdin string, args ...string) (string, error) {
	full := append([]string{"-C", r.Root, "-c", "color.ui=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Stdin = strings.NewReader(stdin)
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

// isConflictError reports whether a merge-ish operation stopped on conflicts.
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "CONFLICT") || strings.Contains(s, "Automatic merge failed") ||
		strings.Contains(s, "could not apply") || strings.Contains(s, "conflict")
}

// isDirtyTreeError reports git refusing an operation because of local changes.
func isDirtyTreeError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "would be overwritten by merge") ||
		strings.Contains(s, "Please commit your changes or stash them") ||
		strings.Contains(s, "cannot rebase: You have unstaged changes") ||
		strings.Contains(s, "your index contains uncommitted changes")
}

// ResolveOurs / ResolveTheirs settle a conflicted file with one side and
// mark it resolved.
func (r *Repo) ResolveOurs(path string) error {
	if _, err := r.git("checkout", "--ours", "--", path); err != nil {
		return err
	}
	_, err := r.git("add", "--", path)
	return err
}

func (r *Repo) ResolveTheirs(path string) error {
	if _, err := r.git("checkout", "--theirs", "--", path); err != nil {
		return err
	}
	_, err := r.git("add", "--", path)
	return err
}

// rawHunks splits a raw git diff for one file into its file header and hunks.
func rawHunks(raw string) (header string, hunks []string) {
	lines := strings.Split(raw, "\n")
	var head []string
	var cur []string
	for _, l := range lines {
		if strings.HasPrefix(l, "@@") {
			if len(cur) > 0 {
				hunks = append(hunks, strings.Join(cur, "\n")+"\n")
			}
			cur = []string{l}
			continue
		}
		if len(cur) > 0 {
			cur = append(cur, l)
		} else {
			head = append(head, l)
		}
	}
	if len(cur) > 0 {
		h := strings.Join(cur, "\n")
		if !strings.HasSuffix(h, "\n") {
			h += "\n"
		}
		hunks = append(hunks, h)
	}
	return strings.Join(head, "\n") + "\n", hunks
}

// StageHunk applies a single hunk of a file's diff to the index (stage) or
// removes it from the index (unstage). idx is the hunk's position in the
// current diff for that side.
func (r *Repo) StageHunk(f FileStatus, idx int, unstage bool) error {
	args := []string{"diff"}
	if unstage {
		args = append(args, "--cached")
	}
	args = append(args, "--", statusTarget(f.Path))
	raw, err := r.git(args...)
	if err != nil {
		return err
	}
	header, hunks := rawHunks(raw)
	if idx < 0 || idx >= len(hunks) {
		return errors.New("hunk out of range — refresh and retry")
	}
	patch := header + hunks[idx]
	applyArgs := []string{"apply", "--cached"}
	if unstage {
		applyArgs = append(applyArgs, "--reverse")
	}
	_, err = r.gitIn(patch, applyArgs...)
	return err
}

// Blame returns an annotated view of a file at HEAD.
func (r *Repo) Blame(path string) ([]string, error) {
	out, err := r.git("blame", "--date=short", "--abbrev=7", "--", path)
	if err != nil {
		return nil, err
	}
	lines := []string{"blame of " + path, ""}
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		lines = append(lines, strings.ReplaceAll(l, "\t", "    "))
	}
	return lines, nil
}

// ---- v0.10: sidebar data ----

type Tag struct {
	Name string
	Hash string
}

func (r *Repo) Tags() ([]Tag, error) {
	out, err := r.git("for-each-ref", "refs/tags", "--sort=-creatordate",
		"--format=%(refname:short)\x1f%(objectname:short)")
	if err != nil {
		return nil, err
	}
	var tags []Tag
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "\x1f", 2)
		if len(parts) == 2 {
			tags = append(tags, Tag{Name: parts[0], Hash: parts[1]})
		}
	}
	return tags, nil
}
