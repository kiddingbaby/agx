package ports

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type GeminiSyncResult struct {
	ConfigPath string
}

type GeminiSyncer interface {
	Snapshot() (*AgentConfigSnapshot, error)
	CreateBackup(id string, content []byte) (string, error)
	Sync(profile domainprofile.Profile) (*GeminiSyncResult, error)
	Restore(backupPath string) (string, error)
	RemoveConfig() (string, error)
	DeleteBackup(backupPath string) error
}
