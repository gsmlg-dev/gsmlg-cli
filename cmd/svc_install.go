/*
Copyright © 2026 Jonathan Gao <gsmlg.com@gmail.com>
*/
package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const supportedSvcInstallApps = "commander, host_agent, secrethub_agent"

type svcInstallApp struct {
	Name         string
	Repo         string
	AppName      string
	DefaultDir   string
	SystemdUnit  string
	LaunchdLabel string
	ServiceTitle string
	AssetNames   func(tagName, goos, goarch string) []string
}

var svcInstallCmd = &cobra.Command{
	Use:   "svc-install <commander|host_agent|secrethub_agent>",
	Short: "Install supported release applications as current-user services",
	Long: `Download a supported release application from GitHub, install it under
the current user's ~/.local/share directory, and register it as a user service.

Supported applications:
  commander        installs gsmlg-dev/gsmlg_umbrella commander to ~/.local/share/gsmlg_commander
  host_agent       installs gsmlg-opt/backplane host_agent to ~/.local/share/host_agent
  secrethub_agent  installs gsmlg-dev/secrethub secrethub_agent to ~/.local/share/secrethub_agent

On Linux, svc-install writes a systemd user unit and restarts it with systemctl --user.
On macOS, svc-install writes a LaunchAgent plist and loads it with launchctl.

Examples:
  gsmlg-cli svc-install commander
  gsmlg-cli svc-install host_agent
  gsmlg-cli svc-install secrethub_agent`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSvcInstall(args[0])
	},
}

var svcInstallRunCommand = func(name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func init() {
	rootCmd.AddCommand(svcInstallCmd)
}

func runSvcInstall(appName string) error {
	app, ok := resolveSvcInstallApp(appName)
	if !ok {
		return fmt.Errorf("unsupported service app %q; supported apps: %s", appName, supportedSvcInstallApps)
	}

	installDir := expandPath(app.DefaultDir)

	fmt.Printf("Runtime: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Fetching latest %s release from %s...\n", app.Name, app.Repo)

	release, err := fetchLatestSvcInstallReleaseForRepo(app.Repo)
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}

	asset, err := selectSvcInstallAsset(release, app, runtime.GOOS, runtime.GOARCH)
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

	fmt.Println("Extracting release...")
	extractDir, err := extractSvcInstallArchive(archivePath, asset.Name)
	if err != nil {
		return fmt.Errorf("failed to extract release: %w", err)
	}
	defer os.RemoveAll(extractDir)

	releaseRoot, err := findSvcInstallReleaseRoot(extractDir, app.AppName)
	if err != nil {
		return err
	}

	fmt.Printf("Installing to %s...\n", installDir)
	if err := installSvcInstallReleaseRoot(releaseRoot, installDir); err != nil {
		return fmt.Errorf("failed to install release: %w", err)
	}

	fmt.Println("Registering user service...")
	if err := registerSvcInstallService(app, installDir, runtime.GOOS); err != nil {
		return err
	}

	fmt.Printf("\nSuccessfully installed %s %s to %s\n", app.Name, release.TagName, installDir)
	return nil
}

func resolveSvcInstallApp(name string) (svcInstallApp, bool) {
	switch name {
	case "commander":
		return svcInstallApp{
			Name:         "commander",
			Repo:         "gsmlg-dev/gsmlg_umbrella",
			AppName:      "commander",
			DefaultDir:   "~/.local/share/gsmlg_commander",
			SystemdUnit:  "gsmlg-commander.service",
			LaunchdLabel: "com.gsmlg.commander",
			ServiceTitle: "GSMLG Commander",
			AssetNames:   commanderSvcReleaseAssetNames,
		}, true
	case "host_agent":
		return svcInstallApp{
			Name:         "host_agent",
			Repo:         "gsmlg-opt/backplane",
			AppName:      "host_agent",
			DefaultDir:   "~/.local/share/host_agent",
			SystemdUnit:  "gsmlg-host-agent.service",
			LaunchdLabel: "com.gsmlg.host_agent",
			ServiceTitle: "GSMLG Host Agent",
			AssetNames:   hostAgentSvcReleaseAssetNames,
		}, true
	case "secrethub_agent":
		return svcInstallApp{
			Name:         "secrethub_agent",
			Repo:         "gsmlg-dev/secrethub",
			AppName:      "secrethub_agent",
			DefaultDir:   "~/.local/share/secrethub_agent",
			SystemdUnit:  "gsmlg-secrethub-agent.service",
			LaunchdLabel: "com.gsmlg.secrethub_agent",
			ServiceTitle: "GSMLG SecretHub Agent",
			AssetNames:   secretHubAgentSvcReleaseAssetNames,
		}, true
	default:
		return svcInstallApp{}, false
	}
}

