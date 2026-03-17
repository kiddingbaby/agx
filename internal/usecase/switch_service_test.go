package usecase

import (
	"path/filepath"
	"testing"

	"github.com/kiddingbaby/agx/internal/adapters/keyfile"
	domainagent "github.com/kiddingbaby/agx/internal/domain/agent"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestFamilyProviderAgentMappingsConsistent(t *testing.T) {
	for _, family := range domainprovider.SupportedFamilies() {
		if _, ok := domainkey.ParseProvider(string(family)); !ok {
			t.Fatalf("missing key provider for family %s", family)
		}
		if _, ok := agentForFamily(family); !ok {
			t.Fatalf("missing agent mapping for family %s", family)
		}
	}
}

type recordingSyncer struct {
	keys []domainkey.Key
}

func (s *recordingSyncer) Apply(_ domainagent.Agent, key domainkey.Key, _ domainprovider.Target) error {
	s.keys = append(s.keys, key)
	return nil
}

type noopUndoStore struct{}

func (s *noopUndoStore) Capture(_ ports.UndoCaptureMeta) (string, error) {
	return "noop", nil
}

func (s *noopUndoStore) LatestID() (string, error) {
	return "noop", nil
}

func (s *noopUndoStore) Restore(id string) (ports.UndoRestoreResult, error) {
	return ports.UndoRestoreResult{ID: id}, nil
}

func TestSwitchServiceRotatesKeysWhenStrategyRoundRobin(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("0123456789abcdef0123456789abcdef")
	repo, err := keyfile.NewRepository(filepath.Join(dir, "keys.yaml"), secret)
	if err != nil {
		t.Fatalf("keyfile.NewRepository() error = %v", err)
	}

	keySvc := NewKeyService(repo)
	k1, err := keySvc.Add(domainkey.ProviderOpenAI, "openrouter", "k1", "sk-1", "", nil)
	if err != nil {
		t.Fatalf("Add(k1) error = %v", err)
	}
	k2, err := keySvc.Add(domainkey.ProviderOpenAI, "openrouter", "k2", "sk-2", "", nil)
	if err != nil {
		t.Fatalf("Add(k2) error = %v", err)
	}
	if err := keySvc.SetProfileStrategy(domainkey.ProviderOpenAI, "openrouter", domainkey.StrategyRoundRobin, ""); err != nil {
		t.Fatalf("SetProfileStrategy(round_robin) error = %v", err)
	}

	providerRepo := &fakeProviderRepo{
		targets: []domainprovider.Target{
			{
				Name:    "openrouter",
				Family:  domainprovider.FamilyOpenAI,
				Kind:    domainprovider.KindOpenAICompatible,
				Access:  domainprovider.AccessThirdParty,
				Auth:    domainprovider.AuthAPIKey,
				BaseURL: "https://openrouter.ai/api/v1",
			},
		},
	}
	providerSvc := NewProviderService(providerRepo)

	syncer := &recordingSyncer{}
	switchSvc := NewSwitchService(keySvc, providerSvc, syncer, &noopUndoStore{})

	got1, err := switchSvc.SwitchByName("openrouter", SwitchOptions{})
	if err != nil {
		t.Fatalf("SwitchByName(1) error = %v", err)
	}
	got2, err := switchSvc.SwitchByName("openrouter", SwitchOptions{})
	if err != nil {
		t.Fatalf("SwitchByName(2) error = %v", err)
	}
	got3, err := switchSvc.SwitchByName("openrouter", SwitchOptions{})
	if err != nil {
		t.Fatalf("SwitchByName(3) error = %v", err)
	}

	if got1.Primary.Key.ID != k1.ID {
		t.Fatalf("SwitchByName(1) key = %s, want %s", got1.Primary.Key.ID, k1.ID)
	}
	if got2.Primary.Key.ID != k2.ID {
		t.Fatalf("SwitchByName(2) key = %s, want %s", got2.Primary.Key.ID, k2.ID)
	}
	if got3.Primary.Key.ID != k1.ID {
		t.Fatalf("SwitchByName(3) key = %s, want %s", got3.Primary.Key.ID, k1.ID)
	}
	if len(syncer.keys) != 3 {
		t.Fatalf("syncer applied keys = %d, want 3", len(syncer.keys))
	}
}
