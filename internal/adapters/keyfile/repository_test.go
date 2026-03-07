package keyfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
)

var testSecret = []byte("12345678901234567890123456789012")

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	path := filepath.Join(t.TempDir(), "keys.yaml")
	repo, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	return repo
}

func TestNewRepositoryValidation(t *testing.T) {
	t.Run("valid secret", func(t *testing.T) {
		_ = newTestRepository(t)
	})

	t.Run("invalid secret length", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "keys.yaml")
		if _, err := NewRepository(path, []byte("short")); err == nil {
			t.Fatal("expected secret length error")
		}
	})
}

func TestRepositoryAddActivateGetActive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.yaml")
	repo, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	apiKey := "sk-ant-api03-xxxxx"
	k, err := repo.Add(domainkey.ProviderClaude, domainkey.DefaultProfile, "test", apiKey, "", []string{"dev"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if k.ID == "" {
		t.Fatal("expected key ID")
	}
	if k.APIKey == apiKey {
		t.Fatal("Add() should not return plaintext api key")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), apiKey) {
		t.Fatal("api key leaked in plaintext file")
	}

	if err := repo.Activate(k.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	active, err := repo.GetActive(domainkey.ProviderClaude, domainkey.DefaultProfile)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.APIKey != apiKey {
		t.Fatalf("GetActive().APIKey = %q, want %q", active.APIKey, apiKey)
	}
	if !repo.HasActive(domainkey.ProviderClaude, domainkey.DefaultProfile) {
		t.Fatal("HasActive() = false, want true")
	}
}

func TestRepositoryActivateDeactivatesWithinProfileOnly(t *testing.T) {
	repo := newTestRepository(t)
	k1, _ := repo.Add(domainkey.ProviderClaude, "default", "c1", "k1", "", nil)
	k2, _ := repo.Add(domainkey.ProviderClaude, "default", "c2", "k2", "", nil)
	k3, _ := repo.Add(domainkey.ProviderClaude, "prod", "c3", "k3", "", nil)

	if err := repo.Activate(k1.ID); err != nil {
		t.Fatalf("Activate(k1) error = %v", err)
	}
	if err := repo.Activate(k3.ID); err != nil {
		t.Fatalf("Activate(k3) error = %v", err)
	}
	if err := repo.Activate(k2.ID); err != nil {
		t.Fatalf("Activate(k2) error = %v", err)
	}

	def, err := repo.GetActive(domainkey.ProviderClaude, "default")
	if err != nil {
		t.Fatalf("GetActive(default) error = %v", err)
	}
	if def.ID != k2.ID {
		t.Fatalf("active default = %s, want %s", def.ID, k2.ID)
	}
	prod, err := repo.GetActive(domainkey.ProviderClaude, "prod")
	if err != nil {
		t.Fatalf("GetActive(prod) error = %v", err)
	}
	if prod.ID != k3.ID {
		t.Fatalf("active prod = %s, want %s", prod.ID, k3.ID)
	}
}

func TestRepositoryResolveRoundRobin(t *testing.T) {
	repo := newTestRepository(t)
	k1, _ := repo.Add(domainkey.ProviderOpenAI, "prod", "k1", "v1", "", nil)
	k2, _ := repo.Add(domainkey.ProviderOpenAI, "prod", "k2", "v2", "", nil)
	k3, _ := repo.Add(domainkey.ProviderOpenAI, "prod", "k3", "v3", "", nil)

	if err := repo.SetProfileStrategy(domainkey.ProviderOpenAI, "prod", domainkey.StrategyRoundRobin, ""); err != nil {
		t.Fatalf("SetProfileStrategy() error = %v", err)
	}

	got1, err := repo.Resolve(domainkey.ProviderOpenAI, "prod", "")
	if err != nil {
		t.Fatalf("Resolve #1 error = %v", err)
	}
	got2, err := repo.Resolve(domainkey.ProviderOpenAI, "prod", "")
	if err != nil {
		t.Fatalf("Resolve #2 error = %v", err)
	}
	got3, err := repo.Resolve(domainkey.ProviderOpenAI, "prod", "")
	if err != nil {
		t.Fatalf("Resolve #3 error = %v", err)
	}

	ids := []string{got1.ID, got2.ID, got3.ID}
	want := []string{k1.ID, k2.ID, k3.ID}
	for i := range ids {
		if ids[i] != want[i] {
			t.Fatalf("round robin ids[%d] = %s, want %s", i, ids[i], want[i])
		}
	}
}

func TestRepositoryResolveFixedByIdentifier(t *testing.T) {
	repo := newTestRepository(t)
	k1, _ := repo.Add(domainkey.ProviderGemini, "work", "g1", "v1", "", nil)
	k2, _ := repo.Add(domainkey.ProviderGemini, "work", "g2", "v2", "", nil)

	if err := repo.SetProfileStrategy(domainkey.ProviderGemini, "work", domainkey.StrategyFixed, k2.Name); err != nil {
		t.Fatalf("SetProfileStrategy() error = %v", err)
	}

	got, err := repo.Resolve(domainkey.ProviderGemini, "work", "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.ID != k2.ID {
		t.Fatalf("Resolve() id = %s, want %s", got.ID, k2.ID)
	}

	gotExplicit, err := repo.Resolve(domainkey.ProviderGemini, "work", k1.ID[:8])
	if err != nil {
		t.Fatalf("Resolve(explicit) error = %v", err)
	}
	if gotExplicit.ID != k1.ID {
		t.Fatalf("Resolve(explicit) id = %s, want %s", gotExplicit.ID, k1.ID)
	}
}

func TestRepositoryResolveRejectsAmbiguousPrefix(t *testing.T) {
	repo := newTestRepository(t)
	_, _ = repo.Add(domainkey.ProviderClaude, "prod", "claude-a", "v1", "", nil)
	_, _ = repo.Add(domainkey.ProviderClaude, "prod", "claude-b", "v2", "", nil)
	if err := repo.withWriteLock(func() error {
		repo.keys[0].ID = "dup-1111"
		repo.keys[1].ID = "dup-2222"
		return nil
	}); err != nil {
		t.Fatalf("withWriteLock() error = %v", err)
	}

	prefix := "dup"
	if _, err := repo.Resolve(domainkey.ProviderClaude, "prod", prefix); err == nil || !strings.Contains(err.Error(), "ambiguous key identifier") {
		t.Fatalf("Resolve(ambiguous prefix) err = %v, want ambiguous key identifier", err)
	}
}

func TestRepositorySetProfileStrategyRejectsAmbiguousFixedKey(t *testing.T) {
	repo := newTestRepository(t)
	_, _ = repo.Add(domainkey.ProviderGemini, "work", "g1", "v1", "", nil)
	_, _ = repo.Add(domainkey.ProviderGemini, "work", "g2", "v2", "", nil)
	if err := repo.withWriteLock(func() error {
		repo.keys[0].ID = "fix-1111"
		repo.keys[1].ID = "fix-2222"
		return nil
	}); err != nil {
		t.Fatalf("withWriteLock() error = %v", err)
	}

	prefix := "fix"
	if err := repo.SetProfileStrategy(domainkey.ProviderGemini, "work", domainkey.StrategyFixed, prefix); err == nil || !strings.Contains(err.Error(), "ambiguous key identifier") {
		t.Fatalf("SetProfileStrategy(ambiguous fixed key) err = %v, want ambiguous key identifier", err)
	}
}

func TestRepositoryUpdatePatchKeepsAPIKeyWhenEmpty(t *testing.T) {
	repo := newTestRepository(t)
	k, err := repo.Add(domainkey.ProviderClaude, "default", "old", "old-key", "https://old.example", []string{"old"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := repo.Activate(k.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	before := k.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	updated, err := repo.Update(k.ID, domainkey.ProviderOpenAI, "prod", "new", "", "https://new.example", []string{"new", "prod"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !updated.UpdatedAt.After(before) {
		t.Fatalf("UpdatedAt not advanced: before=%v after=%v", before, updated.UpdatedAt)
	}

	resolved, err := repo.Resolve(domainkey.ProviderOpenAI, "prod", "new")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.APIKey != "old-key" {
		t.Fatalf("APIKey = %q, want %q", resolved.APIKey, "old-key")
	}
}

func TestRepositoryDeleteAndNotFound(t *testing.T) {
	repo := newTestRepository(t)
	k, _ := repo.Add(domainkey.ProviderClaude, "default", "name", "key", "", nil)
	if err := repo.Delete(k.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got := len(repo.List()); got != 0 {
		t.Fatalf("List() length = %d, want 0", got)
	}
	if err := repo.Delete("missing"); err == nil {
		t.Fatal("expected key not found error")
	}
}

func TestRepositoryPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "keys.yaml")
	repo1, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	k, _ := repo1.Add(domainkey.ProviderClaude, "prod", "persist", "api-key", "", []string{"tag1"})
	if err := repo1.Activate(k.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if err := repo1.SetProfileStrategy(domainkey.ProviderClaude, "prod", domainkey.StrategyFixed, k.ID); err != nil {
		t.Fatalf("SetProfileStrategy() error = %v", err)
	}

	repo2, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	list := repo2.List()
	if len(list) != 1 {
		t.Fatalf("List() length = %d, want 1", len(list))
	}
	if list[0].Name != "persist" || !list[0].Active || list[0].Profile != "prod" {
		t.Fatalf("persisted key mismatch: %+v", list[0])
	}

	profiles := repo2.ListProfiles(domainkey.ProviderClaude)
	if len(profiles) == 0 {
		t.Fatal("expected profiles persisted")
	}
}

func TestRepositoryLoadLegacyMissingUpdatedAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.yaml")
	legacy := `keys:
  - id: legacy-1
    provider: claude
    name: legacy-key
    api_key: Zm9v
    active: false
    created_at: 2026-01-02T03:04:05Z
`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	repo, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	list := repo.List()
	if len(list) != 1 {
		t.Fatalf("List() length = %d, want 1", len(list))
	}
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if !list[0].UpdatedAt.Equal(want) {
		t.Fatalf("UpdatedAt = %v, want %v", list[0].UpdatedAt, want)
	}
	if list[0].Profile != domainkey.DefaultProfile {
		t.Fatalf("Profile = %q, want %q", list[0].Profile, domainkey.DefaultProfile)
	}
}

func TestRepositoryWrongSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	secret2 := []byte("abcdefghijklmnopqrstuvwxyz123456")

	repo1, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	k, _ := repo1.Add(domainkey.ProviderClaude, "default", "test", "my-api-key", "", nil)
	if err := repo1.Activate(k.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	repo2, err := NewRepository(path, secret2)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	if _, err := repo2.GetActive(domainkey.ProviderClaude, "default"); err == nil {
		t.Fatal("expected decryption error")
	}
}

func TestRepositoryConcurrentAddNoLostWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.yaml")
	repo1, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository(repo1) error = %v", err)
	}
	repo2, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository(repo2) error = %v", err)
	}

	const each = 40
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < each; i++ {
			_, addErr := repo1.Add(domainkey.ProviderClaude, "default", fmt.Sprintf("r1-%02d", i), fmt.Sprintf("k1-%02d", i), "", nil)
			if addErr != nil {
				t.Errorf("repo1 Add() error = %v", addErr)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < each; i++ {
			_, addErr := repo2.Add(domainkey.ProviderClaude, "default", fmt.Sprintf("r2-%02d", i), fmt.Sprintf("k2-%02d", i), "", nil)
			if addErr != nil {
				t.Errorf("repo2 Add() error = %v", addErr)
				return
			}
		}
	}()

	wg.Wait()
	finalRepo, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository(final) error = %v", err)
	}
	if got := len(finalRepo.List()); got != each*2 {
		t.Fatalf("List() length = %d, want %d", got, each*2)
	}
}
