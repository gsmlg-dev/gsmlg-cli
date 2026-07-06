/*
Copyright © 2026 Jonathan Gao <gsmlg.com@gmail.com>
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

const binInstallGithubAPIBaseURL = "https://api.github.com/repos"

type binInstallTool struct {
	Name        string
	Repo        string
	BinaryName  string
	DefaultPath string
	AssetNames  func(goos, goarch string) []string
}

var binInstallPath string

var binInstallCmd = &cobra.Command{
	Use:   "bin-install <codex|claude-code>",
	Short: "Install supported CLI binaries from GitHub releases",
	Long: `Download and install supported CLI binaries from their latest GitHub release.

Supported tools:
  codex        installs OpenAI Codex CLI to ~/.local/bin/codex
  claude-code  installs Claude Code CLI to ~/.local/bin/claude

Examples:
  gsmlg-cli bin-install codex
  gsmlg-cli bin-install claude-code
  gsmlg-cli bin-install claude-code --path ~/.local/bin/claude`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBinInstall(args[0])
	},
}

func init() {
	rootCmd.AddCommand(binInstallCmd)
	binInstallCmd.Flags().StringVar(&binInstallPath, "path", "", "Install path override")
}

func runBinInstall(toolName string) error {
	tool, ok := resolveBinInstallTool(toolName)
	if !ok {
		return fmt.Errorf("unsupported tool %q; supported tools: codex, claude-code", toolName)
	}

	targetPath := tool.DefaultPath
	if binInstallPath != "" {
		targetPath = binInstallPath
	}
	targetPath = expandPath(targetPath)

	fmt.Printf("Runtime: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Fetching latest %s release from %s...\n", tool.Name, tool.Repo)

	release, err := fetchLatestReleaseForRepo(tool.Repo)
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}

	asset, err := selectBinInstallAsset(release, tool, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	if asset.BrowserDownloadURL == "" {
		return fmt.Errorf("release asset %q has no download URL", asset.Name)
	}

	fmt.Printf("Latest version: %s\n", release.TagName)
	fmt.Printf("Downloading %s...\n", asset.Name)

	archivePath, err := downloadRelease(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	defer os.Remove(archivePath)

	fmt.Println("Extracting binary...")
	binaryPath, err := extractBinInstallBinary(archivePath, asset.Name, tool)
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}
	defer os.Remove(binaryPath)

	installDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("failed to create install directory %s: %w", installDir, err)
	}

	if err := installBinary(binaryPath, targetPath); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	fmt.Printf("\nSuccessfully installed %s %s to %s\n", tool.Name, release.TagName, targetPath)
	return nil
}

func resolveBinInstallTool(name string) (binInstallTool, bool) {
	switch name {
	case "codex":
		return binInstallTool{
			Name:        "codex",
			Repo:        "openai/codex",
			BinaryName:  "codex",
			DefaultPath: "~/.local/bin/codex",
			AssetNames:  codexReleaseAssetNames,
		}, true
	case "claude-code":
		return binInstallTool{
			Name:        "claude-code",
			Repo:        "anthropics/claude-code",
			BinaryName:  "claude",
			DefaultPath: "~/.local/bin/claude",
			AssetNames:  claudeCodeReleaseAssetNames,
		}, true
	default:
		return binInstallTool{}, false
	}
}

func codexReleaseAssetNames(goos, goarch string) []string {
	arch, ok := codexReleaseArch(goarch)
	if !ok {
		return nil
	}

	switch goos {
	case "darwin":
		return []string{fmt.Sprintf("codex-%s-apple-darwin.tar.gz", arch)}
	case "linux":
		return []string{fmt.Sprintf("codex-%s-unknown-linux-musl.tar.gz", arch)}
	case "windows":
		return []string{fmt.Sprintf("codex-%s-pc-windows-msvc.exe.zip", arch)}
	default:
		return nil
	}
}

func codexReleaseArch(goarch string) (string, bool) {
	switch goarch {
	case "amd64":
		return "x86_64", true
	case "arm64":
		return "aarch64", true
	default:
		return "", false
	}
}

func claudeCodeReleaseAssetNames(goos, goarch string) []string {
	arch, ok := claudeCodeReleaseArch(goarch)
	if !ok {
		return nil
	}

	switch goos {
	case "darwin":
		return []string{fmt.Sprintf("claude-darwin-%s.tar.gz", arch)}
	case "linux":
		return []string{
			fmt.Sprintf("claude-linux-%s.tar.gz", arch),
			fmt.Sprintf("claude-linux-%s-musl.tar.gz", arch),
		}
	case "windows":
		return []string{fmt.Sprintf("claude-win32-%s.zip", arch)}
	default:
		return nil
	}
}

func claudeCodeReleaseArch(goarch string) (string, bool) {
	switch goarch {
	case "amd64":
		return "x64", true
	case "arm64":
		return "arm64", true
	default:
		return "", false
	}
}

func selectBinInstallAsset(release *githubRelease, tool binInstallTool, goos, goarch string) (githubAsset, error) {
	wantedNames := tool.AssetNames(goos, goarch)
	if len(wantedNames) == 0 {
		return githubAsset{}, fmt.Errorf("%s does not support %s/%s", tool.Name, goos, goarch)
	}

	for _, wantedName := range wantedNames {
		for _, asset := range release.Assets {
			if asset.Name == wantedName {
				return asset, nil
			}
		}
	}

	return githubAsset{}, fmt.Errorf("no release asset found for %s on %s/%s (wanted one of: %s)", tool.Name, goos, goarch, strings.Join(wantedNames, ", "))
}

func fetchLatestReleaseForRepo(repo string) (*githubRelease, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", binInstallGithubAPIBaseURL, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "gsmlg-cli/"+Version)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

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

func extractBinInstallBinary(archivePath, assetName string, tool binInstallTool) (string, error) {
	candidates := binInstallBinaryCandidates(assetName, tool.BinaryName)
	if strings.HasSuffix(assetName, ".zip") {
		return extractBinInstallBinaryFromZip(archivePath, candidates)
	}
	if strings.HasSuffix(assetName, ".tar.gz") {
		return extractBinInstallBinaryFromTarGz(archivePath, candidates)
	}
	return "", fmt.Errorf("unsupported archive format for %q", assetName)
}

func binInstallBinaryCandidates(assetName, binaryName string) map[string]struct{} {
	candidates := map[string]struct{}{
		binaryName: {},
	}
	if !strings.HasSuffix(binaryName, ".exe") {
		candidates[binaryName+".exe"] = struct{}{}
	}

	assetBinaryName := assetName
	for _, suffix := range []string{".tar.gz", ".zip"} {
		assetBinaryName = strings.TrimSuffix(assetBinaryName, suffix)
	}
	candidates[assetBinaryName] = struct{}{}

	return candidates
}

func extractBinInstallBinaryFromTarGz(archivePath string, candidates map[string]struct{}) (string, error) {
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

		if _, ok := candidates[filepath.Base(header.Name)]; !ok {
			continue
		}

		return copyBinInstallBinaryToTemp(tr)
	}

	return "", fmt.Errorf("binary not found in archive")
}

func extractBinInstallBinaryFromZip(archivePath string, candidates map[string]struct{}) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if _, ok := candidates[filepath.Base(f.Name)]; !ok {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		return copyBinInstallBinaryToTemp(rc)
	}

	return "", fmt.Errorf("binary not found in archive")
}

func copyBinInstallBinaryToTemp(r io.Reader) (string, error) {
	tmpFile, err := os.CreateTemp("", "gsmlg-cli-bin-install-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, r); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}
