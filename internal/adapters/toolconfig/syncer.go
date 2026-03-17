package toolconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kiddingbaby/agx/internal/config"
	domainagent "github.com/kiddingbaby/agx/internal/domain/agent"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/ports"
)

const (
	codexManagedProvider    = "agx"
	managedBlockStartLine   = "# >>> agx managed >>>"
	managedBlockEndLine     = "# <<< agx managed <<<"
	toolConfigStateVersion  = "agx-toolconfig-state/v1"
	toolConfigStateFileName = "toolconfig_state.json"
)

var _ ports.ToolConfigSyncer = (*Syncer)(nil)

type toolConfigState struct {
	Version string                `json:"version,omitempty"`
	Claude  toolConfigStateClaude `json:"claude,omitempty"`
}

type toolConfigStateClaude struct {
	ManagedEnvKeys []string `json:"managed_env_keys,omitempty"`
}

// Syncer writes resolved runtime config into each CLI's native config files.
type Syncer struct {
	paths config.Paths
}

func NewSyncer(paths config.Paths) *Syncer {
	return &Syncer{paths: paths}
}

func (s *Syncer) Apply(agent domainagent.Agent, key domainkey.Key, target domainprovider.Target) error {
	if err := domainkey.ValidateAPIKey(key.APIKey); err != nil {
		return fmt.Errorf("invalid api key: %w", err)
	}
	if err := domainprovider.ValidateTarget(target); err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}
	switch agent.Name {
	case "claude-code":
		return s.applyClaude(key, target)
	case "codex-cli":
		return s.applyCodex(key, target)
	case "gemini-cli":
		return s.applyGemini(key, target)
	default:
		return fmt.Errorf("unsupported agent for config sync: %s", agent.Name)
	}
}

func (s *Syncer) applyClaude(key domainkey.Key, target domainprovider.Target) error {
	data, err := s.loadJSONFile(s.paths.ClaudeSettingsPath)
	if err != nil {
		return err
	}

	if model := strings.TrimSpace(target.Model); model != "" {
		data["model"] = model
	} else {
		delete(data, "model")
	}

	env := ensureObject(data, "env")
	state := s.loadToolConfigState()
	for _, k := range state.Claude.ManagedEnvKeys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		delete(env, k)
	}
	env["ANTHROPIC_API_KEY"] = key.APIKey
	env["CLAUDE_API_KEY"] = key.APIKey
	delete(env, "ANTHROPIC_AUTH_TOKEN")
	if target.BaseURL != "" {
		env["ANTHROPIC_BASE_URL"] = target.BaseURL
	} else {
		delete(env, "ANTHROPIC_BASE_URL")
	}
	for k, v := range target.Env {
		env[k] = v
	}

	if err := s.saveJSONFile(s.paths.ClaudeSettingsPath, data); err != nil {
		return err
	}

	managedKeys := make([]string, 0, len(target.Env))
	for k := range target.Env {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		managedKeys = append(managedKeys, k)
	}
	sort.Strings(managedKeys)
	state.Version = toolConfigStateVersion
	state.Claude.ManagedEnvKeys = managedKeys
	s.saveToolConfigState(state)
	return nil
}

func (s *Syncer) applyCodex(key domainkey.Key, target domainprovider.Target) error {
	authData, err := s.loadJSONFile(s.paths.CodexAuthPath)
	if err != nil {
		return err
	}
	authData["auth_mode"] = "apikey"
	authData["OPENAI_API_KEY"] = key.APIKey
	if err := s.saveJSONFile(s.paths.CodexAuthPath, authData); err != nil {
		return err
	}

	content, err := os.ReadFile(s.paths.CodexConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	tomlContent := normalizeNewlines(string(content))
	if model := strings.TrimSpace(target.Model); model != "" {
		tomlContent = upsertRootTOMLString(tomlContent, "model", model)
	} else {
		tomlContent = deleteRootTOMLKey(tomlContent, "model")
	}
	tomlContent = upsertRootTOMLString(tomlContent, "model_provider", codexManagedProvider)
	tomlContent = upsertRootTOMLString(tomlContent, "preferred_auth_method", "apikey")
	tomlContent = replaceManagedBlock(tomlContent, s.codexManagedBlock(target))
	return writeAtomicFile(s.paths.CodexConfigPath, []byte(normalizeTrailingNewline(tomlContent)), 0600)
}

func (s *Syncer) codexManagedBlock(target domainprovider.Target) string {
	wireAPI := target.WireAPI
	if strings.TrimSpace(string(wireAPI)) == "" {
		wireAPI = domainprovider.WireAPIResponses
	}
	// Codex CLI currently only supports Responses API. Keep backward compatibility with older
	// AGX configs that persisted `chat_completions` by normalizing it to `responses`.
	if wireAPI == domainprovider.WireAPIChatCompletions {
		wireAPI = domainprovider.WireAPIResponses
	}
	requiresOpenAIAuth := true
	if target.RequiresOpenAIAuth != nil {
		requiresOpenAIAuth = *target.RequiresOpenAIAuth
	}

	lines := []string{
		managedBlockStartLine,
		`[model_providers."` + codexManagedProvider + `"]`,
		`name = "` + escapeTOMLString(codexManagedProvider) + `"`,
		`wire_api = "` + escapeTOMLString(string(wireAPI)) + `"`,
		fmt.Sprintf("requires_openai_auth = %t", requiresOpenAIAuth),
	}
	if target.BaseURL != "" {
		lines = append(lines, `base_url = "`+escapeTOMLString(target.BaseURL)+`"`)
	}
	lines = append(lines, managedBlockEndLine)
	return strings.Join(lines, "\n")
}

func (s *Syncer) applyGemini(key domainkey.Key, target domainprovider.Target) error {
	settingsData, err := s.loadJSONFile(s.paths.GeminiSettingsPath)
	if err != nil {
		return err
	}
	if model := strings.TrimSpace(target.Model); model != "" {
		settingsData["model"] = model
	} else {
		delete(settingsData, "model")
	}
	security := ensureObject(settingsData, "security")
	auth := ensureObject(security, "auth")
	auth["selectedType"] = "gemini-api-key"
	if err := s.saveJSONFile(s.paths.GeminiSettingsPath, settingsData); err != nil {
		return err
	}

	content, err := os.ReadFile(s.paths.GeminiEnvPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	envLines := []string{
		managedBlockStartLine,
		"GEMINI_API_KEY=" + key.APIKey,
		"GOOGLE_GEMINI_API_KEY=" + key.APIKey,
		"GOOGLE_API_KEY=" + key.APIKey,
	}
	if target.BaseURL != "" {
		envLines = append(envLines,
			"GOOGLE_GEMINI_BASE_URL="+target.BaseURL,
			"GEMINI_BASE_URL="+target.BaseURL,
		)
	}
	if len(target.Env) > 0 {
		keys := make([]string, 0, len(target.Env))
		for k := range target.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			envLines = append(envLines, k+"="+target.Env[k])
		}
	}
	envLines = append(envLines, managedBlockEndLine)

	envContent := replaceManagedBlock(normalizeNewlines(string(content)), strings.Join(envLines, "\n"))
	return writeAtomicFile(s.paths.GeminiEnvPath, []byte(normalizeTrailingNewline(envContent)), 0600)
}

func (s *Syncer) loadJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse json %s: %w", path, err)
	}
	return payload, nil
}

