package configfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

func TestProviderRegistryDefaults(t *testing.T) {
	repo, err := NewProviderRegistry(filepath.Join(t.TempDir(), "providers.yaml"))
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}

	targets := repo.ListTargets()
	if len(targets) != 3 {
		t.Fatalf("len(ListTargets()) = %d, want 3", len(targets))
	}

	binding, err := repo.GetBinding(domainprovider.FamilyOpenAI)
	if err != nil {
		t.Fatalf("GetBinding() error = %v", err)
	}
	if binding.Target != domainprovider.DefaultTargetName(domainprovider.FamilyOpenAI) {
		t.Fatalf("binding.Target = %q, want %q", binding.Target, domainprovider.DefaultTargetName(domainprovider.FamilyOpenAI))
	}
}

func TestProviderRegistryCustomTargetAndBinding(t *testing.T) {
	repo, err := NewProviderRegistry(filepath.Join(t.TempDir(), "providers.yaml"))
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}

	target, err := repo.UpsertTarget(domainprovider.Target{
		Name:    "openrouter",
		Family:  domainprovider.FamilyOpenAI,
		Kind:    domainprovider.KindOpenAICompatible,
		Access:  domainprovider.AccessThirdParty,
		Auth:    domainprovider.AuthAPIKey,
		BaseURL: "https://openrouter.ai/api/v1",
	})
	if err != nil {
		t.Fatalf("UpsertTarget() error = %v", err)
	}
	if target.Name != "openrouter" {
		t.Fatalf("target.Name = %q, want openrouter", target.Name)
	}

	binding, err := repo.SetBinding(domainprovider.FamilyOpenAI, "openrouter")
	if err != nil {
		t.Fatalf("SetBinding() error = %v", err)
	}
	if binding.Target != "openrouter" {
		t.Fatalf("binding.Target = %q, want openrouter", binding.Target)
	}

	got, err := repo.GetTarget("openrouter")
	if err != nil {
		t.Fatalf("GetTarget() error = %v", err)
	}
	if got.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("GetTarget().BaseURL = %q, want openrouter URL", got.BaseURL)
	}
}

func TestProviderRegistryRejectsDeletingBoundTarget(t *testing.T) {
	repo, err := NewProviderRegistry(filepath.Join(t.TempDir(), "providers.yaml"))
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}

	_, err = repo.UpsertTarget(domainprovider.Target{
		Name:    "claude-proxy",
		Family:  domainprovider.FamilyClaude,
		Kind:    domainprovider.KindClaude,
		Access:  domainprovider.AccessThirdParty,
		Auth:    domainprovider.AuthAPIKey,
		BaseURL: "https://claude-proxy.local",
	})
	if err != nil {
		t.Fatalf("UpsertTarget() error = %v", err)
	}
	if _, err := repo.SetBinding(domainprovider.FamilyClaude, "claude-proxy"); err != nil {
		t.Fatalf("SetBinding() error = %v", err)
	}
	if err := repo.DeleteTarget("claude-proxy"); err == nil {
		t.Fatal("DeleteTarget(bound) = nil, want error")
	}
}

func TestProviderRegistryCurrentSitePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.yaml")
	repo, err := NewProviderRegistry(path)
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}

	if err := repo.SetCurrentSite(domainprovider.DefaultTargetName(domainprovider.FamilyOpenAI)); err != nil {
		t.Fatalf("SetCurrentSite() error = %v", err)
	}
	if got := repo.GetCurrentSite(); got != domainprovider.DefaultTargetName(domainprovider.FamilyOpenAI) {
		t.Fatalf("GetCurrentSite() = %q, want %q", got, domainprovider.DefaultTargetName(domainprovider.FamilyOpenAI))
	}

	repo2, err := NewProviderRegistry(path)
	if err != nil {
		t.Fatalf("NewProviderRegistry(second) error = %v", err)
	}
	if got := repo2.GetCurrentSite(); got != domainprovider.DefaultTargetName(domainprovider.FamilyOpenAI) {
		t.Fatalf("GetCurrentSite(second) = %q, want %q", got, domainprovider.DefaultTargetName(domainprovider.FamilyOpenAI))
	}
}

func TestProviderRegistryCurrentSiteRejectsUnknownTarget(t *testing.T) {
	repo, err := NewProviderRegistry(filepath.Join(t.TempDir(), "providers.yaml"))
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}
	if err := repo.SetCurrentSite("does-not-exist"); err == nil {
		t.Fatal("SetCurrentSite(unknown) = nil, want error")
	}
}

func TestProviderRegistryInvalidCurrentSiteIsIgnoredOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.yaml")
	raw := `
current-site: "does-not-exist"
targets: []
bindings: []
`
	if err := os.WriteFile(path, []byte(strings.TrimSpace(raw)+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	repo, err := NewProviderRegistry(path)
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}
	if got := repo.GetCurrentSite(); got != "" {
		t.Fatalf("GetCurrentSite() = %q, want empty", got)
	}
}
