package usecase

import (
	"errors"
	"strings"
	"testing"
	"time"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
)

type fakeKeyRepo struct {
	keys     []domainkey.Key
	profiles []domainkey.Profile
}

func (f *fakeKeyRepo) List() []domainkey.Key {
	out := make([]domainkey.Key, len(f.keys))
	copy(out, f.keys)
	return out
}

func (f *fakeKeyRepo) Add(provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	k := domainkey.Key{
		ID:        "new-id",
		Provider:  provider,
		Profile:   domainkey.NormalizeProfileName(profile),
		Name:      name,
		APIKey:    apiKey,
		BaseURL:   baseURL,
		Tags:      tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.keys = append(f.keys, k)
	return &k, nil
}

func (f *fakeKeyRepo) Update(id string, provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	for i := range f.keys {
		if f.keys[i].ID != id {
			continue
		}
		f.keys[i].Provider = provider
		f.keys[i].Profile = domainkey.NormalizeProfileName(profile)
		f.keys[i].Name = name
		if apiKey != "" {
			f.keys[i].APIKey = apiKey
		}
		f.keys[i].BaseURL = baseURL
		f.keys[i].Tags = tags
		f.keys[i].UpdatedAt = time.Now()
		return &f.keys[i], nil
	}
	return nil, errors.New("key not found")
}

func (f *fakeKeyRepo) Delete(id string) error {
	for i := range f.keys {
		if f.keys[i].ID == id {
			f.keys = append(f.keys[:i], f.keys[i+1:]...)
			return nil
		}
	}
	return errors.New("key not found")
}

func (f *fakeKeyRepo) Activate(id string) error {
	var provider domainkey.Provider
	var profile string
	found := false
	for i := range f.keys {
		if f.keys[i].ID == id {
			found = true
			provider = f.keys[i].Provider
			profile = domainkey.NormalizeProfileName(f.keys[i].Profile)
			f.keys[i].Active = true
		}
	}
	if !found {
		return errors.New("key not found")
	}
	for i := range f.keys {
		if f.keys[i].Provider == provider && domainkey.NormalizeProfileName(f.keys[i].Profile) == profile && f.keys[i].ID != id {
			f.keys[i].Active = false
		}
	}
	return nil
}

func (f *fakeKeyRepo) GetActive(provider domainkey.Provider, profile string) (*domainkey.Key, error) {
	profile = domainkey.NormalizeProfileName(profile)
	for i := range f.keys {
		if f.keys[i].Provider == provider && domainkey.NormalizeProfileName(f.keys[i].Profile) == profile && f.keys[i].Active {
			out := f.keys[i]
			return &out, nil
		}
	}
	return nil, errors.New("no active key")
}

func (f *fakeKeyRepo) HasActive(provider domainkey.Provider, profile string) bool {
	profile = domainkey.NormalizeProfileName(profile)
	for i := range f.keys {
		if f.keys[i].Provider == provider && domainkey.NormalizeProfileName(f.keys[i].Profile) == profile && f.keys[i].Active {
			return true
		}
	}
	return false
}

func (f *fakeKeyRepo) Resolve(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	profile = domainkey.NormalizeProfileName(profile)
	var exact *domainkey.Key
	for i := range f.keys {
		if f.keys[i].Provider != provider || domainkey.NormalizeProfileName(f.keys[i].Profile) != profile {
			continue
		}
		if identifier == "" {
			out := f.keys[i]
			return &out, nil
		}
		if f.keys[i].Name == identifier {
			if exact != nil {
				return nil, &AmbiguousKeyIdentifierError{Identifier: identifier}
			}
			out := f.keys[i]
			exact = &out
		}
	}
	if exact != nil {
		return exact, nil
	}

	var prefix *domainkey.Key
	for i := range f.keys {
		if f.keys[i].Provider != provider || domainkey.NormalizeProfileName(f.keys[i].Profile) != profile {
			continue
		}
		if !strings.HasPrefix(f.keys[i].ID, identifier) {
			continue
		}
		if prefix != nil {
			return nil, &AmbiguousKeyIdentifierError{Identifier: identifier}
		}
		out := f.keys[i]
		prefix = &out
	}
	if prefix != nil {
		return prefix, nil
	}
	return nil, &KeyNotFoundError{Identifier: identifier}
}

func (f *fakeKeyRepo) ListProfiles(provider domainkey.Provider) []domainkey.Profile {
	out := make([]domainkey.Profile, 0)
	for _, p := range f.profiles {
		if p.Provider == provider {
			out = append(out, p)
		}
	}
	return out
}

func (f *fakeKeyRepo) SetProfileStrategy(provider domainkey.Provider, profile string, strategy domainkey.RotationStrategy, fixedKey string) error {
	profile = domainkey.NormalizeProfileName(profile)
	for i := range f.profiles {
		if f.profiles[i].Provider == provider && f.profiles[i].Name == profile {
			f.profiles[i].Strategy = strategy
			f.profiles[i].FixedKey = fixedKey
			return nil
		}
	}
	f.profiles = append(f.profiles, domainkey.Profile{
		Provider: provider,
		Name:     profile,
		Strategy: strategy,
		FixedKey: fixedKey,
	})
	return nil
}

func TestFindByIdentifier(t *testing.T) {
	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{ID: "1111-aaaa", Name: "claude-main", Provider: domainkey.ProviderClaude},
			{ID: "2222-bbbb", Name: "openai-main", Provider: domainkey.ProviderOpenAI},
		},
	}
	svc := NewKeyService(repo)

	k, err := svc.FindByIdentifier("claude-main")
	if err != nil {
		t.Fatalf("FindByIdentifier(name) error = %v", err)
	}
	if k.ID != "1111-aaaa" {
		t.Fatalf("FindByIdentifier(name) id = %s", k.ID)
	}

	k, err = svc.FindByIdentifier("2222")
	if err != nil {
		t.Fatalf("FindByIdentifier(prefix) error = %v", err)
	}
	if k.Name != "openai-main" {
		t.Fatalf("FindByIdentifier(prefix) name = %s", k.Name)
	}

	if _, err := svc.FindByIdentifier("missing"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("FindByIdentifier(missing) err = %v, want ErrKeyNotFound", err)
	}
	if _, err := svc.FindByIdentifier("missing"); !IsKeyNotFoundError(err) {
		t.Fatalf("FindByIdentifier(missing) err = %v, want KeyNotFoundError", err)
	}
}

