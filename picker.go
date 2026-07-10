package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The repo picker is shown when git2 is launched outside a git repository.
// It offers the recently opened repos plus a directory browser to walk the
// filesystem and pick any repo.

const (
	pickRecent = iota
	pickBrowse
)

type dirEntry struct {
	name   string
	isRepo bool
}

type pickerModel struct {
	state  State
	mode   int
	width  int
	height int

	recent []string
	rsel   int

	dir     string
	entries []dirEntry
	bsel    int
	boff    int

	choice string
	errMsg string
}

func newPicker(state State) pickerModel {
	p := pickerModel{state: state, recent: state.ExistingRecent()}
	if len(p.recent) == 0 {
		p.mode = pickBrowse
	}
	dir := state.LastDir
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		} else {
			dir, _ = os.UserHomeDir()
		}
	}
	p.setDir(dir)
	return p
}

func (p *pickerModel) setDir(dir string) {
	p.dir = dir
	p.entries = nil
	p.bsel, p.boff = 0, 0
	items, err := os.ReadDir(dir)
	if err != nil {
		p.errMsg = err.Error()
		return
	}
	p.errMsg = ""
	for _, it := range items {
		if !it.IsDir() || strings.HasPrefix(it.Name(), ".") {
			continue
		}
		_, statErr := os.Stat(filepath.Join(dir, it.Name(), ".git"))
		p.entries = append(p.entries, dirEntry{name: it.Name(), isRepo: statErr == nil})
	}
	sort.Slice(p.entries, func(a, b int) bool {
		if p.entries[a].isRepo != p.entries[b].isRepo {
			return p.entries[a].isRepo
		}
		return strings.ToLower(p.entries[a].name) < strings.ToLower(p.entries[b].name)
	})
}

func (p pickerModel) Init() tea.Cmd { return nil }

func (p pickerModel) listHeight() int { return max(p.height-9, 3) }

func (p pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		return p, nil
	case tea.KeyMsg:
		return p.handleKey(msg.String())
	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp {
			return p.move(-1)
		}
		if msg.Button == tea.MouseButtonWheelDown {
			return p.move(1)
		}
	}
	return p, nil
}

func (p pickerModel) move(delta int) (tea.Model, tea.Cmd) {
	if p.mode == pickRecent {
		if len(p.recent) > 0 {
			p.rsel = clamp(p.rsel+delta, 0, len(p.recent)-1)
		}
	} else {
		if len(p.entries) > 0 {
			p.bsel = clamp(p.bsel+delta, 0, len(p.entries)-1)
			ensureVisible(p.bsel, &p.boff, p.listHeight())
		}
	}
	return p, nil
}

func (p pickerModel) handleKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c", "esc":
		return p, tea.Quit
	case "tab":
		if len(p.recent) > 0 {
			if p.mode == pickRecent {
				p.mode = pickBrowse
			} else {
				p.mode = pickRecent
			}
		}
		return p, nil
	case "j", "s", "down":
		return p.move(1)
	case "k", "w", "up":
		return p.move(-1)
	case "~":
		if home, err := os.UserHomeDir(); err == nil {
			p.mode = pickBrowse
			p.setDir(home)
		}
		return p, nil
	}

	if p.mode == pickRecent {
		switch key {
		case "enter", "l", "d", "right":
			if len(p.recent) > 0 {
				p.choice = p.recent[p.rsel]
				return p, tea.Quit
			}
		}
		return p, nil
	}

	// browse mode
	switch key {
	case "h", "a", "left", "backspace":
		parent := filepath.Dir(p.dir)
		if parent != p.dir {
			prev := filepath.Base(p.dir)
			p.setDir(parent)
			for i, e := range p.entries {
				if e.name == prev {
					p.bsel = i
					ensureVisible(p.bsel, &p.boff, p.listHeight())
					break
				}
			}
		}
	case "enter", "l", "d", "right":
		if len(p.entries) == 0 {
			return p, nil
		}
		e := p.entries[p.bsel]
		full := filepath.Join(p.dir, e.name)
		if e.isRepo {
			p.choice = full
			return p, tea.Quit
		}
		p.setDir(full)
	case ".":
		// open the current directory even if git2 didn't mark it as a repo
		// (findRepo walks up, so nested work dirs still resolve)
		p.choice = p.dir
		return p, tea.Quit
	}
	return p, nil
}

func (p pickerModel) View() string {
	if p.width == 0 {
		return "loading…"
	}
	var b strings.Builder
	b.WriteString(sHeaderRepo.Render("● git2 ") + sHeaderInfo.Render("· choose a repository"))
	b.WriteString("\n\n")

	// mode tabs
	recentTab, browseTab := sTabIdle, sTabIdle
	if p.mode == pickRecent {
		recentTab = sTabActive
	} else {
		browseTab = sTabActive
	}
	recentLabel := fmt.Sprintf("★ Recent (%d)", len(p.recent))
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Bottom,
		recentTab.Render(recentLabel), browseTab.Render("⌂ Browse")))
	b.WriteString("\n")

	h := p.listHeight()
	if p.mode == pickRecent {
		if len(p.recent) == 0 {
			b.WriteString(sDim.Render("  no recent repositories — press tab to browse"))
			b.WriteString("\n")
		}
		for i, r := range p.recent {
			if i >= h {
				break
			}
			marker, style := "  ", sText
			if i == p.rsel {
				marker, style = "▸ ", sBright.Background(cSelBg)
			}
			name := filepath.Base(r)
			line := marker + style.Render("⎇ "+name) + "  " + sDim.Render(collapseHome(r))
			b.WriteString(truncateANSI(line, p.width-2) + "\n")
		}
	} else {
		b.WriteString(sPaneTitle.Render(" "+collapseHome(p.dir)) + "\n")
		if p.errMsg != "" {
			b.WriteString(sErr.Render("  "+p.errMsg) + "\n")
		} else if len(p.entries) == 0 {
			b.WriteString(sDim.Render("  (no subdirectories)") + "\n")
		}
		for row := 0; row < h && p.boff+row < len(p.entries); row++ {
			e := p.entries[p.boff+row]
			marker, style := "  ", sText
			if p.boff+row == p.bsel {
				marker, style = "▸ ", sBright.Background(cSelBg)
			}
			icon, note := "▸ ", ""
			if e.isRepo {
				icon = "⎇ "
				note = "  " + sOk.Render("git repo")
			}
			line := marker + style.Render(icon+e.name) + note
			b.WriteString(truncateANSI(line, p.width-2) + "\n")
		}
	}

	b.WriteString("\n")
	hints := "↑↓/ws move · ⏎ open · ←→/ad up/into dir · tab recent/browse · ~ home · . open here · q quit"
	if p.mode == pickRecent {
		hints = "↑↓/ws move · ⏎ open · tab browse · q quit"
	}
	b.WriteString(sFooter.Width(p.width).Render(hints))
	return b.String()
}

func collapseHome(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// runPicker shows the repo picker and returns the chosen path ("" = quit).
func runPicker(state State) (pickerModel, error) {
	p := tea.NewProgram(newPicker(state), tea.WithAltScreen(), tea.WithMouseCellMotion())
	final, err := p.Run()
	if err != nil {
		return pickerModel{}, err
	}
	return final.(pickerModel), nil
}
