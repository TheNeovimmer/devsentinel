package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	Additions  int
	Deletions  int
	ChurnScore int
}

type Branch struct {
	Name       string
	IsCurrent  bool
	LastCommit string
}

type Contributor struct {
	Name      string
	Commits   int
	Additions int
	Deletions int
}

type Model struct {
	commits      []Commit
	fileChurn    []FileChurn
	branches     []Branch
	contributors []Contributor
	heatmapData  [][]int

	commitTable table.Model
	churnTable  table.Model
	branchTable table.Model

	selectedView int
	ready        bool
	width        int
	height       int

	scanning bool
	lastScan time.Time
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

	commitColumns := []table.Column{
		{Title: " Hash ", Width: 8},
		{Title: " Date ", Width: 12},
		{Title: " Author ", Width: 14},
		{Title: " Message ", Width: 40},
		{Title: " Files ", Width: 6},
	}
	m.commitTable = table.New(table.WithColumns(commitColumns))

	churnColumns := []table.Column{
		{Title: " File ", Width: 40},
		{Title: " Churn ", Width: 10},
	}
	m.churnTable = table.New(table.WithColumns(churnColumns))

	branchColumns := []table.Column{
		{Title: " Branch ", Width: 25},
		{Title: " Current ", Width: 8},
		{Title: " Last Commit ", Width: 30},
	}
	m.branchTable = table.New(table.WithColumns(branchColumns))

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
	m.collectContributors()
	m.collectHeatmap()

	m.updateTables()
	m.lastScan = time.Now()
	m.scanning = false
}

func (m *Model) collectCommits() {
	m.commits = []Commit{}

	cmd := exec.Command("git", "log", "--format=%H|%ai|%an|%s", "-n", "50")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	cmd2 := exec.Command("git", "log", "-n", "50", "--name-only", "--pretty=format:")
	out2, _ := cmd2.Output()
	filesByCommit := parseFilesByCommit(string(out2))

	lines := strings.Split(string(out), "\n")
	idx := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[1])

		files := 0
		if idx < len(filesByCommit) {
			files = filesByCommit[idx]
		}

		m.commits = append(m.commits, Commit{
			Hash:    parts[0][:8],
			Date:    date,
			Author:  parts[2],
			Message: parts[3],
			Files:   files,
		})
		idx++
	}
}

func parseFilesByCommit(output string) []int {
	commits := strings.Split(output, "\n")
	counts := []int{}
	currentCount := 0

	for _, line := range commits {
		if strings.HasPrefix(line, "commit ") {
			if currentCount > 0 {
				counts = append(counts, currentCount)
			}
			currentCount = 0
		} else if len(line) > 0 {
			currentCount++
		}
	}

	if currentCount > 0 {
		counts = append(counts, currentCount)
	}

	return counts
}

func (m *Model) collectFileChurn() {
	m.fileChurn = []FileChurn{}

	cmd := exec.Command("git", "log", "--format=", "-n", "100", "--name-only")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	fileChanges := make(map[string]int)
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		fileChanges[line]++
	}

	for path, count := range fileChanges {
		m.fileChurn = append(m.fileChurn, FileChurn{
			Path:       path,
			ChurnScore: count,
		})
	}

	sort.Slice(m.fileChurn, func(i, j int) bool {
		return m.fileChurn[i].ChurnScore > m.fileChurn[j].ChurnScore
	})

	if len(m.fileChurn) > 20 {
		m.fileChurn = m.fileChurn[:20]
	}
}

func (m *Model) collectBranches() {
	m.branches = []Branch{}

	cmd := exec.Command("git", "branch", "-a")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	currentCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	currentBranch, _ := currentCmd.Output()
	current := strings.TrimSpace(string(currentBranch))

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if len(branch) == 0 || strings.Contains(branch, "->") {
			continue
		}

		logCmd := exec.Command("git", "log", "-1", "--format=%h %s", branch)
		logOut, _ := logCmd.Output()

		m.branches = append(m.branches, Branch{
			Name:       branch,
			IsCurrent:  branch == current,
			LastCommit: strings.TrimSpace(string(logOut)),
		})
	}
}

