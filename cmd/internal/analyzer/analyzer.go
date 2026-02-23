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

	selectedView int
	ready        bool
	width        int
	height       int

	analyzing bool
}

var (
	accentColor    = lipgloss.Color("99")
	highlightColor = lipgloss.Color("86")
	warningColor   = lipgloss.Color("226")
	errorColor     = lipgloss.Color("196")
	successColor   = lipgloss.Color("76")
	infoColor      = lipgloss.Color("75")
	subtleColor    = lipgloss.Color("241")
	panelColor     = lipgloss.Color("236")

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlightColor)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	infoStyle = lipgloss.NewStyle().
			Foreground(infoColor)

	tabStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Padding(0, 1)

	activeTabStyle = tabStyle.
			Foreground(highlightColor).
			Bold(true)
)

func NewModel() Model {
	m := Model{
		selectedView: 0,
	}

	moduleColumns := []table.Column{
		{Title: "Module", Width: 25},
		{Title: "Files", Width: 8},
		{Title: "Funcs", Width: 8},
		{Title: "Complexity", Width: 12},
		{Title: "Lines", Width: 10},
	}
	m.moduleTable = table.New(table.WithColumns(moduleColumns))

	issueColumns := []table.Column{
		{Title: "Severity", Width: 10},
		{Title: "Issue", Width: 50},
		{Title: "Location", Width: 30},
	}
	m.issueTable = table.New(table.WithColumns(issueColumns))

	depColumns := []table.Column{
		{Title: "From", Width: 25},
		{Title: "To", Width: 25},
		{Title: "Type", Width: 15},
	}
	m.depTable = table.New(table.WithColumns(depColumns))

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
			issues = append(issues, fmt.Sprintf("[%s] %s has %d lines - consider splitting", warningStyle.Render("WARNING"), mod.Name, mod.Lines))
		}
		if mod.Complexity > 100 {
			issues = append(issues, fmt.Sprintf("[%s] %s has complexity %d - refactor needed", warningStyle.Render("WARNING"), mod.Name, mod.Complexity))
		}
		if mod.Functions > 50 {
			issues = append(issues, fmt.Sprintf("[%s] %s has %d functions - consider splitting", warningStyle.Render("WARNING"), mod.Name, mod.Functions))
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
			issues = append(issues, fmt.Sprintf("[%s] %s has %d incoming dependencies - potential bottleneck", infoStyle.Render("INFO"), dep, count))
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
		} else if strings.Contains(issue, "ERROR") {
			severity = "ERROR"
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.selectedView = (m.selectedView + 1) % 3
		case "k", "up":
			m.selectedView = (m.selectedView - 1 + 3) % 3
		case "r":
			m.Analyze(".")
		}

	case struct{}:
		m.analyzing = false
	}

	return m, nil
}

func (m Model) View() string {
	header := headerStyle.Render("◈ Architecture Analyzer")
	stats := fmt.Sprintf("Modules: %d | Dependencies: %d | Issues: %d",
		len(m.modules), len(m.dependencies), len(m.issues))

	views := []string{"Modules", "Issues", "Dependencies"}
	viewIndicator := ""
	for i, v := range views {
		if i == m.selectedView {
			viewIndicator += activeTabStyle.Render(" "+v+" ") + " "
		} else {
			viewIndicator += tabStyle.Render(" "+v+" ") + " "
		}
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

	legend := "\n" + warningStyle.Render("Legend: ") +
		successStyle.Render("Low ") +
		warningStyle.Render("Medium ") +
		errorStyle.Render("High")

	hint := "\n" + infoStyle.Render("Press 'r' to analyze | j/k to switch views")

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s\n%s\n", header, stats, viewIndicator, content, legend, hint)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
