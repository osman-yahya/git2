package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	var main string
	switch m.view {
	case viewCommits:
		main = m.renderCommitsView()
	case viewStatus:
		main = m.renderStatusView()
	case viewStashes:
		main = m.renderStashDiffView()
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(), main)
	if m.showHelp {
		body = m.overlay(body, m.renderHelp())
	}
	if len(m.choiceOptions) > 0 {
		body = m.overlay(body, m.renderChoice())
	}
	b.WriteString(body)
	b.WriteString("\n")
	b.WriteString(m.renderMsgLine())
	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

// ---- chrome ----

func (m model) renderHeader() string {
	repoPath := collapseHome(m.repo.Root)

	bg := lipgloss.NewStyle().Background(cBarBg)
	branch := m.head.Branch
	if branch == "" {
		branch = "—"
	}
	branchChip := sRefBranch.Render("⎇ " + branch)
	if m.head.Detached {
		branchChip = sRefTag.Render("● " + branch)
	}
	var parts []string
	parts = append(parts, branchChip)
	var counts []string
	if m.head.Ahead > 0 {
		counts = append(counts, sOk.Background(cBarBg).Render(fmt.Sprintf("↑%d", m.head.Ahead)))
	}
	if m.head.Behind > 0 {
		counts = append(counts, sErr.Background(cBarBg).Bold(false).Render(fmt.Sprintf("↓%d", m.head.Behind)))
	}
	if m.head.Dirty > 0 {
		counts = append(counts, sStatusM.Background(cBarBg).Render(fmt.Sprintf("±%d", m.head.Dirty)))
	} else {
		counts = append(counts, sHeaderInfo.Render("✓"))
	}
	parts = append(parts, strings.Join(counts, bg.Render(" ")))
	if m.head.Merging {
		parts = append(parts, sConflictMark.Render(" MERGING "))
	}
	if m.fetching {
		parts = append(parts, sHeaderInfo.Render("⇣ fetching…"))
	} else if !m.head.HasRemote {
		parts = append(parts, sHeaderInfo.Render("no remote"))
	}
	if m.updateAvail != "" {
		parts = append(parts, sRefHead.Render("⬆ v"+m.updateAvail+" · git2 update"))
	}
	right := strings.Join(parts, bg.Render("  "))

	// the right side (branch, merge state, sync info) always wins over the path;
	// inner budget is width-2 because sHeader pads one column each side
	inner := m.width - 2
	maxPath := inner - lipgloss.Width(right) - 12
	left := sHeaderRepo.Render("● git2 ") + sHeaderInfo.Render("· "+truncate(repoPath, max(maxPath, 8)))
	gap := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	pad := lipgloss.NewStyle().Background(cBarBg).Render(strings.Repeat(" ", gap))
	return sHeader.Width(m.width).MaxHeight(1).Render(left + pad + right)
}

// renderSidebar draws the Fork-style navigation column.
func (m model) renderSidebar() string {
	w := m.sbWidth()
	h := m.listHeight()
	var b strings.Builder
	rendered := 0
	for i := m.sbOff; i < len(m.sbItems) && rendered < h; i++ {
		it := m.sbItems[i]
		selected := i == m.sbSel
		var line string
		switch it.kind {
		case sbHeader:
			line = sSectionBand.Foreground(cDim).Width(w - 2).Render(" " + it.label)
		default:
			bg := lipgloss.NewStyle()
			marker := " "
			style := sText
			icon := ""
			switch it.kind {
			case sbChanges:
				icon = ""
				style = sStatusM
			case sbAllCommits:
				icon = ""
				style = lipgloss.NewStyle().Foreground(cAccent)
			case sbBranch:
				icon = "⎇ "
				if it.current {
					style = sOk.Bold(true)
				}
			case sbRemote:
				icon = "☁ "
				style = sDim
			case sbTag:
				icon = "⌂ "
				style = sStatusM
			case sbStash:
				icon = "≡ "
				style = lipgloss.NewStyle().Foreground(cMagenta)
			}
			if selected {
				bg = bg.Background(cSelBg)
				style = style.Background(cSelBg).Bold(true)
				marker = lipgloss.NewStyle().Foreground(cAccent).Background(cSelBg).Render("▌")
			}
			cur := ""
			if it.current {
				cur = "✓ "
			}
			label := truncate(cur+icon+it.label, w-4)
			line = marker + style.Render(label)
			if pad := w - 2 - lipgloss.Width(line); pad > 0 {
				line += bg.Render(strings.Repeat(" ", pad))
			}
		}
		b.WriteString(line)
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
	}
	return m.paneH("Repository", b.String(), w, m.bodyHeight(), m.focus == focusSide)
}

