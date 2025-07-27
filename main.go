package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
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
	fmt.Printf("Add %s to your PATH to use 'nvs' command globally\n", nvs.BinDir)

	return nil
}

// compareVersions compares two semantic versions (e.g., "20.19.4" vs "20.9.0")
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		n1, _ := strconv.Atoi(parts1[i])
		n2, _ := strconv.Atoi(parts2[i])
		if n1 != n2 {
			return n1 - n2
		}
	}
	return len(parts1) - len(parts2)
}

// resolveVersion resolves a partial version (e.g., "20") to the latest full version (e.g., "20.19.4")
func (nvs *NodeVersionSwitcher) resolveVersion(partialVersion string) (string, error) {
	// If the version is already a full semver (e.g., "20.6.0"), return it
	if regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(partialVersion) {
		return partialVersion, nil
	}

	resp, err := http.Get("https://nodejs.org/dist/index.json")
	if err != nil {
		return "", fmt.Errorf("failed to fetch version list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch version list: HTTP %d %s", resp.StatusCode, resp.Status)
	}

	var versions []struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return "", fmt.Errorf("failed to parse version list: %w", err)
	}

	// Find the latest version matching the partial version (e.g., "20" matches "v20.x.x")
	latestVersion := ""
	for _, v := range versions {
		if strings.HasPrefix(v.Version, "v"+partialVersion+".") {
			version := strings.TrimPrefix(v.Version, "v")
			if latestVersion == "" || compareVersions(version, latestVersion) > 0 {
				latestVersion = version
			}
		}
	}

	if latestVersion == "" {
		return "", fmt.Errorf("no version found matching %s", partialVersion)
	}

	return latestVersion, nil
}

// resolveAndUpdateVersion resolves a partial version and updates it, handling errors and logging
func (nvs *NodeVersionSwitcher) resolveAndUpdateVersion(version string) (string, error) {
	resolvedVersion, err := nvs.resolveVersion(version)
	if err != nil {
		return "", fmt.Errorf("failed to resolve version %s: %w", version, err)
	}
	if resolvedVersion != version {
		fmt.Printf("Resolved version %s to %s\n", version, resolvedVersion)
	}
	return resolvedVersion, nil
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
	// Resolve partial version to full version
	version, err := nvs.resolveAndUpdateVersion(version)
	if err != nil {
		return err
	}

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

	if targetOS != "" && targetOS != runtime.GOOS {
		fmt.Printf("⚠️  Note: This is a cross-platform installation for %s\n", targetOS)
	}

	return nil
}

// Use switches to a specific Node.js version
func (nvs *NodeVersionSwitcher) Use(version, targetOS, targetArch string) error {
	// Resolve partial version to full version
	version, err := nvs.resolveAndUpdateVersion(version)
	if err != nil {
		return err
	}

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
	fmt.Printf("\n📍 Node.js binaries available at: %s\n", binPath)

	if runtime.GOOS == "windows" {
		fmt.Printf("\n🔧 To use globally, add to your PATH:\n")
		fmt.Printf("   set PATH=%s;%%PATH%%\n", binPath)
		fmt.Printf("\n   Or run: setx PATH \"%s;%%PATH%%\"\n", binPath)
	} else {
		fmt.Printf("\n🔧 To use globally, add to your PATH:\n")
		fmt.Printf("   export PATH=\"%s:$PATH\"\n", binPath)
		fmt.Printf("\n   Add this to your ~/.bashrc or ~/.zshrc for persistence\n")
	}

	// Create activation script
	if err := nvs.createActivationScript(binPath); err != nil {
		return err
	}

	return nil
}

// createVersionLinks creates symlinks or batch files for easy access
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

// createActivationScript creates an activation script
func (nvs *NodeVersionSwitcher) createActivationScript(binPath string) error {
	isWindows := runtime.GOOS == "windows"
	var scriptExt, scriptContent string

	if isWindows {
		scriptExt = ".bat"
		scriptContent = fmt.Sprintf(`@echo off
echo Activating Node.js environment...
set PATH=%s;%%PATH%%
echo Node.js path updated. You can now use 'node', 'npm', and 'npx' commands.
cmd /k`, binPath)
	} else {
		scriptExt = ".sh"
		scriptContent = fmt.Sprintf(`#!/bin/bash
echo "Activating Node.js environment..."
export PATH="%s:$PATH"
echo "Node.js path updated. You can now use 'node', 'npm', and 'npx' commands."
exec "$SHELL"`, binPath)
	}

	scriptPath := filepath.Join(nvs.NVSDir, "activate"+scriptExt)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to create activation script: %w", err)
	}

	fmt.Printf("\n🚀 Quick activation script created: %s\n", scriptPath)
	fmt.Println("   Run this script to activate Node.js in a new shell session")

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
	version, err := nvs.resolveAndUpdateVersion(version)
	if err != nil {
		return err
	}

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

// ShowHelp displays help information
func (nvs *NodeVersionSwitcher) ShowHelp() {
	help := `
🚀 Node Version Switcher (NVS) - No Admin Required!

USAGE:
  nvs <command> [version] [options]

COMMANDS:
  install <version> [--os <os>] [--arch <arch>]   Install a Node.js version
  use <version> [--os <os>] [--arch <arch>]       Switch to a Node.js version  
  list                                            List all installed versions
  current                                         Show currently active version
  uninstall <version>                             Remove a Node.js version
  help                                            Show this help message

CROSS-PLATFORM OPTIONS:
  --os <platform>       Target OS: windows, linux, darwin (default: current OS)
  --arch <architecture> Target arch: x64, arm64, x86 (default: current arch)

EXAMPLES:
  # Basic usage
  nvs install latest                    # Install the latest Node.js version
  nvs install 20                        # Install latest 20.x.x version
  nvs install 18.17.0                   # Install specific version
  nvs use 20                            # Switch to latest 20.x.x version
  nvs use 18.17.0                       # Switch to v18.17.0
  nvs list                              # Show all installed versions
  
  # Cross-platform installation  
  nvs install 20.5.0 --os linux --arch x64      # Install Linux x64 version
  nvs install 18.17.0 --os windows --arch x64   # Install Windows x64 version
  nvs install 22.16.0 --os darwin --arch arm64  # Install macOS ARM64 version
  
  # Use cross-platform versions
  nvs use 20.5.0 --os linux --arch x64          # Use Linux version (if compatible)

SUPPORTED PLATFORMS:
  • Windows (windows, win) - x64, x86, arm64
  • macOS (darwin, macos) - x64, arm64  
  • Linux (linux) - x64, x86, arm64

FEATURES:
  ✅ No admin/root privileges required
  ✅ Cross-platform installation support
  ✅ Single binary - no dependencies
  ✅ Fast version switching
  ✅ Isolated installations
  ✅ Multiple architectures per version
  ✅ Partial version support (e.g., 'nvs install 20', 'nvs use 20')

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

			if err := nvs.Use(args[0], targetOS, targetArch); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	useCmd.Flags().String("os", "", "Target OS: windows, linux, darwin")
	useCmd.Flags().String("arch", "", "Target arch: x64, arm64, x86")

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

	rootCmd.AddCommand(installCmd, useCmd, listCmd, currentCmd, uninstallCmd, helpCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}
