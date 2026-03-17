package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiddingbaby/agx/internal/adapters/configfile"
	"github.com/kiddingbaby/agx/internal/adapters/keyfile"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func newSiteRoot(t *testing.T) (*Root, *bytes.Buffer, *bytes.Buffer) {
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

func TestSiteLsShowsOfficialAliases(t *testing.T) {
	root, stdout, stderr := newSiteRoot(t)

	if code := root.Execute([]string{"get", "sites"}); code != 0 {
		t.Fatalf("get sites code=%d want 0 stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "  openai  family=openai") {
		t.Fatalf("stdout missing openai alias: %q", got)
	}
	if got := stdout.String(); !strings.Contains(got, "  claude  family=claude") {
		t.Fatalf("stdout missing claude alias: %q", got)
	}
	if got := stdout.String(); !strings.Contains(got, "  gemini  family=gemini") {
		t.Fatalf("stdout missing gemini alias: %q", got)
	}
}

func TestSiteAddCreatesOpenRouter(t *testing.T) {
	root, _, stderr := newSiteRoot(t)

	if code := root.Execute([]string{"create", "site", "openrouter", "--template", "openrouter", "--model", "", "--no-keys"}); code != 0 {
		t.Fatalf("create site code=%d want 0 stderr=%q", code, stderr.String())
	}

	target, err := root.providerSvc.GetTarget("openrouter")
	if err != nil || target == nil {
		t.Fatalf("GetTarget(openrouter) err=%v target=%v", err, target)
	}
	if target.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("BaseURL=%q want openrouter default", target.BaseURL)
	}
}

func TestAddAddsKeyToSiteScope(t *testing.T) {
	root, _, stderr := newSiteRoot(t)

	if code := root.Execute([]string{"create", "site", "openrouter", "--template", "openrouter", "--model", "", "--no-keys"}); code != 0 {
		t.Fatalf("create site code=%d want 0 stderr=%q", code, stderr.String())
	}
	if code := root.Execute([]string{"create", "key", "k1", "--site", "openrouter", "--api-key", "sk-1", "--activate"}); code != 0 {
		t.Fatalf("create key code=%d want 0 stderr=%q", code, stderr.String())
	}

	active, err := root.keySvc.GetActive(domainkey.ProviderOpenAI, "openrouter")
	if err != nil {
		t.Fatalf("GetActive(openai/openrouter) error=%v", err)
	}
	if active.Name != "k1" {
		t.Fatalf("active.Name=%q want k1", active.Name)
	}
}

func TestAddAddsKeyToOfficialScope(t *testing.T) {
	root, _, stderr := newSiteRoot(t)

	if code := root.Execute([]string{"create", "key", "oai1", "--site", "openai", "--api-key", "sk-1", "--activate"}); code != 0 {
		t.Fatalf("create key(openai) code=%d want 0 stderr=%q", code, stderr.String())
	}

	active, err := root.keySvc.GetActive(domainkey.ProviderOpenAI, domainkey.DefaultProfile)
	if err != nil {
		t.Fatalf("GetActive(openai/default) error=%v", err)
	}
	if active.Name != "oai1" {
		t.Fatalf("active.Name=%q want oai1", active.Name)
	}
}

func TestSiteKeyPickActivatesSelection(t *testing.T) {
	root, _, stderr := newSiteRoot(t)

	if _, err := root.keySvc.Add(domainkey.ProviderOpenAI, "default", "k1", "sk-1", "", nil); err != nil {
		t.Fatalf("Add(k1) error=%v", err)
	}
	if _, err := root.keySvc.Add(domainkey.ProviderOpenAI, "default", "k2", "sk-2", "", nil); err != nil {
		t.Fatalf("Add(k2) error=%v", err)
	}

	if code := root.Execute([]string{"patch", "key", "k2", "--site", "openai", "--activate"}); code != 0 {
		t.Fatalf("patch key --activate code=%d want 0 stderr=%q", code, stderr.String())
	}

	active, err := root.keySvc.GetActive(domainkey.ProviderOpenAI, "default")
	if err != nil {
		t.Fatalf("GetActive() error=%v", err)
	}
	if active.Name != "k2" {
		t.Fatalf("active.Name=%q want k2", active.Name)
	}
}

func TestSiteEditOverridesAndResetOfficial(t *testing.T) {
	root, _, stderr := newSiteRoot(t)

	if code := root.Execute([]string{"patch", "site", "openai", "--model", "gpt-4.1"}); code != 0 {
		t.Fatalf("patch site openai code=%d want 0 stderr=%q", code, stderr.String())
	}

	target, err := root.providerSvc.GetTarget("openai-official")
	if err != nil || target == nil {
		t.Fatalf("GetTarget(openai-official) err=%v target=%v", err, target)
	}
	if target.Model != "gpt-4.1" {
		t.Fatalf("Model=%q want gpt-4.1", target.Model)
	}

	if code := root.Execute([]string{"patch", "site", "openai", "--reset"}); code != 0 {
		t.Fatalf("patch site openai --reset code=%d want 0 stderr=%q", code, stderr.String())
	}

	target, err = root.providerSvc.GetTarget("openai-official")
	if err != nil || target == nil {
		t.Fatalf("GetTarget(openai-official after reset) err=%v target=%v", err, target)
	}
	if target.Model != "" {
		t.Fatalf("Model(after reset)=%q want empty", target.Model)
	}
}
