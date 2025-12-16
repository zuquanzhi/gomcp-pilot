package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/logger"
)

// InterceptRequest represents a request that needs user approval.
type InterceptRequest struct {
	Upstream string
	Tool     string
	Args     string
	// ResponseChan is used to send the user's decision back to the bridge.
	// true = approve, false = deny
	ResponseChan chan bool
}

// InterceptChan is the global channel for incoming interception requests.
var InterceptChan = make(chan InterceptRequest)

// UpstreamStatus tracks the state of each upstream service
type UpstreamStatus struct {
	Name      string
	Status    string // "Running", "Stopped", "Error"
	CallCount int
	LastCall  time.Time
	Config    config.Upstream
}

// ToolInfo simplified struct for display
type ToolInfo struct {
	Name        string
	Description string
}

type Model struct {
	logs           []string
	requestPending *InterceptRequest
	quitting       bool
	width          int
	height         int

	// Viewports
	logViewport    viewport.Model
	detailViewport viewport.Model

	// App State
	upstreams []UpstreamStatus
	startTime time.Time

	// Navigation State
	selectedIdx int
	showDetails bool

	// External Helpers
	toolFetcher  func(upstream string) ([]ToolInfo, error)
	currentTools []ToolInfo
	fetchError   string
}

func InitialModel(cfg *config.Config, fetcher func(upstream string) ([]ToolInfo, error)) Model {
	var ups []UpstreamStatus
	if cfg != nil {
		for _, u := range cfg.Upstreams {
			ups = append(ups, UpstreamStatus{
				Name:   u.Name,
				Status: "Running", // Assume running on start for now
				Config: u,
			})
		}
	}

	return Model{
		logs:        []string{"System initialized. Waiting for traffic..."},
		startTime:   time.Now(),
		upstreams:   ups,
		toolFetcher: fetcher,
		selectedIdx: 0,
		// Viewports initialized with default 0 size; resized on WindowSizeMsg
		logViewport:    viewport.New(0, 0),
		detailViewport: viewport.New(0, 0),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		waitForLog(),
		waitForIntercept(),
		tickCmd(),
	)
}

// Tick to update uptime
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

// Msg for async tool fetching
type toolsFetchedMsg struct {
	upstream string
	tools    []ToolInfo
	err      error
}