// renderStashDiffView shows one stash across the whole main area.
func (m model) renderStashDiffView() string {
	w := m.width - m.sbWidth()
	title := "Stash"
	if m.stSel < len(m.stashes) {
		st := m.stashes[m.stSel]
		title = st.Ref + " · " + truncate(st.Desc, w-30)
	}
	return m.pane(title, m.renderDiffLines(m.stDiff, m.stDiffOff, w-2), w, m.focus != focusSide)
}

// renderMsgLine is the dedicated line above the footer: confirmations,
// prompts, search, flash messages — or a contextual breadcrumb when idle.
func (m model) renderMsgLine() string {
	bar := lipgloss.NewStyle().Background(cBarBg).Width(m.width).MaxHeight(1).Padding(0, 1)
	switch {
	case m.confirmMsg != "":
		return bar.Render(sStatusM.Background(cBarBg).Bold(true).Render("⚠ "+m.confirmMsg) +
			sHeaderInfo.Render("  · y confirm · any other key cancels"))
	case m.searching:
		return bar.Render(m.searchInput.View() + "  " + sHeaderInfo.Render("enter apply · esc clear"))
	case m.promptMode != promptNone:
		return bar.Render(m.promptInput.View() + "  " + sHeaderInfo.Render("enter confirm · esc cancel"))
	case m.flash != "":
		style := sOk
		if m.flashErr {
			style = sErr
		}
		return bar.Render(style.Background(cBarBg).Render(truncate(m.flash, m.width-4)))
	}
	// idle: contextual breadcrumb about the selection
	ctx := ""
	switch m.view {
	case viewCommits:
		if c, ok := m.selectedCommit(); ok {
			ctx = c.ShortHash() + " · " + c.Author + " · " + relTime(c.Date) + " · " + c.Subject
		}
	case viewStatus:
		if len(m.statusItems) > 0 && m.fileSel < len(m.statusItems) {
			it := m.statusItems[m.fileSel]
			if it.isStash {
				ctx = it.stash.Ref + " · " + it.stash.Desc
			} else {
				ctx = it.file.Path + " · " + statusCodeWord(it.file)
			}
		}
	case viewBranches:
		if len(m.branches) > 0 && m.brSel < len(m.branches) {
			b := m.branches[m.brSel]
			ctx = b.Name + " · " + b.Hash + " · " + b.Date
			if b.Track != "" {
				ctx += " · " + b.Track
			}
		}
	case viewStashes:
		if len(m.stashes) > 0 && m.stSel < len(m.stashes) {
			st := m.stashes[m.stSel]
			ctx = st.Ref + " · " + st.Desc + " · " + st.Age
		}
	}
	return bar.Render(sHeaderInfo.Render(truncate(ctx, m.width-4)))
}

func statusCodeWord(f FileStatus) string {
	switch {
	case f.Conflict:
		return "conflict — fix then space to resolve"
	case f.Untracked:
		return "untracked"
	case f.Code == "D":
		return "deleted"
	case f.Code == "A":
		return "added"
	case f.Code == "R":
		return "renamed"
	case f.Staged:
		return "staged"
	default:
		return "modified"
	}
}

