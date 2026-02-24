package quality

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	SidebarWidth = 18
	ContentWidth = 58
)

type Complexity struct {
	File       string
	Function   string
	Complexity int
	Line       int
}

type SubView struct {
	Name        string
	Key         string
	Description string
}

type Model struct {
	complexities []Complexity
	compTable    table.Model
	depTable     table.Model
	subViews     []SubView
	selectedView int
	ready        bool
	width        int
	height       int
	scanning     bool
	hasScanned   bool
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
			{"Complexity", "1", "Cyclomatic"},
			{"Dependencies", "2", "Packages"},
			{"Security", "3", "Vulns"},
		},
		selectedView: 0,
	}
	m.compTable = table.New(
		table.WithColumns([]table.Column{
			{Title: "File", Width: 25}, {Title: "Function", Width: 20}, {Title: "Complexity", Width: 10}, {Title: "Line", Width: 6},
		}),
		table.WithFocused(true),
	)
	m.depTable = table.New(
		table.WithColumns([]table.Column{
			{Title: "Package", Width: 35}, {Title: "Version", Width: 15},
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
	m.analyzeComplexity()
	m.updateTables()
	m.scanning = false
	m.hasScanned = true
}

func (m *Model) analyzeComplexity() {
	m.complexities = nil
	fset := token.NewFileSet()
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		ast.Inspect(node, func(n ast.Node) bool {
			if x, ok := n.(*ast.FuncDecl); ok && x.Name != nil {
				complexity := calcComplexity(x)
				if complexity > 5 {
					m.complexities = append(m.complexities, Complexity{File: filepath.Base(path), Function: x.Name.Name, Complexity: complexity, Line: fset.Position(x.Pos()).Line})
				}
			}
			return true
		})
		return nil
	})
	sort.Slice(m.complexities, func(i, j int) bool { return m.complexities[i].Complexity > m.complexities[j].Complexity })
	if len(m.complexities) > 20 {
		m.complexities = m.complexities[:20]
	}
}

func calcComplexity(fn *ast.FuncDecl) int {
	complexity := 1
	if fn.Body == nil {
		return complexity
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.BinaryExpr:
			complexity++
		}
		return true
	})
	return complexity
}

func (m *Model) updateTables() {
	var rows []table.Row
	for _, c := range m.complexities {
		comp := fmt.Sprintf("%d", c.Complexity)
		if c.Complexity > 20 {
			comp = errorStyle.Render(comp)
		} else if c.Complexity > 10 {
			comp = warningStyle.Render(comp)
		}
		rows = append(rows, table.Row{c.File, c.Function, comp, fmt.Sprintf("%d", c.Line)})
	}
	m.compTable.SetRows(rows)
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
			m.compTable, cmd = m.compTable.Update(msg)
		case 1:
			m.depTable, cmd = m.depTable.Update(msg)
		}
	}
	return m, cmd
}

func (m Model) View() string {
	if m.scanning {
		return titleStyle.Render("◉ Code Quality") + "\n\n" + infoStyle.Render("Analyzing...")
	}
	header := titleStyle.Render("◉ Code Quality")
	stats := ""
	if m.hasScanned {
		stats = subtitleStyle.Render(fmt.Sprintf("Complex functions: %d", len(m.complexities)))
	} else {
		stats = infoStyle.Render("Press [r] to analyze")
	}
	content := m.compTable.View()
	if m.selectedView == 0 && len(m.complexities) > 0 {
		highCount := 0
		for _, c := range m.complexities {
			if c.Complexity > 20 {
				highCount++
			}
		}
		if highCount > 0 {
			content += "\n" + errorStyle.Render("⚠ High complexity: "+fmt.Sprintf("%d", highCount))
		}
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
