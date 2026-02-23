package stream

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Source    string
	Count     int
}

type ErrorPattern struct {
	Pattern    string
	Count      int
	LastSeen   time.Time
	Suggestion string
}

type Model struct {
	logs     []LogEntry
	patterns []ErrorPattern

	logTable     table.Model
	patternTable table.Model

	selectedView int
	ready        bool
	width        int
	height       int

	streaming bool
	stopChan  chan bool
}

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("76"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("75"))

	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	activeTabStyle = tabStyle.
			Foreground(lipgloss.Color("232")).
			Background(lipgloss.Color("86"))
)

func NewModel() Model {
	m := Model{
		stopChan: make(chan bool),
	}

	logColumns := []table.Column{
		{Title: "Time", Width: 10},
		{Title: "Level", Width: 8},
		{Title: "Message", Width: 60},
	}
	m.logTable = table.New(table.WithColumns(logColumns))

	patternColumns := []table.Column{
		{Title: "Pattern", Width: 40},
		{Title: "Count", Width: 8},
		{Title: "Suggestion", Width: 40},
	}
	m.patternTable = table.New(table.WithColumns(patternColumns))

	return m
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

func (m *Model) StartStreaming() {
	m.streaming = true
	go m.streamLogs()
}

func (m *Model) StopStreaming() {
	m.streaming = false
	if m.stopChan != nil {
		close(m.stopChan)
	}
	m.stopChan = make(chan bool)
}

func (m *Model) streamLogs() {
	logPatterns := []string{
		"*.log",
		"/var/log/syslog",
	}

	seenMessages := make(map[string]int)

	for {
		select {
		case <-m.stopChan:
			return
		default:
		}

		for _, pattern := range logPatterns {
			cmd := exec.Command("tail", "-n", "50", pattern)
			if pattern == "/var/log/syslog" {
				cmd = exec.Command("tail", "-n", "50", "/var/log/syslog")
			} else {
				cmd = exec.Command("tail", "-n", "50", "-f", pattern)
			}

			out, err := cmd.Output()
			if err != nil {
				continue
			}

			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if len(line) == 0 {
					continue
				}

				entry := parseLogLine(line)
				if entry.Level == "" {
					continue
				}

				m.logs = append(m.logs, entry)

				hash := fmt.Sprintf("%s:%s", entry.Level, entry.Message)
				seenMessages[hash]++

				if entry.Level == "ERROR" || entry.Level == "WARN" {
					m.detectPattern(entry.Message)
				}
			}
		}

		if len(m.logs) > 500 {
			m.logs = m.logs[len(m.logs)-500:]
		}

		m.updateTables()

		time.Sleep(2 * time.Second)
	}
}

func parseLogLine(line string) LogEntry {
	re := regexp.MustCompile(`^(\w{3}\s+\d+\s+[\d:]+|[\d-]+\s+[\d:]+)\s+(\w+)\s+(.+)$`)
	matches := re.FindStringSubmatch(line)

	if matches == nil {
		if strings.Contains(line, "ERROR") || strings.Contains(line, "error") {
			return LogEntry{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Message:   truncate(line, 80),
			}
		}
		return LogEntry{}
	}

	level := "INFO"
	if strings.Contains(matches[3], "ERROR") || strings.Contains(matches[2], "error") {
		level = "ERROR"
	} else if strings.Contains(matches[3], "WARN") || strings.Contains(matches[2], "warn") {
		level = "WARN"
	}

	return LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   truncate(matches[3], 80),
		Source:    "log",
	}
}

func (m *Model) detectPattern(message string) {
	for i, p := range m.patterns {
		if strings.Contains(message, p.Pattern) {
			m.patterns[i].Count++
			m.patterns[i].LastSeen = time.Now()
			return
		}
	}

	suggestion := getSuggestion(message)

	m.patterns = append(m.patterns, ErrorPattern{
		Pattern:    truncate(message, 38),
		Count:      1,
		LastSeen:   time.Now(),
		Suggestion: suggestion,
	})

	if len(m.patterns) > 20 {
		m.patterns = m.patterns[:20]
	}
}

func getSuggestion(message string) string {
	lower := strings.ToLower(message)

	if strings.Contains(lower, "null") || strings.Contains(lower, "nil") {
		return "Check for uninitialized variables"
	}
	if strings.Contains(lower, "connection refused") {
		return "Verify service is running and port is open"
	}
	if strings.Contains(lower, "timeout") {
		return "Increase timeout or check network"
	}
	if strings.Contains(lower, "permission denied") {
		return "Check file/directory permissions"
	}
	if strings.Contains(lower, "out of memory") {
		return "Increase memory or optimize usage"
	}
	if strings.Contains(lower, "panic") {
		return "Check stack trace for root cause"
	}
	if strings.Contains(lower, "deadlock") {
		return "Review goroutine synchronization"
	}

	return "Investigate error context"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

func (m *Model) updateTables() {
	var logRows []table.Row
	start := 0
	if len(m.logs) > 50 {
		start = len(m.logs) - 50
	}

	for _, log := range m.logs[start:] {
		level := log.Level
		if log.Level == "ERROR" {
			level = errorStyle.Render(level)
		} else if log.Level == "WARN" {
			level = warningStyle.Render(level)
		}

		logRows = append(logRows, table.Row{
			log.Timestamp.Format("15:04:05"),
			level,
			log.Message,
		})
	}
	m.logTable.SetRows(logRows)

	var patternRows []table.Row
	for _, p := range m.patterns {
		count := fmt.Sprintf("%d", p.Count)
		if p.Count > 10 {
			count = errorStyle.Render(count)
		} else if p.Count > 5 {
			count = warningStyle.Render(count)
		}

		patternRows = append(patternRows, table.Row{
			p.Pattern,
			count,
			p.Suggestion,
		})
	}
	m.patternTable.SetRows(patternRows)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.selectedView = (m.selectedView + 1) % 2
		case "k", "up":
			m.selectedView = (m.selectedView - 1 + 2) % 2
		case "s":
			if m.streaming {
				m.StopStreaming()
			} else {
				m.StartStreaming()
			}
		case "c":
			m.logs = []LogEntry{}
			m.patterns = []ErrorPattern{}
			m.updateTables()
		}
	}

	return m, nil
}

func (m Model) View() string {
	header := headerStyle.Render("Real-Time Error Stream")

	streamStatus := infoStyle.Render("Stopped")
	if m.streaming {
		streamStatus = successStyle.Render("Streaming")
	}

	errorCount := 0
	warnCount := 0
	for _, log := range m.logs {
		if log.Level == "ERROR" {
			errorCount++
		} else if log.Level == "WARN" {
			warnCount++
		}
	}

	stats := fmt.Sprintf("Status: %s | Errors: %d | Warnings: %d | Patterns: %d",
		streamStatus, errorCount, warnCount, len(m.patterns))

	views := []string{"Live Logs", "Error Patterns"}
	viewIndicator := ""
	for i, v := range views {
		if i == m.selectedView {
			viewIndicator += activeTabStyle.Render(v) + " "
		} else {
			viewIndicator += tabStyle.Render(v) + " "
		}
	}

	var content string
	switch m.selectedView {
	case 0:
		content = m.logTable.View()
	case 1:
		content = m.patternTable.View()
	}

	hint := "\n" + infoStyle.Render("s: Start/Stop | c: Clear | Errors grouped by pattern")

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s\n", header, stats, viewIndicator, content, hint)
}