func (m model) renderFooter() string {
	type hint struct{ key, label string }
	var hints []hint
	if m.focus == focusSide {
		hints = []hint{{"↑↓", "navigate"}, {"⏎", "open"}, {"c", "checkout"}, {"n·e·x", "new·rename·del"},
			{"m", "merge"}, {"p", "pop stash"}, {"O", "PR"}, {"d", "to main pane"},
			{"tab", "cycle focus"}, {"?", "help"}, {"q", "quit"}}
		var parts []string
		for _, h := range hints {
			parts = append(parts, sFooterKey.Render(h.key)+sFooter.UnsetPadding().Render(" "+h.label))
		}
		return sFooter.Width(m.width).MaxHeight(1).Render(strings.Join(parts, sFooter.UnsetPadding().Render(" · ")))
	}
	switch m.view {
	case viewCommits:
		hints = []hint{{"c", "checkout"}, {"b", "branch"}, {"t", "focus"}, {"n", "new"}, {"T", "tag"},
			{"m", "merge"}, {"y", "pick"}, {"R", "rebase"}, {"v", "revert"}, {"/", "search"}}
	case viewStatus:
		switch {
		case m.focus == focusRight:
			hints = []hint{{"[ ]", "hunks"}, {"space", "stage hunk"}, {"↑↓", "scroll"}, {"a", "back to files"}}
		case m.head.Merging:
			hints = []hint{{"⏎", "resolve"}, {"u", "ours"}, {"t", "theirs"}, {"space", "mark ok"},
				{"c", "commit merge"}, {"X", "abort"}}
		default:
			hints = []hint{{"space", "stage"}, {"D", "discard"}, {"c", "commit"}, {"A", "amend"},
				{"S", "stash"}, {"H", "history"}, {"B", "blame"}}
		}
	case viewBranches:
		hints = []hint{{"⏎", "checkout"}, {"n", "new"}, {"e", "rename"}, {"x", "delete"},
			{"m", "merge"}, {"O", "PR"}, {"p", "pull"}, {"P", "push"}}
	case viewStashes:
		hints = []hint{{"↑↓", "scroll"}, {"a", "back to sidebar"}}
	}
	hints = append(hints, hint{"a/d", "panes"}, hint{"tab", "cycle focus"}, hint{"?", "help"}, hint{"q", "quit"})

	var parts []string
	for _, h := range hints {
		parts = append(parts, sFooterKey.Render(h.key)+sFooter.UnsetPadding().Render(" "+h.label))
	}
	return sFooter.Width(m.width).MaxHeight(1).Render(strings.Join(parts, sFooter.UnsetPadding().Render(" · ")))
}

// ---- panes ----

func (m model) pane(title, content string, width int, focused bool) string {
	return m.paneH(title, content, width, m.bodyHeight(), focused)
}

