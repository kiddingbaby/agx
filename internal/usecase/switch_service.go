package usecase

import (
	"fmt"
	"strings"

	domainagent "github.com/kiddingbaby/agx/internal/domain/agent"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/ports"
)

// SwitchService switches the active target binding (by name) and syncs the resolved config into native CLI configs.
//
// It does NOT launch or manage sessions.
type SwitchService struct {
	keySvc      *KeyService
	providerSvc *ProviderService
	syncer      ports.ToolConfigSyncer
	undoStore   ports.UndoStore
}

type SwitchApplied struct {
	Agent   domainagent.Agent
	Target  domainprovider.Target
	Key     domainkey.Key
	Profile string
}

type SwitchResult struct {
	Primary SwitchApplied
	Applied []SwitchApplied
}

func NewSwitchService(keySvc *KeyService, providerSvc *ProviderService, syncer ports.ToolConfigSyncer, undoStore ports.UndoStore) *SwitchService {
	return &SwitchService{
		keySvc:      keySvc,
		providerSvc: providerSvc,
		syncer:      syncer,
		undoStore:   undoStore,
	}
}

type SwitchOptions struct {
	// KeyIdentifier activates and uses the matching key in the computed scope (provider/profile).
	KeyIdentifier string
	// KeyTags selects an active key by tags (AND match) within the computed scope (provider/profile).
	// When multiple keys match and none is active, the switch fails with an ambiguity error.
	KeyTags []string
	// Families selects which agent families to sync together when the site supports multi-protocol siblings
	// (e.g. "<site>" + "<site>-claude" + "<site>-gemini"). If empty, only the requested site is synced.
	Families []domainprovider.Family
	// DryRun resolves targets/keys without mutating stores or writing native configs.
	DryRun bool
}

