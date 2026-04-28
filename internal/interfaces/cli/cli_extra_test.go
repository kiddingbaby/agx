package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty/v2"
	"github.com/kiddingbaby/agx/internal/adapters/profilefile"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/usecase"
)

type failingWriter struct{}
type statErrReader struct{}

type stubProfileRepo struct {
	listErr  error
	getErr   error
	profiles map[string]domainprofile.Profile
}

func (s *stubProfileRepo) List() ([]domainprofile.Profile, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]domainprofile.Profile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		out = append(out, profile)
	}
	return out, nil
}

func (s *stubProfileRepo) Get(name string) (*domainprofile.Profile, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	profile, ok := s.profiles[domainprofile.NormalizeProfileName(name)]
	if !ok {
		return nil, &domainprofile.NotFoundError{Name: name}
	}
	return &profile, nil
}

func (s *stubProfileRepo) Upsert(profile domainprofile.Profile) (*domainprofile.Profile, error) {
	if s.profiles == nil {
		s.profiles = map[string]domainprofile.Profile{}
	}
	s.profiles[profile.Name] = profile
	return &profile, nil
}

func (s *stubProfileRepo) Delete(name string) error {
	delete(s.profiles, domainprofile.NormalizeProfileName(name))
	return nil
}

type stubStateRepo struct {
	loadErr error
	state   domainprofile.State
}

func (s *stubStateRepo) Load() (domainprofile.State, error) {
	if s.loadErr != nil {
		return domainprofile.State{}, s.loadErr
	}
	return s.state, nil
}

func (s *stubStateRepo) Save(state domainprofile.State) (domainprofile.State, error) {
	s.state = state
	return state, nil
}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("boom")
}

func (statErrReader) Read([]byte) (int, error) {
	return 0, io.EOF
}

func TestExecuteHelpAndUnknownCommand(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	if code := root.Execute(nil); code != 0 {
		t.Fatalf("Execute(nil) code=%d want 0", code)
	}
	if !strings.Contains(stdout.String(), "AGX - Relay Manager") {
		t.Fatalf("stdout=%q want help text", stdout.String())
	}
	if !strings.Contains(stdout.String(), "agx doctor [-o json]") {
		t.Fatalf("stdout=%q want doctor JSON help", stdout.String())
	}
	if !strings.Contains(stdout.String(), "agx rm <relay> [-o json]") {
		t.Fatalf("stdout=%q want remove JSON help", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"unknown"}); code != 1 {
		t.Fatalf("Execute(unknown) code=%d want 1", code)
	}
	if !strings.Contains(stderr.String(), "Supported commands:") {
		t.Fatalf("stderr=%q want supported commands", stderr.String())
	}
}

