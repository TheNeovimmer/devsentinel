package ui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"devsentinel/cmd/internal/analyzer"
	"devsentinel/cmd/internal/git"
	"devsentinel/cmd/internal/monitor"
	"devsentinel/cmd/internal/quality"
	"devsentinel/cmd/internal/stream"
)

type Model struct {
	currentTab int
	tabs       []Tab
	ready      bool
	width      int
	height     int

	analyzerModel analyzer.Model
	monitorModel  monitor.Model
	gitModel      git.Model
	qualityModel  quality.Model
	streamModel   stream.Model

	showHelp       bool
	showCmdPalette bool

	spinner spinner.Model

	startedAt    time.Time
	lastActivity time.Time

	keys keyBindings
}

type Tab struct {
	Name  string
	Icon  string
	Short string
}

type keyBindings struct {
	Navigation []key.Binding
	Actions    []key.Binding
	General    []key.Binding
}

var (
	colors = struct {
		primary    lipgloss.Color
		secondary  lipgloss.Color
		accent     lipgloss.Color
		success    lipgloss.Color
		warning    lipgloss.Color
		error      lipgloss.Color
		info       lipgloss.Color
		subtle     lipgloss.Color
		background lipgloss.Color
		surface    lipgloss.Color
		border     lipgloss.Color
	}{
		primary:    lipgloss.Color("99"),
		secondary:  lipgloss.Color("141"),
		accent:     lipgloss.Color("86"),
		success:    lipgloss.Color("76"),
		warning:    lipgloss.Color("226"),
		error:      lipgloss.Color("196"),
		info:       lipgloss.Color("75"),
		subtle:     lipgloss.Color("241"),
		background: lipgloss.Color("235"),
		surface:    lipgloss.Color("236"),
		border:     lipgloss.Color("238"),
	}

	titleStyle = lipgloss.NewStyle().
			Foreground(colors.accent).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colors.subtle)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(colors.accent).
			Background(colors.surface).
			Bold(true).
			Padding(0, 2)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(colors.subtle).
				Padding(0, 2)

	panelStyle = lipgloss.NewStyle().
			Background(colors.surface).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colors.border).
			Padding(1, 2)

	statLabelStyle = lipgloss.NewStyle().
			Foreground(colors.subtle).
			Width(14)

	statValueStyle = lipgloss.NewStyle().
			Foreground(colors.accent).
			Bold(true)

	keyHintStyle = lipgloss.NewStyle().
			Foreground(colors.subtle)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colors.info).
			Background(colors.surface).
			Padding(0, 1)
)

func NewModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colors.accent)

	kb := keyBindings{
		Navigation: []key.Binding{
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
			key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧+tab", "prev")),
			key.NewBinding(key.WithKeys("1", "2", "3", "4", "5", "6"), key.WithHelp("1-6", "direct")),
			key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "home")),
			key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓", "down")),
			key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑", "up")),
		},
		Actions: []key.Binding{
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stream")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear")),
		},
		General: []key.Binding{
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "keys")),
			key.NewBinding(key.WithKeys("q", "ctrl+c", "esc"), key.WithHelp("q", "quit")),
		},
	}

	return Model{
		tabs: []Tab{
			{"Dashboard", "⌂", "home"},
			{"Architecture", "◈", "arch"},
			{"Runtime", "⚡", "rt"},
			{"Git", "⎔", "git"},
			{"Quality", "◉", "quality"},
			{"Logs", "⏺", "logs"},
		},
		ready:          false,
		showHelp:       false,
		showCmdPalette: false,
		spinner:        s,
		startedAt:      time.Now(),
		lastActivity:   time.Now(),
		keys:           kb,
		analyzerModel:  analyzer.NewModel(),
		monitorModel:   monitor.NewModel(),
		gitModel:       git.NewModel(),
		qualityModel:   quality.NewModel(),
		streamModel:    stream.NewModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

func (m *Model) updateActivity() {
	m.lastActivity = time.Now()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.updateActivity()

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.currentTab = (m.currentTab + 1) % len(m.tabs)
		case "shift+tab":
			m.currentTab = (m.currentTab - 1 + len(m.tabs)) % len(m.tabs)
		case "1", "2", "3", "4", "5", "6":
			m.currentTab = int(msg.String()[0] - '1')
		case "?":
			m.showHelp = !m.showHelp
		case "h":
			m.showCmdPalette = !m.showCmdPalette
		case "g":
			m.currentTab = 0
		case "j", "down":
			m.currentTab = minInt(m.currentTab+1, len(m.tabs)-1)
		case "k", "up":
			m.currentTab = maxInt(m.currentTab-1, 0)
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch m.currentTab {
	case 0:
		m2, cmd := m.analyzerModel.Update(msg)
		m.analyzerModel = m2
		cmds = append(cmds, cmd)
	case 1:
		m2, cmd := m.analyzerModel.Update(msg)
		m.analyzerModel = m2
		cmds = append(cmds, cmd)
	case 2:
		m2, cmd := m.monitorModel.Update(msg)
		m.monitorModel = m2
		cmds = append(cmds, cmd)
	case 3:
		m2, cmd := m.gitModel.Update(msg)
		m.gitModel = m2
		cmds = append(cmds, cmd)
	case 4:
		m2, cmd := m.qualityModel.Update(msg)
		m.qualityModel = m2
		cmds = append(cmds, cmd)
	case 5:
		m2, cmd := m.streamModel.Update(msg)
		m.streamModel = m2
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return m.renderSplash()
	}

	var content string

	switch m.currentTab {
	case 0:
		content = m.renderDashboard()
	case 1:
		content = m.analyzerModel.View()
	case 2:
		content = m.monitorModel.View()
	case 3:
		content = m.gitModel.View()
	case 4:
		content = m.qualityModel.View()
	case 5:
		content = m.streamModel.View()
	}

	return m.renderLayout(content)
}

func (m Model) renderSplash() string {
	logo := `
    ██████╗ ███████╗████████╗██████╗  ██████╗ 
    ██╔══██╗██╔════╝╚══██╔══╝██╔══██╗██╔═══██╗
    ██████╔╝█████╗     ██║   ██████╔╝██║   ██║
    ██╔══██╗██╔══╝     ██║   ██╔══██╗██║   ██║
    ██║  ██║███████╗   ██║   ██║  ██║╚██████╔╝
    ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ 
                                            
    ███████╗██╗ ██████╗ ███╗   ██╗ █████╗ ██╗     
    ██╔════╝██║██╔════╝ ████╗  ██║██╔══██╗██║     
    ███████╗██║██║  ███╗██╔██╗ ██║███████║██║     
    ╚════██║██║██║   ██║██║╚██╗██║██╔══██║██║     
    ███████║██║╚██████╔╝██║ ╚████║██║  ██║███████╗
    ╚══════╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝`

	s := strings.Builder{}
	s.WriteString(titleStyle.Render(logo))
	s.WriteString("\n\n")
	s.WriteString(subtitleStyle.Align(lipgloss.Center).Render("Project Intelligence Platform"))
	s.WriteString("\n\n")
	s.WriteString(m.spinner.View())
	s.WriteString(" ")
	s.WriteString(subtitleStyle.Render("Initializing..."))

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(s.String())
}

func (m Model) renderLayout(content string) string {
	header := m.renderHeader()
	nav := m.renderNav()
	main := content
	footer := m.renderFooter()

	layout := strings.Builder{}
	layout.WriteString(header)
	layout.WriteString("\n")
	layout.WriteString(nav)
	layout.WriteString("\n")
	layout.WriteString(main)
	layout.WriteString("\n")
	layout.WriteString(footer)

	if m.showHelp {
		layout.WriteString("\n")
		layout.WriteString(m.renderHelp())
	}

	if m.showCmdPalette {
		layout.WriteString("\n")
		layout.WriteString(m.renderCmdPalette())
	}

	return layout.String()
}

func (m Model) renderHeader() string {
	uptime := time.Since(m.startedAt).Round(time.Second)

	title := titleStyle.Render("DevSentinel")
	subtitle := subtitleStyle.Render(fmt.Sprintf("v1.0.0 • %s", uptime))

	left := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)

	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)
	memMB := memStats.Alloc / 1024 / 1024

	right := subtitleStyle.Render(
		fmt.Sprintf("mem: %dMB | goroutines: %d", memMB, runtime.NumGoroutine()),
	)

	headerWidth := m.width - 4

	return lipgloss.NewStyle().
		Width(headerWidth).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, left, right))
}

func (m Model) renderNav() string {
	var tabs []string

	maxWidth := m.width - 20
	currentWidth := 0

	for i, tab := range m.tabs {
		tabStr := tab.Icon + " " + tab.Name
		if i == m.currentTab {
			currentWidth += len(tabStr) + 3
			tabs = append(tabs, tabActiveStyle.Render(" "+tabStr+" "))
		} else {
			currentWidth += len(tabStr) + 3
			tabs = append(tabs, tabInactiveStyle.Render(" "+tabStr+" "))
		}

		if currentWidth > maxWidth {
			break
		}
	}

	nav := lipgloss.NewStyle().
		Width(m.width - 4).
		Render(lipgloss.JoinHorizontal(0, tabs...))

	currentTabInfo := subtitleStyle.Render(
		fmt.Sprintf("│ %s ", m.tabs[m.currentTab].Name),
	)

	return nav + currentTabInfo
}

