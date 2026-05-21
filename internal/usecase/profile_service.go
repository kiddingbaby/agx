package usecase

import (
	"sort"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

const backupHistoryLimit = 5

type AddProfileInput struct {
	BaseURL  string
	APIKey   string
	Bind     []domainprofile.Agent
	OpenCode *domainprofile.OpenCodeProfileBinding
}

type AddProfileResult struct {
	Relay    *domainprofile.Profile
	Bindings *BindingsResult
}

type EditProfileInput struct {
	Name     *string
	BaseURL  *string
	APIKey   *string
	Bind     []domainprofile.Agent
	Unbind   []domainprofile.Agent
	OpenCode *domainprofile.OpenCodeProfileBinding
}

type EditProfileResult struct {
	Relay    *domainprofile.Profile
	Bindings *BindingsResult
}

type BindingChangeResult struct {
	Agent         domainprofile.Agent
	Action        string
	PreviousRelay string
	Binding       *domainprofile.AgentBinding
	Backup        domainprofile.Backup
	CodexProfile  string
}

type BindingsResult struct {
	Relay   *domainprofile.Profile
	Changed []BindingChangeResult
}

type AgentSetResult struct {
	Agent            domainprofile.Agent
	Profile          *domainprofile.Profile
	Binding          domainprofile.AgentBinding
	Backup           domainprofile.Backup
	CodexProfileName string
}

type RestoreResult struct {
	Agent      domainprofile.Agent
	ConfigPath string
	Backup     domainprofile.Backup
}

type BackupResult struct {
	Agent  domainprofile.Agent
	Backup domainprofile.Backup
}

type RelayTargetInput struct {
	BaseURL        string
	APIKey         string
	ProviderFamily domainprofile.OpenCodeProviderFamily
	ModelID        string
	ModelName      string
}

type TargetResult struct {
	Agent  domainprofile.Agent
	Name   string
	Target domainprofile.TargetState
}

type ManagedPaths struct {
	ContextsDir   string
	BackupsDir    string
	HelperCommand string
}

type ManagedSyncerFactory struct {
	NewCodex    func(configPath, backupsDir, helperCommand string) ports.CodexSyncer
	NewClaude   func(settingsPath, backupsDir, helperPath string) ports.ClaudeSyncer
	NewGemini   func(settingsPath, backupsDir string) ports.GeminiSyncer
	NewOpenCode func(configPath, backupsDir string) ports.OpenCodeSyncer
}

type ProfileService struct {
	profiles       ports.ProfileRepository
	state          ports.StateRepository
	mutationLocker ports.MutationLocker
	journal        ports.OperationJournal
	codex          ports.CodexSyncer
	claude         ports.ClaudeSyncer
	gemini         ports.GeminiSyncer
	openCode       ports.OpenCodeSyncer
	managedPaths   ManagedPaths
	managedSyncers ManagedSyncerFactory
}

type DoctorReport struct {
	OK        bool                   `json:"ok"`
	Operation *ports.OperationRecord `json:"operation,omitempty"`
	Issues    []DoctorIssue          `json:"issues"`
}

type DoctorIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Action   string `json:"action,omitempty"`
}

type EditManagedProfileResult struct {
	Profile         *domainprofile.Profile `json:"profile"`
	ResyncedTargets []TargetSyncOutcome    `json:"resynced_targets,omitempty"`
	FailedTargets   []TargetSyncFailure    `json:"failed_targets,omitempty"`
}

type TargetSyncOutcome struct {
	Agent      domainprofile.Agent `json:"agent"`
	TargetName string              `json:"target"`
	ConfigPath string              `json:"config_path,omitempty"`
}

type TargetSyncFailure struct {
	Agent      domainprofile.Agent `json:"agent"`
	TargetName string              `json:"target"`
	Error      string              `json:"error"`
}

func NewProfileService(profiles ports.ProfileRepository, state ports.StateRepository, codex ports.CodexSyncer, claude ports.ClaudeSyncer, gemini ports.GeminiSyncer, openCode ports.OpenCodeSyncer) *ProfileService {
	return &ProfileService{
		profiles: profiles,
		state:    state,
		codex:    codex,
		claude:   claude,
		gemini:   gemini,
		openCode: openCode,
	}
}

func (s *ProfileService) SetMutationLocker(locker ports.MutationLocker) {
	s.mutationLocker = locker
}

func (s *ProfileService) SetOperationJournal(journal ports.OperationJournal) {
	s.journal = journal
}

func (s *ProfileService) SetManagedRuntime(paths ManagedPaths, syncers ManagedSyncerFactory) {
	s.managedPaths = paths
	s.managedSyncers = syncers
}

func (s *ProfileService) List() ([]domainprofile.Profile, error) {
	profiles, err := s.profiles.List()
	if err != nil {
		return nil, err
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

func (s *ProfileService) Get(name string) (*domainprofile.Profile, error) {
	return s.profiles.Get(domainprofile.NormalizeProfileName(name))
}

func (s *ProfileService) APIKey(name string) (string, error) {
	profile, err := s.Get(name)
	if err != nil {
		return "", err
	}
	return profile.APIKey, nil
}
