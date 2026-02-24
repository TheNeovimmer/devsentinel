package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	SidebarWidth = 18
	ContentWidth = 58
)

type Module struct {
	Name       string
	Path       string
	Files      int
	Functions  int
	Complexity int
	Lines      int
	Deps       []string
}

type Dependency struct {
	From string
	To   string
	Type string
}

type SubView struct {
	Name        string
	Key         string
	Description string
}

type Model struct {
	modules      []Module
	dependencies []Dependency
	issues       []string
	circles      [][]string
	largeFiles   []string
	deadCode     []string

	moduleTable table.Model
	issueTable  table.Model
	depTable    table.Model

	subViews     []SubView
	selectedView int

	ready  bool
	width  int
	height int

	analyzing    bool
	hasScanned   bool
	lastScanTime string
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

	warningStyle = lipgloss.NewStyle().
			Foreground(colors.warning)

	errorStyle = lipgloss.NewStyle().
			Foreground(colors.error)

	successStyle = lipgloss.NewStyle().
			Foreground(colors.success)

	boxStyle = lipgloss.NewStyle().
			Foreground(colors.surface)
)

func NewModel() Model {
	m := Model{
		subViews: []SubView{
			{"Overview", "1", "Code structure"},
			{"Issues", "2", "Warnings"},
			{"Dependencies", "3", "Package deps"},
		},
		selectedView: 0,
	}

	moduleColumns := []table.Column{
		{Title: "Module", Width: 25},
		{Title: "Files", Width: 8},
		{Title: "Funcs", Width: 8},
		{Title: "Complexity", Width: 12},
		{Title: "Lines", Width: 10},
	}
	m.moduleTable = table.New(
		table.WithColumns(moduleColumns),
		table.WithFocused(true),
	)

	issueColumns := []table.Column{
		{Title: "Severity", Width: 10},
		{Title: "Issue", Width: 50},
		{Title: "Location", Width: 30},
	}
	m.issueTable = table.New(
		table.WithColumns(issueColumns),
		table.WithFocused(true),
	)

	depColumns := []table.Column{
		{Title: "From", Width: 25},
		{Title: "To", Width: 25},
		{Title: "Type", Width: 15},
	}
	m.depTable = table.New(
		table.WithColumns(depColumns),
		table.WithFocused(true),
	)

	return m
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

func (m *Model) Analyze(path string) {
	m.analyzing = true

	modules, deps, err := analyzeArchitecture(path)
	if err != nil {
		m.issues = append(m.issues, fmt.Sprintf("Error: %v", err))
		m.analyzing = false
		return
	}

	m.modules = modules
	m.dependencies = deps
	m.issues = detectIssues(modules, deps)
	m.circles = findCircularDeps(deps)
	m.largeFiles = findLargeFiles(path)
	m.deadCode = findDeadCode(path)

	m.updateTables()
	m.analyzing = false
	m.hasScanned = true
}

func analyzeArchitecture(root string) ([]Module, []Dependency, error) {
	var modules []Module
	var deps []Dependency

	fset := token.NewFileSet()

	filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		moduleName := filepath.Dir(path)
		moduleName = strings.TrimPrefix(moduleName, root)
		moduleName = strings.TrimPrefix(moduleName, "/")
		if moduleName == "" {
			moduleName = "root"
		}

		funcs := 0
		complexity := 0
		imports := []string{}

		ast.Inspect(node, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				funcs++
				complexity += estimateComplexity(x)
			case *ast.ImportSpec:
				if x.Path != nil {
					imports = append(imports, x.Path.Value)
				}
			}
			return true
		})

		lineCount := 0
		if fileInfo, err := os.Stat(path); err == nil {
			lineCount = int(fileInfo.Size() / 100)
		}

		existing := -1
		for idx, mod := range modules {
			if mod.Name == moduleName {
				existing = idx
				break
			}
		}

		if existing >= 0 {
			modules[existing].Files++
			modules[existing].Functions += funcs
			modules[existing].Complexity += complexity
			modules[existing].Lines += lineCount
			modules[existing].Deps = append(modules[existing].Deps, imports...)
		} else {
			modules = append(modules, Module{
				Name:       moduleName,
				Path:       path,
				Files:      1,
				Functions:  funcs,
				Complexity: complexity,
				Lines:      lineCount,
				Deps:       imports,
			})
		}

		return nil
	})

	for _, mod := range modules {
		for _, dep := range mod.Deps {
			dep = strings.Trim(dep, `"`)
			if strings.HasPrefix(dep, root) || strings.HasPrefix(dep, ".") {
				continue
			}
			deps = append(deps, Dependency{
				From: mod.Name,
				To:   dep,
				Type: "external",
			})
		}
	}

	return modules, deps, nil
}

