package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// InstallationState represents the current state of NVS installation
type InstallationState int

const (
	StateNotInstalled InstallationState = iota
	StateInstalled
	StateOutdated
	StateCorrupted
)

// Version is set during build time
var Version = "dev"

// InstallationDetector handles detection of existing NVS installations
type InstallationDetector struct {
	HomeDir     string
	NVSDir      string
	BinDir      string
	CLIPath     string
	UIPath      string
	VersionFile string
}

func NewInstallationDetector() *InstallationDetector {
	homeDir := getHomeDir()
	nvsDir := filepath.Join(homeDir, NVS_DIR_NAME)
	binDir := filepath.Join(nvsDir, "bin")

	cliName := "nvs"
	uiName := "nvs-ui"
	if runtime.GOOS == "windows" {
		cliName += ".exe"
		uiName += ".exe"
	}

	return &InstallationDetector{
		HomeDir:     homeDir,
		NVSDir:      nvsDir,
		BinDir:      binDir,
		CLIPath:     filepath.Join(binDir, cliName),
		UIPath:      filepath.Join(binDir, uiName),
		VersionFile: filepath.Join(nvsDir, "version"),
	}
}

// DetectInstallation returns the current installation state and version info
func (d *InstallationDetector) DetectInstallation() (InstallationState, string, error) {
	// Check if NVS directory exists
	if _, err := os.Stat(d.NVSDir); os.IsNotExist(err) {
		return StateNotInstalled, "", nil
	}

	// Check if binaries exist
	cliExists := d.fileExists(d.CLIPath)
	uiExists := d.fileExists(d.UIPath)

	if !cliExists && !uiExists {
		return StateCorrupted, "", fmt.Errorf("NVS directory exists but no binaries found")
	}

	// Check version
	installedVersion, err := d.getInstalledVersion()
	if err != nil {
		// If we can't read version but binaries exist, consider it corrupted
		return StateCorrupted, "", fmt.Errorf("cannot read version: %w", err)
	}

	// Compare versions
	if installedVersion != Version && Version != "dev" {
		return StateOutdated, installedVersion, nil
	}

	return StateInstalled, installedVersion, nil
}

// fileExists checks if a file exists and is not a directory
func (d *InstallationDetector) fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// getInstalledVersion reads the version from the installed NVS
func (d *InstallationDetector) getInstalledVersion() (string, error) {
	// Try to read version file first
	if content, err := os.ReadFile(d.VersionFile); err == nil {
		return string(content), nil
	}

	// If version file doesn't exist, try to execute the CLI to get version
	// This is a fallback for older installations
	if d.fileExists(d.CLIPath) {
		// For now, return "unknown" - we could implement exec version check later
		return "unknown", nil
	}

	return "", fmt.Errorf("no version information available")
}

// writeVersion writes the current version to the version file
func (d *InstallationDetector) writeVersion() error {
	if err := os.MkdirAll(d.NVSDir, 0755); err != nil {
		return fmt.Errorf("failed to create NVS directory: %w", err)
	}

	return os.WriteFile(d.VersionFile, []byte(Version), 0644)
}

// GetInstallationInfo returns formatted information about the installation
func (d *InstallationDetector) GetInstallationInfo() (state InstallationState, version string, details string) {
	state, version, err := d.DetectInstallation()

	switch state {
	case StateNotInstalled:
		details = "NVS is not installed on this system"
	case StateInstalled:
		details = fmt.Sprintf("NVS is installed (version %s)", version)
		if d.fileExists(d.CLIPath) && d.fileExists(d.UIPath) {
			details += " with both CLI and GUI components"
		} else if d.fileExists(d.CLIPath) {
			details += " with CLI component only"
		} else if d.fileExists(d.UIPath) {
			details += " with GUI component only"
		}
	case StateOutdated:
		details = fmt.Sprintf("NVS is installed but outdated (installed: %s, available: %s)", version, Version)
	case StateCorrupted:
		details = "NVS installation appears to be corrupted"
		if err != nil {
			details += fmt.Sprintf(": %v", err)
		}
	}

	return state, version, details
}

// HasCLI returns true if CLI binary exists
func (d *InstallationDetector) HasCLI() bool {
	return d.fileExists(d.CLIPath)
}

// HasUI returns true if UI binary exists
func (d *InstallationDetector) HasUI() bool {
	return d.fileExists(d.UIPath)
}

// RemoveInstallation completely removes NVS installation
func (d *InstallationDetector) RemoveInstallation() error {
	if _, err := os.Stat(d.NVSDir); os.IsNotExist(err) {
		return nil // Already removed
	}

	return os.RemoveAll(d.NVSDir)
}
