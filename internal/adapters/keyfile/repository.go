package keyfile

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/ports"
	"gopkg.in/yaml.v3"
)

var _ ports.KeyRepository = (*Repository)(nil)

type fileModel struct {
	Keys     []domainkey.Key     `yaml:"keys"`
	Profiles []domainkey.Profile `yaml:"profiles,omitempty"`
}

// Repository persists keys in a YAML file and encrypts API keys via AES-GCM.
type Repository struct {
	path     string
	lockPath string
	secret   []byte

	mu       sync.Mutex
	keys     []domainkey.Key
	profiles []domainkey.Profile
}

func NewRepository(path string, secret []byte) (*Repository, error) {
	if len(secret) != 32 {
		return nil, errors.New("secret must be 32 bytes")
	}
	r := &Repository{
		path:     path,
		lockPath: path + ".lock",
		secret:   secret,
		keys:     []domainkey.Key{},
		profiles: []domainkey.Profile{},
	}
	if err := r.loadIfExists(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repository) List() []domainkey.Key {
	keys := make([]domainkey.Key, 0)
	if err := r.withReadLock(func() error {
		keys = make([]domainkey.Key, len(r.keys))
		copy(keys, r.keys)
		return nil
	}); err != nil {
		fallback := make([]domainkey.Key, len(r.keys))
		copy(fallback, r.keys)
		return fallback
	}
	return keys
}

func (r *Repository) Add(provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	if !provider.Valid() {
		return nil, errors.New("invalid provider")
	}
	profile = domainkey.NormalizeProfileName(profile)

	var out domainkey.Key
	err := r.withWriteLock(func() error {
		encrypted, err := encryptAESGCM(r.secret, apiKey)
		if err != nil {
			return err
		}
		now := time.Now()
		k := domainkey.Key{
			ID:        uuid.New().String(),
			Provider:  provider,
			Profile:   profile,
			Name:      name,
			APIKey:    encrypted,
			BaseURL:   baseURL,
			Tags:      tags,
			CreatedAt: now,
			UpdatedAt: now,
		}

		p := r.ensureProfile(provider, profile)
		if !r.hasActive(provider, profile) {
			k.Active = true
		}
		if p.Strategy == "" {
			p.Strategy = domainkey.StrategyFixed
		}
		if p.Strategy == domainkey.StrategyFixed && p.FixedKey == "" {
			p.FixedKey = k.ID
		}
		p.UpdatedAt = now

		r.keys = append(r.keys, k)
		out = k
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Update updates key fields in patch mode.
// API key is only updated when apiKey is non-empty; other fields are always overwritten.
func (r *Repository) Update(id string, provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	if !provider.Valid() {
		return nil, errors.New("invalid provider")
	}
	profile = domainkey.NormalizeProfileName(profile)

	var updated domainkey.Key
	err := r.withWriteLock(func() error {
		for i := range r.keys {
			if r.keys[i].ID != id {
				continue
			}

			oldProvider := r.keys[i].Provider
			oldProfile := r.keys[i].Profile
			encrypted := r.keys[i].APIKey
			if apiKey != "" {
				var err error
				encrypted, err = encryptAESGCM(r.secret, apiKey)
				if err != nil {
					return err
				}
			}

			r.keys[i].Provider = provider
			r.keys[i].Profile = profile
			r.keys[i].Name = name
			r.keys[i].APIKey = encrypted
			r.keys[i].BaseURL = baseURL
			r.keys[i].Tags = tags
			r.keys[i].UpdatedAt = time.Now()

			if r.keys[i].Active {
				r.deactivateOthers(provider, profile, r.keys[i].ID)
			}

			r.ensureProfile(provider, profile)
			r.ensureProfile(oldProvider, oldProfile)
			r.normalizeProfilePointers(provider, profile)
			r.normalizeProfilePointers(oldProvider, oldProfile)

			updated = r.keys[i]
			if apiKey != "" {
				updated.APIKey = apiKey
			}
			return nil
		}
		return errors.New("key not found")
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *Repository) Delete(id string) error {
	return r.withWriteLock(func() error {
		for i := range r.keys {
			if r.keys[i].ID != id {
				continue
			}
			provider := r.keys[i].Provider
			profile := r.keys[i].Profile
			r.keys = append(r.keys[:i], r.keys[i+1:]...)
			r.normalizeProfilePointers(provider, profile)
			return nil
		}
		return errors.New("key not found")
	})
}

// Activate sets a key as active and deactivates other keys of the same provider/profile.
func (r *Repository) Activate(id string) error {
	return r.withWriteLock(func() error {
		var (
			found    bool
			provider domainkey.Provider
			profile  string
		)
		for i := range r.keys {
			if r.keys[i].ID != id {
				continue
			}
			found = true
			provider = r.keys[i].Provider
			profile = r.keys[i].Profile
			r.keys[i].Active = true
		}
		if !found {
			return errors.New("key not found")
		}

		r.deactivateOthers(provider, profile, id)
		p := r.ensureProfile(provider, profile)
		if p.Strategy == domainkey.StrategyFixed {
			p.FixedKey = id
		}
		p.UpdatedAt = time.Now()
		return nil
	})
}

func (r *Repository) HasActive(provider domainkey.Provider, profile string) bool {
	profile = domainkey.NormalizeProfileName(profile)

	active := false
	if err := r.withReadLock(func() error {
		active = r.hasActive(provider, profile)
		return nil
	}); err != nil {
		return r.hasActive(provider, profile)
	}
	return active
}

func (r *Repository) GetActive(provider domainkey.Provider, profile string) (*domainkey.Key, error) {
	profile = domainkey.NormalizeProfileName(profile)

	var out *domainkey.Key
	err := r.withReadLock(func() error {
		for _, k := range r.keys {
			if k.Provider == provider && k.Profile == profile && k.Active {
				decrypted, err := decryptAESGCM(r.secret, k.APIKey)
				if err != nil {
					return err
				}
				copyKey := k
				copyKey.APIKey = decrypted
				out = &copyKey
				return nil
			}
		}
		return errors.New("no active key for provider/profile")
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) Resolve(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	profile = domainkey.NormalizeProfileName(profile)

	var out *domainkey.Key
	err := r.withWriteLock(func() error {
		indexes := r.findKeyIndexes(provider, profile)
		if len(indexes) == 0 {
			return errors.New("no key for provider/profile")
		}

		selectedIdx := -1
		if identifier != "" {
			var err error
			selectedIdx, err = r.findKeyIndexByIdentifier(indexes, identifier)
			if err != nil {
				return err
			}
		} else {
			p := r.ensureProfile(provider, profile)
			if p.Strategy == "" {
				p.Strategy = domainkey.StrategyFixed
			}

			switch p.Strategy {
			case domainkey.StrategyRoundRobin:
				selectedIdx = indexes[p.NextIndex%len(indexes)]
				p.NextIndex = (p.NextIndex + 1) % len(indexes)
				p.UpdatedAt = time.Now()
			case domainkey.StrategyRandom:
				randomIdx, err := randIndex(len(indexes))
				if err != nil {
					return err
				}
				selectedIdx = indexes[randomIdx]
			default:
				selectedIdx = r.pickFixedIndex(indexes, p.FixedKey)
				if selectedIdx < 0 {
					selectedIdx = r.pickActiveIndex(indexes)
				}
				if selectedIdx < 0 {
					selectedIdx = indexes[0]
				}
				p.FixedKey = r.keys[selectedIdx].ID
				p.UpdatedAt = time.Now()
			}
		}

		r.deactivateOthers(provider, profile, r.keys[selectedIdx].ID)
		r.keys[selectedIdx].Active = true
		decrypted, err := decryptAESGCM(r.secret, r.keys[selectedIdx].APIKey)
		if err != nil {
			return err
		}
		copyKey := r.keys[selectedIdx]
		copyKey.APIKey = decrypted
		out = &copyKey
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) ListProfiles(provider domainkey.Provider) []domainkey.Profile {
	profiles := make([]domainkey.Profile, 0)
	if err := r.withReadLock(func() error {
		for _, p := range r.profiles {
			if p.Provider == provider {
				profiles = append(profiles, p)
			}
		}
		return nil
	}); err != nil {
		for _, p := range r.profiles {
			if p.Provider == provider {
				profiles = append(profiles, p)
			}
		}
	}
	return profiles
}

func (r *Repository) SetProfileStrategy(provider domainkey.Provider, profile string, strategy domainkey.RotationStrategy, fixedKey string) error {
	if !provider.Valid() {
		return errors.New("invalid provider")
	}
	if strategy == "" {
		strategy = domainkey.StrategyFixed
	}
	if !strategy.Valid() {
		return errors.New("invalid strategy")
	}
	profile = domainkey.NormalizeProfileName(profile)

	return r.withWriteLock(func() error {
		indexes := r.findKeyIndexes(provider, profile)
		if len(indexes) == 0 {
			return errors.New("no key for provider/profile")
		}

		p := r.ensureProfile(provider, profile)
		p.Strategy = strategy
		p.UpdatedAt = time.Now()
		if strategy != domainkey.StrategyFixed {
			return nil
		}

		selected := -1
		if fixedKey != "" {
			var err error
			selected, err = r.findKeyIndexByIdentifier(indexes, fixedKey)
			if err != nil {
				if strings.Contains(err.Error(), "ambiguous key identifier") {
					return err
				}
				return errors.New("fixed key not found")
			}
		} else {
			selected = r.pickActiveIndex(indexes)
			if selected < 0 {
				selected = indexes[0]
			}
		}

		p.FixedKey = r.keys[selected].ID
		r.deactivateOthers(provider, profile, r.keys[selected].ID)
		r.keys[selected].Active = true
		return nil
	})
}

func (r *Repository) withReadLock(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()

	if err := r.loadIfExists(); err != nil {
		return err
	}
	return fn()
}

func (r *Repository) withWriteLock(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()

	if err := r.loadIfExists(); err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	return r.saveAtomic()
}

func (r *Repository) acquireFileLock() (func(), error) {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(r.lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

func (r *Repository) hasActive(provider domainkey.Provider, profile string) bool {
	for _, k := range r.keys {
		if k.Provider == provider && k.Profile == profile && k.Active {
			return true
		}
	}
	return false
}

func (r *Repository) deactivateOthers(provider domainkey.Provider, profile, keepID string) {
	for i := range r.keys {
		if r.keys[i].Provider == provider && r.keys[i].Profile == profile && r.keys[i].ID != keepID {
			r.keys[i].Active = false
		}
	}
}

func (r *Repository) findKeyIndexes(provider domainkey.Provider, profile string) []int {
	indexes := make([]int, 0)
	for i := range r.keys {
		if r.keys[i].Provider == provider && r.keys[i].Profile == profile {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (r *Repository) findKeyIndexByIdentifier(indexes []int, identifier string) (int, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return -1, errors.New("key not found")
	}

	selected := -1
	for _, idx := range indexes {
		if r.keys[idx].Name == identifier {
			if selected >= 0 {
				return -1, fmt.Errorf("ambiguous key identifier: %s", identifier)
			}
			selected = idx
		}
	}
	if selected >= 0 {
		return selected, nil
	}

	for _, idx := range indexes {
		if strings.HasPrefix(r.keys[idx].ID, identifier) {
			if selected >= 0 {
				return -1, fmt.Errorf("ambiguous key identifier: %s", identifier)
			}
			selected = idx
		}
	}
	if selected >= 0 {
		return selected, nil
	}
	return -1, errors.New("key not found")
}

func (r *Repository) pickFixedIndex(indexes []int, fixedKey string) int {
	if fixedKey == "" {
		return -1
	}
	for _, idx := range indexes {
		if r.keys[idx].ID == fixedKey {
			return idx
		}
	}
	return -1
}

func (r *Repository) pickActiveIndex(indexes []int) int {
	for _, idx := range indexes {
		if r.keys[idx].Active {
			return idx
		}
	}
	return -1
}

func (r *Repository) ensureProfile(provider domainkey.Provider, profile string) *domainkey.Profile {
	profile = domainkey.NormalizeProfileName(profile)
	for i := range r.profiles {
		if r.profiles[i].Provider == provider && r.profiles[i].Name == profile {
			if r.profiles[i].Strategy == "" {
				r.profiles[i].Strategy = domainkey.StrategyFixed
			}
			return &r.profiles[i]
		}
	}

	now := time.Now()
	r.profiles = append(r.profiles, domainkey.Profile{
		Provider:  provider,
		Name:      profile,
		Strategy:  domainkey.StrategyFixed,
		NextIndex: 0,
		UpdatedAt: now,
	})
	return &r.profiles[len(r.profiles)-1]
}

func (r *Repository) normalizeProfilePointers(provider domainkey.Provider, profile string) {
	indexes := r.findKeyIndexes(provider, profile)
	if len(indexes) == 0 {
		return
	}
	p := r.ensureProfile(provider, profile)

	if p.FixedKey != "" && r.pickFixedIndex(indexes, p.FixedKey) >= 0 {
		return
	}

	active := r.pickActiveIndex(indexes)
	if active >= 0 {
		p.FixedKey = r.keys[active].ID
		return
	}

	p.FixedKey = r.keys[indexes[0]].ID
	if p.Strategy == domainkey.StrategyFixed {
		r.keys[indexes[0]].Active = true
	}
}

func (r *Repository) loadIfExists() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			r.keys = []domainkey.Key{}
			r.profiles = []domainkey.Profile{}
			return nil
		}
		return err
	}
	var model fileModel
	if err := yaml.Unmarshal(data, &model); err != nil {
		return err
	}

	r.keys = model.Keys
	r.profiles = model.Profiles

	now := time.Now()
	for i := range r.keys {
		r.keys[i].Profile = domainkey.NormalizeProfileName(r.keys[i].Profile)
		if !r.keys[i].UpdatedAt.IsZero() {
			continue
		}
		if !r.keys[i].CreatedAt.IsZero() {
			r.keys[i].UpdatedAt = r.keys[i].CreatedAt
			continue
		}
		r.keys[i].UpdatedAt = now
	}

	for i := range r.profiles {
		r.profiles[i].Name = domainkey.NormalizeProfileName(r.profiles[i].Name)
		if !r.profiles[i].Strategy.Valid() {
			r.profiles[i].Strategy = domainkey.StrategyFixed
		}
		if r.profiles[i].UpdatedAt.IsZero() {
			r.profiles[i].UpdatedAt = now
		}
	}

	// Backfill profile entries for legacy files where only keys exist.
	for i := range r.keys {
		r.ensureProfile(r.keys[i].Provider, r.keys[i].Profile)
	}
	return nil
}

func (r *Repository) saveAtomic() error {
	model := fileModel{Keys: r.keys, Profiles: r.profiles}
	data, err := yaml.Marshal(model)
	if err != nil {
		return err
	}

	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", r.path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, r.path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func randIndex(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("invalid range")
	}
	max := big.NewInt(int64(n))
	v, err := crand.Int(crand.Reader, max)
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}
