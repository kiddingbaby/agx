package cli

import (
	"path/filepath"
	"strings"
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func TestNativeEnvGeminiIsolatedFromUserHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/polluted-home")
	t.Setenv("GEMINI_CLI_HOME", "/tmp/polluted-home/.gemini")
	t.Setenv("GEMINI_API_KEY", "sk-user")
	t.Setenv("GOOGLE_API_KEY", "google-user")
	t.Setenv("GOOGLE_GEMINI_BASE_URL", "https://user.example")

	contextPath := filepath.Join(t.TempDir(), "context")
	env := nativeEnv(domainprofile.AgentGemini, contextPath)

	counts := map[string]int{}
	values := map[string]string{}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		counts[key]++
		values[key] = value
	}

	if counts["HOME"] != 1 || values["HOME"] != contextPath {
		t.Fatalf("HOME=%q count=%d, want %q once", values["HOME"], counts["HOME"], contextPath)
	}
	if counts["GEMINI_CLI_HOME"] != 1 || values["GEMINI_CLI_HOME"] != contextPath {
		t.Fatalf("GEMINI_CLI_HOME=%q count=%d, want %q once", values["GEMINI_CLI_HOME"], counts["GEMINI_CLI_HOME"], contextPath)
	}
	if counts["GEMINI_CLI_TRUST_WORKSPACE"] != 1 || values["GEMINI_CLI_TRUST_WORKSPACE"] != "true" {
		t.Fatalf("GEMINI_CLI_TRUST_WORKSPACE=%q count=%d, want true once", values["GEMINI_CLI_TRUST_WORKSPACE"], counts["GEMINI_CLI_TRUST_WORKSPACE"])
	}
	if counts["GEMINI_API_KEY"] != 0 {
		t.Fatalf("GEMINI_API_KEY=%q count=%d, want removed from runtime env", values["GEMINI_API_KEY"], counts["GEMINI_API_KEY"])
	}
	if counts["GOOGLE_API_KEY"] != 0 {
		t.Fatalf("GOOGLE_API_KEY=%q count=%d, want removed from runtime env", values["GOOGLE_API_KEY"], counts["GOOGLE_API_KEY"])
	}
	if counts["GOOGLE_GEMINI_BASE_URL"] != 0 {
		t.Fatalf("GOOGLE_GEMINI_BASE_URL=%q count=%d, want removed from runtime env", values["GOOGLE_GEMINI_BASE_URL"], counts["GOOGLE_GEMINI_BASE_URL"])
	}
}
