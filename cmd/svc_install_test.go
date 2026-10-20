package cmd

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestResolveSvcInstallApp(t *testing.T) {
	tests := []struct {
		name         string
		repo         string
		appName      string
		defaultDir   string
		systemdUnit  string
		launchdLabel string
		serviceTitle string
	}{
		{
			name:         "commander",
			repo:         "gsmlg-dev/gsmlg_umbrella",
			appName:      "commander",
			defaultDir:   "~/.local/share/gsmlg_commander",
			systemdUnit:  "gsmlg-commander.service",
			launchdLabel: "com.gsmlg.commander",
			serviceTitle: "GSMLG Commander",
		},
		{
			name:         "host_agent",
			repo:         "gsmlg-opt/backplane",
			appName:      "host_agent",
			defaultDir:   "~/.local/share/host_agent",
			systemdUnit:  "gsmlg-host-agent.service",
			launchdLabel: "com.gsmlg.host_agent",
			serviceTitle: "GSMLG Host Agent",
		},
		{
			name:         "secrethub_agent",
			repo:         "gsmlg-dev/secrethub",
			appName:      "secrethub_agent",
			defaultDir:   "~/.local/share/secrethub_agent",
			systemdUnit:  "gsmlg-secrethub-agent.service",
			launchdLabel: "com.gsmlg.secrethub_agent",
			serviceTitle: "GSMLG SecretHub Agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, ok := resolveSvcInstallApp(tt.name)
			if !ok {
				t.Fatalf("resolveSvcInstallApp(%q) returned !ok", tt.name)
			}
			if app.Repo != tt.repo {
				t.Fatalf("Repo = %q, want %q", app.Repo, tt.repo)
			}
			if app.AppName != tt.appName {
				t.Fatalf("AppName = %q, want %q", app.AppName, tt.appName)
			}
			if app.DefaultDir != tt.defaultDir {
				t.Fatalf("DefaultDir = %q, want %q", app.DefaultDir, tt.defaultDir)
			}
			if app.SystemdUnit != tt.systemdUnit {
				t.Fatalf("SystemdUnit = %q, want %q", app.SystemdUnit, tt.systemdUnit)
			}
			if app.LaunchdLabel != tt.launchdLabel {
				t.Fatalf("LaunchdLabel = %q, want %q", app.LaunchdLabel, tt.launchdLabel)
			}
			if app.ServiceTitle != tt.serviceTitle {
				t.Fatalf("ServiceTitle = %q, want %q", app.ServiceTitle, tt.serviceTitle)
			}
		})
	}
}

func TestResolveSvcInstallAppRejectsUnknownApp(t *testing.T) {
	if _, ok := resolveSvcInstallApp("unknown"); ok {
		t.Fatal("resolveSvcInstallApp returned ok for an unknown app")
	}
}

func TestSelectSvcInstallAsset(t *testing.T) {
	tests := []struct {
		name      string
		appName   string
		tag       string
		goos      string
		goarch    string
		assets    []githubAsset
		wantAsset string
	}{
		{
			name:    "commander uses its release archive",
			appName: "commander",
			tag:     "v5.6.4",
			goos:    "linux",
			goarch:  "amd64",
			assets: []githubAsset{
				{Name: "gsmlg.tar.gz"},
				{Name: "commander.tar.gz"},
			},
			wantAsset: "commander.tar.gz",
		},
		{
			name:    "host agent linux amd64 uses x64 release",
			appName: "host_agent",
			tag:     "v0.3.1",
			goos:    "linux",
			goarch:  "amd64",
			assets: []githubAsset{
				{Name: "host_agent-0.3.1-linux-x64.tar.gz.sha256"},
				{Name: "host_agent-0.3.1-linux-arm64.tar.gz"},
				{Name: "host_agent-0.3.1-linux-x64.tar.gz"},
			},
			wantAsset: "host_agent-0.3.1-linux-x64.tar.gz",
		},
		{
			name:    "host agent macos arm64",
			appName: "host_agent",
			tag:     "v0.3.1",
			goos:    "darwin",
			goarch:  "arm64",
			assets: []githubAsset{
				{Name: "host_agent-0.3.1-macos-x64.tar.gz"},
				{Name: "host_agent-0.3.1-macos-arm64.tar.gz"},
			},
			wantAsset: "host_agent-0.3.1-macos-arm64.tar.gz",
		},
		{
			name:    "secrethub agent linux amd64 uses tagged platform release",
			appName: "secrethub_agent",
			tag:     "v1.0.0-rc8",
			goos:    "linux",
			goarch:  "amd64",
			assets: []githubAsset{
				{Name: "secrethub_core-v1.0.0-rc8-linux-amd64.tar.gz"},
				{Name: "secrethub_agent-v1.0.0-rc8-linux-amd64.tar.gz"},
			},
			wantAsset: "secrethub_agent-v1.0.0-rc8-linux-amd64.tar.gz",
		},
		{
			name:    "secrethub agent falls back to older generic archive",
			appName: "secrethub_agent",
			tag:     "v1.0.0-rc7",
			goos:    "darwin",
			goarch:  "arm64",
			assets: []githubAsset{
				{Name: "secrethub_core-v1.0.0-rc7.tar.gz"},
				{Name: "secrethub_agent-v1.0.0-rc7.tar.gz"},
			},
			wantAsset: "secrethub_agent-v1.0.0-rc7.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, ok := resolveSvcInstallApp(tt.appName)
			if !ok {
				t.Fatalf("resolveSvcInstallApp(%q) returned !ok", tt.appName)
			}

			asset, err := selectSvcInstallAsset(&githubRelease{TagName: tt.tag, Assets: tt.assets}, app, tt.goos, tt.goarch)
			if err != nil {
				t.Fatalf("selectSvcInstallAsset returned error: %v", err)
			}
			if asset.Name != tt.wantAsset {
				t.Fatalf("asset.Name = %q, want %q", asset.Name, tt.wantAsset)
			}
		})
	}
}

