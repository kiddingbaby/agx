package usecase

import (
	"errors"
	"testing"

	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

type fakeProviderRepo struct {
	targets     []domainprovider.Target
	bindings    []domainprovider.Binding
	currentSite string
}

func (f *fakeProviderRepo) ListTargets() []domainprovider.Target {
	out := append([]domainprovider.Target{}, domainprovider.DefaultTargets()...)
	out = append(out, f.targets...)
	return out
}

func (f *fakeProviderRepo) GetTarget(name string) (*domainprovider.Target, error) {
	for _, target := range f.ListTargets() {
		if target.Name == name {
			copyTarget := target
			return &copyTarget, nil
		}
	}
	return nil, &domainprovider.TargetNotFoundError{Name: name}
}

func (f *fakeProviderRepo) UpsertTarget(target domainprovider.Target) (*domainprovider.Target, error) {
	for i := range f.targets {
		if f.targets[i].Name == target.Name {
			f.targets[i] = target
			return &f.targets[i], nil
		}
	}
	f.targets = append(f.targets, target)
	return &f.targets[len(f.targets)-1], nil
}

func (f *fakeProviderRepo) DeleteTarget(name string) error {
	for i := range f.targets {
		if f.targets[i].Name == name {
			f.targets = append(f.targets[:i], f.targets[i+1:]...)
			return nil
		}
	}
	return &domainprovider.TargetNotFoundError{Name: name}
}

func (f *fakeProviderRepo) ListBindings() []domainprovider.Binding {
	if len(f.bindings) == 0 {
		return []domainprovider.Binding{
			{Family: domainprovider.FamilyClaude, Target: domainprovider.DefaultTargetName(domainprovider.FamilyClaude)},
			{Family: domainprovider.FamilyOpenAI, Target: domainprovider.DefaultTargetName(domainprovider.FamilyOpenAI)},
			{Family: domainprovider.FamilyGemini, Target: domainprovider.DefaultTargetName(domainprovider.FamilyGemini)},
		}
	}
	out := make([]domainprovider.Binding, len(f.bindings))
	copy(out, f.bindings)
	return out
}

func (f *fakeProviderRepo) GetBinding(family domainprovider.Family) (*domainprovider.Binding, error) {
	for _, binding := range f.ListBindings() {
		if binding.Family == family {
			copyBinding := binding
			return &copyBinding, nil
		}
	}
	return nil, errors.New("binding not found")
}

func (f *fakeProviderRepo) SetBinding(family domainprovider.Family, target string) (*domainprovider.Binding, error) {
	for i := range f.bindings {
		if f.bindings[i].Family == family {
			f.bindings[i].Target = target
			return &f.bindings[i], nil
		}
	}
	f.bindings = append(f.bindings, domainprovider.Binding{Family: family, Target: target})
	return &f.bindings[len(f.bindings)-1], nil
}

func (f *fakeProviderRepo) GetCurrentSite() string {
	return f.currentSite
}

func (f *fakeProviderRepo) SetCurrentSite(targetName string) error {
	f.currentSite = targetName
	return nil
}

func TestProviderServiceSaveAndUseTarget(t *testing.T) {
	repo := &fakeProviderRepo{}
	svc := NewProviderService(repo)

	target, err := svc.SaveTarget(SaveTargetInput{
		Name:    "openrouter",
		Family:  "openai",
		Kind:    "openai-compatible",
		Access:  "third_party",
		Auth:    "apikey",
		BaseURL: "https://openrouter.ai/api/v1",
	})
	if err != nil {
		t.Fatalf("SaveTarget() error = %v", err)
	}
	if target.Name != "openrouter" {
		t.Fatalf("target.Name = %q, want openrouter", target.Name)
	}

	binding, err := svc.UseTarget("openai", "openrouter")
	if err != nil {
		t.Fatalf("UseTarget() error = %v", err)
	}
	if binding.Target != "openrouter" {
		t.Fatalf("binding.Target = %q, want openrouter", binding.Target)
	}
}

func TestProviderServiceResolveTargetDefaults(t *testing.T) {
	svc := NewProviderService(&fakeProviderRepo{})

	target, err := svc.ResolveTarget(domainprovider.FamilyClaude, "")
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	if target.Name != "claude-official" {
		t.Fatalf("target.Name = %q, want claude-official", target.Name)
	}
}

func TestProviderServiceNormalizesThirdPartyBaseURL(t *testing.T) {
	repo := &fakeProviderRepo{}
	svc := NewProviderService(repo)

	openaiTarget, err := svc.SaveTarget(SaveTargetInput{
		Name:    "gw",
		Family:  "openai",
		Kind:    "openai-compatible",
		Access:  "third_party",
		Auth:    "apikey",
		BaseURL: "https://gateway.example.com/",
	})
	if err != nil {
		t.Fatalf("SaveTarget(openai) error = %v", err)
	}
	if openaiTarget.BaseURL != "https://gateway.example.com/v1" {
		t.Fatalf("openai BaseURL=%q want https://gateway.example.com/v1", openaiTarget.BaseURL)
	}

	claudeTarget, err := svc.SaveTarget(SaveTargetInput{
		Name:    "gw-claude",
		Family:  "claude",
		Kind:    "claude",
		Access:  "third_party",
		Auth:    "apikey",
		BaseURL: "https://gateway.example.com/v1",
	})
	if err != nil {
		t.Fatalf("SaveTarget(claude) error = %v", err)
	}
	if claudeTarget.BaseURL != "https://gateway.example.com" {
		t.Fatalf("claude BaseURL=%q want https://gateway.example.com", claudeTarget.BaseURL)
	}
}

func TestProviderServiceKeyScopeSharesKeysForUniversalGateway(t *testing.T) {
	repo := &fakeProviderRepo{
		targets: []domainprovider.Target{
			{
				Name:    "gw-codex",
				Family:  domainprovider.FamilyOpenAI,
				Kind:    domainprovider.KindOpenAICompatible,
				Access:  domainprovider.AccessThirdParty,
				Auth:    domainprovider.AuthAPIKey,
				BaseURL: "https://gateway.example.com/v1",
			},
			{
				Name:    "gw-claude",
				Family:  domainprovider.FamilyClaude,
				Kind:    domainprovider.KindClaude,
				Access:  domainprovider.AccessThirdParty,
				Auth:    domainprovider.AuthAPIKey,
				BaseURL: "https://gateway.example.com",
			},
		},
	}
	svc := NewProviderService(repo)

	openai, err := svc.GetTarget("gw-codex")
	if err != nil || openai == nil {
		t.Fatalf("GetTarget(gw-codex) err=%v target=%v", err, openai)
	}
	provider, profile, err := svc.KeyScopeForTarget(*openai)
	if err != nil {
		t.Fatalf("KeyScopeForTarget(gw-codex) error = %v", err)
	}
	if provider != "openai" || profile != "gw" {
		t.Fatalf("KeyScopeForTarget(gw-codex)=%s/%s want openai/gw", provider, profile)
	}

	claude, err := svc.GetTarget("gw-claude")
	if err != nil || claude == nil {
		t.Fatalf("GetTarget(gw-claude) err=%v target=%v", err, claude)
	}
	provider, profile, err = svc.KeyScopeForTarget(*claude)
	if err != nil {
		t.Fatalf("KeyScopeForTarget(gw-claude) error = %v", err)
	}
	if provider != "openai" || profile != "gw" {
		t.Fatalf("KeyScopeForTarget(gw-claude)=%s/%s want openai/gw", provider, profile)
	}
}