func (m Model) fetchToolsCmd(upstream string) tea.Cmd {
	return func() tea.Msg {
		if m.toolFetcher == nil {
			return toolsFetchedMsg{upstream: upstream, err: fmt.Errorf("no fetcher")}
		}
		tools, err := m.toolFetcher(upstream)
		return toolsFetchedMsg{upstream: upstream, tools: tools, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.requestPending != nil {
			switch msg.String() {
			case "y", "Y":
				m.requestPending.ResponseChan <- true
				m.requestPending = nil
				return m, waitForIntercept()
			case "n", "N":
				m.requestPending.ResponseChan <- false
				m.requestPending = nil
				return m, waitForIntercept()
			}
			return m, nil
		}

		// Global keys
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter", "space":
			m.showDetails = !m.showDetails
			if m.showDetails {
				return m, m.fetchToolsCmd(m.upstreams[m.selectedIdx].Name)
			}
		case "pgup":
			m.logViewport.HalfViewUp()
			m.detailViewport.HalfViewUp()
		case "pgdown":
			m.logViewport.HalfViewDown()
			m.detailViewport.HalfViewDown()
		}

		// Navigation (Sidebar) - Consumes Up/Down/j/k
		// We do NOT forward these to viewport to avoid double scrolling/conflict
		switch msg.String() {
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				// Reset details scroll to top when switching
				m.detailViewport.GotoTop()
				if m.showDetails {
					cmds = append(cmds, m.fetchToolsCmd(m.upstreams[m.selectedIdx].Name))
				}
			}
			return m, tea.Batch(cmds...) // Return early, don't pass to viewport
		case "down", "j":
			if m.selectedIdx < len(m.upstreams)-1 {
				m.selectedIdx++
				// Reset details scroll to top when switching
				m.detailViewport.GotoTop()
				if m.showDetails {
					cmds = append(cmds, m.fetchToolsCmd(m.upstreams[m.selectedIdx].Name))
				}
			}
			return m, tea.Batch(cmds...) // Return early
		}

		// Other keys - Pass to Viewport
		var cmd tea.Cmd
		m.logViewport, cmd = m.logViewport.Update(msg)
		cmds = append(cmds, cmd)

		m.detailViewport, cmd = m.detailViewport.Update(msg)
		cmds = append(cmds, cmd)

	case tea.MouseMsg:
		if msg.Type == tea.MouseWheelUp {
			m.logViewport.LineUp(3)
			m.detailViewport.LineUp(3)
		} else if msg.Type == tea.MouseWheelDown {
			m.logViewport.LineDown(3)
			m.detailViewport.LineDown(3)
		}

		// Pass to viewport as well, just in case (though we handled scroll essentially)
		var cmd tea.Cmd
		m.logViewport, cmd = m.logViewport.Update(msg)
		cmds = append(cmds, cmd)

		m.detailViewport, cmd = m.detailViewport.Update(msg)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Recalculate layout
		// Sidebar fixed at 25
		sidebarWidth := 25
		// Header (1 line text + 1 line margin) = 2
		headerHeight := 2

		mainWidth := m.width - sidebarWidth - 4
		if mainWidth < 10 {
			mainWidth = 10
		}

		mainHeight := m.height - headerHeight
		if mainHeight < 5 {
			mainHeight = 5
		}

		m.logViewport.Width = mainWidth
		m.logViewport.Height = mainHeight

		m.detailViewport.Width = mainWidth
		m.detailViewport.Height = mainHeight

		// Force re-render content for new width wrap
		// Also ensure we aren't showing stale empty content if we have data?
		// Note: init doesn't fetch, so this is fine.
		m.logViewport.SetContent(m.renderLogContent())
		m.detailViewport.SetContent(m.renderDetailContent())

	case tickMsg:
		cmds = append(cmds, tickCmd())

	case toolsFetchedMsg:
		// Only update if still selected (simple consistency check)
		if m.selectedIdx < len(m.upstreams) && m.upstreams[m.selectedIdx].Name == msg.upstream {
			if msg.err != nil {
				m.fetchError = msg.err.Error()
				m.currentTools = nil
			} else {
				m.fetchError = ""
				m.currentTools = msg.tools
			}
			m.detailViewport.SetContent(m.renderDetailContent())
		}

	case logger.LogEntry:
		// Add new log line
		timeStamp := msg.Timestamp.Format("15:04:05")

		// Apply styles based on level
		lvlStyle := styleLogInfo
		if msg.Level == "WARN" {
			lvlStyle = styleLogWarn
		} else if msg.Level == "ERROR" {
			lvlStyle = styleLogError
		}

		line := fmt.Sprintf("%s %s | %s",
			styleLogTimeStamp.Render("["+timeStamp+"]"),
			lvlStyle.Render(msg.Level),
			msg.Message)

		m.logs = append(m.logs, line)
		// Keep a decent buffer
		if len(m.logs) > 1000 {
			m.logs = m.logs[len(m.logs)-1000:]
		}

		m.logViewport.SetContent(m.renderLogContent())
		// Auto-scroll to bottom if we were already there or close to it?
		// For simplicity, always auto-scroll for logs unless user scrolled up
		// Viewport logic: if AtBottom(), keep AtBottom.
		// Since we don't have complex logic here easily, let's just goto bottom for now.
		m.logViewport.GotoBottom()

		cmds = append(cmds, waitForLog())

	case InterceptRequest:
		m.requestPending = &msg
		// Update stats for this upstream (simple finder)
		for i, u := range m.upstreams {
			if u.Name == msg.Upstream {
				m.upstreams[i].CallCount++
				m.upstreams[i].LastCall = time.Now()
			}
		}
		// Refresh details because stats changed
		m.detailViewport.SetContent(m.renderDetailContent())
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.requestPending != nil {
		return m.renderInterceptModal()
	}

	header := m.renderHeader()
	sidebar := m.renderSidebar()

	var mainPane string
	if m.showDetails {
		mainPane = styleLogPane.Width(m.detailViewport.Width).Render(m.detailViewport.View())
	} else {
		mainPane = styleLogPane.Width(m.logViewport.Width).Render(m.logViewport.View())
	}

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebar,
		mainPane,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
	)
}

func (m Model) renderHeader() string {
	title := styleHeaderTitle.Render("GOMCP PILOT")
	uptime := time.Since(m.startTime).Round(time.Second)
	status := fmt.Sprintf("UPTIME: %s  |  PORT: 8080", uptime)

	mode := "LOG MONITOR"
	if m.showDetails {
		mode = "DETAIL INSPECTOR"
	}
	right := lipgloss.NewStyle().Foreground(cForeground).Render(mode)

	pad := m.width - lipgloss.Width(title) - lipgloss.Width(status) - lipgloss.Width(right) - 4
	if pad < 1 {
		pad = 1
	}
	space := strings.Repeat(" ", pad)

	style := styleHeader
	if m.width > 0 {
		// Use width-1 to avoid accidental wrapping at the exact edge o	if m.width > 0 {
		// Use width-1 to avoid accidental wrapping at the exact edge of the terminal
		style = style.Width(m.width - 1)
	}
	return style.Render(fmt.Sprintf("%s %s%s%s", title, status, space, right))
}

