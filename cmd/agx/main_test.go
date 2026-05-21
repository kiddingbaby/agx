package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMainVersionCommand(t *testing.T) {
	if os.Getenv("AGX_MAIN_HELPER") == "1" {
		version = "1.2.3-test"
		commit = "abc1234"
		date = "2026-04-28T00:00:00Z"
		os.Args = []string{"agx", "version"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainVersionCommand")
	cmd.Env = append(os.Environ(), "AGX_MAIN_HELPER=1", "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process error = %v, output=%s", err, out)
	}

	text := string(out)
	for _, want := range []string{
		"agx 1.2.3-test",
		"commit=abc1234",
		"date=2026-04-28T00:00:00Z",
		"go=go",
		"os=",
		"arch=",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output=%q missing %q", text, want)
		}
	}
}

func TestMainHelpCommand(t *testing.T) {
	if os.Getenv("AGX_MAIN_HELPER") == "help" {
		os.Args = []string{"agx"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelpCommand")
	cmd.Env = append(os.Environ(), "AGX_MAIN_HELPER=help", "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process error = %v, output=%s", err, out)
	}
	if !strings.Contains(string(out), "AGX - Local Multi-Agent Runtime Manager") {
		t.Fatalf("output=%q want help text", string(out))
	}
}

func TestMainBootstrapFailure(t *testing.T) {
	if os.Getenv("AGX_MAIN_HELPER") == "bootstrap-fail" {
		os.Unsetenv("HOME")
		os.Setenv("PATH", t.TempDir())
		os.Args = []string{"agx", "version"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainBootstrapFailure")
	cmd.Env = append(os.Environ(), "AGX_MAIN_HELPER=bootstrap-fail", "HOME=")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("helper process unexpectedly succeeded output=%s", out)
	}
	if !strings.Contains(string(out), "cannot determine home directory") && !strings.Contains(string(out), "resolve agx executable") {
		t.Fatalf("output=%q want bootstrap failure", string(out))
	}
}
