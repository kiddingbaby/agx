package key

import (
	"os"
	"path/filepath"
	"testing"
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
	key, err := store.Add(ProviderClaude, "test-key", "sk-ant-api03-xxxxx", []string{"dev"})
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
	key1, _ := store.Add(ProviderClaude, "key1", "api-key-1", nil)
	key2, _ := store.Add(ProviderClaude, "key2", "api-key-2", nil)

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

	key, _ := store.Add(ProviderClaude, "test", "api-key", nil)

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
	key, _ := store.Add(ProviderClaude, "test", originalKey, nil)

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
	key, _ := store1.Add(ProviderClaude, "persist-test", "api-key", []string{"tag1"})
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
