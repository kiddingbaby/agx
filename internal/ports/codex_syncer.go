package ports

import "fmt"

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type IncompleteManagedBlockError struct {
	Agent      domainprofile.Agent
	ConfigPath string
}

func (e *IncompleteManagedBlockError) Error() string {
	switch {
	case e.Agent != "" && e.ConfigPath != "":
		return fmt.Sprintf("%s config has an incomplete AGX managed block: %s", e.Agent, e.ConfigPath)
	case e.Agent != "":
		return fmt.Sprintf("%s config has an incomplete AGX managed block", e.Agent)
	default:
		return "config has an incomplete AGX managed block"
	}
}

type AgentConfigSnapshot struct {
	ConfigPath string
	Exists     bool
	Content    []byte
}

type CodexManagedProfile struct {
	Name    string
	BaseURL string
}

type CodexUnmanagedProfile struct {
	Name         string
	ProviderID   string
	ProviderName string
	BaseURL      string
	WireAPI      string
	EnvKey       string
	RelayType    string
	Model        string
	ReviewModel  string
}

type CodexConfigStatus struct {
	ConfigPath            string
	ActiveProfileName     string
	DefaultProfileName    string
	ManagedProfilesByID   map[string]CodexManagedProfile
	UnmanagedProfilesByID map[string]CodexUnmanagedProfile
}

type CodexSyncResult struct {
	ProfileName string
	ConfigPath  string
}

type CodexSyncOptions struct {
	DefaultProfileName string
}

type CodexSyncer interface {
	AgentSyncer
	Status() (*CodexConfigStatus, error)
	Sync(profile domainprofile.Profile, options CodexSyncOptions) (*CodexSyncResult, error)
	ClearDefaultProfile() (string, error)
	RemoveProfile(name string) (string, error)
}