func commanderSvcReleaseAssetNames(tagName, goos, goarch string) []string {
	switch goos {
	case "linux", "darwin":
		return []string{"commander.tar.gz"}
	default:
		return nil
	}
}

func hostAgentSvcReleaseAssetNames(tagName, goos, goarch string) []string {
	osName, ok := svcInstallPlatformOS(goos)
	if !ok {
		return nil
	}

	arch, ok := hostAgentSvcReleaseArch(goarch)
	if !ok {
		return nil
	}

	return []string{fmt.Sprintf("host_agent-%s-%s-%s.tar.gz", strings.TrimPrefix(tagName, "v"), osName, arch)}
}

func secretHubAgentSvcReleaseAssetNames(tagName, goos, goarch string) []string {
	osName, ok := svcInstallPlatformOS(goos)
	if !ok {
		return nil
	}

	switch goarch {
	case "amd64", "arm64":
	default:
		return nil
	}

	return []string{
		fmt.Sprintf("secrethub_agent-%s-%s-%s.tar.gz", tagName, osName, goarch),
		fmt.Sprintf("secrethub_agent-%s.tar.gz", tagName),
	}
}

func svcInstallPlatformOS(goos string) (string, bool) {
	switch goos {
	case "linux":
		return "linux", true
	case "darwin":
		return "macos", true
	default:
		return "", false
	}
}

func hostAgentSvcReleaseArch(goarch string) (string, bool) {
	switch goarch {
	case "amd64":
		return "x64", true
	case "arm64":
		return "arm64", true
	default:
		return "", false
	}
}

func selectSvcInstallAsset(release *githubRelease, app svcInstallApp, goos, goarch string) (githubAsset, error) {
	wantedNames := app.AssetNames(release.TagName, goos, goarch)
	if len(wantedNames) == 0 {
		return githubAsset{}, fmt.Errorf("%s does not support %s/%s", app.Name, goos, goarch)
	}

	for _, wantedName := range wantedNames {
		for _, asset := range release.Assets {
			if asset.Name == wantedName {
				return asset, nil
			}
		}
	}

	return githubAsset{}, fmt.Errorf("no release asset found for %s on %s/%s (wanted one of: %s)", app.Name, goos, goarch, strings.Join(wantedNames, ", "))
}

func fetchLatestSvcInstallReleaseForRepo(repo string) (*githubRelease, error) {
	url := fmt.Sprintf("%s/%s/releases?per_page=20", binInstallGithubAPIBaseURL, repo)
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

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	for _, release := range releases {
		if !release.Draft {
			return &release, nil
		}
	}

	return nil, fmt.Errorf("no published releases found for %s", repo)
}

func extractSvcInstallArchive(archivePath, assetName string) (string, error) {
	extractDir, err := os.MkdirTemp("", "gsmlg-cli-svc-install-*")
	if err != nil {
		return "", err
	}

	var extractErr error
	switch {
	case strings.HasSuffix(assetName, ".tar.gz"):
		extractErr = extractSvcInstallTarGz(archivePath, extractDir)
	case strings.HasSuffix(assetName, ".zip"):
		extractErr = extractSvcInstallZip(archivePath, extractDir)
	default:
		extractErr = fmt.Errorf("unsupported archive format for %q", assetName)
	}
	if extractErr != nil {
		os.RemoveAll(extractDir)
		return "", extractErr
	}

	return extractDir, nil
}

func extractSvcInstallTarGz(archivePath, extractDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		targetPath, ok, err := svcInstallArchiveTargetPath(extractDir, header.Name)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		mode := header.FileInfo().Mode()
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, mode.Perm()); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if filepath.IsAbs(header.Linkname) {
				return fmt.Errorf("archive contains absolute symlink target %q", header.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return err
			}
		}
	}
}

func extractSvcInstallZip(archivePath, extractDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		targetPath, ok, err := svcInstallArchiveTargetPath(extractDir, f.Name)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		info := f.FileInfo()
		if info.IsDir() {
			if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}

	return nil
}

func svcInstallArchiveTargetPath(extractDir, archiveName string) (string, bool, error) {
	cleanName := filepath.Clean(archiveName)
	if cleanName == "." {
		return "", false, nil
	}
	if filepath.IsAbs(cleanName) {
		return "", false, fmt.Errorf("archive contains absolute path %q", archiveName)
	}

	targetPath := filepath.Join(extractDir, cleanName)
	rel, err := filepath.Rel(extractDir, targetPath)
	if err != nil {
		return "", false, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false, fmt.Errorf("archive path %q escapes extraction directory", archiveName)
	}

	return targetPath, true, nil
}

