package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiddingbaby/agx/internal/adapters/claudeconfig"
	"github.com/kiddingbaby/agx/internal/adapters/codexconfig"
	"github.com/kiddingbaby/agx/internal/adapters/lockfile"
	"github.com/kiddingbaby/agx/internal/adapters/opjournal"
	"github.com/kiddingbaby/agx/internal/adapters/profilefile"
	"github.com/kiddingbaby/agx/internal/config"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func newProfileRoot(t *testing.T) (*Root, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()

	home := t.TempDir()
	storeDir := filepath.Join(home, ".config", "agx")
	t.Setenv("HOME", home)

	profiles, err := profilefile.NewRepository(filepath.Join(storeDir, "profiles"))
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	state := profilefile.NewStateRepository(filepath.Join(storeDir, "state.yaml"))
	paths, err := config.DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error = %v", err)
	}
	profileService := usecase.NewProfileService(
		profiles,
		state,
		codexconfig.NewSyncer(paths.CodexConfigPath, paths.BackupsDir, "agx"),
		claudeconfig.NewSyncer(paths.ClaudeSettingsPath, paths.BackupsDir, "agx"),
		nil,
	)
	profileService.SetMutationLocker(lockfile.New(filepath.Join(storeDir, "agx.lock")))
	profileService.SetOperationJournal(opjournal.New(filepath.Join(storeDir, "ops", "current.yaml")))

	root := New(profileService, BuildInfo{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdin = strings.NewReader("")
	root.stdout = stdout
	root.stderr = stderr
	root.isTTY = func() bool { return false }
	return root, stdout, stderr, home
}

func newProfileServiceForDir(t *testing.T, home string) *usecase.ProfileService {
	t.Helper()

	storeDir := filepath.Join(home, ".config", "agx")
	profiles, err := profilefile.NewRepository(filepath.Join(storeDir, "profiles"))
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	state := profilefile.NewStateRepository(filepath.Join(storeDir, "state.yaml"))
	paths, err := config.DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error = %v", err)
	}
	service := usecase.NewProfileService(
		profiles,
		state,
		codexconfig.NewSyncer(paths.CodexConfigPath, paths.BackupsDir, "agx"),
		claudeconfig.NewSyncer(paths.ClaudeSettingsPath, paths.BackupsDir, "agx"),
		nil,
	)
	service.SetMutationLocker(lockfile.New(filepath.Join(storeDir, "agx.lock")))
	service.SetOperationJournal(opjournal.New(filepath.Join(storeDir, "ops", "current.yaml")))
	return service
}

func setInteractiveInput(root *Root, input string) {
	root.stdin = strings.NewReader(input)
	root.isTTY = func() bool { return true }
}

func TestVersionCommand(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)
	root.build = BuildInfo{
		Version: "1.2.3",
		Commit:  "abcdef0",
		Date:    "2026-04-25T13:00:00Z",
	}

	if code := root.Execute([]string{"version"}); code != 0 {
		t.Fatalf("version code=%d want 0 stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "agx 1.2.3") || !strings.Contains(got, "commit=abcdef0") || !strings.Contains(got, "date=2026-04-25T13:00:00Z") {
		t.Fatalf("stdout=%q want version details", got)
	}
}

