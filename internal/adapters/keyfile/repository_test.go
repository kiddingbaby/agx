package keyfile

import (
	"os"
	"path/filepath"
	"strings"
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
	k, err := repo.Add(domainkey.ProviderClaude, "test", apiKey, "", []string{"dev"})
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
	active, err := repo.GetActive(domainkey.ProviderClaude)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.APIKey != apiKey {
		t.Fatalf("GetActive().APIKey = %q, want %q", active.APIKey, apiKey)
	}
	if !repo.HasActive(domainkey.ProviderClaude) {
		t.Fatal("HasActive() = false, want true")
	}
}

func TestRepositoryActivateDeactivatesSameProviderOnly(t *testing.T) {
	repo := newTestRepository(t)
	k1, _ := repo.Add(domainkey.ProviderClaude, "c1", "k1", "", nil)
	k2, _ := repo.Add(domainkey.ProviderClaude, "c2", "k2", "", nil)
	k3, _ := repo.Add(domainkey.ProviderOpenAI, "o1", "k3", "", nil)

	if err := repo.Activate(k1.ID); err != nil {
		t.Fatalf("Activate(k1) error = %v", err)
	}
	if err := repo.Activate(k3.ID); err != nil {
		t.Fatalf("Activate(k3) error = %v", err)
	}
	if err := repo.Activate(k2.ID); err != nil {
		t.Fatalf("Activate(k2) error = %v", err)
	}

	claude, err := repo.GetActive(domainkey.ProviderClaude)
	if err != nil {
		t.Fatalf("GetActive(claude) error = %v", err)
	}
	if claude.ID != k2.ID {
		t.Fatalf("active claude = %s, want %s", claude.ID, k2.ID)
	}
	openai, err := repo.GetActive(domainkey.ProviderOpenAI)
	if err != nil {
		t.Fatalf("GetActive(openai) error = %v", err)
	}
	if openai.ID != k3.ID {
		t.Fatalf("active openai = %s, want %s", openai.ID, k3.ID)
	}
}

func TestRepositoryUpdatePatchKeepsAPIKeyWhenEmpty(t *testing.T) {
	repo := newTestRepository(t)
	k, err := repo.Add(domainkey.ProviderClaude, "old", "old-key", "https://old.example", []string{"old"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := repo.Activate(k.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	before := k.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	updated, err := repo.Update(k.ID, domainkey.ProviderOpenAI, "new", "", "https://new.example", []string{"new", "prod"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !updated.UpdatedAt.After(before) {
		t.Fatalf("UpdatedAt not advanced: before=%v after=%v", before, updated.UpdatedAt)
	}

	active, err := repo.GetActive(domainkey.ProviderOpenAI)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.APIKey != "old-key" {
		t.Fatalf("APIKey = %q, want %q", active.APIKey, "old-key")
	}
}

func TestRepositoryDeleteAndNotFound(t *testing.T) {
	repo := newTestRepository(t)
	k, _ := repo.Add(domainkey.ProviderClaude, "name", "key", "", nil)
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

	k, _ := repo1.Add(domainkey.ProviderClaude, "persist", "api-key", "", []string{"tag1"})
	if err := repo1.Activate(k.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	repo2, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	list := repo2.List()
	if len(list) != 1 {
		t.Fatalf("List() length = %d, want 1", len(list))
	}
	if list[0].Name != "persist" || !list[0].Active {
		t.Fatalf("persisted key mismatch: %+v", list[0])
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
}

func TestRepositoryWrongSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	secret2 := []byte("abcdefghijklmnopqrstuvwxyz123456")

	repo1, err := NewRepository(path, testSecret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	k, _ := repo1.Add(domainkey.ProviderClaude, "test", "my-api-key", "", nil)
	if err := repo1.Activate(k.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	repo2, err := NewRepository(path, secret2)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	if _, err := repo2.GetActive(domainkey.ProviderClaude); err == nil {
		t.Fatal("expected decryption error")
	}
}
