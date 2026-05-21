package profilefile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
	"gopkg.in/yaml.v3"
)

var _ ports.ProfileRepository = (*Repository)(nil)

type Repository struct {
	dir      string
	lockPath string
	mu       sync.Mutex
}

func NewRepository(dir string) (*Repository, error) {
	return &Repository{
		dir:      dir,
		lockPath: filepath.Join(dir, ".profiles.lock"),
	}, nil
}

func (r *Repository) List() ([]domainprofile.Profile, error) {
	var profiles []domainprofile.Profile
	err := r.withLock(func() error {
		loaded, err := r.loadAll()
		if err != nil {
			return err
		}
		profiles = loaded
		return nil
	})
	return profiles, err
}

func (r *Repository) Get(name string) (*domainprofile.Profile, error) {
	name = domainprofile.NormalizeProfileName(name)
	if err := domainprofile.ValidateProfileName(name); err != nil {
		return nil, err
	}

	var out *domainprofile.Profile
	err := r.withLock(func() error {
		profile, err := r.loadOne(name)
		if err != nil {
			return err
		}
		out = profile
		return nil
	})
	return out, err
}

func (r *Repository) Upsert(profile domainprofile.Profile) (*domainprofile.Profile, error) {
	profile = domainprofile.NormalizeProfile(profile)
	if err := domainprofile.ValidateProfile(profile); err != nil {
		return nil, err
	}

	var out domainprofile.Profile
	err := r.withLock(func() error {
		if err := r.saveOne(profile); err != nil {
			return err
		}
		out = profile
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) Delete(name string) error {
	name = domainprofile.NormalizeProfileName(name)
	if err := domainprofile.ValidateProfileName(name); err != nil {
		return err
	}

	return r.withLock(func() error {
		path := r.profilePath(name)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return &domainprofile.NotFoundError{Name: name}
			}
			return err
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) withLock(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()

	return fn()
}

func (r *Repository) acquireFileLock() (func(), error) {
	if err := os.MkdirAll(r.dir, 0700); err != nil {
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

func (r *Repository) loadAll() ([]domainprofile.Profile, error) {
	if err := os.MkdirAll(r.dir, 0700); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}

	profiles := make([]domainprofile.Profile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		profile, err := r.loadOne(name)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, *profile)
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

func (r *Repository) loadOne(name string) (*domainprofile.Profile, error) {
	data, err := os.ReadFile(r.profilePath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &domainprofile.NotFoundError{Name: name}
		}
		return nil, err
	}

	var profile domainprofile.Profile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&profile); err != nil {
		return nil, err
	}

	profile = domainprofile.NormalizeProfile(profile)
	if err := domainprofile.ValidateProfile(profile); err != nil {
		return nil, fmt.Errorf("invalid profile file %s: %w", name, err)
	}
	if profile.Name != name {
		return nil, fmt.Errorf("invalid profile file %s: filename does not match profile name", name)
	}
	if profile.CreatedAt.IsZero() {
		return nil, fmt.Errorf("invalid profile file %s: created-at is required", name)
	}
	if profile.UpdatedAt.IsZero() {
		return nil, fmt.Errorf("invalid profile file %s: updated-at is required", name)
	}
	return &profile, nil
}

func (r *Repository) saveOne(profile domainprofile.Profile) error {
	data, err := yaml.Marshal(profile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.dir, 0700); err != nil {
		return err
	}

	path := r.profilePath(profile.Name)
	return fileutil.AtomicWriteFile(path, data, 0600)
}

func (r *Repository) profilePath(name string) string {
	return filepath.Join(r.dir, name+".yaml")
}
