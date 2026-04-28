package cli

import (
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

type profileView struct {
	Name      string    `json:"name"`
	BaseURL   string    `json:"base_url"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type listProfileView struct {
	Name    string                `json:"name"`
	BaseURL string                `json:"base_url"`
	Agents  []domainprofile.Agent `json:"agents"`
	Current bool                  `json:"current,omitempty"`
}

type listAgentRelayView struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Current bool   `json:"current"`
}

type bindingView struct {
	Agent         domainprofile.Agent         `json:"agent"`
	Relay         string                      `json:"relay,omitempty"`
	Status        domainprofile.BindingStatus `json:"status,omitempty"`
	ConfigPath    string                      `json:"config_path,omitempty"`
	LastAppliedAt time.Time                   `json:"last_applied_at,omitempty"`
	LastBackupID  string                      `json:"last_backup_id,omitempty"`
}

type backupView struct {
	ID          string                    `json:"id"`
	AppliedRelay string                   `json:"applied_relay,omitempty"`
	ConfigPath  string                    `json:"config_path,omitempty"`
	BackupPath  string                    `json:"backup_path,omitempty"`
	RestoreMode domainprofile.RestoreMode `json:"restore_mode,omitempty"`
	CreatedAt   time.Time                 `json:"created_at,omitempty"`
}

func toProfileView(profile domainprofile.Profile) profileView {
	return profileView{
		Name:      profile.Name,
		BaseURL:   profile.BaseURL,
		APIKey:    profile.APIKey,
		CreatedAt: profile.CreatedAt,
		UpdatedAt: profile.UpdatedAt,
	}
}

func toBindingView(agent domainprofile.Agent, binding domainprofile.AgentBinding) bindingView {
	return bindingView{
		Agent:         agent,
		Relay:         binding.SourceProfile,
		Status:        binding.Status,
		ConfigPath:    binding.ConfigPath,
		LastAppliedAt: binding.LastAppliedAt,
		LastBackupID:  binding.LastBackupID,
	}
}

func toBackupView(backup domainprofile.Backup) backupView {
	return backupView{
		ID:           backup.ID,
		AppliedRelay: backup.AppliedProfile,
		ConfigPath:   backup.ConfigPath,
		BackupPath:   backup.BackupPath,
		RestoreMode:  backup.RestoreMode,
		CreatedAt:    backup.CreatedAt,
	}
}
