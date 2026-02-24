# DevSentinel

A modern, all-in-one Terminal User Interface (TUI) for project intelligence - combining architecture analysis, runtime monitoring, git insights, code quality metrics, and real-time log streaming.

![DevSentinel](https://img.shields.io/badge/DevSentinel-v1.0.0-green)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8)
![License](https://img.shields.io/badge/License-MIT-blue)

## ✨ Features

### 🏗️ Architecture Analyzer
- **Circular dependency detection** - Find and fix dependency cycles
- **Module complexity metrics** - Lines of code, function counts
- **Large file detection** - Identify files needing refactoring
- **God class detection** - Spot oversized modules
- **Layer validation** - Clean architecture checks

### ⚡ Runtime Monitor
- **CPU/Memory usage** - Real-time system metrics
- **Process list** - Top processes by resource usage
- **Port monitoring** - View open network ports
- **Docker containers** - Container status and health

### ⎔ Git Intelligence
- **Commit history** - Recent commits with details
- **File churn analysis** - Most frequently edited files
- **Branch visualization** - View all branches
- **Contributor stats** - Commit counts by author

### ◉ Code Quality
- **Cyclomatic complexity** - Identify complex functions
- **Dependency analysis** - Package size and version
- **Security scanning** - Vulnerability detection (govulncheck)

### ⏺ Real-Time Log Stream
- **Live log parsing** - Stream application logs
- **Error grouping** - Detect repeated errors
- **Pattern recognition** - Auto-suggest fixes
- **Stack trace highlighting** - Focus on hotspots

## 🚀 Quick Start

```bash
# Run the binary
./devsentinel

# Or build from source
go build -o devsentinel ./cmd
```

## 📺 UI Overview

DevSentinel features a simple, borderless layout:

- **Header**: Shows app name, version, CPU/Memory stats, and tab bar
- **Content**: Interactive tables for each module
- **Footer**: Keyboard shortcut hints

Use `↑`/`↓` to navigate table rows, `1`/`2`/`3` to switch views within tabs.

## ⌨️ Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `←` / `→` or `j` / `k` | Previous/Next tab |
| `1-6` | Direct navigation to tab |
| `g` | Go to dashboard |
| `↑` / `↓` | Navigate rows in tables |
| `1` / `2` / `3` | Switch sub-view within tab |
| `r` | Refresh/Scan data |
| `s` | Start/Stop log stream |
| `a` | Toggle auto-refresh |
| `c` | Clear logs |
| `?` | Toggle help |
| `q` | Quit |

## 🛠️ Requirements

- **Terminal** - 256-color terminal (most modern terminals)
- **Git** - For Git Intelligence tab
- **Docker** - For container monitoring
- **ss** - For port scanning (usually pre-installed)
- **govulncheck** (optional) - `go install golang.org/x/vuln/cmd/govulncheck@latest`

## 📦 Installation

### From Binary

Download from [Releases](https://github.com/yourusername/devsentinel/releases):

```bash
# Linux
wget https://github.com/yourusername/devsentinel/releases/latest/download/devsentinel-linux-amd64
chmod +x devsentinel-linux-amd64
./devsentinel-linux-amd64

# macOS
wget https://github.com/yourusername/devsentinel/releases/latest/download/devsentinel-darwin-amd64
chmod +x devsentinel-darwin-amd64
./devsentinel-darwin-amd64
```

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/devsentinel.git
cd devsentinel

# Build
go build -o devsentinel ./cmd/main.go

# Run
./devsentinel
```

### From Go Install

```bash
go install github.com/yourusername/devsentinel@latest
```

## 🎨 Theme

DevSentinel uses a modern dark theme with:

- **Primary**: Cyan (#45)
- **Accent**: Pink (#219)
- **Success**: Green (#46)
- **Warning**: Orange (#208)
- **Error**: Red (#196)
- **Info**: Light Blue (#75)
- **Background**: Dark (#232)

## 🏗️ Architecture

```
devsentinel/
├── cmd/
│   ├── main.go           # Entry point
│   └── internal/
│       ├── ui/           # Main TUI orchestrator
│       ├── analyzer/     # Architecture analysis
│       ├── monitor/       # Runtime monitoring
│       ├── git/          # Git intelligence
│       ├── quality/      # Code quality metrics
│       └── stream/       # Real-time log streaming
└── bin/
    └── devsentinel       # Compiled binary
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

MIT License - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Charm](https://charm.sh/) - For making terminal UIs beautiful
