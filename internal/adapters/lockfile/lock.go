package lockfile

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/kiddingbaby/agx/internal/ports"
)

var _ ports.MutationLocker = (*Lock)(nil)

type Lock struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Lock {
	return &Lock{path: path}
}

func (l *Lock) Lock() (func(), error) {
	l.mu.Lock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0700); err != nil {
		l.mu.Unlock()
		return nil, err
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		l.mu.Unlock()
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		l.mu.Unlock()
		return nil, err
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
			l.mu.Unlock()
		})
	}, nil
}
