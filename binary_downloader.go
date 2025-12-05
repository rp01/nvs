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
	"runtime"
	"strings"
)

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// BinaryDownloader handles downloading NVS binaries from GitHub releases
type BinaryDownloader struct {
	detector  *InstallationDetector
	repoOwner string
	repoName  string
	version   string
	baseURL   string
}

func NewBinaryDownloader(detector *InstallationDetector, version string) *BinaryDownloader {
	return &BinaryDownloader{
		detector:  detector,
		repoOwner: "rp01", // Update with actual repo owner
		repoName:  "nvs",
		version:   version,
		baseURL:   "https://api.github.com/repos",
	}
}

// DownloadBinaries downloads and installs the CLI and GUI binaries
func (d *BinaryDownloader) DownloadBinaries(progressCallback func(string, float64)) error {
	if progressCallback != nil {
		progressCallback("🔍 Fetching release information...", 0.1)
	}

	// Get release info
	release, err := d.getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to get release info: %w", err)
	}

	if progressCallback != nil {
		progressCallback(fmt.Sprintf("📦 Found release %s", release.TagName), 0.2)
	}

	// Find assets for current platform
	cliAsset, uiAsset, err := d.findPlatformAssets(release)
	if err != nil {
		return fmt.Errorf("failed to find platform binaries: %w", err)
	}

	// Create bin directory
	if err := os.MkdirAll(d.detector.BinDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Download CLI binary
	if progressCallback != nil {
		progressCallback("📥 Downloading CLI binary...", 0.4)
	}
	if err := d.downloadAndExtract(cliAsset.BrowserDownloadURL, d.detector.CLIPath, "CLI"); err != nil {
		return fmt.Errorf("failed to download CLI: %w", err)
	}

	// Download UI binary
	if progressCallback != nil {
		progressCallback("📥 Downloading GUI binary...", 0.7)
	}
	if err := d.downloadAndExtract(uiAsset.BrowserDownloadURL, d.detector.UIPath, "GUI"); err != nil {
		return fmt.Errorf("failed to download GUI: %w", err)
	}

	if progressCallback != nil {
		progressCallback("✅ Binaries downloaded successfully", 1.0)
	}

	return nil
}

// getLatestRelease fetches the latest release from GitHub API
func (d *BinaryDownloader) getLatestRelease() (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/%s/%s/releases/latest", d.baseURL, d.repoOwner, d.repoName)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// findPlatformAssets locates the CLI and GUI binaries for current platform
func (d *BinaryDownloader) findPlatformAssets(release *GitHubRelease) (cliAsset, uiAsset *struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}, err error) {
	platform := d.getPlatformIdentifier()

	var cli, ui *struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)

		// Look for CLI binary
		if strings.Contains(name, "nvs-cli") && strings.Contains(name, platform) {
			cli = &asset
		}

		// Look for GUI binary
		if strings.Contains(name, "nvs-ui") && strings.Contains(name, platform) {
			ui = &asset
		}
	}

	if cli == nil {
		return nil, nil, fmt.Errorf("CLI binary not found for platform %s", platform)
	}

	if ui == nil {
		return nil, nil, fmt.Errorf("GUI binary not found for platform %s", platform)
	}

	return cli, ui, nil
}

// getPlatformIdentifier returns the platform identifier used in release asset names
func (d *BinaryDownloader) getPlatformIdentifier() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go arch names to common names used in releases
	switch arch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	}

	// Map Go OS names to common names
	switch os {
	case "darwin":
		os = "macos"
	case "windows":
		os = "windows"
	case "linux":
		os = "linux"
	}

	return fmt.Sprintf("%s-%s", os, arch)
}

// downloadAndExtract downloads a file and extracts it if necessary
func (d *BinaryDownloader) downloadAndExtract(url, targetPath, component string) error {
	// Create temporary file
	tempFile, err := os.CreateTemp("", fmt.Sprintf("nvs-%s-*.tmp", strings.ToLower(component)))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Download file
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Copy to temp file
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save download: %w", err)
	}

	tempFile.Close()

	// Check if it's an archive
	if strings.HasSuffix(url, ".tar.gz") {
		return d.extractTarGz(tempFile.Name(), targetPath)
	} else if strings.HasSuffix(url, ".zip") {
		return d.extractZip(tempFile.Name(), targetPath)
	} else {
		// Direct binary file
		return d.moveBinary(tempFile.Name(), targetPath)
	}
}

// extractTarGz extracts a tar.gz file
func (d *BinaryDownloader) extractTarGz(archivePath, targetPath string) error {
	file, err := os.Open(archivePath)
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

		// Extract the binary (skip directories)
		if header.Typeflag == tar.TypeReg && (strings.Contains(header.Name, "nvs") || strings.HasSuffix(header.Name, ".exe")) {
			return d.extractBinaryFromTar(tr, targetPath, header.Size)
		}
	}

	return fmt.Errorf("no binary found in archive")
}

// extractZip extracts a zip file
func (d *BinaryDownloader) extractZip(archivePath, targetPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Find the binary file
		if strings.Contains(f.Name, "nvs") || strings.HasSuffix(f.Name, ".exe") {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			return d.extractBinaryFromReader(rc, targetPath)
		}
	}

	return fmt.Errorf("no binary found in zip")
}

// extractBinaryFromTar extracts binary content from tar reader
func (d *BinaryDownloader) extractBinaryFromTar(tr *tar.Reader, targetPath string, size int64) error {
	outFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if _, err := io.CopyN(outFile, tr, size); err != nil {
		return err
	}

	return os.Chmod(targetPath, 0755)
}

// extractBinaryFromReader extracts binary from io.Reader
func (d *BinaryDownloader) extractBinaryFromReader(reader io.Reader, targetPath string) error {
	outFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, reader); err != nil {
		return err
	}

	return os.Chmod(targetPath, 0755)
}

// moveBinary moves a binary file to target location
func (d *BinaryDownloader) moveBinary(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}

	return os.Chmod(targetPath, 0755)
}

// GetDownloadInfo returns information about what will be downloaded
func (d *BinaryDownloader) GetDownloadInfo() (releaseTag string, totalSize int64, err error) {
	release, err := d.getLatestRelease()
	if err != nil {
		return "", 0, err
	}

	_, _, err = d.findPlatformAssets(release)
	if err != nil {
		return "", 0, err
	}

	// Rough estimate: 50MB total
	totalSize = 50 * 1024 * 1024

	return release.TagName, totalSize, nil
}