func TestSelectSvcInstallAssetReportsUnsupportedPlatform(t *testing.T) {
	app, ok := resolveSvcInstallApp("host_agent")
	if !ok {
		t.Fatal("resolveSvcInstallApp returned !ok for host_agent")
	}

	if _, err := selectSvcInstallAsset(&githubRelease{TagName: "v0.3.1"}, app, "windows", "amd64"); err == nil {
		t.Fatal("selectSvcInstallAsset returned nil error for unsupported platform")
	}
}

func TestFindSvcInstallReleaseRoot(t *testing.T) {
	t.Run("direct release root", func(t *testing.T) {
		dir := t.TempDir()
		createExecutable(t, filepath.Join(dir, "bin", "secrethub_agent"))

		got, err := findSvcInstallReleaseRoot(dir, "secrethub_agent")
		if err != nil {
			t.Fatalf("findSvcInstallReleaseRoot returned error: %v", err)
		}
		if got != dir {
			t.Fatalf("root = %q, want %q", got, dir)
		}
	})

	t.Run("nested release root", func(t *testing.T) {
		dir := t.TempDir()
		want := filepath.Join(dir, "commander")
		createExecutable(t, filepath.Join(want, "bin", "commander"))

		got, err := findSvcInstallReleaseRoot(dir, "commander")
		if err != nil {
			t.Fatalf("findSvcInstallReleaseRoot returned error: %v", err)
		}
		if got != want {
			t.Fatalf("root = %q, want %q", got, want)
		}
	})
}

