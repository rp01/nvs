package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2/app"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// Config definition
const (
	NVS_DIR_NAME = ".nvs"
)

// Execution modes
type ExecutionMode int

const (
	ModeSmartInstaller ExecutionMode = iota
	ModeUIManager
	ModeCLI
)

// NodeVersionSwitcher manages the state
type NodeVersionSwitcher struct {
	HomeDir     string
	NVSDir      string
	VersionsDir string
	BinDir      string
	CurrentLink string // The symlink path
}

func NewNodeVersionSwitcher() *NodeVersionSwitcher {
	homeDir := getHomeDir()
	nvsDir := filepath.Join(homeDir, NVS_DIR_NAME)

	return &NodeVersionSwitcher{
		HomeDir:     homeDir,
		NVSDir:      nvsDir,
		VersionsDir: filepath.Join(nvsDir, "versions"),
		BinDir:      filepath.Join(nvsDir, "bin"),
		CurrentLink: filepath.Join(nvsDir, "current"),
	}
}

func getHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}
	return "."
}

// --- CORE OPERATIONS ---

// Init creates the directory structure and installs the binary
func (nvs *NodeVersionSwitcher) Init() error {
	dirs := []string{nvs.NVSDir, nvs.VersionsDir, nvs.BinDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", dir, err)
		}
	}
	return nvs.installSelf()
}

// installSelf copies the running executable to ~/.nvs/bin
func (nvs *NodeVersionSwitcher) installSelf() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	targetName := "nvs"
	if runtime.GOOS == "windows" {
		targetName = "nvs.exe"
	}
	targetPath := filepath.Join(nvs.BinDir, targetName)

	// Windows Fix: Cannot overwrite running executable. Rename old one first.
	if _, err := os.Stat(targetPath); err == nil {
		oldPath := targetPath + ".old"
		os.Remove(oldPath) // Remove ancient backup
		if err := os.Rename(targetPath, oldPath); err != nil {
			// If rename fails, we might not be running from the target, or file is locked
			// Just try to proceed, but warn
			fmt.Printf("⚠️  Warning: Could not move existing binary. If this fails, delete %s manually.\n", targetPath)
		}
	}

	// Copy file
	src, err := os.Open(executable)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	// Permissions
	if runtime.GOOS != "windows" {
		os.Chmod(targetPath, 0755)
	}

	fmt.Printf("✅ NVS installed to %s\n", targetPath)
	return nvs.setupShellEnv()
}

// setupShellEnv tells the user how to configure their PATH once
func (nvs *NodeVersionSwitcher) setupShellEnv() error {
	fmt.Println("\n⚡ ACTION REQUIRED: One-time setup")

	if runtime.GOOS == "windows" {
		pathToAdd := fmt.Sprintf("%s;%s", nvs.BinDir, filepath.Join(nvs.CurrentLink))
		fmt.Println("Run this command in PowerShell to set your PATH:")
		fmt.Printf("\n   [Environment]::SetEnvironmentVariable(\"Path\", $env:Path + \";%s\", \"User\")\n", pathToAdd)
		fmt.Println("\nOr manually add these to your User PATH environment variable:")
		fmt.Printf("   1. %s\n", nvs.BinDir)
		fmt.Printf("   2. %s\n", nvs.CurrentLink)
	} else {
		// Unix
		profile := ".bashrc"
		if strings.Contains(os.Getenv("SHELL"), "zsh") {
			profile = ".zshrc"
		}

		fmt.Printf("Add the following lines to your ~/%s:\n\n", profile)
		fmt.Printf("export NVS_HOME=\"$HOME/%s\"\n", NVS_DIR_NAME)
		// We add both current and current/bin to support Windows/Linux directory differences
		fmt.Println("export PATH=\"$NVS_HOME/bin:$NVS_HOME/current/bin:$NVS_HOME/current:$PATH\"")

		// Attempt auto-append
		rcPath := filepath.Join(nvs.HomeDir, profile)

		// Check if config already exists
		existingContent, err := os.ReadFile(rcPath)
		if err == nil {
			if strings.Contains(string(existingContent), "export NVS_HOME=") {
				fmt.Printf("\n✅ Configuration already exists in %s. Skipping append.\n", rcPath)
				fmt.Println("👉 Restart your terminal to apply changes if you haven't already.")
				return nil
			}
		}

		fmt.Printf("\nAttempting to append to %s... ", rcPath)
		f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("Failed. Please add manually.")
		} else {
			defer f.Close()
			block := fmt.Sprintf("\n# NVS Configuration\nexport NVS_HOME=\"$HOME/%s\"\nexport PATH=\"$NVS_HOME/bin:$NVS_HOME/current/bin:$NVS_HOME/current:$PATH\"\n", NVS_DIR_NAME)
			if _, err := f.WriteString(block); err != nil {
				fmt.Println("Failed to write.")
			} else {
				fmt.Println("Success! ✅")
				fmt.Println("👉 Restart your terminal to apply changes.")
			}
		}
	}
	return nil
}

