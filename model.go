package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const commitLimit = 1500

// how often the background fetch runs when a remote is configured
const autoFetchEvery = 3 * time.Minute

type viewID int

const (
	viewCommits viewID = iota
	viewStatus
	viewBranches
	viewStashes
)

// prompt modal modes (single-line input at the footer)
const (
	promptNone = iota
	promptCommit
	promptStash
	promptOrigin
	promptBranch
	promptAmend
	promptTag
	promptRename
)

const (
	focusLeft = iota
	focusRight
)

type model struct {
	repo   *Repo
	width  int
	height int

	view  viewID
	focus int

	// commits view
	allRefs    bool // true = graph shows --all; false = current branch only
	commits    []Commit
	rows       []GraphRow
	visible    []int // indices into commits (filtered by search)
	sel        int   // index into visible
	offset     int
	details    []string
	detailFor  string
	detailOff  int
	loadingLog bool

	// search
	searching   bool
	searchInput textinput.Model
	query       string

	// status view: files + stashes flattened into selectable items,
	// rendered as rows (tree headers interleaved)
	files       []FileStatus
	statusItems []statusItem
	statusRows  []statusRow
	fileSel     int // index into statusItems
	fileOffset  int // scroll offset in row space
	fileDiff    []string
	diffFor     string
	diffOff     int

	// branches view
	branches []Branch
	brSel    int
	brOffset int
	brLog    []string
	brLogFor string
	brLogOff int

	// stashes view
	stashes   []Stash
	stSel     int
	stOff     int
	stDiff    []string
	stDiffFor string
	stDiffOff int

	// prompt modal (commit message, stash message, origin URL)
	promptMode  int
	promptInput textinput.Model

	// confirm modal (force push, merge, stash drop)
	confirmMsg string
	confirmCmd tea.Cmd

	// choice popup (e.g. blocked checkout: cancel / stash / discard)
	choiceTitle   string
	choiceOptions []choiceOption
	choiceSel     int

	// pending base for the new-branch prompt ("" = HEAD)
	branchBase string
	// pending targets for tag / rename prompts
	tagTarget  string
	renameFrom string

	// double-click detection
	lastClickAt time.Time
	lastClickY  int

	fetching bool
	head     HeadInfo
	showHelp bool
	flash    string
	flashErr bool
	flashAt  time.Time
}

type statusItem struct {
	isStash bool
	file    FileStatus
	stash   Stash
}

// statusRow is one rendered line in the status list: a selectable item or a
// section/directory header (item == -1).
type statusRow struct {
	item int
	text string
}

func itemKey(it statusItem) string {
	if it.isStash {
		return "stash:" + it.stash.Ref
	}
	return fileKey(it.file)
}

func (m *model) setFlash(text string, isErr bool) {
	m.flash = text
	m.flashErr = isErr
	m.flashAt = time.Now()
}

func newModel(repo *Repo) model {
	si := textinput.New()
	si.Placeholder = "filter by subject, author or hash…"
	si.Prompt = "/ "
	si.CharLimit = 120

	pi := textinput.New()
	pi.CharLimit = 300

	return model{
		repo:        repo,
		allRefs:     true,
		searchInput: si,
		promptInput: pi,
	}
}

func (m *model) openPrompt(mode int, prompt, placeholder string) tea.Cmd {
	m.promptMode = mode
	m.promptInput.Prompt = prompt
	m.promptInput.Placeholder = placeholder
	m.promptInput.SetValue("")
	m.promptInput.Focus()
	return textinput.Blink
}

// ---- messages ----

type commitsMsg struct {
	commits []Commit
	err     error
}
type detailsMsg struct {
	hash  string
	lines []string
	err   error
}
type statusListMsg struct {
	files []FileStatus
	err   error
}
type fileDiffMsg struct {
	key   string
	lines []string
	err   error
}
type branchesMsg struct {
	branches []Branch
	err      error
}
type branchLogMsg struct {
	name  string
	lines []string
	err   error
}
type headMsg HeadInfo
type actionMsg struct {
	text        string
	err         error
	reload      bool
	gotoCommits bool // jump to the graph after success (post-commit)
}
type stashesMsg struct {
	stashes []Stash
	err     error
}
type stashDiffMsg struct {
	ref   string
	lines []string
	err   error
}
type fetchDoneMsg struct {
	err    error
	manual bool
}
type autoFetchMsg struct{}
type flashTickMsg struct{}
type openTagPromptMsg struct{ hash string }
type branchDeleteBlockedMsg struct{ name string }
type branchPopupMsg struct{ branches []Branch }

type choiceOption struct {
	label string
	cmd   tea.Cmd
}

// checkoutBlockedMsg opens the popup when git refuses to switch because of
// local changes. target is what we tried to check out.
type checkoutBlockedMsg struct {
	target   string
	desc     string // human name shown in the popup
	isCommit bool
}

// ---- commands ----

func (m model) loadCommits() tea.Cmd {
	r := m.repo
	all := m.allRefs
	return func() tea.Msg {
		commits, err := r.Commits(commitLimit, all)
		return commitsMsg{commits, err}
	}
}

func (m model) loadDetails(hash string) tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		lines, err := r.CommitDetails(hash)
		return detailsMsg{hash, lines, err}
	}
}

func (m model) loadStatus() tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		files, err := r.Status()
		return statusListMsg{files, err}
	}
}

func (m model) loadFileDiff(f FileStatus) tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		lines, err := r.FileDiff(f)
		return fileDiffMsg{fileKey(f), lines, err}
	}
}

func (m model) loadBranches() tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		branches, err := r.Branches()
		return branchesMsg{branches, err}
	}
}

func (m model) loadBranchLog(name string) tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		lines, err := r.BranchLog(name, 100)
		return branchLogMsg{name, lines, err}
	}
}

func (m model) loadHead() tea.Cmd {
	r := m.repo
	return func() tea.Msg { return headMsg(r.Head()) }
}

func (m model) loadStashes() tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		stashes, err := r.Stashes()
		return stashesMsg{stashes, err}
	}
}

func (m model) loadStashDiff(ref string) tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		lines, err := r.StashDiff(ref)
		return stashDiffMsg{ref, lines, err}
	}
}

