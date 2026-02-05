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
// STYLES - Modern Theme
// =============================================================================

var (
	// Color palette - Modern dark theme
	primaryColor   = lipgloss.Color("#8B5CF6") // Vibrant purple
	secondaryColor = lipgloss.Color("#06B6D4") // Cyan accent
	successColor   = lipgloss.Color("#22C55E") // Green
	errorColor     = lipgloss.Color("#EF4444") // Red
	warningColor   = lipgloss.Color("#FBBF24") // Amber
	mutedColor     = lipgloss.Color("#64748B") // Slate
	textColor      = lipgloss.Color("#F1F5F9") // Light text
	dimTextColor   = lipgloss.Color("#94A3B8") // Dimmed text
	highlightColor = lipgloss.Color("#C4B5FD") // Light purple
	bgAccent       = lipgloss.Color("#1E293B") // Dark background accent
	borderColor    = lipgloss.Color("#475569") // Border

	// Logo/Header style
	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	logoAccentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor)

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			Italic(true)

	// Main container box
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 3).
			MarginTop(1)

	// Menu item styles
	selectedStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(textColor)

	dimStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Message styles
	successMsgStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Help/footer style
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// Version highlight
	versionCurrentStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Bold(true)

	// Status bar style
	statusBarStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			Italic(true)

	// Accent badge style
	badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(primaryColor).
			Padding(0, 1).
			Bold(true)

	// Warning badge
	warningBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(warningColor).
				Padding(0, 1).
				Bold(true)
)

// ASCII Logo
func getLogo() string {
	n := logoStyle.Render("███╗   ██╗")
	v := logoAccentStyle.Render("██╗   ██╗")
	s := logoStyle.Render("███████╗")

	n2 := logoStyle.Render("████╗  ██║")
	v2 := logoAccentStyle.Render("██║   ██║")
	s2 := logoStyle.Render("██╔════╝")

	n3 := logoStyle.Render("██╔██╗ ██║")
	v3 := logoAccentStyle.Render("██║   ██║")
	s3 := logoStyle.Render("███████╗")

	n4 := logoStyle.Render("██║╚██╗██║")
	v4 := logoAccentStyle.Render("╚██╗ ██╔╝")
	s4 := logoStyle.Render("╚════██║")

	n5 := logoStyle.Render("██║ ╚████║")
	v5 := logoAccentStyle.Render(" ╚████╔╝ ")
	s5 := logoStyle.Render("███████║")

	n6 := logoStyle.Render("╚═╝  ╚═══╝")
	v6 := logoAccentStyle.Render("  ╚═══╝  ")
	s6 := logoStyle.Render("╚══════╝")

	return fmt.Sprintf("%s%s%s\n%s%s%s\n%s%s%s\n%s%s%s\n%s%s%s\n%s%s%s",
		n, v, s, n2, v2, s2, n3, v3, s3, n4, v4, s4, n5, v5, s5, n6, v6, s6)
}

// Compact header for sub-views
func getCompactHeader() string {
	return logoStyle.Render("NVS") + " " + dimStyle.Render("Node Version Switcher")
}

// =============================================================================
// TYPES
// =============================================================================

type viewState int

