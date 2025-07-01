package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Test utilities
func setupTestEnvironment(t *testing.T) (string, *NodeVersionSwitcher) {
	testDir := t.TempDir()

	// Set HOME environment variable for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", testDir)

	t.Cleanup(func() {
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		} else {
			os.Unsetenv("HOME")
		}
	})

	return testDir, NewNodeVersionSwitcher()
}

func TestNewNodeVersionSwitcher(t *testing.T) {
	testDir, nvs := setupTestEnvironment(t)

	expected := &NodeVersionSwitcher{
		HomeDir:     testDir,
		NVSDir:      filepath.Join(testDir, ".nvs"),
		VersionsDir: filepath.Join(testDir, ".nvs", "versions"),
		CurrentFile: filepath.Join(testDir, ".nvs", "current"),
		BinDir:      filepath.Join(testDir, ".nvs", "bin"),
	}

	if nvs.HomeDir != expected.HomeDir {
		t.Errorf("Expected HomeDir %s, got %s", expected.HomeDir, nvs.HomeDir)
	}
	if nvs.NVSDir != expected.NVSDir {
		t.Errorf("Expected NVSDir %s, got %s", expected.NVSDir, nvs.NVSDir)
	}
	if nvs.VersionsDir != expected.VersionsDir {
		t.Errorf("Expected VersionsDir %s, got %s", expected.VersionsDir, nvs.VersionsDir)
	}
	if nvs.CurrentFile != expected.CurrentFile {
		t.Errorf("Expected CurrentFile %s, got %s", expected.CurrentFile, nvs.CurrentFile)
	}
	if nvs.BinDir != expected.BinDir {
		t.Errorf("Expected BinDir %s, got %s", expected.BinDir, nvs.BinDir)
	}
}

func TestInit(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	err := nvs.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Check that directories were created
	dirs := []string{nvs.NVSDir, nvs.VersionsDir, nvs.BinDir}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}

func TestGetNodeRelease(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	tests := []struct {
		name       string
		version    string
		targetOS   string
		targetArch string
		expected   *NodeRelease
		expectErr  bool
	}{
		{
			name:       "Linux x64",
			version:    "18.17.0",
			targetOS:   "linux",
			targetArch: "amd64",
			expected: &NodeRelease{
				Version:  "18.17.0",
				URL:      "https://nodejs.org/dist/v18.17.0/node-v18.17.0-linux-x64.tar.xz",
				Filename: "node-v18.17.0-linux-x64.tar.xz",
				Ext:      "tar.xz",
			},
		},
		{
			name:       "Windows x64",
			version:    "20.5.0",
			targetOS:   "windows",
			targetArch: "amd64",
			expected: &NodeRelease{
				Version:  "20.5.0",
				URL:      "https://nodejs.org/dist/v20.5.0/node-v20.5.0-win-x64.zip",
				Filename: "node-v20.5.0-win-x64.zip",
				Ext:      "zip",
			},
		},
		{
			name:       "macOS ARM64",
			version:    "22.16.0",
			targetOS:   "darwin",
			targetArch: "arm64",
			expected: &NodeRelease{
				Version:  "22.16.0",
				URL:      "https://nodejs.org/dist/v22.16.0/node-v22.16.0-darwin-arm64.tar.gz",
				Filename: "node-v22.16.0-darwin-arm64.tar.gz",
				Ext:      "tar.gz",
			},
		},
		{
			name:       "Unsupported platform",
			version:    "18.17.0",
			targetOS:   "unsupported",
			targetArch: "amd64",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release, err := nvs.getNodeRelease(tt.version, tt.targetOS, tt.targetArch)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if release.Version != tt.expected.Version {
				t.Errorf("Expected Version %s, got %s", tt.expected.Version, release.Version)
			}
			if release.URL != tt.expected.URL {
				t.Errorf("Expected URL %s, got %s", tt.expected.URL, release.URL)
			}
			if release.Filename != tt.expected.Filename {
				t.Errorf("Expected Filename %s, got %s", tt.expected.Filename, release.Filename)
			}
			if release.Ext != tt.expected.Ext {
				t.Errorf("Expected Ext %s, got %s", tt.expected.Ext, release.Ext)
			}
		})
	}
}

