package codexconfig

import (
	"strconv"
	"strings"

	"github.com/kiddingbaby/agx/internal/ports"
)

func upsertRootProfile(content, profileName string) string {
	if strings.TrimSpace(content) == "" {
		return "profile = " + strconv.Quote(profileName)
	}

	lines := strings.Split(content, "\n")
	firstTable := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			firstTable = i
			break
		}
	}

	prefix := append([]string(nil), lines[:firstTable]...)
	rest := lines[firstTable:]

	replaced := false
	for i, line := range prefix {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "profile") {
			if key, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(key) == "profile" {
				prefix[i] = rewriteProfileLine(line, profileName)
				replaced = true
				break
			}
		}
	}
	if !replaced {
		prefix = append(prefix, "profile = "+strconv.Quote(profileName))
	}

	out := strings.Join(prefix, "\n")
	if len(rest) > 0 {
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += strings.Join(rest, "\n")
	}
	return strings.TrimRight(out, "\n")
}


func removeRootProfile(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	firstTable := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			firstTable = i
			break
		}
	}

	prefix := make([]string, 0, firstTable)
	rest := lines[firstTable:]
	for _, line := range lines[:firstTable] {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && strings.HasPrefix(trimmed, "profile") {
			key, _, ok := parseKeyValueLine(trimmed)
			if ok && key == "profile" {
				continue
			}
		}
		prefix = append(prefix, line)
	}

	out := strings.Join(prefix, "\n")
	if len(rest) > 0 {
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += strings.Join(rest, "\n")
	}
	return strings.TrimRight(out, "\n")
}


func removeManagedRootProfile(content, rootProfile string, profiles map[string]managedProfile) string {
	if normalizeManagedRootProfile(rootProfile, profiles) == "" {
		return strings.TrimRight(content, "\n")
	}
	return removeRootProfile(content)
}


func findRootProfileName(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			return ""
		}
		body := trimmed
		if commentIndex := findCommentIndex(body); commentIndex >= 0 {
			body = strings.TrimSpace(body[:commentIndex])
		}
		key, value, ok := parseKeyValueLine(body)
		if ok && key == "profile" {
			return unquoteTomlValue(value)
		}
	}
	return ""
}


func normalizeManagedRootProfile(rootProfile string, profiles map[string]managedProfile) string {
	rootProfile = managedProfileNameFromProfileID(rootProfile)
	if rootProfile == "" {
		return ""
	}
	if _, ok := profiles[rootProfile]; !ok {
		return ""
	}
	return rootProfile
}


func normalizeCodexRootProfile(rootProfile string, managedProfiles map[string]managedProfile, unmanagedProfiles map[string]ports.CodexUnmanagedProfile) string {
	rootProfile = managedProfileNameFromProfileID(rootProfile)
	if rootProfile == "" {
		return ""
	}
	if _, ok := managedProfiles[rootProfile]; ok {
		return rootProfile
	}
	if _, ok := unmanagedProfiles[rootProfile]; ok {
		return rootProfile
	}
	return ""
}


func rewriteProfileLine(line, profileName string) string {
	commentIndex := findCommentIndex(line)
	body := line
	comment := ""
	if commentIndex >= 0 {
		body = line[:commentIndex]
		comment = line[commentIndex:]
	}

	eqIndex := strings.Index(body, "=")
	if eqIndex < 0 {
		return "profile = " + strconv.Quote(profileName)
	}

	left := body[:eqIndex+1]
	right := body[eqIndex+1:]
	spacesAfterEqLen := len(right) - len(strings.TrimLeft(right, " \t"))
	spacesAfterEq := right[:spacesAfterEqLen]
	trailingSpaces := body[len(strings.TrimRight(body, " \t")):]

	rebuilt := left + spacesAfterEq + strconv.Quote(profileName)
	if comment != "" {
		rebuilt += trailingSpaces + comment
	}
	return rebuilt
}

