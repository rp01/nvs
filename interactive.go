package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// STYLES
// =============================================================================

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED")
	successColor   = lipgloss.Color("#10B981")
	errorColor     = lipgloss.Color("#EF4444")
	warningColor   = lipgloss.Color("#F59E0B")
	mutedColor     = lipgloss.Color("#6B7280")
	textColor      = lipgloss.Color("#F3F4F6")
	highlightColor = lipgloss.Color("#A78BFA")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(1, 2).
			MarginTop(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(textColor)

	dimStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	successMsgStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	versionCurrentStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Bold(true)
)

// =============================================================================
// TYPES
// =============================================================================

type viewState int

const (
	viewMainMenu viewState = iota
	viewInstallInput
	viewSelectVersion
	viewSelectUninstall
	viewListVersions
	viewProcessing
	viewResult
)

type menuItem struct {
	icon        string
	title       string
	description string
	action      string
}

// =============================================================================
// MESSAGES
// =============================================================================

type taskDoneMsg struct {
	success bool
	message string
}

type versionsLoadedMsg struct {
	versions []string
	current  string
}

// =============================================================================
// MODEL
// =============================================================================

type model struct {
	nvs               *NodeVersionSwitcher
	state             viewState
	cursor            int
	menuItems         []menuItem
	installedVersions []string
	currentVersion    string
	textInput         textinput.Model
	spinner           spinner.Model
	processingMsg     string
	resultMsg         string
	resultSuccess     bool
	quitting          bool
	width             int
	height            int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "e.g., 18, 20, lts, latest"
	ti.CharLimit = 32
	ti.Width = 40

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(warningColor)

	return model{
		nvs:    NewNodeVersionSwitcher(),
		state:  viewMainMenu,
		cursor: 0,
		menuItems: []menuItem{
			{"📦", "Install Node.js", "Download and install a new version", "install"},
			{"🔄", "Switch Version", "Change the active Node.js version", "use"},
			{"📋", "List Versions", "Show all installed versions (Enter to switch)", "list"},
			{"🗑️ ", "Uninstall", "Remove an installed version", "uninstall"},
			{"🔧", "Setup", "Initialize NVS and configure PATH", "setup"},
			{"❓", "Help", "Show usage information", "help"},
			{"👋", "Exit", "Quit NVS", "exit"},
		},
		textInput: ti,
		spinner:   sp,
	}
}

// =============================================================================
// INIT
// =============================================================================

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.loadVersionsCmd(),
		m.spinner.Tick,
	)
}

// =============================================================================
// UPDATE
// =============================================================================

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle text input first when in install input state
		if m.state == viewInstallInput {
			key := msg.String()
			// Only handle special keys ourselves
			switch key {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				return m.goBack()
			case "enter":
				version := strings.TrimSpace(m.textInput.Value())
				if version != "" {
					m.state = viewProcessing
					m.processingMsg = fmt.Sprintf("Installing Node.js %s...", version)
					return m, tea.Batch(m.spinner.Tick, m.installCmd(version))
				}
				return m, nil
			default:
				// Pass all other keys to text input
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}
		// For other states, use the key handler
		return m.handleKeyPress(msg)

	case spinner.TickMsg:
		if m.state == viewProcessing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case versionsLoadedMsg:
		m.installedVersions = msg.versions
		m.currentVersion = msg.current
		return m, nil

	case taskDoneMsg:
		m.state = viewResult
		m.resultSuccess = msg.success
		m.resultMsg = msg.message
		return m, m.loadVersionsCmd()
	}

	return m, nil
}

func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEsc:
		return m.goBack()
	}

	key := msg.String()
	if key == "q" && m.state == viewMainMenu {
		m.quitting = true
		return m, tea.Quit
	}

	// State-specific handling
	switch m.state {
	case viewMainMenu:
		return m.handleMainMenu(msg)
	case viewSelectVersion, viewSelectUninstall, viewListVersions:
		return m.handleVersionSelect(msg)
	case viewResult:
		if msg.Type == tea.KeyEnter || key == " " {
			m.state = viewMainMenu
			m.cursor = 0
		}
	}

	return m, nil
}

func (m model) handleMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(m.menuItems)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		return m.executeAction()
	default:
		switch msg.String() {
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "j":
			if m.cursor < len(m.menuItems)-1 {
				m.cursor++
			}
		case " ":
			return m.executeAction()
		}
	}
	return m, nil
}

func (m model) handleVersionSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	selectVersion := func() (tea.Model, tea.Cmd) {
		if len(m.installedVersions) > 0 && m.cursor < len(m.installedVersions) {
			version := m.installedVersions[m.cursor]
			if m.state == viewSelectVersion || m.state == viewListVersions {
				m.state = viewProcessing
				m.processingMsg = fmt.Sprintf("Switching to %s...", version)
				return m, tea.Batch(m.spinner.Tick, m.useCmd(version))
			} else {
				m.state = viewProcessing
				m.processingMsg = fmt.Sprintf("Uninstalling %s...", version)
				return m, tea.Batch(m.spinner.Tick, m.uninstallCmd(version))
			}
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(m.installedVersions)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		return selectVersion()
	default:
		switch msg.String() {
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "j":
			if m.cursor < len(m.installedVersions)-1 {
				m.cursor++
			}
		case " ":
			return selectVersion()
		}
	}
	return m, nil
}

