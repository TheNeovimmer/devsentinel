package quality

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Complexity struct {
	File       string
	Function   string
	Complexity int
	Line       int
}

type DepSize struct {
	Name     string
	Version  string
	Bytes    int64
	Indirect bool
}

type SecurityVuln struct {
	Package    string
	Severity   string
	Title      string
	FixVersion string
}

type Model struct {
	complexities    []Complexity
	depSizes        []DepSize
	vulnerabilities []SecurityVuln
	bundleSize      int64

	compTable table.Model
	depTable  table.Model
	vulnTable table.Model

	selectedView int
	ready        bool
	width        int
	height       int

	scanning bool
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
	m := Model{}

	compColumns := []table.Column{
		{Title: "File", Width: 30},
		{Title: "Function", Width: 25},
		{Title: "Complexity", Width: 12},
		{Title: "Line", Width: 8},
	}
	m.compTable = table.New(table.WithColumns(compColumns))

	depColumns := []table.Column{
		{Title: "Package", Width: 35},
		{Title: "Version", Width: 15},
		{Title: "Size", Width: 12},
		{Title: "Type", Width: 10},
	}
	m.depTable = table.New(table.WithColumns(depColumns))

	vulnColumns := []table.Column{
		{Title: "Package", Width: 25},
		{Title: "Severity", Width: 12},
		{Title: "Title", Width: 40},
		{Title: "Fix", Width: 10},
	}
	m.vulnTable = table.New(table.WithColumns(vulnColumns))

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
	m.analyzeDependencies()
	m.checkVulnerabilities()
	m.estimateBundleSize()

	m.updateTables()
	m.scanning = false
}

func (m *Model) analyzeComplexity() {
	m.complexities = []Complexity{}

	fset := token.NewFileSet()

	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		ast.Inspect(node, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				if x.Name == nil {
					return true
				}

				complexity := calculateComplexity(x)
				if complexity > 5 {
					m.complexities = append(m.complexities, Complexity{
						File:       filepath.Base(path),
						Function:   x.Name.Name,
						Complexity: complexity,
						Line:       fset.Position(x.Pos()).Line,
					})
				}
			}
			return true
		})

		return nil
	})

	sort.Slice(m.complexities, func(i, j int) bool {
		return m.complexities[i].Complexity > m.complexities[j].Complexity
	})

	if len(m.complexities) > 30 {
		m.complexities = m.complexities[:30]
	}
}

func calculateComplexity(fn *ast.FuncDecl) int {
	complexity := 1
	if fn.Body == nil {
		return complexity
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.CaseClause, *ast.CommClause,
			*ast.BinaryExpr:
			complexity++
		}
		return true
	})

	return complexity
}

func (m *Model) analyzeDependencies() {
	m.depSizes = []DepSize{}

	data, err := os.ReadFile("go.mod")
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	re := regexp.MustCompile(`^\s*(\S+)\s+v?([\d.]+)`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			m.depSizes = append(m.depSizes, DepSize{
				Name:    matches[1],
				Version: matches[2],
			})
		}
	}

	sort.Slice(m.depSizes, func(i, j int) bool {
		return m.depSizes[i].Name < m.depSizes[j].Name
	})
}

func (m *Model) checkVulnerabilities() {
	m.vulnerabilities = []SecurityVuln{}

	cmd := exec.Command("govulncheck", "./...")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	re := regexp.MustCompile(`(\S+)\s+(HIGH|MEDIUM|LOW)\s+(.+?)(?:\s+v(\d+\.\d+\.\d+))?`)
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			m.vulnerabilities = append(m.vulnerabilities, SecurityVuln{
				Package:    matches[1],
				Severity:   matches[2],
				Title:      matches[3],
				FixVersion: matches[4],
			})
		}
	}
}

func (m *Model) estimateBundleSize() {
	m.bundleSize = 0

	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".ts") ||
			strings.HasSuffix(path, ".jsx") || strings.HasSuffix(path, ".tsx") {
			m.bundleSize += info.Size()
		}

		return nil
	})
}

func (m *Model) updateTables() {
	var compRows []table.Row
	for _, c := range m.complexities {
		complexity := fmt.Sprintf("%d", c.Complexity)
		if c.Complexity > 20 {
			complexity = errorStyle.Render(complexity)
		} else if c.Complexity > 10 {
			complexity = warningStyle.Render(complexity)
		}

		compRows = append(compRows, table.Row{
			c.File,
			c.Function,
			complexity,
			fmt.Sprintf("%d", c.Line),
		})
	}
	m.compTable.SetRows(compRows)

	var depRows []table.Row
	for _, d := range m.depSizes {
		depRows = append(depRows, table.Row{
			d.Name,
			d.Version,
			"-",
			"direct",
		})
	}
	m.depTable.SetRows(depRows)

	var vulnRows []table.Row
	for _, v := range m.vulnerabilities {
		severity := v.Severity
		if v.Severity == "HIGH" {
			severity = errorStyle.Render(severity)
		} else if v.Severity == "MEDIUM" {
			severity = warningStyle.Render(severity)
		}

		vulnRows = append(vulnRows, table.Row{
			v.Package,
			severity,
			truncate(v.Title, 38),
			v.FixVersion,
		})
	}
	m.vulnTable.SetRows(vulnRows)
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
		return headerStyle.Render("Code Quality Metrics") + "\n\n" +
			infoStyle.Render("Analyzing code quality...")
	}

	header := headerStyle.Render("Code Quality Metrics")

	highCount := 0
	for _, c := range m.complexities {
		if c.Complexity > 20 {
			highCount++
		}
	}

	stats := fmt.Sprintf("Complex functions: %d | Dependencies: %d | Vulnerabilities: %d | Bundle: %.1fMB",
		len(m.complexities), len(m.depSizes), len(m.vulnerabilities), float64(m.bundleSize)/1024/1024)

	views := []string{"Complexity", "Dependencies", "Security"}
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
		content = m.compTable.View()
		if highCount > 0 {
			content += "\n" + errorStyle.Render("⚠ High complexity detected in "+fmt.Sprintf("%d functions", highCount))
		}
	case 1:
		content = m.depTable.View()
	case 2:
		content = m.vulnTable.View()
		if len(m.vulnerabilities) == 0 {
			content += "\n" + successStyle.Render("✓ No vulnerabilities detected")
		}
	}

	hint := "\n" + infoStyle.Render("Press 'r' to scan")

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s\n", header, stats, viewIndicator, content, hint)
}
