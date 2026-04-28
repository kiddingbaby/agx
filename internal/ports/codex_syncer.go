package ports

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type AgentConfigSnapshot struct {
	ConfigPath string
	Exists     bool
	Content    []byte
}

type CodexManagedProfile struct {
	Name    string
	BaseURL string
}

type CodexConfigStatus struct {
	ConfigPath          string
	DefaultProfileName  string
	ManagedProfilesByID map[string]CodexManagedProfile
}

type CodexSyncResult struct {
	ProfileName string
	ConfigPath  string
}

type CodexSyncOptions struct {
	DefaultProfileName string
}

type CodexSyncer interface {
	Snapshot() (*AgentConfigSnapshot, error)
	Status() (*CodexConfigStatus, error)
	CreateBackup(id string, content []byte) (string, error)
	Sync(profile domainprofile.Profile, options CodexSyncOptions) (*CodexSyncResult, error)
	ClearDefaultProfile() (string, error)
	RemoveProfile(name string) (string, error)
	Restore(backupPath string) (string, error)
	RemoveConfig() (string, error)
	DeleteBackup(backupPath string) error
}
