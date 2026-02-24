package monitor

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	SidebarWidth = 18
	ContentWidth = 58
)

type Process struct {
	PID    int
	Name   string
	CPU    float64
	Mem    float64
	Status string
}

type Port struct {
	Port    int
	Proto   string
	State   string
	Process string
}

type Container struct {
	Name   string
	Status string
	Image  string
	Ports  string
}

type SubView struct {
	Name        string
	Key         string
	Description string
}

type Model struct {
	processes  []Process
	ports      []Port
	containers []Container

	cpuPercent float64
	memPercent float64
	memUsed    uint64
	memTotal   uint64

	procTable table.Model
	portTable table.Model
	contTable table.Model

	subViews     []SubView
	selectedView int

	ready  bool
	width  int
	height int

	refreshing bool
	hasScanned bool
	lastUpdate time.Time
}

var (
	colors = struct {
		background  lipgloss.Color
		surface     lipgloss.Color
		primary     lipgloss.Color
		accent      lipgloss.Color
		success     lipgloss.Color
		warning     lipgloss.Color
		error       lipgloss.Color
		info        lipgloss.Color
		textPrimary lipgloss.Color
		textSubtle  lipgloss.Color
	}{
		background:  lipgloss.Color("232"),
		surface:     lipgloss.Color("235"),
		primary:     lipgloss.Color("45"),
		accent:      lipgloss.Color("219"),
		success:     lipgloss.Color("46"),
		warning:     lipgloss.Color("208"),
		error:       lipgloss.Color("196"),
		info:        lipgloss.Color("75"),
		textPrimary: lipgloss.Color("254"),
		textSubtle:  lipgloss.Color("244"),
	}

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.primary).
			Align(lipgloss.Center)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colors.textSubtle).
			Align(lipgloss.Center)

	infoStyle = lipgloss.NewStyle().
			Foreground(colors.info).
			Align(lipgloss.Center)

	successStyle = lipgloss.NewStyle().
			Foreground(colors.success)

	warningStyle = lipgloss.NewStyle().
			Foreground(colors.warning)

	boxStyle = lipgloss.NewStyle().
			Foreground(colors.surface)
)

func NewModel() Model {
	m := Model{
		subViews: []SubView{
			{"Processes", "1", "Running processes"},
			{"Ports", "2", "Open ports"},
			{"Containers", "3", "Docker"},
		},
		selectedView: 0,
	}

	procColumns := []table.Column{
		{Title: " PID ", Width: 8},
		{Title: " Name ", Width: 28},
		{Title: " CPU% ", Width: 8},
		{Title: " MEM% ", Width: 8},
		{Title: " Status ", Width: 12},
	}
	m.procTable = table.New(
		table.WithColumns(procColumns),
		table.WithFocused(true),
	)

	portColumns := []table.Column{
		{Title: " Port ", Width: 8},
		{Title: " Proto ", Width: 8},
		{Title: " State ", Width: 12},
		{Title: " Process ", Width: 20},
	}
	m.portTable = table.New(
		table.WithColumns(portColumns),
		table.WithFocused(true),
	)

	contColumns := []table.Column{
		{Title: " Name ", Width: 25},
		{Title: " Status ", Width: 15},
		{Title: " Image ", Width: 28},
	}
	m.contTable = table.New(
		table.WithColumns(contColumns),
		table.WithFocused(true),
	)

	return m
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

func (m *Model) Refresh() {
	m.refreshing = true
	m.collectSystemInfo()
	m.collectPorts()
	m.collectContainers()
	m.updateTables()
	m.lastUpdate = time.Now()
	m.refreshing = false
	m.hasScanned = true
}

func (m *Model) collectSystemInfo() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.memUsed = memStats.Alloc
	m.memTotal = memStats.Sys

	if m.memTotal > 0 {
		m.memPercent = float64(m.memUsed) / float64(m.memTotal) * 100
	}

	m.cpuPercent = getCPUUsage()
}

func getCPUUsage() float64 {
	cmd := exec.Command("sh", "-c", "top -bn1 | grep 'Cpu(s)' | awk '{print $2}' | cut -d'%' -f1")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	cpu, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return cpu
}

func (m *Model) collectPorts() {
	m.ports = []Port{}
	cmd := exec.Command("ss", "-tulpn")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 || len(line) == 0 {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		var port int
		fmt.Sscanf(parts[4], ":%d", &port)
		state := "LISTEN"
		if len(parts) > 4 {
			state = parts[1]
		}
		process := ""
		if len(parts) > 6 {
			process = parts[6]
		}
		m.ports = append(m.ports, Port{Port: port, Proto: parts[0], State: state, Process: process})
	}
}

func (m *Model) collectContainers() {
	m.containers = []Container{}
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 {
			m.containers = append(m.containers, Container{Name: parts[0], Status: parts[1], Image: parts[2]})
		}
	}
}

