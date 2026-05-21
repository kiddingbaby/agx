package opencodeconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

const schemaURL = "https://opencode.ai/config.json"

var _ ports.OpenCodeSyncer = (*Syncer)(nil)

type Syncer struct {
	configPath string
	backupsDir string
}

func NewSyncer(configPath, backupsDir string) *Syncer {
	return &Syncer{
		configPath: configPath,
		backupsDir: filepath.Join(backupsDir, "opencode"),
	}
}

func (s *Syncer) Sync(profile domainprofile.Profile, options ports.OpenCodeSyncOptions) (*ports.OpenCodeSyncResult, error) {
	if err := domainprofile.ValidateOpenCodeModelID(options.ModelID); err != nil {
		return nil, err
	}
	family := options.ProviderFamily
	if !family.Valid() || family == "" {
		return nil, fmt.Errorf("opencode sync requires a provider family")
	}

	settings, _, err := s.readSettings()
	if err != nil {
		return nil, err
	}

	providerID := domainprofile.OpenCodeProviderID(profile.Name)
	if providerID == "" {
		return nil, fmt.Errorf("opencode sync requires a valid profile name")
	}
	settings["$schema"] = schemaURL
	providers := readObject(settings, "provider")
	providers[providerID] = buildProviderConfig(profile, options)
	settings["provider"] = providers
	if options.SetAsCurrent {
		settings["model"] = providerID + "/" + options.ModelID
	} else if currentModel, ok := settings["model"].(string); ok && strings.TrimSpace(currentModel) == "" {
		settings["model"] = providerID + "/" + options.ModelID
	}

	if err := fileutil.AtomicWriteJSON(s.configPath, settings, 0600); err != nil {
		return nil, err
	}

	return &ports.OpenCodeSyncResult{
		ProviderID: providerID,
		ModelID:    options.ModelID,
		ConfigPath: s.configPath,
	}, nil
}

func (s *Syncer) Status() (*ports.OpenCodeConfigStatus, error) {
	settings, _, err := s.readSettings()
	if err != nil {
		return nil, err
	}

	status := &ports.OpenCodeConfigStatus{
		ConfigPath:           s.configPath,
		ManagedProvidersByID: map[string]ports.OpenCodeManagedProvider{},
	}
	providers := readObject(settings, "provider")
	for id, raw := range providers {
		if !strings.HasPrefix(id, "agx-") {
			continue
		}
		provider, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		family := providerFamilyFromProvider(provider)
		modelID, _ := providerModel(provider)
		if modelID == "" {
			continue
		}
		status.ManagedProvidersByID[id] = ports.OpenCodeManagedProvider{
			ID:     id,
			Name:   stringValue(provider, "name"),
			Family: family,
			Model:  modelID,
		}
	}
	if currentModel, ok := settings["model"].(string); ok && strings.TrimSpace(currentModel) != "" {
		status.DefaultModel = strings.TrimSpace(currentModel)
	}
	return status, nil
}

func (s *Syncer) Snapshot() (*ports.AgentConfigSnapshot, error) {
	content, exists, err := fileutil.ReadIfExists(s.configPath)
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
	name := fmt.Sprintf("opencode.json.%s.bak", id)
	path := filepath.Join(s.backupsDir, name)
	if err := fileutil.AtomicWriteFile(path, content, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Syncer) ClearDefaultModel() (string, error) {
	settings, exists, err := s.readSettings()
	if err != nil {
		return "", err
	}
	if !exists {
		return s.configPath, nil
	}

	delete(settings, "model")
	if isEmptyManagedConfig(settings) {
		if err := os.Remove(s.configPath); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return s.configPath, nil
	}
	if err := fileutil.AtomicWriteJSON(s.configPath, settings, 0600); err != nil {
		return "", err
	}
	return s.configPath, nil
}

func (s *Syncer) RemoveProfile(name string) (string, error) {
	providerID := domainprofile.OpenCodeProviderID(name)
	if strings.TrimSpace(providerID) == "" {
		return s.configPath, nil
	}

	settings, exists, err := s.readSettings()
	if err != nil {
		return "", err
	}
	if !exists {
		return s.configPath, nil
	}

	providers := readObject(settings, "provider")
	delete(providers, providerID)
	if len(providers) == 0 {
		delete(settings, "provider")
	} else {
		settings["provider"] = providers
	}

	if current, _ := settings["model"].(string); strings.HasPrefix(current, providerID+"/") {
		delete(settings, "model")
	}
	if isEmptyManagedConfig(settings) {
		if err := os.Remove(s.configPath); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return s.configPath, nil
	}
	if err := fileutil.AtomicWriteJSON(s.configPath, settings, 0600); err != nil {
		return "", err
	}
	return s.configPath, nil
}

func (s *Syncer) Restore(backupPath string) (string, error) {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return "", err
	}
	if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
		if !json.Valid(data) {
			return "", fmt.Errorf("opencode backup %s is not valid JSON", backupPath)
		}
	}
	if err := fileutil.AtomicWriteFile(s.configPath, data, 0600); err != nil {
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

func (s *Syncer) readSettings() (map[string]any, bool, error) {
	data, exists, err := fileutil.ReadIfExists(s.configPath)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return map[string]any{}, false, nil
	}
	parsed, err := parseJSONC([]byte(data))
	if err != nil {
		return nil, false, err
	}
	if parsed == nil {
		parsed = map[string]any{}
	}
	return parsed, true, nil
}

