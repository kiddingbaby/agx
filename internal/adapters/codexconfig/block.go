package codexconfig

import (
	"sort"
	"strconv"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func stripManagedBlock(content, configPath string) (string, error) {
	for {
		start := strings.Index(content, beginMarker)
		if start < 0 {
			return content, nil
		}
		end := strings.Index(content[start:], endMarker)
		if end < 0 {
			return "", &ports.IncompleteManagedBlockError{
				Agent:      domainprofile.AgentCodex,
				ConfigPath: configPath,
			}
		}
		end += start + len(endMarker)
		if end < len(content) && content[end] == '\n' {
			end++
		}

		before := strings.TrimRight(content[:start], "\n")
		after := strings.TrimLeft(content[end:], "\n")
		switch {
		case before == "":
			content = after
		case after == "":
			content = before
		default:
			content = before + "\n\n" + after
		}
	}
}


func validateManagedBlock(content string) error {
	_, err := stripManagedBlock(content, "")
	return err
}


func removeLegacyManagedSections(content string, profiles map[string]managedProfile) string {
	if strings.TrimSpace(content) == "" || len(profiles) == 0 {
		return strings.TrimRight(content, "\n")
	}

	legacyProfiles := map[string]struct{}{}
	for name := range profiles {
		legacyProfiles[name] = struct{}{}
	}

	profileProviders := map[string]string{}
	providerBaseURLs := map[string]string{}
	seenProviders := map[string]struct{}{}
	referencedByUnmanagedProfile := map[string]struct{}{}

	currentSection := ""
	currentName := ""
	currentLegacyProfile := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			currentSection, currentName = parseCodexConfigSection(trimmed)
			currentLegacyProfile = ""
			if currentSection == "legacy_profile" {
				if _, ok := legacyProfiles[currentName]; ok {
					currentLegacyProfile = currentName
				}
			}
			continue
		}

		switch currentSection {
		case "legacy_provider":
			seenProviders[currentName] = struct{}{}
			key, value, ok := parseKeyValueLine(trimmed)
			if ok && key == "base_url" {
				providerBaseURLs[currentName] = domainprofile.NormalizeBaseURL(unquoteTomlValue(value))
			}
		case "legacy_profile":
			key, value, ok := parseKeyValueLine(trimmed)
			if !ok || key != "model_provider" {
				continue
			}
			providerID := unquoteTomlValue(value)
			if currentLegacyProfile != "" {
				profileProviders[currentLegacyProfile] = providerID
			} else {
				referencedByUnmanagedProfile[providerID] = struct{}{}
			}
		}
	}

	removeProfiles := map[string]struct{}{}
	removeProviders := map[string]struct{}{}
	for profileName, providerID := range profileProviders {
		if providerID == "" {
			removeProfiles[profileName] = struct{}{}
			continue
		}
		if _, referenced := referencedByUnmanagedProfile[providerID]; referenced {
			continue
		}
		if _, ok := seenProviders[providerID]; !ok {
			removeProfiles[profileName] = struct{}{}
			continue
		}
		providerBaseURL := providerBaseURLs[providerID]
		if providerBaseURL == "" {
			continue
		}
		profileBaseURL := domainprofile.NormalizeBaseURL(profiles[profileName].BaseURL)
		profileCodexBaseURL := codexBaseURL(profileBaseURL)
		if providerBaseURL == profileBaseURL || providerBaseURL == profileCodexBaseURL {
			removeProfiles[profileName] = struct{}{}
			removeProviders[providerID] = struct{}{}
		}
	}

	var kept []string
	skipping := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section, name := parseCodexConfigSection(trimmed)
			skipping = false
			switch section {
			case "legacy_profile", "legacy_profile_nested":
				_, skipping = removeProfiles[name]
			case "legacy_provider", "legacy_provider_nested":
				_, skipping = removeProviders[name]
			}
		}
		if skipping {
			continue
		}
		kept = append(kept, line)
	}

	return strings.TrimRight(collapseBlankLines(strings.Join(kept, "\n")), "\n")
}


