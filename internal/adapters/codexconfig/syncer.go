package codexconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

const (
	beginMarker = "# >>> AGX managed Codex config >>>"
	endMarker   = "# <<< AGX managed Codex config <<<"
)

var _ ports.CodexSyncer = (*Syncer)(nil)

type Syncer struct {
	configPath    string
	backupsDir    string
	helperCommand string
}

type managedProfile struct {
	Name    string
	BaseURL string
	Extras  []string
}

func NewSyncer(configPath, backupsDir, helperCommand string) *Syncer {
	return &Syncer{
		configPath:    configPath,
		backupsDir:    filepath.Join(backupsDir, "codex"),
		helperCommand: helperCommand,
	}
}

func (s *Syncer) Sync(profile domainprofile.Profile, options ports.CodexSyncOptions) (*ports.CodexSyncResult, error) {
	if strings.TrimSpace(s.helperCommand) == "" {
		return nil, fmt.Errorf("codex sync requires AGX to be installed or run from a stable binary path")
	}

	existing, _, err := readIfExists(s.configPath)
	if err != nil {
		return nil, err
	}

	managedProfiles := extractManagedProfiles(existing)
	entry := managedProfiles[profile.Name]
	entry.Name = profile.Name
	entry.BaseURL = profile.BaseURL
	if len(entry.Extras) == 0 {
		entry.Extras = selectManagedProfileExtras(existing, managedProfiles)
	}
	managedProfiles[profile.Name] = entry

	unmanaged := stripManagedBlock(existing)
	next := unmanaged
	if strings.TrimSpace(options.DefaultProfileName) != "" {
		next = upsertRootProfile(next, codexProfileName(options.DefaultProfileName))
	}
	next = appendManagedBlock(next, renderManagedBlock(managedProfiles, s.helperCommand, codexTablePresence(next)))

	if err := fileutil.AtomicWriteFile(s.configPath, []byte(next), 0600); err != nil {
		return nil, err
	}

	return &ports.CodexSyncResult{
		ProfileName: codexProfileName(profile.Name),
		ConfigPath:  s.configPath,
	}, nil
}

func (s *Syncer) Status() (*ports.CodexConfigStatus, error) {
	content, exists, err := readIfExists(s.configPath)
	if err != nil {
		return nil, err
	}

	status := &ports.CodexConfigStatus{
		ConfigPath:          s.configPath,
		ManagedProfilesByID: map[string]ports.CodexManagedProfile{},
	}
	if !exists {
		return status, nil
	}

	managedProfiles := extractManagedProfiles(content)
	for name, profile := range managedProfiles {
		status.ManagedProfilesByID[name] = ports.CodexManagedProfile{
			Name:    profile.Name,
			BaseURL: profile.BaseURL,
		}
	}
	status.DefaultProfileName = normalizeManagedRootProfile(findRootProfileName(content), managedProfiles)
	return status, nil
}

func (s *Syncer) Restore(backupPath string) (string, error) {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return "", err
	}
	if err := fileutil.AtomicWriteFile(s.configPath, data, 0600); err != nil {
		return "", err
	}
	return s.configPath, nil
}

func (s *Syncer) Snapshot() (*ports.AgentConfigSnapshot, error) {
	content, exists, err := readIfExists(s.configPath)
	if err != nil {
		return nil, err
	}
	return &ports.AgentConfigSnapshot{
		ConfigPath: s.configPath,
		Exists:     exists,
		Content:    []byte(content),
	}, nil
}