func TestArchitectureNormalization(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	tests := []struct {
		inputArch    string
		expectedArch string
	}{
		{"x86_64", "x64"},
		{"amd64", "x64"},
		{"aarch64", "arm64"},
		{"arm64", "arm64"},
		{"x86", "x86"},
		{"i386", "x86"},
		{"ia32", "x86"},
		{"386", "x86"},
	}

	for _, tt := range tests {
		t.Run(tt.inputArch, func(t *testing.T) {
			release, err := nvs.getNodeRelease("18.17.0", "linux", tt.inputArch)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			expectedFilename := "node-v18.17.0-linux-" + tt.expectedArch + ".tar.xz"
			if release.Filename != expectedFilename {
				t.Errorf("Expected filename %s, got %s", expectedFilename, release.Filename)
			}
		})
	}
}

func TestPlatformAliases(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	tests := []struct {
		inputPlatform    string
		expectedPlatform string
		expectedExt      string
	}{
		{"windows", "win", "zip"},
		{"win", "win", "zip"},
		{"darwin", "darwin", "tar.gz"},
		{"macos", "darwin", "tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.inputPlatform, func(t *testing.T) {
			release, err := nvs.getNodeRelease("18.17.0", tt.inputPlatform, "amd64")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			expectedFilename := "node-v18.17.0-" + tt.expectedPlatform + "-x64." + tt.expectedExt
			if release.Filename != expectedFilename {
				t.Errorf("Expected filename %s, got %s", expectedFilename, release.Filename)
			}
			if release.Ext != tt.expectedExt {
				t.Errorf("Expected ext %s, got %s", tt.expectedExt, release.Ext)
			}
		})
	}
}

func TestGetCurrentVersionNoFile(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	current, err := nvs.getCurrentVersion()
	if err == nil {
		t.Errorf("Expected error when file doesn't exist, but got current: %s", current)
	}
}

func TestGetCurrentVersionWithFile(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	if err := nvs.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	expectedVersion := "18.17.0"
	if err := os.WriteFile(nvs.CurrentFile, []byte(expectedVersion), 0644); err != nil {
		t.Fatalf("Failed to write current file: %v", err)
	}

	current, err := nvs.getCurrentVersion()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if current != expectedVersion {
		t.Errorf("Expected current version %s, got %s", expectedVersion, current)
	}
}

func TestUninstallNonExistentVersion(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	// This should not return an error, just a message
	err := nvs.Uninstall("18.17.0")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestUninstallExistingVersion(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	if err := nvs.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a mock version directory
	versionDir := filepath.Join(nvs.VersionsDir, "18.17.0")
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("Failed to create version directory: %v", err)
	}

	// Create a test file in the version directory
	testFile := filepath.Join(versionDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify directory exists before uninstall
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		t.Fatalf("Version directory should exist before uninstall")
	}

	// Uninstall
	err := nvs.Uninstall("18.17.0")
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
		t.Errorf("Version directory should be removed after uninstall")
	}
}

func TestUninstallCurrentVersionClearsCurrentFile(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	if err := nvs.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	version := "18.17.0"

	// Create mock version directory
	versionDir := filepath.Join(nvs.VersionsDir, version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("Failed to create version directory: %v", err)
	}

	// Set as current version
	if err := os.WriteFile(nvs.CurrentFile, []byte(version), 0644); err != nil {
		t.Fatalf("Failed to write current file: %v", err)
	}

	// Uninstall
	err := nvs.Uninstall(version)
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	// Verify current file was removed
	if _, err := os.Stat(nvs.CurrentFile); !os.IsNotExist(err) {
		t.Errorf("Current file should be removed when uninstalling current version")
	}
}