func appendManagedBlock(content, block string) string {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return block + "\n"
	}
	return content + "\n\n" + block + "\n"
}


func renderManagedBlock(profiles map[string]managedProfile, helperCommand string, tables codexTables) string {
	var b strings.Builder
	b.WriteString(beginMarker)
	b.WriteString("\n")
	b.WriteString("# AGX rewrites only this block.\n")
	b.WriteString("# The rest of this file stays in your original order.\n")
	b.WriteString("# Managed profiles: ")
	b.WriteString(strings.Join(sortedManagedProfileNames(profiles), ", "))
	b.WriteString("\n\n")
	if !tables.HasModelProviders {
		b.WriteString("[model_providers]\n\n")
	}
	if !tables.HasProfiles {
		b.WriteString("[profiles]\n\n")
	}

	for index, name := range sortedManagedProfileNames(profiles) {
		profile := profiles[name]
		profileID := codexProfileName(profile.Name)
		providerID := codexProviderID(profile.Name)

		b.WriteString("[model_providers.")
		b.WriteString(strconv.Quote(providerID))
		b.WriteString("]\n")
		b.WriteString("name = ")
		b.WriteString(strconv.Quote(profile.Name))
		b.WriteString("\n")
		b.WriteString("base_url = ")
		b.WriteString(strconv.Quote(codexBaseURL(profile.BaseURL)))
		b.WriteString("\n")
		wireAPI := strings.TrimSpace(profile.WireAPI)
		if wireAPI == "" {
			wireAPI = string(domainprofile.CodexWireAPIResponses)
		}
		b.WriteString("wire_api = ")
		b.WriteString(strconv.Quote(wireAPI))
		b.WriteString("\n\n")
		b.WriteString("[model_providers.")
		b.WriteString(strconv.Quote(providerID))
		b.WriteString(".auth]\n")
		b.WriteString("command = ")
		b.WriteString(strconv.Quote(helperCommand))
		b.WriteString("\n")
		b.WriteString("args = [")
		b.WriteString(strconv.Quote("__api-key"))
		b.WriteString(", ")
		b.WriteString(strconv.Quote(profile.Name))
		b.WriteString("]\n\n")
		b.WriteString("[profiles.")
		b.WriteString(strconv.Quote(profileID))
		b.WriteString("]\n")
		b.WriteString("model_provider = ")
		b.WriteString(strconv.Quote(providerID))
		b.WriteString("\n")
		for _, line := range profile.Extras {
			if line == "" {
				b.WriteString("\n")
				continue
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		if index < len(profiles)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString(endMarker)
	return b.String()
}


func extractManagedBlock(content string) (string, bool) {
	start := strings.Index(content, beginMarker)
	if start < 0 {
		return "", false
	}
	end := strings.Index(content[start:], endMarker)
	if end < 0 {
		return content[start:], true
	}
	end += start + len(endMarker)
	return content[start:end], true
}


func extractManagedProfiles(content string) map[string]managedProfile {
	block, ok := extractManagedBlock(content)
	if !ok {
		return map[string]managedProfile{}
	}

	lines := strings.Split(block, "\n")
	profiles := map[string]managedProfile{}
	providers := map[string]string{}
	currentName := ""
	currentSection := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == beginMarker || trimmed == endMarker {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			currentSection, currentName = parseManagedSection(trimmed)
			if currentName == "" {
				currentSection = ""
				continue
			}
			profile := profiles[currentName]
			profile.Name = currentName
			switch currentSection {
			case "provider":
				providers[codexProviderID(currentName)] = currentName
			case "profile", "profile_nested":
				if profile.ProfileID == "" {
					profile.ProfileID = parseManagedProfileID(trimmed)
				}
				if currentSection == "profile_nested" {
					profile.Extras = append(profile.Extras, strings.TrimRight(line, " \t"))
					currentSection = "profile"
				}
			}
			profiles[currentName] = profile
			continue
		}
		if currentName == "" {
			continue
		}
		profile := profiles[currentName]
		switch currentSection {
		case "provider":
			key, value, ok := parseKeyValueLine(trimmed)
			if !ok {
				continue
			}
			switch key {
			case "base_url":
				profile.BaseURL = unquoteTomlValue(value)
			case "wire_api":
				profile.WireAPI = unquoteTomlValue(value)
			}
		case "profile":
			if trimmed == "" {
				if len(profile.Extras) > 0 && profile.Extras[len(profile.Extras)-1] != "" {
					profile.Extras = append(profile.Extras, "")
				}
				profiles[currentName] = profile
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				profile.Extras = append(profile.Extras, strings.TrimRight(line, " \t"))
				profiles[currentName] = profile
				continue
			}
			key, value, ok := parseKeyValueLine(trimmed)
			if !ok || strings.TrimSpace(key) == "model_provider" {
				if ok && strings.TrimSpace(key) == "model_provider" {
					profile.ProfileProvider = unquoteTomlValue(value)
				}
				profiles[currentName] = profile
				continue
			}
			profile.Extras = append(profile.Extras, strings.TrimRight(line, " \t"))
		}
		profiles[currentName] = profile
	}

	for name, profile := range profiles {
		if profile.Name == "" || profile.BaseURL == "" {
			delete(profiles, name)
			continue
		}
		for len(profile.Extras) > 0 && profile.Extras[0] == "" {
			profile.Extras = profile.Extras[1:]
		}
		for len(profile.Extras) > 0 && profile.Extras[len(profile.Extras)-1] == "" {
			profile.Extras = profile.Extras[:len(profile.Extras)-1]
		}
		profiles[name] = profile
	}

	for name, profile := range profiles {
		providerName, ok := providers[profile.ProfileProvider]
		if profile.ProfileProvider == "" || !ok || providerName != name {
			delete(profiles, name)
			continue
		}
	}
	return profiles
}


func parseManagedSection(line string) (string, string) {
	switch {
	case strings.HasPrefix(line, "[model_providers."):
		name, rest, ok := parseSectionHead(line, "[model_providers.")
		if !ok || !strings.HasPrefix(name, managedPrefix) {
			return "", ""
		}
		name = strings.TrimPrefix(name, managedPrefix)
		if name == "" || domainprofile.ValidateProfileName(name) != nil {
			return "", ""
		}
		if rest == "auth" {
			return "", ""
		}
		if rest != "" {
			return "", ""
		}
		return "provider", name
	case strings.HasPrefix(line, "[profiles."):
		name, rest, ok := parseSectionHead(line, "[profiles.")
		if !ok {
			return "", ""
		}
		name = managedProfileNameFromProfileID(name)
		if name == "" {
			return "", ""
		}
		if rest != "" {
			return "profile_nested", name
		}
		return "profile", name
	default:
		return "", ""
	}
}


func parseManagedProfileID(line string) string {
	name, _, ok := parseSectionHead(line, "[profiles.")
	if !ok {
		return ""
	}
	return name
}


func sortedManagedProfileNames(profiles map[string]managedProfile) []string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}


func mergeModelIntoExtras(extras []string, modelID string) []string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		// 沿用 Extras 中已有 model 行 (典型来自 root profile carry-over)
		return extras
	}
	out := make([]string, 0, len(extras)+1)
	for _, line := range extras {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			continue
		}
		key, _, ok := parseKeyValueLine(trimmed)
		if ok && strings.TrimSpace(key) == "model" {
			continue
		}
		out = append(out, line)
	}
	modelLine := "model = " + strconv.Quote(modelID)
	merged := make([]string, 0, len(out)+1)
	merged = append(merged, modelLine)
	merged = append(merged, out...)
	return merged
}


func selectManagedProfileExtras(content string, profiles map[string]managedProfile) []string {
	rootProfile := normalizeManagedRootProfile(findRootProfileName(content), profiles)
	if rootProfile != "" {
		if profile, ok := profiles[rootProfile]; ok && len(profile.Extras) > 0 {
			return append([]string(nil), profile.Extras...)
		}
	}
	if len(profiles) == 1 {
		for _, profile := range profiles {
			if len(profile.Extras) > 0 {
				return append([]string(nil), profile.Extras...)
			}
		}
	}
	return nil
}

