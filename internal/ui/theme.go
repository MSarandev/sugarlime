package ui

import "github.com/charmbracelet/lipgloss"

// An adaptive palette, so the UI stays legible on light and dark terminals.
var (
	colText    = lipgloss.AdaptiveColor{Light: "236", Dark: "252"}
	colDim     = lipgloss.AdaptiveColor{Light: "244", Dark: "245"}
	colFaint   = lipgloss.AdaptiveColor{Light: "250", Dark: "240"}
	colAccent  = lipgloss.AdaptiveColor{Light: "63", Dark: "111"}
	colAccent2 = lipgloss.AdaptiveColor{Light: "97", Dark: "140"}
	colOK      = lipgloss.AdaptiveColor{Light: "28", Dark: "78"}
	colWarn    = lipgloss.AdaptiveColor{Light: "130", Dark: "214"}
	colErr     = lipgloss.AdaptiveColor{Light: "124", Dark: "203"}
	colBorder  = lipgloss.AdaptiveColor{Light: "252", Dark: "238"}
)

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleText  = lipgloss.NewStyle().Foreground(colText)
	styleDim   = lipgloss.NewStyle().Foreground(colDim)
	styleFaint = lipgloss.NewStyle().Foreground(colFaint)
	styleOK    = lipgloss.NewStyle().Foreground(colOK)
	styleWarn  = lipgloss.NewStyle().Foreground(colWarn)
	styleErr   = lipgloss.NewStyle().Foreground(colErr)

	styleTabActive = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).
			Background(colAccent).Padding(0, 1)
	styleTabIdle = lipgloss.NewStyle().Foreground(colDim).Padding(0, 1)

	styleCursor   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(colAccent)
	styleSelected = lipgloss.NewStyle().Bold(true).Foreground(colAccent)

	styleKey = lipgloss.NewStyle().Bold(true).Foreground(colAccent2)

	// styleFileName is the headline: which settings file is open.
	styleFileName = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("231")).Background(colAccent)

	styleValue   = lipgloss.NewStyle().Foreground(colOK)
	styleUnset   = lipgloss.NewStyle().Foreground(colFaint).Italic(true)
	styleBadge   = lipgloss.NewStyle().Foreground(colWarn)
	stylePane    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colBorder)
	styleOverlay = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).Padding(1, 2)
)

// icons kept ASCII-safe so the UI survives a terminal without a Nerd Font.
const (
	iconCursor   = "›"
	iconDrill    = "▸"
	iconDirty    = "●"
	iconBullet   = "•"
	iconCheckOn  = "[x]"
	iconCheckOff = "[ ]"
)
