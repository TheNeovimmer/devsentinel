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

const (
	ContentWidth = 80
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

	showHelp   bool
	firstVisit bool

	spinner   spinner.Model
	startedAt time.Time
	loadedAt  time.Time

	keys        keyBindings
	hasScanned  map[int]bool
	autoRefresh bool
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
			Foreground(colors.primary).
			Bold(true).
			Align(lipgloss.Center)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colors.textSubtle).
			Align(lipgloss.Center)

	boxStyle = lipgloss.NewStyle().
			Foreground(colors.surface)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colors.info).
			Background(colors.surface).
			Padding(0, 1)
)

func NewModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colors.primary)

	kb := keyBindings{
		Navigation: []key.Binding{
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
			key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧+tab", "prev")),
			key.NewBinding(key.WithKeys("1", "2", "3", "4", "5", "6"), key.WithHelp("1-6", "direct")),
			key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "home")),
			key.NewBinding(key.WithKeys("left", "right", "j", "k"), key.WithHelp("←/→", "nav")),
		},
		Actions: []key.Binding{
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "scan")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stream")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "auto")),
		},
		General: []key.Binding{
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			key.NewBinding(key.WithKeys("q", "ctrl+c", "esc"), key.WithHelp("q", "quit")),
		},
	}

	return Model{
		tabs: []Tab{
			{"Dashboard", "⌂", "home"},
			{"Architecture", "◈", "arch"},
			{"Runtime", "⚡", "rt"},
			{"Git", "⎔", "git"},
			{"Quality", "◉", "qual"},
			{"Logs", "⏺", "logs"},
		},
		ready:         false,
		showHelp:      false,
		firstVisit:    true,
		spinner:       s,
		startedAt:     time.Now(),
		loadedAt:      time.Now(),
		keys:          kb,
		hasScanned:    make(map[int]bool),
		autoRefresh:   false,
		analyzerModel: analyzer.NewModel(),
		monitorModel:  monitor.NewModel(),
		gitModel:      git.NewModel(),
		qualityModel:  quality.NewModel(),
		streamModel:   stream.NewModel(),
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

func (m *Model) scanCurrentTab() {
	if m.hasScanned[m.currentTab] {
		return
	}

	switch m.currentTab {
	case 1:
		m.analyzerModel.Analyze(".")
	case 2:
		m.monitorModel.Refresh()
	case 3:
		m.gitModel.Scan()
	case 4:
		m.qualityModel.Scan()
	}
	m.hasScanned[m.currentTab] = true
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.currentTab = (m.currentTab + 1) % len(m.tabs)
			m.scanCurrentTab()
		case "shift+tab":
			m.currentTab = (m.currentTab - 1 + len(m.tabs)) % len(m.tabs)
			m.scanCurrentTab()
		case "1", "2", "3", "4", "5", "6":
			m.currentTab = int(msg.String()[0] - '1')
			m.scanCurrentTab()
		case "?":
			m.showHelp = !m.showHelp
		case "g":
			m.currentTab = 0
		case "left", "k":
			m.currentTab = (m.currentTab - 1 + len(m.tabs)) % len(m.tabs)
			m.scanCurrentTab()
		case "right", "j":
			m.currentTab = (m.currentTab + 1) % len(m.tabs)
			m.scanCurrentTab()
		case "r":
			m.hasScanned = make(map[int]bool)
			m.scanCurrentTab()
		case "a":
			m.autoRefresh = !m.autoRefresh
		case "s":
			if m.currentTab == 5 {
				if m.streamModel.GetStreaming() {
					m.streamModel.StopStreaming()
				} else {
					m.streamModel.StartStreaming()
				}
			}
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.loadedAt = time.Now()

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
		return m.renderWelcome()
	}

	m.firstVisit = false

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

func (m Model) renderWelcome() string {
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
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("Project Intelligence Platform"))
	s.WriteString("\n\n")
	s.WriteString(lipgloss.NewStyle().Align(lipgloss.Center).Render(m.spinner.View() + "  Loading..."))

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center).
		Render(s.String())
}

func padCenter(text string, width int) string {
	textLen := len(text)
	if textLen >= width {
		return text[:width]
	}
	padding := (width - textLen) / 2
	return strings.Repeat(" ", padding) + text + strings.Repeat(" ", width-textLen-padding)
}

func (m Model) renderLayout(content string) string {
	header := m.renderHeader()
	footer := m.renderFooter()

	layout := header + "\n" + content + "\n" + footer

	if m.showHelp {
		layout += "\n" + m.renderHelp()
	}

	return layout
}

func (m Model) renderHeader() string {
	uptime := time.Since(m.startedAt).Round(time.Second)

	title := titleStyle.Render("DevSentinel")
	subtitle := subtitleStyle.Render(fmt.Sprintf("v1.0.0  •  %s", uptime))

	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)
	memMB := memStats.Alloc / 1024 / 1024

	autoStatus := subtitleStyle.Render("off")
	if m.autoRefresh {
		autoStatus = lipgloss.NewStyle().Foreground(colors.success).Bold(true).Render(" on")
	}

	cpuBar := m.renderMiniProgressBar(35, 8)

	headerContent := fmt.Sprintf("%s  %s  CPU %s  MEM %dMB  auto:%s\n", title, subtitle, cpuBar, memMB, autoStatus)
	headerContent += m.renderTabBar()

	return lipgloss.NewStyle().Width(ContentWidth).Render(headerContent)
}

