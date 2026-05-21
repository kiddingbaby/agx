package geminiconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

var _ ports.GeminiSyncer = (*Syncer)(nil)

// Syncer materializes Gemini's settings.json inside an agx-managed context.
// The Gemini credential (GEMINI_API_KEY) and base URL (GOOGLE_GEMINI_BASE_URL)
// are injected into the process environment by the native runtime at launch
// time; they are never persisted to disk. settings.json holds the
// non-credential pieces — api-key auth selection, sandbox disabled — plus
// whatever top-level keys the user adds (mcpServers, extensions, ui, ...).
type Syncer struct {
	settingsPath string
	backupsDir   string
}

func NewSyncer(settingsPath, backupsDir string) *Syncer {
	return &Syncer{
		settingsPath: settingsPath,
		backupsDir:   filepath.Join(backupsDir, "gemini"),
	}
}

// Sync ensures settings.json contains agx's required selections, preserving
// any other keys the user (or a previous tool) added.
func (s *Syncer) Sync(_ domainprofile.Profile) (*ports.GeminiSyncResult, error) {
	settings, err := s.loadSettings()
	if err != nil {
		return nil, err
	}
	setNested(settings, []string{"security", "auth", "selectedType"}, "gemini-api-key")
	setNested(settings, []string{"tools", "sandbox"}, false)
	if err := fileutil.AtomicWriteJSON(s.settingsPath, settings, 0600); err != nil {
		return nil, err
	}
	return &ports.GeminiSyncResult{ConfigPath: s.settingsPath}, nil
}

func (s *Syncer) Snapshot() (*ports.AgentConfigSnapshot, error) {
	content, exists, err := fileutil.ReadIfExists(s.settingsPath)
	if err != nil {
		return nil, err
	}
	return &ports.AgentConfigSnapshot{
		ConfigPath: s.settingsPath,
		Exists:     exists,
		Content:    []byte(content),
	}, nil
}

func (s *Syncer) Restore(backupPath string) (string, error) {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(data)) == "" {
		if err := os.Remove(s.settingsPath); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return s.settingsPath, nil
	}
	if err := fileutil.AtomicWriteFile(s.settingsPath, data, 0600); err != nil {
		return "", err
	}
	return s.settingsPath, nil
}

func (s *Syncer) CreateBackup(id string, content []byte) (string, error) {
	if err := os.MkdirAll(s.backupsDir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(s.backupsDir, fmt.Sprintf("settings.json.%s.bak", id))
	if err := fileutil.AtomicWriteFile(path, content, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Syncer) RemoveConfig() (string, error) {
	if err := os.Remove(s.settingsPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return s.settingsPath, nil
}

func (s *Syncer) DeleteBackup(backupPath string) error {
	if strings.TrimSpace(backupPath) == "" {
		return nil
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Syncer) loadSettings() (map[string]any, error) {
	raw, exists, err := fileutil.ReadIfExists(s.settingsPath)
	if err != nil {
		return nil, err
	}
	if !exists || strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.settingsPath, err)
	}
	if settings == nil {
		return map[string]any{}, nil
	}
	return settings, nil
}

// setNested assigns value into a nested object inside m, creating
// intermediate maps as needed. path must be non-empty.
func setNested(m map[string]any, path []string, value any) {
	cur := m
	for _, key := range path[:len(path)-1] {
		next, ok := cur[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[key] = next
		}
		cur = next
	}
	cur[path[len(path)-1]] = value
}
