package codexconfig

import (
	"strconv"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func parseCodexConfigSection(line string) (string, string) {
	switch {
	case strings.HasPrefix(line, "[model_providers."):
		name, rest, ok := parseSectionHead(line, "[model_providers.")
		if !ok || strings.HasPrefix(name, managedPrefix) {
			return "", ""
		}
		if rest == "" {
			return "legacy_provider", name
		}
		return "legacy_provider_nested", name
	case strings.HasPrefix(line, "[profiles."):
		name, rest, ok := parseSectionHead(line, "[profiles.")
		if !ok || strings.HasPrefix(name, managedPrefix) {
			return "", ""
		}
		name = domainprofile.NormalizeProfileName(name)
		if name == "" || domainprofile.ValidateProfileName(name) != nil {
			return "", ""
		}
		if rest == "" {
			return "legacy_profile", name
		}
		return "legacy_profile_nested", name
	default:
		return "", ""
	}
}


func collapseBlankLines(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}


func codexBaseURL(raw string) string {
	return domainprofile.AgentBaseURL(domainprofile.AgentCodex, raw)
}


func codexProfileName(profileName string) string {
	return managedPrefix + profileName
}


func codexProviderID(profileName string) string {
	return managedPrefix + profileName
}


func managedProfileNameFromProfileID(profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return ""
	}
	profileID = strings.TrimPrefix(profileID, managedPrefix)
	name := domainprofile.NormalizeProfileName(profileID)
	if name == "" || domainprofile.ValidateProfileName(name) != nil {
		return ""
	}
	return name
}


func parseSectionHead(line, prefix string) (string, string, bool) {
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	value := strings.TrimPrefix(line, prefix)
	value = strings.TrimSuffix(value, "]")
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}
	if value[0] == '"' {
		escaped := false
		for i := 1; i < len(value); i++ {
			switch value[i] {
			case '\\':
				if !escaped {
					escaped = true
					continue
				}
			case '"':
				if !escaped {
					head := unquoteTomlValue(value[:i+1])
					rest := strings.TrimSpace(value[i+1:])
					rest = strings.TrimPrefix(rest, ".")
					return head, rest, true
				}
			}
			escaped = false
		}
		return "", "", false
	}

	head := value
	rest := ""
	if dot := strings.IndexRune(value, '.'); dot >= 0 {
		head = strings.TrimSpace(value[:dot])
		rest = strings.TrimSpace(value[dot+1:])
	}
	if head == "" {
		return "", "", false
	}
	return unquoteTomlValue(head), rest, true
}


func parseKeyValueLine(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), true
}


func unquoteTomlValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	unquoted, err := strconv.Unquote(value)
	if err == nil {
		return unquoted
	}
	return value
}


func findCommentIndex(line string) int {
	inSingle := false
	inDouble := false
	escaped := false
	for i, r := range line {
		switch r {
		case '\\':
			if inDouble && !escaped {
				escaped = true
				continue
			}
		case '"':
			if !inSingle && !escaped {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '#':
			if !inSingle && !inDouble {
				return i
			}
		}
		escaped = false
	}
	return -1
}


type codexTables struct {
	HasModelProviders bool
	HasProfiles       bool
}


func codexTablePresence(content string) codexTables {
	var tables codexTables
	for _, line := range strings.Split(content, "\n") {
		switch strings.TrimSpace(line) {
		case "[model_providers]":
			tables.HasModelProviders = true
		case "[profiles]":
			tables.HasProfiles = true
		}
		if tables.HasModelProviders && tables.HasProfiles {
			return tables
		}
	}
	return tables
}


func extractUnmanagedProfiles(content string) map[string]ports.CodexUnmanagedProfile {
	if strings.TrimSpace(content) == "" {
		return map[string]ports.CodexUnmanagedProfile{}
	}

	providers := map[string]unmanagedProvider{}
	profiles := map[string]unmanagedProfile{}
	currentSection := ""
	currentName := ""

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			currentSection, currentName = parseCodexConfigSection(trimmed)
			continue
		}

		key, value, ok := parseKeyValueLine(trimmed)
		if !ok {
			continue
		}

		switch currentSection {
		case "legacy_provider":
			provider := providers[currentName]
			provider.ID = currentName
			switch key {
			case "name":
				provider.Name = unquoteTomlValue(value)
			case "base_url":
				provider.BaseURL = unquoteTomlValue(value)
			case "wire_api":
				provider.WireAPI = unquoteTomlValue(value)
			case "env_key":
				provider.EnvKey = unquoteTomlValue(value)
			}
			providers[currentName] = provider
		case "legacy_profile":
			profile := profiles[currentName]
			profile.Name = currentName
			switch key {
			case "model_provider":
				profile.ProviderID = unquoteTomlValue(value)
			case "model":
				profile.Model = unquoteTomlValue(value)
			case "review_model":
				profile.ReviewModel = unquoteTomlValue(value)
			}
			profiles[currentName] = profile
		}
	}

	out := map[string]ports.CodexUnmanagedProfile{}
	for name, profile := range profiles {
		if profile.Name == "" || profile.ProviderID == "" {
			continue
		}
		provider, ok := providers[profile.ProviderID]
		if !ok || strings.TrimSpace(provider.BaseURL) == "" {
			continue
		}
		provider.DerivedType = inferCodexRelayType(provider.BaseURL, provider.ID, provider.Name, provider.WireAPI)
		out[name] = ports.CodexUnmanagedProfile{
			Name:         profile.Name,
			ProviderID:   provider.ID,
			ProviderName: provider.Name,
			BaseURL:      domainprofile.NormalizeBaseURL(provider.BaseURL),
			WireAPI:      provider.WireAPI,
			EnvKey:       provider.EnvKey,
			RelayType:    provider.DerivedType,
			Model:        profile.Model,
			ReviewModel:  profile.ReviewModel,
		}
	}
	return out
}


func inferCodexRelayType(baseURL, providerID, providerName, wireAPI string) string {
	text := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(baseURL),
		strings.TrimSpace(providerID),
		strings.TrimSpace(providerName),
		strings.TrimSpace(wireAPI),
	}, " "))
	switch {
	case strings.Contains(text, "newapi"):
		return "newapi"
	case strings.Contains(text, "sub2api") || strings.Contains(text, "pincc"):
		return "sub2api"
	case strings.Contains(text, "litellm"):
		return "litellm"
	default:
		return "custom"
	}
}