// resolveVersion resolves semantic version aliases (e.g. "18" -> "v18.16.0", "latest", "lts")
func (nvs *NodeVersionSwitcher) resolveVersion(input string) (string, error) {
	fmt.Printf("🔎 Resolving version for '%s'...\n", input)

	resp, err := http.Get("https://nodejs.org/dist/index.json")
	if err != nil {
		return "", fmt.Errorf("failed to fetch version index: %w", err)
	}
	defer resp.Body.Close()

	var versions []struct {
		Version string      `json:"version"`
		Lts     interface{} `json:"lts"` // can be bool or string
	}

	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return "", fmt.Errorf("failed to decode version index: %w", err)
	}

	cleanInput := strings.TrimPrefix(input, "v")

	// 1. Handle "latest" or "current"
	if cleanInput == "latest" || cleanInput == "current" {
		return versions[0].Version, nil
	}

	// 2. Handle "lts"
	if cleanInput == "lts" {
		for _, v := range versions {
			// lts field is false (bool) or codename (string)
			// we want the first one that is NOT false
			if ltsVal, ok := v.Lts.(bool); ok && !ltsVal {
				continue
			}
			return v.Version, nil
		}
		return "", fmt.Errorf("no LTS version found")
	}

	// 3. Handle Partial Matching (e.g. "18" -> "v18.x.x")
	// The index.json is sorted new -> old. The first match is the latest minor version.

	// Exact match check first (e.g. user typed "18.16.0")
	exactTarget := "v" + cleanInput
	for _, v := range versions {
		if v.Version == exactTarget {
			return v.Version, nil
		}
	}

	// Prefix match (e.g. user typed "18", we match "v18.")
	// We add a dot to ensure "1" doesn't match "18".
	prefixTarget := "v" + cleanInput + "."
	for _, v := range versions {
		if strings.HasPrefix(v.Version, prefixTarget) {
			return v.Version, nil
		}
	}

	return "", fmt.Errorf("version '%s' not found", input)
}

// Install downloads and extracts a version
func (nvs *NodeVersionSwitcher) Install(requestedVersion string) error {
	// Step 1: Resolve the version (handles "18", "lts", "latest")
	resolvedVersion, err := nvs.resolveVersion(requestedVersion)
	if err != nil {
		return err
	}

	// Normalize version (remove 'v' prefix if exists)
	version := strings.TrimPrefix(resolvedVersion, "v")

	// 1. Determine URL and Filename
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go arch to Node arch
	if arch == "amd64" {
		arch = "x64"
	} else if arch == "386" {
		arch = "x86"
	}

	extension := "tar.gz"
	if osName == "windows" {
		osName = "win"
		extension = "zip"
	} else if osName == "darwin" {
		// Node uses 'darwin'
	}

	fileName := fmt.Sprintf("node-v%s-%s-%s.%s", version, osName, arch, extension)
	url := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", version, fileName)

	// Target Directory: ~/.nvs/versions/v18.0.0
	targetDir := filepath.Join(nvs.VersionsDir, "v"+version)
	if _, err := os.Stat(targetDir); err == nil {
		fmt.Printf("Version v%s is already installed.\n", version)
		return nil
	}

	// 2. Download
	tmpFile := filepath.Join(nvs.NVSDir, "temp-"+fileName)
	defer os.Remove(tmpFile)

	fmt.Printf("Downloading Node.js v%s...\n", version)
	if err := downloadFile(url, tmpFile); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// 3. Extract
	fmt.Println("Extracting...")
	extractTempDir := filepath.Join(nvs.NVSDir, "temp-extract-"+version)
	os.RemoveAll(extractTempDir) // ensure clean

	if extension == "zip" {
		if err := unzip(tmpFile, extractTempDir); err != nil {
			return err
		}
	} else {
		if err := untar(tmpFile, extractTempDir); err != nil {
			return err
		}
	}

	// 4. Move to final location
	// The archive usually contains a root folder like "node-v18.0.0-linux-x64"
	// We want to find that folder and move its *contents* or rename *it* to targetDir
	files, _ := os.ReadDir(extractTempDir)
	var rootFolder string
	for _, f := range files {
		if f.IsDir() && strings.HasPrefix(f.Name(), "node-") {
			rootFolder = filepath.Join(extractTempDir, f.Name())
			break
		}
	}

	if rootFolder == "" {
		// Fallback if structure is weird
		rootFolder = extractTempDir
	}

	if err := os.Rename(rootFolder, targetDir); err != nil {
		// Cross-device link error fallback
		return fmt.Errorf("failed to move extracted files: %w", err)
	}
	os.RemoveAll(extractTempDir)

	// 5. Fix Symlinks (Unix Only)
	// We explicitly recreate npm/npx symlinks to point to the actual library files
	// This solves issues where the tarball's symlinks are relative in a way that breaks
	// or if permissions weren't preserved correctly.
	if runtime.GOOS != "windows" {
		if err := nvs.fixNpmSymlinks(targetDir); err != nil {
			fmt.Printf("⚠️  Warning: Failed to fix npm symlinks: %v\n", err)
		}
	}

	fmt.Printf("✅ Installed Node.js v%s\n", version)
	return nil
}

