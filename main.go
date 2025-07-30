package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/mholt/archives"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// NodeRelease represents a Node.js release
type NodeRelease struct {
	Version  string
	URL      string
	Filename string
	Ext      string
}

// NodeVersionSwitcher manages Node.js versions
type NodeVersionSwitcher struct {
	HomeDir     string
	NVSDir      string
	VersionsDir string
	CurrentFile string
	BinDir      string
}

// NewNodeVersionSwitcher creates a new instance
func NewNodeVersionSwitcher() *NodeVersionSwitcher {
	homeDir := getHomeDir()
	nvsDir := filepath.Join(homeDir, ".nvs")

	return &NodeVersionSwitcher{
		HomeDir:     homeDir,
		NVSDir:      nvsDir,
		VersionsDir: filepath.Join(nvsDir, "versions"),
		CurrentFile: filepath.Join(nvsDir, "current"),
		BinDir:      filepath.Join(nvsDir, "bin"),
	}
}

// getHomeDir returns the user's home directory
func getHomeDir() string {
	if homeDir := os.Getenv("HOME"); homeDir != "" {
		return homeDir
	}
	if homeDir := os.Getenv("USERPROFILE"); homeDir != "" {
		return homeDir
	}
	return ""
}

// Init initializes the NVS directory structure
func (nvs *NodeVersionSwitcher) Init() error {
	dirs := []string{nvs.NVSDir, nvs.VersionsDir, nvs.BinDir}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nvs.installSelf()
}

// installSelf copies the current executable to the bin directory
func (nvs *NodeVersionSwitcher) installSelf() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	targetName := "nvs"
	if runtime.GOOS == "windows" {
		targetName = "nvs.exe"
	}

	targetPath := filepath.Join(nvs.BinDir, targetName)

	// Check if already installed
	if _, err := os.Stat(targetPath); err == nil {
		return nil
	}

	fmt.Println("Installing NVS to ~/.nvs/bin/")

	source, err := os.Open(executable)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer source.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target: %w", err)
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	fmt.Printf("NVS installed to: %s\n", targetPath)

	// Attempt automatic PATH setup
	if err := nvs.setupInitialPath(); err != nil {
		fmt.Printf("Add %s to your PATH to use 'nvs' command globally\n", nvs.BinDir)
		fmt.Printf("Or run: nvs setup (for detailed instructions)\n")
	} else {
		fmt.Printf("✅ NVS has been added to your PATH automatically!\n")
		fmt.Printf("🔄 Restart your terminal or run the appropriate source command to use 'nvs'\n")
	}

	return nil
}

// setupInitialPath attempts to add NVS bin directory to PATH automatically
func (nvs *NodeVersionSwitcher) setupInitialPath() error {
	fmt.Println("🔧 Attempting automatic PATH setup...")

	if runtime.GOOS == "windows" {
		// Detect if running in Git Bash or similar Unix-like environment on Windows
		isGitBash := os.Getenv("MSYSTEM") != "" || os.Getenv("TERM") != "" || strings.Contains(strings.ToLower(os.Getenv("SHELL")), "bash")

		if isGitBash {
			// Try to append to .bashrc
			bashrcPath := filepath.Join(os.Getenv("HOME"), ".bashrc")
			exportLine := fmt.Sprintf("export PATH=\"%s:$PATH\"", nvs.BinDir)

			if err := nvs.appendToFile(bashrcPath, exportLine); err != nil {
				return fmt.Errorf("could not update .bashrc: %w", err)
			}
			fmt.Printf("✅ Updated ~/.bashrc\n")
			fmt.Printf("   Restart your Git Bash or run: source ~/.bashrc\n")
		} else {
			return fmt.Errorf("automatic setup not supported for Command Prompt - use 'nvs setup' for manual instructions")
		}
	} else {
		// Unix-like systems
		shell := os.Getenv("SHELL")
		configFile := ".bashrc"
		if strings.Contains(shell, "zsh") {
			configFile = ".zshrc"
		}

		configPath := filepath.Join(os.Getenv("HOME"), configFile)
		exportLine := fmt.Sprintf("export PATH=\"%s:$PATH\"", nvs.BinDir)

		if err := nvs.appendToFile(configPath, exportLine); err != nil {
			return fmt.Errorf("could not update %s: %w", configFile, err)
		}
		fmt.Printf("✅ Updated ~/%s\n", configFile)
		fmt.Printf("   Restart your terminal or run: source ~/%s\n", configFile)
	}

	return nil
}

