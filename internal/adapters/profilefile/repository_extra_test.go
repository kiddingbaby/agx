package profilefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func TestRepositoryDeleteAndGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	repo, err := NewRepository(dir)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	profile := domainprofile.Profile{
		Name:      "relay-a",
		BaseURL:   "https://relay.example/v1",
		APIKey:    "sk-a",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if _, err := repo.Upsert(profile); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := repo.Delete("relay-a"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get("relay-a"); err == nil {
		t.Fatal("Get() unexpectedly succeeded after delete")
	}
	if err := repo.Delete("relay-a"); err == nil {
		t.Fatal("Delete() unexpectedly succeeded for missing profile")
	}
}

func TestStateRepositoryAcquireLockAndLoadEmpty(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.yaml")
	repo := NewStateRepository(statePath)

	state, err := repo.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state.Codex.SourceProfile != "" || state.Claude.SourceProfile != "" || state.Gemini.SourceProfile != "" {
		t.Fatalf("Load() = %+v, want empty state", state)
	}
	if _, err := os.Stat(statePath + ".lock"); err != nil {
		t.Fatalf("expected lock file to exist after Load(), err = %v", err)
	}
}

func TestRepositoryListValidationAndSaveStateBranches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	repo, err := NewRepository(dir)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	now := time.Now().UTC()
	if _, err := repo.Upsert(domainprofile.Profile{
		Name:      "Relay-A",
		BaseURL:   "https://relay.example/v1/",
		APIKey:    "sk-a",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	listed, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Name != "relay-a" || listed[0].BaseURL != "https://relay.example/v1" {
		t.Fatalf("List() = %+v, want normalized profile", listed)
	}

	if err := os.WriteFile(filepath.Join(dir, "relay-b.yaml"), []byte("name: relay-c\nbase-url: https://relay.example/v1\napi-key: sk-b\ncreated-at: 2026-04-28T00:00:00Z\nupdated-at: 2026-04-28T00:00:00Z\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := repo.Get("relay-b"); err == nil || !strings.Contains(err.Error(), "filename does not match profile name") {
		t.Fatalf("Get(mismatched filename) err=%v, want filename mismatch", err)
	}
	if err := os.Remove(filepath.Join(dir, "relay-b.yaml")); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "relay-invalid.yaml"), []byte("name: relay-invalid\nbase-url: https://relay.example/v1\napi-key: sk-invalid\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := repo.List(); err == nil || !strings.Contains(err.Error(), "created-at is required") {
		t.Fatalf("List(invalid metadata) err=%v, want created-at validation error", err)
	}

	statePath := filepath.Join(t.TempDir(), "state.yaml")
	stateRepo := NewStateRepository(statePath)
	state := domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-a", Status: domainprofile.BindingStatusApplied},
		},
	}
	if _, err := stateRepo.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := stateRepo.Load()
	if err != nil {
		t.Fatalf("Load(saved) error = %v", err)
	}
	if loaded.Codex.SourceProfile != "relay-a" || loaded.Codex.Status != domainprofile.BindingStatusApplied {
		t.Fatalf("Load(saved) = %+v, want persisted state", loaded.Codex)
	}
}