func TestHandleRemoveAndInternalAPIKey(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{internalAPIKeyCommand, "relay-a"}); code != 0 {
		t.Fatalf("__api-key code=%d stderr=%q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "sk-a" {
		t.Fatalf("stdout=%q want sk-a", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{internalAPIKeyCommand}); code == 0 {
		t.Fatalf("__api-key without relay unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "requires exactly one relay") {
		t.Fatalf("stderr=%q want helper usage failure", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"rm", "relay-a"}); code != 0 {
		t.Fatalf("rm code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Removed relay: relay-a") {
		t.Fatalf("stdout=%q want remove output", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"rm", "relay-a", "-o", "json"}); code == 0 {
		t.Fatalf("rm missing relay unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestParseHelpersAndRenderHelpers(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)

	agent, asJSON, ok := parseAgentOnly(root, []string{"codex", "-o", "json"}, "usage")
	if !ok || agent != domainprofile.AgentCodex || !asJSON {
		t.Fatalf("parseAgentOnly() = (%q,%v,%v)", agent, asJSON, ok)
	}
	if renderCurrentRelay("") != "-" || renderCurrentRelay("relay-a") != "relay-a" {
		t.Fatalf("renderCurrentRelay() mismatch")
	}

	state := domainprofile.State{
		CodexProfiles: map[string]domainprofile.CodexProfileBinding{
			"relay-a": {Status: domainprofile.BindingStatusApplied},
		},
	}
	if binding := codexProfileBinding(state, "Relay-A"); binding.Status != domainprofile.BindingStatusApplied {
		t.Fatalf("codexProfileBinding() = %+v, want applied status", binding)
	}
	if got, ok := parseJSONOnlyArgs(root, []string{"-o", "json"}, "usage"); !ok || !got {
		t.Fatalf("parseJSONOnlyArgs() = (%v,%v), want true,true", got, ok)
	}
	if _, _, _, ok := parseRestoreArgs(root, []string{"--agent", "codex", "--to", "backup-1", "-o", "json"}, "usage"); !ok {
		t.Fatal("parseRestoreArgs() unexpectedly failed")
	}

	stderr.Reset()
	if _, _, ok := parseAgentOnly(root, []string{"bad-agent"}, "usage"); ok {
		t.Fatal("parseAgentOnly() unexpectedly succeeded")
	}
	if !strings.Contains(stderr.String(), "Agent must be one of") {
		t.Fatalf("stderr=%q want invalid agent message", stderr.String())
	}

	stderr.Reset()
	if _, _, _, ok := parseOptionalAgentFlag(root, []string{"--agent", "codex", "--agent", "claude"}, "usage"); ok {
		t.Fatal("parseOptionalAgentFlag() unexpectedly succeeded with duplicate flag")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage message", stderr.String())
	}

	stderr.Reset()
	if _, ok := parseJSONOnlyArgs(root, []string{"-o", "text"}, "usage"); ok {
		t.Fatal("parseJSONOnlyArgs() unexpectedly succeeded with invalid value")
	}
	if !strings.Contains(stderr.String(), "-o requires value json") {
		t.Fatalf("stderr=%q want invalid -o message", stderr.String())
	}

	stderr.Reset()
	if _, _, ok := parseNameWithJSON(root, []string{"relay-a", "extra"}, "usage"); ok {
		t.Fatal("parseNameWithJSON() unexpectedly succeeded with duplicate positional args")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage message", stderr.String())
	}

	stderr.Reset()
	if _, _, _, ok := parseRestoreArgs(root, []string{"--to", "backup-1"}, "usage"); ok {
		t.Fatal("parseRestoreArgs() unexpectedly succeeded without agent")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for missing agent", stderr.String())
	}
}

func TestInteractiveHelpersAndJSONWriter(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	session := newPromptSession(strings.NewReader("1\nhttps://relay-new.example/v1\n2\nsk-new\n\n"))
	current := domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-old"}
	parsed, err := root.promptForEdit(session, profileMutationArgs{name: "relay-a"}, current)
	if err != nil {
		t.Fatalf("promptForEdit() error = %v", err)
	}
	if parsed.baseURL == nil || *parsed.baseURL != "https://relay-new.example/v1" || parsed.apiKey == nil || *parsed.apiKey != "sk-new" {
		t.Fatalf("parsed = %+v want updated values", parsed)
	}

	apiKey, err := root.promptAPIKey(newPromptSession(strings.NewReader("sk-buffered\n")), "API key: ", "")
	if err != nil || apiKey != "sk-buffered" {
		t.Fatalf("promptAPIKey() = (%q,%v), want sk-buffered nil", apiKey, err)
	}

	root.stdout = failingWriter{}
	if code := root.writeJSON(map[string]string{"ok": "1"}); code != 1 {
		t.Fatalf("writeJSON() code=%d want 1", code)
	}
	if !strings.Contains(stderr.String(), "failed to encode JSON output") {
		t.Fatalf("stderr=%q want JSON failure", stderr.String())
	}

	root.stdout = stdout

	value, err := root.promptValidatedText(newPromptSession(strings.NewReader("bad\nrelay-ok\n")), "Relay name: ", "", domainprofile.ValidateProfileName)
	if err != nil || value != "bad" {
		t.Fatalf("promptValidatedText() = (%q,%v), want first valid input", value, err)
	}

	_, err = root.promptValidatedText(newPromptSession(strings.NewReader("")), "Relay name: ", "", domainprofile.ValidateProfileName)
	if !errors.Is(err, errInteractiveCanceled) {
		t.Fatalf("promptValidatedText(cancel) err=%v, want errInteractiveCanceled", err)
	}
}

func TestInteractiveAdditionalBranchCoverage(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	session := newPromptSession(strings.NewReader(""))
	if _, err := root.promptForAdd(session, profileMutationArgs{}); !errors.Is(err, errInteractiveCanceled) {
		t.Fatalf("promptForAdd(cancel) err=%v, want errInteractiveCanceled", err)
	}

	session = newPromptSession(strings.NewReader(""))
	if _, err := root.promptForEdit(session, profileMutationArgs{}, domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); !errors.Is(err, errInteractiveCanceled) {
		t.Fatalf("promptForEdit(cancel) err=%v, want errInteractiveCanceled", err)
	}

	session = newPromptSession(strings.NewReader("3\n2\nsk-new\n\n"))
	parsed, err := root.promptForEdit(session, profileMutationArgs{}, domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"})
	if err != nil {
		t.Fatalf("promptForEdit(retry) error = %v", err)
	}
	if parsed.apiKey == nil || *parsed.apiKey != "sk-new" {
		t.Fatalf("promptForEdit(retry) parsed=%+v, want api key updated", parsed)
	}
	if !strings.Contains(stdout.String(), "Invalid value: use 1/2 or url/key") {
		t.Fatalf("stdout=%q want invalid choice message", stdout.String())
	}

	if isTerminalReader(statErrReader{}) {
		t.Fatal("isTerminalReader(non-file reader) = true, want false")
	}
	if root.canPrompt(true) {
		t.Fatal("canPrompt(json) = true, want false")
	}

	rootNilTTY := New(nil, BuildInfo{})
	rootNilTTY.isTTY = nil
	if rootNilTTY.canPrompt(false) {
		t.Fatal("canPrompt(nil isTTY) = true, want false")
	}

	tempFile, err := os.CreateTemp(t.TempDir(), "api-key-*")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer tempFile.Close()
	cancelSession := &promptSession{lines: newPromptSession(strings.NewReader("")).lines, passwordFile: tempFile}
	if _, err := root.promptAPIKey(cancelSession, "API key: ", ""); !errors.Is(err, errInteractiveCanceled) {
		t.Fatalf("promptAPIKey(password error) err=%v, want errInteractiveCanceled", err)
	}

	root.stdout = failingWriter{}
	if code := root.writeRestoreResult(&usecase.RestoreResult{
		Agent:      domainprofile.AgentCodex,
		ConfigPath: "/tmp/codex/config.toml",
		Backup:     domainprofile.Backup{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRestoreFile},
	}, true, false); code != 1 {
		t.Fatalf("writeRestoreResult(json encode failure) code=%d, want 1", code)
	}
	root.stdout = stdout
	stderr.Reset()
}

func TestPromptAPIKeyWithTTY(t *testing.T) {
	root, _, _, _ := newProfileRoot(t)
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() error = %v", err)
	}
	defer ptyMaster.Close()
	defer ptySlave.Close()

	done := make(chan struct {
		value string
		err   error
	}, 1)
	go func() {
		value, err := root.promptAPIKey(&promptSession{
			lines:        &fileLineReader{file: ptySlave},
			passwordFile: ptySlave,
		}, "API key: ", "")
		done <- struct {
			value string
			err   error
		}{value: value, err: err}
	}()

	if _, err := ptyMaster.Write([]byte("sk-tty-test\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	result := <-done
	if result.err != nil || result.value != "sk-tty-test" {
		t.Fatalf("promptAPIKey(TTY) = (%q,%v), want sk-tty-test nil", result.value, result.err)
	}
}

func TestTerminalReaders(t *testing.T) {
	nullFile, err := os.Open("/dev/null")
	if err != nil {
		t.Fatalf("Open(/dev/null) error = %v", err)
	}
	defer nullFile.Close()
	if !isTerminalReader(nullFile) {
		t.Fatal("isTerminalReader(/dev/null) = false, want true for char device")
	}

	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("line-1\nline-2\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()
	if isTerminalReader(file) {
		t.Fatal("isTerminalReader(regular file) = true, want false")
	}

	reader := &fileLineReader{file: file}
	line, err := reader.ReadLine()
	if err != nil || line != "line-1\n" {
		t.Fatalf("ReadLine() = (%q,%v), want first line", line, err)
	}
}

func TestUserErrorMessages(t *testing.T) {
	root, _, _, _ := newProfileRoot(t)
	cases := []error{
		&domainprofile.NotFoundError{Name: "relay-a"},
		&usecase.ProfileAlreadyExistsError{Name: "relay-a"},
		&usecase.ProfileInUseError{Name: "relay-a", Agents: []domainprofile.Agent{domainprofile.AgentCodex}},
		&usecase.ConflictingAgentChangesError{Agents: []domainprofile.Agent{domainprofile.AgentCodex}},
		&usecase.AgentNotBoundToRelayError{Agent: domainprofile.AgentClaude, Relay: "relay-a"},
		&usecase.BackupNotFoundError{ID: "backup-1"},
		&usecase.InvalidAgentError{Agent: "bad"},
		&usecase.NoBackupError{Agent: domainprofile.AgentCodex},
	}
	for _, err := range cases {
		if strings.TrimSpace(root.userErrorMessage(err)) == "" {
			t.Fatalf("userErrorMessage(%T) returned empty string", err)
		}
	}
	if root.userErrorMessage(errInteractiveCanceled) != "Canceled." {
		t.Fatalf("interactive cancel message mismatch")
	}
}

func TestRelayRenderHelpers(t *testing.T) {
	now := time.Now().UTC()
	state := domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-a",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/codex/config.toml",
				LastAppliedAt: now,
				LastBackupID:  "backup-codex",
			},
		},
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Status:        domainprofile.BindingStatusApplied,
		},
	}

	agents := relayAgents("relay-a", state)
	if len(agents) != 2 || renderAgents(agents) != "codex,claude" {
		t.Fatalf("relayAgents/renderAgents mismatch: %v", agents)
	}
	bindings := relayBindings("relay-a", state)
	if len(bindings) != 2 {
		t.Fatalf("relayBindings() = %+v, want 2 bindings", bindings)
	}
}

func TestListBackupRestoreAndDoctorOutputEdges(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)
	if code := root.Execute([]string{"ls", "--agent", "codex"}); code != 0 {
		t.Fatalf("ls --agent code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "(no relays)") {
		t.Fatalf("stdout=%q want no relays text", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "ls", "--agent", "codex"}); code != 0 {
		t.Fatalf("backup ls code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "(no backups for codex)") {
		t.Fatalf("stdout=%q want no backups text", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"restore", "--agent", "codex"}); code == 0 {
		t.Fatalf("restore without backup unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "No Codex backup available") {
		t.Fatalf("stderr=%q want no-backup message", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"doctor", "-o", "json"}); code != 0 {
		t.Fatalf("doctor json code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"ok\":true") {
		t.Fatalf("stdout=%q want doctor JSON", stdout.String())
	}
}

func TestMutationCommandHelpAndJSONBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	if code := root.Execute([]string{"add", "--help"}); code != 0 || !strings.Contains(stdout.String(), "Usage: agx add") {
		t.Fatalf("add --help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a", "-o", "json"}); code != 0 {
		t.Fatalf("add json code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"relay\"") {
		t.Fatalf("stdout=%q want add JSON", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "relay-a", "--api-key", "sk-b", "-o", "json"}); code != 0 {
		t.Fatalf("edit json code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"api_key\":\"sk-b\"") {
		t.Fatalf("stdout=%q want edit JSON", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"rm", "relay-a", "-o", "json"}); code != 0 {
		t.Fatalf("rm json code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"relay\"") {
		t.Fatalf("stdout=%q want remove JSON", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "--help"}); code != 0 || !strings.Contains(stdout.String(), "Usage: agx edit") {
		t.Fatalf("edit --help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestServiceUnavailableBranches(t *testing.T) {
	root := New(nil, BuildInfo{})
	root.stdout = &bytes.Buffer{}
	root.stderr = &bytes.Buffer{}
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }

	commands := [][]string{
		{"add", "relay-a"},
		{"edit", "relay-a"},
		{"ls"},
		{"show", "relay-a"},
		{"restore", "--agent", "codex"},
		{"backup", "ls", "--agent", "codex"},
		{"doctor"},
		{"rm", "relay-a"},
		{internalAPIKeyCommand, "relay-a"},
	}
	for _, args := range commands {
		root.stdout.(*bytes.Buffer).Reset()
		root.stderr.(*bytes.Buffer).Reset()
		if code := root.Execute(args); code == 0 {
			t.Fatalf("Execute(%v) unexpectedly succeeded", args)
		}
		if !strings.Contains(root.stderr.(*bytes.Buffer).String(), "service is unavailable") {
			t.Fatalf("stderr=%q want unavailable error for %v", root.stderr.(*bytes.Buffer).String(), args)
		}
	}
}

func TestListShowAndBackupJSONBranches(t *testing.T) {
	root, stdout, stderr, home := newProfileRoot(t)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("profile = \"before\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d stderr=%q", code, stderr.String())
	}
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex"}); code != 0 {
		t.Fatalf("edit bind code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls", "--agent", "codex", "-o", "json"}); code != 0 {
		t.Fatalf("ls json code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"current_relay\":\"relay-a\"") {
		t.Fatalf("stdout=%q want current relay in JSON", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"show", "relay-a", "-o", "json"}); code != 0 {
		t.Fatalf("show json code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"agent_bindings\"") {
		t.Fatalf("stdout=%q want agent bindings JSON", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "ls", "--agent", "codex", "-o", "json"}); code != 0 {
		t.Fatalf("backup json code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"backups\"") {
		t.Fatalf("stdout=%q want backups JSON", stdout.String())
	}
}

func TestParseAndInteractiveAdditionalBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	if _, _, ok := parseNameWithJSON(root, []string{"-o", "json"}, "usage"); ok {
		t.Fatal("parseNameWithJSON() unexpectedly succeeded without name")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for missing name", stderr.String())
	}

	stderr.Reset()
	if _, _, ok := parseAgentOnlyFlagRequired(root, []string{"-o", "json"}, "usage"); ok {
		t.Fatal("parseAgentOnlyFlagRequired() unexpectedly succeeded without --agent")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for missing --agent", stderr.String())
	}

	stderr.Reset()
	if _, _, _, ok := parseOptionalAgentFlag(root, []string{"--agent", "bad"}, "usage"); ok {
		t.Fatal("parseOptionalAgentFlag() unexpectedly succeeded with invalid agent")
	}
	if !strings.Contains(stderr.String(), "Agent must be one of") {
		t.Fatalf("stderr=%q want invalid agent message", stderr.String())
	}

	stderr.Reset()
	if _, _, _, ok := parseRestoreArgs(root, []string{"--agent", "codex", "extra"}, "usage"); ok {
		t.Fatal("parseRestoreArgs() unexpectedly succeeded with positional arg")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for invalid restore args", stderr.String())
	}

	stderr.Reset()
	if agents, ok := parseAgentList(root, ""); ok || agents != nil {
		t.Fatalf("parseAgentList(empty) = (%v,%v), want nil,false", agents, ok)
	}
	if !strings.Contains(stderr.String(), "agent list cannot be empty") {
		t.Fatalf("stderr=%q want empty agent list message", stderr.String())
	}

	stderr.Reset()
	if agents, ok := parseAgentList(root, "codex,"); ok || agents != nil {
		t.Fatalf("parseAgentList(trailing empty) = (%v,%v), want nil,false", agents, ok)
	}
	if !strings.Contains(stderr.String(), "cannot contain empty values") {
		t.Fatalf("stderr=%q want empty element message", stderr.String())
	}

	session := newPromptSession(strings.NewReader("line\n"))
	if session.passwordFile != nil {
		t.Fatalf("newPromptSession(strings.Reader) passwordFile = %v, want nil", session.passwordFile)
	}
	if _, ok := session.lines.(*bufferedLineReader); !ok {
		t.Fatalf("newPromptSession(strings.Reader) lines = %T, want *bufferedLineReader", session.lines)
	}

	stdout.Reset()
	choice, done, err := root.promptEditField(newPromptSession(strings.NewReader("bad\n1\n")), false)
	if err != nil || done || choice != "base_url" {
		t.Fatalf("promptEditField() = (%q,%v,%v), want base_url,false,nil", choice, done, err)
	}
	if !strings.Contains(stdout.String(), "Invalid value") {
		t.Fatalf("stdout=%q want invalid prompt feedback", stdout.String())
	}

	stdout.Reset()
	choice, done, err = root.promptEditField(newPromptSession(strings.NewReader("\n")), false)
	if err != nil || !done || choice != "" {
		t.Fatalf("promptEditField(done) = (%q,%v,%v), want empty,true,nil", choice, done, err)
	}
	if !strings.Contains(stdout.String(), "No changes made.") {
		t.Fatalf("stdout=%q want no changes message", stdout.String())
	}

	stdout.Reset()
	value, err := root.promptValidatedText(newPromptSession(strings.NewReader("\n")), "Relay name: ", "relay-a", domainprofile.ValidateProfileName)
	if err != nil || value != "relay-a" {
		t.Fatalf("promptValidatedText(current) = (%q,%v), want relay-a,nil", value, err)
	}

	stdout.Reset()
	value, err = root.promptValidatedText(newPromptSession(strings.NewReader("not-a-url\nhttps://relay.example/v1\n")), "Base URL: ", "", domainprofile.ValidateBaseURL)
	if err != nil || value != "https://relay.example/v1" {
		t.Fatalf("promptValidatedText(retry) = (%q,%v), want retried URL,nil", value, err)
	}
	if !strings.Contains(stdout.String(), "Invalid value") {
		t.Fatalf("stdout=%q want validation failure message", stdout.String())
	}

	value, err = root.promptAPIKey(newPromptSession(strings.NewReader("\n")), "API key: ", "sk-current")
	if err != nil || value != "sk-current" {
		t.Fatalf("promptAPIKey(current) = (%q,%v), want sk-current,nil", value, err)
	}
}

func TestRootAndTextOutputAdditionalBranches(t *testing.T) {
	root := New(nil, BuildInfo{})
	if root.build.Version != "dev" || root.build.Commit != "unknown" || root.build.Date != "unknown" {
		t.Fatalf("New(nil, empty build).build = %+v, want normalized defaults", root.build)
	}

	root.stdout = &bytes.Buffer{}
	root.stderr = &bytes.Buffer{}
	root.printUserError(nil)
	if root.stderr.(*bytes.Buffer).Len() != 0 {
		t.Fatalf("stderr=%q want empty after printUserError(nil)", root.stderr.(*bytes.Buffer).String())
	}

	root.printUserError(errors.New("boom"))
	if !strings.Contains(root.stderr.(*bytes.Buffer).String(), "boom") {
		t.Fatalf("stderr=%q want generic error text", root.stderr.(*bytes.Buffer).String())
	}
}

func TestListAndDoctorTextBranches(t *testing.T) {
	root, stdout, stderr, home := newProfileRoot(t)

	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("profile = \"before\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d stderr=%q", code, stderr.String())
	}
	if code := root.Execute([]string{"edit", "relay-a", "--bind", "codex"}); code != 0 {
		t.Fatalf("edit bind code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls", "--agent", "codex"}); code != 0 {
		t.Fatalf("ls --agent code=%d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Agent: codex") || !strings.Contains(got, "Current: relay-a") || !strings.Contains(got, "* relay-a") {
		t.Fatalf("stdout=%q want agent-scoped list output", got)
	}

	stateRepo := profilefile.NewStateRepository(filepath.Join(home, ".config", "agx", "state.yaml"))
	state, err := stateRepo.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	state.Claude = domainprofile.AgentBinding{
		SourceProfile: "relay-a",
		Status:        domainprofile.BindingStatus("broken"),
	}
	if _, err := stateRepo.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"doctor"}); code != 1 {
		t.Fatalf("doctor code=%d stdout=%q stderr=%q want 1", code, stdout.String(), stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Doctor: issues found") || !strings.Contains(got, "[error] invalid_binding_status") {
		t.Fatalf("stdout=%q want doctor issue output", got)
	}
}

func TestInteractiveAddAndEditAdditionalBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	setInteractiveInput(root, "relay-a\nnot-a-url\nhttps://relay.example/v1\nsk-a\n")
	if code := root.Execute([]string{"add"}); code != 0 {
		t.Fatalf("interactive add code=%d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Invalid value:") || !strings.Contains(got, "Added relay: relay-a") {
		t.Fatalf("stdout=%q want validation retry during add", got)
	}

	stdout.Reset()
	stderr.Reset()
	setInteractiveInput(root, "1\nhttps://relay-edited.example/v1\n2\nsk-b\n\n")
	if code := root.Execute([]string{"edit", "relay-a"}); code != 0 {
		t.Fatalf("interactive edit code=%d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Edit more") || !strings.Contains(got, "Edited relay: relay-a") || !strings.Contains(got, "api_key=sk-b") {
		t.Fatalf("stdout=%q want multi-step interactive edit flow", got)
	}
}

func TestMutationAndReadUsageBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	if code := root.Execute([]string{"show", "--help"}); code != 0 || !strings.Contains(stdout.String(), "Usage: agx show") {
		t.Fatalf("show --help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "--help"}); code != 0 || !strings.Contains(stdout.String(), "Usage: agx backup ls") {
		t.Fatalf("backup --help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "rm"}); code == 0 {
		t.Fatalf("backup invalid subcommand unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: agx backup ls") {
		t.Fatalf("stderr=%q want backup usage", stderr.String())
	}

	stderr.Reset()
	if _, _, ok := parseAgentOnly(root, []string{"codex", "--bad"}, "usage"); ok {
		t.Fatal("parseAgentOnly() unexpectedly succeeded with unknown flag")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for parseAgentOnly unknown flag", stderr.String())
	}

	stderr.Reset()
	if _, _, _, ok := parseOptionalAgentFlag(root, []string{"--agent"}, "usage"); ok {
		t.Fatal("parseOptionalAgentFlag() unexpectedly succeeded with missing value")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for missing --agent value", stderr.String())
	}
}

func TestRenderHelpersAdditionalBranches(t *testing.T) {
	if got := renderAgents(nil); got != "-" {
		t.Fatalf("renderAgents(nil) = %q, want -", got)
	}
	if got := bindingForAgent(domainprofile.State{}, domainprofile.Agent("bad")); got.SourceProfile != "" || got.Status != "" || got.ConfigPath != "" || len(got.Backups) != 0 {
		t.Fatalf("bindingForAgent(invalid) = %+v, want zero binding", got)
	}
	if got := codexProfileBinding(domainprofile.State{}, "relay-a"); got != (domainprofile.CodexProfileBinding{}) {
		t.Fatalf("codexProfileBinding(empty state) = %+v, want zero binding", got)
	}
}

func TestPromptSessionAndMutationInteractiveEdgeBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() error = %v", err)
	}
	defer ptyMaster.Close()
	defer ptySlave.Close()

	session := newPromptSession(ptySlave)
	if session.passwordFile == nil {
		t.Fatal("newPromptSession(pty) passwordFile = nil, want tty file")
	}
	if _, ok := session.lines.(*fileLineReader); !ok {
		t.Fatalf("newPromptSession(pty) lines = %T, want *fileLineReader", session.lines)
	}

	stdout.Reset()
	stderr.Reset()
	setInteractiveInput(root, "")
	if code := root.Execute([]string{"add"}); code == 0 {
		t.Fatalf("interactive add cancel unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Canceled.") {
		t.Fatalf("stderr=%q want canceled message", stderr.String())
	}

	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("seed add code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	setInteractiveInput(root, "relay-a\n\n")
	if code := root.Execute([]string{"edit"}); code != 0 {
		t.Fatalf("interactive edit without name code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "No changes made.") {
		t.Fatalf("stdout=%q want no changes path", stdout.String())
	}
}

func TestMutationCommandErrorBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	setInteractiveInput(root, "relay-a\nhttps://relay.example/v1\nsk-a\n")
	if code := root.Execute([]string{"add"}); code != 0 {
		t.Fatalf("seed interactive add code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code == 0 {
		t.Fatalf("duplicate add unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr=%q want duplicate add error", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "missing", "--api-key", "sk-b"}); code == 0 {
		t.Fatalf("edit missing unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("stderr=%q want missing relay error", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"rm", "missing"}); code == 0 {
		t.Fatalf("rm missing unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("stderr=%q want missing relay remove error", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{internalAPIKeyCommand, "missing"}); code == 0 {
		t.Fatalf("__api-key missing unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("stderr=%q want missing relay api key error", stderr.String())
	}
}

func TestInteractiveMutationHelpAndJSONEdgeBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	if code := root.Execute([]string{"rm", "--help"}); code != 0 || !strings.Contains(stdout.String(), "Usage: agx rm") {
		t.Fatalf("rm --help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"add", "-o", "json"}); code == 0 {
		t.Fatalf("add json without required fields unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: agx add") {
		t.Fatalf("stderr=%q want add usage in json mode", stderr.String())
	}

	setInteractiveInput(root, "relay-a\nhttps://relay.example/v1\nsk-a\n")
	if code := root.Execute([]string{"add"}); code != 0 {
		t.Fatalf("seed add code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"edit", "-o", "json"}); code == 0 {
		t.Fatalf("edit json without name unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: agx edit") {
		t.Fatalf("stderr=%q want edit usage in json mode", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"rm", "-o", "json"}); code == 0 {
		t.Fatalf("rm json without name unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: agx rm") {
		t.Fatalf("stderr=%q want rm usage in json mode", stderr.String())
	}
}

func TestInteractiveReadersAndPromptAPIKeyAdditionalBranches(t *testing.T) {
	root, stdout, _, _ := newProfileRoot(t)

	path := filepath.Join(t.TempDir(), "single-line.txt")
	if err := os.WriteFile(path, []byte("tail-without-newline"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	reader := &fileLineReader{file: file}
	line, err := reader.ReadLine()
	if err != io.EOF && err != nil {
		t.Fatalf("ReadLine() unexpected err=%v", err)
	}
	if line != "tail-without-newline" {
		t.Fatalf("ReadLine() line=%q want tail-without-newline", line)
	}

	stdout.Reset()
	value, err := root.promptAPIKey(newPromptSession(strings.NewReader(" \nsk-value\n")), "API key: ", "")
	if err != nil || value != "sk-value" {
		t.Fatalf("promptAPIKey(retry buffered) = (%q,%v), want sk-value,nil", value, err)
	}
	if !strings.Contains(stdout.String(), "Invalid value:") {
		t.Fatalf("stdout=%q want invalid api key feedback", stdout.String())
	}
}

func TestParseProfileMutationArgsAdditionalBranches(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)

	parsed, ok := parseProfileMutationArgs(root, []string{"relay-a", "--bind", "codex", "--bind", "claude,codex", "--unbind", "gemini", "-o", "json"}, "usage")
	if !ok {
		t.Fatalf("parseProfileMutationArgs() unexpectedly failed stderr=%q", stderr.String())
	}
	if parsed.name != "relay-a" || !parsed.asJSON || len(parsed.bind) != 2 || parsed.bind[0] != domainprofile.AgentClaude || parsed.bind[1] != domainprofile.AgentCodex || len(parsed.unbind) != 1 || parsed.unbind[0] != domainprofile.AgentGemini {
		t.Fatalf("parsed = %+v, want normalized name/bind/unbind/json", parsed)
	}

	cases := [][]string{
		{"relay-a", "--base-url"},
		{"relay-a", "--api-key"},
		{"relay-a", "--bind"},
		{"relay-a", "--unbind"},
		{"relay-a", "-o", "text"},
		{"relay-a", "--unknown"},
	}
	for _, args := range cases {
		stderr.Reset()
		if _, ok := parseProfileMutationArgs(root, args, "usage"); ok {
			t.Fatalf("parseProfileMutationArgs(%v) unexpectedly succeeded", args)
		}
		if stderr.Len() == 0 {
			t.Fatalf("parseProfileMutationArgs(%v) did not write stderr", args)
		}
	}
}

func TestParseHelpersAdditionalErrorBranches(t *testing.T) {
	root, _, stderr, _ := newProfileRoot(t)

	if _, ok := parseJSONOnlyArgs(root, []string{"unexpected"}, "usage"); ok {
		t.Fatal("parseJSONOnlyArgs() unexpectedly succeeded with positional arg")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for parseJSONOnlyArgs positional arg", stderr.String())
	}

	stderr.Reset()
	if _, _, ok := parseNameWithJSON(root, []string{"-o"}, "usage"); ok {
		t.Fatal("parseNameWithJSON() unexpectedly succeeded with missing -o value")
	}
	if !strings.Contains(stderr.String(), "-o requires value json") {
		t.Fatalf("stderr=%q want invalid -o message", stderr.String())
	}

	stderr.Reset()
	if _, _, ok := parseAgentOnly(root, []string{"codex", "claude"}, "usage"); ok {
		t.Fatal("parseAgentOnly() unexpectedly succeeded with multiple positional args")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for parseAgentOnly duplicate args", stderr.String())
	}

	stderr.Reset()
	if _, _, _, ok := parseOptionalAgentFlag(root, []string{"-o"}, "usage"); ok {
		t.Fatal("parseOptionalAgentFlag() unexpectedly succeeded with missing -o value")
	}
	if !strings.Contains(stderr.String(), "-o requires value json") {
		t.Fatalf("stderr=%q want invalid -o for parseOptionalAgentFlag", stderr.String())
	}

	stderr.Reset()
	if _, _, _, ok := parseRestoreArgs(root, []string{"-o"}, "usage"); ok {
		t.Fatal("parseRestoreArgs() unexpectedly succeeded with missing -o value")
	}
	if !strings.Contains(stderr.String(), "-o requires value json") {
		t.Fatalf("stderr=%q want invalid -o for parseRestoreArgs", stderr.String())
	}

	stderr.Reset()
	if _, _, _, ok := parseRestoreArgs(root, []string{"--agent", "codex", "--to", "   "}, "usage"); ok {
		t.Fatal("parseRestoreArgs() unexpectedly succeeded with blank backup id")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr=%q want usage for blank backup id", stderr.String())
	}

	stderr.Reset()
	if agents, ok := parseAgentList(root, "bad-agent"); ok || agents != nil {
		t.Fatalf("parseAgentList(invalid) = (%v,%v), want nil,false", agents, ok)
	}
	if !strings.Contains(stderr.String(), "Agent must be one of") {
		t.Fatalf("stderr=%q want invalid agent error", stderr.String())
	}

	if got := normalizeParsedAgents([]domainprofile.Agent{domainprofile.Agent("bad")}); len(got) != 0 {
		t.Fatalf("normalizeParsedAgents(invalid only) = %v, want empty slice", got)
	}
}

func TestReadHandlersHelpAndErrorBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	if code := root.Execute([]string{"ls", "--help"}); code != 0 || !strings.Contains(stdout.String(), "Usage: agx ls") {
		t.Fatalf("ls --help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"doctor", "--help"}); code != 0 || !strings.Contains(stdout.String(), "Usage: agx doctor") {
		t.Fatalf("doctor --help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"show"}); code == 0 {
		t.Fatalf("show without args unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: agx show") {
		t.Fatalf("stderr=%q want show usage", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"doctor", "extra"}); code == 0 {
		t.Fatalf("doctor with extra arg unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: agx doctor") {
		t.Fatalf("stderr=%q want doctor usage", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "ls", "-o", "json"}); code == 0 {
		t.Fatalf("backup ls without agent unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: agx backup ls") {
		t.Fatalf("stderr=%q want backup usage", stderr.String())
	}
}

func TestReadHandlersRuntimeErrorBranches(t *testing.T) {
	root := New(usecase.NewProfileService(&stubProfileRepo{listErr: errors.New("list failed")}, &stubStateRepo{}, nil, nil, nil), BuildInfo{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }

	if code := root.Execute([]string{"ls"}); code == 0 {
		t.Fatalf("ls unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	profiles := &stubProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	root = New(usecase.NewProfileService(profiles, &stubStateRepo{loadErr: errors.New("state failed")}, nil, nil, nil), BuildInfo{})
	root.stdout = stdout
	root.stderr = stderr
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"show", "relay-a"}); code == 0 {
		t.Fatalf("show unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	root = New(usecase.NewProfileService(profiles, &stubStateRepo{}, nil, nil, nil), BuildInfo{})
	root.stdout = stdout
	root.stderr = stderr
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "ls", "--agent", "bad"}); code == 0 {
		t.Fatalf("backup invalid agent unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestInteractiveZeroCoverageBranches(t *testing.T) {
	root, stdout, _, _ := newProfileRoot(t)

	path := filepath.Join(t.TempDir(), "closed.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	file.Close()

	reader := &fileLineReader{file: file}
	line, err := reader.ReadLine()
	if err == nil || line != "" {
		t.Fatalf("ReadLine(closed file) = (%q,%v), want empty line and read error", line, err)
	}
	if isTerminalReader(file) {
		t.Fatal("isTerminalReader(closed file) = true, want false")
	}

	parsed, err := root.promptForAdd(newPromptSession(strings.NewReader("relay-a\n")), profileMutationArgs{})
	if !errors.Is(err, errInteractiveCanceled) || parsed.name != "" || parsed.baseURL != nil || parsed.apiKey != nil || len(parsed.bind) != 0 || len(parsed.unbind) != 0 || parsed.mutationFlags != 0 || parsed.asJSON {
		t.Fatalf("promptForAdd(base url cancel) = (%+v,%v), want zero args + cancel", parsed, err)
	}

	parsed, err = root.promptForAdd(newPromptSession(strings.NewReader("relay-a\nhttps://relay.example/v1\n")), profileMutationArgs{})
	if !errors.Is(err, errInteractiveCanceled) || parsed.name != "" || parsed.baseURL != nil || parsed.apiKey != nil || len(parsed.bind) != 0 || len(parsed.unbind) != 0 || parsed.mutationFlags != 0 || parsed.asJSON {
		t.Fatalf("promptForAdd(api key cancel) = (%+v,%v), want zero args + cancel", parsed, err)
	}

	parsed, err = root.promptForEdit(newPromptSession(strings.NewReader("1\n")), profileMutationArgs{}, domainprofile.Profile{
		Name:    "relay-a",
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
	})
	if !errors.Is(err, errInteractiveCanceled) || parsed.name != "" || parsed.baseURL != nil || parsed.apiKey != nil || len(parsed.bind) != 0 || len(parsed.unbind) != 0 || parsed.mutationFlags != 0 || parsed.asJSON {
		t.Fatalf("promptForEdit(base url cancel) = (%+v,%v), want zero args + cancel", parsed, err)
	}

	parsed, err = root.promptForEdit(newPromptSession(strings.NewReader("2\n")), profileMutationArgs{}, domainprofile.Profile{
		Name:    "relay-a",
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
	})
	if !errors.Is(err, errInteractiveCanceled) || parsed.name != "" || parsed.baseURL != nil || parsed.apiKey != nil || len(parsed.bind) != 0 || len(parsed.unbind) != 0 || parsed.mutationFlags != 0 || parsed.asJSON {
		t.Fatalf("promptForEdit(api key cancel) = (%+v,%v), want zero args + cancel", parsed, err)
	}

	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() error = %v", err)
	}
	defer ptyMaster.Close()
	defer ptySlave.Close()

	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, err := root.promptAPIKey(&promptSession{
			lines:        &fileLineReader{file: ptySlave},
			passwordFile: ptySlave,
		}, "API key [keep current]: ", "sk-current")
		done <- result{value: value, err: err}
	}()
	if _, err := ptyMaster.Write([]byte("\n")); err != nil {
		t.Fatalf("Write(blank) error = %v", err)
	}
	got := <-done
	if got.err != nil || got.value != "sk-current" {
		t.Fatalf("promptAPIKey(TTY keep current) = (%q,%v), want sk-current,nil", got.value, got.err)
	}

	stdout.Reset()
	ptyMaster2, ptySlave2, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() second error = %v", err)
	}
	defer ptyMaster2.Close()
	defer ptySlave2.Close()

	done = make(chan result, 1)
	go func() {
		value, err := root.promptAPIKey(&promptSession{
			lines:        &fileLineReader{file: ptySlave2},
			passwordFile: ptySlave2,
		}, "API key: ", "")
		done <- result{value: value, err: err}
	}()
	if _, err := ptyMaster2.Write([]byte(" \nsk-tty-retry\n")); err != nil {
		t.Fatalf("Write(retry) error = %v", err)
	}
	got = <-done
	if got.err != nil || got.value != "sk-tty-retry" {
		t.Fatalf("promptAPIKey(TTY retry) = (%q,%v), want sk-tty-retry,nil", got.value, got.err)
	}
	if !strings.Contains(stdout.String(), "Invalid value:") {
		t.Fatalf("stdout=%q want invalid tty api key feedback", stdout.String())
	}
}

func TestReadAndRootZeroCoverageBranches(t *testing.T) {
	root, stdout, stderr, _ := newProfileRoot(t)

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"restore", "--help"}); code != 0 {
		t.Fatalf("restore --help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: agx restore") {
		t.Fatalf("stdout=%q want restore help", stdout.String())
	}

	stdout.Reset()
	if code := root.writeRestoreResult(&usecase.RestoreResult{
		Agent:      domainprofile.AgentClaude,
		ConfigPath: "/tmp/claude/settings.json",
		Backup: domainprofile.Backup{
			ID:          "backup-1",
			RestoreMode: domainprofile.RestoreModeRemoveCreatedFile,
		},
	}, false, true); code != 0 {
		t.Fatalf("writeRestoreResult(cleared text) code=%d stdout=%q", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Unbound agent: claude") {
		t.Fatalf("stdout=%q want cleared restore text", stdout.String())
	}

	defaultRoot := New(nil, BuildInfo{})
	defaultRoot.stdin = strings.NewReader("")
	if defaultRoot.canPrompt(false) {
		t.Fatal("defaultRoot.canPrompt(false) = true, want false")
	}
	defaultRoot.stdout = &bytes.Buffer{}
	defaultRoot.stderr = &bytes.Buffer{}
	if code := defaultRoot.Execute([]string{"--help"}); code != 0 {
		t.Fatalf("Execute(--help) code=%d", code)
	}
	helpText := defaultRoot.stdout.(*bytes.Buffer).String()
	if !strings.Contains(helpText, "AGX - Relay Manager") {
		t.Fatalf("stdout=%q want help text", helpText)
	}
	if !strings.Contains(helpText, "agx doctor [-o json]") || !strings.Contains(helpText, "agx rm <relay> [-o json]") {
		t.Fatalf("stdout=%q want root help JSON usage lines", helpText)
	}

	root = New(usecase.NewProfileService(&stubProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}, &stubStateRepo{loadErr: errors.New("state failed")}, nil, nil, nil), BuildInfo{})
	root.stdout = stdout
	root.stderr = stderr
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls"}); code == 0 {
		t.Fatalf("ls state failure unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "state failed") {
		t.Fatalf("stderr=%q want list state error", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"ls", "--agent"}); code == 0 {
		t.Fatalf("ls invalid args unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: agx ls") {
		t.Fatalf("stderr=%q want ls usage", stderr.String())
	}

	root = New(usecase.NewProfileService(&stubProfileRepo{getErr: errors.New("get failed")}, &stubStateRepo{}, nil, nil, nil), BuildInfo{})
	root.stdout = stdout
	root.stderr = stderr
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"show", "relay-a"}); code == 0 {
		t.Fatalf("show get failure unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "get failed") {
		t.Fatalf("stderr=%q want show get error", stderr.String())
	}

	root = New(usecase.NewProfileService(&stubProfileRepo{}, &stubStateRepo{loadErr: errors.New("backup failed")}, nil, nil, nil), BuildInfo{})
	root.stdout = stdout
	root.stderr = stderr
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"backup", "ls", "--agent", "codex"}); code == 0 {
		t.Fatalf("backup list failure unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "backup failed") {
		t.Fatalf("stderr=%q want backup list error", stderr.String())
	}

	root = New(usecase.NewProfileService(&stubProfileRepo{listErr: errors.New("doctor failed")}, &stubStateRepo{}, nil, nil, nil), BuildInfo{})
	root.stdout = stdout
	root.stderr = stderr
	root.stdin = strings.NewReader("")
	root.isTTY = func() bool { return false }

	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"doctor"}); code == 0 {
		t.Fatalf("doctor failure unexpectedly succeeded stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "doctor failed") {
		t.Fatalf("stderr=%q want doctor error", stderr.String())
	}

	root, stdout, stderr, _ = newProfileRoot(t)
	root.stdout = failingWriter{}
	if code := root.Execute([]string{"doctor", "-o", "json"}); code != 1 {
		t.Fatalf("doctor json encode failure code=%d stderr=%q", code, stderr.String())
	}
}
