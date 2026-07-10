package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var tabLabels = []string{"⌥ Commits", "± Status", "⎇ Branches", "≡ Stashes"}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	var body string
	switch m.view {
	case viewCommits:
		body = m.renderCommitsView()
	case viewStatus:
		body = m.renderStatusView()
	case viewBranches:
		body = m.renderBranchesView()
	case viewStashes:
		body = m.renderStashesView()
	}
	if m.showHelp {
		body = m.overlay(body, m.renderHelp())
	}
	b.WriteString(body)
	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

// ---- chrome ----

func (m model) renderHeader() string {
	left := sHeaderRepo.Render("● git2 ") + sHeaderInfo.Render("· "+m.repo.Root)

	var parts []string
	branch := m.head.Branch
	if branch == "" {
		branch = "—"
	}
	if m.head.Detached {
		parts = append(parts, "detached @ "+branch)
	} else {
		parts = append(parts, "⎇ "+branch)
	}
	if m.head.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", m.head.Ahead))
	}
	if m.head.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", m.head.Behind))
	}
	if m.head.Dirty > 0 {
		parts = append(parts, fmt.Sprintf("±%d", m.head.Dirty))
	} else {
		parts = append(parts, "✓ clean")
	}
	if m.fetching {
		parts = append(parts, "⇣ fetching…")
	} else if !m.head.HasRemote {
		parts = append(parts, "no remote")
	}
	right := sHeaderInfo.Render(strings.Join(parts, "  "))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	bar := left + sHeader.Render(strings.Repeat(" ", gap)) + right
	return sHeader.Width(m.width).MaxHeight(1).Render(bar)
}