func (s *SwitchService) SwitchByName(name string, opts SwitchOptions) (*SwitchResult, error) {
	if s.keySvc == nil || s.providerSvc == nil || s.syncer == nil || s.undoStore == nil {
		return nil, fmt.Errorf("switch service is unavailable")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	group, err := s.resolveSwitchTargets(name, opts.Families)
	if err != nil {
		return nil, err
	}
	primary := group.Primary

	provider, profile, err := s.providerSvc.KeyScopeForTarget(primary)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(opts.KeyIdentifier) != "" && len(opts.KeyTags) > 0 {
		return nil, fmt.Errorf("--key and -l selector cannot be used together")
	}

	if !hasAnyKeyInScope(s.keySvc.List(), provider, profile) {
		return nil, &NoActiveKeyError{Provider: string(provider), Profile: profile}
	}

	primaryAgent, ok := agentForFamily(primary.Family)
	if !ok {
		return nil, fmt.Errorf("no agent mapped for family %s", primary.Family)
	}

	rollback := func() {}
	if !opts.DryRun {
		undoID, err := s.undoStore.Capture(ports.UndoCaptureMeta{
			Command: "switch",
			Agent:   primaryAgent,
			Target:  primary,
		})
		if err != nil {
			return nil, fmt.Errorf("capture undo point: %w", err)
		}
		rollback = func() {
			_, _ = s.undoStore.Restore(undoID)
		}
	}

	var active *domainkey.Key
	if keyIdent := strings.TrimSpace(opts.KeyIdentifier); keyIdent != "" {
		if opts.DryRun {
			active, err = s.keySvc.PreviewResolve(provider, profile, keyIdent)
			if err != nil {
				return nil, err
			}
		} else {
			if _, err := s.keySvc.ActivateByIdentifierInScope(provider, profile, keyIdent); err != nil {
				rollback()
				return nil, err
			}
			active, err = s.keySvc.GetActive(provider, profile)
			if err != nil {
				rollback()
				return nil, &NoActiveKeyError{Provider: string(provider), Profile: profile}
			}
		}
	} else if len(opts.KeyTags) > 0 {
		matches := listKeysMatchingTagsInScope(s.keySvc.List(), provider, profile, opts.KeyTags)
		switch len(matches) {
		case 0:
			return nil, &KeySelectorNoMatchError{Provider: string(provider), Profile: profile, Tags: opts.KeyTags}
		case 1:
			if opts.DryRun {
				active, err = s.keySvc.PreviewResolve(provider, profile, matches[0].ID)
				if err != nil {
					return nil, err
				}
			} else {
				if err := s.keySvc.Activate(matches[0].ID); err != nil {
					rollback()
					return nil, err
				}
				active, err = s.keySvc.GetActive(provider, profile)
				if err != nil {
					rollback()
					return nil, err
				}
			}
		default:
			if activeKey, err := s.keySvc.GetActive(provider, profile); err == nil && activeKey != nil && keyHasAllTags(activeKey.Tags, opts.KeyTags) {
				active = activeKey
			} else {
				labels := make([]string, 0, len(matches))
				for _, k := range matches {
					if strings.TrimSpace(k.Name) != "" {
						labels = append(labels, k.Name)
						continue
					}
					labels = append(labels, k.ID)
				}
				return nil, &KeySelectorAmbiguousError{Provider: string(provider), Profile: profile, Tags: opts.KeyTags, Matches: labels}
			}
		}
	} else {
		if opts.DryRun {
			active, err = s.keySvc.PreviewResolve(provider, profile, "")
			if err != nil {
				return nil, err
			}
		} else {
			active, err = s.keySvc.Resolve(provider, profile, "")
			if err != nil {
				rollback()
				if !hasAnyKeyInScope(s.keySvc.List(), provider, profile) {
					return nil, &NoActiveKeyError{Provider: string(provider), Profile: profile}
				}
				return nil, err
			}
		}
	}

	if !opts.DryRun {
		if err := s.providerSvc.SetCurrentSite(primary.Name); err != nil {
			rollback()
			return nil, err
		}
	}

	applied := make([]SwitchApplied, 0, len(group.Targets))
	for _, target := range group.Targets {
		agent, ok := agentForFamily(target.Family)
		if !ok {
			rollback()
			return nil, fmt.Errorf("no agent mapped for family %s", target.Family)
		}

		keyToUse := active
		keyProvider, keyProfile, scopeErr := s.providerSvc.KeyScopeForTarget(target)
		if scopeErr != nil {
			return nil, scopeErr
		}
		if keyProvider != provider || domainkey.NormalizeProfileName(keyProfile) != domainkey.NormalizeProfileName(profile) {
			if hasAnyKeyInScope(s.keySvc.List(), keyProvider, keyProfile) {
				if opts.DryRun {
					k, err := s.keySvc.PreviewResolve(keyProvider, keyProfile, "")
					if err != nil {
						return nil, err
					}
					keyToUse = k
				} else {
					k, err := s.keySvc.Resolve(keyProvider, keyProfile, "")
					if err != nil {
						rollback()
						return nil, err
					}
					keyToUse = k
				}
			}
		}

		profileForSync := keyProfile
		if target.Access == domainprovider.AccessThirdParty {
			profileForSync = domainkey.NormalizeProfileName(profileForSync)
		} else {
			profileForSync = domainkey.DefaultProfile
		}

		if !opts.DryRun {
			if _, err := s.providerSvc.UseTarget(string(target.Family), target.Name); err != nil {
				rollback()
				return nil, err
			}
			if err := s.syncer.Apply(agent, *keyToUse, target); err != nil {
				rollback()
				return nil, err
			}
		}

		applied = append(applied, SwitchApplied{
			Agent:   agent,
			Target:  target,
			Key:     *keyToUse,
			Profile: profileForSync,
		})
	}

	res := SwitchResult{Applied: applied}
	if len(applied) > 0 {
		res.Primary = applied[0]
	}
	return &res, nil
}

func (s *SwitchService) UndoLatest() (ports.UndoRestoreResult, error) {
	if s.undoStore == nil {
		return ports.UndoRestoreResult{}, fmt.Errorf("undo store is unavailable")
	}
	id, err := s.undoStore.LatestID()
	if err != nil {
		return ports.UndoRestoreResult{}, err
	}
	return s.undoStore.Restore(id)
}

func (s *SwitchService) resolveTargetByName(name string) (*domainprovider.Target, error) {
	target, err := s.providerSvc.GetTarget(name)
	if err == nil && target != nil {
		return target, nil
	}

	// Convenience: `agx use openai|claude|gemini` maps to the built-in official target.
	if family, ok := domainprovider.ParseFamily(name); ok {
		return s.providerSvc.GetTarget(domainprovider.DefaultTargetName(family))
	}
	return nil, err
}

type switchTargetGroup struct {
	Primary domainprovider.Target
	Targets []domainprovider.Target
}

func (s *SwitchService) resolveSwitchTargets(name string, families []domainprovider.Family) (switchTargetGroup, error) {
	anchor, err := s.resolveTargetByName(name)
	if err != nil {
		return switchTargetGroup{}, err
	}
	if anchor == nil {
		return switchTargetGroup{}, fmt.Errorf("site not found: %s", name)
	}

	if len(families) == 0 {
		primary := *anchor
		return switchTargetGroup{Primary: primary, Targets: []domainprovider.Target{primary}}, nil
	}

	selected, err := normalizeSwitchFamilies(families)
	if err != nil {
		return switchTargetGroup{}, err
	}
	if _, ok := selected[anchor.Family]; !ok {
		return switchTargetGroup{}, fmt.Errorf("agents must include %s for site %s", anchor.Family, anchor.Name)
	}
	if len(selected) == 1 {
		primary := *anchor
		return switchTargetGroup{Primary: primary, Targets: []domainprovider.Target{primary}}, nil
	}

	openaiPrimary, ok := s.resolveUniversalGatewayPrimary(*anchor)
	if !ok {
		return switchTargetGroup{}, fmt.Errorf("site does not support multi-agent sync: %s", anchor.Name)
	}

	prefix, ok := universalGatewayGroupPrefix(openaiPrimary)
	if !ok {
		return switchTargetGroup{}, fmt.Errorf("site does not support multi-agent sync: %s", anchor.Name)
	}

	byFamily := map[domainprovider.Family]domainprovider.Target{}
	byFamily[domainprovider.FamilyOpenAI] = openaiPrimary
	if t, ok := s.tryGetSiblingTarget(prefix+"-claude", domainprovider.FamilyClaude); ok && isUniversalGatewaySibling(openaiPrimary, t) {
		byFamily[domainprovider.FamilyClaude] = t
	}
	if t, ok := s.tryGetSiblingTarget(prefix+"-gemini", domainprovider.FamilyGemini); ok && isUniversalGatewaySibling(openaiPrimary, t) {
		byFamily[domainprovider.FamilyGemini] = t
	}

	primary := *anchor
	targets := []domainprovider.Target{primary}
	for _, family := range []domainprovider.Family{domainprovider.FamilyOpenAI, domainprovider.FamilyClaude, domainprovider.FamilyGemini} {
		if family == primary.Family {
			continue
		}
		if _, want := selected[family]; !want {
			continue
		}
		t, ok := byFamily[family]
		if !ok {
			return switchTargetGroup{}, fmt.Errorf("site %s does not have agent %s configured", anchor.Name, family)
		}
		targets = append(targets, t)
	}

	return switchTargetGroup{Primary: primary, Targets: targets}, nil
}

func (s *SwitchService) tryGetSiblingTarget(name string, family domainprovider.Family) (domainprovider.Target, bool) {
	t, err := s.providerSvc.GetTarget(name)
	if err != nil || t == nil {
		return domainprovider.Target{}, false
	}
	if t.Family != family {
		return domainprovider.Target{}, false
	}
	return *t, true
}

func normalizeSwitchFamilies(families []domainprovider.Family) (map[domainprovider.Family]struct{}, error) {
	set := make(map[domainprovider.Family]struct{}, len(families))
	for _, f := range families {
		if !f.Valid() {
			return nil, fmt.Errorf("invalid agent family: %s", f)
		}
		set[f] = struct{}{}
	}
	return set, nil
}

func (s *SwitchService) resolveUniversalGatewayPrimary(anchor domainprovider.Target) (domainprovider.Target, bool) {
	switch {
	case anchor.Access == domainprovider.AccessThirdParty && anchor.Family == domainprovider.FamilyOpenAI:
		return anchor, true
	case anchor.Access == domainprovider.AccessThirdParty && anchor.Family == domainprovider.FamilyClaude && strings.HasSuffix(anchor.Name, "-claude"):
		base := strings.TrimSuffix(anchor.Name, "-claude")
		if strings.TrimSpace(base) == "" {
			return domainprovider.Target{}, false
		}
		candidates := []string{base + "-codex", base}
		for _, candidate := range candidates {
			t, err := s.providerSvc.GetTarget(candidate)
			if err == nil && t != nil && t.Access == domainprovider.AccessThirdParty && t.Family == domainprovider.FamilyOpenAI {
				if isUniversalGatewaySibling(*t, anchor) {
					return *t, true
				}
			}
		}
	case anchor.Access == domainprovider.AccessThirdParty && anchor.Family == domainprovider.FamilyGemini && strings.HasSuffix(anchor.Name, "-gemini"):
		base := strings.TrimSuffix(anchor.Name, "-gemini")
		if strings.TrimSpace(base) == "" {
			return domainprovider.Target{}, false
		}
		candidates := []string{base + "-codex", base}
		for _, candidate := range candidates {
			t, err := s.providerSvc.GetTarget(candidate)
			if err == nil && t != nil && t.Access == domainprovider.AccessThirdParty && t.Family == domainprovider.FamilyOpenAI {
				if isUniversalGatewaySibling(*t, anchor) {
					return *t, true
				}
			}
		}
	}
	return domainprovider.Target{}, false
}

func universalGatewayGroupPrefix(openaiPrimary domainprovider.Target) (string, bool) {
	if openaiPrimary.Access != domainprovider.AccessThirdParty || openaiPrimary.Family != domainprovider.FamilyOpenAI {
		return "", false
	}
	name := strings.TrimSpace(openaiPrimary.Name)
	if name == "" {
		return "", false
	}
	if strings.HasSuffix(name, "-codex") {
		prefix := strings.TrimSpace(strings.TrimSuffix(name, "-codex"))
		if prefix == "" {
			return "", false
		}
		return prefix, true
	}
	return name, true
}

func isUniversalGatewaySibling(openaiTarget domainprovider.Target, sibling domainprovider.Target) bool {
	if openaiTarget.Family != domainprovider.FamilyOpenAI {
		return false
	}
	openaiBase := strings.TrimRight(strings.TrimSpace(openaiTarget.BaseURL), "/")
	siblingBase := strings.TrimRight(strings.TrimSpace(sibling.BaseURL), "/")
	if !strings.HasSuffix(openaiBase, "/v1") {
		return false
	}
	host := strings.TrimRight(strings.TrimSuffix(openaiBase, "/v1"), "/")
	if host == "" || siblingBase == "" {
		return false
	}
	return host == siblingBase
}

func agentForFamily(family domainprovider.Family) (domainagent.Agent, bool) {
	switch family {
	case domainprovider.FamilyOpenAI:
		return domainagent.Find("codex-cli")
	case domainprovider.FamilyClaude:
		return domainagent.Find("claude-code")
	case domainprovider.FamilyGemini:
		return domainagent.Find("gemini-cli")
	default:
		return domainagent.Agent{}, false
	}
}

func hasAnyKeyInScope(keys []domainkey.Key, provider domainkey.Provider, profile string) bool {
	profile = domainkey.NormalizeProfileName(profile)
	for _, k := range keys {
		if k.Provider != provider {
			continue
		}
		if domainkey.NormalizeProfileName(k.Profile) != profile {
			continue
		}
		return true
	}
	return false
}

func keyHasAllTags(tags []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		set[t] = struct{}{}
	}
	for _, r := range required {
		if _, ok := set[r]; !ok {
			return false
		}
	}
	return true
}

func listKeysMatchingTagsInScope(keys []domainkey.Key, provider domainkey.Provider, profile string, requiredTags []string) []domainkey.Key {
	profile = domainkey.NormalizeProfileName(profile)
	out := make([]domainkey.Key, 0)
	for _, k := range keys {
		if k.Provider != provider {
			continue
		}
		if domainkey.NormalizeProfileName(k.Profile) != profile {
			continue
		}
		if !keyHasAllTags(k.Tags, requiredTags) {
			continue
		}
		out = append(out, k)
	}
	return out
}
