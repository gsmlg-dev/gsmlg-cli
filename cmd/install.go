/*
Copyright © 2022 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	githubRepo       = "gsmlg-dev/gsmlg-cli"
	githubAPIBaseURL = "https://api.github.com/repos/" + githubRepo
	defaultInstallPath = "~/.local/bin/gsmlg"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var installPath string

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Download and install the latest version of gsmlg-cli",
	Long: `Download the latest release of gsmlg-cli from GitHub and install it.

By default, the binary is installed to ~/.local/bin/gsmlg.
Use --path to specify a custom install location.

Examples:
  gsmlg-cli install
  gsmlg-cli install --path /usr/local/bin/gsmlg
  gsmlg-cli install --path ~/bin/gsmlg`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(cmd)
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringVar(&installPath, "path", defaultInstallPath, "Install path for the binary")
}

func runInstall(cmd *cobra.Command) error {
	targetPath := expandPath(installPath)

	fmt.Printf("Current version: %s\n", Version)
	fmt.Printf("Runtime: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Fetch latest release info
	fmt.Println("Fetching latest release from GitHub...")
	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}

	fmt.Printf("Latest version: %s\n", release.TagName)

	// Find the matching asset by OS/arch suffix
	assetSuffix := buildAssetSuffix(runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	var assetName string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, assetSuffix) {
			downloadURL = asset.BrowserDownloadURL
			assetName = asset.Name
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s/%s (looking for *%s)", runtime.GOOS, runtime.GOARCH, assetSuffix)
	}

	fmt.Printf("Downloading %s...\n", assetName)

	// Download the archive
	archivePath, err := downloadRelease(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	defer os.Remove(archivePath)

	// Extract the binary
	fmt.Println("Extracting binary...")
	binaryPath, err := extractBinary(archivePath, assetName)
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}
	defer os.Remove(binaryPath)

	// Ensure the install directory exists
	installDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("failed to create install directory %s: %w", installDir, err)
	}

	// Move binary to target path
	if err := installBinary(binaryPath, targetPath); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	fmt.Printf("\n✅ Successfully installed gsmlg-cli %s to %s\n", release.TagName, targetPath)
	return nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func fetchLatestRelease() (*githubRelease, error) {
	url := githubAPIBaseURL + "/releases/latest"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "gsmlg-cli/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func buildAssetSuffix(goos, goarch string) string {
	osName := goos
	if osName == "darwin" {
		osName = "mac"
	}

	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	// Matches the goreleaser name_template suffix:
	// gsmlg-cli_<version>_<os>_<arch>.<ext>
	return fmt.Sprintf("_%s_%s.%s", osName, goarch, ext)
}

func downloadRelease(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "gsmlg-cli-download-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	size, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	fmt.Printf("Downloaded %.2f MB\n", float64(size)/1024/1024)
	return tmpFile.Name(), nil
}

func extractBinary(archivePath, assetName string) (string, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractFromZip(archivePath)
	}
	return extractFromTarGz(archivePath)
}

func extractFromTarGz(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	binaryName := "gsmlg-cli"

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		baseName := filepath.Base(header.Name)
		if baseName == binaryName {
			tmpFile, err := os.CreateTemp("", "gsmlg-cli-binary-*")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmpFile, tr); err != nil {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				return "", err
			}
			tmpFile.Close()
			return tmpFile.Name(), nil
		}
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

func extractFromZip(archivePath string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	binaryName := "gsmlg-cli.exe"

	for _, f := range r.File {
		baseName := filepath.Base(f.Name)
		if baseName == binaryName {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			tmpFile, err := os.CreateTemp("", "gsmlg-cli-binary-*")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmpFile, rc); err != nil {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				return "", err
			}
			tmpFile.Close()
			return tmpFile.Name(), nil
		}
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

func installBinary(src, dst string) error {
	// Read the source binary
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	// Write to destination (atomic: write to temp then rename)
	tmpDst := dst + ".tmp"
	if err := os.WriteFile(tmpDst, data, 0o755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	if err := os.Rename(tmpDst, dst); err != nil {
		os.Remove(tmpDst)
		return fmt.Errorf("failed to move binary to final location: %w", err)
	}

	return nil
}
