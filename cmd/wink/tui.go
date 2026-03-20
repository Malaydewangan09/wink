package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// service palette — distinct, readable on dark bg
	serviceColors = []lipgloss.Color{
		"#5eba8a", // teal-green
		"#d4a04a", // amber
		"#a87fd4", // purple
		"#6699cc", // blue
		"#c46a6a", // red
		"#8ab84a", // lime
	}

	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleFaint   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	styleTs      = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	styleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))

	styleHeader = lipgloss.NewStyle().
			Background(lipgloss.Color("233")).
			Foreground(lipgloss.Color("240"))

	styleFooter = lipgloss.NewStyle().
			Background(lipgloss.Color("233")).
			Foreground(lipgloss.Color("238"))

	styleLogErr   = lipgloss.NewStyle().Foreground(lipgloss.Color("167")) // soft red
	styleLogWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("179")) // soft amber
	styleLogOk    = lipgloss.NewStyle().Foreground(lipgloss.Color("72"))  // medium green
	styleLogDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // readable grey
	styleLogTrace = lipgloss.NewStyle().Foreground(lipgloss.Color("242")) // stack traces, dimmer but legible
)

type tickMsg time.Time
type logsMsg []LogLine
type servicesMsg map[string]*Service
type actionDoneMsg struct {
	name   string
	action string
	err    error
}

type focusPane int

const (
	paneServices focusPane = iota
	paneLogs
)

type model struct {
	services      map[string]*Service
	serviceOrder  []string
	logs          []LogLine
	selectedSvc   int
	logOffset     int
	focus         focusPane
	width         int
	height        int
	filter        string
	showAll       bool
	ready         bool
	searchMode    bool
	searchQuery   string
	confirmRemove  bool
	statusMsg      string
	statusClears   int
	showTimestamps bool
}

func initialModel() model {
	return model{
		services:       map[string]*Service{},
		focus:          paneServices,
		showTimestamps: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), loadDataCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		services, _ := loadServices()
		return servicesMsg(services)
	}
}

