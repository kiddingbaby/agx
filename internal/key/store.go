package key

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Provider represents an API key provider
type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderClaude Provider = "claude"
	ProviderGemini Provider = "gemini"
)

// Key represents an API key entry
type Key struct {
	ID        string    `yaml:"id"`
	Provider  Provider  `yaml:"provider"`
	Name      string    `yaml:"name"`
	APIKey    string    `yaml:"api_key"`            // encrypted
	BaseURL   string    `yaml:"base_url,omitempty"` // plaintext, optional
	Tags      []string  `yaml:"tags,omitempty"`
	Active    bool      `yaml:"active"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty"`
}

// Store manages key storage
type Store struct {
	path   string
	secret []byte
	Keys   []Key `yaml:"keys"`
}

// NewStore creates a new key store
func NewStore(path string, secret []byte) (*Store, error) {
	if len(secret) != 32 {
		return nil, errors.New("secret must be 32 bytes")
	}
	s := &Store{
		path:   path,
		secret: secret,
		Keys:   []Key{},
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// Add adds a new key
func (s *Store) Add(provider Provider, name, apiKey, baseURL string, tags []string) (*Key, error) {
	encrypted, err := s.encrypt(apiKey)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	key := Key{
		ID:        uuid.New().String(),
		Provider:  provider,
		Name:      name,
		APIKey:    encrypted,
		BaseURL:   baseURL,
		Tags:      tags,
		Active:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.Keys = append(s.Keys, key)
	return &key, s.save()
}

// Update updates key fields in patch mode.
// API key is only updated when apiKey is non-empty; other fields are always overwritten.
func (s *Store) Update(id string, provider Provider, name, apiKey, baseURL string, tags []string) (*Key, error) {
	for i, k := range s.Keys {
		if k.ID != id {
			continue
		}

		encrypted := k.APIKey
		if apiKey != "" {
			var err error
			encrypted, err = s.encrypt(apiKey)
			if err != nil {
				return nil, err
			}
		}

		s.Keys[i].Provider = provider
		s.Keys[i].Name = name
		s.Keys[i].APIKey = encrypted
		s.Keys[i].BaseURL = baseURL
		s.Keys[i].Tags = tags
		s.Keys[i].UpdatedAt = time.Now()
		if s.Keys[i].Active {
			for j := range s.Keys {
				if j != i && s.Keys[j].Provider == provider {
					s.Keys[j].Active = false
				}
			}
		}

		if err := s.save(); err != nil {
			return nil, err
		}

		updated := s.Keys[i]
		if apiKey != "" {
			updated.APIKey = apiKey
		}
		return &updated, nil
	}
	return nil, errors.New("key not found")
}

// Delete removes a key by ID
func (s *Store) Delete(id string) error {
	for i, k := range s.Keys {
		if k.ID == id {
			s.Keys = append(s.Keys[:i], s.Keys[i+1:]...)
			return s.save()
		}
	}
	return errors.New("key not found")
}

// Activate sets a key as active (deactivates others of same provider)
func (s *Store) Activate(id string) error {
	var found bool
	var provider Provider
	for i, k := range s.Keys {
		if k.ID == id {
			found = true
			provider = k.Provider
			s.Keys[i].Active = true
		}
	}
	if !found {
		return errors.New("key not found")
	}
	// Deactivate other keys of same provider
	for i, k := range s.Keys {
		if k.Provider == provider && k.ID != id {
			s.Keys[i].Active = false
		}
	}
	return s.save()
}

// List returns all keys (without decrypting API keys)
func (s *Store) List() []Key {
	return s.Keys
}

// HasActive returns true if there is an active key for the given provider.
// Unlike GetActive, this does not decrypt the API key.
func (s *Store) HasActive(provider Provider) bool {
	for _, k := range s.Keys {
		if k.Provider == provider && k.Active {
			return true
		}
	}
	return false
}

// GetActive returns the active key for a provider
func (s *Store) GetActive(provider Provider) (*Key, error) {
	for _, k := range s.Keys {
		if k.Provider == provider && k.Active {
			decrypted, err := s.decrypt(k.APIKey)
			if err != nil {
				return nil, err
			}
			result := k
			result.APIKey = decrypted
			return &result, nil
		}
	}
	return nil, errors.New("no active key for provider")
}

func (s *Store) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.secret)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *Store) decrypt(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.secret)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, s); err != nil {
		return err
	}

	// Backward compatibility: old entries may not have updated_at.
	for i := range s.Keys {
		if !s.Keys[i].UpdatedAt.IsZero() {
			continue
		}
		if !s.Keys[i].CreatedAt.IsZero() {
			s.Keys[i].UpdatedAt = s.Keys[i].CreatedAt
		} else {
			s.Keys[i].UpdatedAt = time.Now()
		}
	}
	return nil
}

func (s *Store) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
