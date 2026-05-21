package usecase

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kiddingbaby/agx/internal/adapters/opencodeconfig"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestOpenCodeStateCleanupHelpers(t *testing.T) {
	repo := &fakeProfileRepo{}
	stateRepo := &fakeStateRepo{state: domainprofile.State{
		OpenCode: domainprofile.OpenCodeState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-a",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/opencode.json",
			},
		},
	}}
	openCode := &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json")}
	openCode.status = &ports.OpenCodeConfigStatus{
		ConfigPath:           "/tmp/opencode.json",
		DefaultModel:         "",
		ManagedProvidersByID: map[string]ports.OpenCodeManagedProvider{},
	}
	svc := NewProfileService(repo, stateRepo, nil, nil, nil, nil)
	svc.openCode = openCode

	if err := svc.clearOpenCodeSourceProfileIfMatches("relay-a"); err != nil {
		t.Fatalf("clearOpenCodeSourceProfileIfMatches() error = %v", err)
	}
	if stateRepo.state.OpenCode.SourceProfile != "" {
		t.Fatalf("OpenCode.SourceProfile = %q, want cleared", stateRepo.state.OpenCode.SourceProfile)
	}

	if err := svc.removeOpenCodeProfileArtifacts("relay-a"); err != nil {
		t.Fatalf("removeOpenCodeProfileArtifacts() error = %v", err)
	}
	if len(openCode.removeProfileCalls) != 1 || openCode.removeProfileCalls[0] != "relay-a" {
		t.Fatalf("removeOpenCodeProfileArtifacts() removeProfileCalls=%v", openCode.removeProfileCalls)
	}
}

func TestClearOpenCodeOnlyClearsDefaultModel(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	backupsDir := filepath.Join(dir, "backups")
	raw := `{
  "$schema": "https://opencode.ai/config.json",
  "model": "agx-relay-open/gpt-4o",
  "provider": {
    "agx-relay-open": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "relay-open",
      "models": {
        "gpt-4o": {
          "name": "GPT-4o"
        }
      },
      "options": {
        "baseURL": "https://relay.example/v1",
        "apiKey": "sk-open"
      }
    },
    "external": {
      "name": "External"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile(opencode) error = %v", err)
	}

	now := time.Now().UTC()
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-open": {Name: "relay-open", BaseURL: "https://relay.example/v1", APIKey: "sk-open", CreatedAt: now, UpdatedAt: now},
	}}
	stateRepo := &fakeStateRepo{state: domainprofile.State{
		OpenCode: domainprofile.OpenCodeState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-open",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    configPath,
			},
		},
	}}
	svc := NewProfileService(repo, stateRepo, nil, nil, nil, opencodeconfig.NewSyncer(configPath, backupsDir))

	result, err := svc.Clear(domainprofile.AgentOpenCode)
	if err != nil {
		t.Fatalf("Clear(opencode) error = %v", err)
	}
	if result.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", result.ConfigPath, configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(opencode after clear) error = %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Unmarshal(opencode after clear) error = %v", err)
	}
	if _, ok := settings["model"]; ok {
		t.Fatalf("opencode model still exists after clear: %+v", settings)
	}
	providers, ok := settings["provider"].(map[string]any)
	if !ok {
		t.Fatalf("provider missing after clear: %+v", settings)
	}
	if _, ok := providers["agx-relay-open"]; !ok {
		t.Fatalf("managed provider missing after clear: %+v", providers)
	}
	if _, ok := providers["external"]; !ok {
		t.Fatalf("external provider missing after clear: %+v", providers)
	}
	if stateRepo.state.OpenCode.SourceProfile != "" {
		t.Fatalf("OpenCode.SourceProfile = %q, want cleared", stateRepo.state.OpenCode.SourceProfile)
	}
}

func TestUnbindCurrentOpenCodeRemovesManagedProviderOnly(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	raw := `{
  "$schema": "https://opencode.ai/config.json",
  "model": "agx-relay-open/gpt-4o",
  "provider": {
    "agx-relay-open": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "relay-open",
      "models": {
        "gpt-4o": {
          "name": "GPT-4o"
        }
      },
      "options": {
        "baseURL": "https://relay.example/v1",
        "apiKey": "sk-open"
      }
    },
    "external": {
      "name": "External"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile(opencode) error = %v", err)
	}

	now := time.Now().UTC()
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-open": {Name: "relay-open", BaseURL: "https://relay.example/v1", APIKey: "sk-open", CreatedAt: now, UpdatedAt: now},
	}}
	stateRepo := &fakeStateRepo{state: domainprofile.State{
		OpenCode: domainprofile.OpenCodeState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-open",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    configPath,
			},
		},
	}}
	svc := NewProfileService(repo, stateRepo, nil, nil, nil, opencodeconfig.NewSyncer(configPath, filepath.Join(dir, "backups")))

	result, err := svc.applyRelayBindingChanges("relay-open", nil, []domainprofile.Agent{domainprofile.AgentOpenCode})
	if err != nil {
		t.Fatalf("applyRelayBindingChanges(unbind opencode) error = %v", err)
	}
	if len(result.Changed) != 1 || result.Changed[0].Action != "unbind" {
		t.Fatalf("applyRelayBindingChanges() = %+v, want one unbind", result.Changed)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(opencode after unbind) error = %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Unmarshal(opencode after unbind) error = %v", err)
	}
	if _, ok := settings["model"]; ok {
		t.Fatalf("opencode model still exists after unbind: %+v", settings)
	}
	providers, ok := settings["provider"].(map[string]any)
	if !ok {
		t.Fatalf("provider missing after unbind: %+v", settings)
	}
	if _, ok := providers["agx-relay-open"]; ok {
		t.Fatalf("managed provider still exists after unbind: %+v", providers)
	}
	if _, ok := providers["external"]; !ok {
		t.Fatalf("external provider missing after unbind: %+v", providers)
	}
}

