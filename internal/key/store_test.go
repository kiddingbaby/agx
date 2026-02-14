package key

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	secret := []byte("12345678901234567890123456789012") // 32 bytes

	t.Run("valid secret", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "keys.yaml")

		store, err := NewStore(path, secret)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		if store == nil {
			t.Fatal("NewStore() returned nil")
		}
		if len(store.Keys) != 0 {
			t.Errorf("expected empty keys, got %d", len(store.Keys))
		}
	})

	t.Run("invalid secret length", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "keys.yaml")

		_, err := NewStore(path, []byte("short"))
		if err == nil {
			t.Error("expected error for short secret")
		}
	})
}

func TestStoreAddAndGet(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Add a key
	key, err := store.Add(ProviderClaude, "test-key", "sk-ant-api03-xxxxx", "", []string{"dev"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if key.ID == "" {
		t.Error("key ID is empty")
	}
	if key.Provider != ProviderClaude {
		t.Errorf("Provider = %v, want claude", key.Provider)
	}
	if key.Name != "test-key" {
		t.Errorf("Name = %v, want test-key", key.Name)
	}
	if key.Active {
		t.Error("new key should not be active")
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("keys file not created")
	}

	// Verify key is in store
	if len(store.Keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(store.Keys))
	}
}

func TestStoreActivate(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Add two keys
	key1, _ := store.Add(ProviderClaude, "key1", "api-key-1", "", nil)
	key2, _ := store.Add(ProviderClaude, "key2", "api-key-2", "", nil)

	// Activate first key
	if err := store.Activate(key1.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	// Verify first is active
	active, err := store.GetActive(ProviderClaude)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.ID != key1.ID {
		t.Errorf("active key ID = %v, want %v", active.ID, key1.ID)
	}

	// Activate second key
	if err := store.Activate(key2.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	// Verify second is now active (first is deactivated)
	active, err = store.GetActive(ProviderClaude)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.ID != key2.ID {
		t.Errorf("active key ID = %v, want %v", active.ID, key2.ID)
	}
}

func TestStoreDelete(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	key, _ := store.Add(ProviderClaude, "test", "api-key", "", nil)

	if err := store.Delete(key.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if len(store.Keys) != 0 {
		t.Errorf("expected 0 keys after delete, got %d", len(store.Keys))
	}
}

func TestStoreDeleteNotFound(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if err := store.Delete("nonexistent"); err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestStoreEncryptDecrypt(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	originalKey := "sk-ant-api03-xxxxxxxxxxxxx"
	key, _ := store.Add(ProviderClaude, "test", originalKey, "", nil)

	// Activate to get decrypted key
	store.Activate(key.ID)
	active, err := store.GetActive(ProviderClaude)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}

	if active.APIKey != originalKey {
		t.Errorf("decrypted key = %v, want %v", active.APIKey, originalKey)
	}
}

func TestStorePersistence(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	// Create store and add key
	store1, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	key, _ := store1.Add(ProviderClaude, "persist-test", "api-key", "", []string{"tag1"})
	store1.Activate(key.ID)

	// Create new store from same path
	store2, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if len(store2.Keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(store2.Keys))
	}

	if store2.Keys[0].Name != "persist-test" {
		t.Errorf("Name = %v, want persist-test", store2.Keys[0].Name)
	}
	if !store2.Keys[0].Active {
		t.Error("key should be active")
	}
}

// Edge case tests

func TestStoreActivateNotFound(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	err = store.Activate("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestStoreGetActiveNoActive(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Add key but don't activate
	store.Add(ProviderClaude, "test", "api-key", "", nil)

	_, err = store.GetActive(ProviderClaude)
	if err == nil {
		t.Error("expected error when no active key")
	}
}

func TestStoreGetActiveWrongProvider(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	key, _ := store.Add(ProviderClaude, "test", "api-key", "", nil)
	store.Activate(key.ID)

	// Try to get active for different provider
	_, err = store.GetActive(ProviderOpenAI)
	if err == nil {
		t.Error("expected error for wrong provider")
	}
}

func TestStoreMultipleProviders(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Add keys for multiple providers
	claudeKey, _ := store.Add(ProviderClaude, "claude-key", "sk-claude", "", nil)
	openaiKey, _ := store.Add(ProviderOpenAI, "openai-key", "sk-openai", "", nil)
	geminiKey, _ := store.Add(ProviderGemini, "gemini-key", "sk-gemini", "", nil)

	// Activate all
	store.Activate(claudeKey.ID)
	store.Activate(openaiKey.ID)
	store.Activate(geminiKey.ID)

	// Verify each provider has its own active key
	active, err := store.GetActive(ProviderClaude)
	if err != nil || active.APIKey != "sk-claude" {
		t.Error("Claude key not correctly activated")
	}

	active, err = store.GetActive(ProviderOpenAI)
	if err != nil || active.APIKey != "sk-openai" {
		t.Error("OpenAI key not correctly activated")
	}

	active, err = store.GetActive(ProviderGemini)
	if err != nil || active.APIKey != "sk-gemini" {
		t.Error("Gemini key not correctly activated")
	}
}

func TestStoreActivateDeactivatesOthers(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Add multiple keys for same provider
	key1, _ := store.Add(ProviderClaude, "key1", "api1", "", nil)
	key2, _ := store.Add(ProviderClaude, "key2", "api2", "", nil)
	key3, _ := store.Add(ProviderClaude, "key3", "api3", "", nil)

	// Activate key1
	store.Activate(key1.ID)
	if !store.Keys[0].Active || store.Keys[1].Active || store.Keys[2].Active {
		t.Error("Only key1 should be active")
	}

	// Activate key2 - key1 should be deactivated
	store.Activate(key2.ID)
	if store.Keys[0].Active || !store.Keys[1].Active || store.Keys[2].Active {
		t.Error("Only key2 should be active")
	}

	// Activate key3 - key2 should be deactivated
	store.Activate(key3.ID)
	if store.Keys[0].Active || store.Keys[1].Active || !store.Keys[2].Active {
		t.Error("Only key3 should be active")
	}
}

func TestStoreList(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Initially empty
	if len(store.List()) != 0 {
		t.Error("expected empty list initially")
	}

	// Add keys
	store.Add(ProviderClaude, "key1", "api1", "", nil)
	store.Add(ProviderOpenAI, "key2", "api2", "", nil)

	list := store.List()
	if len(list) != 2 {
		t.Errorf("expected 2 keys, got %d", len(list))
	}
}

func TestStoreEmptyAPIKey(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Add with empty API key (should still work - validation is caller's responsibility)
	key, err := store.Add(ProviderClaude, "empty-key", "", "", nil)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	store.Activate(key.ID)
	active, err := store.GetActive(ProviderClaude)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.APIKey != "" {
		t.Error("expected empty API key")
	}
}

func TestStoreSpecialCharactersInAPIKey(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// API key with special characters
	specialKey := "sk-ant-api03-xxx'yyy$zzz`cmd`\nline2\ttab"
	key, err := store.Add(ProviderClaude, "special", specialKey, "", nil)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	store.Activate(key.ID)
	active, err := store.GetActive(ProviderClaude)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.APIKey != specialKey {
		t.Errorf("APIKey = %q, want %q", active.APIKey, specialKey)
	}
}

func TestStoreUnicodeInName(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Unicode in name and tags
	key, err := store.Add(ProviderClaude, "测试密钥", "api-key", "", []string{"开发", "测试"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if key.Name != "测试密钥" {
		t.Errorf("Name = %v, want 测试密钥", key.Name)
	}
	if len(key.Tags) != 2 || key.Tags[0] != "开发" {
		t.Errorf("Tags not preserved correctly: %v", key.Tags)
	}
}

func TestStoreCorruptedFile(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	// Write corrupted YAML
	os.WriteFile(path, []byte("invalid: yaml: content: [[["), 0600)

	_, err := NewStore(path, secret)
	if err == nil {
		t.Error("expected error for corrupted file")
	}
}

func TestStoreWrongSecret(t *testing.T) {
	secret1 := []byte("12345678901234567890123456789012")
	secret2 := []byte("abcdefghijklmnopqrstuvwxyz123456")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	// Create store and add key with secret1
	store1, _ := NewStore(path, secret1)
	key, _ := store1.Add(ProviderClaude, "test", "my-api-key", "", nil)
	store1.Activate(key.ID)

	// Try to read with secret2
	store2, err := NewStore(path, secret2)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// GetActive should fail because decryption fails
	_, err = store2.GetActive(ProviderClaude)
	if err == nil {
		t.Error("expected decryption error with wrong secret")
	}
}

func TestStoreDeleteMiddleKey(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, _ := NewStore(path, secret)

	key1, _ := store.Add(ProviderClaude, "key1", "api1", "", nil)
	key2, _ := store.Add(ProviderClaude, "key2", "api2", "", nil)
	key3, _ := store.Add(ProviderClaude, "key3", "api3", "", nil)

	// Delete middle key
	store.Delete(key2.ID)

	if len(store.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(store.Keys))
	}
	if store.Keys[0].ID != key1.ID || store.Keys[1].ID != key3.ID {
		t.Error("wrong keys remaining after delete")
	}
}

func TestStoreCreatesDirectory(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "dir", "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Add and save to trigger directory creation
	store.Add(ProviderClaude, "test", "api-key", "", nil)

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file not created in nested directory")
	}
}

func TestStoreUpdatePatch(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	original, err := store.Add(ProviderClaude, "old-name", "old-key", "https://old.example", []string{"old"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	before := original.UpdatedAt

	time.Sleep(10 * time.Millisecond)

	updated, err := store.Update(
		original.ID,
		ProviderOpenAI,
		"new-name",
		"",
		"https://new.example",
		[]string{"new", "prod"},
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.Provider != ProviderOpenAI {
		t.Fatalf("Provider = %v, want %v", updated.Provider, ProviderOpenAI)
	}
	if updated.Name != "new-name" {
		t.Fatalf("Name = %q, want %q", updated.Name, "new-name")
	}
	if updated.BaseURL != "https://new.example" {
		t.Fatalf("BaseURL = %q, want %q", updated.BaseURL, "https://new.example")
	}
	if len(updated.Tags) != 2 || updated.Tags[0] != "new" || updated.Tags[1] != "prod" {
		t.Fatalf("Tags = %v, want [new prod]", updated.Tags)
	}
	if !updated.UpdatedAt.After(before) {
		t.Fatalf("UpdatedAt not advanced: before=%v after=%v", before, updated.UpdatedAt)
	}

	// Empty apiKey means keep original key.
	if err := store.Activate(original.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	active, err := store.GetActive(ProviderOpenAI)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.APIKey != "old-key" {
		t.Fatalf("APIKey = %q, want %q", active.APIKey, "old-key")
	}
}

func TestStoreLoadLegacyMissingUpdatedAt(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

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

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if len(store.Keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(store.Keys))
	}
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if !store.Keys[0].UpdatedAt.Equal(want) {
		t.Fatalf("UpdatedAt = %v, want %v", store.Keys[0].UpdatedAt, want)
	}
}

func TestStoreUpdateKeepsSingleActivePerProvider(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.yaml")

	store, err := NewStore(path, secret)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	key1, err := store.Add(ProviderClaude, "k1", "key-1", "", nil)
	if err != nil {
		t.Fatalf("Add key1 error = %v", err)
	}
	key2, err := store.Add(ProviderOpenAI, "k2", "key-2", "", nil)
	if err != nil {
		t.Fatalf("Add key2 error = %v", err)
	}

	if err := store.Activate(key1.ID); err != nil {
		t.Fatalf("Activate key1 error = %v", err)
	}
	if err := store.Activate(key2.ID); err != nil {
		t.Fatalf("Activate key2 error = %v", err)
	}

	// Move active key1 from Claude to OpenAI; only one OpenAI key should remain active.
	if _, err := store.Update(key1.ID, ProviderOpenAI, "k1", "", "", nil); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	activeOpenAI, err := store.GetActive(ProviderOpenAI)
	if err != nil {
		t.Fatalf("GetActive(OpenAI) error = %v", err)
	}
	if activeOpenAI.ID != key1.ID {
		t.Fatalf("active OpenAI key = %s, want %s", activeOpenAI.ID, key1.ID)
	}
}
