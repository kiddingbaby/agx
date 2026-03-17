package usecase

import (
	"fmt"
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/ports"
)

// ProviderService manages reusable provider targets and active family bindings.
type ProviderService struct {
	repo ports.ProviderConfigRepository
}

func NewProviderService(repo ports.ProviderConfigRepository) *ProviderService {
	return &ProviderService{repo: repo}
}

type SaveTargetInput struct {
	Name               string
	Family             string
	Kind               string
	Access             string
	Auth               string
	BaseURL            string
	Model              string
	Env                map[string]string
	WireAPI            string
	RequiresOpenAIAuth *bool
}

func (s *ProviderService) ListTargets() []domainprovider.Target {
	return s.repo.ListTargets()
}

func (s *ProviderService) GetCurrentSite() string {
	return s.repo.GetCurrentSite()
}

func (s *ProviderService) SetCurrentSite(targetName string) error {
	return s.repo.SetCurrentSite(targetName)
}

func (s *ProviderService) ListBindings() []domainprovider.Binding {
	return s.repo.ListBindings()
}

func (s *ProviderService) GetTarget(name string) (*domainprovider.Target, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("target name is required")
	}
	return s.repo.GetTarget(name)
}

func (s *ProviderService) SaveTarget(in SaveTargetInput) (*domainprovider.Target, error) {
	family, ok := domainprovider.ParseFamily(in.Family)
	if !ok {
		return nil, fmt.Errorf("invalid family %q", in.Family)
	}
	kind, ok := domainprovider.ParseKind(in.Kind)
	if !ok {
		return nil, fmt.Errorf("invalid kind %q", in.Kind)
	}
	access, ok := domainprovider.ParseAccessMode(in.Access)
	if !ok {
		return nil, fmt.Errorf("invalid access %q", in.Access)
	}
	authRaw := in.Auth
	if strings.TrimSpace(authRaw) == "" {
		authRaw = string(domainprovider.AuthAPIKey)
	}
	auth, ok := domainprovider.ParseAuthMode(authRaw)
	if !ok {
		return nil, fmt.Errorf("invalid auth %q", authRaw)
	}

	var wireAPI domainprovider.WireAPI
	if strings.TrimSpace(in.WireAPI) != "" {
		parsed, ok := domainprovider.ParseWireAPI(in.WireAPI)
		if !ok {
			return nil, fmt.Errorf("invalid wire-api %q", in.WireAPI)
		}
		wireAPI = parsed
	}

	baseURL := strings.TrimSpace(in.BaseURL)
	if access == domainprovider.AccessThirdParty {
		baseURL = normalizeThirdPartyBaseURL(family, baseURL)
	}

	return s.repo.UpsertTarget(domainprovider.Target{
		Name:               strings.TrimSpace(in.Name),
		Family:             family,
		Kind:               kind,
		Access:             access,
		Auth:               auth,
		BaseURL:            baseURL,
		Model:              strings.TrimSpace(in.Model),
		Env:                in.Env,
		WireAPI:            wireAPI,
		RequiresOpenAIAuth: in.RequiresOpenAIAuth,
	})
}

func (s *ProviderService) DeleteTarget(name string) error {
	return s.repo.DeleteTarget(name)
}

func (s *ProviderService) UseTarget(familyRaw, targetName string) (*domainprovider.Binding, error) {
	family, ok := domainprovider.ParseFamily(familyRaw)
	if !ok {
		return nil, fmt.Errorf("invalid family %q", familyRaw)
	}
	return s.repo.SetBinding(family, targetName)
}

func (s *ProviderService) ResolveTarget(family domainprovider.Family, targetName string) (*domainprovider.Target, error) {
	if !family.Valid() {
		return nil, fmt.Errorf("invalid family %q", family)
	}
	targetName = strings.TrimSpace(targetName)
	if targetName == "" {
		binding, err := s.repo.GetBinding(family)
		if err != nil {
			return nil, err
		}
		targetName = binding.Target
	}

	target, err := s.repo.GetTarget(targetName)
	if err != nil {
		return nil, err
	}
	if target.Family != family {
		return nil, fmt.Errorf("target %s belongs to family %s, not %s", target.Name, target.Family, family)
	}
	return target, nil
}

func (s *ProviderService) ResolveTargetForLaunch(provider domainkey.Provider, keyBaseURL, targetName string) (*domainprovider.Target, error) {
	family, ok := domainprovider.ParseFamily(string(provider))
	if !ok {
		return nil, fmt.Errorf("invalid provider family %q", provider)
	}
	targetName = strings.TrimSpace(targetName)
	if targetName != "" {
		return s.ResolveTarget(family, targetName)
	}

	bound, err := s.ResolveTarget(family, "")
	if err != nil {
		if baseURL := strings.TrimSpace(keyBaseURL); baseURL != "" {
			return &domainprovider.Target{
				Name:    "legacy-inline",
				Family:  family,
				Kind:    domainprovider.DefaultThirdPartyKind(family),
				Access:  domainprovider.AccessThirdParty,
				Auth:    domainprovider.AuthAPIKey,
				BaseURL: baseURL,
			}, nil
		}
		return nil, err
	}

	if baseURL := strings.TrimSpace(keyBaseURL); baseURL != "" && bound.Access == domainprovider.AccessOfficial {
		return &domainprovider.Target{
			Name:    "legacy-inline",
			Family:  family,
			Kind:    domainprovider.DefaultThirdPartyKind(family),
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: baseURL,
		}, nil
	}

	return bound, nil
}

