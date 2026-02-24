package keyfile

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/ports"
	"gopkg.in/yaml.v3"
)

var _ ports.KeyRepository = (*Repository)(nil)

type fileModel struct {
	Keys []domainkey.Key `yaml:"keys"`
}

// Repository persists keys in a YAML file and encrypts API keys via AES-GCM.
type Repository struct {
	path   string
	secret []byte
	keys   []domainkey.Key
}

func NewRepository(path string, secret []byte) (*Repository, error) {
	if len(secret) != 32 {
		return nil, errors.New("secret must be 32 bytes")
	}
	r := &Repository{
		path:   path,
		secret: secret,
		keys:   []domainkey.Key{},
	}
	if err := r.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return r, nil
}

func (r *Repository) List() []domainkey.Key {
	out := make([]domainkey.Key, len(r.keys))
	copy(out, r.keys)
	return out
}

func (r *Repository) Add(provider domainkey.Provider, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	encrypted, err := encryptAESGCM(r.secret, apiKey)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	k := domainkey.Key{
		ID:        uuid.New().String(),
		Provider:  provider,
		Name:      name,
		APIKey:    encrypted,
		BaseURL:   baseURL,
		Tags:      tags,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.keys = append(r.keys, k)
	if err := r.save(); err != nil {
		return nil, err
	}
	out := k
	return &out, nil
}

// Update updates key fields in patch mode.
// API key is only updated when apiKey is non-empty; other fields are always overwritten.
func (r *Repository) Update(id string, provider domainkey.Provider, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	for i := range r.keys {
		if r.keys[i].ID != id {
			continue
		}

		encrypted := r.keys[i].APIKey
		if apiKey != "" {
			var err error
			encrypted, err = encryptAESGCM(r.secret, apiKey)
			if err != nil {
				return nil, err
			}
		}

		r.keys[i].Provider = provider
		r.keys[i].Name = name
		r.keys[i].APIKey = encrypted
		r.keys[i].BaseURL = baseURL
		r.keys[i].Tags = tags
		r.keys[i].UpdatedAt = time.Now()
		if r.keys[i].Active {
			for j := range r.keys {
				if i != j && r.keys[j].Provider == provider {
					r.keys[j].Active = false
				}
			}
		}

		if err := r.save(); err != nil {
			return nil, err
		}

		updated := r.keys[i]
		if apiKey != "" {
			updated.APIKey = apiKey
		}
		return &updated, nil
	}
	return nil, errors.New("key not found")
}

func (r *Repository) Delete(id string) error {
	for i := range r.keys {
		if r.keys[i].ID != id {
			continue
		}
		r.keys = append(r.keys[:i], r.keys[i+1:]...)
		return r.save()
	}
	return errors.New("key not found")
}

// Activate sets a key as active and deactivates other keys of the same provider.
func (r *Repository) Activate(id string) error {
	var (
		found    bool
		provider domainkey.Provider
	)
	for i := range r.keys {
		if r.keys[i].ID != id {
			continue
		}
		found = true
		provider = r.keys[i].Provider
		r.keys[i].Active = true
	}
	if !found {
		return errors.New("key not found")
	}
	for i := range r.keys {
		if r.keys[i].Provider == provider && r.keys[i].ID != id {
			r.keys[i].Active = false
		}
	}
	return r.save()
}

func (r *Repository) HasActive(provider domainkey.Provider) bool {
	for _, k := range r.keys {
		if k.Provider == provider && k.Active {
			return true
		}
	}
	return false
}

func (r *Repository) GetActive(provider domainkey.Provider) (*domainkey.Key, error) {
	for _, k := range r.keys {
		if k.Provider == provider && k.Active {
			decrypted, err := decryptAESGCM(r.secret, k.APIKey)
			if err != nil {
				return nil, err
			}
			out := k
			out.APIKey = decrypted
			return &out, nil
		}
	}
	return nil, errors.New("no active key for provider")
}

func (r *Repository) load() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}
	var model fileModel
	if err := yaml.Unmarshal(data, &model); err != nil {
		return err
	}

	r.keys = model.Keys
	for i := range r.keys {
		if !r.keys[i].UpdatedAt.IsZero() {
			continue
		}
		if !r.keys[i].CreatedAt.IsZero() {
			r.keys[i].UpdatedAt = r.keys[i].CreatedAt
			continue
		}
		r.keys[i].UpdatedAt = time.Now()
	}
	return nil
}

func (r *Repository) save() error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	model := fileModel{Keys: r.keys}
	data, err := yaml.Marshal(model)
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0600)
}
