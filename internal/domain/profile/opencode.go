package profile

import (
	"fmt"
	"strings"
	"time"
)

type OpenCodeProviderFamily string

const (
	OpenCodeProviderFamilyOpenAICompatible OpenCodeProviderFamily = "openai-compatible"
	OpenCodeProviderFamilyAnthropic        OpenCodeProviderFamily = "anthropic"
	OpenCodeProviderFamilyGemini           OpenCodeProviderFamily = "gemini"
)

func (f OpenCodeProviderFamily) Valid() bool {
	switch f {
	case "", OpenCodeProviderFamilyOpenAICompatible, OpenCodeProviderFamilyAnthropic, OpenCodeProviderFamilyGemini:
		return true
	default:
		return false
	}
}

func ParseOpenCodeProviderFamily(raw string) (OpenCodeProviderFamily, bool) {
	family := OpenCodeProviderFamily(strings.TrimSpace(strings.ToLower(raw)))
	if !family.Valid() || family == "" {
		return "", false
	}
	return family, true
}

func OpenCodeProviderID(name string) string {
	name = NormalizeProfileName(name)
	if name == "" {
		return ""
	}
	return "agx-" + name
}

func ValidateOpenCodeModelID(modelID string) error {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fmt.Errorf("opencode model is required")
	}
	if strings.ContainsAny(modelID, "\x00\n\r") {
		return fmt.Errorf("opencode model contains invalid characters")
	}
	return nil
}

type OpenCodeProfileBinding struct {
	Status         BindingStatus          `yaml:"status,omitempty"`
	ConfigPath     string                 `yaml:"config-path,omitempty"`
	LastAppliedAt  time.Time              `yaml:"last-applied-at,omitempty"`
	LastBackupID   string                 `yaml:"last-backup-id,omitempty"`
	ProviderID     string                 `yaml:"provider-id,omitempty"`
	ProviderFamily OpenCodeProviderFamily `yaml:"provider-family,omitempty"`
	ModelID        string                 `yaml:"model-id,omitempty"`
	ModelName      string                 `yaml:"model-name,omitempty"`
}

type OpenCodeState struct {
	BindingView `yaml:",inline"`
	Backups     []Backup `yaml:"backups,omitempty"`
}

func (s OpenCodeState) AgentBinding() AgentBinding {
	return AgentBinding{
		SourceProfile: s.SourceProfile,
		Status:        s.Status,
		ConfigPath:    s.ConfigPath,
		LastAppliedAt: s.LastAppliedAt,
		LastBackupID:  s.LastBackupID,
		Backups:       append([]Backup(nil), s.Backups...),
	}
}

func (s *OpenCodeState) SetAgentBinding(binding AgentBinding) {
	s.BindingView = BindingView{
		SourceProfile: binding.SourceProfile,
		Status:        binding.Status,
		ConfigPath:    binding.ConfigPath,
		LastAppliedAt: binding.LastAppliedAt,
		LastBackupID:  binding.LastBackupID,
	}
	s.Backups = append([]Backup(nil), binding.Backups...)
}