func TestRepositoryValidationAndLockBranches(t *testing.T) {
	repo, err := NewRepository(filepath.Join(t.TempDir(), "profiles"))
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	if _, err := repo.Upsert(domainprofile.Profile{Name: "bad name", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err == nil {
		t.Fatal("Upsert(invalid name) unexpectedly succeeded")
	}
	if _, err := repo.Upsert(domainprofile.Profile{Name: "relay-a", BaseURL: "ftp://bad", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err == nil {
		t.Fatal("Upsert(invalid url) unexpectedly succeeded")
	}
	if err := repo.Delete("bad name"); err == nil {
		t.Fatal("Delete(invalid name) unexpectedly succeeded")
	}
	if _, err := repo.Get("bad name"); err == nil {
		t.Fatal("Get(invalid name) unexpectedly succeeded")
	}

	unlock, err := repo.acquireFileLock()
	if err != nil {
		t.Fatalf("acquireFileLock() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(repo.lockPath), ".profiles.lock")); err != nil {
		t.Fatalf("lock file missing, err = %v", err)
	}
	unlock()

	stateRepo := NewStateRepository(filepath.Join(t.TempDir(), "state.yaml"))
	unlockState, err := stateRepo.acquireFileLock()
	if err != nil {
		t.Fatalf("state acquireFileLock() error = %v", err)
	}
	if _, err := os.Stat(stateRepo.lockPath); err != nil {
		t.Fatalf("state lock file missing, err = %v", err)
	}
	unlockState()
}

func TestRepositoryLoadOneAndSaveOneAdditionalBranches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	repo, err := NewRepository(dir)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	body := "name: relay-a\nbase-url: ftp://relay.example\napi-key: sk-a\ncreated-at: 2026-04-28T00:00:00Z\nupdated-at: 2026-04-28T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "relay-a.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := repo.Get("relay-a"); err == nil || !strings.Contains(err.Error(), "base url must start with http:// or https://") {
		t.Fatalf("Get(invalid base url file) err=%v, want base url validation error", err)
	}

	body = "name: relay-b\nbase-url: https://relay.example/v1\napi-key: \ncreated-at: 2026-04-28T00:00:00Z\nupdated-at: 2026-04-28T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "relay-b.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := repo.Get("relay-b"); err == nil || !strings.Contains(err.Error(), "relay credential is required") {
		t.Fatalf("Get(invalid api key file) err=%v, want relay credential validation error", err)
	}

	profile := domainprofile.Profile{
		Name:      "relay-save",
		BaseURL:   "https://relay.example/v1",
		APIKey:    "sk-a",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.saveOne(profile); err != nil {
		t.Fatalf("saveOne() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "relay-save.yaml")); err != nil {
		t.Fatalf("saved profile missing, err = %v", err)
	}
}

func TestStateRepositoryLoadInvalidYAMLAndDirectorySaveFailure(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.yaml")
	repo := NewStateRepository(statePath)

	if err := os.WriteFile(statePath, []byte("codex: [\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := repo.Load(); err == nil {
		t.Fatal("Load(invalid yaml) unexpectedly succeeded")
	}

	badRepo := NewStateRepository(filepath.Join(statePath, "child", "state.yaml"))
	if err := os.WriteFile(statePath, []byte("blocker"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	if _, err := badRepo.Save(domainprofile.State{}); err == nil {
		t.Fatal("Save() unexpectedly succeeded when parent path is a file")
	}
}

func TestRepositoryAdditionalErrorBranches(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "profiles")
	repo, err := NewRepository(dir)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	if _, err := repo.Upsert(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: ""}); err == nil {
		t.Fatal("Upsert(empty api key) unexpectedly succeeded")
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.yaml"), []byte("ignored"), 0o600); err != nil {
		t.Fatalf("WriteFile(hidden) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o700); err != nil {
		t.Fatalf("MkdirAll(subdir) error = %v", err)
	}
	listed, err := repo.List()
	if err != nil {
		t.Fatalf("List(ignored entries) error = %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("List(ignored entries) = %+v, want empty", listed)
	}

	if err := os.WriteFile(filepath.Join(dir, "relay-b.yaml"), []byte("name: relay-b\nbase-url: https://relay.example/v1\napi-key: sk-b\ncreated-at: 2026-04-28T00:00:00Z\nupdated-at: 2026-04-28T00:00:00Z\nextra: true\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(extra field) error = %v", err)
	}
	if _, err := repo.Get("relay-b"); err == nil {
		t.Fatal("Get(unknown yaml field) unexpectedly succeeded")
	}
}

func TestRepositoryAndStateRepositoryFileSystemFailures(t *testing.T) {
	base := t.TempDir()
	var err error

	repo := &Repository{
		dir:      filepath.Join(base, "missing", "profiles"),
		lockPath: filepath.Join(base, "blocking-file", ".profiles.lock"),
	}
	if err := os.WriteFile(filepath.Join(base, "blocking-file"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(lock blocker) error = %v", err)
	}
	if _, err := repo.Upsert(domainprofile.Profile{
		Name:      "relay-a",
		BaseURL:   "https://relay.example/v1",
		APIKey:    "sk-a",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err == nil {
		t.Fatal("Upsert() unexpectedly succeeded with invalid lock path parent")
	}

	dirBlocker := filepath.Join(base, "dir-blocker")
	if err := os.WriteFile(dirBlocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(dir blocker) error = %v", err)
	}
	repo = &Repository{
		dir:      filepath.Join(dirBlocker, "profiles"),
		lockPath: filepath.Join(dirBlocker, "profiles", ".profiles.lock"),
	}
	if _, err := repo.acquireFileLock(); err == nil {
		t.Fatal("acquireFileLock() unexpectedly succeeded with file parent")
	}
	if _, err := repo.List(); err == nil {
		t.Fatal("List() unexpectedly succeeded with file parent")
	}

	profilesDir := filepath.Join(base, "profiles-ok")
	repo, err = NewRepository(profilesDir)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	if err := os.MkdirAll(profilesDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	profilePath := filepath.Join(profilesDir, "relay-a.yaml")
	if err := os.WriteFile(profilePath, []byte("name: bad name\nbase-url: https://relay.example/v1\napi-key: sk-a\ncreated-at: 2026-04-28T00:00:00Z\nupdated-at: 2026-04-28T00:00:00Z\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := repo.Get("relay-a"); err == nil || !strings.Contains(err.Error(), "cannot contain whitespace") {
		t.Fatalf("Get(invalid normalized name) err=%v, want whitespace validation error", err)
	}
	if err := os.WriteFile(profilePath, []byte("name: relay-a\nbase-url: https://relay.example/v1\napi-key: sk-a\ncreated-at: 2026-04-28T00:00:00Z\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(updated-at missing) error = %v", err)
	}
	if _, err := repo.Get("relay-a"); err == nil || !strings.Contains(err.Error(), "updated-at is required") {
		t.Fatalf("Get(updated-at missing) err=%v, want updated-at required", err)
	}

	blockingDir := filepath.Join(base, "state-blocker")
	if err := os.WriteFile(blockingDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(state blocker) error = %v", err)
	}
	stateRepo := NewStateRepository(filepath.Join(blockingDir, "state.yaml"))
	if _, err := stateRepo.acquireFileLock(); err == nil {
		t.Fatal("state acquireFileLock() unexpectedly succeeded with file parent")
	}
	if _, err := stateRepo.Load(); err == nil {
		t.Fatal("Load() unexpectedly succeeded with file parent")
	}
}