func loadLogsCmd(filter string) tea.Cmd {
	return func() tea.Msg {
		logs, _ := readLogs(filter)
		return logsMsg(logs)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case actionDoneMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
		} else {
			m.statusMsg = msg.name + " " + msg.action
		}
		m.statusClears = 4
		return m, nil

	case tickMsg:
		if m.statusClears > 0 {
			m.statusClears--
			if m.statusClears == 0 {
				m.statusMsg = ""
			}
		}
		return m, tea.Batch(tickCmd(), loadDataCmd(), loadLogsCmd(m.filter))

	case servicesMsg:
		wasEmpty := len(m.serviceOrder) == 0
		m.services = map[string]*Service(msg)
		m.serviceOrder = sortedKeys(m.services)
		if m.selectedSvc >= len(m.serviceOrder) {
			m.selectedSvc = max(0, len(m.serviceOrder)-1)
		}
		// on first load, sync filter to selected service
		if wasEmpty && len(m.serviceOrder) > 0 {
			m.updateFilter()
		}
		return m, nil

	case logsMsg:
		m.logs = []LogLine(msg)
		return m, nil

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.logOffset = min(m.logOffset+3, m.maxLogOffset())
		case tea.MouseButtonWheelDown:
			m.logOffset = max(m.logOffset-3, 0)
		}

	case tea.KeyMsg:
		// search mode intercepts all keys
		if m.searchMode {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.searchMode = false
				m.searchQuery = ""
				m.logOffset = 0
			case "enter":
				m.searchMode = false
				m.logOffset = 0
			case "backspace", "ctrl+h":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.logOffset = 0
				}
			default:
				if len(msg.String()) == 1 {
					m.searchQuery += msg.String()
					m.logOffset = 0
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			if m.focus == paneServices {
				m.focus = paneLogs
			} else {
				m.focus = paneServices
			}

		case "up", "k":
			if m.focus == paneServices {
				if m.selectedSvc > 0 {
					m.selectedSvc--
					m.showAll = false
					m.updateFilter()
					m.logOffset = 0
				}
			} else {
				// up = older logs = increase offset
				m.logOffset = min(m.logOffset+1, m.maxLogOffset())
			}

		case "down", "j":
			if m.focus == paneServices {
				if m.selectedSvc < len(m.serviceOrder)-1 {
					m.selectedSvc++
					m.showAll = false
					m.updateFilter()
					m.logOffset = 0
				}
			} else {
				// down = newer logs = decrease offset
				m.logOffset = max(m.logOffset-1, 0)
			}

		case "a":
			m.showAll = true
			m.filter = ""
			m.logOffset = 0

		case "o":
			if len(m.serviceOrder) > 0 {
				name := m.serviceOrder[m.selectedSvc]
				if svc, ok := m.services[name]; ok && svc.Port > 0 {
					url := fmt.Sprintf("http://localhost:%d", svc.Port)
					_ = exec.Command("open", url).Start()
				}
			}

		case "s":
			if len(m.serviceOrder) > 0 {
				name := m.serviceOrder[m.selectedSvc]
				if svc, ok := m.services[name]; ok && svc.Status == StatusRunning {
					proc, err := os.FindProcess(svc.PID)
					if err == nil {
						_ = proc.Signal(syscall.SIGTERM)
					}
				}
			}

		case "r":
			if len(m.serviceOrder) > 0 {
				name := m.serviceOrder[m.selectedSvc]
				m.confirmRemove = false
				return m, tuiRestartCmd(name)
			}

		case "x":
			if len(m.serviceOrder) > 0 {
				name := m.serviceOrder[m.selectedSvc]
				if svc, ok := m.services[name]; ok && svc.Status == StatusRunning {
					m.statusMsg = name + " is running, stop first"
					m.statusClears = 4
				} else if m.confirmRemove {
					m.confirmRemove = false
					return m, tuiRemoveCmd(name)
				} else {
					m.confirmRemove = true
				}
			}

		case "G":
			m.logOffset = 0

		case "g":
			// jump to top
			m.logOffset = m.maxLogOffset()

		case "t":
			m.showTimestamps = !m.showTimestamps

		case "/":
			m.searchMode = true
			m.searchQuery = ""
			m.logOffset = 0

		case "esc":
			if m.confirmRemove {
				m.confirmRemove = false
			} else if m.searchQuery != "" {
				m.searchQuery = ""
				m.logOffset = 0
			}
		}
	}

	return m, nil
}

func (m *model) updateFilter() {
	if len(m.serviceOrder) == 0 {
		m.filter = ""
		return
	}
	m.filter = m.serviceOrder[m.selectedSvc]
}

func (m model) View() string {
	if !m.ready {
		return "\n  loading..."
	}

	sidebarW := 22
	contentH := m.height - 2 // header + footer

	header := m.renderHeader()
	sidebar := m.renderServices(sidebarW, contentH)
	divider := m.renderDivider(contentH)
	logs := m.renderLogs(m.width-sidebarW-1, contentH)
	footer := m.renderFooter()

	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divider, logs)
	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m model) renderHeader() string {
	running, dead := 0, 0
	for _, svc := range m.services {
		if svc.Status == StatusRunning {
			running++
		}
		if svc.Status == StatusDead {
			dead++
		}
	}

	name := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true).
		Render("wink")

	sep := styleDivider.Render("  ·  ")
	total := styleDim.Render(fmt.Sprintf("%d services", len(m.services)))
	run := lipgloss.NewStyle().Foreground(lipgloss.Color("72")).Render(fmt.Sprintf("%d running", running))
	ded := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(fmt.Sprintf("%d dead", dead))

	left := "  " + name + sep + total + sep + run + sep + ded
	return styleHeader.Width(m.width).Render(left)
}

func (m model) renderDivider(height int) string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = styleDivider.Render("│")
	}
	return strings.Join(lines, "\n")
}

