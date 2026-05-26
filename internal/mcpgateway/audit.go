package mcpgateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEvent is one row in the gateway's jsonl audit log.
type AuditEvent struct {
	Timestamp time.Time     `json:"ts"`
	Server    string        `json:"server"`
	Tool      string        `json:"tool"`
	Duration  time.Duration `json:"duration_ns"`
	Status    string        `json:"status"`
	Error     string        `json:"error,omitempty"`
}

// AuditLog appends events to a jsonl file. Nil-safe: Record on a nil
// AuditLog is a no-op so callers don't need to check.
type AuditLog struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

func OpenAuditLog(path string) (*AuditLog, error) {
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log %s: %w", path, err)
	}
	return &AuditLog{f: f, path: path}, nil
}

func (a *AuditLog) Path() string {
	if a == nil {
		return ""
	}
	return a.path
}

func (a *AuditLog) Record(ev AuditEvent) {
	if a == nil || a.f == nil {
		return
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return
	}
	line = append(line, '\n')
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.f.Write(line)
}

func (a *AuditLog) Close() error {
	if a == nil || a.f == nil {
		return nil
	}
	return a.f.Close()
}