func (m model) goBack() (tea.Model, tea.Cmd) {
	if m.state != viewMainMenu && m.state != viewProcessing {
		m.state = viewMainMenu
		m.cursor = 0
		m.textInput.Reset()
	}
	return m, nil
}

func (m model) executeAction() (tea.Model, tea.Cmd) {
	action := m.menuItems[m.cursor].action

	switch action {
	case "install":
		m.state = viewInstallInput
		m.textInput.Reset()
		m.textInput.Focus()
		return m, textinput.Blink

	case "use":
		if len(m.installedVersions) == 0 {
			m.state = viewResult
			m.resultSuccess = false
			m.resultMsg = "No versions installed.\n\nUse 'Install Node.js' to get started."
			return m, nil
		}
		m.state = viewSelectVersion
		m.cursor = 0
		return m, nil

	case "list":
		if len(m.installedVersions) == 0 {
			m.state = viewResult
			m.resultSuccess = false
			m.resultMsg = "No versions installed.\n\nUse 'Install Node.js' to get started."
			return m, nil
		}
		m.state = viewListVersions
		m.cursor = 0
		return m, nil

	case "uninstall":
		if len(m.installedVersions) == 0 {
			m.state = viewResult
			m.resultSuccess = false
			m.resultMsg = "No versions installed."
			return m, nil
		}
		m.state = viewSelectUninstall
		m.cursor = 0
		return m, nil

	case "setup":
		m.state = viewProcessing
		m.processingMsg = "Setting up NVS..."
		return m, tea.Batch(m.spinner.Tick, m.setupCmd())

	case "help":
		m.state = viewResult
		m.resultSuccess = true
		m.resultMsg = m.getHelpText()
		return m, nil

	case "exit":
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

// =============================================================================
// VIEW
// =============================================================================

func (m model) View() string {
	if m.quitting {
		return "\n  👋 Goodbye!\n\n"
	}

	var b strings.Builder

	// Header
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("🚀 NVS - Node Version Switcher"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("No admin privileges required"))
	b.WriteString("\n")

	// Content
	switch m.state {
	case viewMainMenu:
		b.WriteString(m.renderMainMenu())
	case viewInstallInput:
		b.WriteString(m.renderInstallInput())
	case viewSelectVersion:
		b.WriteString(m.renderVersionSelect("Select version to use:", false))
	case viewSelectUninstall:
		b.WriteString(m.renderVersionSelect("Select version to uninstall:", true))
	case viewListVersions:
		b.WriteString(m.renderVersionSelect("Installed versions (Enter to switch):", false))
	case viewProcessing:
		b.WriteString(m.renderProcessing())
	case viewResult:
		b.WriteString(m.renderResult())
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(m.getKeyHints()))
	b.WriteString("\n")

	return b.String()
}

func (m model) renderMainMenu() string {
	var b strings.Builder

	for i, item := range m.menuItems {
		cursor := "   "
		style := normalStyle
		if i == m.cursor {
			cursor = " ▸ "
			style = selectedStyle
		}

		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, item.icon, style.Render(item.title)))

		// Show description for selected item
		if i == m.cursor {
			b.WriteString(fmt.Sprintf("     %s\n", dimStyle.Render(item.description)))
		}
	}

	// Status line
	b.WriteString("\n")
	status := fmt.Sprintf("📦 %d version(s) installed", len(m.installedVersions))
	if m.currentVersion != "" {
		status += fmt.Sprintf("  •  Active: %s", m.currentVersion)
	}
	b.WriteString(dimStyle.Render(status))

	return boxStyle.Render(b.String())
}

func (m model) renderInstallInput() string {
	var b strings.Builder

	b.WriteString("Enter Node.js version to install:\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Examples: 18, 20, 22, lts, latest, 20.10.0"))

	return boxStyle.Render(b.String())
}

func (m model) renderVersionSelect(title string, isDanger bool) string {
	var b strings.Builder

	b.WriteString(title)
	b.WriteString("\n\n")

	if len(m.installedVersions) == 0 {
		b.WriteString(dimStyle.Render("No versions installed"))
	} else {
		for i, v := range m.installedVersions {
			cursor := "   "
			style := normalStyle
			suffix := ""

			if i == m.cursor {
				cursor = " ▸ "
				if isDanger {
					style = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
				} else {
					style = selectedStyle
				}
			}

			if v == m.currentVersion {
				suffix = " (current)"
				if i == m.cursor && !isDanger {
					style = versionCurrentStyle
				}
			}

			b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, style.Render(v), dimStyle.Render(suffix)))
		}
	}

	return boxStyle.Render(b.String())
}