func (m Model) renderSidebar() string {
	s := styleSidebarHeader.Render("UPSTREAMS") + "\n\n"

	for i, u := range m.upstreams {
		icon := styleStatusRunning
		if u.Status != "Running" {
			icon = styleStatusStopped
		}

		name := u.Name
		if len(name) > 15 {
			name = name[:12] + "..."
		}

		line := fmt.Sprintf("%s %s", icon, name)

		if i == m.selectedIdx {
			line = "> " + line
			line = lipgloss.NewStyle().Foreground(cHighlight).Bold(true).Render(line)
		} else {
			line = "  " + line
			line = styleUpstreamItem.Render(line)
		}

		s += line + "\n"
	}

	s += "\n\n" + lipgloss.NewStyle().Foreground(cComment).Italic(true).Render("Use ↑/↓ to nav\nEnter for details")

	return styleSidebar.Render(s)
}

func (m Model) renderLogContent() string {
	// Simple join
	if len(m.logs) == 0 {
		return "No logs yet..."
	}
	return strings.Join(m.logs, "\n")
}

func (m Model) renderDetailContent() string {
	if m.selectedIdx >= len(m.upstreams) {
		return "No selection"
	}
	u := m.upstreams[m.selectedIdx]

	s := lipgloss.NewStyle().Foreground(cAccent).Bold(true).Underline(true).Render(strings.ToUpper(u.Name)) + "\n\n"

	// Stats
	s += fmt.Sprintf("State:      %s\n", u.Status)
	s += fmt.Sprintf("Calls:      %d\n", u.CallCount)
	s += fmt.Sprintf("Last Call:  %s\n\n", u.LastCall.Format("15:04:05"))

	// Config
	kStyle := lipgloss.NewStyle().Foreground(cComment)
	vStyle := lipgloss.NewStyle().Foreground(cForeground)

	s += lipgloss.NewStyle().Foreground(cForeground).Bold(true).Render("CONFIGURATION:") + "\n"
	s += fmt.Sprintf("%s %s\n", kStyle.Render("Command:"), vStyle.Render(u.Config.Command))
	s += fmt.Sprintf("%s    %s\n", kStyle.Render("Args:"), vStyle.Render(strings.Join(u.Config.Args, " ")))
	if u.Config.Workdir != "" {
		s += fmt.Sprintf("%s %s\n", kStyle.Render("Workdir:"), vStyle.Render(u.Config.Workdir))
	}
	s += "\n"

	// Tools
	s += lipgloss.NewStyle().Foreground(cForeground).Bold(true).Render("AVAILABLE TOOLS:") + "\n"

	if m.fetchError != "" {
		s += lipgloss.NewStyle().Foreground(cDanger).Render("Error fetching tools: "+m.fetchError) + "\n"
	} else if len(m.currentTools) == 0 {
		s += lipgloss.NewStyle().Foreground(cComment).Render("No tools exposed or loading...") + "\n"
	} else {
		for _, t := range m.currentTools {
			s += fmt.Sprintf("• %s\n", lipgloss.NewStyle().Foreground(cHighlight).Render(t.Name))
			if t.Description != "" {
				// Explicitly wrap description to viewport width
				// Viewport width might technically include padding logic in SetContent if styleLogPane has padding.
				// styleLogPane has PaddingLeft(1). So effectively available is Width-1.
				wrapWidth := m.detailViewport.Width - 2
				if wrapWidth < 10 {
					wrapWidth = 10
				}

				descStyle := lipgloss.NewStyle().Foreground(cComment).Width(wrapWidth)
				s += fmt.Sprintf("  %s\n", descStyle.Render(t.Description))
			}
			s += "\n"
		}
	}

	return s
}

func (m Model) renderInterceptModal() string {
	req := m.requestPending

	header := styleModalHeader.Render("⚠️  INTERCEPTION REQUIRED ⚠️")

	// Industrial detail view
	kStyle := styleKeyParams
	vStyle := lipgloss.NewStyle().Foreground(cForeground)

	details := fmt.Sprintf(
		"%s %s\n%s     %s\n%s     %s\n",
		kStyle.Render("UPSTREAM:"), vStyle.Render(req.Upstream),
		kStyle.Render("TOOL:"), vStyle.Render(req.Tool),
		kStyle.Render("ARGS:"), vStyle.Render(req.Args),
	)

	question := "\n" + lipgloss.NewStyle().Bold(true).Render("ALLOW EXECUTION? (Y/N)")

	box := styleModalBox.Render(header + "\n\n" + details + question)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

// Commands
func waitForLog() tea.Cmd {
	return func() tea.Msg {
		return <-logger.LogChan
	}
}

func waitForIntercept() tea.Cmd {
	return func() tea.Msg {
		return <-InterceptChan
	}
}
