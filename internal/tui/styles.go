package tui

import "github.com/charmbracelet/lipgloss"

// Color Palette - Industrial / Cyberpunk-lite
var (
	cBackground = lipgloss.Color("#1a1b26") // Deep Night
	cSidebar    = lipgloss.Color("#16161e") // Darker Side
	cForeground = lipgloss.Color("#a9b1d6") // Soft White
	cComment    = lipgloss.Color("#565f89") // Grey
	cAccent     = lipgloss.Color("#7aa2f7") // Blue
	cSuccess    = lipgloss.Color("#9ece6a") // Green
	cWarning    = lipgloss.Color("#e0af68") // Orange
	cDanger     = lipgloss.Color("#f7768e") // Red
	cHighlight  = lipgloss.Color("#bb9af7") // Purple
	cBorder     = lipgloss.Color("#2ac3de") // Cyan Border
)

var (
	// Layout
	styleApp = lipgloss.NewStyle().
			Background(cBackground).
			Foreground(cForeground)

	// Header
	styleHeader = lipgloss.NewStyle().
			Foreground(cBackground).
			Background(cAccent).
			Bold(true).
			Padding(0, 1).
			MarginBottom(1)

	styleHeaderTitle = lipgloss.NewStyle().
				Foreground(cAccent).
				Bold(true).
				MarginRight(1)

	// Sidebar (Upstreams)
	styleSidebar = lipgloss.NewStyle().
			Width(25).
			Border(lipgloss.NormalBorder(), false, true, false, false). // Right border
			BorderForeground(cComment).
			PaddingRight(1).
			MarginRight(1)

	styleSidebarHeader = lipgloss.NewStyle().
				Foreground(cComment).
				Bold(true).
				Underline(true).
				MarginBottom(1)

	styleUpstreamItem = lipgloss.NewStyle().
				PaddingLeft(1)

	styleStatusRunning = lipgloss.NewStyle().Foreground(cSuccess).SetString("●")
	styleStatusStopped = lipgloss.NewStyle().Foreground(cComment).SetString("○")
	styleStatusError   = lipgloss.NewStyle().Foreground(cDanger).SetString("✖")

	// Main Content (Logs)
	styleLogPane = lipgloss.NewStyle().
		// Border(lipgloss.NormalBorder()).
		// BorderForeground(cComment).
		PaddingLeft(1)

	styleLogTimeStamp = lipgloss.NewStyle().Foreground(cComment)
	styleLogInfo      = lipgloss.NewStyle().Foreground(cAccent)
	styleLogWarn      = lipgloss.NewStyle().Foreground(cWarning)
	styleLogError     = lipgloss.NewStyle().Foreground(cDanger)

	// Intercept Modal
	styleModalBox = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(cDanger).
			Padding(1, 2).
			Align(lipgloss.Center)

	styleModalHeader = lipgloss.NewStyle().
				Foreground(cDanger).
				Bold(true).
				Background(lipgloss.Color("#2a1b1b")). // Dark Red bg
				Padding(0, 1).
				MarginBottom(1)

	styleKeyParams = lipgloss.NewStyle().Foreground(cHighlight)
)
