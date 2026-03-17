package provider

import (
	"fmt"
	"strings"
	"unicode"
)

var supportedFamilies = []Family{
	FamilyClaude,
	FamilyOpenAI,
	FamilyGemini,
}

var supportedKinds = []Kind{
	KindClaude,
	KindOpenAI,
	KindOpenAICompatible,
	KindGemini,
}

var supportedAccessModes = []AccessMode{
	AccessOfficial,
	AccessThirdParty,
}

var supportedAuthModes = []AuthMode{
	AuthAPIKey,
}

var supportedWireAPIs = []WireAPI{
	WireAPIResponses,
	WireAPIChatCompletions,
}

var reservedEnvKeys = map[string]struct{}{
	"ANTHROPIC_API_KEY":      {},
	"CLAUDE_API_KEY":         {},
	"OPENAI_API_KEY":         {},
	"GEMINI_API_KEY":         {},
	"GOOGLE_GEMINI_API_KEY":  {},
	"GOOGLE_API_KEY":         {},
	"ANTHROPIC_AUTH_TOKEN":   {},
	"ANTHROPIC_BASE_URL":     {},
	"OPENAI_BASE_URL":        {},
	"OPENAI_API_BASE":        {},
	"GOOGLE_GEMINI_BASE_URL": {},
	"GEMINI_BASE_URL":        {},
}

// SupportedFamilies returns a copy of allowed provider families.
func SupportedFamilies() []Family {
	out := make([]Family, len(supportedFamilies))
	copy(out, supportedFamilies)
	return out
}

// SupportedKinds returns a copy of allowed target kinds.
func SupportedKinds() []Kind {
	out := make([]Kind, len(supportedKinds))
	copy(out, supportedKinds)
	return out
}

// SupportedAccessModes returns a copy of allowed access modes.
func SupportedAccessModes() []AccessMode {
	out := make([]AccessMode, len(supportedAccessModes))
	copy(out, supportedAccessModes)
	return out
}

// SupportedAuthModes returns a copy of allowed auth modes.
func SupportedAuthModes() []AuthMode {
	out := make([]AuthMode, len(supportedAuthModes))
	copy(out, supportedAuthModes)
	return out
}

func ParseWireAPI(raw string) (WireAPI, bool) {
	candidate := WireAPI(strings.TrimSpace(strings.ToLower(raw)))
	for _, wire := range supportedWireAPIs {
		if wire == candidate {
			return wire, true
		}
	}
	return "", false
}

func ParseFamily(raw string) (Family, bool) {
	candidate := Family(strings.TrimSpace(strings.ToLower(raw)))
	for _, family := range supportedFamilies {
		if family == candidate {
			return family, true
		}
	}
	return "", false
}

func ParseKind(raw string) (Kind, bool) {
	candidate := Kind(strings.TrimSpace(strings.ToLower(raw)))
	for _, kind := range supportedKinds {
		if kind == candidate {
			return kind, true
		}
	}
	return "", false
}

func ParseAccessMode(raw string) (AccessMode, bool) {
	candidate := AccessMode(strings.TrimSpace(strings.ToLower(raw)))
	for _, mode := range supportedAccessModes {
		if mode == candidate {
			return mode, true
		}
	}
	return "", false
}

func ParseAuthMode(raw string) (AuthMode, bool) {
	candidate := AuthMode(strings.TrimSpace(strings.ToLower(raw)))
	for _, mode := range supportedAuthModes {
		if mode == candidate {
			return mode, true
		}
	}
	return "", false
}

func (f Family) Valid() bool {
	_, ok := ParseFamily(string(f))
	return ok
}

func (k Kind) Valid() bool {
	_, ok := ParseKind(string(k))
	return ok
}

func (m AccessMode) Valid() bool {
	_, ok := ParseAccessMode(string(m))
	return ok
}

func (m AuthMode) Valid() bool {
	_, ok := ParseAuthMode(string(m))
	return ok
}

func (w WireAPI) Valid() bool {
	_, ok := ParseWireAPI(string(w))
	return ok
}

// DefaultTargetName returns the built-in official target name for a family.
func DefaultTargetName(family Family) string {
	return fmt.Sprintf("%s-official", family)
}

// DefaultKindForFamily returns the canonical official kind for a family.
func DefaultKindForFamily(family Family) Kind {
	switch family {
	case FamilyOpenAI:
		return KindOpenAI
	case FamilyClaude:
		return KindClaude
	case FamilyGemini:
		return KindGemini
	default:
		return ""
	}
}

// DefaultThirdPartyKind returns the default third-party kind for legacy fallback.
func DefaultThirdPartyKind(family Family) Kind {
	if family == FamilyOpenAI {
		return KindOpenAICompatible
	}
	return DefaultKindForFamily(family)
}

