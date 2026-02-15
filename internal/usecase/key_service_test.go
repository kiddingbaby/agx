package usecase

import (
	"errors"
	"testing"
	"time"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
)

type fakeKeyRepo struct {
	keys []domainkey.Key
}

func (f *fakeKeyRepo) List() []domainkey.Key {
	out := make([]domainkey.Key, len(f.keys))
	copy(out, f.keys)
	return out
}

func (f *fakeKeyRepo) Add(provider domainkey.Provider, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	k := domainkey.Key{
		ID:        "new-id",
		Provider:  provider,
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

func (f *fakeKeyRepo) Update(id string, provider domainkey.Provider, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	for i := range f.keys {
		if f.keys[i].ID != id {
			continue
		}
		f.keys[i].Provider = provider
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
	found := false
	for i := range f.keys {
		if f.keys[i].ID == id {
			found = true
			provider = f.keys[i].Provider
			f.keys[i].Active = true
		}
	}
	if !found {
		return errors.New("key not found")
	}
	for i := range f.keys {
		if f.keys[i].Provider == provider && f.keys[i].ID != id {
			f.keys[i].Active = false
		}
	}
	return nil
}

func (f *fakeKeyRepo) GetActive(provider domainkey.Provider) (*domainkey.Key, error) {
	for i := range f.keys {
		if f.keys[i].Provider == provider && f.keys[i].Active {
			out := f.keys[i]
			return &out, nil
		}
	}
	return nil, errors.New("no active key")
}

func (f *fakeKeyRepo) HasActive(provider domainkey.Provider) bool {
	for i := range f.keys {
		if f.keys[i].Provider == provider && f.keys[i].Active {
			return true
		}
	}
	return false
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
	active, err := svc.GetActive(domainkey.ProviderClaude)
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