// getNodeRelease generates Node.js release information
func (nvs *NodeVersionSwitcher) getNodeRelease(version, targetOS, targetArch string) (*NodeRelease, error) {
	platform := targetOS
	if platform == "" {
		platform = runtime.GOOS
	}

	arch := targetArch
	if arch == "" {
		arch = runtime.GOARCH
	}

	var platformName, ext string

	switch platform {
	case "windows", "win":
		platformName = "win"
		ext = "zip"
	case "darwin", "macos":
		platformName = "darwin"
		ext = "tar.gz"
	case "linux":
		platformName = "linux"
		ext = "tar.xz"
	default:
		return nil, fmt.Errorf("unsupported platform: %s. Supported: windows, darwin, linux", platform)
	}

	// Normalize architecture names
	var archName string
	switch arch {
	case "x86_64", "amd64":
		archName = "x64"
	case "aarch64", "arm64":
		archName = "arm64"
	case "x86", "i386", "ia32", "386":
		archName = "x86"
	default:
		archName = arch
	}

	filename := fmt.Sprintf("node-v%s-%s-%s.%s", version, platformName, archName, ext)
	url := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", version, filename)

	return &NodeRelease{
		Version:  version,
		URL:      url,
		Filename: filename,
		Ext:      ext,
	}, nil
}

// downloadWithProgress downloads a file with progress bar
func (nvs *NodeVersionSwitcher) downloadWithProgress(url, destination string) error {
	fmt.Printf("Downloading from: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %d %s", resp.StatusCode, resp.Status)
	}

	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	var bar *progressbar.ProgressBar
	if resp.ContentLength > 0 {
		bar = progressbar.DefaultBytes(resp.ContentLength, "downloading")
	} else {
		bar = progressbar.DefaultBytes(-1, "downloading")
	}

	_, err = io.Copy(io.MultiWriter(file, bar), resp.Body)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	fmt.Println("\nDownload completed!")
	return nil
}