const (
	viewMainMenu viewState = iota
	viewInstallInput
	viewInstallOptions
	viewManageVersions
	viewSelectUninstall
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
	pendingVersion    string // Version to install (used in install options flow)
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
			{"📋", "List/Switch", "View installed versions and switch between them", "list"},
			{"🗑️ ", "Uninstall", "Remove an installed version", "uninstall"},
			{"🔧", "Setup", "Initialize NVS and configure PATH", "setup"},
			{"🔓", "Toggle TLS Skip", "Skip TLS verification (for VPN/proxy issues)", "tls"},
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
					m.pendingVersion = version
					m.state = viewInstallOptions
					m.cursor = 0
				}
				return m, nil
			default:
				// Pass all other keys to text input
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

		// Handle install options selection
		if m.state == viewInstallOptions {
			return m.handleInstallOptions(msg)
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
	case viewManageVersions, viewSelectUninstall:
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
			if m.state == viewManageVersions {
				m.state = viewProcessing
				m.processingMsg = fmt.Sprintf("Switching to %s...", version)
				return m, tea.Batch(m.spinner.Tick, m.useCmd(version))
			} else if m.state == viewSelectUninstall {
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
	if m.state == viewInstallOptions {
		m.state = viewInstallInput
		m.cursor = 0
		return m, nil
	}
	if m.state != viewMainMenu && m.state != viewProcessing {
		m.state = viewMainMenu
		m.cursor = 0
		m.textInput.Reset()
	}
	return m, nil
}

func (m model) handleInstallOptions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEsc:
		return m.goBack()
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < 1 {
			m.cursor++
		}
	case tea.KeyEnter:
		if m.cursor == 0 {
			// Normal install (secure)
			insecureMode = false
		} else {
			// Insecure install (skip TLS)
			insecureMode = true
		}
		m.state = viewProcessing
		m.processingMsg = fmt.Sprintf("Installing Node.js %s...", m.pendingVersion)
		if insecureMode {
			m.processingMsg += " (TLS skip enabled)"
		}
		return m, tea.Batch(m.spinner.Tick, m.installCmd(m.pendingVersion))
	default:
		switch msg.String() {
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "j":
			if m.cursor < 1 {
				m.cursor++
			}
		case " ":
			if m.cursor == 0 {
				insecureMode = false
			} else {
				insecureMode = true
			}
			m.state = viewProcessing
			m.processingMsg = fmt.Sprintf("Installing Node.js %s...", m.pendingVersion)
			if insecureMode {
				m.processingMsg += " (TLS skip enabled)"
			}
			return m, tea.Batch(m.spinner.Tick, m.installCmd(m.pendingVersion))
		}
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

	case "list":
		if len(m.installedVersions) == 0 {
			m.state = viewResult
			m.resultSuccess = false
			m.resultMsg = "No versions installed.\n\nUse 'Install Node.js' to get started."
			return m, nil
		}
		m.state = viewManageVersions
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

	case "tls":
		insecureMode = !insecureMode
		m.state = viewResult
		m.resultSuccess = true
		if insecureMode {
			m.resultMsg = "🔓 TLS verification DISABLED\n\nCertificate errors will be ignored.\nUse this if behind corporate VPN/proxy (Cato, Zscaler, etc.)"
		} else {
			m.resultMsg = "🔒 TLS verification ENABLED\n\nSecure mode restored."
		}
		return m, nil

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
		farewell := lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Render("Thanks for using NVS!")
		return fmt.Sprintf("\n  👋 %s\n\n", farewell)
	}

	var b strings.Builder

	// Header - Show full logo only on main menu
	b.WriteString("\n")
	if m.state == viewMainMenu {
		b.WriteString(getLogo())
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("  Node Version Switcher • No admin required"))
	} else {
		b.WriteString(getCompactHeader())
	}
	b.WriteString("\n")

	// Content
	switch m.state {
	case viewMainMenu:
		b.WriteString(m.renderMainMenu())
	case viewInstallInput:
		b.WriteString(m.renderInstallInput())
	case viewInstallOptions:
		b.WriteString(m.renderInstallOptions())
	case viewManageVersions:
		b.WriteString(m.renderVersionSelect("Installed versions (Enter to switch):", false))
	case viewSelectUninstall:
		b.WriteString(m.renderVersionSelect("Select version to uninstall:", true))
	case viewProcessing:
		b.WriteString(m.renderProcessing())
	case viewResult:
		b.WriteString(m.renderResult())
	}

	// Footer with key hints
	b.WriteString("\n")
	hints := m.getKeyHints()
	styledHints := helpStyle.Render("  " + hints)
	b.WriteString(styledHints)
	b.WriteString("\n")

	return b.String()
}

func (m model) renderMainMenu() string {
	var b strings.Builder

	b.WriteString("\n")

	for i, item := range m.menuItems {
		cursor := "   "
		style := normalStyle
		icon := dimStyle.Render(item.icon)

		if i == m.cursor {
			cursor = " ▸ "
			style = selectedStyle
			icon = item.icon // Full color when selected
		}

		b.WriteString(fmt.Sprintf("%s%s  %s\n", cursor, icon, style.Render(item.title)))

		// Show description for selected item
		if i == m.cursor {
			b.WriteString(fmt.Sprintf("       %s\n", dimStyle.Render(item.description)))
		}
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 44))
	b.WriteString("\n")

	// Version count badge
	versionCount := fmt.Sprintf(" %d installed ", len(m.installedVersions))
	if len(m.installedVersions) > 0 {
		b.WriteString(badgeStyle.Render(versionCount))
	} else {
		b.WriteString(dimStyle.Render("  No versions installed"))
	}

	// Current version
	if m.currentVersion != "" {
		b.WriteString("  ")
		b.WriteString(dimStyle.Render("Active: "))
		b.WriteString(versionCurrentStyle.Render(m.currentVersion))
	}

	// TLS warning badge
	if insecureMode {
		b.WriteString("  ")
		b.WriteString(warningBadgeStyle.Render(" TLS SKIP "))
	}

	return boxStyle.Render(b.String())
}