func (m model) doFetch(manual bool) tea.Cmd {
	r := m.repo
	return func() tea.Msg { return fetchDoneMsg{r.Fetch(), manual} }
}

func (m model) doPull() tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		if err := r.Pull(); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: "✓ pulled (fast-forward)", reload: true}
	}
}

func (m model) doPush(force bool) tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		text, err := r.Push(force)
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: text, reload: true}
	}
}

func autoFetchTick() tea.Cmd {
	return tea.Tick(autoFetchEvery, func(time.Time) tea.Msg { return autoFetchMsg{} })
}

func flashTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return flashTickMsg{} })
}

func (m model) openBranchPopup() tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		branches, err := r.Branches()
		if err != nil {
			return actionMsg{err: err}
		}
		return branchPopupMsg{branches}
	}
}

func (m model) loadItemDiff(it statusItem) tea.Cmd {
	r := m.repo
	if it.isStash {
		ref := it.stash.Ref
		return func() tea.Msg {
			lines, err := r.StashDiff(ref)
			return fileDiffMsg{"stash:" + ref, lines, err}
		}
	}
	return m.loadFileDiff(it.file)
}

// rebuildStatusRows flattens files (grouped: conflicts / staged / changes,
// with directory headers) plus stashes into the selectable status list.
func (m *model) rebuildStatusRows() {
	m.statusItems = m.statusItems[:0]
	m.statusRows = m.statusRows[:0]
	header := func(t string) {
		m.statusRows = append(m.statusRows, statusRow{item: -1, text: t})
	}
	addFiles := func(title string, files []FileStatus) {
		if len(files) == 0 {
			return
		}
		header(title)
		sort.Slice(files, func(a, b int) bool {
			return statusTarget(files[a].Path) < statusTarget(files[b].Path)
		})
		lastDir := "."
		for _, f := range files {
			dir := filepath.Dir(statusTarget(f.Path))
			if dir != lastDir {
				if dir != "." {
					header("   ▾ " + dir + "/")
				}
				lastDir = dir
			}
			idx := len(m.statusItems)
			m.statusItems = append(m.statusItems, statusItem{file: f})
			m.statusRows = append(m.statusRows, statusRow{item: idx})
		}
	}
	var conflicts, staged, unstaged []FileStatus
	for _, f := range m.files {
		switch {
		case f.Conflict:
			conflicts = append(conflicts, f)
		case f.Staged:
			staged = append(staged, f)
		default:
			unstaged = append(unstaged, f)
		}
	}
	addFiles("✗ Conflicts — fix, then space to mark resolved", conflicts)
	addFiles("● Staged", staged)
	addFiles("○ Changes", unstaged)
	if len(m.stashes) > 0 {
		header(fmt.Sprintf("≡ Stashes (%d) — ⏎ apply · x drop", len(m.stashes)))
		for _, st := range m.stashes {
			idx := len(m.statusItems)
			m.statusItems = append(m.statusItems, statusItem{isStash: true, stash: st})
			m.statusRows = append(m.statusRows, statusRow{item: idx})
		}
	}
	if m.fileSel >= len(m.statusItems) {
		m.fileSel = max(len(m.statusItems)-1, 0)
	}
}

// itemRow maps an item index to its row index for scrolling.
func (m model) itemRow(i int) int {
	for r, row := range m.statusRows {
		if row.item == i {
			return r
		}
	}
	return 0
}

func (m model) moveItem(delta int) (tea.Model, tea.Cmd) {
	if len(m.statusItems) == 0 {
		return m, nil
	}
	m.fileSel = clamp(m.fileSel+delta, 0, len(m.statusItems)-1)
	ensureVisible(m.itemRow(m.fileSel), &m.fileOffset, m.listHeight())
	it := m.statusItems[m.fileSel]
	if itemKey(it) != m.diffFor {
		return m, m.loadItemDiff(it)
	}
	return m, nil
}

// toggleStage stages/unstages a file, or marks a conflict as resolved.
func (m model) toggleStage(it statusItem) tea.Cmd {
	if it.isStash {
		return nil
	}
	f := it.file
	r := m.repo
	return func() tea.Msg {
		var err error
		var verb string
		switch {
		case f.Conflict:
			err, verb = r.StageFile(f), "marked resolved:"
		case f.Staged:
			err, verb = r.UnstageFile(f), "unstaged"
		default:
			err, verb = r.StageFile(f), "staged"
		}
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: "✓ " + verb + " " + f.Path, reload: true}
	}
}

func (m model) loadFileHistory(f FileStatus) tea.Cmd {
	r := m.repo
	path := statusTarget(f.Path)
	return func() tea.Msg {
		lines, err := r.FileHistory(path, 200)
		return fileDiffMsg{"hist:" + path, lines, err}
	}
}

// checkoutSelectedCommit prefers a local branch pointing at the commit.
func (m model) checkoutSelectedCommit() tea.Cmd {
	c, ok := m.selectedCommit()
	if !ok {
		return nil
	}
	for _, ref := range c.Refs {
		if ref.Kind == RefBranch || (ref.Kind == RefHead && ref.Name != "HEAD") {
			return m.doCheckout(ref.Name, ref.Name, false)
		}
	}
	return m.doCheckout(c.Hash, c.ShortHash(), true)
}

// doCheckout tries to switch to a branch or commit; when git refuses because
// of local changes it opens the cancel/stash/discard popup instead of failing.
func (m model) doCheckout(target, desc string, isCommit bool) tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		var text string
		var err error
		if isCommit {
			text, err = r.CheckoutCommit(target)
		} else {
			text, err = r.CheckoutBranch(target)
		}
		if isBlockedCheckout(err) {
			return checkoutBlockedMsg{target: target, desc: desc, isCommit: isCommit}
		}
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: text, reload: true}
	}
}

func (m model) stashSwitchReapply(target string, isCommit bool) tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		if err := r.StashPush("git2: auto-stash before switching"); err != nil {
			return actionMsg{err: err}
		}
		var text string
		var err error
		if isCommit {
			text, err = r.CheckoutCommit(target)
		} else {
			text, err = r.CheckoutBranch(target)
		}
		if err != nil {
			_ = r.StashPop("stash@{0}") // roll the stash back where we started
			return actionMsg{err: err}
		}
		if err := r.StashPop("stash@{0}"); err != nil {
			return actionMsg{text: text + " · stash re-apply conflicted — resolve in Status view (stash kept)", reload: true}
		}
		return actionMsg{text: text + " · changes re-applied", reload: true}
	}
}