// extractArchive extracts various archive formats using the archives library
func (nvs *NodeVersionSwitcher) extractArchive(archivePath, extractPath, ext string) error {
	fmt.Println("Extracting archive...")

	if err := os.MkdirAll(extractPath, 0755); err != nil {
		return fmt.Errorf("failed to create extract directory: %w", err)
	}

	// Open the archive file
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	// Identify the format
	format, stream, err := archives.Identify(context.Background(), archivePath, file)
	if err != nil {
		return fmt.Errorf("failed to identify archive format: %w", err)
	}

	// Check if it's an extractor
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("format does not support extraction: %s", ext)
	}

	// Extract the archive
	err = extractor.Extract(context.Background(), stream, func(ctx context.Context, f archives.FileInfo) error {
		// Get the destination path
		destPath := filepath.Join(extractPath, f.NameInArchive)

		// Security check: prevent directory traversal
		if !strings.HasPrefix(destPath, filepath.Clean(extractPath)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", f.NameInArchive)
		}

		if f.IsDir() {
			return os.MkdirAll(destPath, f.Mode())
		}

		// Create directory for the file
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Open the file from the archive
		fileReader, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open file from archive: %w", err)
		}
		defer fileReader.Close()

		// Create the destination file
		destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create destination file: %w", err)
		}
		defer destFile.Close()

		// Copy file contents
		_, err = io.Copy(destFile, fileReader)
		if err != nil {
			return fmt.Errorf("failed to copy file contents: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Clean up archive
	os.Remove(archivePath)
	fmt.Println("Extraction completed")
	return nil
}

// Install installs a Node.js version
func (nvs *NodeVersionSwitcher) Install(version, targetOS, targetArch string) error {
	osInfo := ""
	if targetOS != "" {
		osInfo = fmt.Sprintf(" for %s", targetOS)
	}
	archInfo := ""
	if targetArch != "" {
		archInfo = fmt.Sprintf("-%s", targetArch)
	}

	fmt.Printf("Installing Node.js v%s%s%s...\n", version, osInfo, archInfo)

	// Create unique directory name for cross-platform installs
	versionKey := version
	if targetOS != "" || targetArch != "" {
		os := targetOS
		if os == "" {
			os = runtime.GOOS
		}
		arch := targetArch
		if arch == "" {
			arch = runtime.GOARCH
		}
		versionKey = fmt.Sprintf("%s-%s-%s", version, os, arch)
	}

	versionDir := filepath.Join(nvs.VersionsDir, versionKey)

	if _, err := os.Stat(versionDir); err == nil {
		fmt.Printf("Node.js v%s%s%s is already installed\n", version, osInfo, archInfo)
		return nil
	}

	release, err := nvs.getNodeRelease(version, targetOS, targetArch)
	if err != nil {
		return err
	}

	downloadPath := filepath.Join(nvs.NVSDir, release.Filename)

	// Download
	if err := nvs.downloadWithProgress(release.URL, downloadPath); err != nil {
		return err
	}

	// Extract
	if err := nvs.extractArchive(downloadPath, versionDir, release.Ext); err != nil {
		// Cleanup on failure
		os.RemoveAll(versionDir)
		os.Remove(downloadPath)
		return err
	}

	// Reorganize extracted files
	entries, err := os.ReadDir(versionDir)
	if err != nil {
		return fmt.Errorf("failed to read version directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "node-") {
			extractedPath := filepath.Join(versionDir, entry.Name())

			// Move contents up one level
			subEntries, err := os.ReadDir(extractedPath)
			if err != nil {
				return fmt.Errorf("failed to read extracted directory: %w", err)
			}

			for _, subEntry := range subEntries {
				src := filepath.Join(extractedPath, subEntry.Name())
				dest := filepath.Join(versionDir, subEntry.Name())
				if err := os.Rename(src, dest); err != nil {
					return fmt.Errorf("failed to move file: %w", err)
				}
			}

			os.Remove(extractedPath)
			break
		}
	}

	fmt.Printf("Node.js v%s%s%s installed successfully\n", version, osInfo, archInfo)
	fmt.Printf("📁 Installed to: %s\n", versionDir)

	// Fix npm/npx symlinks on Unix systems
	if runtime.GOOS != "windows" {
		if err := nvs.fixNpmSymlinks(versionDir); err != nil {
			fmt.Printf("⚠️  Warning: Could not fix npm/npx symlinks: %v\n", err)
		}
	}

	if targetOS != "" && targetOS != runtime.GOOS {
		fmt.Printf("⚠️  Note: This is a cross-platform installation for %s\n", targetOS)
	}

	return nil
}

// fixNpmSymlinks fixes npm/npx symlinks that may have been extracted incorrectly
func (nvs *NodeVersionSwitcher) fixNpmSymlinks(versionDir string) error {
	binDir := filepath.Join(versionDir, "bin")
	npmLibBin := filepath.Join(versionDir, "lib", "node_modules", "npm", "bin")

	// Check if npm lib directory exists
	if _, err := os.Stat(npmLibBin); err != nil {
		return nil // No npm to fix
	}

	// Fix npm
	npmBin := filepath.Join(binDir, "npm")
	npmTarget := filepath.Join(npmLibBin, "npm-cli.js")

	if stat, err := os.Stat(npmBin); err == nil && stat.Size() == 0 {
		os.Remove(npmBin)
		// Create a shell script wrapper to npm-cli.js
		npmScript := fmt.Sprintf("#!/bin/sh\nexec \"%s\" \"$@\"\n", npmTarget)
		if err := os.WriteFile(npmBin, []byte(npmScript), 0755); err != nil {
			return fmt.Errorf("failed to create npm script: %w", err)
		}
	}

	// Fix npx
	npxBin := filepath.Join(binDir, "npx")
	npxTarget := filepath.Join(npmLibBin, "npx-cli.js")

	if stat, err := os.Stat(npxBin); err == nil && stat.Size() == 0 {
		os.Remove(npxBin)
		// Create a shell script wrapper to npx-cli.js
		npxScript := fmt.Sprintf("#!/bin/sh\nexec \"%s\" \"$@\"\n", npxTarget)
		if err := os.WriteFile(npxBin, []byte(npxScript), 0755); err != nil {
			return fmt.Errorf("failed to create npx script: %w", err)
		}
	}

	return nil
}

// Use switches to a specific Node.js version
func (nvs *NodeVersionSwitcher) Use(version, targetOS, targetArch string, global bool) error {
	// Create version key for lookup
	versionKey := version
	if targetOS != "" || targetArch != "" {
		os := targetOS
		if os == "" {
			os = runtime.GOOS
		}
		arch := targetArch
		if arch == "" {
			arch = runtime.GOARCH
		}
		versionKey = fmt.Sprintf("%s-%s-%s", version, os, arch)
	}

	versionDir := filepath.Join(nvs.VersionsDir, versionKey)

	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		fmt.Printf("❌ Node.js v%s is not installed.\n", version)
		if targetOS != "" || targetArch != "" {
			osInfo := ""
			if targetOS != "" {
				osInfo = fmt.Sprintf(" --os %s", targetOS)
			}
			archInfo := ""
			if targetArch != "" {
				archInfo = fmt.Sprintf(" --arch %s", targetArch)
			}
			fmt.Printf("Run 'nvs install %s%s%s' first.\n", version, osInfo, archInfo)
		} else {
			fmt.Printf("Run 'nvs install %s' first.\n", version)
		}
		return nil
	}

	// Check if this is a cross-platform installation
	if targetOS != "" && targetOS != runtime.GOOS {
		fmt.Printf("⚠️  Warning: You're trying to use %s binaries on %s\n", targetOS, runtime.GOOS)
		fmt.Println("   This will likely not work. Consider installing for your current platform.")
	}

	binPath := versionDir
	if runtime.GOOS != "windows" {
		binPath = filepath.Join(versionDir, "bin")
	}

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		fmt.Printf("❌ Invalid Node.js installation for v%s\n", version)
		return nil
	}

	// Save current version
	if err := os.WriteFile(nvs.CurrentFile, []byte(versionKey), 0644); err != nil {
		return fmt.Errorf("failed to save current version: %w", err)
	}

	// Create version links
	if err := nvs.createVersionLinks(versionKey, binPath); err != nil {
		return err
	}

	fmt.Printf("✅ Switched to Node.js v%s\n", version)
	if targetOS != "" || targetArch != "" {
		platform := targetOS
		if platform == "" {
			platform = runtime.GOOS
		}
		arch := targetArch
		if arch == "" {
			arch = runtime.GOARCH
		}
		fmt.Printf("   Platform: %s-%s\n", platform, arch)
	}

	if global {
		// Global installation - update shell configuration files
		return nvs.setupGlobalEnvironment(binPath)
	} else {
		// Local session - just show the export command and set in current process
		fmt.Printf("\n📍 Node.js binaries available at: %s\n", binPath)
		return nvs.setLocalEnvironment(binPath)
	}
}

