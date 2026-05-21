package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

type nativeRuntime interface {
	Run(agent domainprofile.Agent, contextPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type execNativeRuntime struct{}

func (execNativeRuntime) Run(agent domainprofile.Agent, contextPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return runNativeCommand(agentBinaryName(agent), nativeRunArgs(agent, contextPath, args), nativeEnv(agent, contextPath), stdin, stdout, stderr)
}

func runNativeCommand(name string, args []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func nativeEnv(agent domainprofile.Agent, contextPath string) []string {
	env := filteredEnv(nativeEnvRemovals(agent))
	switch agent {
	case domainprofile.AgentCodex:
		env = append(env, "CODEX_HOME="+contextPath)
	case domainprofile.AgentClaude:
		env = append(env, "CLAUDE_CONFIG_DIR="+contextPath)
	case domainprofile.AgentGemini:
		env = append(env, "HOME="+contextPath)
		env = append(env, "GEMINI_CLI_HOME="+contextPath)
		env = append(env, "GEMINI_CLI_TRUST_WORKSPACE=true")
	case domainprofile.AgentOpenCode:
		env = append(env, "XDG_CONFIG_HOME="+filepath.Join(contextPath, "xdg"))
	}
	return env
}

func nativeEnvRemovals(agent domainprofile.Agent) map[string]struct{} {
	removed := map[string]struct{}{}
	for _, key := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR"} {
		removed[key] = struct{}{}
	}
	switch agent {
	case domainprofile.AgentClaude:
		for _, key := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "CLAUDE_CODE_API_KEY_HELPER_TTL_MS"} {
			removed[key] = struct{}{}
		}
	case domainprofile.AgentGemini:
		for _, key := range []string{
			"GOOGLE_GENAI_USE_VERTEXAI",
			"GEMINI_CLI_HOME",
			"HOME",
			"GEMINI_API_KEY",
			"GOOGLE_API_KEY",
			"GOOGLE_GEMINI_BASE_URL",
		} {
			removed[key] = struct{}{}
		}
	case domainprofile.AgentOpenCode:
		for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENAI_USE_VERTEXAI"} {
			removed[key] = struct{}{}
		}
	}
	return removed
}

func filteredEnv(removed map[string]struct{}) []string {
	raw := os.Environ()
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, skip := removed[key]; skip {
			continue
		}
		out = append(out, item)
	}
	return out
}

func agentBinaryName(agent domainprofile.Agent) string {
	return string(agent)
}

func nativeRunArgs(agent domainprofile.Agent, contextPath string, args []string) []string {
	out := append([]string(nil), args...)
	if agent != domainprofile.AgentClaude || hasClaudeSettingsArg(out) {
		return out
	}

	settingsPath := filepath.Join(contextPath, "settings.json")
	if _, err := os.Stat(settingsPath); err != nil {
		return out
	}
	return append([]string{"--settings", settingsPath}, out...)
}

func hasClaudeSettingsArg(args []string) bool {
	for _, arg := range args {
		if arg == "--settings" || strings.HasPrefix(arg, "--settings=") {
			return true
		}
	}
	return false
}