func (m model) renderTabs() string {
	var tabs []string
	for i, label := range tabLabels {
		if viewID(i) == m.view {
			tabs = append(tabs, sTabActive.Render(label))
		} else {
			tabs = append(tabs, sTabIdle.Render(label))
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	// fill the rest of the line with a bottom rule
	fill := m.width - lipgloss.Width(row)
	if fill > 0 {
		rule := sDim.Render(strings.Repeat(" ", fill))
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, lipgloss.NewStyle().
			Border(lipgloss.Border{Bottom: "─"}, false, false, true, false).
			BorderForeground(cDim).Render(strings.Repeat(" ", fill)))
		_ = rule
	}
	return row
}

func (m model) renderFooter() string {
	if m.confirmMsg != "" {
		return sFooter.Width(m.width).Render(sStatusM.Background(cBarBg).
			Render("⚠ " + m.confirmMsg))
	}
	if m.searching {
		return sFooter.Width(m.width).Render(m.searchInput.View() + "  " +
			sDim.Render("enter apply · esc clear"))
	}
	if m.promptMode != promptNone {
		return sFooter.Width(m.width).Render(m.promptInput.View() + "  " +
			sDim.Render("enter confirm · esc cancel"))
	}
	if m.flash != "" {
		style := sOk
		if m.flashErr {
			style = sErr
		}
		return sFooter.Width(m.width).Render(style.
			Background(cBarBg).Render(truncate(m.flash, m.width-4)))
	}

	type hint struct{ key, label string }
	var hints []hint
	switch m.view {
	case viewCommits:
		hints = []hint{{"↑↓", "navigate"}, {"⏎", "diff"}, {"/", "search"}, {"f", "fetch"}, {"P", "push"}}
	case viewStatus:
		hints = []hint{{"↑↓", "navigate"}, {"space", "stage"}, {"c", "commit"}, {"S", "stash"}}
	case viewBranches:
		hints = []hint{{"↑↓", "navigate"}, {"⏎", "checkout"}, {"m", "merge"}, {"p", "pull"}, {"P", "push"}}
	case viewStashes:
		hints = []hint{{"↑↓", "navigate"}, {"⏎", "apply"}, {"p", "pop"}, {"x", "drop"}}
	}
	hints = append(hints, hint{"1-4", "views"}, hint{"?", "help"}, hint{"q", "quit"})

	var parts []string
	for _, h := range hints {
		parts = append(parts, sFooterKey.Render(h.key)+sFooter.UnsetPadding().Render(" "+h.label))
	}
	return sFooter.Width(m.width).Render(strings.Join(parts, sFooter.UnsetPadding().Render("  ·  ")))
}

// ---- panes ----

func (m model) pane(title, content string, width int, focused bool) string {
	style := sPaneBlur
	if focused {
		style = sPaneFocus
	}
	inner := lipgloss.NewStyle().
		Width(width - 2).
		Height(m.bodyHeight() - 2).
		MaxWidth(width - 2).
		MaxHeight(m.bodyHeight() - 2).
		Render(content)
	box := style.Width(width - 2).Height(m.bodyHeight() - 2).Render(inner)
	// stamp the title into the top border
	lines := strings.SplitN(box, "\n", 2)
	if len(lines) == 2 {
		t := " " + title + " "
		top := []rune(stripANSI(lines[0]))
		if len(t)+4 < len(top) {
			color := cDim
			if focused {
				color = lipgloss.AdaptiveColor{Light: string(cAccent), Dark: string(cAccent)}
			}
			styled := lipgloss.NewStyle().Foreground(color).Render(string(top[:2])) +
				sPaneTitle.Render(t) +
				lipgloss.NewStyle().Foreground(color).Render(string(top[2+len([]rune(t)):]))
			return styled + "\n" + lines[1]
		}
	}
	return box
}

// ---- commits view ----

func (m model) renderCommitsView() string {
	lw, rw := m.leftWidth(), m.rightWidth()
	title := fmt.Sprintf("Commits · %d", len(m.visible))
	if m.query != "" {
		title = fmt.Sprintf("Commits · %d/%d · “%s”", len(m.visible), len(m.commits), m.query)
	}
	left := m.pane(title, m.renderCommitList(lw-2), lw, m.focus == focusLeft)

	detailTitle := "Details"
	if c, ok := m.selectedCommit(); ok {
		detailTitle = "Details · " + c.ShortHash()
	}
	right := m.pane(detailTitle, m.renderDiffLines(m.details, m.detailOff, rw-2), rw, m.focus == focusRight)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m model) renderCommitList(width int) string {
	if m.loadingLog {
		return sDim.Render("  loading commits…")
	}
	if len(m.commits) == 0 {
		return sDim.Render("  no commits yet — make your first commit ✨")
	}
	if len(m.visible) == 0 {
		return sDim.Render("  no matches for “" + m.query + "”")
	}
	h := m.listHeight()
	gw := GraphWidth(m.rows, 12)
	if m.query != "" {
		gw = 1
	}

	var b strings.Builder
	for row := 0; row < h && m.offset+row < len(m.visible); row++ {
		i := m.visible[m.offset+row]
		c := m.commits[i]
		selected := m.offset+row == m.sel
		b.WriteString(m.renderCommitRow(c, m.rows[i], gw, width, selected))
		if row < h-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m model) renderCommitRow(c Commit, gr GraphRow, gw, width int, selected bool) string {
	bg := lipgloss.NewStyle()
	if selected {
		bg = bg.Background(cSelBg)
	}

	// graph cells
	var graph strings.Builder
	if m.query != "" {
		st := laneStyle(gr.Color)
		if selected {
			st = st.Background(cSelBg)
		}
		graph.WriteString(st.Render("●"))
	} else {
		for i := 0; i < gw; i++ {
			ch := ' '
			color := 0
			if i < len(gr.Cells) {
				ch, color = gr.Cells[i].Ch, gr.Cells[i].Color
			}
			st := laneStyle(color)
			if selected {
				st = st.Background(cSelBg)
			}
			graph.WriteString(st.Render(string(ch)))
			if i < gw-1 {
				pad := ' '
				padColor := color
				if ch == '─' || ch == '╭' || ch == '╰' || ch == '┼' {
					pad = '─'
				}
				if i+1 < len(gr.Cells) {
					nc := gr.Cells[i+1].Ch
					if nc == '─' || nc == '╮' || nc == '╯' || nc == '┤' || nc == '┼' {
						pad = '─'
						padColor = gr.Cells[i+1].Color
					}
				}
				pst := laneStyle(padColor)
				if selected {
					pst = pst.Background(cSelBg)
				}
				graph.WriteString(pst.Render(string(pad)))
			}
		}
	}
	graphW := gw*2 - 1
	if m.query != "" {
		graphW = 1
	}

	// badges
	var badges strings.Builder
	badgeW := 0
	for _, ref := range c.Refs {
		if badgeW > width/3 {
			break
		}
		var chip string
		switch ref.Kind {
		case RefHead:
			chip = sRefHead.Render(ref.Name)
		case RefBranch:
			chip = sRefBranch.Render(ref.Name)
		case RefRemote:
			chip = sRefRemote.Render(shortRemote(ref.Name))
		case RefTag:
			chip = sRefTag.Render("⌂ " + ref.Name)
		}
		badges.WriteString(chip)
		badges.WriteString(bg.Render(" "))
		badgeW += lipgloss.Width(chip) + 1
	}

	// right-aligned metadata
	meta := fmt.Sprintf("%s · %s · %s", truncate(c.Author, 16), relTime(c.Date), c.ShortHash()[:7])
	metaW := len([]rune(meta)) + 1
	if width < 70 {
		meta = fmt.Sprintf("%s · %s", relTime(c.Date), c.ShortHash()[:7])
		metaW = len([]rune(meta)) + 1
	}

	subjectW := width - graphW - 1 - badgeW - metaW - 1
	if subjectW < 8 {
		metaW = 0
		meta = ""
		subjectW = width - graphW - 1 - badgeW - 1
	}
	subject := truncate(c.Subject, max(subjectW, 4))

	subjStyle := sText
	if selected {
		subjStyle = sBright.Background(cSelBg)
	}
	metaStyle := sDim
	hashStyle := laneStyle(gr.Color)
	if selected {
		metaStyle = metaStyle.Background(cSelBg)
		hashStyle = hashStyle.Background(cSelBg)
	}

	line := graph.String() + bg.Render(" ") + badges.String() + subjStyle.Render(subject)
	used := graphW + 1 + badgeW + lipgloss.Width(subject)
	gap := width - used - metaW
	if gap > 0 {
		line += bg.Render(strings.Repeat(" ", gap))
	}
	if meta != "" {
		line += metaStyle.Render(" " + meta)
	}
	return line
}

func shortRemote(name string) string {
	if len(name) > 24 {
		return name[:23] + "…"
	}
	return name
}

// ---- status view ----

func (m model) renderStatusView() string {
	lw, rw := m.leftWidth(), m.rightWidth()

	staged := 0
	for _, f := range m.files {
		if f.Staged {
			staged++
		}
	}
	title := fmt.Sprintf("Working tree · %d staged · %d unstaged", staged, len(m.files)-staged)
	left := m.pane(title, m.renderFileList(lw-2), lw, m.focus == focusLeft)

	diffTitle := "Diff"
	if len(m.files) > 0 && m.fileSel < len(m.files) {
		diffTitle = "Diff · " + truncate(m.files[m.fileSel].Path, rw-12)
	}
	right := m.pane(diffTitle, m.renderDiffLines(m.fileDiff, m.diffOff, rw-2), rw, m.focus == focusRight)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m model) renderFileList(width int) string {
	if len(m.files) == 0 {
		return sOk.Render("  ✓ working tree clean") + "\n\n" +
			sDim.Render("  nothing to stage or commit")
	}
	h := m.listHeight()
	var b strings.Builder
	lastStaged := true
	first := true
	rendered := 0
	for i := m.fileOffset; i < len(m.files) && rendered < h; i++ {
		f := m.files[i]
		if first || f.Staged != lastStaged {
			if !first {
				// blank separator only if it fits
			}
		}
		first = false
		lastStaged = f.Staged

		selected := i == m.fileSel
		bg := lipgloss.NewStyle()
		if selected {
			bg = bg.Background(cSelBg)
		}
		section := "○"
		secStyle := sDim
		if f.Staged {
			section = "●"
			secStyle = sOk
		}
		if selected {
			secStyle = secStyle.Background(cSelBg)
		}
		codeStyle := statusCodeStyle(f.Code)
		if selected {
			codeStyle = codeStyle.Background(cSelBg)
		}
		nameStyle := sText
		if selected {
			nameStyle = sBright.Background(cSelBg)
		}
		path := truncate(f.Path, width-8)
		line := bg.Render(" ") + secStyle.Render(section) + bg.Render(" ") +
			codeStyle.Render(f.Code) + bg.Render(" ") + nameStyle.Render(path)
		pad := width - lipgloss.Width(line)
		if pad > 0 {
			line += bg.Render(strings.Repeat(" ", pad))
		}
		b.WriteString(line)
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
	}
	if rendered < h {
		b.WriteString("\n" + sDim.Render("  ● staged   ○ unstaged   space to toggle"))
	}
	return b.String()
}

// ---- branches view ----

func (m model) renderBranchesView() string {
	lw, rw := m.leftWidth(), m.rightWidth()
	title := fmt.Sprintf("Branches · %d", len(m.branches))
	left := m.pane(title, m.renderBranchList(lw-2), lw, m.focus == focusLeft)

	logTitle := "History"
	if m.brLogFor != "" {
		logTitle = "History · " + m.brLogFor
	}
	right := m.pane(logTitle, m.renderBranchLog(rw-2), rw, m.focus == focusRight)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m model) renderBranchList(width int) string {
	if len(m.branches) == 0 {
		return sDim.Render("  no branches")
	}
	h := m.listHeight()
	var b strings.Builder
	rendered := 0
	for i := m.brOffset; i < len(m.branches) && rendered < h; i++ {
		br := m.branches[i]
		selected := i == m.brSel
		bg := lipgloss.NewStyle()
		if selected {
			bg = bg.Background(cSelBg)
		}

		marker := "  "
		markStyle := sDim
		if br.Current {
			marker = "✓ "
			markStyle = sOk
		}
		if selected {
			markStyle = markStyle.Background(cSelBg)
		}
		icon := "⎇ "
		iconStyle := lipgloss.NewStyle().Foreground(cAccent)
		if br.Remote {
			icon = "☁ "
			iconStyle = sDim
		}
		if selected {
			iconStyle = iconStyle.Background(cSelBg)
		}
		nameStyle := sText
		if br.Current {
			nameStyle = sOk.Bold(true)
		}
		if selected {
			nameStyle = nameStyle.Background(cSelBg).Bold(true)
		}
		track := ""
		if br.Track != "" {
			track = " " + br.Track
		}
		date := br.Date
		nameW := width - 5 - len([]rune(track)) - len([]rune(date)) - 2
		name := truncate(br.Name, max(nameW, 8))

		trackStyle := sStatusM
		dimStyle := sDim
		if selected {
			trackStyle = trackStyle.Background(cSelBg)
			dimStyle = dimStyle.Background(cSelBg)
		}
		line := bg.Render(" ") + markStyle.Render(marker) + iconStyle.Render(icon) +
			nameStyle.Render(name) + trackStyle.Render(track)
		gap := width - lipgloss.Width(line) - len([]rune(date)) - 1
		if gap > 0 {
			line += bg.Render(strings.Repeat(" ", gap))
		}
		line += dimStyle.Render(date + " ")
		b.WriteString(line)
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m model) renderBranchLog(width int) string {
	if len(m.brLog) == 0 {
		return sDim.Render("  select a branch")
	}
	h := m.listHeight()
	var b strings.Builder
	rendered := 0
	for i := m.brLogOff; i < len(m.brLog) && rendered < h; i++ {
		parts := strings.SplitN(m.brLog[i], "\x1f", 4)
		var line string
		if len(parts) == 4 {
			metaW := len([]rune(parts[3])) + 1
			subjW := width - 9 - metaW - len([]rune(parts[2])) - 3
			line = lipgloss.NewStyle().Foreground(cYellow).Render(parts[0]) + " " +
				sText.Render(truncate(parts[1], max(subjW, 8))) + " " +
				sDim.Render("· "+parts[2]+" · "+parts[3])
		} else {
			line = sText.Render(truncate(m.brLog[i], width))
		}
		b.WriteString(truncateANSI(line, width))
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ---- stashes view ----

func (m model) renderStashesView() string {
	lw, rw := m.leftWidth(), m.rightWidth()
	title := fmt.Sprintf("Stashes · %d", len(m.stashes))
	left := m.pane(title, m.renderStashList(lw-2), lw, m.focus == focusLeft)

	diffTitle := "Stash diff"
	if m.stDiffFor != "" {
		diffTitle = "Stash diff · " + m.stDiffFor
	}
	right := m.pane(diffTitle, m.renderDiffLines(m.stDiff, m.stDiffOff, rw-2), rw, m.focus == focusRight)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m model) renderStashList(width int) string {
	if len(m.stashes) == 0 {
		return sDim.Render("  no stashes") + "\n\n" +
			sDim.Render("  press S in the Status view to stash your changes")
	}
	h := m.listHeight()
	var b strings.Builder
	rendered := 0
	for i := m.stOff; i < len(m.stashes) && rendered < h; i++ {
		st := m.stashes[i]
		selected := i == m.stSel
		bg := lipgloss.NewStyle()
		if selected {
			bg = bg.Background(cSelBg)
		}
		refStyle := lipgloss.NewStyle().Foreground(cMagenta).Bold(true)
		descStyle := sText
		ageStyle := sDim
		if selected {
			refStyle = refStyle.Background(cSelBg)
			descStyle = sBright.Background(cSelBg)
			ageStyle = ageStyle.Background(cSelBg)
		}
		age := st.Age
		descW := width - len([]rune(st.Ref)) - len([]rune(age)) - 5
		desc := truncate(st.Desc, max(descW, 8))
		line := bg.Render(" ") + refStyle.Render(st.Ref) + bg.Render(" ") + descStyle.Render(desc)
		gap := width - lipgloss.Width(line) - len([]rune(age)) - 1
		if gap > 0 {
			line += bg.Render(strings.Repeat(" ", gap))
		}
		line += ageStyle.Render(age + " ")
		b.WriteString(line)
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ---- diff rendering ----

func (m model) renderDiffLines(lines []string, offset, width int) string {
	if len(lines) == 0 {
		return sDim.Render("  nothing selected")
	}
	h := m.listHeight()
	var b strings.Builder
	rendered := 0
	for i := offset; i < len(lines) && rendered < h; i++ {
		b.WriteString(styleDiffLine(lines[i], width))
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
	}
	// scroll indicator
	if len(lines) > h {
		pct := 100 * (offset + min(h, len(lines)-offset)) / len(lines)
		ind := fmt.Sprintf(" %d%% ", pct)
		_ = ind
	}
	return b.String()
}

func styleDiffLine(line string, width int) string {
	t := truncate(line, width)
	switch {
	case strings.HasPrefix(line, "diff --git"), strings.HasPrefix(line, "commit "):
		return sDiffHeader.Render(t)
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		return sDiffMeta.Render(t)
	case strings.HasPrefix(line, "@@"):
		return sDiffHunk.Render(t)
	case strings.HasPrefix(line, "+"):
		return sDiffAdd.Render(t)
	case strings.HasPrefix(line, "-"):
		return sDiffDel.Render(t)
	case strings.HasPrefix(line, "author "), strings.HasPrefix(line, "date   "):
		return sDim.Render(t)
	case strings.Contains(line, " | ") && (strings.Contains(line, "+") || strings.Contains(line, "-")):
		// diffstat line: colorize the +/- bar
		if i := strings.LastIndex(t, " "); i > 0 {
			bar := t[i+1:]
			colored := strings.ReplaceAll(bar, "+", sDiffAdd.Render("+"))
			colored = strings.ReplaceAll(colored, "-", sDiffDel.Render("-"))
			return sText.Render(t[:i+1]) + colored
		}
		return sText.Render(t)
	default:
		return sText.Render(t)
	}
}

// ---- help overlay ----

func (m model) renderHelp() string {
	rows := [][2]string{
		{"1 / 2 / 3 / 4", "commits · status · branches · stashes"},
		{"tab / ← → / a d", "switch pane focus"},
		{"↑ ↓ / w s / j k", "move selection / scroll"},
		{"ctrl+d ctrl+u", "half-page down / up"},
		{"g / G", "jump to top / bottom"},
		{"enter", "focus diff · checkout · apply stash"},
		{"/", "search commits"},
		{"space", "stage / unstage file"},
		{"c", "commit staged changes"},
		{"S", "stash working tree (status view)"},
		{"m", "merge branch into current (branches view)"},
		{"f", "fetch all remotes (auto every 3 min)"},
		{"p", "pull (fast-forward) · pop stash"},
		{"P / F", "push · force-push with lease"},
		{"o", "add / show origin remote"},
		{"x", "drop stash"},
		{"r", "refresh"},
		{"?", "toggle this help"},
		{"q", "quit"},
	}
	var b strings.Builder
	b.WriteString(sBright.Render("git2 — keyboard reference") + "\n\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("%s  %s\n",
			sHelpKey.Width(17).Render(r[0]), sText.Render(r[1])))
	}
	return sHelpBox.Render(strings.TrimRight(b.String(), "\n"))
}

func (m model) overlay(base, box string) string {
	return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, box)
}

// ---- text helpers ----

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

// truncateANSI trims a styled string to width using lipgloss measurement.
func truncateANSI(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	// crude but safe: strip styling, truncate plain
	return truncate(stripANSI(s), w)
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/(24*365)))
	}
}
