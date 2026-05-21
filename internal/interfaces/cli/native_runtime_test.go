package cli

import (
	"path/filepath"
	"strings"
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

// TestNativeEnvGeminiInjectsCredentialsAndKeepsHostHome verifies that the
// gemini runtime env (a) routes Gemini state through GEMINI_CLI_HOME so the
// per-target context is honored, (b) injects the agx profile credentials so
// any ~/.gemini/.env discovered via cwd-walk or homedir fallback cannot win,
// and (c) leaves the host HOME alone so the user still sees ~/.ssh and
// ~/.gitconfig etc.
func TestNativeEnvGeminiInjectsCredentialsAndKeepsHostHome(t *testing.T) {
	t.Setenv("HOME", "/host/home")
	t.Setenv("GEMINI_CLI_HOME", "/host/home/.gemini")
	t.Setenv("GEMINI_API_KEY", "sk-host")
	t.Setenv("GOOGLE_API_KEY", "google-host")
	t.Setenv("GOOGLE_GEMINI_BASE_URL", "https://host.example")

	contextPath := filepath.Join(t.TempDir(), "ctx")
	profile := domainprofile.Profile{
		Name:    "any",
		Kind:    domainprofile.ProfileKindRelay,
		BaseURL: "https://anyrouter.top",
		APIKey:  "sk-agx",
	}
	values := envMap(nativeEnv(domainprofile.AgentGemini, contextPath, profile))

	if got, found := values["HOME"]; !found || got != "/host/home" {
		t.Fatalf("HOME=%q found=%v, want host HOME passthrough", got, found)
	}
	if got := values["GEMINI_CLI_HOME"]; got != contextPath {
		t.Fatalf("GEMINI_CLI_HOME=%q, want %q", got, contextPath)
	}
	if got := values["GEMINI_API_KEY"]; got != "sk-agx" {
		t.Fatalf("GEMINI_API_KEY=%q, want sk-agx (host value must not leak)", got)
	}
	wantBase := domainprofile.AgentBaseURL(domainprofile.AgentGemini, profile.BaseURL)
	if got := values["GOOGLE_GEMINI_BASE_URL"]; got != wantBase {
		t.Fatalf("GOOGLE_GEMINI_BASE_URL=%q, want %q", got, wantBase)
	}
	if _, found := values["GOOGLE_API_KEY"]; found {
		t.Fatalf("GOOGLE_API_KEY leaked from host environment")
	}
	if got := values["GEMINI_CLI_TRUST_WORKSPACE"]; got != "true" {
		t.Fatalf("GEMINI_CLI_TRUST_WORKSPACE=%q, want true", got)
	}
}

// TestNativeEnvCodexRedirectsConfigDir keeps the codex/claude/opencode plumbing
// honest: each agent's config dir env points at the per-target context and
// HOME is left untouched.
func TestNativeEnvCodexClaudeOpenCodeRedirectConfigDir(t *testing.T) {
	t.Setenv("HOME", "/host/home")
	contextPath := filepath.Join(t.TempDir(), "ctx")
	profile := domainprofile.Profile{Name: "any", APIKey: "sk-x", BaseURL: "https://x.example"}

	codex := envMap(nativeEnv(domainprofile.AgentCodex, contextPath, profile))
	if codex["CODEX_HOME"] != contextPath {
		t.Fatalf("codex CODEX_HOME=%q, want %q", codex["CODEX_HOME"], contextPath)
	}
	if codex["HOME"] != "/host/home" {
		t.Fatalf("codex HOME=%q, want host passthrough", codex["HOME"])
	}

	claude := envMap(nativeEnv(domainprofile.AgentClaude, contextPath, profile))
	if claude["CLAUDE_CONFIG_DIR"] != contextPath {
		t.Fatalf("claude CLAUDE_CONFIG_DIR=%q, want %q", claude["CLAUDE_CONFIG_DIR"], contextPath)
	}

	opencode := envMap(nativeEnv(domainprofile.AgentOpenCode, contextPath, profile))
	wantXDG := filepath.Join(contextPath, "xdg")
	if opencode["XDG_CONFIG_HOME"] != wantXDG {
		t.Fatalf("opencode XDG_CONFIG_HOME=%q, want %q", opencode["XDG_CONFIG_HOME"], wantXDG)
	}
}

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, item := range env {
		if key, value, ok := strings.Cut(item, "="); ok {
			out[key] = value
		}
	}
	return out
}
