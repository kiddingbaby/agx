package undofile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kiddingbaby/agx/internal/config"
	"github.com/kiddingbaby/agx/internal/ports"
)

const manifestVersion = "agx-undo-manifest/v1"

type Store struct {
	paths config.Paths
}

func NewStore(paths config.Paths) *Store {
	return &Store{paths: paths}
}

type manifestFile struct {
	Version   string          `json:"version"`
	ID        string          `json:"id"`
	CreatedAt string          `json:"created_at"`
	Command   string          `json:"command"`
	Agent     string          `json:"agent"`
	Family    string          `json:"family"`
	Target    string          `json:"target"`
	Files     []manifestEntry `json:"files"`
}

type manifestEntry struct {
	Path      string `json:"path"`
	Existed   bool   `json:"existed"`
	Mode      int    `json:"mode,omitempty"`
	BackupRel string `json:"backup_rel,omitempty"`
}

func (s *Store) Capture(meta ports.UndoCaptureMeta) (string, error) {
	root := s.backupRoot()
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}

	id, err := newUndoID()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(root, id)
	filesDir := filepath.Join(dir, "files")
	if err := os.MkdirAll(filesDir, 0700); err != nil {
		return "", err
	}

	paths := s.switchFileSet(meta.Agent.Name)
	entries := make([]manifestEntry, 0, len(paths))

	for i, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		fi, statErr := os.Stat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				entries = append(entries, manifestEntry{
					Path:    path,
					Existed: false,
				})
				continue
			}
			return "", statErr
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		backupName := fmt.Sprintf("%03d.bak", i)
		backupRel := filepath.ToSlash(filepath.Join("files", backupName))
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(backupRel)), data, 0600); err != nil {
			return "", err
		}

		entries = append(entries, manifestEntry{
			Path:      path,
			Existed:   true,
			Mode:      int(fi.Mode() & 0777),
			BackupRel: backupRel,
		})
	}

	manifest := manifestFile{
		Version:   manifestVersion,
		ID:        id,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Command:   strings.TrimSpace(meta.Command),
		Agent:     meta.Agent.Name,
		Family:    string(meta.Target.Family),
		Target:    meta.Target.Name,
		Files:     entries,
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		return "", err
	}

	if err := writeAtomicTextFile(filepath.Join(root, "LATEST"), id+"\n", 0600); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) LatestID() (string, error) {
	data, err := os.ReadFile(filepath.Join(s.backupRoot(), "LATEST"))
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", errors.New("no undo point found")
	}
	return id, nil
}

func (s *Store) Restore(id string) (ports.UndoRestoreResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ports.UndoRestoreResult{}, errors.New("undo id is required")
	}

	dir := filepath.Join(s.backupRoot(), id)
	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ports.UndoRestoreResult{}, err
	}

	var manifest manifestFile
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ports.UndoRestoreResult{}, fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.Version != manifestVersion {
		return ports.UndoRestoreResult{}, fmt.Errorf("unsupported manifest version: %s", manifest.Version)
	}

	restored := []string{}
	deleted := []string{}
	for _, entry := range manifest.Files {
		dst := strings.TrimSpace(entry.Path)
		if dst == "" {
			continue
		}
		if !entry.Existed {
			if _, err := os.Stat(dst); err == nil {
				if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
					return ports.UndoRestoreResult{}, err
				}
				deleted = append(deleted, dst)
			}
			continue
		}

		src := filepath.Join(dir, filepath.FromSlash(entry.BackupRel))
		backupBytes, err := os.ReadFile(src)
		if err != nil {
			return ports.UndoRestoreResult{}, err
		}
		mode := os.FileMode(0600)
		if entry.Mode != 0 {
			mode = os.FileMode(entry.Mode)
		}
		if err := writeAtomicBytesFile(dst, backupBytes, mode); err != nil {
			return ports.UndoRestoreResult{}, err
		}
		restored = append(restored, dst)
	}

	sort.Strings(restored)
	sort.Strings(deleted)
	return ports.UndoRestoreResult{
		ID:       id,
		Restored: restored,
		Deleted:  deleted,
	}, nil
}

func (s *Store) backupRoot() string {
	return filepath.Join(s.paths.ConfigDir, "backups")
}

func (s *Store) switchFileSet(agentName string) []string {
	files := []string{
		s.paths.StorePath,
		s.paths.ProviderConfigPath,
	}
	if strings.TrimSpace(s.paths.ConfigDir) != "" {
		files = append(files, filepath.Join(s.paths.ConfigDir, "toolconfig_state.json"))
	}
	// Capture all agent-native config files so `agx undo` and rollback can safely restore
	// multi-agent switches (e.g. multi-protocol gateways like NewAPI/new-api).
	_ = agentName // retained for manifest metadata; file set is intentionally global.
	files = append(files,
		s.paths.CodexAuthPath,
		s.paths.CodexConfigPath,
		s.paths.ClaudeSettingsPath,
		s.paths.GeminiSettingsPath,
		s.paths.GeminiEnvPath,
	)
	return files
}

func newUndoID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	suffix := hex.EncodeToString(buf)
	return fmt.Sprintf("%s_%s", time.Now().UTC().Format("20060102-150405-000000000"), suffix), nil
}

func writeAtomicTextFile(path, content string, perm os.FileMode) error {
	return writeAtomicBytesFile(path, []byte(content), perm)
}

func writeAtomicBytesFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