func TestRemoveOpenCodeProfileLockedErrorBranches(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-open": {Name: "relay-open", BaseURL: "https://relay.example/v1", APIKey: "sk-open", CreatedAt: now, UpdatedAt: now},
	}}
	state := domainprofile.State{
		OpenCode: domainprofile.OpenCodeState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-open",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/opencode.json",
			},
		},
	}

	if _, err := NewProfileService(repo, &fakeStateRepo{state: state}, nil, nil, nil, nil).removeOpenCodeProfileLocked("relay-open"); err == nil {
		t.Fatal("removeOpenCodeProfileLocked(nil syncer) unexpectedly succeeded")
	}

	openCode := &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json"), removeProfileErr: errors.New("remove profile failed")}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: state}, nil, nil, nil, openCode).removeOpenCodeProfileLocked("relay-open"); err == nil || !strings.Contains(err.Error(), "remove profile failed") {
		t.Fatalf("removeOpenCodeProfileLocked(remove failure) err=%v, want remove profile failed", err)
	}

	openCode = &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json"), statusErr: errors.New("status failed")}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: state}, nil, nil, nil, openCode).removeOpenCodeProfileLocked("relay-open"); err == nil || !strings.Contains(err.Error(), "status failed") {
		t.Fatalf("removeOpenCodeProfileLocked(status failure) err=%v, want status failed", err)
	}
}

func TestOpenCodeRenameUnbindAndRestoreBranches(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-old":     {Name: "relay-old", BaseURL: "https://relay.example/v1", APIKey: "sk-old", CreatedAt: now, UpdatedAt: now},
		"relay-current": {Name: "relay-current", BaseURL: "https://relay.example/v1", APIKey: "sk-current", CreatedAt: now, UpdatedAt: now},
	}}

	openCode := &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json")}
	openCode.snapshotContent = []byte("before openCode")
	state := domainprofile.State{
		OpenCode: domainprofile.OpenCodeState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-old",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/opencode.json",
			},
		},
	}
	svc := NewProfileService(repo, &fakeStateRepo{state: state}, nil, nil, nil, openCode)

	renamed, err := svc.renameProfile(
		"relay-old",
		domainprofile.Profile{Name: "relay-renamed", BaseURL: "https://relay.example/v1", APIKey: "sk-old", CreatedAt: now, UpdatedAt: now},
		state,
		now,
	)
	if err != nil {
		t.Fatalf("renameProfile() error = %v", err)
	}
	if renamed.Name != "relay-renamed" {
		t.Fatalf("renamed profile = %q, want relay-renamed", renamed.Name)
	}
	if len(openCode.removeProfileCalls) != 1 || openCode.removeProfileCalls[0] != "relay-old" {
		t.Fatalf("renameProfile() removeProfileCalls=%v, want relay-old", openCode.removeProfileCalls)
	}

	stateRepo2 := &fakeStateRepo{state: domainprofile.State{
		OpenCode: domainprofile.OpenCodeState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-current",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/opencode.json",
			},
		},
	}}
	openCode2 := &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json")}
	openCode2.snapshotContent = []byte("before unbind")
	openCode2.status = &ports.OpenCodeConfigStatus{
		ConfigPath:   "/tmp/opencode.json",
		DefaultModel: "agx-relay-current/model-current",
		ManagedProvidersByID: map[string]ports.OpenCodeManagedProvider{
			"agx-relay-current": {ID: "agx-relay-current", Family: domainprofile.OpenCodeProviderFamilyGemini, Model: "model-current"},
		},
	}
	svc2 := NewProfileService(repo, stateRepo2, nil, nil, nil, openCode2)

	backupFile := filepath.Join(t.TempDir(), "opencode-backup.txt")
	if err := os.WriteFile(backupFile, []byte("restored openCode"), 0o600); err != nil {
		t.Fatalf("WriteFile(backup) error = %v", err)
	}
	if err := svc2.restoreBindingChange(BindingChangeResult{
		Agent: domainprofile.AgentOpenCode,
		Backup: domainprofile.Backup{
			ID:          "backup-1",
			RestoreMode: domainprofile.RestoreModeRestoreFile,
			BackupPath:  backupFile,
		},
	}); err != nil {
		t.Fatalf("restoreBindingChange(openCode) error = %v", err)
	}
	if string(openCode2.snapshotContent) != "restored openCode" {
		t.Fatalf("restoreBindingChange(openCode) snapshot=%q, want restored content", string(openCode2.snapshotContent))
	}
}