func TestProfileLifecycleWithBindUnbindAndRestore(t *testing.T) {
	root, stdout, stderr, home := newProfileRoot(t)

	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("profile = \"before\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte("{\"env\":{\"KEEP\":\"1\"}}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}

	content, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	text := string(content)
	if text != "profile = \"before\"\n" {
		t.Fatalf("config=%q want add without --bind to keep codex config unchanged", text)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex,claude"}); code != 0 {
		t.Fatalf("edit bind code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Updated relay bindings: relay-a") || !strings.Contains(got, "bind claude") || !strings.Contains(got, "bind codex") {
		t.Fatalf("stdout=%q want bind output", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls"}); code != 0 {
		t.Fatalf("ls code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "agents=codex,claude") {
		t.Fatalf("stdout=%q want bound multi-agent summary", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"show", "relay-a"}); code != 0 {
		t.Fatalf("show code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "agents=codex,claude") || !strings.Contains(got, "codex status=applied") || !strings.Contains(got, "claude status=applied") {
		t.Fatalf("stdout=%q want binding details", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "ls", "--agent", "codex"}); code != 0 {
		t.Fatalf("backup ls code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Backups for codex:") || !strings.Contains(got, "backup_id=before-codex-sync-") || !strings.Contains(got, "restore_mode=restore_file") {
		t.Fatalf("stdout=%q want backup list", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--unbind", "codex"}); code != 0 {
		t.Fatalf("edit unbind code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Updated relay bindings: relay-a") || !strings.Contains(got, "unbind codex") {
		t.Fatalf("stdout=%q want unbind output", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"show", "relay-a"}); code != 0 {
		t.Fatalf("show after clear code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "agents=claude") || strings.Contains(got, "codex status=") {
		t.Fatalf("stdout=%q want codex binding removed", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"restore", "--agent", "codex"}); code != 0 {
		t.Fatalf("restore code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Restored agent: codex") {
		t.Fatalf("stdout=%q want restore output", got)
	}
}

func TestEditAutoSyncsActiveAgents(t *testing.T) {
	root, stdout, stderr, home := newProfileRoot(t)

	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("profile = \"before\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte("{\"env\":{\"KEEP\":\"1\"}}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex,claude"}); code != 0 {
		t.Fatalf("edit bind code=%d want 0 stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--base-url", "https://relay-new.example/v1", "--api-key", "sk-rotated"}); code != 0 {
		t.Fatalf("edit code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Edited relay: relay-a") || !strings.Contains(got, "base_url=https://relay-new.example/v1") || !strings.Contains(got, "api_key=sk-rotated") {
		t.Fatalf("stdout=%q want edited relay output", got)
	}

	codexConfig, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("ReadFile(codex) error = %v", err)
	}
	if !strings.Contains(string(codexConfig), "base_url = \"https://relay-new.example/v1\"") {
		t.Fatalf("codex config=%q want updated base_url", string(codexConfig))
	}

	claudeConfig, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("ReadFile(claude) error = %v", err)
	}
	claudeText := string(claudeConfig)
	if !strings.Contains(claudeText, "\"ANTHROPIC_BASE_URL\": \"https://relay-new.example/v1\"") {
		t.Fatalf("claude config=%q want updated base url", claudeText)
	}
	if !strings.Contains(claudeText, "\"apiKeyHelper\": \"agx __api-key relay-a\"") {
		t.Fatalf("claude config=%q want helper still present", claudeText)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls"}); code != 0 {
		t.Fatalf("ls code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "agents=codex,claude") {
		t.Fatalf("stdout=%q want bound agents after auto-sync", got)
	}
}

func TestAddAndEditInteractivePrompts(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)
	setInteractiveInput(root, "relay-a\nhttps://relay.example/v1\nsk-a\n")

	if code := root.Execute([]string{"add"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Relay name: ") || !strings.Contains(got, "Base URL: ") || !strings.Contains(got, "API key: ") || !strings.Contains(got, "Added relay: relay-a") {
		t.Fatalf("stdout=%q want interactive add flow", got)
	}

	stdout.Reset()
	stderr.Reset()
	setInteractiveInput(root, "2\nsk-rotated\n\n")
	if code := root.Execute([]string{"edit", "relay-a"}); code != 0 {
		t.Fatalf("edit code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Current relay: relay-a") || !strings.Contains(got, "base_url=https://relay.example/v1") || !strings.Contains(got, "api_key=[hidden]") || !strings.Contains(got, "Edit [1 url, 2 key, Enter done]: ") || !strings.Contains(got, "API key [keep current]: ") || !strings.Contains(got, "api_key=sk-rotated") {
		t.Fatalf("stdout=%q want interactive edit flow", got)
	}
}

func TestInvalidCLIArgsReturnUsage(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)

	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a", "--agent", "codex"}); code == 0 {
		t.Fatal("invalid add args unexpectedly succeeded")
	}
	if got := stderr.String(); !strings.Contains(got, "Usage: agx add <relay> --base-url URL --api-key KEY [--bind codex,claude,gemini] [-o json]") {
		t.Fatalf("stderr=%q want add usage", got)
	}

	stderr.Reset()
	if code := root.Execute([]string{"restore", "codex"}); code == 0 {
		t.Fatal("restore positional agent unexpectedly succeeded")
	}
	if got := stderr.String(); !strings.Contains(got, "Usage: agx restore --agent codex|claude|gemini [--to BACKUP_ID] [-o json]") {
		t.Fatalf("stderr=%q want restore usage", got)
	}

	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a"}); code == 0 {
		t.Fatal("edit without flags unexpectedly succeeded in non-interactive mode")
	}
	if got := stderr.String(); !strings.Contains(got, "Error: edit requires at least one of --base-url, --api-key, --bind, or --unbind") {
		t.Fatalf("stderr=%q want edit error", got)
	}
}

func TestUnknownCommandListsCurrentTopLevelCommands(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)

	if code := root.Execute([]string{"use"}); code == 0 {
		t.Fatal("unknown command unexpectedly succeeded")
	}
	got := stderr.String()
	if !strings.Contains(got, "Error: unknown command: use") {
		t.Fatalf("stderr=%q want unknown command error", got)
	}
	if !strings.Contains(got, "Supported commands: add, edit, ls, show, restore, backup, doctor, rm, version") {
		t.Fatalf("stderr=%q want supported commands list", got)
	}
}

func TestInvalidAgentShowsDirectError(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)

	for _, args := range [][]string{
		{"backup", "ls", "--agent", "wrong"},
		{"restore", "--agent", "wrong"},
	} {
		stderr.Reset()
		if code := root.Execute(args); code == 0 {
			t.Fatalf("%v unexpectedly succeeded", args)
		}
		if got := stderr.String(); !strings.Contains(got, "Agent must be one of: codex, claude, gemini.") {
			t.Fatalf("%v stderr=%q want invalid agent error", args, got)
		}
	}
}

func TestJSONOutputsExposeCurrentFields(t *testing.T) {
	root, stdout, stderr, home := newProfileRoot(t)

	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("profile = \"before\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex", "-o", "json"}); code != 0 {
		t.Fatalf("edit bind json code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "\"relay\"") || !strings.Contains(got, "\"changes\"") || !strings.Contains(got, "\"action\":\"bind\"") {
		t.Fatalf("stdout=%q want relay binding json fields", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls", "-o", "json"}); code != 0 {
		t.Fatalf("ls json code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "\"relays\"") || !strings.Contains(got, "\"agents\":[\"codex\"]") {
		t.Fatalf("stdout=%q want ls json fields", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls", "--agent", "codex", "-o", "json"}); code != 0 {
		t.Fatalf("ls agent json code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "\"agent\":\"codex\"") || !strings.Contains(got, "\"current_relay\":\"relay-a\"") || !strings.Contains(got, "\"current\":true") {
		t.Fatalf("stdout=%q want ls --agent json fields", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"show", "relay-a", "-o", "json"}); code != 0 {
		t.Fatalf("show json code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "\"relay\"") || !strings.Contains(got, "\"agent_bindings\"") || !strings.Contains(got, "\"status\":\"applied\"") {
		t.Fatalf("stdout=%q want show json bindings", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "ls", "--agent", "codex", "-o", "json"}); code != 0 {
		t.Fatalf("backup ls json code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "\"agent\":\"codex\"") || !strings.Contains(got, "\"backups\"") {
		t.Fatalf("stdout=%q want backup ls json fields", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"restore", "--agent", "codex", "-o", "json"}); code != 0 {
		t.Fatalf("restore json code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "\"agent\":\"codex\"") || !strings.Contains(got, "\"backup\"") {
		t.Fatalf("stdout=%q want restore json fields", got)
	}
}

func TestEditRejectsOverlappingBindAndUnbind(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}

	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex", "--unbind", "codex"}); code == 0 {
		t.Fatal("overlapping bind/unbind unexpectedly succeeded")
	}
	if got := stderr.String(); !strings.Contains(got, "bind and unbind contain overlapping agents: codex") {
		t.Fatalf("stderr=%q want overlapping agent error", got)
	}
}

func TestEditDeduplicatesRepeatedBindFlagsInOutput(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex", "--bind", "codex,claude"}); code != 0 {
		t.Fatalf("edit code=%d want 0 stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if strings.Count(got, "bind codex") != 1 || strings.Count(got, "bind claude") != 1 {
		t.Fatalf("stdout=%q want deduplicated bind output", got)
	}
}

func TestEditRejectsEmptyAgentListValues(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}

	for _, args := range [][]string{
		{"edit", "relay-a", "--bind", ""},
		{"edit", "relay-a", "--bind", "codex,"},
		{"edit", "relay-a", "--unbind", ",claude"},
	} {
		stderr.Reset()
		if code := root.Execute(args); code == 0 {
			t.Fatalf("%v unexpectedly succeeded", args)
		}
	}
	if got := stderr.String(); !strings.Contains(got, "agent list") {
		t.Fatalf("stderr=%q want agent list parse error", got)
	}
}

func TestEditUnbindRequiresCurrentRelayBinding(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay-a.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add relay-a code=%d want 0 stderr=%q", code, stderr.String())
	}
	if code := root.Execute([]string{"add", "relay-b", "--base-url", "https://relay-b.example/v1", "--api-key", "sk-b"}); code != 0 {
		t.Fatalf("add relay-b code=%d want 0 stderr=%q", code, stderr.String())
	}
	if code := root.Execute([]string{"edit", "relay-b", "--bind", "codex"}); code != 0 {
		t.Fatalf("edit bind code=%d want 0 stderr=%q", code, stderr.String())
	}

	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--unbind", "codex"}); code == 0 {
		t.Fatal("unbind from non-current relay unexpectedly succeeded")
	}
	if got := stderr.String(); !strings.Contains(got, "codex is not currently bound to relay relay-a") {
		t.Fatalf("stderr=%q want relay-specific unbind error", got)
	}
}

func TestInternalAPIKeyHelper(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example", "--api-key", "sk-secret"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{internalAPIKeyCommand, "relay-a"}); code != 0 {
		t.Fatalf("internal key code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "sk-secret" {
		t.Fatalf("stdout=%q want raw api key", got)
	}
}

func TestSetClearAndRestoreClearOperationJournal(t *testing.T) {
	root, stdout, stderr, home := newProfileRoot(t)

	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("profile = \"before\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d want 0 stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex"}); code != 0 {
		t.Fatalf("edit bind code=%d want 0 stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "agx", "ops", "current.yaml")); !os.IsNotExist(err) {
		t.Fatalf("operation journal should be cleared after bind, err=%v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"restore", "--agent", "codex"}); code != 0 {
		t.Fatalf("restore code=%d want 0 stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "agx", "ops", "current.yaml")); !os.IsNotExist(err) {
		t.Fatalf("operation journal should be cleared after restore, err=%v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex"}); code != 0 {
		t.Fatalf("edit bind code=%d want 0 stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--unbind", "codex"}); code != 0 {
		t.Fatalf("edit unbind code=%d want 0 stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "agx", "ops", "current.yaml")); !os.IsNotExist(err) {
		t.Fatalf("operation journal should be cleared after unbind, err=%v", err)
	}
}

func TestDoctorReportsOKAndUnfinishedOperation(t *testing.T) {
	root, stdout, stderr, home := newProfileRoot(t)

	if code := root.Execute([]string{"doctor"}); code != 0 {
		t.Fatalf("doctor code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Doctor: ok") {
		t.Fatalf("stdout=%q want doctor ok", got)
	}

	now := time.Now().UTC()
	journal := opjournal.New(filepath.Join(home, ".config", "agx", "ops", "current.yaml"))
	if err := journal.Begin(ports.OperationRecord{
		ID:        "op-test",
		Command:   "set",
		Agent:     domainprofile.AgentCodex,
		Profile:   "relay-a",
		Stage:     "config_written",
		StartedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("journal Begin() error = %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"doctor"}); code != 1 {
		t.Fatalf("doctor code=%d want 1 stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "Doctor: issues found") || !strings.Contains(got, "unfinished_operation id=op-test") {
		t.Fatalf("stdout=%q want unfinished operation issue", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"doctor", "-o", "json"}); code != 1 {
		t.Fatalf("doctor json code=%d want 1 stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if got := stdout.String(); !strings.Contains(got, "\"ok\":false") || !strings.Contains(got, "\"code\":\"unfinished_operation\"") {
		t.Fatalf("stdout=%q want doctor json issue", got)
	}
}

func TestConcurrentSetDifferentAgentsDoesNotLoseBindings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("profile = \"before\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	serviceA := newProfileServiceForDir(t, home)
	serviceB := newProfileServiceForDir(t, home)
	if _, err := serviceA.Add("relay-a", usecase.AddProfileInput{BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := serviceA.AgentSet(domainprofile.AgentCodex, "relay-a")
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := serviceB.AgentSet(domainprofile.AgentClaude, "relay-a")
		errs <- err
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("AgentSet() error = %v", err)
		}
	}

	state, err := serviceA.State()
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Codex.SourceProfile != "relay-a" || state.Claude.SourceProfile != "relay-a" {
		t.Fatalf("state = %+v, want codex and claude active on relay-a", state)
	}
}