func estimateComplexity(fn *ast.FuncDecl) int {
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

func detectIssues(modules []Module, deps []Dependency) []string {
	var issues []string

	for _, mod := range modules {
		if mod.Lines > 10000 {
			issues = append(issues, fmt.Sprintf("[%s] %s has %d lines", warningStyle.Render("WARNING"), mod.Name, mod.Lines))
		}
		if mod.Complexity > 100 {
			issues = append(issues, fmt.Sprintf("[%s] %s complexity %d", warningStyle.Render("WARNING"), mod.Name, mod.Complexity))
		}
		if mod.Functions > 50 {
			issues = append(issues, fmt.Sprintf("[%s] %s has %d functions", warningStyle.Render("WARNING"), mod.Name, mod.Functions))
		}
	}

	depCounts := make(map[string]int)
	for _, dep := range deps {
		if dep.Type == "external" {
			depCounts[dep.To]++
		}
	}

	for dep, count := range depCounts {
		if count > 10 {
			issues = append(issues, fmt.Sprintf("[%s] %s has %d deps", infoStyle.Render("INFO"), dep, count))
		}
	}

	return issues
}

func findCircularDeps(deps []Dependency) [][]string {
	var circles [][]string
	type pair struct{ from, to string }
	edges := make(map[pair]bool)
	nodes := make(map[string]bool)

	for _, dep := range deps {
		if dep.Type == "external" {
			edges[pair{dep.From, dep.To}] = true
			nodes[dep.From] = true
			nodes[dep.To] = true
		}
	}

	for node := range nodes {
		visited := make(map[string]bool)
		path := []string{}

		var dfs func(current string) bool
		dfs = func(current string) bool {
			if visited[current] {
				for i, p := range path {
					if p == current {
						circles = append(circles, path[i:])
						return true
					}
				}
				return false
			}
			visited[current] = true
			path = append(path, current)
			for n := range nodes {
				if edges[pair{current, n}] {
					if dfs(n) {
						return true
					}
				}
			}
			path = path[:len(path)-1]
			return false
		}

		dfs(node)
	}

	return circles
}

func findLargeFiles(root string) []string {
	var largeFiles []string
	filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if info.Size() > 5000 {
			largeFiles = append(largeFiles, fmt.Sprintf("%s (%dkB)", path, info.Size()/1000))
		}
		return nil
	})
	return largeFiles
}

func findDeadCode(root string) []string {
	var deadCode []string
	filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if info.Size() < 100 {
			deadCode = append(deadCode, path)
		}
		return nil
	})
	return deadCode
}

func (m *Model) updateTables() {
	var moduleRows []table.Row
	for _, mod := range m.modules {
		moduleRows = append(moduleRows, table.Row{
			mod.Name,
			fmt.Sprintf("%d", mod.Files),
			fmt.Sprintf("%d", mod.Functions),
			fmt.Sprintf("%d", mod.Complexity),
			fmt.Sprintf("%d", mod.Lines),
		})
	}
	m.moduleTable.SetRows(moduleRows)

	var issueRows []table.Row
	for _, issue := range m.issues {
		severity := "INFO"
		if strings.Contains(issue, "WARNING") {
			severity = "WARNING"
		}
		parts := strings.SplitN(issue, "]", 2)
		location := ""
		if len(parts) > 1 {
			location = parts[1]
		}
		issueRows = append(issueRows, table.Row{severity, parts[0] + "]", location})
	}
	m.issueTable.SetRows(issueRows)

	var depRows []table.Row
	for _, dep := range m.dependencies[:min(50, len(m.dependencies))] {
		depRows = append(depRows, table.Row{dep.From, dep.To, dep.Type})
	}
	m.depTable.SetRows(depRows)
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
		case "r":
			m.Analyze(".")
		}

		switch m.selectedView {
		case 0:
			m.moduleTable, cmd = m.moduleTable.Update(msg)
		case 1:
			m.issueTable, cmd = m.issueTable.Update(msg)
		case 2:
			m.depTable, cmd = m.depTable.Update(msg)
		}

	case struct{}:
		m.analyzing = false
	}

	return m, cmd
}

func (m Model) View() string {
	if m.analyzing {
		return titleStyle.Render("◈ Architecture") + "\n\n" +
			infoStyle.Render("Scanning code structure...")
	}

	header := titleStyle.Render("◈ Architecture")

	stats := ""
	if m.hasScanned {
		stats = subtitleStyle.Render(fmt.Sprintf("Modules: %d  |  Deps: %d  |  Issues: %d",
			len(m.modules), len(m.dependencies), len(m.issues)))
	} else {
		stats = infoStyle.Render("Press [r] to scan")
	}

	var content string
	switch m.selectedView {
	case 0:
		content = m.moduleTable.View()
	case 1:
		content = m.issueTable.View()
	case 2:
		content = m.depTable.View()
	}

	legend := ""
	if len(m.issues) > 0 {
		legend = "\n" + warningStyle.Render("Warnings: ") + fmt.Sprintf("%d", len(m.issues))
	}

	mainContent := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", header, stats, content, legend, subtitleStyle.Render("[1]/[2]/[3]: view  ↑/↓: select  r: rescan"))

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
