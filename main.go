package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const version = "0.4.0"

func main() {
	path := "."
	printMode := false
	printLimit := 30

	args := os.Args[1:]
	for _, a := range args {
		switch a {
		case "-v", "--version":
			fmt.Println("git2 " + version)
			return
		case "-h", "--help":
			usage()
			return
		case "-p", "--print":
			printMode = true
		case "update", "--update":
			if err := selfUpdate(); err != nil {
				fmt.Fprintln(os.Stderr, "git2: "+err.Error())
				os.Exit(1)
			}
			return
		default:
			if !strings.HasPrefix(a, "-") {
				path = a
			}
		}
	}

	state := loadState()

	repo, err := findRepo(path)
	if err != nil {
		if printMode {
			fmt.Fprintln(os.Stderr, "git2: "+err.Error())
			os.Exit(1)
		}
		// not inside a repo: open the picker instead of bailing out
		lipgloss.SetHasDarkBackground(lipgloss.HasDarkBackground())
		picked, perr := runPicker(state)
		if perr != nil {
			fmt.Fprintln(os.Stderr, "git2: "+perr.Error())
			os.Exit(1)
		}
		state.LastDir = picked.dir
		if picked.choice == "" {
			saveState(state)
			return
		}
		repo, err = findRepo(picked.choice)
		if err != nil {
			saveState(state)
			fmt.Fprintln(os.Stderr, "git2: "+err.Error())
			os.Exit(1)
		}
	}

	if printMode {
		printGraph(repo, printLimit)
		return
	}

	state.Touch(repo.Root)
	saveState(state)

	// resolve light/dark background once, before bubbletea owns stdin
	lipgloss.SetHasDarkBackground(lipgloss.HasDarkBackground())

	p := tea.NewProgram(newModel(repo), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "git2: "+err.Error())
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`git2 — a beautiful terminal git client

usage:
  git2 [path]        open the repo at path (default: current directory);
                     outside a repo, a picker offers recent repos and a
                     directory browser
  git2 -p, --print   print the commit graph and exit
  git2 update        replace this binary with the latest release
  git2 -v, --version print version`)
}

// printGraph dumps the rendered commit tree to stdout (no TUI).
func printGraph(repo *Repo, limit int) {
	commits, err := repo.Commits(limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "git2: "+err.Error())
		os.Exit(1)
	}
	if len(commits) == 0 {
		fmt.Println("no commits")
		return
	}
	rows := BuildGraph(commits)
	gw := GraphWidth(rows, 16)
	for i, c := range commits {
		var g strings.Builder
		for j := 0; j < gw; j++ {
			ch := ' '
			color := 0
			if j < len(rows[i].Cells) {
				ch, color = rows[i].Cells[j].Ch, rows[i].Cells[j].Color
			}
			g.WriteString(laneStyle(color).Render(string(ch)))
			if j < gw-1 {
				pad := " "
				if ch == '─' || ch == '╭' || ch == '╰' || ch == '┼' {
					pad = "─"
				} else if j+1 < len(rows[i].Cells) {
					nc := rows[i].Cells[j+1].Ch
					if nc == '─' || nc == '╮' || nc == '╯' || nc == '┤' || nc == '┼' {
						pad = "─"
					}
				}
				g.WriteString(laneStyle(color).Render(pad))
			}
		}
		refs := ""
		for _, r := range c.Refs {
			switch r.Kind {
			case RefHead:
				refs += "[HEAD→" + r.Name + "] "
			case RefTag:
				refs += "(" + r.Name + ") "
			default:
				refs += "[" + r.Name + "] "
			}
		}
		fmt.Printf("%s %s%s %s\n", g.String(), refs, c.Subject, sDim.Render(c.ShortHash()[:7]))
	}
}