// fixNpmSymlinks manually forces bin/npm and bin/npx to point to lib/node_modules
func (nvs *NodeVersionSwitcher) fixNpmSymlinks(versionDir string) error {
	binDir := filepath.Join(versionDir, "bin")
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		// Should generally not happen on unix, but good check
		return nil
	}

	// Ensure the actual CLI files are executable
	// Sometimes tar extraction might miss the +x bit on the target files
	npmCli := filepath.Join(versionDir, "lib", "node_modules", "npm", "bin", "npm-cli.js")
	npxCli := filepath.Join(versionDir, "lib", "node_modules", "npm", "bin", "npx-cli.js")
	os.Chmod(npmCli, 0755)
	os.Chmod(npxCli, 0755)

	// Define standard symlinks
	links := map[string]string{
		"npm": "../lib/node_modules/npm/bin/npm-cli.js",
		"npx": "../lib/node_modules/npm/bin/npx-cli.js",
	}

	for name, target := range links {
		linkPath := filepath.Join(binDir, name)

		// Remove whatever is there (old symlink or file)
		_ = os.Remove(linkPath)

		// Create a clean symlink
		if err := os.Symlink(target, linkPath); err != nil {
			return fmt.Errorf("failed to link %s: %w", name, err)
		}
	}
	return nil
}

// Use switches the version by updating the symlink
func (nvs *NodeVersionSwitcher) Use(version string) error {
	// Note: We don't use resolveVersion here because Use works on LOCAL installed versions.
	// Users should type "nvs use 18" and expect it to find the installed v18.
	// Implementing partial local matching would be good, but for now we expect exact or simple v-strip

	version = strings.TrimPrefix(version, "v")
	targetVersionDir := filepath.Join(nvs.VersionsDir, "v"+version)

	// Simple fuzzy match: if exact folder doesn't exist, try to find a folder starting with "v"+version
	if _, err := os.Stat(targetVersionDir); os.IsNotExist(err) {
		// Check for partial local match
		files, _ := os.ReadDir(nvs.VersionsDir)
		prefix := "v" + version + "."
		var found string
		for _, f := range files {
			if strings.HasPrefix(f.Name(), prefix) {
				found = f.Name() // files are roughly sorted, we'll take the first or implement logic to take best
				// Since we just want *a* match, let's grab the last one (usually highest version if sorted alphabetically)
			}
		}
		if found != "" {
			fmt.Printf("Auto-selected %s for '%s'\n", found, version)
			targetVersionDir = filepath.Join(nvs.VersionsDir, found)
			version = strings.TrimPrefix(found, "v")
		} else {
			return fmt.Errorf("version v%s is not installed. Run 'nvs install %s' first", version, version)
		}
	}

	// 1. Remove existing symlink/junction
	// We check Lstat to see if the link exists (even if broken)
	if _, err := os.Lstat(nvs.CurrentLink); err == nil {
		if err := os.Remove(nvs.CurrentLink); err != nil {
			return fmt.Errorf("failed to remove existing link: %w", err)
		}
	}

	// 2. Create new link
	fmt.Printf("Switching to v%s...\n", version)

	if runtime.GOOS == "windows" {
		// Windows: Use Directory Junction.
		// Go's os.Symlink requires Admin. 'mklink /J' does not.
		cmd := exec.Command("cmd", "/c", "mklink", "/J", nvs.CurrentLink, targetVersionDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("windows junction failed: %s: %w", string(output), err)
		}
	} else {
		// Unix: Standard Symlink
		if err := os.Symlink(targetVersionDir, nvs.CurrentLink); err != nil {
			return fmt.Errorf("symlink failed: %w", err)
		}
	}

	fmt.Printf("✅ Now using Node.js v%s\n", version)
	// Check if PATH is set correctly
	checkPath(nvs.CurrentLink)
	return nil
}