func findSvcInstallReleaseRoot(extractDir, appName string) (string, error) {
	if svcInstallBinaryExists(extractDir, appName) {
		return extractDir, nil
	}

	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(extractDir, entry.Name())
		if svcInstallBinaryExists(candidate, appName) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("release binary %q not found under extracted release", filepath.Join("bin", appName))
}

func svcInstallBinaryExists(releaseRoot, appName string) bool {
	info, err := os.Stat(filepath.Join(releaseRoot, "bin", appName))
	return err == nil && !info.IsDir()
}

func installSvcInstallReleaseRoot(releaseRoot, installDir string) error {
	parentDir := filepath.Dir(installDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp(parentDir, filepath.Base(installDir)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := copySvcInstallDir(releaseRoot, tmpDir); err != nil {
		return err
	}

	if err := os.RemoveAll(installDir); err != nil {
		return err
	}

	return os.Rename(tmpDir, installDir)
}

func copySvcInstallDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		entryInfo, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}

		mode := entryInfo.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(linkTarget, dstPath); err != nil {
				return err
			}
		case mode.IsDir():
			if err := copySvcInstallDir(srcPath, dstPath); err != nil {
				return err
			}
		case mode.IsRegular():
			if err := copySvcInstallFile(srcPath, dstPath, mode.Perm()); err != nil {
				return err
			}
		}
	}

	return nil
}

func copySvcInstallFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func registerSvcInstallService(app svcInstallApp, installDir, goos string) error {
	switch goos {
	case "linux":
		return registerSystemdUserService(app, installDir)
	case "darwin":
		return registerLaunchdAgent(app, installDir)
	default:
		return fmt.Errorf("svc-install does not support service registration on %s", goos)
	}
}

func registerSystemdUserService(app svcInstallApp, installDir string) error {
	unitPath, err := writeSystemdUserService(app, installDir)
	if err != nil {
		return fmt.Errorf("failed to write systemd user service: %w", err)
	}

	if err := svcInstallRunCommand("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd user services: %w", err)
	}
	if err := svcInstallRunCommand("systemctl", "--user", "enable", app.SystemdUnit); err != nil {
		return fmt.Errorf("failed to enable systemd user service %s: %w", app.SystemdUnit, err)
	}
	if err := svcInstallRunCommand("systemctl", "--user", "restart", app.SystemdUnit); err != nil {
		return fmt.Errorf("failed to restart systemd user service %s: %w", app.SystemdUnit, err)
	}

	fmt.Printf("Wrote systemd user service: %s\n", unitPath)
	return nil
}

func writeSystemdUserService(app svcInstallApp, installDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return "", err
	}

	unitPath := filepath.Join(unitDir, app.SystemdUnit)
	content := fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s start
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, app.ServiceTitle, installDir, filepath.Join(installDir, "bin", app.AppName))

	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return unitPath, nil
}

func registerLaunchdAgent(app svcInstallApp, installDir string) error {
	plistPath, err := writeLaunchdAgent(app, installDir)
	if err != nil {
		return fmt.Errorf("failed to write launchd agent: %w", err)
	}

	domain := fmt.Sprintf("gui/%d", os.Getuid())
	serviceTarget := domain + "/" + app.LaunchdLabel
	_ = svcInstallRunCommand("launchctl", "bootout", serviceTarget)
	if err := svcInstallRunCommand("launchctl", "bootstrap", domain, plistPath); err != nil {
		return fmt.Errorf("failed to bootstrap launchd agent %s: %w", app.LaunchdLabel, err)
	}
	if err := svcInstallRunCommand("launchctl", "enable", serviceTarget); err != nil {
		return fmt.Errorf("failed to enable launchd agent %s: %w", app.LaunchdLabel, err)
	}
	if err := svcInstallRunCommand("launchctl", "kickstart", "-k", serviceTarget); err != nil {
		return fmt.Errorf("failed to start launchd agent %s: %w", app.LaunchdLabel, err)
	}

	fmt.Printf("Wrote launchd agent: %s\n", plistPath)
	return nil
}

func writeLaunchdAgent(app svcInstallApp, installDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	agentDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return "", err
	}

	plistPath := filepath.Join(agentDir, app.LaunchdLabel+".plist")
	binaryPath := filepath.Join(installDir, "bin", app.AppName)
	stdoutPath := filepath.Join(installDir, app.AppName+".out.log")
	stderrPath := filepath.Join(installDir, app.AppName+".err.log")
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>start</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>%s</string>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, xmlText(app.LaunchdLabel), xmlText(binaryPath), xmlText(installDir), xmlText(stdoutPath), xmlText(stderrPath))

	if err := os.WriteFile(plistPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return plistPath, nil
}

func xmlText(value string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return value
	}
	return b.String()
}
