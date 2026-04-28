package profile

import "time"

type Agent string

const (
	AgentCodex  Agent = "codex"
	AgentClaude Agent = "claude"
	AgentGemini Agent = "gemini"
)

type BindingStatus string

const (
	BindingStatusApplied BindingStatus = "applied"
)

type RestoreMode string

const (
	RestoreModeRestoreFile       RestoreMode = "restore_file"
	RestoreModeRemoveCreatedFile RestoreMode = "remove_created_file"
)

type Profile struct {
	Name      string    `yaml:"name"`
	BaseURL   string    `yaml:"base-url"`
	APIKey    string    `yaml:"api-key"`
	CreatedAt time.Time `yaml:"created-at"`
	UpdatedAt time.Time `yaml:"updated-at"`
}

type Backup struct {
	ID             string      `yaml:"id"`
	AppliedProfile string      `yaml:"applied-profile,omitempty"`
	ConfigPath     string      `yaml:"config-path,omitempty"`
	BackupPath     string      `yaml:"backup-path,omitempty"`
	RestoreMode    RestoreMode `yaml:"restore-mode,omitempty"`
	CreatedAt      time.Time   `yaml:"created-at,omitempty"`
}

type BindingView struct {
	SourceProfile string        `yaml:"source-profile,omitempty"`
	Status        BindingStatus `yaml:"status,omitempty"`
	ConfigPath    string        `yaml:"config-path,omitempty"`
	LastAppliedAt time.Time     `yaml:"last-applied-at,omitempty"`
	LastBackupID  string        `yaml:"last-backup-id,omitempty"`
}

type AgentBinding struct {
	SourceProfile string        `yaml:"source-profile,omitempty"`
	Status        BindingStatus `yaml:"status,omitempty"`
	ConfigPath    string        `yaml:"config-path,omitempty"`
	LastAppliedAt time.Time     `yaml:"last-applied-at,omitempty"`
	LastBackupID  string        `yaml:"last-backup-id,omitempty"`
	Backups       []Backup      `yaml:"backups,omitempty"`
}

type CodexProfileBinding struct {
	Status        BindingStatus `yaml:"status,omitempty"`
	ConfigPath    string        `yaml:"config-path,omitempty"`
	LastAppliedAt time.Time     `yaml:"last-applied-at,omitempty"`
	LastBackupID  string        `yaml:"last-backup-id,omitempty"`
}

type CodexState struct {
	BindingView `yaml:",inline"`
	Backups     []Backup `yaml:"backups,omitempty"`
}

func (s CodexState) AgentBinding() AgentBinding {
	return AgentBinding{
		SourceProfile: s.SourceProfile,
		Status:        s.Status,
		ConfigPath:    s.ConfigPath,
		LastAppliedAt: s.LastAppliedAt,
		LastBackupID:  s.LastBackupID,
		Backups:       append([]Backup(nil), s.Backups...),
	}
}

func (s *CodexState) SetAgentBinding(binding AgentBinding) {
	s.BindingView = BindingView{
		SourceProfile: binding.SourceProfile,
		Status:        binding.Status,
		ConfigPath:    binding.ConfigPath,
		LastAppliedAt: binding.LastAppliedAt,
		LastBackupID:  binding.LastBackupID,
	}
	s.Backups = append([]Backup(nil), binding.Backups...)
}

type State struct {
	Codex         CodexState                     `yaml:"codex,omitempty"`
	CodexProfiles map[string]CodexProfileBinding `yaml:"codex-profiles,omitempty"`
	Claude        AgentBinding                   `yaml:"claude,omitempty"`
	Gemini        AgentBinding                   `yaml:"gemini,omitempty"`
	UpdatedAt     time.Time                      `yaml:"updated-at,omitempty"`
}