func parseJSONC(data []byte) (map[string]any, error) {
	trimmed := stripJSONComments(data)
	if len(strings.TrimSpace(string(trimmed))) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return nil, fmt.Errorf("parse opencode config: %w", err)
	}
	return out, nil
}

func stripJSONComments(data []byte) []byte {
	const (
		stateNormal = iota
		stateString
		stateLineComment
		stateBlockComment
	)

	out := make([]byte, 0, len(data))
	state := stateNormal
	escaped := false
	for i := 0; i < len(data); i++ {
		ch := data[i]
		next := byte(0)
		if i+1 < len(data) {
			next = data[i+1]
		}

		switch state {
		case stateNormal:
			if ch == '"' {
				state = stateString
				out = append(out, ch)
				continue
			}
			if ch == '/' && next == '/' {
				state = stateLineComment
				i++
				continue
			}
			if ch == '/' && next == '*' {
				state = stateBlockComment
				i++
				continue
			}
			out = append(out, ch)
		case stateString:
			out = append(out, ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				state = stateNormal
			}
		case stateLineComment:
			if ch == '\n' {
				out = append(out, ch)
				state = stateNormal
			}
		case stateBlockComment:
			if ch == '\n' {
				out = append(out, ch)
			}
			if ch == '*' && next == '/' {
				i++
				state = stateNormal
			}
		}
	}
	return out
}

func readObject(settings map[string]any, key string) map[string]any {
	if settings == nil {
		return map[string]any{}
	}
	if raw, ok := settings[key]; ok && raw != nil {
		if obj, ok := raw.(map[string]any); ok {
			return obj
		}
	}
	return map[string]any{}
}

func buildProviderConfig(profile domainprofile.Profile, options ports.OpenCodeSyncOptions) map[string]any {
	providerName := strings.TrimSpace(options.ProviderName)
	if providerName == "" {
		providerName = profile.Name
	}
	modelName := strings.TrimSpace(options.ModelName)
	if modelName == "" {
		modelName = options.ModelID
	}

	config := map[string]any{
		"name": providerName,
		"options": map[string]any{
			"baseURL": providerBaseURL(options.ProviderFamily, profile.BaseURL),
			"apiKey":  profile.APIKey,
		},
		"models": map[string]any{
			options.ModelID: map[string]any{
				"name": modelName,
			},
		},
	}
	if npm := providerNPM(options.ProviderFamily); npm != "" {
		config["npm"] = npm
	}
	return config
}

func providerBaseURL(family domainprofile.OpenCodeProviderFamily, raw string) string {
	switch family {
	case domainprofile.OpenCodeProviderFamilyAnthropic, domainprofile.OpenCodeProviderFamilyOpenAICompatible:
		return domainprofile.BaseURLWithV1(raw)
	case domainprofile.OpenCodeProviderFamilyGemini:
		return domainprofile.BaseURLWithoutTrailingV1(raw)
	default:
		return domainprofile.NormalizeBaseURL(raw)
	}
}

func providerNPM(family domainprofile.OpenCodeProviderFamily) string {
	switch family {
	case domainprofile.OpenCodeProviderFamilyOpenAICompatible:
		return "@ai-sdk/openai-compatible"
	case domainprofile.OpenCodeProviderFamilyAnthropic:
		return "@ai-sdk/anthropic"
	case domainprofile.OpenCodeProviderFamilyGemini:
		return "@ai-sdk/google"
	default:
		return ""
	}
}

func providerFamilyFromProvider(provider map[string]any) domainprofile.OpenCodeProviderFamily {
	if npm, _ := provider["npm"].(string); npm != "" {
		switch npm {
		case "@ai-sdk/openai-compatible", "@ai-sdk/openai":
			return domainprofile.OpenCodeProviderFamilyOpenAICompatible
		case "@ai-sdk/anthropic":
			return domainprofile.OpenCodeProviderFamilyAnthropic
		case "@ai-sdk/google":
			return domainprofile.OpenCodeProviderFamilyGemini
		}
	}
	return domainprofile.OpenCodeProviderFamilyOpenAICompatible
}

func providerModel(provider map[string]any) (string, string) {
	models, ok := provider["models"].(map[string]any)
	if !ok {
		return "", ""
	}
	// Iterate in sorted ID order so Status() / DefaultModel() return a stable
	// answer when a provider has multiple models — Go map iteration order is
	// randomized and the previous implementation could return different
	// model IDs across calls, causing harmless-looking but real drift in
	// state.OpenCode.DefaultModel.
	ids := make([]string, 0, len(models))
	for id := range models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		raw := models[id]
		model, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		return id, stringValue(model, "name")
	}
	return "", ""
}

func stringValue(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	value, _ := obj[key].(string)
	return strings.TrimSpace(value)
}

func isEmptyManagedConfig(settings map[string]any) bool {
	if len(settings) == 0 {
		return true
	}
	for key := range settings {
		if key == "$schema" {
			continue
		}
		return false
	}
	return true
}
