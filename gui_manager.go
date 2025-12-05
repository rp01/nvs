package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type NVSManager struct {
	app               fyne.App
	window            fyne.Window
	nvs               *NodeVersionSwitcher
	versionList       *widget.List
	currentLabel      *widget.Label
	installEntry      *widget.Entry
	statusLabel       *widget.Label
	refreshBtn        *widget.Button
	installBtn        *widget.Button
	uninstallBtn      *widget.Button
	installedVersions []string
}

func NewNVSManager(app fyne.App) *NVSManager {
	window := app.NewWindow("NVS Manager - Node Version Switcher")
	window.Resize(fyne.NewSize(600, 500))
	window.CenterOnScreen()

	manager := &NVSManager{
		app:    app,
		window: window,
		nvs:    NewNodeVersionSwitcher(),
	}

	manager.setupManagerUI()
	manager.refreshVersions()
	return manager
}

func (mgr *NVSManager) setupManagerUI() {
	// Header
	title := widget.NewLabel("🎛️ NVS Manager")
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Current version display
	mgr.currentLabel = widget.NewLabel("Current: Not set")
	mgr.currentLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Version installation section
	installLabel := widget.NewLabel("Install New Version:")
	mgr.installEntry = widget.NewEntry()
	mgr.installEntry.SetPlaceHolder("e.g., 18, lts, latest, 16.14.0")

	mgr.installBtn = widget.NewButton("📥 Install", func() {
		mgr.handleInstall("")
	})
	mgr.installBtn.Importance = widget.HighImportance

	installContainer := container.NewBorder(nil, nil, installLabel, mgr.installBtn, mgr.installEntry)

	// Installed versions list
	mgr.versionList = widget.NewList(
		func() int { return len(mgr.installedVersions) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("template"),
				widget.NewButton("Use", nil),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(mgr.installedVersions) {
				return
			}

			version := mgr.installedVersions[id]
			hbox := obj.(*fyne.Container)
			label := hbox.Objects[0].(*widget.Label)
			useBtn := hbox.Objects[1].(*widget.Button) // Check if this is the current version
			current := mgr.getCurrentVersion()
			if version == current {
				label.SetText("➤ " + version + " (current)")
				label.TextStyle = fyne.TextStyle{Bold: true}
				useBtn.Disable()
			} else {
				label.SetText("   " + version)
				label.TextStyle = fyne.TextStyle{}
				useBtn.Enable()
				useBtn.OnTapped = func() {
					mgr.handleUse(version)
				}
			}
		},
	)

	// Control buttons
	mgr.refreshBtn = widget.NewButton("🔄 Refresh", mgr.refreshVersions)
	mgr.uninstallBtn = widget.NewButton("🗑️ Uninstall Selected", mgr.handleUninstall)
	mgr.uninstallBtn.Importance = widget.DangerImportance

	helpBtn := widget.NewButton("❓ Help", mgr.showHelp)
	closeBtn := widget.NewButton("❌ Close", func() {
		mgr.window.Hide()
	})

	buttonContainer := container.NewGridWithColumns(4,
		mgr.refreshBtn,
		mgr.uninstallBtn,
		helpBtn,
		closeBtn,
	)

	// Status bar
	mgr.statusLabel = widget.NewLabel("Ready")
	mgr.statusLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Main layout
	content := container.NewVBox(
		container.NewCenter(title),
		widget.NewSeparator(),
		mgr.currentLabel,
		widget.NewSeparator(),
		widget.NewLabel("📦 Installed Versions:"),
		container.NewBorder(nil, nil, nil, nil, mgr.versionList),
		widget.NewSeparator(),
		installContainer,
		widget.NewSeparator(),
		buttonContainer,
		widget.NewSeparator(),
		mgr.statusLabel,
	)

	mgr.window.SetContent(container.NewPadded(content))
}

func (mgr *NVSManager) refreshVersions() {
	mgr.statusLabel.SetText("🔄 Refreshing versions...")
	mgr.installedVersions = []string{}

	// Read versions directory
	files, err := os.ReadDir(mgr.nvs.VersionsDir)
	if err != nil {
		mgr.statusLabel.SetText("❌ Error reading versions directory")
		return
	}

	for _, file := range files {
		if file.IsDir() {
			mgr.installedVersions = append(mgr.installedVersions, file.Name())
		}
	}

	// Update current version display
	current := mgr.getCurrentVersion()
	if current != "" {
		mgr.currentLabel.SetText(fmt.Sprintf("Current: %s", current))
	} else {
		mgr.currentLabel.SetText("Current: None selected")
	}

	mgr.versionList.Refresh()
	mgr.statusLabel.SetText(fmt.Sprintf("✅ Found %d installed versions", len(mgr.installedVersions)))
}

func (mgr *NVSManager) getCurrentVersion() string {
	// Check what the current symlink points to
	target, err := filepath.EvalSymlinks(mgr.nvs.CurrentLink)
	if err != nil {
		return ""
	}

	// Extract version from path
	if strings.Contains(target, mgr.nvs.VersionsDir) {
		return filepath.Base(target)
	}

	return ""
}

