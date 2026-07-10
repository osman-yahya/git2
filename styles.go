package main

import "github.com/charmbracelet/lipgloss"

// lane palette: bright, distinct hues that cycle as branches multiply
var lanePalette = []lipgloss.Color{
	"#7aa2f7", // blue
	"#f7768e", // rose
	"#9ece6a", // green
	"#e0af68", // amber
	"#bb9af7", // purple
	"#7dcfff", // sky
	"#ff9e64", // orange
	"#2ac3de", // teal
}

var (
	cAccent  = lipgloss.Color("#7aa2f7")
	cText    = lipgloss.AdaptiveColor{Light: "#343b58", Dark: "#c0caf5"}
	cDim     = lipgloss.AdaptiveColor{Light: "#8990b3", Dark: "#565f89"}
	cBright  = lipgloss.AdaptiveColor{Light: "#0f0f14", Dark: "#ffffff"}
	cSelBg   = lipgloss.AdaptiveColor{Light: "#d5d9f0", Dark: "#2a2f4a"}
	cBarBg   = lipgloss.AdaptiveColor{Light: "#e4e6f3", Dark: "#1f2335"}
	cGreen   = lipgloss.Color("#9ece6a")
	cRed     = lipgloss.Color("#f7768e")
	cYellow  = lipgloss.Color("#e0af68")
	cCyan    = lipgloss.Color("#7dcfff")
	cMagenta = lipgloss.Color("#bb9af7")

	sText   = lipgloss.NewStyle().Foreground(cText)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sBright = lipgloss.NewStyle().Foreground(cBright).Bold(true)

	// header / footer bars
	sHeader     = lipgloss.NewStyle().Background(cBarBg).Padding(0, 1)
	sHeaderRepo = lipgloss.NewStyle().Background(cBarBg).Foreground(cAccent).Bold(true)
	sHeaderInfo = lipgloss.NewStyle().Background(cBarBg).Foreground(cDim)
	sFooter     = lipgloss.NewStyle().Background(cBarBg).Foreground(cDim).Padding(0, 1)
	sFooterKey  = lipgloss.NewStyle().Background(cBarBg).Foreground(cBright).Bold(true)

	// tabs
	sTabActive = lipgloss.NewStyle().Foreground(cBright).Bold(true).
			Padding(0, 2).Border(lipgloss.Border{Bottom: "─"}, false, false, true, false).
			BorderForeground(cAccent)
	sTabIdle = lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).
			Border(lipgloss.Border{Bottom: "─"}, false, false, true, false).
			BorderForeground(cDim)

	// panes
	sPaneFocus = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent)
	sPaneBlur  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cDim)
	sPaneTitle = lipgloss.NewStyle().Foreground(cAccent).Bold(true)

	// ref badges
	sRefHead = lipgloss.NewStyle().Background(lipgloss.Color("#9ece6a")).
			Foreground(lipgloss.Color("#1a1b26")).Bold(true).Padding(0, 1)
	sRefBranch = lipgloss.NewStyle().Background(lipgloss.Color("#7aa2f7")).
			Foreground(lipgloss.Color("#1a1b26")).Bold(true).Padding(0, 1)
	sRefRemote = lipgloss.NewStyle().Background(lipgloss.Color("#414868")).
			Foreground(lipgloss.Color("#c0caf5")).Padding(0, 1)
	sRefTag = lipgloss.NewStyle().Background(lipgloss.Color("#e0af68")).
		Foreground(lipgloss.Color("#1a1b26")).Bold(true).Padding(0, 1)

	// diff colors
	sDiffAdd    = lipgloss.NewStyle().Foreground(cGreen)
	sDiffDel    = lipgloss.NewStyle().Foreground(cRed)
	sDiffHunk   = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	sDiffHeader = lipgloss.NewStyle().Foreground(cMagenta).Bold(true)
	sDiffMeta   = lipgloss.NewStyle().Foreground(cDim)

	// status codes
	sStatusM = lipgloss.NewStyle().Foreground(cYellow).Bold(true)
	sStatusA = lipgloss.NewStyle().Foreground(cGreen).Bold(true)
	sStatusD = lipgloss.NewStyle().Foreground(cRed).Bold(true)
	sStatusQ = lipgloss.NewStyle().Foreground(cCyan).Bold(true)

	sErr = lipgloss.NewStyle().Foreground(cRed).Bold(true)
	sOk  = lipgloss.NewStyle().Foreground(cGreen)

	sHelpBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(cAccent).Padding(1, 3)
	sHelpKey = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
)

func laneStyle(i int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lanePalette[i%len(lanePalette)])
}

func statusCodeStyle(code string) lipgloss.Style {
	switch code {
	case "A", "?":
		return sStatusA
	case "D":
		return sStatusD
	case "R", "C":
		return sStatusQ
	default:
		return sStatusM
	}
}