func (m *Model) updateTables() {
	cmd := exec.Command("ps", "aux")
	out, _ := cmd.Output()
	lines := strings.Split(string(out), "\n")

	m.processes = []Process{}
	for i, line := range lines {
		if i == 0 || len(line) == 0 {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 11 {
			continue
		}
		pid, _ := strconv.Atoi(parts[1])
		cpu, _ := strconv.ParseFloat(parts[2], 64)
		mem, _ := strconv.ParseFloat(parts[3], 64)
		m.processes = append(m.processes, Process{PID: pid, Name: parts[10], CPU: cpu, Mem: mem, Status: parts[7]})
		if len(m.processes) > 20 {
			break
		}
	}

	var procRows []table.Row
	for _, p := range m.processes {
		status := p.Status
		if p.CPU > 80 {
			status = warningStyle.Render(p.Status)
		}
		procRows = append(procRows, table.Row{
			fmt.Sprintf("%d", p.PID),
			truncate(p.Name, 26),
			fmt.Sprintf("%.1f", p.CPU),
			fmt.Sprintf("%.1f", p.Mem),
			status,
		})
	}
	m.procTable.SetRows(procRows)

	var portRows []table.Row
	for _, p := range m.ports[:20] {
		stateStr := p.State
		if p.State == "LISTEN" {
			stateStr = successStyle.Render(p.State)
		}
		portRows = append(portRows, table.Row{
			fmt.Sprintf("%d", p.Port),
			p.Proto,
			stateStr,
			truncate(p.Process, 18),
		})
	}
	m.portTable.SetRows(portRows)

	var contRows []table.Row
	for _, c := range m.containers {
		status := c.Status
		if !strings.Contains(c.Status, "Up") {
			status = warningStyle.Render(c.Status)
		}
		contRows = append(contRows, table.Row{c.Name, status, truncate(c.Image, 26)})
	}
	m.contTable.SetRows(contRows)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			m.selectedView = 0
		case "2":
			m.selectedView = 1
		case "3":
			m.selectedView = 2
		case "j", "down", "right":
			m.selectedView = (m.selectedView + 1) % len(m.subViews)
		case "k", "up", "left":
			m.selectedView = (m.selectedView - 1 + len(m.subViews)) % len(m.subViews)
		case "r", "ctrl+r":
			m.Refresh()
		}

		switch m.selectedView {
		case 0:
			m.procTable, cmd = m.procTable.Update(msg)
		case 1:
			m.portTable, cmd = m.portTable.Update(msg)
		case 2:
			m.contTable, cmd = m.contTable.Update(msg)
		}
	}

	return m, cmd
}

func (m Model) View() string {
	header := titleStyle.Render("⚡ Runtime")

	cpuBar := m.renderProgressBar(m.cpuPercent, 12)
	memBar := m.renderProgressBar(m.memPercent, 12)

	statsBar := ""
	if m.hasScanned {
		statsBar = subtitleStyle.Render(fmt.Sprintf("CPU %s  |  Memory %s  |  %s", cpuBar, memBar, m.lastUpdate.Format("15:04:05")))
	} else {
		statsBar = infoStyle.Render("Press [r] to fetch metrics")
	}

	var content string
	switch m.selectedView {
	case 0:
		content = m.procTable.View()
	case 1:
		content = m.portTable.View()
	case 2:
		content = m.contTable.View()
	}

	mainContent := fmt.Sprintf("%s\n%s\n%s\n%s", header, statsBar, content, subtitleStyle.Render("[1]/[2]/[3]: view  ↑/↓: select  r: refresh"))

	return mainContent
}

func (m Model) renderProgressBar(percent float64, width int) string {
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	color := colors.success
	if percent > 70 {
		color = colors.warning
	}
	if percent > 90 {
		color = colors.error
	}
	return lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("[%s]%.0f%%", bar, percent))
}

func (m Model) renderNav() string {
	s := strings.Builder{}

	for i, v := range m.subViews {
		indicator := " "
		if i == m.selectedView {
			indicator = "▶"
		}

		key := lipgloss.NewStyle().Foreground(colors.info).Render("[" + v.Key + "]")
		content := ""
		if i == m.selectedView {
			content = lipgloss.NewStyle().Foreground(colors.primary).Bold(true).Render(" " + indicator + " " + key + " " + v.Name)
		} else {
			content = lipgloss.NewStyle().Foreground(colors.textSubtle).Render(" " + indicator + " " + key + " " + v.Name)
		}

		s.WriteString(content)
		s.WriteString("\n")

		descContent := lipgloss.NewStyle().Foreground(colors.textSubtle).Render("   " + v.Description)
		s.WriteString(descContent)
		s.WriteString("\n")
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