func checkPath(linkPath string) {
	pathEnv := os.Getenv("PATH")
	if !strings.Contains(pathEnv, NVS_DIR_NAME) {
		fmt.Println("⚠️  Warning: NVS directory is not in your PATH.")
		fmt.Println("   Run 'nvs setup' to see how to fix this.")
	}
}

// List installed versions
func (nvs *NodeVersionSwitcher) List() {
	files, err := os.ReadDir(nvs.VersionsDir)
	if err != nil {
		fmt.Println("No versions installed.")
		return
	}

	// Get current target
	currentTarget, _ := filepath.EvalSymlinks(nvs.CurrentLink)

	fmt.Println("Installed Versions:")
	for _, f := range files {
		if f.IsDir() {
			prefix := "  "
			fullPath := filepath.Join(nvs.VersionsDir, f.Name())
			if fullPath == currentTarget {
				prefix = "👉"
			}
			fmt.Printf("%s %s\n", prefix, f.Name())
		}
	}
}

// --- UTILITIES ---

func downloadFile(url string, dest string) error {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}

	f, _ := os.Create(dest)
	defer f.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"downloading",
	)
	io.Copy(io.MultiWriter(f, bar), resp.Body)
	return nil
}

func untar(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			os.Chmod(target, os.FileMode(header.Mode))
		case tar.TypeSymlink:
			// Handle symlinks (like bin/npm -> ../lib/node_modules/npm/bin/npm-cli.js)
			os.MkdirAll(filepath.Dir(target), 0755)
			// Remove if exists to avoid error
			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s: %w", target, header.Linkname, err)
			}
		}
	}
	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()
	}
	return nil
}

// --- EXECUTION MODE DETECTION ---

// detectExecutionMode determines how to run based on binary name and arguments
func detectExecutionMode() ExecutionMode {
	// Check binary name
	executable, _ := os.Executable()
	execName := filepath.Base(executable)

	// Remove extension on Windows
	if runtime.GOOS == "windows" {
		execName = strings.TrimSuffix(execName, ".exe")
	}

	// Check for GUI mode indicators
	if strings.Contains(execName, "nvs-ui") {
		return ModeUIManager
	}

	// Check for installer indicators
	if strings.Contains(execName, "installer") || len(os.Args) == 1 {
		return ModeSmartInstaller
	}

	// Default to CLI
	return ModeCLI
}

// runCLI runs the command line interface
func runCLI() {
	nvs := NewNodeVersionSwitcher()

	var rootCmd = &cobra.Command{Use: "nvs", Short: "Rootless Node Version Switcher"}

	var guiCmd = &cobra.Command{
		Use:   "gui",
		Short: "Launch NVS graphical interface",
		Run: func(cmd *cobra.Command, args []string) {
			gui := NewSmartInstallerGUI()
			gui.Run()
		},
	}

	var initCmd = &cobra.Command{
		Use:   "setup",
		Short: "Initialize NVS and install to bin",
		Run: func(cmd *cobra.Command, args []string) {
			if err := nvs.Init(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		},
	}

	var installCmd = &cobra.Command{
		Use:   "install [version]",
		Short: "Install a node version (e.g., 18, 18.16, lts)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := nvs.Install(args[0]); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		},
	}

	var useCmd = &cobra.Command{
		Use:   "use [version]",
		Short: "Switch to a specific version (e.g. 18)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := nvs.Use(args[0]); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		},
	}

	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List installed versions",
		Run: func(cmd *cobra.Command, args []string) {
			nvs.List()
		},
	}

	rootCmd.AddCommand(guiCmd, initCmd, installCmd, useCmd, listCmd)
	rootCmd.Execute()
}

// --- MAIN CLI ---

func main() {
	// Detect execution mode based on binary name and arguments
	mode := detectExecutionMode()

	switch mode {
	case ModeSmartInstaller:
		// Run the smart installer GUI
		gui := NewSmartInstallerGUI()
		gui.Run()
		return

	case ModeUIManager:
		// Run the NVS Manager GUI
		app := app.NewWithID("com.nvs.manager")
		manager := NewNVSManager(app)
		manager.Show()
		app.Run()
		return

	case ModeCLI:
		// Run CLI mode
		runCLI()
		return
	}
}
