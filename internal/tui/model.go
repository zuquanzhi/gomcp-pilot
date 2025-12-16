package tui

import (
	"fmt"
	"strings"
	"time"

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
			})
		}
	}

	return Model{
		logs:        []string{"System initialized. Waiting for traffic..."},
		startTime:   time.Now(),
		upstreams:   ups,
		toolFetcher: fetcher,
		selectedIdx: 0,
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

		// Navigation
		switch msg.String() {
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				if m.showDetails {
					return m, m.fetchToolsCmd(m.upstreams[m.selectedIdx].Name)
				}
			}
		case "down", "j":
			if m.selectedIdx < len(m.upstreams)-1 {
				m.selectedIdx++
				if m.showDetails {
					return m, m.fetchToolsCmd(m.upstreams[m.selectedIdx].Name)
				}
			}
		case "enter", "space":
			m.showDetails = !m.showDetails
			if m.showDetails {
				// Trigger fetch
				return m, m.fetchToolsCmd(m.upstreams[m.selectedIdx].Name)
			}
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tickCmd()

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
		if len(m.logs) > 50 {
			m.logs = m.logs[len(m.logs)-50:]
		}
		return m, waitForLog()

	case InterceptRequest:
		m.requestPending = &msg
		// Update stats for this upstream (simple finder)
		for i, u := range m.upstreams {
			if u.Name == msg.Upstream {
				m.upstreams[i].CallCount++
				m.upstreams[i].LastCall = time.Now()
			}
		}
		return m, nil
	}

	return m, nil
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
	var mainContent string

	if m.showDetails {
		mainContent = m.renderDetailView()
	} else {
		mainContent = m.renderLogView()
	}

	// Calculate dynamic widths
	// Sidebar is fixed width in style (25)
	mainWidth := m.width - 25 - 4 // minus margins/borders
	if mainWidth < 10 {
		mainWidth = 10
	}

	var mainPane string
	if m.showDetails {
		// Details view doesn't have the log pane border usually, but let's keep it consistent
		mainPane = styleLogPane.Width(mainWidth).Render(mainContent)
	} else {
		mainPane = styleLogPane.Width(mainWidth).Render(mainContent)
	}

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebar,
		mainPane,
	)

	return styleApp.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
	))
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

	// Padding
	pad := m.width - lipgloss.Width(title) - lipgloss.Width(status) - lipgloss.Width(right) - 4
	if pad < 1 {
		pad = 1
	}
	space := strings.Repeat(" ", pad)

	return styleHeader.Width(m.width).Render(fmt.Sprintf("%s %s%s%s", title, status, space, right))
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

		// Highlight selected
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

func (m Model) renderDetailView() string {
	if m.selectedIdx >= len(m.upstreams) {
		return "No selection"
	}
	u := m.upstreams[m.selectedIdx]

	s := lipgloss.NewStyle().Foreground(cAccent).Bold(true).Underline(true).Render(strings.ToUpper(u.Name)) + "\n\n"

	// Stats
	s += fmt.Sprintf("State:      %s\n", u.Status)
	s += fmt.Sprintf("Calls:      %d\n", u.CallCount)
	s += fmt.Sprintf("Last Call:  %s\n\n", u.LastCall.Format("15:04:05"))

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
				// Wrap description gently?
				desc := t.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				s += fmt.Sprintf("  %s\n", lipgloss.NewStyle().Foreground(cComment).Render(desc))
			}
			s += "\n"
		}
	}

	return s
}

func (m Model) renderLogView() string {
	// Simple join
	if len(m.logs) == 0 {
		return "No logs yet..."
	}
	return strings.Join(m.logs, "\n")
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