func (m model) discardSwitch(target string, isCommit bool) tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		if err := r.DiscardAll(); err != nil {
			return actionMsg{err: err}
		}
		var text string
		var err error
		if isCommit {
			text, err = r.CheckoutCommit(target)
		} else {
			text, err = r.CheckoutBranch(target)
		}
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: text + " · local changes discarded", reload: true}
	}
}

func fileKey(f FileStatus) string {
	side := "u"
	if f.Staged {
		side = "s"
	}
	return side + ":" + f.Path
}

func (m model) Init() tea.Cmd {
	m.loadingLog = true
	return tea.Batch(m.loadCommits(), m.loadHead(), autoFetchTick(), flashTick())
}

// ---- geometry helpers ----

// header (1) + tab bar with its underline (2) + message line (1) + footer (1)
func (m model) bodyHeight() int { return max(m.height-5, 1) }

// listHeight is the number of content rows inside a bordered pane.
func (m model) listHeight() int { return max(m.bodyHeight()-2, 1) }

func (m model) leftWidth() int {
	w := m.width * 11 / 20
	return clamp(w, 30, max(m.width-28, 30))
}

func (m model) rightWidth() int { return max(m.width-m.leftWidth(), 20) }

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ensureVisible(sel int, offset *int, height int) {
	if sel < *offset {
		*offset = sel
	}
	if sel >= *offset+height {
		*offset = sel - height + 1
	}
	if *offset < 0 {
		*offset = 0
	}
}

// ---- filtering ----

func (m *model) applyFilter() {
	m.visible = m.visible[:0]
	q := strings.ToLower(m.query)
	for i, c := range m.commits {
		if q == "" ||
			strings.Contains(strings.ToLower(c.Subject), q) ||
			strings.Contains(strings.ToLower(c.Author), q) ||
			strings.HasPrefix(strings.ToLower(c.Hash), q) {
			m.visible = append(m.visible, i)
		}
	}
	if m.sel >= len(m.visible) {
		m.sel = max(len(m.visible)-1, 0)
	}
	m.offset = 0
	ensureVisible(m.sel, &m.offset, m.listHeight())
}

func (m model) selectedCommit() (Commit, bool) {
	if len(m.visible) == 0 || m.sel >= len(m.visible) {
		return Commit{}, false
	}
	return m.commits[m.visible[m.sel]], true
}

// ---- update ----

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		ensureVisible(m.sel, &m.offset, m.listHeight())
		return m, nil

	case commitsMsg:
		m.loadingLog = false
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		m.commits = msg.commits
		m.rows = BuildGraph(m.commits)
		m.applyFilter()
		if c, ok := m.selectedCommit(); ok && c.Hash != m.detailFor {
			return m, m.loadDetails(c.Hash)
		}
		return m, nil

	case detailsMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		if c, ok := m.selectedCommit(); ok && c.Hash == msg.hash {
			m.details, m.detailFor, m.detailOff = msg.lines, msg.hash, 0
		}
		return m, nil

	case statusListMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		m.files = msg.files
		m.rebuildStatusRows()
		ensureVisible(m.itemRow(m.fileSel), &m.fileOffset, m.listHeight())
		if len(m.statusItems) > 0 {
			return m, m.loadItemDiff(m.statusItems[m.fileSel])
		}
		m.fileDiff, m.diffFor = nil, ""
		return m, nil

	case fileDiffMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		match := false
		if len(m.statusItems) > 0 && m.fileSel < len(m.statusItems) {
			it := m.statusItems[m.fileSel]
			match = itemKey(it) == msg.key ||
				(!it.isStash && msg.key == "hist:"+statusTarget(it.file.Path))
		}
		if match {
			m.fileDiff, m.diffFor, m.diffOff = msg.lines, msg.key, 0
		}
		return m, nil

	case branchesMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		m.branches = msg.branches
		if m.brSel >= len(m.branches) {
			m.brSel = max(len(m.branches)-1, 0)
		}
		ensureVisible(m.brSel, &m.brOffset, m.listHeight())
		if len(m.branches) > 0 {
			return m, m.loadBranchLog(m.branches[m.brSel].Name)
		}
		return m, nil

	case branchLogMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		if len(m.branches) > 0 && m.brSel < len(m.branches) && m.branches[m.brSel].Name == msg.name {
			m.brLog, m.brLogFor, m.brLogOff = msg.lines, msg.name, 0
		}
		return m, nil

	case headMsg:
		m.head = HeadInfo(msg)
		return m, nil

	case stashesMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		m.stashes = msg.stashes
		m.rebuildStatusRows()
		if m.stSel >= len(m.stashes) {
			m.stSel = max(len(m.stashes)-1, 0)
		}
		ensureVisible(m.stSel, &m.stOff, m.listHeight())
		if m.view == viewStashes && len(m.stashes) > 0 {
			return m, m.loadStashDiff(m.stashes[m.stSel].Ref)
		}
		if len(m.stashes) == 0 {
			m.stDiff, m.stDiffFor = nil, ""
		}
		return m, nil

	case stashDiffMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		if len(m.stashes) > 0 && m.stSel < len(m.stashes) && m.stashes[m.stSel].Ref == msg.ref {
			m.stDiff, m.stDiffFor, m.stDiffOff = msg.lines, msg.ref, 0
		}
		return m, nil

	case fetchDoneMsg:
		m.fetching = false
		if msg.err != nil {
			if msg.manual {
				m.setFlash(msg.err.Error(), true)
			}
			return m, nil
		}
		if msg.manual {
			m.setFlash("✓ fetched all remotes", false)
		}
		return m, m.refresh()

	case autoFetchMsg:
		if m.head.HasRemote && !m.fetching {
			m.fetching = true
			return m, tea.Batch(m.doFetch(false), autoFetchTick())
		}
		return m, autoFetchTick()

	case checkoutBlockedMsg:
		m.choiceTitle = "You have local changes — switch to " + msg.desc + "?"
		m.choiceSel = 0
		m.choiceOptions = []choiceOption{
			{label: "Don't switch — keep working here", cmd: nil},
			{label: "Stash changes, switch, re-apply them", cmd: m.stashSwitchReapply(msg.target, msg.isCommit)},
			{label: "Discard changes and switch  ⚠ irreversible", cmd: m.discardSwitch(msg.target, msg.isCommit)},
		}
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, m.loadHead()
		} else {
			m.setFlash(msg.text, false)
			if msg.gotoCommits {
				m.view = viewCommits
				m.focus = focusLeft
			}
		}
		if msg.reload {
			return m, m.refresh()
		}
		return m, nil

	case flashTickMsg:
		if m.flash != "" && !m.flashAt.IsZero() && time.Since(m.flashAt) > 4*time.Second {
			m.flash = ""
		}
		return m, flashTick()

	case branchDeleteBlockedMsg:
		name := msg.name
		r := m.repo
		m.choiceTitle = name + " has commits that are not merged anywhere"
		m.choiceSel = 0
		m.choiceOptions = []choiceOption{
			{label: "Keep the branch", cmd: nil},
			{label: "Force delete — the commits will be lost  ⚠", cmd: func() tea.Msg {
				if err := r.DeleteBranch(name, true); err != nil {
					return actionMsg{err: err}
				}
				return actionMsg{text: "✓ force-deleted branch " + name, reload: true}
			}},
		}
		return m, nil

	case openTagPromptMsg:
		m.tagTarget = msg.hash
		return m, m.openPrompt(promptTag, "⌂ ", "tag name for "+msg.hash[:min(8, len(msg.hash))])

	case branchPopupMsg:
		var opts []choiceOption
		for _, b := range msg.branches {
			if b.Remote {
				continue
			}
			if b.Current {
				opts = append(opts, choiceOption{label: "✓ " + b.Name + "  (current)", cmd: nil})
				continue
			}
			name := b.Name
			opts = append(opts, choiceOption{label: "⎇ " + name, cmd: m.doCheckout(name, name, false)})
		}
		if len(opts) == 0 {
			m.setFlash("no local branches", false)
			return m, nil
		}
		m.choiceTitle = "Switch branch"
		m.choiceOptions = opts
		m.choiceSel = 0
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) refresh() tea.Cmd {
	cmds := []tea.Cmd{m.loadHead()}
	switch m.view {
	case viewCommits:
		cmds = append(cmds, m.loadCommits())
	case viewStatus:
		cmds = append(cmds, m.loadStatus(), m.loadStashes())
	case viewBranches:
		cmds = append(cmds, m.loadBranches())
	case viewStashes:
		cmds = append(cmds, m.loadStashes())
	}
	return tea.Batch(cmds...)
}

