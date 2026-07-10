package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const commitLimit = 1500

type viewID int

const (
	viewCommits viewID = iota
	viewStatus
	viewBranches
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

	// status view
	files      []FileStatus
	fileSel    int
	fileOffset int
	fileDiff   []string
	diffFor    string
	diffOff    int

	// branches view
	branches []Branch
	brSel    int
	brOffset int
	brLog    []string
	brLogFor string
	brLogOff int

	// commit message input
	committing  bool
	commitInput textinput.Model

	head     HeadInfo
	showHelp bool
	flash    string
	flashErr bool
}

func newModel(repo *Repo) model {
	si := textinput.New()
	si.Placeholder = "filter by subject, author or hash…"
	si.Prompt = "/ "
	si.CharLimit = 120

	ci := textinput.New()
	ci.Placeholder = "commit message"
	ci.Prompt = "✎ "
	ci.CharLimit = 200

	return model{
		repo:        repo,
		searchInput: si,
		commitInput: ci,
	}
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
	text   string
	err    error
	reload bool
}

// ---- commands ----

func (m model) loadCommits() tea.Cmd {
	r := m.repo
	return func() tea.Msg {
		commits, err := r.Commits(commitLimit)
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

func fileKey(f FileStatus) string {
	side := "u"
	if f.Staged {
		side = "s"
	}
	return side + ":" + f.Path
}

func (m model) Init() tea.Cmd {
	m.loadingLog = true
	return tea.Batch(m.loadCommits(), m.loadHead())
}

// ---- geometry helpers ----

func (m model) bodyHeight() int { return max(m.height-3, 1) }

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
			m.flash, m.flashErr = msg.err.Error(), true
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
			m.flash, m.flashErr = msg.err.Error(), true
			return m, nil
		}
		if c, ok := m.selectedCommit(); ok && c.Hash == msg.hash {
			m.details, m.detailFor, m.detailOff = msg.lines, msg.hash, 0
		}
		return m, nil

	case statusListMsg:
		if msg.err != nil {
			m.flash, m.flashErr = msg.err.Error(), true
			return m, nil
		}
		m.files = msg.files
		if m.fileSel >= len(m.files) {
			m.fileSel = max(len(m.files)-1, 0)
		}
		ensureVisible(m.fileSel, &m.fileOffset, m.listHeight())
		if len(m.files) > 0 {
			return m, m.loadFileDiff(m.files[m.fileSel])
		}
		m.fileDiff, m.diffFor = nil, ""
		return m, nil

	case fileDiffMsg:
		if msg.err != nil {
			m.flash, m.flashErr = msg.err.Error(), true
			return m, nil
		}
		if len(m.files) > 0 && m.fileSel < len(m.files) && fileKey(m.files[m.fileSel]) == msg.key {
			m.fileDiff, m.diffFor, m.diffOff = msg.lines, msg.key, 0
		}
		return m, nil

	case branchesMsg:
		if msg.err != nil {
			m.flash, m.flashErr = msg.err.Error(), true
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
			m.flash, m.flashErr = msg.err.Error(), true
			return m, nil
		}
		if len(m.branches) > 0 && m.brSel < len(m.branches) && m.branches[m.brSel].Name == msg.name {
			m.brLog, m.brLogFor, m.brLogOff = msg.lines, msg.name, 0
		}
		return m, nil

	case headMsg:
		m.head = HeadInfo(msg)
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.flash, m.flashErr = msg.err.Error(), true
		} else {
			m.flash, m.flashErr = msg.text, false
		}
		if msg.reload {
			return m, m.refresh()
		}
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
		cmds = append(cmds, m.loadStatus())
	case viewBranches:
		cmds = append(cmds, m.loadBranches())
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
		return m, tea.Batch(m.loadStatus(), m.loadHead())
	case viewBranches:
		return m, tea.Batch(m.loadBranches(), m.loadHead())
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

	// modal: commit message input
	if m.committing {
		switch key {
		case "esc":
			m.committing = false
			m.commitInput.Blur()
			return m, nil
		case "enter":
			message := strings.TrimSpace(m.commitInput.Value())
			if message == "" {
				return m, nil
			}
			m.committing = false
			m.commitInput.Blur()
			m.commitInput.SetValue("")
			r := m.repo
			return m, func() tea.Msg {
				if err := r.Commit(message); err != nil {
					return actionMsg{err: err}
				}
				return actionMsg{text: "✓ committed: " + message, reload: true}
			}
		}
		var cmd tea.Cmd
		m.commitInput, cmd = m.commitInput.Update(msg)
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
	case "tab", "left", "right", "h", "l", "a", "d":
		if key == "left" || key == "h" || key == "a" {
			m.focus = focusLeft
		} else if key == "right" || key == "l" || key == "d" {
			m.focus = focusRight
		} else if m.focus == focusLeft {
			m.focus = focusRight
		} else {
			m.focus = focusLeft
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
	moveFile := func(delta int) (tea.Model, tea.Cmd) {
		if len(m.files) == 0 {
			return m, nil
		}
		m.fileSel = clamp(m.fileSel+delta, 0, len(m.files)-1)
		ensureVisible(m.fileSel, &m.fileOffset, m.listHeight())
		f := m.files[m.fileSel]
		if fileKey(f) != m.diffFor {
			return m, m.loadFileDiff(f)
		}
		return m, nil
	}
	switch key {
	case "j", "s", "down":
		return moveFile(1)
	case "k", "w", "up":
		return moveFile(-1)
	case "ctrl+d", "pgdown":
		return moveFile(page / 2)
	case "ctrl+u", "pgup":
		return moveFile(-page / 2)
	case "g", "home":
		return moveFile(-len(m.files))
	case "G", "end":
		return moveFile(len(m.files))
	case " ":
		if len(m.files) == 0 {
			return m, nil
		}
		f := m.files[m.fileSel]
		r := m.repo
		return m, func() tea.Msg {
			var err error
			var verb string
			if f.Staged {
				err, verb = r.UnstageFile(f), "unstaged"
			} else {
				err, verb = r.StageFile(f), "staged"
			}
			if err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ " + verb + " " + f.Path, reload: true}
		}
	case "c":
		m.committing = true
		m.commitInput.Focus()
		return m, textinput.Blink
	case "enter":
		m.focus = focusRight
		return m, nil
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
			m.flash, m.flashErr = "already on "+b.Name, false
			return m, nil
		}
		r := m.repo
		return m, func() tea.Msg {
			if err := r.Checkout(b.Name); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "✓ checked out " + b.Name, reload: true}
		}
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
		// list rows: body starts at y=2, +1 for the pane border
		row := msg.Y - 3
		if row < 0 || row >= m.listHeight() {
			return m, nil
		}
		if m.mouseInRight(msg.X) {
			m.focus = focusRight
			return m, nil
		}
		m.focus = focusLeft
		switch m.view {
		case viewCommits:
			target := m.offset + row
			if target < len(m.visible) {
				return m.moveSel(target - m.sel)
			}
		case viewStatus:
			target := m.fileOffset + row
			if target < len(m.files) {
				m.fileSel = target
				f := m.files[m.fileSel]
				if fileKey(f) != m.diffFor {
					return m, m.loadFileDiff(f)
				}
			}
		case viewBranches:
			target := m.brOffset + row
			if target < len(m.branches) {
				m.brSel = target
				b := m.branches[m.brSel]
				if b.Name != m.brLogFor {
					return m, m.loadBranchLog(b.Name)
				}
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
		if len(m.files) == 0 {
			return m, nil
		}
		m.fileSel = clamp(m.fileSel+delta, 0, len(m.files)-1)
		ensureVisible(m.fileSel, &m.fileOffset, m.listHeight())
		f := m.files[m.fileSel]
		if fileKey(f) != m.diffFor {
			return m, m.loadFileDiff(f)
		}
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
	}
	return m, nil
}