func (m model) renderInstallInput() string {
	var b strings.Builder

	// Section header
	header := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("📦 Install Node.js")

	b.WriteString(header)
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("Enter version:"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	// Examples in a nice format
	exampleStyle := lipgloss.NewStyle().Foreground(secondaryColor)
	b.WriteString(dimStyle.Render("  Examples: "))
	b.WriteString(exampleStyle.Render("22"))
	b.WriteString(dimStyle.Render(", "))
	b.WriteString(exampleStyle.Render("lts"))
	b.WriteString(dimStyle.Render(", "))
	b.WriteString(exampleStyle.Render("latest"))
	b.WriteString(dimStyle.Render(", "))
	b.WriteString(exampleStyle.Render("20.10.0"))

	return boxStyle.Render(b.String())
}

func (m model) renderInstallOptions() string {
	var b strings.Builder

	// Header with version badge
	header := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("📦 Install Node.js")

	versionBadge := badgeStyle.Render(fmt.Sprintf(" v%s ", m.pendingVersion))

	b.WriteString(header)
	b.WriteString("  ")
	b.WriteString(versionBadge)
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("Select connection mode:"))
	b.WriteString("\n\n")

	options := []struct {
		icon  string
		title string
		desc  string
	}{
		{"🔒", "Normal (Secure)", "Standard HTTPS with certificate verification"},
		{"🔓", "Skip TLS Verification", "For corporate VPN/proxy (Cato, Zscaler, etc.)"},
	}

	for i, opt := range options {
		cursor := "   "
		style := normalStyle
		icon := dimStyle.Render(opt.icon)

		if i == m.cursor {
			cursor = " ▸ "
			style = selectedStyle
			icon = opt.icon
		}

		b.WriteString(fmt.Sprintf("%s%s  %s\n", cursor, icon, style.Render(opt.title)))
		if i == m.cursor {
			b.WriteString(fmt.Sprintf("       %s\n", dimStyle.Render(opt.desc)))
		}
	}

	return boxStyle.Render(b.String())
}

func (m model) renderVersionSelect(title string, isDanger bool) string {
	var b strings.Builder

	// Header icon based on mode
	headerIcon := "📋"
	if isDanger {
		headerIcon = "🗑️"
	}

	header := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render(fmt.Sprintf("%s %s", headerIcon, title))

	b.WriteString(header)
	b.WriteString("\n\n")

	if len(m.installedVersions) == 0 {
		b.WriteString(dimStyle.Render("  No versions installed"))
	} else {
		for i, v := range m.installedVersions {
			cursor := "   "
			style := normalStyle
			suffix := ""
			icon := dimStyle.Render("○")

			if i == m.cursor {
				cursor = " ▸ "
				icon = "●"
				if isDanger {
					style = lipgloss.NewStyle().Foreground(errorColor).Bold(true)
					icon = lipgloss.NewStyle().Foreground(errorColor).Render("●")
				} else {
					style = selectedStyle
					icon = lipgloss.NewStyle().Foreground(successColor).Render("●")
				}
			}

			if v == m.currentVersion {
				suffix = " ★ current"
				if i == m.cursor && !isDanger {
					style = versionCurrentStyle
				}
			}

			b.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, icon, style.Render(v), dimStyle.Render(suffix)))
		}
	}

	return boxStyle.Render(b.String())
}

func (m model) renderProcessing() string {
	var b strings.Builder

	// Animated spinner with message
	spinnerView := lipgloss.NewStyle().
		Foreground(secondaryColor).
		Render(m.spinner.View())

	msgStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Bold(true)

	b.WriteString("\n")
	b.WriteString(spinnerView)
	b.WriteString("  ")
	b.WriteString(msgStyle.Render(m.processingMsg))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Please wait..."))
	b.WriteString("\n")

	return boxStyle.Render(b.String())
}

func (m model) renderResult() string {
	var b strings.Builder

	// Icon and style based on result
	icon := "✓"
	style := successMsgStyle
	if !m.resultSuccess {
		icon = "✗"
		style = errorMsgStyle
	}

	iconStyle := style.Copy().Bold(true)

	b.WriteString("\n")
	b.WriteString(iconStyle.Render(icon))
	b.WriteString("  ")
	b.WriteString(style.Render(m.resultMsg))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Press Enter to continue..."))
	b.WriteString("\n")

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
	keyStyle := lipgloss.NewStyle().Foreground(secondaryColor)
	sepStyle := lipgloss.NewStyle().Foreground(mutedColor)

	sep := sepStyle.Render(" │ ")

	switch m.state {
	case viewMainMenu:
		return keyStyle.Render("↑↓") + " navigate" + sep + keyStyle.Render("⏎") + " select" + sep + keyStyle.Render("q") + " quit"
	case viewInstallInput:
		return keyStyle.Render("⏎") + " continue" + sep + keyStyle.Render("esc") + " back"
	case viewInstallOptions:
		return keyStyle.Render("↑↓") + " navigate" + sep + keyStyle.Render("⏎") + " install" + sep + keyStyle.Render("esc") + " back"
	case viewManageVersions:
		return keyStyle.Render("↑↓") + " navigate" + sep + keyStyle.Render("⏎") + " switch" + sep + keyStyle.Render("esc") + " back"
	case viewSelectUninstall:
		return keyStyle.Render("↑↓") + " navigate" + sep + keyStyle.Render("⏎") + " uninstall" + sep + keyStyle.Render("esc") + " back"
	case viewResult:
		return keyStyle.Render("⏎") + " continue"
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