func (m Model) renderTabBar() string {
	s := strings.Builder{}
	for i, tab := range m.tabs {
		if i == m.currentTab {
			s.WriteString(lipgloss.NewStyle().Foreground(colors.primary).Bold(true).Render("[" + tab.Name + "]"))
		} else {
			s.WriteString(lipgloss.NewStyle().Foreground(colors.textSubtle).Render(tab.Name))
		}
		if i < len(m.tabs)-1 {
			s.WriteString(" | ")
		}
	}
	s.WriteString("\n")
	return s.String()
}

func (m Model) renderMiniProgressBar(percent, width int) string {
	filled := percent * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	color := colors.success
	if percent > 70 {
		color = colors.warning
	}
	if percent > 90 {
		color = colors.error
	}
	return lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("[%s]%d%%", bar, percent))
}

func (m Model) renderSidebar() string {
	s := strings.Builder{}

	for i, tab := range m.tabs {
		indicator := " "
		if i == m.currentTab {
			indicator = "▶"
		}

		content := ""
		if i == m.currentTab {
			content = lipgloss.NewStyle().Foreground(colors.primary).Bold(true).Render(" " + indicator + " " + tab.Icon + " " + tab.Name)
		} else {
			content = lipgloss.NewStyle().Foreground(colors.textSubtle).Render(" " + indicator + " " + tab.Icon + " " + tab.Name)
		}

		s.WriteString(content)
		s.WriteString("\n")
	}

	s.WriteString("\n")

	hints := []string{"←/→ navigate", "r: scan", "?: help"}
	for _, hint := range hints {
		content := lipgloss.NewStyle().Foreground(colors.textSubtle).Render("  " + hint)
		s.WriteString(content)
		s.WriteString("\n")
	}

	return s.String()
}

func (m Model) renderMainContent(content string) string {
	return content
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

func (m Model) renderFooter() string {
	hint := "←/→:nav  r:scan  a:auto  ?:help  q:quit"
	return lipgloss.NewStyle().Width(ContentWidth).Render(hint)
}

func (m Model) renderDashboard() string {
	s := strings.Builder{}

	s.WriteString(titleStyle.Render("◈ Dashboard"))
	s.WriteString("\n\n")

	s.WriteString(m.renderQuickStats())
	s.WriteString("\n\n")

	s.WriteString(titleStyle.Render("Quick Actions"))
	s.WriteString("\n")
	s.WriteString(m.renderQuickActions())
	s.WriteString("\n")

	s.WriteString(titleStyle.Render("Getting Started"))
	s.WriteString("\n")
	s.WriteString(m.renderGettingStarted())

	return s.String()
}

func (m Model) renderQuickStats() string {
	stats := [][2]string{
		{"Status", "● Running"},
		{"Uptime", time.Since(m.startedAt).Round(time.Second).String()},
		{"Platform", runtime.GOOS + "/" + runtime.GOARCH},
		{"Go", runtime.Version()},
	}

	var rows []string
	for _, stat := range stats {
		label := lipgloss.NewStyle().Foreground(colors.textSubtle).Width(10).Render(stat[0])

		valueColor := colors.primary
		if stat[1] == "● Running" {
			valueColor = colors.success
		}

		value := lipgloss.NewStyle().Foreground(valueColor).Bold(true).Render(stat[1])

		row := label + "   " + value
		rows = append(rows, row)
	}

	return lipgloss.NewStyle().Width(ContentWidth).Align(lipgloss.Center).Render(strings.Join(rows, "    "))
}

func (m Model) renderQuickActions() string {
	actions := `
 [1] Architecture   - Code structure & dependencies
 [2] Runtime        - System metrics & processes  
 [3] Git            - Commits, branches & contributors
 [4] Quality         - Code quality & vulnerabilities
 [5] Logs           - Live application logs

 Press ←/→ or number to navigate`
	return lipgloss.NewStyle().Foreground(colors.textSubtle).Width(ContentWidth - 10).Render(actions)
}

func (m Model) renderGettingStarted() string {
	guide := `
 → [r] scan/analyze current view
 → [a] toggle auto-refresh
 → [?] keyboard shortcuts
 → [q] quit`
	return lipgloss.NewStyle().Foreground(colors.textSubtle).Width(ContentWidth - 10).Render(guide)
}

func (m Model) renderHelp() string {
	s := strings.Builder{}
	s.WriteString("\n")
	s.WriteString(titleStyle.Render("? Keyboard Shortcuts"))
	s.WriteString("\n\n")

	s.WriteString(strings.TrimSpace(`
 Navigation                    Actions                      General
─────────────────────────────────────────────────────────────────────────
 ←/→     Previous/Next tab     r  Scan/Analyze              ?  Toggle help  
 tab     Next tab             a  Toggle auto-refresh       q  Quit
 1-6     Direct navigation    s  Start/Stop stream (Logs)  
 g       Go to dashboard
`))

	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(" • Views auto-scan when first visited"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(" • Use 'a' for auto-refresh"))

	return lipgloss.NewStyle().Width(ContentWidth).Render(s.String())
}