func (m Model) renderFooter() string {
	keys := []string{
		helpKeyStyle.Render("tab") + " next",
		helpKeyStyle.Render("g") + " home",
		helpKeyStyle.Render("?") + " help",
		helpKeyStyle.Render("q") + " quit",
	}

	hint := strings.Join(keys, "  ")

	page := subtitleStyle.Render(
		fmt.Sprintf("%d / %d", m.currentTab+1, len(m.tabs)),
	)

	footerWidth := m.width - 4

	return lipgloss.NewStyle().
		Width(footerWidth).
		Render(hint + strings.Repeat(" ", 10) + page)
}

func (m Model) renderDashboard() string {
	s := strings.Builder{}

	s.WriteString(titleStyle.Render("◈ DevSentinel Dashboard"))
	s.WriteString("\n\n")

	s.WriteString(m.renderStatRow([][2]string{
		{"Status", "● Running"},
		{"Uptime", time.Since(m.startedAt).Round(time.Second).String()},
		{"Platform", runtime.GOOS + "/" + runtime.GOARCH},
	}))
	s.WriteString("\n")

	s.WriteString(m.renderStatRow([][2]string{
		{"Modules", "5 Active"},
		{"Views", fmt.Sprintf("%d Tabs", len(m.tabs))},
		{"Go Version", runtime.Version()},
	}))
	s.WriteString("\n\n")

	s.WriteString(titleStyle.Render("Quick Actions"))
	s.WriteString("\n\n")

	actions := panelStyle.Width(m.width - 16).Render(
		" [1] Architecture    [2] Runtime    [3] Git\n\n" +
			" [4] Quality         [5] Logs       [6] Settings",
	)
	s.WriteString(actions)
	s.WriteString("\n\n")

	s.WriteString(titleStyle.Render("Navigation"))
	s.WriteString("\n\n")

	navHelp := panelStyle.Width(m.width - 16).Render(
		" Tab / ⇧+Tab : Next/Prev    1-6 : Direct    g : Home\n" +
			" ↑/↓        : Navigate       ?  : Help       q : Quit",
	)
	s.WriteString(navHelp)

	return panelStyle.Width(m.width - 8).Render(s.String())
}

func (m Model) renderStatRow(stats [][2]string) string {
	var row []string

	for _, stat := range stats {
		label := statLabelStyle.Render(stat[0])

		valueColor := colors.accent
		if stat[1] == "● Running" {
			valueColor = colors.success
		}

		value := lipgloss.NewStyle().Foreground(valueColor).Bold(true).Render(stat[1])

		row = append(row, lipgloss.JoinHorizontal(0, label, value))
	}

	return strings.Join(row, "    ")
}

func (m Model) renderHelp() string {
	s := strings.Builder{}

	s.WriteString("\n")
	s.WriteString(titleStyle.Render("? Keyboard Shortcuts"))
	s.WriteString("\n\n")

	s.WriteString(panelStyle.Width(m.width - 12).Render(
		"Navigation                    Actions                      General\n" +
			"───────────────────────────────────────────────────────────────────────────\n" +
			" tab      Next tab            r  Refresh/Scan             ?  Toggle help\n" +
			" ⇧+tab    Previous tab        s  Start/Stop stream        h  Show keys\n" +
			" 1-6      Direct navigation   c  Clear data              q  Quit\n" +
			" g        Go to dashboard     ←/→ Navigate lists\n" +
			" ↑/↓      Navigate            Enter Select",
	))

	return s.String()
}

func (m Model) renderCmdPalette() string {
	s := strings.Builder{}

	s.WriteString("\n")
	s.WriteString(titleStyle.Render("h Command Palette"))
	s.WriteString("\n\n")

	var allKeys []string
	allKeys = append(allKeys, "tab: next", "⇧+tab: prev", "1-6: direct", "g: home")
	allKeys = append(allKeys, "r: refresh", "s: stream", "c: clear")
	allKeys = append(allKeys, "?: help", "h: keys", "q: quit")

	palette := panelStyle.Width(m.width - 12).Render(strings.Join(allKeys, "  |  "))
	s.WriteString(palette)

	return s.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