func (s *Syncer) saveJSONFile(path string, payload map[string]any) error {
	data, err := marshalSortedJSON(payload)
	if err != nil {
		return err
	}
	return writeAtomicFile(path, data, 0600)
}

func ensureObject(parent map[string]any, key string) map[string]any {
	if raw, ok := parent[key]; ok {
		if obj, ok := raw.(map[string]any); ok {
			return obj
		}
	}
	obj := map[string]any{}
	parent[key] = obj
	return obj
}

func (s *Syncer) toolConfigStatePath() string {
	if strings.TrimSpace(s.paths.ConfigDir) != "" {
		return filepath.Join(s.paths.ConfigDir, toolConfigStateFileName)
	}
	if strings.TrimSpace(s.paths.StorePath) != "" {
		return filepath.Join(filepath.Dir(s.paths.StorePath), toolConfigStateFileName)
	}
	if strings.TrimSpace(s.paths.ProviderConfigPath) != "" {
		return filepath.Join(filepath.Dir(s.paths.ProviderConfigPath), toolConfigStateFileName)
	}
	return ""
}

func (s *Syncer) loadToolConfigState() toolConfigState {
	path := s.toolConfigStatePath()
	if strings.TrimSpace(path) == "" {
		return toolConfigState{Version: toolConfigStateVersion}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return toolConfigState{Version: toolConfigStateVersion}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return toolConfigState{Version: toolConfigStateVersion}
	}
	var state toolConfigState
	if err := json.Unmarshal(data, &state); err != nil {
		return toolConfigState{Version: toolConfigStateVersion}
	}
	if strings.TrimSpace(state.Version) == "" {
		state.Version = toolConfigStateVersion
	}
	return state
}

func (s *Syncer) saveToolConfigState(state toolConfigState) {
	path := s.toolConfigStatePath()
	if strings.TrimSpace(path) == "" {
		return
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	data = append(data, '\n')
	_ = writeAtomicFile(path, data, 0600)
}

func marshalSortedJSON(payload map[string]any) ([]byte, error) {
	normalized := normalizeJSONValue(payload)
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func normalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(typed))
		for _, key := range keys {
			out[key] = normalizeJSONValue(typed[key])
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = normalizeJSONValue(typed[i])
		}
		return out
	default:
		return value
	}
}

func replaceManagedBlock(content, block string) string {
	content = normalizeNewlines(content)
	content = strings.TrimRight(content, "\n")
	re := regexp.MustCompile(`(?ms)^# >>> agx managed >>>\n.*?\n# <<< agx managed <<<\n?`)
	if re.MatchString(content) {
		return re.ReplaceAllString(content, block+"\n")
	}
	if content == "" {
		return block
	}
	return content + "\n\n" + block
}

func upsertRootTOMLString(content, key, value string) string {
	root, rest := splitTOMLRoot(content)
	line := fmt.Sprintf(`%s = "%s"`, key, escapeTOMLString(value))
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=.*$`)
	if re.MatchString(root) {
		return re.ReplaceAllString(root, line) + rest
	}
	root = strings.TrimLeft(root, "\n")
	if root == "" && rest == "" {
		return line + "\n"
	}
	if root == "" {
		return line + "\n" + rest
	}
	return line + "\n" + root + rest
}

func deleteRootTOMLKey(content, key string) string {
	root, rest := splitTOMLRoot(content)
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=.*\n?`)
	return re.ReplaceAllString(root, "") + rest
}

func escapeTOMLString(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(value)
}

func normalizeTrailingNewline(content string) string {
	return strings.TrimRight(content, "\n") + "\n"
}

func normalizeNewlines(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

func splitTOMLRoot(content string) (root, rest string) {
	tableHeader := regexp.MustCompile(`(?m)^[ \t]*\[`)
	loc := tableHeader.FindStringIndex(content)
	if loc == nil {
		return content, ""
	}
	return content[:loc[0]], content[loc[0]:]
}

func writeAtomicFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmpPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