// setLocalEnvironment sets up the environment for the current session
func (nvs *NodeVersionSwitcher) setLocalEnvironment(binPath string) error {
	currentBinPath := filepath.Join(nvs.NVSDir, "current-bin")

	// Set environment for current process (this affects child processes)
	currentPath := os.Getenv("PATH")
	var newPath string

	if runtime.GOOS == "windows" {
		newPath = fmt.Sprintf("%s;%s;%s", currentBinPath, nvs.BinDir, currentPath)
	} else {
		newPath = fmt.Sprintf("%s:%s:%s", currentBinPath, nvs.BinDir, currentPath)
	}

	os.Setenv("PATH", newPath)

	fmt.Printf("\n🔧 Environment set for current session!\n")
	fmt.Printf("   You can now use: node, npm, npx\n")

	// Also show the export command for manual use in other terminals
	if runtime.GOOS == "windows" {
		isGitBash := os.Getenv("MSYSTEM") != "" || os.Getenv("TERM") != "" || strings.Contains(strings.ToLower(os.Getenv("SHELL")), "bash")
		if isGitBash {
			fmt.Printf("\n💡 To use in other Git Bash sessions:\n")
			fmt.Printf("   export PATH=\"%s:$PATH\"\n", binPath)
		} else {
			fmt.Printf("\n💡 To use in other Command Prompt sessions:\n")
			fmt.Printf("   set PATH=%s;%%PATH%%\n", binPath)
		}
	} else {
		fmt.Printf("\n� To use in other terminal sessions:\n")
		fmt.Printf("   export PATH=\"%s:$PATH\"\n", binPath)
	}

	fmt.Printf("\n� For permanent setup across all sessions, use: nvs use %s --global\n", getCurrentVersionFromPath(binPath))

	return nil
}

