package cmd

import "testing"

func TestResolveBinInstallTool(t *testing.T) {
	tests := []struct {
		name        string
		repo        string
		binaryName  string
		defaultPath string
	}{
		{
			name:        "codex",
			repo:        "openai/codex",
			binaryName:  "codex",
			defaultPath: "~/.local/bin/codex",
		},
		{
			name:        "claude-code",
			repo:        "anthropics/claude-code",
			binaryName:  "claude",
			defaultPath: "~/.local/bin/claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := resolveBinInstallTool(tt.name)
			if !ok {
				t.Fatalf("resolveBinInstallTool(%q) returned !ok", tt.name)
			}
			if tool.Repo != tt.repo {
				t.Fatalf("Repo = %q, want %q", tool.Repo, tt.repo)
			}
			if tool.BinaryName != tt.binaryName {
				t.Fatalf("BinaryName = %q, want %q", tool.BinaryName, tt.binaryName)
			}
			if tool.DefaultPath != tt.defaultPath {
				t.Fatalf("DefaultPath = %q, want %q", tool.DefaultPath, tt.defaultPath)
			}
		})
	}
}

func TestResolveBinInstallToolRejectsUnknownTool(t *testing.T) {
	if _, ok := resolveBinInstallTool("unknown"); ok {
		t.Fatal("resolveBinInstallTool returned ok for an unknown tool")
	}
}

func TestSelectBinInstallAsset(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		goos      string
		goarch    string
		assets    []githubAsset
		wantAsset string
	}{
		{
			name:     "codex linux amd64 prefers cli musl tarball",
			toolName: "codex",
			goos:     "linux",
			goarch:   "amd64",
			assets: []githubAsset{
				{Name: "codex-package-x86_64-unknown-linux-musl.tar.gz"},
				{Name: "codex-x86_64-unknown-linux-musl.zst"},
				{Name: "codex-x86_64-unknown-linux-musl.tar.gz"},
			},
			wantAsset: "codex-x86_64-unknown-linux-musl.tar.gz",
		},
		{
			name:     "codex darwin arm64",
			toolName: "codex",
			goos:     "darwin",
			goarch:   "arm64",
			assets: []githubAsset{
				{Name: "codex-aarch64-apple-darwin.dmg"},
				{Name: "codex-aarch64-apple-darwin.tar.gz"},
			},
			wantAsset: "codex-aarch64-apple-darwin.tar.gz",
		},
		{
			name:     "claude-code linux amd64 prefers non-musl tarball",
			toolName: "claude-code",
			goos:     "linux",
			goarch:   "amd64",
			assets: []githubAsset{
				{Name: "claude-linux-x64-musl.tar.gz"},
				{Name: "claude-linux-x64.tar.gz"},
			},
			wantAsset: "claude-linux-x64.tar.gz",
		},
		{
			name:     "claude-code windows arm64",
			toolName: "claude-code",
			goos:     "windows",
			goarch:   "arm64",
			assets: []githubAsset{
				{Name: "claude-win32-x64.zip"},
				{Name: "claude-win32-arm64.zip"},
			},
			wantAsset: "claude-win32-arm64.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := resolveBinInstallTool(tt.toolName)
			if !ok {
				t.Fatalf("resolveBinInstallTool(%q) returned !ok", tt.toolName)
			}

			asset, err := selectBinInstallAsset(&githubRelease{Assets: tt.assets}, tool, tt.goos, tt.goarch)
			if err != nil {
				t.Fatalf("selectBinInstallAsset returned error: %v", err)
			}
			if asset.Name != tt.wantAsset {
				t.Fatalf("asset.Name = %q, want %q", asset.Name, tt.wantAsset)
			}
		})
	}
}

func TestSelectBinInstallAssetReportsUnsupportedPlatform(t *testing.T) {
	tool, ok := resolveBinInstallTool("codex")
	if !ok {
		t.Fatal("resolveBinInstallTool returned !ok for codex")
	}

	if _, err := selectBinInstallAsset(&githubRelease{}, tool, "plan9", "amd64"); err == nil {
		t.Fatal("selectBinInstallAsset returned nil error for unsupported platform")
	}
}