// DefaultTargets returns built-in official targets.
func DefaultTargets() []Target {
	return []Target{
		{
			Name:   DefaultTargetName(FamilyClaude),
			Family: FamilyClaude,
			Kind:   KindClaude,
			Access: AccessOfficial,
			Auth:   AuthAPIKey,
		},
		{
			Name:   DefaultTargetName(FamilyOpenAI),
			Family: FamilyOpenAI,
			Kind:   KindOpenAI,
			Access: AccessOfficial,
			Auth:   AuthAPIKey,
		},
		{
			Name:   DefaultTargetName(FamilyGemini),
			Family: FamilyGemini,
			Kind:   KindGemini,
			Access: AccessOfficial,
			Auth:   AuthAPIKey,
		},
	}
}

// CompatibleKind reports whether a target kind can be used by the family.
func CompatibleKind(family Family, kind Kind) bool {
	switch family {
	case FamilyOpenAI:
		return kind == KindOpenAI || kind == KindOpenAICompatible
	case FamilyClaude:
		return kind == KindClaude
	case FamilyGemini:
		return kind == KindGemini
	default:
		return false
	}
}

// ValidateTarget validates a provider target definition.
func ValidateTarget(target Target) error {
	target.Name = strings.TrimSpace(target.Name)
	target.BaseURL = strings.TrimSpace(target.BaseURL)
	target.Model = strings.TrimSpace(target.Model)
	target.WireAPI = WireAPI(strings.TrimSpace(strings.ToLower(string(target.WireAPI))))

	if target.Name == "" {
		return fmt.Errorf("target name is required")
	}
	if strings.IndexFunc(target.Name, unicode.IsSpace) >= 0 {
		return fmt.Errorf("target name cannot contain whitespace")
	}
	if !target.Family.Valid() {
		return fmt.Errorf("invalid family %q", target.Family)
	}
	if !target.Kind.Valid() {
		return fmt.Errorf("invalid kind %q", target.Kind)
	}
	if !CompatibleKind(target.Family, target.Kind) {
		return fmt.Errorf("kind %q is not compatible with family %q", target.Kind, target.Family)
	}
	if !target.Access.Valid() {
		return fmt.Errorf("invalid access %q", target.Access)
	}
	if !target.Auth.Valid() {
		return fmt.Errorf("invalid auth %q", target.Auth)
	}
	if target.Kind == KindOpenAICompatible && target.Access != AccessThirdParty {
		return fmt.Errorf("openai-compatible targets must use third_party access")
	}
	if target.Access == AccessOfficial && target.BaseURL != "" {
		return fmt.Errorf("official targets cannot set base-url")
	}
	if target.Access == AccessThirdParty && target.BaseURL == "" {
		return fmt.Errorf("third_party targets require base-url")
	}
	if strings.ContainsAny(target.BaseURL, "\x00\n\r") {
		return fmt.Errorf("base-url contains invalid characters")
	}
	if strings.ContainsAny(target.Model, "\x00\n\r") {
		return fmt.Errorf("model contains invalid characters")
	}

	if target.WireAPI != "" && target.Family != FamilyOpenAI {
		return fmt.Errorf("wire-api is only supported for openai family targets")
	}
	if target.WireAPI != "" && !target.WireAPI.Valid() {
		return fmt.Errorf("invalid wire-api %q", target.WireAPI)
	}
	if target.RequiresOpenAIAuth != nil && target.Family != FamilyOpenAI {
		return fmt.Errorf("requires-openai-auth is only supported for openai family targets")
	}
	if err := ValidateEnv(target.Env); err != nil {
		return err
	}
	return nil
}

// ValidateEnv validates provider-level env overrides. Values are treated as plain strings.
//
// NOTE: This env map is persisted in providers.yaml (not encrypted). API keys and base-url related env vars are
// reserved and must be configured via keys.yaml / target.base-url instead.
func ValidateEnv(env map[string]string) error {
	for k, v := range env {
		key := strings.TrimSpace(k)
		if key == "" {
			return fmt.Errorf("env key cannot be empty")
		}
		if key != k {
			return fmt.Errorf("env key contains leading/trailing whitespace: %q", k)
		}
		if _, blocked := reservedEnvKeys[key]; blocked {
			return fmt.Errorf("env key %q is reserved by agx", key)
		}
		if !isValidEnvKey(key) {
			return fmt.Errorf("invalid env key %q", key)
		}
		if strings.ContainsAny(v, "\x00\n\r") {
			return fmt.Errorf("env value for %q contains invalid characters", key)
		}
	}
	return nil
}

func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
