package codexconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

const (
	beginMarker = "# >>> AGX managed Codex config >>>"
	endMarker   = "# <<< AGX managed Codex config <<<"

	managedPrefix = "agx/"
)

var _ ports.CodexSyncer = (*Syncer)(nil)

type Syncer struct {
	configPath    string
	backupsDir    string
	helperCommand string
}


type managedProfile struct {
	Name            string
	BaseURL         string
	Extras          []string
	ProfileID       string
	ProfileProvider string
}


type unmanagedProvider struct {
	ID          string
	Name        string
	BaseURL     string
	WireAPI     string
	EnvKey      string
	DerivedType string
}


type unmanagedProfile struct {
	Name        string
	ProviderID  string
	Model       string
	ReviewModel string
}

func NewSyncer(configPath, backupsDir, helperCommand string) *Syncer {
	return &Syncer{
		configPath:    configPath,
		backupsDir:    filepath.Join(backupsDir, "codex"),
		helperCommand: helperCommand,
	}
}


func (s *Syncer) Sync(profile domainprofile.Profile, options ports.CodexSyncOptions) (*ports.CodexSyncResult, error) {
	if strings.TrimSpace(s.helperCommand) == "" {
		return nil, fmt.Errorf("codex sync requires AGX to be installed or run from a stable binary path")
	}

	existing, _, err := fileutil.ReadIfExists(s.configPath)
	if err != nil {
		return nil, err
	}

	managedProfiles := extractManagedProfiles(existing)
	entry := managedProfiles[profile.Name]
	entry.Name = profile.Name
	entry.BaseURL = profile.BaseURL
	if len(entry.Extras) == 0 {
		entry.Extras = selectManagedProfileExtras(existing, managedProfiles)
	}
	entry.Extras = mergeModelIntoExtras(entry.Extras, profile.ModelID)
	managedProfiles[profile.Name] = entry

	unmanaged, err := stripManagedBlock(existing, s.configPath)
	if err != nil {
		return nil, err
	}
	unmanaged = removeLegacyManagedSections(unmanaged, managedProfiles)
	next := unmanaged
	if strings.TrimSpace(options.DefaultProfileName) != "" {
		next = upsertRootProfile(next, codexProfileName(options.DefaultProfileName))
	} else if rootProfile := normalizeManagedRootProfile(findRootProfileName(existing), managedProfiles); rootProfile != "" {
		next = upsertRootProfile(next, codexProfileName(rootProfile))
	}
	next = appendManagedBlock(next, renderManagedBlock(managedProfiles, s.helperCommand, codexTablePresence(next)))

	if err := fileutil.AtomicWriteFile(s.configPath, []byte(next), 0600); err != nil {
		return nil, err
	}

	return &ports.CodexSyncResult{
		ProfileName: codexProfileName(profile.Name),
		ConfigPath:  s.configPath,
	}, nil
}


func (s *Syncer) Status() (*ports.CodexConfigStatus, error) {
	content, exists, err := fileutil.ReadIfExists(s.configPath)
	if err != nil {
		return nil, err
	}
	if err := validateManagedBlock(content); err != nil {
		return nil, err
	}

	status := &ports.CodexConfigStatus{
		ConfigPath:            s.configPath,
		ManagedProfilesByID:   map[string]ports.CodexManagedProfile{},
		UnmanagedProfilesByID: map[string]ports.CodexUnmanagedProfile{},
	}
	if !exists {
		return status, nil
	}

	managedProfiles := extractManagedProfiles(content)
	unmanagedProfiles := extractUnmanagedProfiles(content)
	for name, profile := range managedProfiles {
		status.ManagedProfilesByID[name] = ports.CodexManagedProfile{
			Name:    profile.Name,
			BaseURL: profile.BaseURL,
		}
	}
	for name, profile := range unmanagedProfiles {
		status.UnmanagedProfilesByID[name] = profile
	}
	status.ActiveProfileName = normalizeCodexRootProfile(findRootProfileName(content), managedProfiles, unmanagedProfiles)
	status.DefaultProfileName = normalizeManagedRootProfile(findRootProfileName(content), managedProfiles)
	return status, nil
}


