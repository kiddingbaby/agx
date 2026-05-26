package mcpinject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func TestInjectCodexPreservesPriorContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	prior := "profile = \"agx/work\"\n\n# >>> AGX managed Codex config >>>\n[profiles.\"agx/work\"]\n# <<< AGX managed Codex config <<<\n"
	if err := os.WriteFile(path, []byte(prior), 0o600); err != nil {
		t.Fatal(err)
	}
	ep := Endpoint{URL: "http://127.0.0.1:8765/mcp", BearerEnv: "AGX_MCP_TOKEN"}
	if err := Inject(domainprofile.AgentCodex, path, ep); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, prior[:strings.Index(prior, "\n\n")]) {
		t.Errorf("prior profile block lost: %s", got)
	}
	if !strings.Contains(got, "[mcp_servers.agx]") {
		t.Errorf("mcp block missing: %s", got)
	}
	if !strings.Contains(got, "bearer_token_env_var = \"AGX_MCP_TOKEN\"") {
		t.Errorf("bearer env not rendered: %s", got)
	}
}

func TestInjectCodexIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("profile = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ep := Endpoint{URL: "http://example/mcp"}
	for i := 0; i < 3; i++ {
		if err := Inject(domainprofile.AgentCodex, path, ep); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	body, _ := os.ReadFile(path)
	if c := strings.Count(string(body), "AGX managed MCP >>>"); c != 1 {
		t.Errorf("expected exactly 1 MCP block, got %d", c)
	}
}

func TestClearCodexLeavesProfileBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	prior := "profile = \"x\"\n[profiles.x]\nfoo = 1\n"
	if err := os.WriteFile(path, []byte(prior), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Inject(domainprofile.AgentCodex, path, Endpoint{URL: "http://e/mcp"}); err != nil {
		t.Fatal(err)
	}
	if err := Clear(domainprofile.AgentCodex, path); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "AGX managed MCP") {
		t.Errorf("clear left mcp block: %s", string(body))
	}
	if !strings.Contains(string(body), "[profiles.x]") {
		t.Errorf("clear destroyed profile block: %s", string(body))
	}
}

func TestInjectClaudeMerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	prior := `{"apiKeyHelper":"helper","mcpServers":{"existing":{"command":"x"}}}`
	if err := os.WriteFile(path, []byte(prior), 0o600); err != nil {
		t.Fatal(err)
	}
	ep := Endpoint{URL: "http://127.0.0.1:8765/mcp", BearerEnv: "TOK"}
	if err := Inject(domainprofile.AgentClaude, path, ep); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["apiKeyHelper"] != "helper" {
		t.Errorf("apiKeyHelper lost")
	}
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["existing"]; !ok {
		t.Errorf("user-added server lost: %v", servers)
	}
	agx := servers["agx"].(map[string]any)
	if agx["type"] != "http" || agx["url"] != ep.URL {
		t.Errorf("agx entry wrong: %v", agx)
	}
	headers := agx["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer ${TOK}" {
		t.Errorf("auth header wrong: %v", headers)
	}
}

func TestClearJSONOnlyRemovesAgx(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	prior := `{"mcpServers":{"existing":{"command":"x"},"agx":{"type":"http","url":"u"}}}`
	if err := os.WriteFile(path, []byte(prior), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Clear(domainprofile.AgentClaude, path); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["agx"]; ok {
		t.Errorf("agx still present")
	}
	if _, ok := servers["existing"]; !ok {
		t.Errorf("user server removed")
	}
}

func TestInjectMissingFileSkipsGracefully(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "settings.json")
	if err := Inject(domainprofile.AgentGemini, path, Endpoint{URL: "http://e/mcp"}); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file not created when config absent, got %v", err)
	}
}

func TestInjectMissingCodexFileSkipsGracefully(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := Inject(domainprofile.AgentCodex, path, Endpoint{URL: "http://e/mcp"}); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file not created when config absent, got %v", err)
	}
}

func TestInjectRejectsEmptyURL(t *testing.T) {
	if err := Inject(domainprofile.AgentClaude, "/tmp/foo.json", Endpoint{}); err == nil {
		t.Errorf("expected error for empty URL")
	}
}

func TestConfigPath(t *testing.T) {
	root := "/ctx"
	cases := map[domainprofile.Agent]string{
		domainprofile.AgentCodex:    "/ctx/config.toml",
		domainprofile.AgentClaude:   "/ctx/settings.json",
		domainprofile.AgentGemini:   "/ctx/.gemini/settings.json",
		domainprofile.AgentOpenCode: "/ctx/xdg/opencode/opencode.json",
	}
	for agent, want := range cases {
		if got := ConfigPath(agent, root); got != want {
			t.Errorf("%s: want %s, got %s", agent, want, got)
		}
	}
}
