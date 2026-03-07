package cli

import (
	"bytes"
	"errors"
	"testing"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/usecase"
)

type keyRepoStub struct {
	keys []domainkey.Key
}

func (s *keyRepoStub) List() []domainkey.Key {
	out := make([]domainkey.Key, len(s.keys))
	copy(out, s.keys)
	return out
}

func (s *keyRepoStub) Add(provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	return nil, errors.New("not implemented")
}

func (s *keyRepoStub) Update(id string, provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	return nil, errors.New("not implemented")
}

func (s *keyRepoStub) Delete(id string) error {
	for i := range s.keys {
		if s.keys[i].ID == id {
			s.keys = append(s.keys[:i], s.keys[i+1:]...)
			return nil
		}
	}
	return errors.New("key not found")
}

func (s *keyRepoStub) Activate(id string) error {
	for i := range s.keys {
		s.keys[i].Active = s.keys[i].ID == id
	}
	return nil
}

func (s *keyRepoStub) GetActive(provider domainkey.Provider, profile string) (*domainkey.Key, error) {
	for i := range s.keys {
		if s.keys[i].Provider == provider && s.keys[i].Profile == profile && s.keys[i].Active {
			out := s.keys[i]
			return &out, nil
		}
	}
	return nil, errors.New("no active key")
}

func (s *keyRepoStub) HasActive(provider domainkey.Provider, profile string) bool {
	_, err := s.GetActive(provider, profile)
	return err == nil
}

func (s *keyRepoStub) Resolve(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	for i := range s.keys {
		if s.keys[i].Provider != provider || s.keys[i].Profile != profile {
			continue
		}
		if identifier == "" || s.keys[i].ID == identifier || s.keys[i].Name == identifier {
			out := s.keys[i]
			return &out, nil
		}
	}
	return nil, &usecase.KeyNotFoundError{Identifier: identifier}
}

func (s *keyRepoStub) ListProfiles(provider domainkey.Provider) []domainkey.Profile {
	return nil
}

func (s *keyRepoStub) SetProfileStrategy(provider domainkey.Provider, profile string, strategy domainkey.RotationStrategy, fixedKey string) error {
	return nil
}

func TestHandleKeysLsShortIDDoesNotPanic(t *testing.T) {
	repo := &keyRepoStub{keys: []domainkey.Key{{
		ID:       "short",
		Name:     "demo",
		Provider: domainkey.ProviderClaude,
		Profile:  domainkey.DefaultProfile,
		Active:   true,
	}}}
	root := &Root{keySvc: usecase.NewKeyService(repo)}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr

	if rc := root.handleKeysLs(nil); rc != 0 {
		t.Fatalf("handleKeysLs() rc = %d, stderr=%s", rc, stderr.String())
	}
	if got := stdout.String(); !bytes.Contains([]byte(got), []byte("(short)")) {
		t.Fatalf("handleKeysLs() output = %q, want short ID shown", got)
	}
}

func TestHandleKeysActivateRejectsAmbiguousPrefix(t *testing.T) {
	repo := &keyRepoStub{keys: []domainkey.Key{
		{ID: "dup-1111", Name: "first", Provider: domainkey.ProviderClaude, Profile: domainkey.DefaultProfile},
		{ID: "dup-2222", Name: "second", Provider: domainkey.ProviderClaude, Profile: domainkey.DefaultProfile},
	}}
	root := &Root{keySvc: usecase.NewKeyService(repo)}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr

	if rc := root.handleKeysActivate([]string{"dup"}); rc != 1 {
		t.Fatalf("handleKeysActivate() rc = %d, want 1", rc)
	}
	if got := stderr.String(); !bytes.Contains([]byte(got), []byte("ambiguous key identifier: dup")) {
		t.Fatalf("handleKeysActivate() stderr = %q, want ambiguous identifier", got)
	}
}