func (s *Syncer) Restore(backupPath string) (string, error) {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return "", err
	}
	// Reject obviously corrupted snapshots before clobbering the user's
	// active config. validateManagedBlock catches half-written AGX-managed
	// blocks; an empty backup is also suspicious and should not silently
	// blank out config.toml.
	if len(data) == 0 {
		return "", fmt.Errorf("codex backup %s is empty", backupPath)
	}
	if err := validateManagedBlock(string(data)); err != nil {
		return "", fmt.Errorf("codex backup %s is corrupt: %w", backupPath, err)
	}
	if err := fileutil.AtomicWriteFile(s.configPath, data, 0600); err != nil {
		return "", err
	}
	return s.configPath, nil
}


func (s *Syncer) Snapshot() (*ports.AgentConfigSnapshot, error) {
	content, exists, err := fileutil.ReadIfExists(s.configPath)
	if err != nil {
		return nil, err
	}
	return &ports.AgentConfigSnapshot{
		ConfigPath: s.configPath,
		Exists:     exists,
		Content:    []byte(content),
	}, nil
}


func (s *Syncer) CreateBackup(id string, content []byte) (string, error) {
	if err := os.MkdirAll(s.backupsDir, 0700); err != nil {
		return "", err
	}
	name := fmt.Sprintf("config.toml.%s.bak", id)
	path := filepath.Join(s.backupsDir, name)
	if err := fileutil.AtomicWriteFile(path, content, 0600); err != nil {
		return "", err
	}
	return path, nil
}


func (s *Syncer) ClearDefaultProfile() (string, error) {
	existing, exists, err := fileutil.ReadIfExists(s.configPath)
	if err != nil {
		return "", err
	}
	if !exists {
		return s.configPath, nil
	}

	managedProfiles := extractManagedProfiles(existing)
	stripped, err := stripManagedBlock(existing, s.configPath)
	if err != nil {
		return "", err
	}
	next := removeManagedRootProfile(stripped, findRootProfileName(existing), managedProfiles)
	if len(managedProfiles) > 0 {
		next = appendManagedBlock(next, renderManagedBlock(managedProfiles, s.helperCommand, codexTablePresence(next)))
	}

	if err := fileutil.AtomicWriteFile(s.configPath, []byte(next), 0600); err != nil {
		return "", err
	}
	return s.configPath, nil
}


func (s *Syncer) RemoveProfile(name string) (string, error) {
	name = domainprofile.NormalizeProfileName(name)
	if strings.TrimSpace(name) == "" {
		return s.configPath, nil
	}

	existing, exists, err := fileutil.ReadIfExists(s.configPath)
	if err != nil {
		return "", err
	}
	if !exists {
		return s.configPath, nil
	}

	managedProfiles := extractManagedProfiles(existing)
	rootProfile := normalizeManagedRootProfile(findRootProfileName(existing), managedProfiles)
	delete(managedProfiles, name)

	next, err := stripManagedBlock(existing, s.configPath)
	if err != nil {
		return "", err
	}
	if rootProfile == name {
		next = removeRootProfile(next)
	} else if rootProfile != "" {
		next = upsertRootProfile(next, codexProfileName(rootProfile))
	}
	if len(managedProfiles) > 0 {
		next = appendManagedBlock(next, renderManagedBlock(managedProfiles, s.helperCommand, codexTablePresence(next)))
	}

	if strings.TrimSpace(next) == "" {
		return s.RemoveConfig()
	}
	if err := fileutil.AtomicWriteFile(s.configPath, []byte(next), 0600); err != nil {
		return "", err
	}
	return s.configPath, nil
}


func (s *Syncer) RemoveConfig() (string, error) {
	if err := os.Remove(s.configPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return s.configPath, nil
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

