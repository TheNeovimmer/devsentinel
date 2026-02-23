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

	selectedView int
	ready        bool
	width        int
	height       int

	refreshing bool
	lastUpdate time.Time
}

var (
	colors = struct {
		accent  lipgloss.Color
		success lipgloss.Color
		warning lipgloss.Color
		error   lipgloss.Color
		info    lipgloss.Color
		subtle  lipgloss.Color
		surface lipgloss.Color
	}{
		accent:  lipgloss.Color("86"),
		success: lipgloss.Color("76"),
		warning: lipgloss.Color("226"),
		error:   lipgloss.Color("196"),
		info:    lipgloss.Color("75"),
		subtle:  lipgloss.Color("241"),
		surface: lipgloss.Color("236"),
	}

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.accent)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(colors.accent).
			Bold(true).
			Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(colors.subtle).
				Padding(0, 1)
)

func NewModel() Model {
	m := Model{}

	procColumns := []table.Column{
		{Title: " PID ", Width: 8},
		{Title: " Name ", Width: 28},
		{Title: " CPU% ", Width: 8},
		{Title: " MEM% ", Width: 8},
		{Title: " Status ", Width: 12},
	}
	m.procTable = table.New(table.WithColumns(procColumns))

	portColumns := []table.Column{
		{Title: " Port ", Width: 8},
		{Title: " Proto ", Width: 8},
		{Title: " State ", Width: 12},
		{Title: " Process ", Width: 20},
	}
	m.portTable = table.New(table.WithColumns(portColumns))

	contColumns := []table.Column{
		{Title: " Name ", Width: 25},
		{Title: " Status ", Width: 15},
		{Title: " Image ", Width: 28},
		{Title: " Ports ", Width: 15},
	}
	m.contTable = table.New(table.WithColumns(contColumns))

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

func (m *Model) collectProcesses() {
	m.processes = []Process{}

	cmd := exec.Command("ps", "aux")
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
		if len(parts) < 11 {
			continue
		}

		pid, _ := strconv.Atoi(parts[1])
		cpu, _ := strconv.ParseFloat(parts[2], 64)
		mem, _ := strconv.ParseFloat(parts[3], 64)

		m.processes = append(m.processes, Process{
			PID:    pid,
			Name:   parts[10],
			CPU:    cpu,
			Mem:    mem,
			Status: parts[7],
		})

		if len(m.processes) > 50 {
			break
		}
	}
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

		m.ports = append(m.ports, Port{
			Port:    port,
			Proto:   parts[0],
			State:   state,
			Process: process,
		})
	}
}

func (m *Model) collectContainers() {
	m.containers = []Container{}

	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}\t{{.Ports}}")
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
		if len(parts) < 3 {
			continue
		}

		ports := ""
		if len(parts) > 3 {
			ports = parts[3]
		}

		m.containers = append(m.containers, Container{
			Name:   parts[0],
			Status: parts[1],
			Image:  parts[2],
			Ports:  ports,
		})
	}
}

func (m *Model) updateTables() {
	m.collectProcesses()

	var procRows []table.Row
	for _, p := range m.processes[:minInt(20, len(m.processes))] {
		status := p.Status
		if p.CPU > 80 {
			status = lipgloss.NewStyle().Foreground(colors.warning).Render(p.Status)
		} else if p.Mem > 80 {
			status = lipgloss.NewStyle().Foreground(colors.error).Render(p.Status)
		}

		cpuStr := fmt.Sprintf("%.1f", p.CPU)
		if p.CPU > 80 {
			cpuStr = lipgloss.NewStyle().Foreground(colors.warning).Render(cpuStr)
		}

		memStr := fmt.Sprintf("%.1f", p.Mem)
		if p.Mem > 80 {
			memStr = lipgloss.NewStyle().Foreground(colors.error).Render(memStr)
		}

		procRows = append(procRows, table.Row{
			fmt.Sprintf("%d", p.PID),
			truncate(p.Name, 26),
			cpuStr,
			memStr,
			status,
		})
	}
	m.procTable.SetRows(procRows)

	var portRows []table.Row
	for _, p := range m.ports[:minInt(20, len(m.ports))] {
		stateStr := p.State
		if p.State == "LISTEN" {
			stateStr = lipgloss.NewStyle().Foreground(colors.success).Render(p.State)
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
			status = lipgloss.NewStyle().Foreground(colors.warning).Render(c.Status)
		}

		contRows = append(contRows, table.Row{
			c.Name,
			status,
			truncate(c.Image, 26),
			truncate(c.Ports, 13),
		})
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.selectedView = (m.selectedView + 1) % 3
		case "k", "up":
			m.selectedView = (m.selectedView - 1 + 3) % 3
		case "r", "ctrl+r":
			m.Refresh()
		}
	}

	return m, nil
}

func (m Model) View() string {
	header := headerStyle.Render("⚡ Runtime Monitor")

	cpuStr := fmt.Sprintf("%.1f%%", m.cpuPercent)
	memStr := fmt.Sprintf("%.1f%% (%.1fMB / %.1fMB)", m.memPercent,
		float64(m.memUsed)/1024/1024, float64(m.memTotal)/1024/1024)

	cpuColor := colors.success
	if m.cpuPercent > 70 {
		cpuColor = colors.warning
	}
	if m.cpuPercent > 90 {
		cpuColor = colors.error
	}

	memColor := colors.success
	if m.memPercent > 70 {
		memColor = colors.warning
	}
	if m.memPercent > 90 {
		memColor = colors.error
	}

	statsBar := fmt.Sprintf("CPU: %s  |  Memory: %s  |  Last: %s",
		lipgloss.NewStyle().Foreground(cpuColor).Render(cpuStr),
		lipgloss.NewStyle().Foreground(memColor).Render(memStr),
		m.lastUpdate.Format("15:04:05"))

	views := []string{"Processes", "Ports", "Containers"}
	viewIndicator := ""
	for i, v := range views {
		if i == m.selectedView {
			viewIndicator += tabActiveStyle.Render(" "+v+" ") + " "
		} else {
			viewIndicator += tabInactiveStyle.Render(" "+v+" ") + " "
		}
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

	hint := lipgloss.NewStyle().Foreground(colors.info).Render("\nPress 'r' to refresh | j/k to switch views")

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s\n", header, statsBar, viewIndicator, content, hint)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
