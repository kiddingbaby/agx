package profile

import (
	"testing"
	"time"
)

func TestNotFoundError(t *testing.T) {
	if (&NotFoundError{}).Error() != "relay not found" {
		t.Fatalf("empty NotFoundError message mismatch")
	}
	if (&NotFoundError{Name: "relay-a"}).Error() != "relay not found: relay-a" {
		t.Fatalf("named NotFoundError message mismatch")
	}
}

func TestValidityHelpers(t *testing.T) {
	if !AgentCodex.Valid() || Agent("other").Valid() {
		t.Fatalf("Agent.Valid() mismatch")
	}
	if !BindingStatusApplied.Valid() || BindingStatus("broken").Valid() {
		t.Fatalf("BindingStatus.Valid() mismatch")
	}
	if !RestoreModeRestoreFile.Valid() || RestoreMode("broken").Valid() {
		t.Fatalf("RestoreMode.Valid() mismatch")
	}
}

func TestCodexStateAgentBindingRoundTrip(t *testing.T) {
	binding := AgentBinding{
		SourceProfile: "relay-a",
		Status:        BindingStatusApplied,
		ConfigPath:    "/tmp/codex/config.toml",
		LastBackupID:  "backup-1",
		Backups:       []Backup{{ID: "backup-1"}},
	}

	var state CodexState
	state.SetAgentBinding(binding)
	got := state.AgentBinding()
	if got.SourceProfile != binding.SourceProfile || got.Status != binding.Status || got.ConfigPath != binding.ConfigPath || got.LastBackupID != binding.LastBackupID || len(got.Backups) != 1 {
		t.Fatalf("AgentBinding round trip mismatch: %+v", got)
	}
}

func TestValidateProfileName(t *testing.T) {
	valid := []string{"relay-a", "relay_a", "relay.a", "relay1"}
	for _, name := range valid {
		if err := ValidateProfileName(name); err != nil {
			t.Fatalf("ValidateProfileName(%q) error = %v", name, err)
		}
	}

	invalid := []string{"", "relay a", "relay/a"}
	for _, name := range invalid {
		if err := ValidateProfileName(name); err == nil {
			t.Fatalf("ValidateProfileName(%q) unexpectedly succeeded", name)
		}
	}
}

func TestValidateTargetName(t *testing.T) {
	valid := []string{"work", "work_1", "work.prod", "Work"}
	for _, name := range valid {
		if err := ValidateTargetName(name); err != nil {
			t.Fatalf("ValidateTargetName(%q) error = %v", name, err)
		}
	}

	invalid := []string{"", "work alias", "../work", "work/alias"}
	for _, name := range invalid {
		if err := ValidateTargetName(name); err == nil {
			t.Fatalf("ValidateTargetName(%q) unexpectedly succeeded", name)
		}
	}
}

func TestValidateBaseURLAndAPIKey(t *testing.T) {
	if got := NormalizeBaseURL(" HTTPS://Relay.EXAMPLE/v1/ "); got != "https://relay.example/v1" {
		t.Fatalf("NormalizeBaseURL() = %q, want https://relay.example/v1", got)
	}
	if err := ValidateBaseURL("https://relay.example/v1"); err != nil {
		t.Fatalf("ValidateBaseURL(valid) error = %v", err)
	}
	if err := ValidateBaseURL("ftp://relay.example/v1"); err == nil {
		t.Fatal("ValidateBaseURL(invalid) unexpectedly succeeded")
	}
	if err := ValidateAPIKey("sk-test"); err != nil {
		t.Fatalf("ValidateAPIKey(valid) error = %v", err)
	}
	if err := ValidateAPIKey("bad\nkey"); err == nil {
		t.Fatal("ValidateAPIKey(invalid) unexpectedly succeeded")
	}
}

