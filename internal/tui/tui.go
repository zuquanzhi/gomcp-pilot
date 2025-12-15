package tui

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UI 负责事件流可视化，提供请求列表、详情和状态栏。
type UI struct {
	events chan tea.Msg
	nextID int32
}

func New() *UI {
	return &UI{
		events: make(chan tea.Msg, 64),
	}
}

// Log 将文本事件推送到 UI。
func (u *UI) Log(msg string) { u.push(logMsg(msg)) }

// Status 刷新底部状态栏。
func (u *UI) Status(msg string) { u.push(statusMsg(msg)) }

// BeginRequest 记录请求并返回请求 ID。
func (u *UI) BeginRequest(tool, action, target string) int {
	id := int(atomic.AddInt32(&u.nextID, 1))
	u.push(requestMsg{
		id:     id,
		tool:   tool,
		action: action,
		target: target,
		status: "pending",
	})
	u.Log(fmt.Sprintf("[call:%d] %s %s %s", id, tool, action, target))
	return id
}

// ResolveRequest 更新请求结果。
func (u *UI) ResolveRequest(id int, status, reason string) {
	u.push(requestMsg{id: id, status: status, reason: reason})
	u.Log(fmt.Sprintf("[decision:%d] %s (%s)", id, status, reason))
}

// push 将消息放入 channel，背压时丢弃。
func (u *UI) push(msg tea.Msg) {
	select {
	case u.events <- msg:
	default:
	}
}

// Run 启动 Bubbletea UI，显示事件流。
func (u *UI) Run(ctx context.Context) {
	m := model{
		events: u.events,
		status: "ready",
	}
	p := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithAltScreen(),       // 使用备用屏幕，退出时恢复原终端内容
		tea.WithMouseCellMotion(), // 支持鼠标
	)
	_, _ = p.Run()
}

type model struct {
	events <-chan tea.Msg
	logs   []string

	requests []requestRow
	selected int
	status   string
}

type logMsg string
type statusMsg string

type requestMsg struct {
	id     int
	tool   string
	action string
	target string
	status string
	reason string
}

type requestRow struct {
	id     int
	tool   string
	action string
	target string
	status string
	reason string
}

func (m model) Init() tea.Cmd {
	return waitForEvent(m.events)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case logMsg:
		m.logs = append(m.logs, string(v))
		if len(m.logs) > 50 {
			m.logs = m.logs[len(m.logs)-50:]
		}
		return m, waitForEvent(m.events)
	case statusMsg:
		m.status = string(v)
		return m, waitForEvent(m.events)
	case requestMsg:
		m.upsertRequest(v)
		return m, waitForEvent(m.events)
	case tea.KeyMsg:
		switch v.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.selected < len(m.requests)-1 {
				m.selected++
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	// 样式定义
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00D9FF")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFA500")).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(0, 1).
		MarginBottom(1)

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(0, 1).
		MarginBottom(1)

	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(titleStyle.Render("🚀 GoMCP Pilot Dashboard"))
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render("📋 Requests (j/k to navigate, q to quit)"))
	b.WriteString("\n")
	b.WriteString(boxStyle.Render(renderRequests(m.requests, m.selected)))
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("🔍 Details"))
	b.WriteString("\n")
	b.WriteString(boxStyle.Render(renderDetails(m.requests, m.selected)))
	b.WriteString("\n")
	statusText := m.status
	if statusText == "" {
		statusText = "ready"
	}
	b.WriteString(statusStyle.Render("⚡ Status: " + statusText))
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render("📡 Events"))
	b.WriteString("\n")
	b.WriteString(boxStyle.Render(renderLogs(m.logs)))
	return b.String()
}

func waitForEvent(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *model) upsertRequest(r requestMsg) {
	for i := range m.requests {
		if m.requests[i].id == r.id {
			if r.tool != "" {
				m.requests[i].tool = r.tool
			}
			if r.action != "" {
				m.requests[i].action = r.action
			}
			if r.target != "" {
				m.requests[i].target = r.target
			}
			if r.status != "" {
				m.requests[i].status = r.status
			}
			if r.reason != "" {
				m.requests[i].reason = r.reason
			}
			return
		}
	}
	m.requests = append(m.requests, requestRow{
		id:     r.id,
		tool:   r.tool,
		action: r.action,
		target: r.target,
		status: r.status,
		reason: r.reason,
	})
}

func renderRequests(reqs []requestRow, selected int) string {
	var b strings.Builder
	
	// 表头样式
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFD700"))
	
	// 行样式
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#5F5FAF")).
		Foreground(lipgloss.Color("#FFFFFF"))
	
	pendingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	deniedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-4s %-16s %-14s %-20s %-14s", 
		"ID", "Tool", "Action", "Target", "Status")))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 72))
	b.WriteString("\n")
	
	for i, r := range reqs {
		prefix := "  "
		if i == selected {
			prefix = "▶ "
		}
		
		statusStr := r.status
		var statusStyle lipgloss.Style
		switch r.status {
		case "pending":
			statusStyle = pendingStyle
			statusStr = "⏳ " + statusStr
		case "done", "accepted":
			statusStyle = doneStyle
			statusStr = "✅ " + statusStr
		case "denied":
			statusStyle = deniedStyle
			statusStr = "❌ " + statusStr
		default:
			statusStyle = lipgloss.NewStyle()
		}
		
		line := fmt.Sprintf("%s%-3d %-16s %-14s %-20s", 
			prefix, r.id, 
			trim(r.tool, 16), 
			trim(r.action, 14), 
			trim(r.target, 20))
		
		if i == selected {
			line = selectedStyle.Render(line + " " + statusStr)
		} else {
			line = line + " " + statusStyle.Render(statusStr)
		}
		b.WriteString(line + "\n")
	}
	
	if len(reqs) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
		b.WriteString(emptyStyle.Render("  No requests yet..."))
	}
	return b.String()
}

func renderDetails(reqs []requestRow, selected int) string {
	if len(reqs) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
		return emptyStyle.Render("No request selected")
	}
	
	r := reqs[selected]
	
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00D9FF"))
	
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))
	
	var b strings.Builder
	b.WriteString(labelStyle.Render("Tool:    ") + valueStyle.Render(r.tool) + "\n")
	b.WriteString(labelStyle.Render("Action:  ") + valueStyle.Render(r.action) + "\n")
	b.WriteString(labelStyle.Render("Target:  ") + valueStyle.Render(r.target) + "\n")
	b.WriteString(labelStyle.Render("Status:  ") + valueStyle.Render(r.status) + "\n")
	if r.reason != "" {
		b.WriteString(labelStyle.Render("Reason:  ") + valueStyle.Render(r.reason) + "\n")
	}
	
	return b.String()
}

func renderStatus(status string) string {
	if status == "" {
		status = "ready"
	}
	return status
}

func renderLogs(logs []string) string {
	var b strings.Builder
	
	logStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	importantStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")).
		Bold(true)
	
	maxLogs := 8
	start := 0
	if len(logs) > maxLogs {
		start = len(logs) - maxLogs
	}
	
	for i := start; i < len(logs); i++ {
		l := logs[i]
		if strings.Contains(l, "started") || strings.Contains(l, "allowed") || strings.Contains(l, "done") {
			b.WriteString(importantStyle.Render("• " + l) + "\n")
		} else {
			b.WriteString(logStyle.Render("• " + l) + "\n")
		}
	}
	
	if len(logs) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
		b.WriteString(emptyStyle.Render("No events yet..."))
	}
	
	return b.String()
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
