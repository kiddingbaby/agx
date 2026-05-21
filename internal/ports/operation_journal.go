package ports

import (
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

type OperationRecord struct {
	ID         string              `yaml:"id" json:"id"`
	Command    string              `yaml:"command" json:"command"`
	Agent      domainprofile.Agent `yaml:"agent" json:"agent"`
	Profile    string              `yaml:"profile,omitempty" json:"relay,omitempty"`
	BackupID   string              `yaml:"backup-id,omitempty" json:"backup_id,omitempty"`
	ConfigPath string              `yaml:"config-path,omitempty" json:"config_path,omitempty"`
	BackupPath string              `yaml:"backup-path,omitempty" json:"backup_path,omitempty"`
	Stage      string              `yaml:"stage" json:"stage"`
	StartedAt  time.Time           `yaml:"started-at" json:"started_at"`
	UpdatedAt  time.Time           `yaml:"updated-at" json:"updated_at"`
}

type OperationJournal interface {
	Current() (*OperationRecord, error)
	Begin(record OperationRecord) error
	Update(record OperationRecord) error
	Clear(id string) error
}
