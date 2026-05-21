package profilefile

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
	"gopkg.in/yaml.v3"
)

var _ ports.StateRepository = (*StateRepository)(nil)

type StateRepository struct {
	path     string
	lockPath string
	mu       sync.Mutex
}

func NewStateRepository(path string) *StateRepository {
	return &StateRepository{
		path:     path,
		lockPath: path + ".lock",
	}
}

func (r *StateRepository) Load() (domainprofile.State, error) {
	var state domainprofile.State
	err := r.withLock(func() error {
		loaded, err := r.loadUnlocked()
		if err != nil {
			return err
		}
		state = loaded
		return nil
	})
	return state, err
}

func (r *StateRepository) Save(state domainprofile.State) (domainprofile.State, error) {
	err := r.withLock(func() error {
		return r.saveUnlocked(state)
	})
	return state, err
}

func (r *StateRepository) withLock(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()

	return fn()
}

func (r *StateRepository) acquireFileLock() (func(), error) {
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

func (r *StateRepository) loadUnlocked() (domainprofile.State, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return domainprofile.State{}, nil
		}
		return domainprofile.State{}, err
	}

	var state domainprofile.State
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&state); err != nil {
		return domainprofile.State{}, err
	}
	return state, nil
}

func (r *StateRepository) saveUnlocked(state domainprofile.State) error {
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}

	return fileutil.AtomicWriteFile(r.path, data, 0600)
}