func TestFindByIdentifierAmbiguousPrefix(t *testing.T) {
	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{ID: "abcd-1111", Name: "claude-main", Provider: domainkey.ProviderClaude},
			{ID: "abcd-2222", Name: "claude-backup", Provider: domainkey.ProviderClaude},
		},
	}
	svc := NewKeyService(repo)

	if _, err := svc.FindByIdentifier("abcd"); !IsAmbiguousKeyIdentifierError(err) {
		t.Fatalf("FindByIdentifier(ambiguous) err = %v, want AmbiguousKeyIdentifierError", err)
	}
}

func TestFindByIdentifierInScopeAmbiguousPrefix(t *testing.T) {
	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{ID: "prod-1111", Name: "claude-main", Provider: domainkey.ProviderClaude, Profile: "prod"},
			{ID: "prod-2222", Name: "claude-backup", Provider: domainkey.ProviderClaude, Profile: "prod"},
			{ID: "prod-3333", Name: "other", Provider: domainkey.ProviderClaude, Profile: "default"},
		},
	}
	svc := NewKeyService(repo)

	if _, err := svc.FindByIdentifierInScope(domainkey.ProviderClaude, "prod", "prod"); !IsAmbiguousKeyIdentifierError(err) {
		t.Fatalf("FindByIdentifierInScope(ambiguous) err = %v, want AmbiguousKeyIdentifierError", err)
	}
}

func TestActivateAndDeleteByIdentifier(t *testing.T) {
	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{ID: "1111-aaaa", Name: "claude-1", Provider: domainkey.ProviderClaude},
			{ID: "2222-bbbb", Name: "claude-2", Provider: domainkey.ProviderClaude, Active: true},
		},
	}
	svc := NewKeyService(repo)

	activated, err := svc.ActivateByIdentifier("1111")
	if err != nil {
		t.Fatalf("ActivateByIdentifier error = %v", err)
	}
	if activated.ID != "1111-aaaa" {
		t.Fatalf("activated id = %s", activated.ID)
	}
	active, err := svc.GetActive(domainkey.ProviderClaude, domainkey.DefaultProfile)
	if err != nil {
		t.Fatalf("GetActive error = %v", err)
	}
	if active.ID != "1111-aaaa" {
		t.Fatalf("active id = %s, want 1111-aaaa", active.ID)
	}

	deleted, err := svc.DeleteByIdentifier("claude-2")
	if err != nil {
		t.Fatalf("DeleteByIdentifier error = %v", err)
	}
	if deleted.ID != "2222-bbbb" {
		t.Fatalf("deleted id = %s", deleted.ID)
	}
	if len(repo.keys) != 1 {
		t.Fatalf("repo key count = %d, want 1", len(repo.keys))
	}
}
