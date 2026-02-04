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
	APIKey    string    `yaml:"api_key"` // encrypted
	Tags      []string  `yaml:"tags,omitempty"`
	Active    bool      `yaml:"active"`
	CreatedAt time.Time `yaml:"created_at"`
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
func (s *Store) Add(provider Provider, name, apiKey string, tags []string) (*Key, error) {
	encrypted, err := s.encrypt(apiKey)
	if err != nil {
		return nil, err
	}
	key := Key{
		ID:        uuid.New().String(),
		Provider:  provider,
		Name:      name,
		APIKey:    encrypted,
		Tags:      tags,
		Active:    false,
		CreatedAt: time.Now(),
	}
	s.Keys = append(s.Keys, key)
	return &key, s.save()
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
	return yaml.Unmarshal(data, s)
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
