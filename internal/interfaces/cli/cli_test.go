package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiddingbaby/agx/internal/app"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

type fakeNativeRuntime struct {
	runCalls []fakeRunCall
}

type fakeRunCall struct {
	agent       domainprofile.Agent
	contextPath string
	profile     domainprofile.Profile
	args        []string
}

func (f *fakeNativeRuntime) Run(agent domainprofile.Agent, contextPath string, profile domainprofile.Profile, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	f.runCalls = append(f.runCalls, fakeRunCall{
		agent:       agent,
		contextPath: contextPath,
		profile:     profile,
		args:        append([]string(nil), args...),
	})
	return nil
}

func newV2Root(t *testing.T) (*Root, *bytes.Buffer, *bytes.Buffer, *fakeNativeRuntime, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	container, err := app.Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	root := New(container.ProfileService, BuildInfo{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }
	native := &fakeNativeRuntime{}
	root.native = native
	return root, stdout, stderr, native, home
}

func decodeJSON(t *testing.T, raw string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", raw, err)
	}
}

func TestRootHelpAndRemovedLegacyCommandsV3(t *testing.T) {
	root, stdout, stderr, _, _ := newV2Root(t)

	if code := root.Execute(nil); code != 0 {
		t.Fatalf("Execute(nil) code=%d stderr=%q", code, stderr.String())
	}
	help := stdout.String()
	if !strings.Contains(help, "AGX - Local Multi-Agent Runtime Manager") || !strings.Contains(help, "agx add <profile> --base-url URL") || !strings.Contains(help, "agx current") || !strings.Contains(help, "agx run <agent> [profile] [-- native args...]") || !strings.Contains(help, "agx restore <agent>") || !strings.Contains(help, "agx doctor") {
		t.Fatalf("help=%q missing profile-first command tree", help)
	}
	if strings.Contains(help, "agx codex [profile]") || strings.Contains(help, "agx claude [profile]") || strings.Contains(help, "agx gemini [profile]") || strings.Contains(help, "agx opencode [profile]") {
		t.Fatalf("help=%q should hide per-agent launcher aliases", help)
	}
	if strings.Contains(help, "migrate") || strings.Contains(help, "channel") || strings.Contains(help, "agx relay") {
		t.Fatalf("help=%q should keep v0.1 GA surface narrow", help)
	}
	if strings.Contains(help, "--api-key-env") {
		t.Fatalf("help=%q should not expose api-key-env on the public path", help)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"relay", "ls"}); code != 1 {
		t.Fatalf("removed relay code=%d stderr=%q", code, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "unknown command") {
		t.Fatalf("stderr=%q want unknown command", got)
	}
}