func (mgr *NVSManager) handleInstall(version string) {
	if version == "" {
		version = strings.TrimSpace(mgr.installEntry.Text)
	}

	if version == "" {
		dialog.ShowError(fmt.Errorf("please enter a version to install"), mgr.window)
		return
	}

	mgr.installBtn.Disable()
	mgr.statusLabel.SetText(fmt.Sprintf("📥 Installing %s...", version))

	go func() {
		defer func() {
			mgr.installBtn.Enable()
		}()

		if err := mgr.nvs.Install(version); err != nil {
			mgr.statusLabel.SetText(fmt.Sprintf("❌ Installation failed: %v", err))
			dialog.ShowError(fmt.Errorf("failed to install %s: %w", version, err), mgr.window)
			return
		}

		mgr.statusLabel.SetText(fmt.Sprintf("✅ Successfully installed %s", version))
		mgr.installEntry.SetText("")
		mgr.refreshVersions()

		// Ask if user wants to use this version
		dialog.ShowConfirm("Installation Complete",
			fmt.Sprintf("Successfully installed %s. Would you like to use it now?", version),
			func(yes bool) {
				if yes {
					mgr.handleUse(version)
				}
			}, mgr.window)
	}()
}

func (mgr *NVSManager) handleUse(version string) {
	mgr.statusLabel.SetText(fmt.Sprintf("🔄 Switching to %s...", version))

	go func() {
		// Remove 'v' prefix if present and find the actual installed version
		cleanVersion := strings.TrimPrefix(version, "v")

		if err := mgr.nvs.Use(cleanVersion); err != nil {
			mgr.statusLabel.SetText(fmt.Sprintf("❌ Switch failed: %v", err))
			dialog.ShowError(fmt.Errorf("failed to use %s: %w", version, err), mgr.window)
			return
		}

		mgr.statusLabel.SetText(fmt.Sprintf("✅ Now using %s", version))
		mgr.refreshVersions()
	}()
}

func (mgr *NVSManager) handleUninstall() {
	// For now, show a dialog to select version since list selection API changed
	if len(mgr.installedVersions) == 0 {
		dialog.ShowError(fmt.Errorf("no versions installed"), mgr.window)
		return
	}

	// Create selection dialog
	items := make([]string, len(mgr.installedVersions))
	copy(items, mgr.installedVersions)

	selected := widget.NewSelect(items, func(version string) {
		if version != "" {
			// Confirm deletion
			dialog.ShowConfirm("Confirm Uninstall",
				fmt.Sprintf("Are you sure you want to uninstall %s?\nThis action cannot be undone.", version),
				func(confirmed bool) {
					if confirmed {
						mgr.performUninstall(version)
					}
				}, mgr.window)
		}
	})

	content := container.NewVBox(
		widget.NewLabel("Select version to uninstall:"),
		selected,
	)

	dialog.ShowCustom("Uninstall Version", "Cancel", content, mgr.window)
}

func (mgr *NVSManager) performUninstall(version string) {
	mgr.statusLabel.SetText(fmt.Sprintf("🗑️ Uninstalling %s...", version))

	versionPath := filepath.Join(mgr.nvs.VersionsDir, version)

	// Check if this is the current version
	current := mgr.getCurrentVersion()
	if version == current {
		// Remove current symlink
		os.Remove(mgr.nvs.CurrentLink)
	}

	// Remove version directory
	if err := os.RemoveAll(versionPath); err != nil {
		mgr.statusLabel.SetText(fmt.Sprintf("❌ Uninstall failed: %v", err))
		dialog.ShowError(fmt.Errorf("failed to uninstall %s: %w", version, err), mgr.window)
		return
	}

	mgr.statusLabel.SetText(fmt.Sprintf("✅ Successfully uninstalled %s", version))
	mgr.refreshVersions()
}

func (mgr *NVSManager) showHelp() {
	helpText := `# NVS Manager Help

## Installing Node.js Versions
Enter any of these in the install field:
- **18** - Latest Node 18.x version
- **lts** - Latest LTS (Long Term Support) version  
- **latest** - Latest available version
- **16.14.0** - Specific version number

## Managing Versions
- **Use** - Switch to a specific version
- **Uninstall** - Remove a version (requires confirmation)
- **Refresh** - Update the version list

## Command Line Usage
You can also use NVS from the terminal:
- ` + "`nvs install 18`" + ` - Install Node 18.x
- ` + "`nvs use 18`" + ` - Switch to Node 18.x  
- ` + "`nvs list`" + ` - List installed versions

## Tips
- The current version is marked with ➤
- Versions are installed in isolated directories
- No admin privileges required
- Works across Windows, macOS, and Linux`

	helpDialog := dialog.NewCustom("NVS Manager Help", "Close",
		container.NewScroll(widget.NewRichTextFromMarkdown(helpText)),
		mgr.window)
	helpDialog.Resize(fyne.NewSize(500, 400))
	helpDialog.Show()
}

func (mgr *NVSManager) Show() {
	mgr.window.Show()
}