// paneH renders a bordered pane with an explicit total height.
func (m model) paneH(title, content string, width, height int, focused bool) string {
	style := sPaneBlur
	if focused {
		style = sPaneFocus
	}
	inner := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		MaxWidth(width - 2).
		MaxHeight(height - 2).
		Render(content)
	box := style.Width(width - 2).Height(height - 2).Render(inner)
	// stamp the title into the top border
	lines := strings.SplitN(box, "\n", 2)
	if len(lines) == 2 {
		t := " " + title + " "
		if focused {
			t = " ▶ " + title + " "
		}
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
	title := fmt.Sprintf("All Commits · %d", len(m.visible))
	if m.graphRef != "" {
		title = fmt.Sprintf("⎇ %s · %d", m.graphRef, len(m.visible))
	}
	if m.query != "" {
		title = fmt.Sprintf("Commits · %d/%d · “%s”", len(m.visible), len(m.commits), m.query)
	}
	left := m.pane(title, m.renderCommitList(lw-2), lw, m.focus == focusLeft)

	metaTitle := "Details"
	if c, ok := m.selectedCommit(); ok {
		metaTitle = "Details · " + c.ShortHash()
	}
	metaH := m.detailMetaHeight()
	var metaBody strings.Builder
	for i, l := range m.detailMeta {
		if i >= metaH-2 {
			break
		}
		switch {
		case strings.HasPrefix(l, "commit "), strings.HasPrefix(l, "parents"):
			metaBody.WriteString(sDim.Render(truncate(l, rw-4)))
		case strings.HasPrefix(l, "author "), strings.HasPrefix(l, "date "):
			metaBody.WriteString(sText.Render(truncate(l, rw-4)))
		default:
			metaBody.WriteString(sBright.Render(truncate(l, rw-4)))
		}
		metaBody.WriteString("\n")
	}
	metaPane := m.paneH(metaTitle, strings.TrimRight(metaBody.String(), "\n"), rw, metaH, false)
	patchPane := m.paneH("Changes", m.renderDiffLines(m.details, m.detailOff, rw-2), rw,
		m.bodyHeight()-metaH, m.focus == focusRight)
	right := lipgloss.JoinVertical(lipgloss.Left, metaPane, patchPane)
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

	cw := width - 2
	barOn := lipgloss.NewStyle().Foreground(cAccent).Render("▌")
	var b strings.Builder
	for row := 0; row < h && m.offset+row < len(m.visible); row++ {
		i := m.visible[m.offset+row]
		c := m.commits[i]
		selected := m.offset+row == m.sel
		if selected {
			b.WriteString(barOn)
		} else {
			b.WriteString(" ")
		}
		b.WriteString(m.renderCommitRow(c, m.rows[i], gw, cw, selected))
		if row < h-1 {
			b.WriteString("\n")
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(cw+1).MaxWidth(cw+1).Render(b.String()),
		scrollbarCol(len(m.visible), m.offset, h))
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

	conflicts, staged, unstaged := 0, 0, 0
	for _, f := range m.files {
		switch {
		case f.Conflict:
			conflicts++
		case f.Staged:
			staged++
		default:
			unstaged++
		}
	}
	title := fmt.Sprintf("Working tree · %d staged · %d changed", staged, unstaged)
	if conflicts > 0 {
		title = fmt.Sprintf("Working tree · %d conflicts · %d staged · %d changed", conflicts, staged, unstaged)
	}
	if m.head.Merging {
		title += " · MERGING"
	}
	left := m.pane(title, m.renderStatusList(lw-2), lw, m.focus == focusLeft)

	diffTitle := "Diff"
	if len(m.statusItems) > 0 && m.fileSel < len(m.statusItems) {
		it := m.statusItems[m.fileSel]
		if it.isStash {
			diffTitle = "Diff · " + it.stash.Ref
		} else {
			diffTitle = "Diff · " + truncate(it.file.Path, rw-12)
		}
	}
	hunks := hunkRanges(m.fileDiff)
	if len(hunks) > 0 && m.focus == focusRight {
		diffTitle += fmt.Sprintf(" · hunk %d/%d", clamp(m.hunkSel, 0, len(hunks)-1)+1, len(hunks))
	}
	right := m.pane(diffTitle, m.renderStatusDiff(rw-2), rw, m.focus == focusRight)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderStatusDiff is renderDiffLines plus hunk-selection highlighting.
func (m model) renderStatusDiff(width int) string {
	lines := m.fileDiff
	if len(lines) == 0 {
		return sDim.Render("  nothing selected")
	}
	hunks := hunkRanges(lines)
	selStart := -1
	if len(hunks) > 0 {
		selStart = hunks[clamp(m.hunkSel, 0, len(hunks)-1)][0]
	}
	h := m.listHeight()
	cw := width - 1
	var b strings.Builder
	rendered := 0
	for i := m.diffOff; i < len(lines) && rendered < h; i++ {
		if i == selStart && m.focus == focusRight {
			b.WriteString(sHunkSel.Width(cw).Render(truncate(" "+lines[i]+" · space to stage/unstage ", cw)))
		} else {
			b.WriteString(styleDiffLine(lines[i], cw))
		}
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(cw).MaxWidth(cw).Render(b.String()),
		scrollbarCol(len(lines), m.diffOff, h))
}

// stashLabel renders a stash description as a branch chip + clean message.
func stashLabel(st Stash, selected bool, width int) string {
	branch, msg := stashMeta(st.Desc)
	bg := lipgloss.NewStyle()
	if selected {
		bg = bg.Background(cSelBg)
	}
	refStyle := lipgloss.NewStyle().Foreground(cMagenta).Bold(true)
	msgStyle := sText
	if selected {
		refStyle = refStyle.Background(cSelBg)
		msgStyle = sBright.Background(cSelBg)
	}
	out := refStyle.Render(st.Ref)
	used := len([]rune(st.Ref))
	if branch != "" {
		chip := sRefBranch.Render("⎇ " + branch)
		out += bg.Render(" ") + chip
		used += 1 + lipgloss.Width(chip)
	}
	out += bg.Render(" ") + msgStyle.Render(truncate(msg, max(width-used-1, 4)))
	return out
}

func (m model) renderStatusList(width int) string {
	if len(m.statusRows) == 0 {
		return sOk.Render("  ✓ working tree clean") + "\n\n" +
			sDim.Render("  nothing to stage or commit")
	}
	h := m.listHeight()
	var b strings.Builder
	rendered := 0
	for i := m.fileOffset; i < len(m.statusRows) && rendered < h; i++ {
		row := m.statusRows[i]
		if row.item < 0 {
			// section or directory header: full-width band
			style := sSectionBand.Foreground(cAccent)
			if strings.HasPrefix(row.text, "   ▾") {
				style = sDim
			} else if strings.HasPrefix(row.text, "✗") {
				style = sSectionBand.Foreground(cRed)
			} else if strings.HasPrefix(row.text, "≡") {
				style = sSectionBand.Foreground(cMagenta)
			}
			if strings.HasPrefix(row.text, "   ▾") {
				b.WriteString(style.Render(truncate(row.text, width)))
			} else {
				b.WriteString(style.Width(width).Render(truncate(row.text, width)))
			}
		} else {
			it := m.statusItems[row.item]
			selected := row.item == m.fileSel
			bg := lipgloss.NewStyle()
			if selected {
				bg = bg.Background(cSelBg)
			}
			var line string
			if it.isStash {
				line = bg.Render("  ") + stashLabel(it.stash, selected, width-2)
			} else {
				f := it.file
				target := statusTarget(f.Path)
				name := filepath.Base(target)
				indent := "  "
				if filepath.Dir(target) != "." {
					indent = "    "
				}
				codeStyle := statusCodeStyle(f.Code)
				if f.Conflict {
					codeStyle = sErr
				}
				nameStyle := sText
				if selected {
					codeStyle = codeStyle.Background(cSelBg)
					nameStyle = sBright.Background(cSelBg)
				}
				line = bg.Render(indent) + codeStyle.Render(f.Code) + bg.Render(" ") +
					nameStyle.Render(truncate(name, width-len(indent)-3))
			}
			pad := width - lipgloss.Width(line)
			if pad > 0 {
				line += bg.Render(strings.Repeat(" ", pad))
			}
			b.WriteString(line)
		}
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
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
		ageStyle := sDim
		if selected {
			ageStyle = ageStyle.Background(cSelBg)
		}
		age := st.Age
		line := bg.Render(" ") + stashLabel(st, selected, width-len([]rune(age))-3)
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
	if m.view == viewCommits {
		h = m.bodyHeight() - m.detailMetaHeight() - 2
	}
	cw := width - 1
	var b strings.Builder
	rendered := 0
	for i := offset; i < len(lines) && rendered < h; i++ {
		b.WriteString(styleDiffLine(lines[i], cw))
		rendered++
		if rendered < h {
			b.WriteString("\n")
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(cw).MaxWidth(cw).Render(b.String()),
		scrollbarCol(len(lines), offset, h))
}

func styleDiffLine(line string, width int) string {
	t := truncate(line, width)
	switch {
	case strings.HasPrefix(line, "<<<<<<<"), strings.HasPrefix(line, ">>>>>>>"),
		strings.HasPrefix(line, "======="):
		return sConflictMark.Width(width).Render(t)
	case strings.HasPrefix(line, "diff --git"), strings.HasPrefix(line, "commit "),
		strings.HasPrefix(line, "▸ "):
		return sDiffHeader.Width(width).Render(t)
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		return sDiffMeta.Render(t)
	case strings.HasPrefix(line, "@@"):
		return sDiffHunk.Width(width).Render(t)
	case strings.HasPrefix(line, "+"):
		return sDiffAdd.Width(width).Render(t)
	case strings.HasPrefix(line, "-"):
		return sDiffDel.Width(width).Render(t)
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

// ---- choice popup ----

func (m model) renderChoice() string {
	var b strings.Builder
	b.WriteString(sPopupTitle.Render(" "+m.choiceTitle+" ") + "\n\n")
	for i, opt := range m.choiceOptions {
		marker, style := "  ", sText.Background(cBarBg)
		if i == m.choiceSel {
			marker, style = "▸ ", sBright.Background(cSelBg)
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", marker,
			sHelpKey.Background(cBarBg).Render(fmt.Sprintf("%d", i+1)), style.Render(" "+opt.label+" ")))
	}
	b.WriteString("\n" + sDim.Background(cBarBg).Render("↑↓ or click · ⏎ confirm · esc cancel"))
	return sPopup.Render(strings.TrimRight(b.String(), "\n"))
}

// ---- help overlay ----

func (m model) renderHelp() string {
	type row struct{ k, v string }
	section := func(title string, rows []row) string {
		var b strings.Builder
		b.WriteString(sPopupTitle.Render(" "+title+" ") + "\n")
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("%s %s\n",
				sHelpKey.Background(cBarBg).Width(15).Render(r.k),
				sText.Background(cBarBg).Render(r.v)))
		}
		return b.String()
	}
	left := section("Navigate", []row{
		{"↑↓ / ws / jk", "move · scroll"},
		{"tab / 1-4", "switch view"},
		{"a / d", "focus list ↔ details"},
		{"g G · ctrl+d u", "top/bottom · half page"},
		{"/", "search commits"},
		{"?", "this help · q quit"},
	}) + "\n" + section("Commits", []row{
		{"c / dbl-click", "checkout (picker if multi)"},
		{"b · t", "branch popup · focus mode"},
		{"n · T", "new branch · tag"},
		{"m y R v", "merge · pick · rebase · revert"},
	})
	right := section("Status", []row{
		{"space", "stage file / hunk"},
		{"[ ]", "select hunk"},
		{"D · A", "discard · amend"},
		{"S · H · B", "stash · history · blame"},
		{"⏎ u t · X", "resolve conflict · abort"},
		{"c", "commit → graph"},
	}) + "\n" + section("Branches & sync", []row{
		{"⏎ · n e x", "checkout · new/rename/del"},
		{"m · O", "merge · PR page"},
		{"f · p · P F", "fetch · pull · push/force"},
		{"o", "add / show origin"},
	})
	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().MarginRight(3).Render(strings.TrimRight(left, "\n")),
		strings.TrimRight(right, "\n"))
	title := sBright.Background(cBarBg).Render("git2 " + version + " — keyboard reference")
	return sPopup.Render(title + "\n\n" + cols)
}

func (m model) overlay(base, box string) string {
	return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, box)
}

// scrollbarCol draws a slim right-edge scrollbar for a list of rows.
func scrollbarCol(total, offset, rows int) string {
	if rows < 1 {
		rows = 1
	}
	var b strings.Builder
	if total <= rows {
		for i := 0; i < rows; i++ {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(" ")
		}
		return b.String()
	}
	thumb := max(rows*rows/total, 1)
	top := 0
	if total > rows {
		top = offset * (rows - thumb) / max(total-rows, 1)
	}
	track := sDim.Render("│")
	bar := lipgloss.NewStyle().Foreground(cAccent).Render("┃")
	for i := 0; i < rows; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		if i >= top && i < top+thumb {
			b.WriteString(bar)
		} else {
			b.WriteString(track)
		}
	}
	return b.String()
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
