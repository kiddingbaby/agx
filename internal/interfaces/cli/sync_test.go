package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kiddingbaby/agx/internal/config"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func TestSync_SystemPromptAndSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths, err := config.DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error = %v", err)
	}
	envSyncSvc := usecase.NewEnvSyncService(paths, nil)

	root := New(nil, nil, nil, envSyncSvc)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr

	skillsHub := filepath.Join(home, "skills-hub")
	systemPrompt := filepath.Join(skillsHub, "system-prompt", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(systemPrompt), 0755); err != nil {
		t.Fatalf("MkdirAll(system-prompt) error = %v", err)
	}
	if err := os.WriteFile(systemPrompt, []byte("# system prompt\n"), 0644); err != nil {
		t.Fatalf("WriteFile(system prompt) error = %v", err)
	}

	skillFile := filepath.Join(skillsHub, "skills", "tools", "demo", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillFile), 0755); err != nil {
		t.Fatalf("MkdirAll(skill) error = %v", err)
	}
	if err := os.WriteFile(skillFile, []byte("# demo\n"), 0644); err != nil {
		t.Fatalf("WriteFile(skill) error = %v", err)
	}

	bundle := `
assets:
  skills-hub-home: "` + skillsHub + `"
  system-prompt-path: system-prompt/AGENTS.md
  skills:
    source: skills/tools
    prune: true
`
	bundlePath := filepath.Join(home, ".config", "agx", "agx.yml")
	if err := os.MkdirAll(filepath.Dir(bundlePath), 0700); err != nil {
		t.Fatalf("MkdirAll(bundle dir) error = %v", err)
	}
	if err := os.WriteFile(bundlePath, []byte(bundle), 0600); err != nil {
		t.Fatalf("WriteFile(bundle) error = %v", err)
	}

	code := root.Execute([]string{"sync", bundlePath, "-o", "json"})
	if code != 0 {
		t.Fatalf("sync code=%d want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	var env resultEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("parse stdout json error=%v; stdout=%q", err, stdout.String())
	}
	if env.SchemaVersion != "agx/sync/v1" || env.Tool != "agx" || env.Status != "ok" {
		t.Fatalf("envelope=%+v, want schema=agx/sync/v1 tool=agx status=ok", env)
	}

	codexLink := filepath.Join(home, ".codex", "AGENTS.md")
	if _, err := os.Lstat(codexLink); err != nil {
		t.Fatalf("codex AGENTS.md missing: %v", err)
	}
	gotTarget, err := os.Readlink(codexLink)
	if err != nil {
		t.Fatalf("Readlink(codex) error = %v", err)
	}
	if filepath.Clean(gotTarget) != filepath.Clean(systemPrompt) {
		t.Fatalf("codex link target=%q want %q", gotTarget, systemPrompt)
	}

	gotSkill := filepath.Join(home, ".codex", "skills", "tools", "demo", "SKILL.md")
	if _, err := os.Stat(gotSkill); err != nil {
		t.Fatalf("skill not synced: %v", err)
	}
}