// setupGlobalEnvironment sets up permanent global environment
func (nvs *NodeVersionSwitcher) setupGlobalEnvironment(binPath string) error {
	currentBinPath := filepath.Join(nvs.NVSDir, "current-bin")

	fmt.Printf("\n🌍 Setting up global environment...\n")

	if runtime.GOOS == "windows" {
		isGitBash := os.Getenv("MSYSTEM") != "" || os.Getenv("TERM") != "" || strings.Contains(strings.ToLower(os.Getenv("SHELL")), "bash")

		if isGitBash {
			// Try to append to .bashrc
			bashrcPath := filepath.Join(os.Getenv("HOME"), ".bashrc")
			exportLine := fmt.Sprintf("export PATH=\"%s:%s:$PATH\"", currentBinPath, nvs.BinDir)

			if err := nvs.appendToFile(bashrcPath, exportLine); err != nil {
				fmt.Printf("⚠️  Could not automatically update .bashrc: %v\n", err)
				fmt.Printf("   Please manually add: %s\n", exportLine)
			} else {
				fmt.Printf("✅ Updated ~/.bashrc\n")
				fmt.Printf("   Restart your terminal or run: source ~/.bashrc\n")
			}
		} else {
			fmt.Printf("⚠️  Automatic global setup not supported for Command Prompt\n")
			fmt.Printf("   Please manually add to your PATH environment variable:\n")
			fmt.Printf("   %s;%s\n", currentBinPath, nvs.BinDir)
		}
	} else {
		// Unix-like systems
		shell := os.Getenv("SHELL")
		configFile := ".bashrc"
		if strings.Contains(shell, "zsh") {
			configFile = ".zshrc"
		}

		configPath := filepath.Join(os.Getenv("HOME"), configFile)
		exportLine := fmt.Sprintf("export PATH=\"%s:%s:$PATH\"", currentBinPath, nvs.BinDir)

		if err := nvs.appendToFile(configPath, exportLine); err != nil {
			fmt.Printf("⚠️  Could not automatically update %s: %v\n", configFile, err)
			fmt.Printf("   Please manually add: %s\n", exportLine)
		} else {
			fmt.Printf("✅ Updated ~/%s\n", configFile)
			fmt.Printf("   Restart your terminal or run: source ~/%s\n", configFile)
		}
	}

	return nil
}

// appendToFile appends a line to a file if it doesn't already exist
func (nvs *NodeVersionSwitcher) appendToFile(filePath, line string) error {
	// Check if line already exists
	content, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if strings.Contains(string(content), line) {
		return nil // Line already exists
	}

	// Append to file
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString("\n" + line + "\n")
	return err
}

