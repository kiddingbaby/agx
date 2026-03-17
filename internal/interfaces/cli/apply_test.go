package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiddingbaby/agx/internal/adapters/configfile"
	"github.com/kiddingbaby/agx/internal/adapters/keyfile"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func newApplyRoot(t *testing.T) (*Root, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	tmp := t.TempDir()
	secret := []byte("12345678901234567890123456789012")
	keyRepo, err := keyfile.NewRepository(filepath.Join(tmp, "keys.yaml"), secret)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	keySvc := usecase.NewKeyService(keyRepo)
	providerRepo, err := configfile.NewProviderRegistry(filepath.Join(tmp, "providers.yaml"))
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}
	providerSvc := usecase.NewProviderService(providerRepo)

	root := New(keySvc, providerSvc, nil, nil)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr
	return root, stdout, stderr
}

func TestApplyBundleCreatesAndUpdates(t *testing.T) {
	root, stdout, stderr := newApplyRoot(t)

	t.Setenv("AGX_TEST_OPENAI_1", "sk-openai-1")
	bundle1 := `
keys:
  - provider: openai
    name: oai-01
    key-env: AGX_TEST_OPENAI_1
    tags: [work, primary]
    base-url: "https://legacy.proxy.local"
    activate: true
targets:
  - name: openrouter
    family: openai
    kind: openai-compatible
    access: third_party
    base-url: "https://openrouter.ai/api/v1"
bindings:
  - family: openai
    target: openrouter
profiles:
  - provider: openai
    profile: default
    strategy: round_robin
`
	path := filepath.Join(t.TempDir(), "bundle.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(bundle1)+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile(bundle) error = %v", err)
	}

	if code := root.Execute([]string{"apply", path}); code != 0 {
		t.Fatalf("apply code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Applied:") {
		t.Fatalf("stdout = %q, want summary", stdout.String())
	}

	key, err := root.keySvc.FindByIdentifierInScope(domainkey.ProviderOpenAI, "default", "oai-01")
	if err != nil || key == nil {
		t.Fatalf("FindByIdentifierInScope() err=%v key=%v", err, key)
	}

	active, err := root.keySvc.GetActive(domainkey.ProviderOpenAI, "default")
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.Name != "oai-01" {
		t.Fatalf("active.Name = %q, want oai-01", active.Name)
	}

	target, err := root.providerSvc.GetTarget("openrouter")
	if err != nil {
		t.Fatalf("GetTarget(openrouter) error = %v", err)
	}
	if target.Family != domainprovider.FamilyOpenAI || target.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("target = %+v, want openai/openrouter base_url", target)
	}

	bindings := root.providerSvc.ListBindings()
	found := false
	for _, b := range bindings {
		if b.Family == domainprovider.FamilyOpenAI && b.Target == "openrouter" {
			found = true
		}
	}
	if !found {
		t.Fatalf("bindings missing openai -> openrouter: %+v", bindings)
	}

	// Apply again: update tags only, base_url omitted -> should keep existing base_url.
	bundle2 := `
keys:
  - provider: openai
    name: oai-01
    tags: [work, rotated]
`
	path2 := filepath.Join(t.TempDir(), "bundle2.yaml")
	if err := os.WriteFile(path2, []byte(strings.TrimSpace(bundle2)+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile(bundle2) error = %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := root.Execute([]string{"apply", path2, "-o", "json"}); code != 0 {
		t.Fatalf("apply2 code = %d, want 0; stderr=%q", code, stderr.String())
	}

	updated, err := root.keySvc.FindByIdentifierInScope(domainkey.ProviderOpenAI, "default", "oai-01")
	if err != nil || updated == nil {
		t.Fatalf("FindByIdentifierInScope(after) err=%v key=%v", err, updated)
	}
	if updated.BaseURL != "https://legacy.proxy.local" {
		t.Fatalf("updated.BaseURL = %q, want legacy proxy unchanged", updated.BaseURL)
	}
	if len(updated.Tags) != 2 || updated.Tags[1] != "rotated" {
		t.Fatalf("updated.Tags = %+v, want [work rotated]", updated.Tags)
	}
}