func TestValidateBaseURLAndNormalizeAdditionalBranches(t *testing.T) {
	if got := NormalizeBaseURL("https://relay.example/"); got != "https://relay.example/" {
		t.Fatalf("NormalizeBaseURL(root slash) = %q, want https://relay.example/", got)
	}
	if got := NormalizeBaseURL("://bad url"); got != "://bad url" {
		t.Fatalf("NormalizeBaseURL(parse fail) = %q, want original input", got)
	}
	if err := ValidateBaseURL(""); err == nil {
		t.Fatal("ValidateBaseURL(empty) unexpectedly succeeded")
	}
	if err := ValidateBaseURL("https:///path-only"); err == nil {
		t.Fatal("ValidateBaseURL(missing host) unexpectedly succeeded")
	}
	if err := ValidateBaseURL("https://relay.example/\x00"); err == nil {
		t.Fatal("ValidateBaseURL(invalid chars) unexpectedly succeeded")
	}
	if err := ValidateAPIKey(""); err == nil {
		t.Fatal("ValidateAPIKey(empty) unexpectedly succeeded")
	}
}

func TestAgentBaseURL(t *testing.T) {
	cases := []struct {
		agent Agent
		raw   string
		want  string
	}{
		{agent: AgentCodex, raw: "https://relay.example", want: "https://relay.example/v1"},
		{agent: AgentCodex, raw: "https://relay.example/v1/", want: "https://relay.example/v1"},
		{agent: AgentCodex, raw: "https://relay.example/api", want: "https://relay.example/api/v1"},
		{agent: AgentCodex, raw: "://bad url", want: "://bad url"},
		{agent: AgentClaude, raw: "https://relay.example/v1", want: "https://relay.example"},
		{agent: AgentClaude, raw: "https://relay.example/api/v1/", want: "https://relay.example/api"},
		{agent: AgentClaude, raw: "https://relay.example", want: "https://relay.example"},
		{agent: AgentGemini, raw: "https://relay.example/v1", want: "https://relay.example"},
		{agent: AgentGemini, raw: "https://relay.example/gemini", want: "https://relay.example/gemini"},
		{agent: AgentOpenCode, raw: "https://relay.example/v1", want: "https://relay.example/v1"},
		{agent: Agent("other"), raw: "https://relay.example/v1/", want: "https://relay.example/v1"},
	}
	for _, tc := range cases {
		if got := AgentBaseURL(tc.agent, tc.raw); got != tc.want {
			t.Fatalf("AgentBaseURL(%q, %q) = %q, want %q", tc.agent, tc.raw, got, tc.want)
		}
	}
}

func TestBaseURLHelpersAdditionalBranches(t *testing.T) {
	if got := BaseURLWithV1("https://relay.example/"); got != "https://relay.example/v1" {
		t.Fatalf("BaseURLWithV1(root) = %q", got)
	}
	if got := BaseURLWithoutTrailingV1("://bad url"); got != "://bad url" {
		t.Fatalf("BaseURLWithoutTrailingV1(parse fail) = %q", got)
	}
	if got := BaseURLWithoutTrailingV1("https://relay.example/"); got != "https://relay.example/" {
		t.Fatalf("BaseURLWithoutTrailingV1(root) = %q", got)
	}
}