func (m model) renderServices(width, height int) string {
	label := styleFaint.Render("  SERVICES")
	var rows []string
	rows = append(rows, label)

	for i, name := range m.serviceOrder {
		svc := m.services[name]
		col := serviceColors[i%len(serviceColors)]
		dot := dotForStatus(svc.Status)
		dotCol := colorStrForStatus(svc.Status)

		dotStr := lipgloss.NewStyle().Foreground(lipgloss.Color(dotCol)).Render(dot)
		nameStr := lipgloss.NewStyle().Foreground(col).Bold(true).Render(truncate(name, width-6))

		portStr := ""
		if svc.Port > 0 {
			portStr = styleDim.Render(fmt.Sprintf(" :%d", svc.Port))
		}

		var line string
		if i == m.selectedSvc {
			accent := lipgloss.NewStyle().Foreground(col).Render("▌")
			bg := lipgloss.NewStyle().Background(lipgloss.Color("234"))
			line = bg.Width(width).Render(accent + " " + dotStr + " " + nameStr + portStr)
		} else {
			line = "   " + dotStr + " " + styleDim.Render(truncate(name, width-6)) + portStr
		}

		rows = append(rows, line)
	}

	if len(m.serviceOrder) == 0 {
		rows = append(rows, styleDim.Render("  no services"))
	}

	for len(rows) < height {
		rows = append(rows, "")
	}

	return strings.Join(rows[:min(len(rows), height)], "\n")
}

func (m model) renderLogs(width, height int) string {
	var filtered []LogLine
	sq := strings.ToLower(m.searchQuery)
	for _, l := range m.logs {
		if m.filter == "" || l.Service == m.filter {
			if sq == "" || strings.Contains(strings.ToLower(l.Text), sq) {
				filtered = append(filtered, l)
			}
		}
	}

	label := styleFaint.Render("  LOGS")
	if m.showAll {
		label = styleFaint.Render("  LOGS") + "  " + styleDim.Render("all")
	} else if m.filter != "" {
		label = styleFaint.Render("  LOGS") + "  " + styleDim.Render(m.filter)
	}
	if m.searchQuery != "" {
		label += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("72")).Render("/"+m.searchQuery)
	}

	headerRow := []string{label}
	visibleCount := height - 1

	total := len(filtered)
	end := total - m.logOffset
	if end < 0 {
		end = 0
	}
	start := end - visibleCount
	if start < 0 {
		start = 0
	}

	var rows []string
	for i := start; i < end; i++ {
		l := filtered[i]

		svcIdx := 0
		for j, name := range m.serviceOrder {
			if name == l.Service {
				svcIdx = j
				break
			}
		}
		col := serviceColors[svcIdx%len(serviceColors)]

		svcLabel := lipgloss.NewStyle().
			Foreground(col).
			Bold(true).
			Width(10).
			Render(truncate(l.Service, 10))

		tsWidth := 0
		tsStr := ""
		if m.showTimestamps {
			tsWidth = 10
			tsStr = styleTs.Render(l.Timestamp.Format("15:04:05"))
		}

		msgWidth := width - 10 - tsWidth - 4
		if msgWidth < 10 {
			msgWidth = 10
		}

		text := truncate(stripAnsi(l.Text), msgWidth)
		var msg string
		if l.Stream == "stderr" {
			msg = colorizeStderr(text)
		} else {
			msg = colorizeLog(text)
		}

		if m.showTimestamps {
			rows = append(rows, fmt.Sprintf("  %s  %s  %s", svcLabel, tsStr, msg))
		} else {
			rows = append(rows, fmt.Sprintf("  %s  %s", svcLabel, msg))
		}
	}

	if len(filtered) == 0 {
		rows = append(rows, styleDim.Render("  no logs yet"))
	}

	all := append(headerRow, rows...)
	for len(all) < height {
		all = append(all, "")
	}

	return strings.Join(all[:min(len(all), height)], "\n")
}

