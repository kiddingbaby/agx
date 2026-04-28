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

func TestShowJSONMatchesSchema(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), "profile = \"before\"\n")

	runAGX(t, home, "add", "relay-a", "--base-url", "https://relay-a.example/v1", "--api-key", "sk-a")
	runAGX(t, home, "edit", "relay-a", "--bind", "codex")

	validateCommandAgainstSchema(t, home, "show.schema.json", "show", "relay-a", "-o", "json")
}

func TestAddJSONMatchesSchema(t *testing.T) {
	home := t.TempDir()
	validateCommandAgainstSchema(t, home, "add.schema.json", "add", "relay-a", "--base-url", "https://relay-a.example/v1", "--api-key", "sk-a", "-o", "json")
}

func TestSetJSONMatchesSchema(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), "profile = \"before\"\n")

	runAGX(t, home, "add", "relay-a", "--base-url", "https://relay-a.example/v1", "--api-key", "sk-a")
	validateCommandAgainstSchema(t, home, "set.schema.json", "edit", "relay-a", "--bind", "codex", "-o", "json")
}

func TestBackupJSONMatchesSchema(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), "profile = \"before\"\n")

	runAGX(t, home, "add", "relay-a", "--base-url", "https://relay-a.example/v1", "--api-key", "sk-a")
	runAGX(t, home, "edit", "relay-a", "--bind", "codex")

	validateCommandAgainstSchema(t, home, "backup.schema.json", "backup", "ls", "--agent", "codex", "-o", "json")
}

func TestRestoreJSONMatchesSchema(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), "profile = \"before\"\n")

	runAGX(t, home, "add", "relay-a", "--base-url", "https://relay-a.example/v1", "--api-key", "sk-a")
	runAGX(t, home, "edit", "relay-a", "--bind", "codex")

	validateCommandAgainstSchema(t, home, "restore.schema.json", "restore", "--agent", "codex", "-o", "json")
}

func TestRemoveJSONMatchesSchema(t *testing.T) {
	home := t.TempDir()
	runAGX(t, home, "add", "relay-a", "--base-url", "https://relay-a.example/v1", "--api-key", "sk-a")

	validateCommandAgainstSchema(t, home, "remove.schema.json", "rm", "relay-a", "-o", "json")
}

func TestDoctorJSONMatchesSchema(t *testing.T) {
	home := t.TempDir()
	validateCommandAgainstSchema(t, home, "doctor.schema.json", "doctor", "-o", "json")
}

func validateCommandAgainstSchema(t *testing.T, home, schemaName string, args ...string) {
	t.Helper()

	repoRoot := repoRoot(t)
	cmd := exec.Command(binaryPath(repoRoot), args...)
	cmd.Env = append(os.Environ(), "HOME="+home)

	stdout, err := cmd.Output()
	assert.NilError(t, err)

	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaName)
	assert.NilError(t, err)

	var payload any
	err = json.Unmarshal(stdout, &payload)
	assert.NilError(t, err)
	assert.NilError(t, schema.Validate(payload))
}

func runAGX(t *testing.T, home string, args ...string) {
	t.Helper()

	cmd := exec.Command(binaryPath(repoRoot(t)), args...)
	cmd.Env = append(os.Environ(), "HOME="+home)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("agx %v failed: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := os.Getwd()
	assert.NilError(t, err)
	return filepath.Clean(filepath.Join(root, "..", "..", ".."))
}
