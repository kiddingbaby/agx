package profile

import "testing"

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