func TestOpenCodeHelpers(t *testing.T) {
	for _, tc := range []struct {
		family OpenCodeProviderFamily
		valid  bool
	}{
		{family: "", valid: true},
		{family: OpenCodeProviderFamilyOpenAICompatible, valid: true},
		{family: OpenCodeProviderFamilyAnthropic, valid: true},
		{family: OpenCodeProviderFamilyGemini, valid: true},
		{family: OpenCodeProviderFamily("other"), valid: false},
	} {
		if got := tc.family.Valid(); got != tc.valid {
			t.Fatalf("OpenCodeProviderFamily(%q).Valid() = %v, want %v", tc.family, got, tc.valid)
		}
	}

	for _, tc := range []struct {
		raw  string
		want OpenCodeProviderFamily
		ok   bool
	}{
		{raw: " OPENAI-COMPATIBLE ", want: OpenCodeProviderFamilyOpenAICompatible, ok: true},
		{raw: "anthropic", want: OpenCodeProviderFamilyAnthropic, ok: true},
		{raw: "gemini", want: OpenCodeProviderFamilyGemini, ok: true},
		{raw: "", want: "", ok: false},
		{raw: "other", want: "", ok: false},
	} {
		got, ok := ParseOpenCodeProviderFamily(tc.raw)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("ParseOpenCodeProviderFamily(%q) = (%q,%v), want (%q,%v)", tc.raw, got, ok, tc.want, tc.ok)
		}
	}

	if got := OpenCodeProviderID(" Relay-A "); got != "agx-relay-a" {
		t.Fatalf("OpenCodeProviderID() = %q, want agx-relay-a", got)
	}
	if got := OpenCodeProviderID(" "); got != "" {
		t.Fatalf("OpenCodeProviderID(blank) = %q, want empty", got)
	}

	if err := ValidateOpenCodeModelID("gpt-4o"); err != nil {
		t.Fatalf("ValidateOpenCodeModelID(valid) error = %v", err)
	}
	for _, modelID := range []string{"", " \t ", "bad\nmodel"} {
		if err := ValidateOpenCodeModelID(modelID); err == nil {
			t.Fatalf("ValidateOpenCodeModelID(%q) unexpectedly succeeded", modelID)
		}
	}

	state := OpenCodeState{
		BindingView: BindingView{
			SourceProfile: "relay-a",
			Status:        BindingStatusApplied,
			ConfigPath:    "/tmp/opencode.json",
			LastAppliedAt: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC),
			LastBackupID:  "backup-1",
		},
		Backups: []Backup{{ID: "backup-1", BackupPath: "/tmp/backup-1"}},
	}
	gotBinding := state.AgentBinding()
	if gotBinding.SourceProfile != "relay-a" || gotBinding.Status != BindingStatusApplied || gotBinding.ConfigPath != "/tmp/opencode.json" || gotBinding.LastBackupID != "backup-1" || len(gotBinding.Backups) != 1 {
		t.Fatalf("OpenCodeState.AgentBinding() = %+v", gotBinding)
	}

	var roundTrip OpenCodeState
	roundTrip.SetAgentBinding(AgentBinding{
		SourceProfile: "relay-b",
		Status:        BindingStatusApplied,
		ConfigPath:    "/tmp/opencode-b.json",
		LastAppliedAt: time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC),
		LastBackupID:  "backup-2",
		Backups:       []Backup{{ID: "backup-2", BackupPath: "/tmp/backup-2"}},
	})
	if roundTrip.SourceProfile != "relay-b" || roundTrip.Status != BindingStatusApplied || roundTrip.ConfigPath != "/tmp/opencode-b.json" || roundTrip.LastBackupID != "backup-2" || len(roundTrip.Backups) != 1 {
		t.Fatalf("OpenCodeState.SetAgentBinding() = %+v", roundTrip)
	}
}

func TestNormalizeValidateAndResolveManagedProfile(t *testing.T) {
	profile := NormalizeProfile(Profile{
		Name:           " Work ",
		BaseURL:        " HTTPS://Relay.EXAMPLE/v1/ ",
		APIKey:         " sk-test ",
		ProviderFamily: OpenCodeProviderFamily(" OPENAI-COMPATIBLE "),
		ModelID:        " gpt-4o ",
	})
	if profile.Name != "work" || profile.Kind != ProfileKindRelay || profile.BaseURL != "https://relay.example/v1" || profile.APIKey != "sk-test" || profile.ProviderFamily != OpenCodeProviderFamilyOpenAICompatible || profile.ModelID != "gpt-4o" {
		t.Fatalf("NormalizeProfile() = %+v", profile)
	}
	if err := ValidateProfile(profile); err != nil {
		t.Fatalf("ValidateProfile(relay) error = %v", err)
	}

	key, err := ResolveCredential(profile)
	if err != nil || key != "sk-test" {
		t.Fatalf("ResolveCredential() = (%q,%v), want (sk-test,nil)", key, err)
	}

	bareProfile := NormalizeProfile(Profile{Name: "personal"})
	if bareProfile.Kind != ProfileKindRelay {
		t.Fatalf("NormalizeProfile(bare) kind = %q, want relay (only kind)", bareProfile.Kind)
	}
	if err := ValidateProfile(bareProfile); err == nil {
		t.Fatal("ValidateProfile(no base_url/api_key) should fail because relay requires both")
	}
}

func TestResolveCredentialRejectsMissingEnvValue(t *testing.T) {
	if _, err := ResolveCredential(Profile{Name: "work", Kind: ProfileKindRelay, BaseURL: "https://relay.example/v1"}); err == nil {
		t.Fatal("ResolveCredential(missing api key) unexpectedly succeeded")
	}
}
