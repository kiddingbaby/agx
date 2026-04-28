package profilefile

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	repo, err := NewRepository(filepath.Join(t.TempDir(), "profiles"))
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	return repo
}

func TestRepositoryUpsertGetListDelete(t *testing.T) {
	repo := newTestRepository(t)
	now := time.Now()
	profile := domainprofile.Profile{
		Name:      "relay-a",
		BaseURL:   "https://relay.example/v1",
		APIKey:    "sk-a",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := repo.Upsert(profile); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	got, err := repo.Get("relay-a")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.BaseURL != profile.BaseURL {
		t.Fatalf("got.BaseURL = %q, want %q", got.BaseURL, profile.BaseURL)
	}

	list, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}

	if err := repo.Delete("relay-a"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get("relay-a"); err == nil {
		t.Fatal("expected profile not found after delete")
	}
}

func TestRepositoryRejectsUnexpectedProfileFields(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	body := `name: relay-a
agent: codex
base-url: https://relay.example
api-key: sk-a
created-at: 2026-01-01T00:00:00Z
updated-at: 2026-01-01T00:00:00Z
`
	if err := os.WriteFile(filepath.Join(dir, "relay-a.yaml"), []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	repo, err := NewRepository(dir)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	_, err = repo.Get("relay-a")
	if err == nil || !strings.Contains(err.Error(), "field agent not found") {
		t.Fatalf("Get() err = %v, want unknown field error", err)
	}
}

func TestRepositoryConcurrentUpsertNoLostWrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	repo1, _ := NewRepository(dir)
	repo2, _ := NewRepository(dir)

	const each = 20
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < each; i++ {
			_, err := repo1.Upsert(domainprofile.Profile{
				Name:      "a-" + time.Now().Format("150405.000000000"),
				BaseURL:   "https://a.example/" + time.Now().Format("150405.000000000"),
				APIKey:    "sk-a",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
			if err != nil {
				t.Errorf("repo1 Upsert() error = %v", err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < each; i++ {
			_, err := repo2.Upsert(domainprofile.Profile{
				Name:      "b-" + time.Now().Format("150405.000000000"),
				BaseURL:   "https://b.example/" + time.Now().Format("150405.000000000"),
				APIKey:    "sk-b",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
			if err != nil {
				t.Errorf("repo2 Upsert() error = %v", err)
				return
			}
		}
	}()

	wg.Wait()

	list, err := repo1.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != each*2 {
		t.Fatalf("len(list) = %d, want %d", len(list), each*2)
	}
}

func TestStateRepositoryLoadSaveNewSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	repo := NewStateRepository(path)

	state := domainprofile.State{
		Codex: domainprofile.CodexState{
				BindingView: domainprofile.BindingView{
					SourceProfile: "relay-a",
					Status:        domainprofile.BindingStatusApplied,
					ConfigPath:    "/tmp/codex/config.toml",
					LastBackupID:  "before-codex-sync-20260424T000000Z",
				},
		},
		UpdatedAt: time.Now(),
	}
	if _, err := repo.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(raw), "codex:") || !strings.Contains(string(raw), "source-profile: relay-a") {
		t.Fatalf("state=%q want codex source-profile in latest schema", string(raw))
	}
	if strings.Contains(string(raw), "codex-default-profile:") {
		t.Fatalf("state=%q want no legacy codex-default-profile field", string(raw))
	}

	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Codex.SourceProfile != "relay-a" {
		t.Fatalf("Codex.SourceProfile = %q, want relay-a", got.Codex.SourceProfile)
	}
}

func TestStateRepositoryRejectsLegacyCodexDefaultField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	body := `codex:
  source-profile: relay-a
  status: applied
codex-default-profile: relay-a
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	repo := NewStateRepository(path)
	_, err := repo.Load()
	if err == nil || !strings.Contains(err.Error(), "field codex-default-profile not found") {
		t.Fatalf("Load() err = %v, want legacy codex-default-profile rejection", err)
	}
}

func TestStateRepositoryRejectsUnexpectedStateFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	body := `current-profile: relay-a
codex:
  source-profile: relay-a
  profile-name: agx/relay-a
  config-path: /tmp/codex/config.toml
  backup-path: /tmp/codex/config.toml.bak
  synced-at: 2026-01-01T00:00:00Z
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	repo := NewStateRepository(path)
	_, err := repo.Load()
	if err == nil || !strings.Contains(err.Error(), "field current-profile not found") {
		t.Fatalf("Load() err = %v, want unknown field error", err)
	}
}