func (m model) switchView(v viewID) (tea.Model, tea.Cmd) {
	m.view = v
	m.focus = focusLeft
	m.flash = ""
	switch v {
	case viewCommits:
		return m, tea.Batch(m.loadCommits(), m.loadHead())
	case viewStatus:
		return m, tea.Batch(m.loadStatus(), m.loadStashes(), m.loadHead())
	case viewBranches:
		return m, tea.Batch(m.loadBranches(), m.loadHead())
	case viewStashes:
		return m, tea.Batch(m.loadStashes(), m.loadHead())
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// modal: help overlay
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	// modal: choice popup
	if len(m.choiceOptions) > 0 {
		switch key {
		case "esc", "q":
			m.choiceTitle, m.choiceOptions = "", nil
			m.setFlash("cancelled", false)
			return m, nil
		case "j", "s", "down", "tab":
			m.choiceSel = (m.choiceSel + 1) % len(m.choiceOptions)
			return m, nil
		case "k", "w", "up":
			m.choiceSel = (m.choiceSel + len(m.choiceOptions) - 1) % len(m.choiceOptions)
			return m, nil
		case "enter":
			cmd := m.choiceOptions[m.choiceSel].cmd
			m.choiceTitle, m.choiceOptions = "", nil
			if cmd == nil {
				m.setFlash("staying put", false)
			}
			return m, cmd
		}
		if n := int(key[0] - '0'); len(key) == 1 && n >= 1 && n <= len(m.choiceOptions) {
			cmd := m.choiceOptions[n-1].cmd
			m.choiceTitle, m.choiceOptions = "", nil
			if cmd == nil {
				m.setFlash("staying put", false)
			}
			return m, cmd
		}
		return m, nil
	}

	// modal: confirm y/n
	if m.confirmMsg != "" {
		cmd := m.confirmCmd
		m.confirmMsg, m.confirmCmd = "", nil
		if key == "y" || key == "Y" || key == "enter" {
			return m, cmd
		}
		m.setFlash("cancelled", false)
		return m, nil
	}

	// modal: single-line prompt (commit / stash message, origin URL)
	if m.promptMode != promptNone {
		switch key {
		case "esc":
			m.promptMode = promptNone
			m.promptInput.Blur()
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.promptInput.Value())
			mode := m.promptMode
			if mode == promptCommit && value == "" {
				return m, nil // a commit needs a message
			}
			if mode == promptOrigin && value == "" {
				return m, nil
			}
			if (mode == promptBranch || mode == promptAmend || mode == promptTag || mode == promptRename) && value == "" {
				return m, nil
			}
			m.promptMode = promptNone
			m.promptInput.Blur()
			r := m.repo
			switch mode {
			case promptCommit:
				return m, func() tea.Msg {
					if err := r.Commit(value); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{text: "✓ committed: " + value, reload: true, gotoCommits: true}
				}
			case promptStash:
				return m, func() tea.Msg {
					if err := r.StashPush(value); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{text: "✓ stashed working tree", reload: true}
				}
			case promptOrigin:
				return m, func() tea.Msg {
					if err := r.AddRemote("origin", value); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{text: "✓ added origin " + value + " — press f to fetch", reload: true}
				}
			case promptBranch:
				base := m.branchBase
				return m, func() tea.Msg {
					if err := r.CreateBranch(value, base); err != nil {
						return actionMsg{err: err}
					}
					from := ""
					if base != "" {
						from = " at " + base[:min(8, len(base))]
					}
					return actionMsg{text: "✓ created & switched to " + value + from, reload: true}
				}
			case promptAmend:
				return m, func() tea.Msg {
					if err := r.CommitAmend(value); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{text: "✓ amended: " + value, reload: true, gotoCommits: true}
				}
			case promptTag:
				hash := m.tagTarget
				return m, func() tea.Msg {
					if err := r.CreateTag(value, hash); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{text: "✓ tagged " + hash[:min(8, len(hash))] + " as " + value, reload: true}
				}
			case promptRename:
				oldName := m.renameFrom
				return m, func() tea.Msg {
					if err := r.RenameBranch(oldName, value); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{text: "✓ renamed " + oldName + " → " + value, reload: true}
				}
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.promptInput, cmd = m.promptInput.Update(msg)
		return m, cmd
	}

	// modal: search input
	if m.searching {
		switch key {
		case "esc":
			m.searching = false
			m.searchInput.Blur()
			m.searchInput.SetValue("")
			m.query = ""
			m.applyFilter()
			return m, nil
		case "enter":
			m.searching = false
			m.searchInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != m.query {
			m.query = m.searchInput.Value()
			m.applyFilter()
			if c, ok := m.selectedCommit(); ok && c.Hash != m.detailFor {
				return m, tea.Batch(cmd, m.loadDetails(c.Hash))
			}
		}
		return m, cmd
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "r":
		m.flash = ""
		return m, m.refresh()
	case "1":
		return m.switchView(viewCommits)
	case "2":
		return m.switchView(viewStatus)
	case "3":
		return m.switchView(viewBranches)
	case "4":
		return m.switchView(viewStashes)
	case "f":
		if !m.head.HasRemote {
			m.setFlash("no remote configured — press o to add origin", true)
			return m, nil
		}
		m.fetching = true
		m.setFlash("⇣ fetching…", false)
		return m, m.doFetch(true)
	case "p":
		if m.view == viewStashes {
			break // handled below: pop
		}
		if !m.head.HasRemote {
			m.setFlash("no remote configured — press o to add origin", true)
			return m, nil
		}
		m.setFlash("⇣ pulling…", false)
		return m, m.doPull()
	case "P":
		if !m.head.HasRemote {
			m.setFlash("no remote configured — press o to add origin", true)
			return m, nil
		}
		m.setFlash("⇡ pushing…", false)
		return m, m.doPush(false)
	case "F":
		if !m.head.HasRemote {
			m.setFlash("no remote configured — press o to add origin", true)
			return m, nil
		}
		m.confirmMsg = "Force-push " + m.head.Branch + " (with lease)? y/N"
		m.confirmCmd = m.doPush(true)
		return m, nil
	case "o":
		if url := m.repo.RemoteURL("origin"); url != "" {
			m.setFlash("origin → "+url, false)
			return m, nil
		}
		return m, m.openPrompt(promptOrigin, "⇄ origin url: ", "git@github.com:user/repo.git or https://…")
	case "tab":
		return m.switchView((m.view + 1) % 4)
	case "shift+tab":
		return m.switchView((m.view + 3) % 4)
	case "left", "right", "h", "l", "a", "d":
		if key == "left" || key == "h" || key == "a" {
			m.focus = focusLeft
		} else {
			m.focus = focusRight
		}
		return m, nil
	}

	switch m.view {
	case viewCommits:
		return m.handleCommitsKey(key)
	case viewStatus:
		return m.handleStatusKey(key)
	case viewBranches:
		return m.handleBranchesKey(key)
	case viewStashes:
		return m.handleStashesKey(key)
	}
	return m, nil
}

func (m model) handleStashesKey(key string) (tea.Model, tea.Cmd) {
	page := m.listHeight()
	if m.focus == focusRight {
		maxOff := max(len(m.stDiff)-m.listHeight(), 0)
		switch key {
		case "j", "s", "down":
			m.stDiffOff = min(m.stDiffOff+1, maxOff)
		case "k", "w", "up":
			m.stDiffOff = max(m.stDiffOff-1, 0)
		case "ctrl+d", "pgdown", " ":
			m.stDiffOff = min(m.stDiffOff+page/2, maxOff)
		case "ctrl+u", "pgup":
			m.stDiffOff = max(m.stDiffOff-page/2, 0)
		case "g", "home":
			m.stDiffOff = 0
		case "G", "end":
			m.stDiffOff = maxOff
		case "esc":
			m.focus = focusLeft
		}
		return m, nil
	}
	moveStash := func(delta int) (tea.Model, tea.Cmd) {
		if len(m.stashes) == 0 {
			return m, nil
		}
		m.stSel = clamp(m.stSel+delta, 0, len(m.stashes)-1)
		ensureVisible(m.stSel, &m.stOff, m.listHeight())
		st := m.stashes[m.stSel]
		if st.Ref != m.stDiffFor {
			return m, m.loadStashDiff(st.Ref)
		}
		return m, nil
	}
	switch key {
	case "j", "s", "down":
		return moveStash(1)
	case "k", "w", "up":
		return moveStash(-1)
	case "ctrl+d", "pgdown":
		return moveStash(page / 2)
	case "ctrl+u", "pgup":
		return moveStash(-page / 2)
	case "g", "home":
		return moveStash(-len(m.stashes))
	case "G", "end":
		return moveStash(len(m.stashes))
	case "enter":
		if len(m.stashes) == 0 {
			return m, nil
		}
		st := m.stashes[m.stSel]
		r := m.repo
		return m, func() tea.Msg {
			if err := r.StashApply(st.Ref); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ applied " + st.Ref + " (kept in list)", reload: true}
		}
	case "p":
		if len(m.stashes) == 0 {
			return m, nil
		}
		st := m.stashes[m.stSel]
		r := m.repo
		return m, func() tea.Msg {
			if err := r.StashPop(st.Ref); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ popped " + st.Ref, reload: true}
		}
	case "x":
		if len(m.stashes) == 0 {
			return m, nil
		}
		st := m.stashes[m.stSel]
		r := m.repo
		m.confirmMsg = "Drop " + st.Ref + " (" + truncate(st.Desc, 40) + ")? y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.StashDrop(st.Ref); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ dropped " + st.Ref, reload: true}
		}
		return m, nil
	}
	return m, nil
}

func (m model) moveSel(delta int) (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	m.sel = clamp(m.sel+delta, 0, len(m.visible)-1)
	ensureVisible(m.sel, &m.offset, m.listHeight())
	if c, ok := m.selectedCommit(); ok && c.Hash != m.detailFor {
		return m, m.loadDetails(c.Hash)
	}
	return m, nil
}

func (m model) handleCommitsKey(key string) (tea.Model, tea.Cmd) {
	page := m.listHeight()
	if m.focus == focusRight {
		maxOff := max(len(m.details)-m.listHeight(), 0)
		switch key {
		case "j", "s", "down":
			m.detailOff = min(m.detailOff+1, maxOff)
		case "k", "w", "up":
			m.detailOff = max(m.detailOff-1, 0)
		case "ctrl+d", "pgdown", " ":
			m.detailOff = min(m.detailOff+page/2, maxOff)
		case "ctrl+u", "pgup":
			m.detailOff = max(m.detailOff-page/2, 0)
		case "g", "home":
			m.detailOff = 0
		case "G", "end":
			m.detailOff = maxOff
		case "esc":
			m.focus = focusLeft
		}
		return m, nil
	}
	switch key {
	case "j", "s", "down":
		return m.moveSel(1)
	case "k", "w", "up":
		return m.moveSel(-1)
	case "ctrl+d", "pgdown":
		return m.moveSel(page / 2)
	case "ctrl+u", "pgup":
		return m.moveSel(-page / 2)
	case "g", "home":
		return m.moveSel(-len(m.visible))
	case "G", "end":
		return m.moveSel(len(m.visible))
	case "/":
		m.searching = true
		m.searchInput.Focus()
		return m, textinput.Blink
	case "esc":
		if m.query != "" {
			m.query = ""
			m.searchInput.SetValue("")
			m.applyFilter()
		}
		return m, nil
	case "enter":
		m.focus = focusRight
		return m, nil
	case "c":
		return m, m.checkoutSelectedCommit()
	case "t":
		m.allRefs = !m.allRefs
		if m.allRefs {
			m.setFlash("showing all branches", false)
		} else {
			m.setFlash("⎇ branch focus: "+m.head.Branch+" only — t to show all", false)
		}
		return m, m.loadCommits()
	case "b":
		return m, m.openBranchPopup()
	case "n":
		c, ok := m.selectedCommit()
		if !ok {
			return m, nil
		}
		m.branchBase = c.Hash
		return m, m.openPrompt(promptBranch, "⎇ ", "new branch name (from "+c.ShortHash()+")")
	case "T":
		c, ok := m.selectedCommit()
		if !ok {
			return m, nil
		}
		tags := m.repo.TagsAt(c.Hash)
		if len(tags) == 0 {
			m.tagTarget = c.Hash
			return m, m.openPrompt(promptTag, "⌂ ", "tag name for "+c.ShortHash())
		}
		// commit already tagged: offer create + delete
		opts := []choiceOption{{label: "Create another tag on " + c.ShortHash(), cmd: func() tea.Msg { return openTagPromptMsg{c.Hash} }}}
		r := m.repo
		for _, t := range tags {
			tag := t
			opts = append(opts, choiceOption{label: "Delete tag " + tag, cmd: func() tea.Msg {
				if err := r.DeleteTag(tag); err != nil {
					return actionMsg{err: err}
				}
				return actionMsg{text: "✓ deleted tag " + tag, reload: true}
			}})
		}
		m.choiceTitle = "Tags on " + c.ShortHash()
		m.choiceOptions = opts
		m.choiceSel = 0
		return m, nil
	case "m":
		c, ok := m.selectedCommit()
		if !ok {
			return m, nil
		}
		r := m.repo
		hash := c.Hash
		m.confirmMsg = "Merge " + c.ShortHash() + " into " + m.head.Branch + "? y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.Merge(hash); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ merged " + hash[:8] + " into " + m.head.Branch, reload: true}
		}
		return m, nil
	case "y":
		c, ok := m.selectedCommit()
		if !ok {
			return m, nil
		}
		r := m.repo
		hash := c.Hash
		m.confirmMsg = "Cherry-pick " + c.ShortHash() + " onto " + m.head.Branch + "? y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.CherryPick(hash); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ cherry-picked " + hash[:8] + " onto " + m.head.Branch, reload: true}
		}
		return m, nil
	case "R":
		c, ok := m.selectedCommit()
		if !ok {
			return m, nil
		}
		r := m.repo
		hash := c.Hash
		m.confirmMsg = "Rebase " + m.head.Branch + " onto " + c.ShortHash() + "? y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.Rebase(hash); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ rebased " + m.head.Branch + " onto " + hash[:8], reload: true}
		}
		return m, nil
	case "v":
		c, ok := m.selectedCommit()
		if !ok {
			return m, nil
		}
		r := m.repo
		hash := c.Hash
		m.confirmMsg = "Revert " + c.ShortHash() + " (creates a new commit)? y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.Revert(hash); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ reverted " + hash[:8], reload: true}
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleStatusKey(key string) (tea.Model, tea.Cmd) {
	page := m.listHeight()
	if m.focus == focusRight {
		maxOff := max(len(m.fileDiff)-m.listHeight(), 0)
		switch key {
		case "j", "s", "down":
			m.diffOff = min(m.diffOff+1, maxOff)
		case "k", "w", "up":
			m.diffOff = max(m.diffOff-1, 0)
		case "ctrl+d", "pgdown", " ":
			m.diffOff = min(m.diffOff+page/2, maxOff)
		case "ctrl+u", "pgup":
			m.diffOff = max(m.diffOff-page/2, 0)
		case "g", "home":
			m.diffOff = 0
		case "G", "end":
			m.diffOff = maxOff
		case "esc":
			m.focus = focusLeft
		}
		return m, nil
	}
	switch key {
	case "j", "s", "down":
		return m.moveItem(1)
	case "k", "w", "up":
		return m.moveItem(-1)
	case "ctrl+d", "pgdown":
		return m.moveItem(page / 2)
	case "ctrl+u", "pgup":
		return m.moveItem(-page / 2)
	case "g", "home":
		return m.moveItem(-len(m.statusItems))
	case "G", "end":
		return m.moveItem(len(m.statusItems))
	case " ":
		if len(m.statusItems) == 0 {
			return m, nil
		}
		it := m.statusItems[m.fileSel]
		if it.isStash {
			m.setFlash("⏎ applies a stash · x drops it", false)
			return m, nil
		}
		return m, m.toggleStage(it)
	case "enter":
		if len(m.statusItems) == 0 {
			return m, nil
		}
		it := m.statusItems[m.fileSel]
		if it.isStash {
			ref := it.stash.Ref
			r := m.repo
			return m, func() tea.Msg {
				if err := r.StashApply(ref); err != nil {
					return actionMsg{err: err}
				}
				return actionMsg{text: "✓ applied " + ref, reload: true}
			}
		}
		m.focus = focusRight
		return m, nil
	case "x":
		if len(m.statusItems) == 0 {
			return m, nil
		}
		if it := m.statusItems[m.fileSel]; it.isStash {
			st := it.stash
			r := m.repo
			m.confirmMsg = "Drop " + st.Ref + " (" + truncate(st.Desc, 40) + ")? y/N"
			m.confirmCmd = func() tea.Msg {
				if err := r.StashDrop(st.Ref); err != nil {
					return actionMsg{err: err}
				}
				return actionMsg{text: "✓ dropped " + st.Ref, reload: true}
			}
		}
		return m, nil
	case "D":
		if len(m.statusItems) == 0 {
			return m, nil
		}
		it := m.statusItems[m.fileSel]
		if it.isStash {
			return m, nil
		}
		f := it.file
		r := m.repo
		m.confirmMsg = "Discard changes to " + f.Path + "? This cannot be undone. y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.DiscardFile(f); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ discarded " + f.Path, reload: true}
		}
		return m, nil
	case "A":
		cmd := m.openPrompt(promptAmend, "✎ ", "amend last commit message")
		if msg := m.repo.LastCommitMessage(); msg != "" {
			m.promptInput.SetValue(msg)
		}
		return m, cmd
	case "H":
		if len(m.statusItems) == 0 {
			return m, nil
		}
		it := m.statusItems[m.fileSel]
		if it.isStash {
			return m, nil
		}
		if strings.HasPrefix(m.diffFor, "hist:") {
			return m, m.loadItemDiff(it) // toggle back to the diff
		}
		return m, m.loadFileHistory(it.file)
	case "X":
		if !m.head.Merging {
			return m, nil
		}
		r := m.repo
		m.confirmMsg = "Abort the merge and go back? y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.MergeAbort(); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ merge aborted", reload: true}
		}
		return m, nil
	case "c":
		cmd := m.openPrompt(promptCommit, "✎ ", "commit message")
		if m.head.Merging {
			if msg := m.repo.MergeMessage(); msg != "" {
				m.promptInput.SetValue(msg)
			}
		}
		return m, cmd
	case "S":
		hasFile := false
		for _, it := range m.statusItems {
			if !it.isStash {
				hasFile = true
				break
			}
		}
		if !hasFile {
			m.setFlash("nothing to stash", false)
			return m, nil
		}
		return m, m.openPrompt(promptStash, "≡ ", "stash message (optional)")
	}
	return m, nil
}
func (m model) handleBranchesKey(key string) (tea.Model, tea.Cmd) {
	page := m.listHeight()
	if m.focus == focusRight {
		maxOff := max(len(m.brLog)-m.listHeight(), 0)
		switch key {
		case "j", "s", "down":
			m.brLogOff = min(m.brLogOff+1, maxOff)
		case "k", "w", "up":
			m.brLogOff = max(m.brLogOff-1, 0)
		case "ctrl+d", "pgdown", " ":
			m.brLogOff = min(m.brLogOff+page/2, maxOff)
		case "ctrl+u", "pgup":
			m.brLogOff = max(m.brLogOff-page/2, 0)
		case "g", "home":
			m.brLogOff = 0
		case "G", "end":
			m.brLogOff = maxOff
		case "esc":
			m.focus = focusLeft
		}
		return m, nil
	}
	moveBr := func(delta int) (tea.Model, tea.Cmd) {
		if len(m.branches) == 0 {
			return m, nil
		}
		m.brSel = clamp(m.brSel+delta, 0, len(m.branches)-1)
		ensureVisible(m.brSel, &m.brOffset, m.listHeight())
		b := m.branches[m.brSel]
		if b.Name != m.brLogFor {
			return m, m.loadBranchLog(b.Name)
		}
		return m, nil
	}
	switch key {
	case "j", "s", "down":
		return moveBr(1)
	case "k", "w", "up":
		return moveBr(-1)
	case "ctrl+d", "pgdown":
		return moveBr(page / 2)
	case "ctrl+u", "pgup":
		return moveBr(-page / 2)
	case "g", "home":
		return moveBr(-len(m.branches))
	case "G", "end":
		return moveBr(len(m.branches))
	case "enter":
		if len(m.branches) == 0 {
			return m, nil
		}
		b := m.branches[m.brSel]
		if b.Current {
			m.setFlash("already on "+b.Name, false)
			return m, nil
		}
		return m, m.doCheckout(b.Name, b.Name, false)
	case "n":
		m.branchBase = ""
		return m, m.openPrompt(promptBranch, "⎇ ", "new branch name (from "+m.head.Branch+")")
	case "x":
		if len(m.branches) == 0 {
			return m, nil
		}
		b := m.branches[m.brSel]
		if b.Remote {
			m.setFlash("delete remote branches from the host — this removes local ones", true)
			return m, nil
		}
		if b.Current {
			m.setFlash("cannot delete the branch you are on", true)
			return m, nil
		}
		r := m.repo
		name := b.Name
		m.confirmMsg = "Delete branch " + name + "? y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.DeleteBranch(name, false); err != nil {
				if strings.Contains(err.Error(), "not fully merged") {
					return branchDeleteBlockedMsg{name}
				}
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ deleted branch " + name, reload: true}
		}
		return m, nil
	case "e":
		if len(m.branches) == 0 {
			return m, nil
		}
		b := m.branches[m.brSel]
		if b.Remote {
			m.setFlash("cannot rename a remote branch here", true)
			return m, nil
		}
		m.renameFrom = b.Name
		cmd := m.openPrompt(promptRename, "⎇ ", "new name for "+b.Name)
		m.promptInput.SetValue(b.Name)
		return m, cmd
	case "O":
		if len(m.branches) == 0 {
			return m, nil
		}
		url := m.repo.RemoteURL("origin")
		if url == "" {
			m.setFlash("no origin remote — press o to add one", true)
			return m, nil
		}
		b := m.branches[m.brSel]
		branch := b.Name
		if b.Remote {
			if i := strings.Index(branch, "/"); i >= 0 {
				branch = branch[i+1:]
			}
		}
		pr := prURL(url, branch)
		if err := openBrowser(pr); err != nil {
			m.setFlash("could not open browser: "+pr, true)
			return m, nil
		}
		m.setFlash("✓ opened PR page for "+branch+" → "+pr, false)
		return m, nil
	case "m":
		if len(m.branches) == 0 {
			return m, nil
		}
		b := m.branches[m.brSel]
		if b.Current {
			m.setFlash("cannot merge a branch into itself", true)
			return m, nil
		}
		r := m.repo
		name := b.Name
		m.confirmMsg = "Merge " + name + " into " + m.head.Branch + "? y/N"
		m.confirmCmd = func() tea.Msg {
			if err := r.Merge(name); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ merged " + name + " into " + m.head.Branch, reload: true}
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.mouseInRight(msg.X) {
			m.focus = focusRight
			return m.scrollRight(-3)
		}
		m.focus = focusLeft
		return m.scrollLeft(-3)
	case tea.MouseButtonWheelDown:
		if m.mouseInRight(msg.X) {
			m.focus = focusRight
			return m.scrollRight(3)
		}
		m.focus = focusLeft
		return m.scrollLeft(3)
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		// tab bar row
		if msg.Y == 1 {
			pos := 0
			for i, t := range tabLabels {
				w := len(t) + 4 // padding 2 each side
				if msg.X >= pos && msg.X < pos+w {
					return m.switchView(viewID(i))
				}
				pos += w
			}
			return m, nil
		}
		// list rows: body starts at y=3 (header + 2-line tab bar), +1 border
		row := msg.Y - 4
		if row < 0 || row >= m.listHeight() {
			return m, nil
		}
		if m.mouseInRight(msg.X) {
			m.focus = focusRight
			return m, nil
		}
		double := time.Since(m.lastClickAt) < 450*time.Millisecond && m.lastClickY == msg.Y
		m.lastClickAt = time.Now()
		m.lastClickY = msg.Y
		m.focus = focusLeft
		switch m.view {
		case viewCommits:
			target := m.offset + row
			if target >= len(m.visible) {
				return m, nil
			}
			if double && target == m.sel {
				return m, m.checkoutSelectedCommit()
			}
			return m.moveSel(target - m.sel)
		case viewStatus:
			rowIdx := m.fileOffset + row
			if rowIdx >= len(m.statusRows) || m.statusRows[rowIdx].item < 0 {
				return m, nil
			}
			target := m.statusRows[rowIdx].item
			if double && target == m.fileSel {
				it := m.statusItems[target]
				if it.isStash {
					ref := it.stash.Ref
					r := m.repo
					return m, func() tea.Msg {
						if err := r.StashApply(ref); err != nil {
							return actionMsg{err: err}
						}
						return actionMsg{text: "✓ applied " + ref, reload: true}
					}
				}
				return m, m.toggleStage(it)
			}
			m.fileSel = target
			it := m.statusItems[target]
			if itemKey(it) != m.diffFor {
				return m, m.loadItemDiff(it)
			}
		case viewBranches:
			target := m.brOffset + row
			if target >= len(m.branches) {
				return m, nil
			}
			if double && target == m.brSel {
				b := m.branches[target]
				if b.Current {
					m.setFlash("already on "+b.Name, false)
					return m, nil
				}
				return m, m.doCheckout(b.Name, b.Name, false)
			}
			m.brSel = target
			b := m.branches[m.brSel]
			if b.Name != m.brLogFor {
				return m, m.loadBranchLog(b.Name)
			}
		case viewStashes:
			target := m.stOff + row
			if target >= len(m.stashes) {
				return m, nil
			}
			if double && target == m.stSel {
				st := m.stashes[target]
				r := m.repo
				return m, func() tea.Msg {
					if err := r.StashApply(st.Ref); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{text: "✓ applied " + st.Ref, reload: true}
				}
			}
			m.stSel = target
			st := m.stashes[m.stSel]
			if st.Ref != m.stDiffFor {
				return m, m.loadStashDiff(st.Ref)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) mouseInRight(x int) bool { return x >= m.leftWidth() }

func (m model) scrollLeft(delta int) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewCommits:
		return m.moveSel(delta)
	case viewStatus:
		return m.moveItem(delta)
	case viewBranches:
		if len(m.branches) == 0 {
			return m, nil
		}
		m.brSel = clamp(m.brSel+delta, 0, len(m.branches)-1)
		ensureVisible(m.brSel, &m.brOffset, m.listHeight())
		b := m.branches[m.brSel]
		if b.Name != m.brLogFor {
			return m, m.loadBranchLog(b.Name)
		}
	case viewStashes:
		if len(m.stashes) == 0 {
			return m, nil
		}
		m.stSel = clamp(m.stSel+delta, 0, len(m.stashes)-1)
		ensureVisible(m.stSel, &m.stOff, m.listHeight())
		st := m.stashes[m.stSel]
		if st.Ref != m.stDiffFor {
			return m, m.loadStashDiff(st.Ref)
		}
	}
	return m, nil
}

func (m model) scrollRight(delta int) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewCommits:
		m.detailOff = clamp(m.detailOff+delta, 0, max(len(m.details)-m.listHeight(), 0))
	case viewStatus:
		m.diffOff = clamp(m.diffOff+delta, 0, max(len(m.fileDiff)-m.listHeight(), 0))
	case viewBranches:
		m.brLogOff = clamp(m.brLogOff+delta, 0, max(len(m.brLog)-m.listHeight(), 0))
	case viewStashes:
		m.stDiffOff = clamp(m.stDiffOff+delta, 0, max(len(m.stDiff)-m.listHeight(), 0))
	}
	return m, nil
}
