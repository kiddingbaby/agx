package claudeconfig

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

const apiKeyHelperTTL = "3600000"

var _ ports.ClaudeSyncer = (*Syncer)(nil)

type Syncer struct {
	settingsPath string
	backupsDir   string
	helperPath   string
}

func NewSyncer(settingsPath, backupsDir, helperPath string) *Syncer {
	return &Syncer{
		settingsPath: settingsPath,
		backupsDir:   filepath.Join(backupsDir, "claude"),
		helperPath:   helperPath,
	}
}

func (s *Syncer) Sync(profile domainprofile.Profile) (*ports.ClaudeSyncResult, error) {
	if strings.TrimSpace(s.helperPath) == "" {
		return nil, fmt.Errorf("claude sync requires AGX to be installed or run from a stable binary path")
	}

	existing, _, _, err := s.readSettings()
	if err != nil {
		return nil, err
	}

	next, err := buildSettings(existing, profile, s.helperPath)
	if err != nil {
		return nil, err
	}
	if err := fileutil.AtomicWriteJSON(s.settingsPath, next, 0600); err != nil {
		return nil, err
	}

	return &ports.ClaudeSyncResult{
		ConfigPath: s.settingsPath,
	}, nil
}

func (s *Syncer) Restore(backupPath string) (string, error) {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return "", err
	}
	// Trimmed empty content is acceptable (Claude treats an empty settings
	// file as "no overrides"). Anything else must parse as JSON before we
	// clobber the active settings file.
	if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
		if !json.Valid(data) {
			return "", fmt.Errorf("claude backup %s is not valid JSON", backupPath)
		}
	}
	if err := fileutil.AtomicWriteFile(s.settingsPath, data, 0600); err != nil {
		return "", err
	}
	return s.settingsPath, nil
}

func (s *Syncer) Snapshot() (*ports.AgentConfigSnapshot, error) {
	_, data, exists, err := s.readSettings()
	if err != nil {
		return nil, err
	}
	return &ports.AgentConfigSnapshot{
		ConfigPath: s.settingsPath,
		Exists:     exists,
		Content:    data,
	}, nil
}

func (s *Syncer) CreateBackup(id string, content []byte) (string, error) {
	if err := os.MkdirAll(s.backupsDir, 0700); err != nil {
		return "", err
	}

	name := fmt.Sprintf("settings.json.%s.bak", id)
	path := filepath.Join(s.backupsDir, name)
	if err := fileutil.AtomicWriteFile(path, content, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Syncer) RemoveConfig() (string, error) {
	settings, _, exists, err := s.readSettings()
	if err != nil {
		return "", err
	}
	if !exists {
		return s.settingsPath, nil
	}

	delete(settings, "apiKeyHelper")
	if raw, ok := settings["env"]; ok && raw != nil {
		if env, ok := raw.(map[string]any); ok {
			delete(env, "ANTHROPIC_BASE_URL")
			delete(env, "CLAUDE_CODE_API_KEY_HELPER_TTL_MS")
			if len(env) == 0 {
				delete(settings, "env")
			} else {
				settings["env"] = env
			}
		}
	}
	if len(settings) == 0 {
		if err := os.Remove(s.settingsPath); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return s.settingsPath, nil
	}
	if err := fileutil.AtomicWriteJSON(s.settingsPath, settings, 0600); err != nil {
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

func (s *Syncer) readSettings() (map[string]any, []byte, bool, error) {
	data, err := os.ReadFile(s.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil, false, nil
		}
		return nil, nil, false, err
	}
	if len(data) == 0 {
		return map[string]any{}, data, true, nil
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, nil, false, fmt.Errorf("parse claude settings: %w", err)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	return settings, data, true, nil
}

func buildSettings(settings map[string]any, profile domainprofile.Profile, helperPath string) (map[string]any, error) {
	if settings == nil {
		settings = map[string]any{}
	}

	env := map[string]any{}
	if raw, ok := settings["env"]; ok && raw != nil {
		existingEnv, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("claude settings env must be an object")
		}
		for key, value := range existingEnv {
			env[key] = value
		}
	}

	settings["apiKeyHelper"] = shellCommand(helperPath, "__api-key", profile.Name)
	env["ANTHROPIC_BASE_URL"] = domainprofile.AgentBaseURL(domainprofile.AgentClaude, profile.BaseURL)
	env["CLAUDE_CODE_API_KEY_HELPER_TTL_MS"] = apiKeyHelperTTL
	settings["env"] = env
	return settings, nil
}

func shellCommand(parts ...string) string {
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, shellQuote(part))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if isShellSafeWord(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func isShellSafeWord(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.', r == '/', r == ':':
		default:
			return false
		}
	}
	return true
}
