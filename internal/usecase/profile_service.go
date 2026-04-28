package usecase

import (
	"sort"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

const backupHistoryLimit = 5

type AddProfileInput struct {
	BaseURL string
	APIKey  string
	Bind    []domainprofile.Agent
}

type AddProfileResult struct {
	Relay    *domainprofile.Profile
	Bindings *BindingsResult
}

type EditProfileInput struct {
	BaseURL *string
	APIKey  *string
	Bind    []domainprofile.Agent
	Unbind  []domainprofile.Agent
}

type EditProfileResult struct {
	Relay    *domainprofile.Profile
	Bindings *BindingsResult
}

type BindingChangeResult struct {
	Agent        domainprofile.Agent
	Action       string
	Binding      *domainprofile.AgentBinding
	Backup       domainprofile.Backup
	CodexProfile string
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

type ProfileService struct {
	profiles       ports.ProfileRepository
	state          ports.StateRepository
	mutationLocker ports.MutationLocker
	journal        ports.OperationJournal
	codex          ports.CodexSyncer
	claude         ports.ClaudeSyncer
	gemini         ports.GeminiSyncer
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
}

func NewProfileService(profiles ports.ProfileRepository, state ports.StateRepository, codex ports.CodexSyncer, claude ports.ClaudeSyncer, gemini ports.GeminiSyncer) *ProfileService {
	return &ProfileService{
		profiles: profiles,
		state:    state,
		codex:    codex,
		claude:   claude,
		gemini:   gemini,
	}
}

func (s *ProfileService) SetMutationLocker(locker ports.MutationLocker) {
	s.mutationLocker = locker
}

func (s *ProfileService) SetOperationJournal(journal ports.OperationJournal) {
	s.journal = journal
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
