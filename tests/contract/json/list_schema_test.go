//go:build contract

package jsoncontract

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gotest.tools/v3/assert"
)

func TestListJSONMatchesSchema(t *testing.T) {
	repoRoot, err := os.Getwd()
	assert.NilError(t, err)
	repoRoot = filepath.Clean(filepath.Join(repoRoot, "..", "..", ".."))

	home := t.TempDir()
	cmd := exec.Command(binaryPath(repoRoot), "ls", "-o", "json")
	cmd.Env = append(os.Environ(), "HOME="+home)

	stdout, err := cmd.Output()
	assert.NilError(t, err)

	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile("list.schema.json")
	assert.NilError(t, err)

	var payload any
	err = json.Unmarshal(stdout, &payload)
	assert.NilError(t, err)
	assert.NilError(t, schema.Validate(payload))
}

func TestListAgentJSONMatchesSchema(t *testing.T) {
	repoRoot, err := os.Getwd()
	assert.NilError(t, err)
	repoRoot = filepath.Clean(filepath.Join(repoRoot, "..", "..", ".."))

	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), "profile = \"before\"\n")
	runAGX(t, home, "add", "relay-a", "--base-url", "https://relay-a.example/v1", "--api-key", "sk-a")
	runAGX(t, home, "edit", "relay-a", "--bind", "codex")

	cmd := exec.Command(binaryPath(repoRoot), "ls", "--agent", "codex", "-o", "json")
	cmd.Env = append(os.Environ(), "HOME="+home)

	stdout, err := cmd.Output()
	assert.NilError(t, err)

	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile("list-agent.schema.json")
	assert.NilError(t, err)

	var payload any
	err = json.Unmarshal(stdout, &payload)
	assert.NilError(t, err)
	assert.NilError(t, schema.Validate(payload))
}

func binaryPath(repoRoot string) string {
	if cacheDir := os.Getenv("AGX_CACHE_DIR"); cacheDir != "" {
		return filepath.Join(cacheDir, "bin", "agx")
	}
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, "agx", "bin", "agx")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(repoRoot, "agx")
	}
	return filepath.Join(home, ".cache", "agx", "bin", "agx")
}
