// Package mcpinject writes (or clears) a single "this is the agx MCP
// gateway" entry into each agent's managed context config. It exists so
// agx can keep four agents pointed at one local gateway without the
// gateway needing to know anything about the agents' config formats —
// agents stay decoupled from the gateway implementation; the gateway
// stays decoupled from the agents.
//
// Convention:
//   - codex   → marker-comment block inside config.toml (since codex's
//     managed file is shared with the profile block, we need an explicit
//     marker to scope what we own).
//   - claude / gemini → mcpServers.<name> key in the JSON file. agx
//     owns whatever entry sits under the agreed server name.
//   - opencode        → mcp.<name> key in the JSON file. Same convention.
//
// `Clear` removes only the agx-owned slice; user-added MCP servers under
// other names are untouched.
package mcpinject

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

const (
	defaultName = "agx"

	codexBegin = "# >>> AGX managed MCP >>>"
	codexEnd   = "# <<< AGX managed MCP <<<"
)

// ConfigPath returns the file each agent's managed-context config lives in,
// rooted at contextRoot (which is per-target). Mirrors the layout the
// agent syncers themselves use; exported so `agx mcp sync` can target the
// right file without duplicating that knowledge.
func ConfigPath(agent domainprofile.Agent, contextRoot string) string {
	switch agent {
	case domainprofile.AgentCodex:
		return filepath.Join(contextRoot, "config.toml")
	case domainprofile.AgentClaude:
		return filepath.Join(contextRoot, "settings.json")
	case domainprofile.AgentGemini:
		return filepath.Join(contextRoot, ".gemini", "settings.json")
	case domainprofile.AgentOpenCode:
		return filepath.Join(contextRoot, "xdg", "opencode", "opencode.json")
	}
	return ""
}

// Endpoint describes what to inject. URL must be non-empty for Inject; an
// empty URL is invalid (callers should call Clear in that case).
type Endpoint struct {
	// Name is the server name agents will see (default "agx").
	Name string
	// URL is the MCP HTTP endpoint.
	URL string
	// BearerEnv, if set, names an env var holding the bearer token. The
	// resulting config references the env var (codex via
	// bearer_token_env_var, others via ${VAR} in headers) so secrets do
	// not land on disk.
	BearerEnv string
}

func (e Endpoint) name() string {
	if strings.TrimSpace(e.Name) == "" {
		return defaultName
	}
	return e.Name
}

// Inject writes the endpoint into the agent's managed config file. The
// file must already exist (typically created by the agent's main syncer
// during `agx use`). For agents we don't recognize, returns an error.
func Inject(agent domainprofile.Agent, configPath string, ep Endpoint) error {
	if strings.TrimSpace(ep.URL) == "" {
		return errors.New("mcpinject: endpoint URL required")
	}
	if strings.TrimSpace(configPath) == "" {
		return errors.New("mcpinject: config path required")
	}
	switch agent {
	case domainprofile.AgentCodex:
		return injectCodex(configPath, ep)
	case domainprofile.AgentClaude:
		return injectJSON(configPath, ep, "mcpServers", claudeEntry)
	case domainprofile.AgentGemini:
		return injectJSON(configPath, ep, "mcpServers", geminiEntry)
	case domainprofile.AgentOpenCode:
		return injectJSON(configPath, ep, "mcp", opencodeEntry)
	default:
		return fmt.Errorf("mcpinject: unsupported agent %q", agent)
	}
}

// Clear removes the agx-owned MCP entry. Missing files / missing entries
// are a no-op.
func Clear(agent domainprofile.Agent, configPath string) error {
	if strings.TrimSpace(configPath) == "" {
		return nil
	}
	switch agent {
	case domainprofile.AgentCodex:
		return clearCodex(configPath)
	case domainprofile.AgentClaude:
		return clearJSON(configPath, "mcpServers", defaultName)
	case domainprofile.AgentGemini:
		return clearJSON(configPath, "mcpServers", defaultName)
	case domainprofile.AgentOpenCode:
		return clearJSON(configPath, "mcp", defaultName)
	default:
		return fmt.Errorf("mcpinject: unsupported agent %q", agent)
	}
}

func injectCodex(path string, ep Endpoint) error {
	body, exists, err := fileutil.ReadIfExists(path)
	if err != nil {
		return err
	}
	// We never create the codex managed config from here; that's the
	// codex syncer's job (it owns the profile block above ours). Skip
	// silently so `agx mcp sync` doesn't fail for agents the user has
	// never run.
	if !exists {
		return nil
	}
	stripped := stripCodexBlock(body)
	block := renderCodexBlock(ep)
	next := strings.TrimRight(stripped, "\n")
	if next != "" {
		next += "\n\n"
	}
	next += block + "\n"
	return fileutil.AtomicWriteFile(path, []byte(next), 0o600)
}