// getCurrentVersionFromPath extracts version from path
func getCurrentVersionFromPath(path string) string {
	parts := strings.Split(path, string(os.PathSeparator))
	for _, part := range parts {
		if strings.Contains(part, "versions") {
			// Find the next part which should be the version
			for i, p := range parts {
				if p == "versions" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}
	return "unknown"
} // createVersionLinks creates symlinks or batch files for easy access
func (nvs *NodeVersionSwitcher) createVersionLinks(version, binPath string) error {
	linkDir := filepath.Join(nvs.NVSDir, "current-bin")

	// Remove existing links
	os.RemoveAll(linkDir)
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		return fmt.Errorf("failed to create link directory: %w", err)
	}

	isWindows := runtime.GOOS == "windows"
	binaries := []string{"node", "npm", "npx"}

	for _, binary := range binaries {
		var sourcePath, linkPath string

		if isWindows {
			if binary == "node" {
				sourcePath = filepath.Join(binPath, "node.exe")
			} else {
				sourcePath = filepath.Join(binPath, binary+".cmd")
			}
			linkPath = filepath.Join(linkDir, binary+".bat")
		} else {
			sourcePath = filepath.Join(binPath, binary)
			linkPath = filepath.Join(linkDir, binary)
		}

		if _, err := os.Stat(sourcePath); err == nil {
			if isWindows {
				// Create batch file wrapper
				batchContent := fmt.Sprintf("@echo off\n\"%s\" %%*", sourcePath)
				if err := os.WriteFile(linkPath, []byte(batchContent), 0644); err != nil {
					return fmt.Errorf("failed to create batch file: %w", err)
				}
			} else {
				// Try to create symlink, fallback to script
				if err := os.Symlink(sourcePath, linkPath); err != nil {
					// Fallback: create shell script wrapper
					scriptContent := fmt.Sprintf("#!/bin/bash\nexec \"%s\" \"$@\"", sourcePath)
					if err := os.WriteFile(linkPath, []byte(scriptContent), 0755); err != nil {
						return fmt.Errorf("failed to create script wrapper: %w", err)
					}
				}
			}
		}
	}

	return nil
}

// List lists all installed Node.js versions
func (nvs *NodeVersionSwitcher) List() error {
	fmt.Println("📦 Installed Node.js versions:")

	if _, err := os.Stat(nvs.VersionsDir); os.IsNotExist(err) {
		fmt.Println("   No versions installed")
		return nil
	}

	entries, err := os.ReadDir(nvs.VersionsDir)
	if err != nil {
		return fmt.Errorf("failed to read versions directory: %w", err)
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}

	if len(versions) == 0 {
		fmt.Println("   No versions installed")
		return nil
	}

	current, _ := nvs.getCurrentVersion()

	// Sort versions
	sort.Slice(versions, func(i, j int) bool {
		partsI := strings.Split(versions[i], "-")
		partsJ := strings.Split(versions[j], "-")

		// Compare version numbers first
		if len(partsI) > 0 && len(partsJ) > 0 {
			if partsI[0] != partsJ[0] {
				return partsI[0] < partsJ[0]
			}
		}

		return versions[i] < versions[j]
	})

	for _, versionKey := range versions {
		marker := ""
		if versionKey == current {
			marker = " ✅ (current)"
		}

		// Parse version key to show readable format
		parts := strings.Split(versionKey, "-")
		if len(parts) == 3 {
			fmt.Printf("   %s (%s-%s)%s\n", parts[0], parts[1], parts[2], marker)
		} else {
			fmt.Printf("   %s%s\n", versionKey, marker)
		}
	}

	return nil
}

// getCurrentVersion returns the currently active version
func (nvs *NodeVersionSwitcher) getCurrentVersion() (string, error) {
	content, err := os.ReadFile(nvs.CurrentFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

// Current shows the currently active version
func (nvs *NodeVersionSwitcher) Current() error {
	current, err := nvs.getCurrentVersion()
	if err != nil {
		fmt.Println("No version currently selected")
		return nil
	}

	fmt.Printf("Currently using Node.js v%s\n", current)
	return nil
}

// Uninstall removes a Node.js version
func (nvs *NodeVersionSwitcher) Uninstall(version string) error {
	versionDir := filepath.Join(nvs.VersionsDir, version)

	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		fmt.Printf("Node.js v%s is not installed\n", version)
		return nil
	}

	if err := os.RemoveAll(versionDir); err != nil {
		return fmt.Errorf("failed to remove version directory: %w", err)
	}

	fmt.Printf("Node.js v%s uninstalled\n", version)

	// Clear current if it was the uninstalled version
	if current, err := nvs.getCurrentVersion(); err == nil && current == version {
		os.Remove(nvs.CurrentFile)
	}

	return nil
}

// Setup provides instructions for permanent PATH configuration
func (nvs *NodeVersionSwitcher) Setup() error {
	currentBinPath := filepath.Join(nvs.NVSDir, "current-bin")

	fmt.Println("🔧 NVS Permanent Setup Instructions")
	fmt.Println("=====================================")
	fmt.Println("\nRun ONE of the following commands to set up NVS permanently:")

	if runtime.GOOS == "windows" {
		// Detect if running in Git Bash or similar Unix-like environment on Windows
		isGitBash := os.Getenv("MSYSTEM") != "" || os.Getenv("TERM") != "" || strings.Contains(strings.ToLower(os.Getenv("SHELL")), "bash")

		if isGitBash {
			fmt.Println("\n📝 For Git Bash/MSYS2 (add to ~/.bashrc):")
			fmt.Printf("   echo 'export PATH=\"%s:%s:$PATH\"' >> ~/.bashrc\n", currentBinPath, nvs.BinDir)
			fmt.Println("   source ~/.bashrc")
		} else {
			fmt.Println("\n📝 For Command Prompt/PowerShell (run as admin):")
			fmt.Printf("   setx PATH \"%s;%s;%%PATH%%\"\n", currentBinPath, nvs.BinDir)
		}

		fmt.Println("\n📝 Alternative - Manual setup:")
		fmt.Println("   1. Open System Properties → Environment Variables")
		fmt.Printf("   2. Add these paths to your PATH variable:\n")
		fmt.Printf("      %s\n", currentBinPath)
		fmt.Printf("      %s\n", nvs.BinDir)
	} else {
		// Detect shell
		shell := os.Getenv("SHELL")
		configFile := "~/.bashrc"
		if strings.Contains(shell, "zsh") {
			configFile = "~/.zshrc"
		}

		fmt.Printf("\n📝 For %s (add to %s):\n", filepath.Base(shell), configFile)
		fmt.Printf("   echo 'export PATH=\"%s:%s:$PATH\"' >> %s\n", currentBinPath, nvs.BinDir, configFile)
		fmt.Printf("   source %s\n", configFile)
	}

	fmt.Println("\n✅ After setup, you can:")
	fmt.Println("   • Run 'nvs' from anywhere")
	fmt.Println("   • Use 'nvs use <version>' to switch Node.js versions instantly")
	fmt.Println("   • Node.js commands (node, npm, npx) will automatically use the current version")

	fmt.Println("\n💡 Benefits of permanent setup:")
	fmt.Println("   ✓ No need to export PATH manually each time")
	fmt.Println("   ✓ Works across all terminal sessions")
	fmt.Println("   ✓ Automatically updates when you switch versions")

	return nil
}

// ShowHelp displays help information
func (nvs *NodeVersionSwitcher) ShowHelp() {
	help := `
🚀 Node Version Switcher (NVS) - No Admin Required!

USAGE:
  nvs <command> [version] [options]

COMMANDS:
  install <version> [--os <os>] [--arch <arch>]   Install a Node.js version
  use <version> [--os <os>] [--arch <arch>] [--global]  Switch to a Node.js version  
  list                                            List all installed versions
  current                                         Show currently active version
  setup                                           Show permanent PATH setup instructions
  uninstall <version>                             Remove a Node.js version
  help                                            Show this help message

FLAGS:
  --global                                        Set Node.js version globally for all sessions
  --os <platform>                                 Target OS: windows, linux, darwin (default: current OS)
  --arch <architecture>                           Target arch: x64, arm64, x86 (default: current arch)

EXAMPLES:
  # Basic usage
  nvs install 18.17.0                    # Install for current platform
  nvs use 18.17.0                        # Switch to v18.17.0 (current session only)
  nvs use 18.17.0 --global               # Switch to v18.17.0 globally (all sessions)
  nvs list                               # Show all installed versions
  
  # One-time setup (alternative to --global)
  nvs setup                              # Show permanent PATH setup instructions
  
  # Cross-platform installation  
  nvs install 20.5.0 --os linux --arch x64      # Install Linux x64 version
  nvs install 18.17.0 --os windows --arch x64   # Install Windows x64 version
  nvs install 22.16.0 --os darwin --arch arm64  # Install macOS ARM64 version
  
  # Use cross-platform versions
  nvs use 20.5.0 --os linux --arch x64          # Use Linux version (if compatible)
  nvs use 20.5.0 --os linux --arch x64 --global # Set Linux version globally

SUPPORTED PLATFORMS:
  • Windows (windows, win) - x64, x86, arm64
  • macOS (darwin, macos) - x64, arm64  
  • Linux (linux) - x64, x86, arm64

FEATURES:
  ✅ No admin/root privileges required
  ✅ Cross-platform installation support
  ✅ Single binary - no dependencies
  ✅ Fast version switching (local and global)
  ✅ Isolated installations
  ✅ Multiple architectures per version
  ✅ Git Bash / MSYS2 support on Windows
  ✅ Session-specific and global environment setup

SESSION MANAGEMENT:
  • 'nvs use <version>' - Sets version for current session only
  • 'nvs use <version> --global' - Sets version globally for all sessions
  • 'nvs setup' - Manual permanent PATH configuration

All Node.js versions are installed to: ~/.nvs/versions/
`
	fmt.Print(help)
}

func main() {
	nvs := NewNodeVersionSwitcher()

	if err := nvs.Init(); err != nil {
		fmt.Printf("❌ Error initializing NVS: %v\n", err)
		os.Exit(1)
	}

	var rootCmd = &cobra.Command{
		Use:   "nvs",
		Short: "Node Version Switcher - No Admin Required!",
		Long:  "A fast, lightweight Node.js version manager that requires NO admin privileges!",
		Run: func(cmd *cobra.Command, args []string) {
			nvs.ShowHelp()
		},
	}

	// Install command
	var installCmd = &cobra.Command{
		Use:   "install <version>",
		Short: "Install a Node.js version",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			targetOS, _ := cmd.Flags().GetString("os")
			targetArch, _ := cmd.Flags().GetString("arch")

			if err := nvs.Install(args[0], targetOS, targetArch); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	installCmd.Flags().String("os", "", "Target OS: windows, linux, darwin")
	installCmd.Flags().String("arch", "", "Target arch: x64, arm64, x86")

	// Use command
	var useCmd = &cobra.Command{
		Use:   "use <version>",
		Short: "Switch to a Node.js version",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			targetOS, _ := cmd.Flags().GetString("os")
			targetArch, _ := cmd.Flags().GetString("arch")
			global, _ := cmd.Flags().GetBool("global")

			if err := nvs.Use(args[0], targetOS, targetArch, global); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	useCmd.Flags().String("os", "", "Target OS: windows, linux, darwin")
	useCmd.Flags().String("arch", "", "Target arch: x64, arm64, x86")
	useCmd.Flags().Bool("global", false, "Set globally for all terminal sessions")

	// List command
	var listCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all installed versions",
		Run: func(cmd *cobra.Command, args []string) {
			if err := nvs.List(); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Current command
	var currentCmd = &cobra.Command{
		Use:   "current",
		Short: "Show currently active version",
		Run: func(cmd *cobra.Command, args []string) {
			if err := nvs.Current(); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Setup command
	var setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Show instructions for permanent PATH setup",
		Run: func(cmd *cobra.Command, args []string) {
			if err := nvs.Setup(); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Uninstall command
	var uninstallCmd = &cobra.Command{
		Use:     "uninstall <version>",
		Aliases: []string{"remove", "rm"},
		Short:   "Remove a Node.js version",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := nvs.Uninstall(args[0]); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Help command
	var helpCmd = &cobra.Command{
		Use:   "help",
		Short: "Show help message",
		Run: func(cmd *cobra.Command, args []string) {
			nvs.ShowHelp()
		},
	}

	rootCmd.AddCommand(installCmd, useCmd, listCmd, currentCmd, setupCmd, uninstallCmd, helpCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}