func TestEnsureSecretHubAgentConfigGeneratesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	installDir := filepath.Join(home, ".local", "share", "secrethub_agent")
	writeSecretHubAgentScript(t, installDir, `#!/bin/sh
if [ "$#" -ne 2 ] || [ "$1" != config ] || [ "$2" != generate ]; then
  exit 64
fi
printf '[agent]\nname = "installed"\n'
`)

	if err := ensureSecretHubAgentConfig(installDir); err != nil {
		t.Fatalf("ensureSecretHubAgentConfig returned error: %v", err)
	}
	configPath := filepath.Join(home, ".config", "secrethub_agent", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if want := "[agent]\nname = \"installed\"\n"; string(data) != want {
		t.Fatalf("config = %q, want %q", data, want)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestEnsureSecretHubAgentConfigPreservesExistingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	installDir := filepath.Join(home, ".local", "share", "secrethub_agent")
	writeSecretHubAgentScript(t, installDir, "#!/bin/sh\ntouch \"$HOME/generator-called\"\nexit 1\n")
	configPath := filepath.Join(home, ".config", "secrethub_agent", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	want := "[agent]\nname = \"existing\"\n"
	if err := os.WriteFile(configPath, []byte(want), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := ensureSecretHubAgentConfig(installDir); err != nil {
		t.Fatalf("ensureSecretHubAgentConfig returned error: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("config = %q, want %q", data, want)
	}
	if _, err := os.Stat(filepath.Join(home, "generator-called")); !os.IsNotExist(err) {
		t.Fatalf("generator marker Stat error = %v, want file not to exist", err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("existing config permissions = %o, want 640", info.Mode().Perm())
	}
}

func TestEnsureSecretHubAgentConfigCanRetryAfterGenerationFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	installDir := filepath.Join(home, ".local", "share", "secrethub_agent")
	writeSecretHubAgentScript(t, installDir, `#!/bin/sh
if [ ! -e "$HOME/generator-retry" ]; then
  touch "$HOME/generator-retry"
  printf 'incomplete config'
  exit 1
fi
printf '[agent]\nname = "retried"\n'
`)

	if err := ensureSecretHubAgentConfig(installDir); err == nil {
		t.Fatal("ensureSecretHubAgentConfig returned nil error for failed generation")
	}
	configPath := filepath.Join(home, ".config", "secrethub_agent", "config.toml")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("failed config Stat error = %v, want file not to exist", err)
	}
	if err := ensureSecretHubAgentConfig(installDir); err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[agent]\nname = \"retried\"\n"; string(data) != want {
		t.Fatalf("retried config = %q, want %q", data, want)
	}
}

func writeSecretHubAgentScript(t *testing.T, installDir, script string) {
	t.Helper()
	path := filepath.Join(installDir, "bin", "secrethub_agent")
	createExecutable(t, path)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func TestWriteSystemdUserService(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	app, ok := resolveSvcInstallApp("commander")
	if !ok {
		t.Fatal("resolveSvcInstallApp returned !ok for commander")
	}
	installDir := filepath.Join(home, ".local", "share", "gsmlg_commander")

	servicePath, err := writeSystemdUserService(app, installDir)
	if err != nil {
		t.Fatalf("writeSystemdUserService returned error: %v", err)
	}

	wantPath := filepath.Join(home, ".config", "systemd", "user", "gsmlg-commander.service")
	if servicePath != wantPath {
		t.Fatalf("servicePath = %q, want %q", servicePath, wantPath)
	}

	data, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) returned error: %v", servicePath, err)
	}
	content := string(data)
	for _, want := range []string{
		"Description=GSMLG Commander",
		"WorkingDirectory=" + installDir,
		"ExecStart=" + filepath.Join(installDir, "bin", "commander") + " start",
		"Restart=always",
		"WantedBy=default.target",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("systemd service missing %q in:\n%s", want, content)
		}
	}
}

func TestWriteSystemdUserServiceSecretHubAgentConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	app, ok := resolveSvcInstallApp("secrethub_agent")
	if !ok {
		t.Fatal("resolveSvcInstallApp returned !ok for secrethub_agent")
	}
	installDir := filepath.Join(home, ".local", "share", "secrethub_agent")
	servicePath, err := writeSystemdUserService(app, installDir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Environment=\"SECRET_HUB_AGENT_CONFIG_FILE=" + filepath.Join(home, ".config", "secrethub_agent", "config.toml") + "\"\n",
		"ExecStart=" + filepath.Join(installDir, "bin", "secrethub_agent") + " start\n",
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("systemd service missing %q in:\n%s", want, data)
		}
	}
}

func TestRegisterSystemdUserServiceRunsUserSystemctlCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	app, ok := resolveSvcInstallApp("commander")
	if !ok {
		t.Fatal("resolveSvcInstallApp returned !ok for commander")
	}

	var calls []string
	restore := replaceSvcInstallRunCommand(func(name string, args ...string) error {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return nil
	})
	defer restore()

	if err := registerSystemdUserService(app, filepath.Join(home, ".local", "share", "gsmlg_commander")); err != nil {
		t.Fatalf("registerSystemdUserService returned error: %v", err)
	}

	want := []string{
		"systemctl --user daemon-reload",
		"systemctl --user enable gsmlg-commander.service",
		"systemctl --user restart gsmlg-commander.service",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("systemctl calls = %#v, want %#v", calls, want)
	}
}

func TestWriteLaunchdAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	app, ok := resolveSvcInstallApp("host_agent")
	if !ok {
		t.Fatal("resolveSvcInstallApp returned !ok for host_agent")
	}
	installDir := filepath.Join(home, ".local", "share", "host_agent")

	plistPath, err := writeLaunchdAgent(app, installDir)
	if err != nil {
		t.Fatalf("writeLaunchdAgent returned error: %v", err)
	}

	wantPath := filepath.Join(home, "Library", "LaunchAgents", "com.gsmlg.host_agent.plist")
	if plistPath != wantPath {
		t.Fatalf("plistPath = %q, want %q", plistPath, wantPath)
	}

	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) returned error: %v", plistPath, err)
	}
	content := string(data)
	for _, want := range []string{
		"<string>com.gsmlg.host_agent</string>",
		"<string>" + filepath.Join(installDir, "bin", "host_agent") + "</string>",
		"<string>start</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"<key>WorkingDirectory</key>",
		"<string>" + installDir + "</string>",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("launchd plist missing %q in:\n%s", want, content)
		}
	}
}

func TestWriteLaunchdAgentSecretHubAgentConfig(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home & agent")
	t.Setenv("HOME", home)
	app, ok := resolveSvcInstallApp("secrethub_agent")
	if !ok {
		t.Fatal("resolveSvcInstallApp returned !ok for secrethub_agent")
	}
	installDir := filepath.Join(home, ".local", "share", "secrethub_agent")
	plistPath, err := writeLaunchdAgent(app, installDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "Library", "LaunchAgents", "com.gsmlg.secrethub_agent.plist"); plistPath != want {
		t.Fatalf("plistPath = %q, want %q", plistPath, want)
	}
	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	type node struct {
		XMLName  xml.Name
		Text     string `xml:",chardata"`
		Children []node `xml:",any"`
	}
	var plist node
	if err := xml.Unmarshal(data, &plist); err != nil {
		t.Fatalf("invalid plist XML: %v", err)
	}
	if plist.XMLName.Local != "plist" || len(plist.Children) != 1 || plist.Children[0].XMLName.Local != "dict" {
		t.Fatalf("unexpected plist root: %#v", plist)
	}
	value := func(dict node, key string) node {
		t.Helper()
		for i := 0; i+1 < len(dict.Children); i += 2 {
			if dict.Children[i].XMLName.Local == "key" && dict.Children[i].Text == key {
				return dict.Children[i+1]
			}
		}
		t.Fatalf("plist dictionary missing key %q", key)
		return node{}
	}
	dict := plist.Children[0]
	environment := value(dict, "EnvironmentVariables")
	if environment.XMLName.Local != "dict" {
		t.Fatalf("EnvironmentVariables type = %q, want dict", environment.XMLName.Local)
	}
	config := value(environment, "SECRET_HUB_AGENT_CONFIG_FILE")
	if want := filepath.Join(home, ".config", "secrethub_agent", "config.toml"); config.XMLName.Local != "string" || config.Text != want {
		t.Fatalf("config environment = %#v, want string %q", config, want)
	}
	arguments := value(dict, "ProgramArguments")
	wantArgs := []string{filepath.Join(installDir, "bin", "secrethub_agent"), "start"}
	var gotArgs []string
	for _, argument := range arguments.Children {
		gotArgs = append(gotArgs, argument.Text)
	}
	if arguments.XMLName.Local != "array" || !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("ProgramArguments = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestRegisterLaunchdAgentRunsLaunchctlCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	app, ok := resolveSvcInstallApp("host_agent")
	if !ok {
		t.Fatal("resolveSvcInstallApp returned !ok for host_agent")
	}

	var calls []string
	restore := replaceSvcInstallRunCommand(func(name string, args ...string) error {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return nil
	})
	defer restore()

	installDir := filepath.Join(home, ".local", "share", "host_agent")
	if err := registerLaunchdAgent(app, installDir); err != nil {
		t.Fatalf("registerLaunchdAgent returned error: %v", err)
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.gsmlg.host_agent.plist")
	serviceTarget := "gui/" + strconv.Itoa(os.Getuid()) + "/com.gsmlg.host_agent"
	want := []string{
		"launchctl bootout " + serviceTarget,
		"launchctl bootstrap gui/" + strconv.Itoa(os.Getuid()) + " " + plistPath,
		"launchctl enable " + serviceTarget,
		"launchctl kickstart -k " + serviceTarget,
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("launchctl calls = %#v, want %#v", calls, want)
	}
}

func replaceSvcInstallRunCommand(fn func(string, ...string) error) func() {
	original := svcInstallRunCommand
	svcInstallRunCommand = fn
	return func() {
		svcInstallRunCommand = original
	}
}

func createExecutable(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
