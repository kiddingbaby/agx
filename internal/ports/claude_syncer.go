package ports

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type ClaudeSyncResult struct {
	ConfigPath string
}

type ClaudeSyncer interface {
	Snapshot() (*AgentConfigSnapshot, error)
	CreateBackup(id string, content []byte) (string, error)
	Sync(profile domainprofile.Profile) (*ClaudeSyncResult, error)
	Restore(backupPath string) (string, error)
	RemoveConfig() (string, error)
	DeleteBackup(backupPath string) error
}