func (m model) renderFooter() string {
	if m.searchMode {
		prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("72")).Render("/")
		cursor := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("▌")
		hint := styleDim.Render("  enter: apply  esc: cancel")
		content := "  " + prompt + " " + m.searchQuery + cursor + hint
		return styleFooter.Width(m.width).Render(content)
	}

	if m.confirmRemove {
		warn := lipgloss.NewStyle().Foreground(lipgloss.Color("167")).Render("x again to confirm remove")
		esc := styleDim.Render("  esc: cancel")
		return styleFooter.Width(m.width).Render("  " + warn + esc)
	}

	if m.statusMsg != "" {
		msg := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(m.statusMsg)
		return styleFooter.Width(m.width).Render("  " + msg)
	}

	keys := []struct{ key, desc string }{
		{"tab", "switch"},
		{"↑↓", "navigate"},
		{"a", "all logs"},
		{"/", "search"},
		{"t", "timestamps"},
		{"o", "open"},
		{"s", "stop"},
		{"r", "restart"},
		{"x", "remove"},
		{"q", "quit"},
	}

	var parts []string
	for _, k := range keys {
		key := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(k.key)
		desc := styleDim.Render(k.desc)
		parts = append(parts, key+" "+desc)
	}

	content := "  " + strings.Join(parts, styleDivider.Render("  ·  "))
	return styleFooter.Width(m.width).Render(content)
}

func dotForStatus(s Status) string {
	switch s {
	case StatusRunning:
		return "●"
	case StatusDead:
		return "✗"
	default:
		return "○"
	}
}

func colorStrForStatus(s Status) string {
	switch s {
	case StatusRunning:
		return "72"
	case StatusDead:
		return "167"
	default:
		return "240"
	}
}

func colorizeLog(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic"):
		return styleLogErr.Render(text)
	case strings.Contains(lower, "warn"):
		return styleLogWarn.Render(text)
	case strings.Contains(lower, "started") || strings.Contains(lower, "listening") ||
		strings.Contains(lower, "ready") || strings.Contains(lower, "processed") ||
		strings.Contains(lower, "success"):
		return styleLogOk.Render(text)
	default:
		return styleLogDim.Render(text)
	}
}

// colorizeStderr handles stderr lines — softens stack traces so they don't dominate
func colorizeStderr(text string) string {
	trimmed := strings.TrimSpace(text)
	// stack frame lines — very dim
	if strings.HasPrefix(trimmed, "at ") ||
		strings.HasPrefix(trimmed, "... ") ||
		strings.HasPrefix(trimmed, "Caused by:") ||
		strings.HasPrefix(trimmed, "Suppressed:") {
		return styleLogTrace.Render(text)
	}
	// actual error/exception lines
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "exception") || strings.Contains(lower, "error") ||
		strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") {
		return styleLogErr.Render(text)
	}
	// warn
	if strings.Contains(lower, "warn") {
		return styleLogWarn.Render(text)
	}
	// anything else on stderr — slightly dim red
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(text)
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func stripAnsi(s string) string {
	var out []byte
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
		} else {
			out = append(out, s[i])
			i++
		}
	}
	return string(out)
}


func (m model) maxLogOffset() int {
	var filtered []LogLine
	sq := strings.ToLower(m.searchQuery)
	for _, l := range m.logs {
		if m.filter == "" || l.Service == m.filter {
			if sq == "" || strings.Contains(strings.ToLower(l.Text), sq) {
				filtered = append(filtered, l)
			}
		}
	}
	visibleCount := m.height - 3 // header + footer + label row
	max := len(filtered) - visibleCount
	if max < 0 {
		return 0
	}
	return max
}

func tuiRestartCmd(name string) tea.Cmd {
	return func() tea.Msg {
		self, err := os.Executable()
		if err != nil {
			return actionDoneMsg{name: name, action: "restart", err: err}
		}
		err = exec.Command(self, "restart", name).Run()
		if err != nil {
			return actionDoneMsg{name: name, action: "restart", err: err}
		}
		return actionDoneMsg{name: name, action: "restarted"}
	}
}

func tuiRemoveCmd(name string) tea.Cmd {
	return func() tea.Msg {
		self, err := os.Executable()
		if err != nil {
			return actionDoneMsg{name: name, action: "remove", err: err}
		}
		err = exec.Command(self, "rm", name).Run()
		if err != nil {
			return actionDoneMsg{name: name, action: "remove", err: err}
		}
		return actionDoneMsg{name: name, action: "removed"}
	}
}

func cmdTUI() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fatal(err)
	}
}