func TestAdapterShortcutCommandsV3(t *testing.T) {
	root, stdout, stderr, native, home := newV2Root(t)

	if code := root.Execute([]string{"add", "work", "--base-url", "https://relay.example/v1", "--api-key", "sk-a", "-o", "json"}); code != 0 {
		t.Fatalf("add code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var addPayload struct {
		Profile managedProfileView `json:"profile"`
	}
	decodeJSON(t, stdout.String(), &addPayload)
	if addPayload.Profile.Name != "work" || addPayload.Profile.Kind != "relay" || addPayload.Profile.CredentialRef != "api_key" {
		t.Fatalf("profile add payload=%+v", addPayload.Profile)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"use", "work", "-o", "json"}); code != 0 {
		t.Fatalf("use code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var usePayload struct {
		Profile managedProfileView `json:"profile"`
	}
	decodeJSON(t, stdout.String(), &usePayload)
	if usePayload.Profile.Name != "work" || !usePayload.Profile.Current {
		t.Fatalf("use payload=%+v", usePayload.Profile)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"current", "-o", "json"}); code != 0 {
		t.Fatalf("current code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var currentPayload struct {
		Profile *managedProfileView `json:"profile"`
	}
	decodeJSON(t, stdout.String(), &currentPayload)
	if currentPayload.Profile == nil || currentPayload.Profile.Name != "work" || !currentPayload.Profile.Current {
		t.Fatalf("current payload=%+v", currentPayload.Profile)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"show", "work", "-o", "json"}); code != 0 {
		t.Fatalf("show code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var showPayload struct {
		Profile managedProfileView `json:"profile"`
	}
	decodeJSON(t, stdout.String(), &showPayload)
	if showPayload.Profile.Name != "work" || showPayload.Profile.APIKey != "sk-a" || showPayload.Profile.CredentialRef != "api_key" {
		t.Fatalf("show payload=%+v", showPayload.Profile)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"show", "work"}); code != 0 {
		t.Fatalf("show text code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "api_key: sk-a") {
		t.Fatalf("show text stdout=%q want api_key", got)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"edit", "work", "--name", "focus", "-o", "json"}); code != 0 {
		t.Fatalf("edit rename code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var editPayload struct {
		Profile managedProfileView `json:"profile"`
	}
	decodeJSON(t, stdout.String(), &editPayload)
	if editPayload.Profile.Name != "focus" || !editPayload.Profile.Current {
		t.Fatalf("edit payload=%+v", editPayload.Profile)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"current", "-o", "json"}); code != 0 {
		t.Fatalf("current after rename code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	decodeJSON(t, stdout.String(), &currentPayload)
	if currentPayload.Profile == nil || currentPayload.Profile.Name != "focus" || !currentPayload.Profile.Current {
		t.Fatalf("current after rename payload=%+v", currentPayload.Profile)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"codex", "--", "status"}); code != 0 {
		t.Fatalf("launcher run code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	targetPath := filepath.Join(home, ".config", "agx", "contexts", "codex", "targets", "focus")
	if len(native.runCalls) != 1 || native.runCalls[0].contextPath != targetPath {
		t.Fatalf("run calls=%+v want context %q", native.runCalls, targetPath)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls", "-o", "json"}); code != 0 {
		t.Fatalf("ls code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var listPayload struct {
		Profiles []managedProfileView `json:"profiles"`
	}
	decodeJSON(t, stdout.String(), &listPayload)
	if len(listPayload.Profiles) != 1 || listPayload.Profiles[0].Name != "focus" {
		t.Fatalf("list payload=%+v", listPayload.Profiles)
	}
}

func TestCodexWireAPIFlagSurfacesInProfile(t *testing.T) {
	root, stdout, stderr, _, _ := newV2Root(t)

	if code := root.Execute([]string{"add", "newapi", "--base-url", "https://newapi.example/v1", "--api-key", "sk-n", "--codex-wire-api", "chat", "-o", "json"}); code != 0 {
		t.Fatalf("add code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var addPayload struct {
		Profile managedProfileView `json:"profile"`
	}
	decodeJSON(t, stdout.String(), &addPayload)
	if addPayload.Profile.CodexWireAPI != "chat" {
		t.Fatalf("add payload codex_wire_api=%q want chat", addPayload.Profile.CodexWireAPI)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"show", "newapi"}); code != 0 {
		t.Fatalf("show code=%d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "codex_wire_api: chat") {
		t.Fatalf("show stdout=%q want codex_wire_api line", got)
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"edit", "newapi", "--codex-wire-api", "responses", "-o", "json"}); code != 0 {
		t.Fatalf("edit code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var editPayload struct {
		Profile managedProfileView `json:"profile"`
	}
	decodeJSON(t, stdout.String(), &editPayload)
	if editPayload.Profile.CodexWireAPI != "responses" {
		t.Fatalf("edit payload codex_wire_api=%q want responses", editPayload.Profile.CodexWireAPI)
	}

	if code := root.Execute([]string{"add", "bad", "--base-url", "https://x/v1", "--api-key", "sk", "--codex-wire-api", "grpc"}); code == 0 {
		t.Fatalf("expected validation failure for invalid --codex-wire-api value, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestCodexWireAPIDefaultsToResponsesInProfile(t *testing.T) {
	root, stdout, stderr, _, _ := newV2Root(t)

	if code := root.Execute([]string{"add", "oai", "--base-url", "https://api.openai.com/v1", "--api-key", "sk-o", "-o", "json"}); code != 0 {
		t.Fatalf("add code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var payload struct {
		Profile managedProfileView `json:"profile"`
	}
	decodeJSON(t, stdout.String(), &payload)
	// Empty means "use adapter default" (responses); show + JSON should
	// surface the empty value so contract stays explicit per profile YAML.
	if payload.Profile.CodexWireAPI != "" {
		t.Fatalf("default add payload codex_wire_api=%q want empty", payload.Profile.CodexWireAPI)
	}
}








func TestNativeRunArgsInjectClaudeSettingsWhenManagedContextExists(t *testing.T) {
	contextPath := t.TempDir()
	settingsPath := filepath.Join(contextPath, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}

	got := nativeRunArgs(domainprofile.AgentClaude, contextPath, []string{"--bare", "-p", "say ok"})
	if strings.Join(got, " ") != "--settings "+settingsPath+" --bare -p say ok" {
		t.Fatalf("native args=%q", strings.Join(got, " "))
	}

	got = nativeRunArgs(domainprofile.AgentClaude, contextPath, []string{"--settings", "/custom/settings.json", "--bare"})
	if strings.Join(got, " ") != "--settings /custom/settings.json --bare" {
		t.Fatalf("native args with explicit settings=%q", strings.Join(got, " "))
	}
}
