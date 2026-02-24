package git

import (
	"fmt"
	"os/exec"
	"sort"
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

type Commit struct {
	Hash    string
	Date    time.Time
	Author  string
	Message string
	Files   int
}

type FileChurn struct {
	Path       string
	ChurnScore int
}

type Branch struct {
	Name       string
	IsCurrent  bool
	LastCommit string
}

type SubView struct {
	Name        string
	Key         string
	Description string
}

type Model struct {
	commits      []Commit
	fileChurn    []FileChurn
	branches     []Branch
	commitTable  table.Model
	churnTable   table.Model
	branchTable  table.Model
	subViews     []SubView
	selectedView int
	ready        bool
	width        int
	height       int
	scanning     bool
	hasScanned   bool
	lastScan     time.Time
}

var (
	colors = struct {
		background lipgloss.Color
		surface    lipgloss.Color
		primary    lipgloss.Color
		success    lipgloss.Color
		warning    lipgloss.Color
		info       lipgloss.Color
		textSubtle lipgloss.Color
	}{
		background: lipgloss.Color("232"),
		surface:    lipgloss.Color("235"),
		primary:    lipgloss.Color("45"),
		success:    lipgloss.Color("46"),
		warning:    lipgloss.Color("208"),
		info:       lipgloss.Color("75"),
		textSubtle: lipgloss.Color("244"),
	}

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colors.primary).Align(lipgloss.Center)
	subtitleStyle = lipgloss.NewStyle().Foreground(colors.textSubtle).Align(lipgloss.Center)
	infoStyle     = lipgloss.NewStyle().Foreground(colors.info).Align(lipgloss.Center)
	successStyle  = lipgloss.NewStyle().Foreground(colors.success)
	warningStyle  = lipgloss.NewStyle().Foreground(colors.warning)
	boxStyle      = lipgloss.NewStyle().Foreground(colors.surface)
)

func NewModel() Model {
	m := Model{
		subViews: []SubView{
			{"Commits", "1", "History"},
			{"Files", "2", "Churn"},
			{"Branches", "3", "Branches"},
		},
		selectedView: 0,
	}
	m.commitTable = table.New(
		table.WithColumns([]table.Column{
			{Title: " Hash ", Width: 8}, {Title: " Date ", Width: 12}, {Title: " Author ", Width: 14}, {Title: " Message ", Width: 30},
		}),
		table.WithFocused(true),
	)
	m.churnTable = table.New(
		table.WithColumns([]table.Column{
			{Title: " File ", Width: 40}, {Title: " Churn ", Width: 10},
		}),
		table.WithFocused(true),
	)
	m.branchTable = table.New(
		table.WithColumns([]table.Column{
			{Title: " Branch ", Width: 25}, {Title: " Current ", Width: 8}, {Title: " Last ", Width: 20},
		}),
		table.WithFocused(true),
	)
	return m
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

func (m *Model) Scan() {
	m.scanning = true
	m.collectCommits()
	m.collectFileChurn()
	m.collectBranches()
	m.updateTables()
	m.lastScan = time.Now()
	m.scanning = false
	m.hasScanned = true
}

func (m *Model) collectCommits() {
	m.commits = nil
	cmd := exec.Command("git", "log", "--format=%H|%ai|%an|%s", "-n", "30")
	out, _ := cmd.Output()
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[1])
		m.commits = append(m.commits, Commit{Hash: parts[0][:8], Date: date, Author: parts[2], Message: parts[3]})
	}
}

func (m *Model) collectFileChurn() {
	m.fileChurn = nil
	cmd := exec.Command("git", "log", "--format=", "-n", "100", "--name-only")
	out, _ := cmd.Output()
	changes := make(map[string]int)
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) > 0 {
			changes[line]++
		}
	}
	for path, count := range changes {
		m.fileChurn = append(m.fileChurn, FileChurn{Path: path, ChurnScore: count})
	}
	sort.Slice(m.fileChurn, func(i, j int) bool { return m.fileChurn[i].ChurnScore > m.fileChurn[j].ChurnScore })
	if len(m.fileChurn) > 15 {
		m.fileChurn = m.fileChurn[:15]
	}
}

func (m *Model) collectBranches() {
	m.branches = nil
	cmd := exec.Command("git", "branch", "-a")
	out, _ := cmd.Output()
	curCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cur, _ := curCmd.Output()
	current := strings.TrimSpace(string(cur))
	for _, line := range strings.Split(string(out), "\n") {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if len(branch) == 0 || strings.Contains(branch, "->") {
			continue
		}
		logCmd := exec.Command("git", "log", "-1", "--format=%h %s", branch)
		logOut, _ := logCmd.Output()
		m.branches = append(m.branches, Branch{Name: branch, IsCurrent: branch == current, LastCommit: strings.TrimSpace(string(logOut))})
	}
}

func (m *Model) updateTables() {
	var commitRows []table.Row
	maxCommits := 10
	if len(m.commits) < maxCommits {
		maxCommits = len(m.commits)
	}
	for _, c := range m.commits[:maxCommits] {
		commitRows = append(commitRows, table.Row{c.Hash, c.Date.Format("2006-01-02"), truncate(c.Author, 12), truncate(c.Message, 28)})
	}
	m.commitTable.SetRows(commitRows)

	var churnRows []table.Row
	for _, fc := range m.fileChurn {
		churnRows = append(churnRows, table.Row{truncate(fc.Path, 38), fmt.Sprintf("%d", fc.ChurnScore)})
	}
	m.churnTable.SetRows(churnRows)

	var branchRows []table.Row
	maxBranches := 10
	if len(m.branches) < maxBranches {
		maxBranches = len(m.branches)
	}
	for _, b := range m.branches[:maxBranches] {
		current := " "
		if b.IsCurrent {
			current = successStyle.Render("●")
		}
		branchRows = append(branchRows, table.Row{b.Name, current, truncate(b.LastCommit, 18)})
	}
	m.branchTable.SetRows(branchRows)
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
		case "1", "2", "3":
			m.selectedView = int(msg.String()[0] - '1')
		case "j", "right", "down":
			m.selectedView = (m.selectedView + 1) % 3
		case "k", "left", "up":
			m.selectedView = (m.selectedView + 2) % 3
		case "r":
			m.Scan()
		}

		switch m.selectedView {
		case 0:
			m.commitTable, cmd = m.commitTable.Update(msg)
		case 1:
			m.churnTable, cmd = m.churnTable.Update(msg)
		case 2:
			m.branchTable, cmd = m.branchTable.Update(msg)
		}
	}
	return m, cmd
}

func (m Model) View() string {
	if m.scanning {
		return titleStyle.Render("⎔ Git") + "\n\n" + infoStyle.Render("Scanning...")
	}
	header := titleStyle.Render("⎔ Git")
	stats := ""
	if m.hasScanned {
		stats = subtitleStyle.Render(fmt.Sprintf("Commits: %d  |  Branches: %d", len(m.commits), len(m.branches)))
	} else {
		stats = infoStyle.Render("Press [r] to scan")
	}
	var content string
	switch m.selectedView {
	case 0:
		content = m.commitTable.View()
	case 1:
		content = m.churnTable.View()
	case 2:
		content = m.branchTable.View()
	}
	mainContent := fmt.Sprintf("%s\n%s\n%s\n%s", header, stats, content, subtitleStyle.Render("[1]/[2]/[3]: view  ↑/↓: select  r: rescan"))
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