// KeyScopeForTarget resolves the key provider/profile scope to use for a given target.
//
// By default, keys are scoped by (provider=target.family, profile=target.name for third_party, default for official).
//
// For multi-protocol gateways (e.g. NewAPI/new-api), sites like "<site>-codex", "<site>-claude" and "<site>-gemini"
// are treated as sharing a single key scope (provider=openai, profile="<site>"), so users only need to manage one key set.
//
// Legacy: "<site>" (OpenAI) + "<site>-claude"/"<site>-gemini" also share keys under profile "<site>".
func (s *ProviderService) KeyScopeForTarget(target domainprovider.Target) (provider domainkey.Provider, profile string, err error) {
	if s == nil {
		return "", "", fmt.Errorf("provider service is unavailable")
	}

	// Multi-protocol key sharing (best-effort; falls back to default scoping).
	if target.Access == domainprovider.AccessThirdParty {
		name := strings.TrimSpace(target.Name)
		base := ""
		switch target.Family {
		case domainprovider.FamilyClaude:
			if strings.HasSuffix(name, "-claude") {
				base = strings.TrimSuffix(name, "-claude")
			}
		case domainprovider.FamilyGemini:
			if strings.HasSuffix(name, "-gemini") {
				base = strings.TrimSuffix(name, "-gemini")
			}
		}
		base = strings.TrimSpace(base)
		if base != "" {
			candidates := []string{base + "-codex", base}
			for _, candidate := range candidates {
				baseTarget, err := s.repo.GetTarget(candidate)
				if err != nil || baseTarget == nil {
					continue
				}
				if baseTarget.Access != domainprovider.AccessThirdParty || baseTarget.Family != domainprovider.FamilyOpenAI {
					continue
				}
				openaiBase := strings.TrimRight(strings.TrimSpace(baseTarget.BaseURL), "/")
				siblingBase := strings.TrimRight(strings.TrimSpace(target.BaseURL), "/")
				// Guard against accidental name collisions: only share keys when the OpenAI base_url looks like
				// "<host>/v1" and the sibling base_url is exactly "<host>".
				if strings.HasSuffix(openaiBase, "/v1") {
					host := strings.TrimRight(strings.TrimSuffix(openaiBase, "/v1"), "/")
					if host != "" && siblingBase != "" && host == siblingBase {
						return domainkey.ProviderOpenAI, domainkey.NormalizeProfileName(base), nil
					}
				}
			}
		}

		if target.Family == domainprovider.FamilyOpenAI && strings.HasSuffix(name, "-codex") {
			base := strings.TrimSpace(strings.TrimSuffix(name, "-codex"))
			if base != "" {
				claudeName := base + "-claude"
				if sibling, err := s.repo.GetTarget(claudeName); err == nil && sibling != nil {
					if sibling.Access == domainprovider.AccessThirdParty && sibling.Family == domainprovider.FamilyClaude {
						openaiBase := strings.TrimRight(strings.TrimSpace(target.BaseURL), "/")
						siblingBase := strings.TrimRight(strings.TrimSpace(sibling.BaseURL), "/")
						if strings.HasSuffix(openaiBase, "/v1") {
							host := strings.TrimRight(strings.TrimSuffix(openaiBase, "/v1"), "/")
							if host != "" && siblingBase != "" && host == siblingBase {
								return domainkey.ProviderOpenAI, domainkey.NormalizeProfileName(base), nil
							}
						}
					}
				}

				geminiName := base + "-gemini"
				if sibling, err := s.repo.GetTarget(geminiName); err == nil && sibling != nil {
					if sibling.Access == domainprovider.AccessThirdParty && sibling.Family == domainprovider.FamilyGemini {
						openaiBase := strings.TrimRight(strings.TrimSpace(target.BaseURL), "/")
						siblingBase := strings.TrimRight(strings.TrimSpace(sibling.BaseURL), "/")
						if strings.HasSuffix(openaiBase, "/v1") {
							host := strings.TrimRight(strings.TrimSuffix(openaiBase, "/v1"), "/")
							if host != "" && siblingBase != "" && host == siblingBase {
								return domainkey.ProviderOpenAI, domainkey.NormalizeProfileName(base), nil
							}
						}
					}
				}
			}
		}
	}

	provider, ok := domainkey.ParseProvider(string(target.Family))
	if !ok {
		return "", "", fmt.Errorf("unsupported provider family: %s", target.Family)
	}
	profile = domainkey.DefaultProfile
	if target.Access == domainprovider.AccessThirdParty {
		profile = domainkey.NormalizeProfileName(target.Name)
	}
	return provider, profile, nil
}

func normalizeThirdPartyBaseURL(family domainprovider.Family, raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		return base
	}

	base = strings.TrimRight(base, "/")
	switch family {
	case domainprovider.FamilyOpenAI:
		if strings.HasSuffix(base, "/v1") {
			return base
		}
		return base + "/v1"
	case domainprovider.FamilyClaude, domainprovider.FamilyGemini:
		if strings.HasSuffix(base, "/v1") {
			base = strings.TrimSuffix(base, "/v1")
			base = strings.TrimRight(base, "/")
		}
		return base
	default:
		return base
	}
}