func (m model) renderProcessing() string {
	var b strings.Builder

	b.WriteString(m.spinner.View())
	b.WriteString(" ")
	b.WriteString(m.processingMsg)
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Please wait..."))

	return boxStyle.Render(b.String())
}

func (m model) renderResult() string {
	var b strings.Builder

	style := successMsgStyle
	if !m.resultSuccess {
		style = errorMsgStyle
	}

	b.WriteString(style.Render(m.resultMsg))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Press Enter to continue..."))

	return boxStyle.Render(b.String())
}

// =============================================================================
// COMMANDS
// =============================================================================

func (m model) loadVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		versions := []string{}
		current := ""

		if files, err := os.ReadDir(m.nvs.VersionsDir); err == nil {
			for _, f := range files {
				if f.IsDir() {
					versions = append(versions, f.Name())
				}
			}
		}

		if target, err := filepath.EvalSymlinks(m.nvs.CurrentLink); err == nil {
			current = filepath.Base(target)
		}

		return versionsLoadedMsg{versions: versions, current: current}
	}
}

func (m model) installCmd(version string) tea.Cmd {
	return func() tea.Msg {
		if err := m.nvs.Init(); err != nil {
			return taskDoneMsg{false, fmt.Sprintf("❌ Init failed: %v", err)}
		}
		if err := m.nvs.Install(version); err != nil {
			return taskDoneMsg{false, fmt.Sprintf("❌ Install failed: %v", err)}
		}
		return taskDoneMsg{true, fmt.Sprintf("✅ Node.js %s installed successfully!", version)}
	}
}

func (m model) useCmd(version string) tea.Cmd {
	return func() tea.Msg {
		cleanVersion := strings.TrimPrefix(version, "v")
		if err := m.nvs.Use(cleanVersion); err != nil {
			return taskDoneMsg{false, fmt.Sprintf("❌ Switch failed: %v", err)}
		}
		return taskDoneMsg{true, fmt.Sprintf("✅ Now using Node.js %s", version)}
	}
}

func (m model) uninstallCmd(version string) tea.Cmd {
	return func() tea.Msg {
		cleanVersion := strings.TrimPrefix(version, "v")
		if err := m.nvs.Uninstall(cleanVersion); err != nil {
			return taskDoneMsg{false, fmt.Sprintf("❌ Uninstall failed: %v", err)}
		}
		return taskDoneMsg{true, fmt.Sprintf("✅ Uninstalled %s", version)}
	}
}

func (m model) setupCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.nvs.Init(); err != nil {
			return taskDoneMsg{false, fmt.Sprintf("❌ Setup failed: %v", err)}
		}

		msg := fmt.Sprintf(`✅ NVS initialized successfully!

📁 Install directory: %s
📁 Versions directory: %s

Run 'nvs setup' in your terminal to see
PATH configuration instructions.`, m.nvs.NVSDir, m.nvs.VersionsDir)

		return taskDoneMsg{true, msg}
	}
}

// =============================================================================
// HELPERS
// =============================================================================

func (m model) formatVersionList() string {
	if len(m.installedVersions) == 0 {
		return "📦 No versions installed\n\nUse 'Install Node.js' to get started."
	}

	var b strings.Builder
	b.WriteString("📦 Installed Node.js versions:\n\n")

	for _, v := range m.installedVersions {
		prefix := "   "
		suffix := ""
		if v == m.currentVersion {
			prefix = " ▸ "
			suffix = " (current)"
		}
		b.WriteString(fmt.Sprintf("%s%s%s\n", prefix, v, suffix))
	}

	return b.String()
}

func (m model) getKeyHints() string {
	switch m.state {
	case viewMainMenu:
		return "↑/↓ navigate  •  enter select  •  q quit"
	case viewInstallInput:
		return "enter install  •  esc back"
	case viewSelectVersion, viewListVersions:
		return "↑/↓ navigate  •  enter switch  •  esc back"
	case viewSelectUninstall:
		return "↑/↓ navigate  •  enter uninstall  •  esc back"
	case viewResult:
		return "enter continue"
	default:
		return ""
	}
}

func (m model) getHelpText() string {
	return `🚀 NVS - Node Version Switcher

INTERACTIVE MODE
  Run 'nvs' without arguments to launch this TUI.

CLI COMMANDS
  nvs install <version>   Install a Node.js version
  nvs use <version>       Switch to a version
  nvs list                List installed versions
  nvs current             Show active version
  nvs uninstall <version> Remove a version
  nvs setup               Configure PATH

VERSION FORMATS
  18        Latest Node.js 18.x
  20.10.0   Specific version
  lts       Latest LTS version
  latest    Latest available

EXAMPLES
  nvs install 20
  nvs install lts
  nvs use 18

No admin privileges required! 🎉`
}

// =============================================================================
// ENTRY POINT
// =============================================================================

// RunInteractiveCLI starts the interactive TUI
func RunInteractiveCLI() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