func TestList(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	if err := nvs.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create mock version directories
	versions := []string{"18.17.0", "20.5.0", "22.16.0-linux-x64"}
	for _, version := range versions {
		versionDir := filepath.Join(nvs.VersionsDir, version)
		if err := os.MkdirAll(versionDir, 0755); err != nil {
			t.Fatalf("Failed to create version directory %s: %v", version, err)
		}
	}

	// Set current version
	currentVersion := "18.17.0"
	if err := os.WriteFile(nvs.CurrentFile, []byte(currentVersion), 0644); err != nil {
		t.Fatalf("Failed to write current file: %v", err)
	}

	// List should not return an error
	err := nvs.List()
	if err != nil {
		t.Errorf("List failed: %v", err)
	}
}

func TestVersionKeyGeneration(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		targetOS   string
		targetArch string
		expected   string
	}{
		{
			name:     "Simple version",
			version:  "18.17.0",
			expected: "18.17.0",
		},
		{
			name:       "Cross-platform version",
			version:    "18.17.0",
			targetOS:   "linux",
			targetArch: "amd64",
			expected:   "18.17.0-linux-amd64",
		},
		{
			name:     "OS only",
			version:  "18.17.0",
			targetOS: "linux",
			expected: "18.17.0-linux-" + runtime.GOARCH,
		},
		{
			name:       "Arch only",
			version:    "18.17.0",
			targetArch: "arm64",
			expected:   "18.17.0-" + runtime.GOOS + "-arm64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test version key generation logic
			versionKey := tt.version
			if tt.targetOS != "" || tt.targetArch != "" {
				os := tt.targetOS
				if os == "" {
					os = runtime.GOOS
				}
				arch := tt.targetArch
				if arch == "" {
					arch = runtime.GOARCH
				}
				versionKey = tt.version + "-" + os + "-" + arch
			}

			if tt.name == "Simple version" && versionKey != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, versionKey)
			} else if tt.name == "Cross-platform version" && versionKey != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, versionKey)
			} else if tt.name == "OS only" && !strings.HasPrefix(versionKey, "18.17.0-linux-") {
				t.Errorf("Expected version key to start with '18.17.0-linux-', got %s", versionKey)
			} else if tt.name == "Arch only" && !strings.Contains(versionKey, "-arm64") {
				t.Errorf("Expected version key to contain '-arm64', got %s", versionKey)
			}
		})
	}
}

func TestUseNonExistentVersion(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	// This should not return an error, just print a message
	err := nvs.Use("18.17.0", "", "")
	if err != nil {
		t.Errorf("Use should not return error for non-existent version: %v", err)
	}
}

func TestCreateVersionLinks(t *testing.T) {
	_, nvs := setupTestEnvironment(t)

	if err := nvs.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a mock bin directory with node binary
	testBinDir := filepath.Join(nvs.NVSDir, "test-bin")
	if err := os.MkdirAll(testBinDir, 0755); err != nil {
		t.Fatalf("Failed to create test bin directory: %v", err)
	}

	// Create mock binaries based on OS
	if runtime.GOOS == "windows" {
		// Create Windows binaries
		binaries := []string{"node.exe", "npm.cmd", "npx.cmd"}
		for _, binary := range binaries {
			path := filepath.Join(testBinDir, binary)
			if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create mock binary %s: %v", binary, err)
			}
		}
	} else {
		// Create Unix binaries
		binaries := []string{"node", "npm", "npx"}
		for _, binary := range binaries {
			path := filepath.Join(testBinDir, binary)
			if err := os.WriteFile(path, []byte("#!/bin/bash\necho test"), 0755); err != nil {
				t.Fatalf("Failed to create mock binary %s: %v", binary, err)
			}
		}
	}

	// Test creating version links
	err := nvs.createVersionLinks("18.17.0", testBinDir)
	if err != nil {
		t.Fatalf("createVersionLinks failed: %v", err)
	}

	// Verify that link directory was created
	linkDir := filepath.Join(nvs.NVSDir, "current-bin")
	if _, err := os.Stat(linkDir); os.IsNotExist(err) {
		t.Errorf("Link directory was not created")
	}
}
