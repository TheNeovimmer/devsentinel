package stream

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	SidebarWidth = 18
	ContentWidth = 58
)

type SubView struct {
	Name        string
	Key         string
	Description string
}

type Model struct {
	logs         []string
	selectedView int
	ready        bool
	width        int
	height       int
	streaming    bool
	stopChan     chan bool
	subViews     []SubView
}

var (
	colors = struct {
		background lipgloss.Color
		surface    lipgloss.Color
		primary    lipgloss.Color
		success    lipgloss.Color
		warning    lipgloss.Color
		error      lipgloss.Color
		info       lipgloss.Color
		textSubtle lipgloss.Color
	}{
		background: lipgloss.Color("232"),
		surface:    lipgloss.Color("235"),
		primary:    lipgloss.Color("45"),
		success:    lipgloss.Color("46"),
		warning:    lipgloss.Color("208"),
		error:      lipgloss.Color("196"),
		info:       lipgloss.Color("75"),
		textSubtle: lipgloss.Color("244"),
	}

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colors.primary).Align(lipgloss.Center)
	subtitleStyle = lipgloss.NewStyle().Foreground(colors.textSubtle).Align(lipgloss.Center)
	infoStyle     = lipgloss.NewStyle().Foreground(colors.info).Align(lipgloss.Center)
	successStyle  = lipgloss.NewStyle().Foreground(colors.success)
	warningStyle  = lipgloss.NewStyle().Foreground(colors.warning)
	errorStyle    = lipgloss.NewStyle().Foreground(colors.error)
	boxStyle      = lipgloss.NewStyle().Foreground(colors.surface)
)

func NewModel() Model {
	m := Model{
		subViews: []SubView{
			{"Live Logs", "1", "Stream"},
			{"Patterns", "2", "Errors"},
		},
		selectedView: 0,
		stopChan:     make(chan bool),
		logs:         []string{},
	}
	return m
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

func (m *Model) StartStreaming() {
	m.streaming = true
	go func() {
		for {
			select {
			case <-m.stopChan:
				return
			default:
				m.logs = append(m.logs, fmt.Sprintf("%s  %s  Log stream connected", time.Now().Format("15:04:05"), successStyle.Render("INFO")))
				if len(m.logs) > 20 {
					m.logs = m.logs[len(m.logs)-20:]
				}
				time.Sleep(2 * time.Second)
			}
		}
	}()
}

func (m *Model) StopStreaming() {
	m.streaming = false
	if m.stopChan != nil {
		close(m.stopChan)
	}
	m.stopChan = make(chan bool)
}

func (m *Model) GetStreaming() bool {
	return m.streaming
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch msg.String() {
		case "1", "2":
			m.selectedView = int(msg.String()[0] - '1')
		case "j", "right", "down":
			m.selectedView = (m.selectedView + 1) % 2
		case "k", "left", "up":
			m.selectedView = (m.selectedView + 1) % 2
		case "s":
			if m.streaming {
				m.StopStreaming()
			} else {
				m.StartStreaming()
			}
		case "c":
			m.logs = []string{}
		}
	}
	return m, nil
}

func (m Model) View() string {
	header := titleStyle.Render("⏺ Live Logs")
	status := infoStyle.Render("Stopped")
	if m.streaming {
		status = successStyle.Render("Streaming")
	}
	stats := subtitleStyle.Render(fmt.Sprintf("Status: %s", status))

	var content string
	if len(m.logs) == 0 {
		content = infoStyle.Render("No logs yet. Press [s] to start streaming.")
	} else {
		content = strings.Join(m.logs, "\n")
	}

	mainContent := fmt.Sprintf("%s\n%s\n%s\n%s", header, stats, content, subtitleStyle.Render("[1]/[2]: view  s: start/stop  c: clear"))
	return mainContent
}

func (m Model) renderNav() string {
	s := strings.Builder{}
	for i, v := range m.subViews {
		indicator := " "
		if i == m.selectedView {
			indicator = "▶"
		}
		key := lipgloss.NewStyle().Foreground(colors.info).Render("[" + v.Key + "]")
		var content string
		if i == m.selectedView {
			content = lipgloss.NewStyle().Foreground(colors.primary).Bold(true).Render(" " + indicator + " " + key + " " + v.Name)
		} else {
			content = lipgloss.NewStyle().Foreground(colors.textSubtle).Render(" " + indicator + " " + key + " " + v.Name)
		}
		s.WriteString(content + "\n")
		descContent := lipgloss.NewStyle().Foreground(colors.textSubtle).Render("   " + v.Description)
		s.WriteString(descContent + "\n")
	}
	return s.String()
}

func stripAnsi(s string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		result += string(s[i])
	}
	return result
}