func (m *Model) collectContributors() {
	m.contributors = []Contributor{}

	cmd := exec.Command("git", "shortlog", "-sne", "-n", "10")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	re := regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		commits, _ := strconv.Atoi(matches[1])

		m.contributors = append(m.contributors, Contributor{
			Name:    matches[2],
			Commits: commits,
		})
	}
}

func (m *Model) collectHeatmap() {
	m.heatmapData = make([][]int, 7)
	for i := range m.heatmapData {
		m.heatmapData[i] = make([]int, 52)
	}

	cmd := exec.Command("git", "log", "--format=%ai", "-n", "365")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	re := regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`)
	lines := strings.Split(string(out), "\n")

	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		date, err := time.Parse("2006-01-02", matches[0])
		if err != nil {
			continue
		}

		if date.Before(oneYearAgo) {
			continue
		}

		week := int(now.Sub(date).Hours() / 24 / 7)
		day := int(date.Weekday())

		if week < 52 && day >= 0 && day < 7 {
			m.heatmapData[day][51-week]++
		}
	}
}

func (m *Model) updateTables() {
	var commitRows []table.Row
	for _, c := range m.commits[:minInt(15, len(m.commits))] {
		commitRows = append(commitRows, table.Row{
			c.Hash,
			c.Date.Format("2006-01-02"),
			truncate(c.Author, 12),
			truncate(c.Message, 38),
			fmt.Sprintf("%d", c.Files),
		})
	}
	m.commitTable.SetRows(commitRows)

	var churnRows []table.Row
	for _, fc := range m.fileChurn {
		churnStr := fmt.Sprintf("%d", fc.ChurnScore)
		if fc.ChurnScore > 10 {
			churnStr = lipgloss.NewStyle().Foreground(colors.warning).Render(churnStr)
		}

		churnRows = append(churnRows, table.Row{
			truncate(fc.Path, 38),
			churnStr,
		})
	}
	m.churnTable.SetRows(churnRows)

	var branchRows []table.Row
	for _, b := range m.branches[:minInt(15, len(m.branches))] {
		current := " "
		if b.IsCurrent {
			current = lipgloss.NewStyle().Foreground(colors.success).Render("●")
		}
		branchRows = append(branchRows, table.Row{
			b.Name,
			current,
			truncate(b.LastCommit, 28),
		})
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
			m.Scan()
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.scanning {
		return headerStyle.Render("⎔ Git Intelligence") + "\n\n" +
			lipgloss.NewStyle().Foreground(colors.info).Render("Scanning git repository...")
	}

	header := headerStyle.Render("⎔ Git Intelligence")
	stats := fmt.Sprintf("Commits: %d | Branches: %d | Contributors: %d",
		len(m.commits), len(m.branches), len(m.contributors))

	views := []string{"Commits", "File Churn", "Branches"}
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
		content = m.commitTable.View()
	case 1:
		content = m.renderChurnView()
	case 2:
		content = m.branchTable.View()
	}

	hint := lipgloss.NewStyle().Foreground(colors.info).Render("\nPress 'r' to scan | j/k to switch views")

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s\n", header, stats, viewIndicator, content, hint)
}

func (m Model) renderChurnView() string {
	var riskyFiles []string
	for _, fc := range m.fileChurn {
		if fc.ChurnScore > 10 {
			riskyFiles = append(riskyFiles, fc.Path)
		}
	}

	content := m.churnTable.View()

	if len(riskyFiles) > 0 {
		content += "\n" + lipgloss.NewStyle().Foreground(colors.warning).Render("⚠ Risky files (edited 10+ times):")
		for _, f := range riskyFiles[:5] {
			content += "\n  - " + f
		}
	}

	return content
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