func (s *Syncer) CreateBackup(id string, content []byte) (string, error) {
	if err := os.MkdirAll(s.backupsDir, 0700); err != nil {
		return "", err
	}
	name := fmt.Sprintf("config.toml.%s.bak", id)
	path := filepath.Join(s.backupsDir, name)
	if err := fileutil.AtomicWriteFile(path, content, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Syncer) ClearDefaultProfile() (string, error) {
	existing, exists, err := readIfExists(s.configPath)
	if err != nil {
		return "", err
	}
	if !exists {
		return s.configPath, nil
	}

	managedProfiles := extractManagedProfiles(existing)
	next := removeRootProfile(stripManagedBlock(existing))
	if len(managedProfiles) > 0 {
		next = appendManagedBlock(next, renderManagedBlock(managedProfiles, s.helperCommand, codexTablePresence(next)))
	}

	if err := fileutil.AtomicWriteFile(s.configPath, []byte(next), 0600); err != nil {
		return "", err
	}
	return s.configPath, nil
}

func (s *Syncer) RemoveProfile(name string) (string, error) {
	name = domainprofile.NormalizeProfileName(name)
	if strings.TrimSpace(name) == "" {
		return s.configPath, nil
	}

	existing, exists, err := readIfExists(s.configPath)
	if err != nil {
		return "", err
	}
	if !exists {
		return s.configPath, nil
	}

	managedProfiles := extractManagedProfiles(existing)
	delete(managedProfiles, name)

	next := removeRootProfile(stripManagedBlock(existing))
	if len(managedProfiles) > 0 {
		rootProfile := normalizeManagedRootProfile(findRootProfileName(existing), managedProfiles)
		if rootProfile != "" {
			next = upsertRootProfile(next, rootProfile)
		}
		next = appendManagedBlock(next, renderManagedBlock(managedProfiles, s.helperCommand, codexTablePresence(next)))
	}

	if strings.TrimSpace(next) == "" {
		return s.RemoveConfig()
	}
	if err := fileutil.AtomicWriteFile(s.configPath, []byte(next), 0600); err != nil {
		return "", err
	}
	return s.configPath, nil
}

func (s *Syncer) RemoveConfig() (string, error) {
	if err := os.Remove(s.configPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return s.configPath, nil
}

func (s *Syncer) DeleteBackup(backupPath string) error {
	if strings.TrimSpace(backupPath) == "" {
		return nil
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func readIfExists(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}

func stripManagedBlock(content string) string {
	for {
		start := strings.Index(content, beginMarker)
		if start < 0 {
			return content
		}
		end := strings.Index(content[start:], endMarker)
		if end < 0 {
			return strings.TrimRight(content[:start], "\n")
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
		b.WriteString(strconv.Quote(profile.BaseURL))
		b.WriteString("\n")
		b.WriteString("wire_api = \"responses\"\n\n")
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

func extractManagedProfiles(content string) map[string]managedProfile {
	block, ok := extractManagedBlock(content)
	if !ok {
		return map[string]managedProfile{}
	}

	lines := strings.Split(block, "\n")
	profiles := map[string]managedProfile{}
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
				continue
			}
			profile := profiles[currentName]
			profile.Name = currentName
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
			if key == "base_url" {
				profile.BaseURL = unquoteTomlValue(value)
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
			key, _, ok := strings.Cut(trimmed, "=")
			if !ok || strings.TrimSpace(key) == "model_provider" {
				profiles[currentName] = profile
				continue
			}
			profile.Extras = append(profile.Extras, strings.TrimRight(line, " \t"))
		}
		profiles[currentName] = profile
	}

	for name, profile := range profiles {
		for len(profile.Extras) > 0 && profile.Extras[0] == "" {
			profile.Extras = profile.Extras[1:]
		}
		for len(profile.Extras) > 0 && profile.Extras[len(profile.Extras)-1] == "" {
			profile.Extras = profile.Extras[:len(profile.Extras)-1]
		}
		profiles[name] = profile
	}
	return profiles
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

func codexProfileName(profileName string) string {
	return profileName
}

func codexProviderID(profileName string) string {
	return "agx/" + profileName
}

func sortedManagedProfileNames(profiles map[string]managedProfile) []string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseManagedSection(line string) (string, string) {
	switch {
	case strings.HasPrefix(line, "[model_providers."):
		name := parseQuotedSectionValue(line, "[model_providers.")
		name = strings.TrimPrefix(name, "agx/")
		if name == "" || strings.HasSuffix(line, ".auth]") {
			return "", ""
		}
		return "provider", name
	case strings.HasPrefix(line, "[profiles."):
		name := parseQuotedSectionValue(line, "[profiles.")
		if name == "" {
			return "", ""
		}
		return "profile", name
	default:
		return "", ""
	}
}

func parseQuotedSectionValue(line, prefix string) string {
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	value := strings.TrimPrefix(line, prefix)
	value = strings.TrimSuffix(value, "]")
	value = strings.TrimSpace(value)
	return unquoteTomlValue(value)
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

func selectManagedProfileExtras(content string, profiles map[string]managedProfile) []string {
	rootProfile := findRootProfileName(content)
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
		key, value, ok := parseKeyValueLine(trimmed)
		if ok && key == "profile" {
			return unquoteTomlValue(value)
		}
	}
	return ""
}

func normalizeManagedRootProfile(rootProfile string, profiles map[string]managedProfile) string {
	rootProfile = strings.TrimSpace(rootProfile)
	if rootProfile == "" {
		return ""
	}
	if _, ok := profiles[rootProfile]; !ok {
		return ""
	}
	return rootProfile
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