func clearCodex(path string) error {
	body, exists, err := fileutil.ReadIfExists(path)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	stripped := stripCodexBlock(body)
	if stripped == body {
		return nil
	}
	return fileutil.AtomicWriteFile(path, []byte(stripped), 0o600)
}

func stripCodexBlock(content string) string {
	start := strings.Index(content, codexBegin)
	if start < 0 {
		return content
	}
	end := strings.Index(content[start:], codexEnd)
	if end < 0 {
		return strings.TrimRight(content[:start], "\n")
	}
	end += start + len(codexEnd)
	rest := content[end:]
	for strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	pre := strings.TrimRight(content[:start], "\n")
	if pre == "" {
		return rest
	}
	if rest == "" {
		return pre + "\n"
	}
	return pre + "\n\n" + rest
}

func renderCodexBlock(ep Endpoint) string {
	var b strings.Builder
	b.WriteString(codexBegin)
	b.WriteString("\n")
	b.WriteString("# Edited by `agx mcp sync`. Do not edit by hand; changes inside this\n")
	b.WriteString("# block will be overwritten the next time agx syncs the MCP endpoint.\n\n")
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", ep.name())
	fmt.Fprintf(&b, "url = %q\n", ep.URL)
	if ep.BearerEnv != "" {
		fmt.Fprintf(&b, "bearer_token_env_var = %q\n", ep.BearerEnv)
	}
	b.WriteString(codexEnd)
	return b.String()
}

// jsonEntryFn produces the per-agent JSON value for an mcp entry.
type jsonEntryFn func(ep Endpoint) map[string]any

func claudeEntry(ep Endpoint) map[string]any {
	entry := map[string]any{
		"type": "http",
		"url":  ep.URL,
	}
	if ep.BearerEnv != "" {
		entry["headers"] = map[string]any{
			"Authorization": "Bearer ${" + ep.BearerEnv + "}",
		}
	}
	return entry
}

func geminiEntry(ep Endpoint) map[string]any {
	entry := map[string]any{
		"httpUrl": ep.URL,
		"trust":   true,
	}
	if ep.BearerEnv != "" {
		entry["headers"] = map[string]any{
			"Authorization": "Bearer ${" + ep.BearerEnv + "}",
		}
	}
	return entry
}

func opencodeEntry(ep Endpoint) map[string]any {
	entry := map[string]any{
		"type":    "remote",
		"url":     ep.URL,
		"enabled": true,
	}
	if ep.BearerEnv != "" {
		entry["headers"] = map[string]any{
			"Authorization": "Bearer ${" + ep.BearerEnv + "}",
		}
	}
	return entry
}

func injectJSON(path string, ep Endpoint, topKey string, entry jsonEntryFn) error {
	body, exists, err := fileutil.ReadIfExists(path)
	if err != nil {
		return err
	}
	// Mirror injectCodex: we don't create files the agent syncer hasn't
	// touched yet. Inject is purely "add an MCP entry to an existing
	// managed config", never "bootstrap a managed config".
	if !exists {
		return nil
	}
	doc := map[string]any{}
	if strings.TrimSpace(body) != "" {
		if err := json.Unmarshal([]byte(body), &doc); err != nil {
			return fmt.Errorf("mcpinject: parse %s: %w", path, err)
		}
	}
	servers, _ := doc[topKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[ep.name()] = entry(ep)
	doc[topKey] = servers
	return writeJSONDoc(path, doc)
}

func clearJSON(path, topKey, name string) error {
	body, exists, err := fileutil.ReadIfExists(path)
	if err != nil {
		return err
	}
	if !exists || strings.TrimSpace(body) == "" {
		return nil
	}
	doc := map[string]any{}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return fmt.Errorf("mcpinject: parse %s: %w", path, err)
	}
	servers, ok := doc[topKey].(map[string]any)
	if !ok {
		return nil
	}
	if _, present := servers[name]; !present {
		return nil
	}
	delete(servers, name)
	if len(servers) == 0 {
		delete(doc, topKey)
	} else {
		doc[topKey] = servers
	}
	return writeJSONDoc(path, doc)
}

func writeJSONDoc(path string, doc map[string]any) error {
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return fileutil.AtomicWriteFile(path, body, 0o600)
}
