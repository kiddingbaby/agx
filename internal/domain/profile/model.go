package profile

import (
	"strings"
	"time"
)

type Agent string

const (
	AgentCodex    Agent = "codex"
	AgentClaude   Agent = "claude"
	AgentGemini   Agent = "gemini"
	AgentOpenCode Agent = "opencode"
)

type BindingStatus string

const (
	BindingStatusApplied BindingStatus = "applied"
	BindingStatusBound   BindingStatus = "bound"
)

type RestoreMode string

const (
	RestoreModeRestoreFile       RestoreMode = "restore_file"
	RestoreModeRemoveCreatedFile RestoreMode = "remove_created_file"
)

type ProfileKind string

const (
	ProfileKindRelay ProfileKind = "relay"
)

type TargetKind string

const (
	TargetKindRelay TargetKind = "relay"
)

// CodexWireAPI selects which OpenAI wire schema the codex CLI sends:
// "responses" (default, OpenAI official) or "chat" (国内中转 / 国模 / 多数
// 自建网关，走 /v1/chat/completions). Empty == default == "responses".
type CodexWireAPI string

const (
	CodexWireAPIResponses CodexWireAPI = "responses"
	CodexWireAPIChat      CodexWireAPI = "chat"
)

func (w CodexWireAPI) Valid() bool {
	switch w.Normalized() {
	case "", CodexWireAPIResponses, CodexWireAPIChat:
		return true
	}
	return false
}

func (w CodexWireAPI) Normalized() CodexWireAPI {
	return CodexWireAPI(strings.TrimSpace(strings.ToLower(string(w))))
}

func (w CodexWireAPI) Effective() CodexWireAPI {
	if normalized := w.Normalized(); normalized != "" {
		return normalized
	}
	return CodexWireAPIResponses
}

type Profile struct {
	Name           string                 `yaml:"name"`
	Kind           ProfileKind            `yaml:"kind,omitempty"`
	BaseURL        string                 `yaml:"base-url,omitempty"`
	APIKey         string                 `yaml:"api-key,omitempty"`
	ModelID        string                 `yaml:"model,omitempty"`
	CodexWireAPI   CodexWireAPI           `yaml:"codex-wire-api,omitempty"`
	ProviderFamily OpenCodeProviderFamily `yaml:"provider-family,omitempty"` // Deprecated: opencode syncer writes all 3 provider families; field kept for read compatibility only.
	CreatedAt      time.Time              `yaml:"created-at"`
	UpdatedAt      time.Time              `yaml:"updated-at"`
}

type Backup struct {
	ID             string      `yaml:"id"`
	AppliedProfile string      `yaml:"applied-profile,omitempty"`
	ConfigPath     string      `yaml:"config-path,omitempty"`
	BackupPath     string      `yaml:"backup-path,omitempty"`
	RestoreMode    RestoreMode `yaml:"restore-mode,omitempty"`
	CreatedAt      time.Time   `yaml:"created-at,omitempty"`
}

type ContextBackup struct {
	ID         string     `yaml:"id"`
	TargetKind TargetKind `yaml:"target-kind,omitempty"`
	TargetName string     `yaml:"target-name,omitempty"`
	Path       string     `yaml:"path,omitempty"`
	CreatedAt  time.Time  `yaml:"created-at,omitempty"`
}

type CurrentTarget struct {
	Kind TargetKind `yaml:"kind,omitempty"`
	Name string     `yaml:"name,omitempty"`
}

type RelayTargetState struct {
	ProfileName    string                 `yaml:"profile-name,omitempty"`
	BaseURL        string                 `yaml:"base-url,omitempty"`
	ProviderFamily OpenCodeProviderFamily `yaml:"provider-family,omitempty"`
	ModelID        string                 `yaml:"model-id,omitempty"`
	ModelName      string                 `yaml:"model-name,omitempty"`
}

type TargetState struct {
	Kind        TargetKind       `yaml:"kind,omitempty"`
	ContextPath string           `yaml:"context-path,omitempty"`
	ConfigPath  string           `yaml:"config-path,omitempty"`
	Relay       RelayTargetState `yaml:"relay,omitempty"`
	Backups     []ContextBackup  `yaml:"backups,omitempty"`
	CreatedAt   time.Time        `yaml:"created-at,omitempty"`
	UpdatedAt   time.Time        `yaml:"updated-at,omitempty"`
}

type ManagedAgentState struct {
	CurrentTarget CurrentTarget          `yaml:"current-target,omitempty"`
	Targets       map[string]TargetState `yaml:"targets,omitempty"`
	UpdatedAt     time.Time              `yaml:"updated-at,omitempty"`
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

type AgentProfileBinding struct {
	Status        BindingStatus `yaml:"status,omitempty"`
	ConfigPath    string        `yaml:"config-path,omitempty"`
	LastAppliedAt time.Time     `yaml:"last-applied-at,omitempty"`
	LastBackupID  string        `yaml:"last-backup-id,omitempty"`
}

type CodexProfileBinding = AgentProfileBinding

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
	Codex          CodexState                  `yaml:"codex,omitempty"`
	Claude         AgentBinding                `yaml:"claude,omitempty"`
	Gemini         AgentBinding                `yaml:"gemini,omitempty"`
	OpenCode       OpenCodeState               `yaml:"opencode,omitempty"`
	CurrentProfile string                      `yaml:"current-profile,omitempty"`
	ManagedAgents  map[Agent]ManagedAgentState `yaml:"managed-agents,omitempty"`
	UpdatedAt      time.Time                   `yaml:"updated-at,omitempty"`
}
