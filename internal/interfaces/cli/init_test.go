package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWritesDefaultBundle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := New(nil, nil, nil, nil)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr

	if code := root.Execute([]string{"init"}); code != 0 {
		t.Fatalf("init code=%d want 0 stderr=%q", code, stderr.String())
	}

	path := filepath.Join(home, ".config", "agx", "agx.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(bundle) error=%v", err)
	}
	if !strings.Contains(string(data), "AGX agx.yml") {
		t.Fatalf("bundle content missing header: %q", string(data))
	}
	if !strings.Contains(stdout.String(), "Wrote config template:") {
		t.Fatalf("stdout=%q want wrote message", stdout.String())
	}
}

func TestInitPrintDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := New(nil, nil, nil, nil)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr

	if code := root.Execute([]string{"init", "--print"}); code != 0 {
		t.Fatalf("init --print code=%d want 0 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "keys: []") {
		t.Fatalf("stdout=%q want template", stdout.String())
	}
	path := filepath.Join(home, ".config", "agx", "agx.yml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("bundle should not be written, stat err=%v", err)
	}
}

func TestInitDoesNotOverwriteWithoutForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".config", "agx", "agx.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll error=%v", err)
	}
	if err := os.WriteFile(path, []byte("hello\n"), 0600); err != nil {
		t.Fatalf("WriteFile error=%v", err)
	}

	root := New(nil, nil, nil, nil)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr

	if code := root.Execute([]string{"init"}); code != 0 {
		t.Fatalf("init code=%d want 0 stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error=%v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("bundle overwritten unexpectedly: %q", string(data))
	}
	if !strings.Contains(stdout.String(), "Config already exists:") {
		t.Fatalf("stdout=%q want exists message", stdout.String())
	}
}
