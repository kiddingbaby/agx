package cli

import (
	"strings"
	"testing"
)

// TestCommandErrorPaths exercises the common failure shapes each
// profile-management command must surface to the user: usage errors
// (missing or empty args), not-found errors, and post-create state
// errors. It runs against the real Bootstrap container — same setup
// as TestAdapterShortcutCommandsV3 — so behavior matches what users
// see, not a hand-rolled fake.
//
// `cmd.Usage()` writes the "Usage: …" block to stdout; messages from
// `printUserError` go to stderr. Each case picks the stream it's
// actually asserting on.
func TestCommandErrorPaths(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr []string
	}{
		{
			name:       "add missing required args",
			args:       []string{"add"},
			wantCode:   1,
			wantStdout: []string{"Usage:", "agx add <profile>"},
		},
		{
			name:       "add empty profile name",
			args:       []string{"add", "", "--base-url", "https://relay.example", "--api-key", "sk-a"},
			wantCode:   1,
			wantStdout: []string{"Usage:"},
		},
		{
			name:       "rm missing profile",
			args:       []string{"rm", "ghost"},
			wantCode:   1,
			wantStderr: []string{"Profile ghost not found"},
		},
		{
			name:       "show missing profile",
			args:       []string{"show", "ghost"},
			wantCode:   1,
			wantStderr: []string{"Profile ghost not found"},
		},
		{
			name:       "use missing profile",
			args:       []string{"use", "ghost"},
			wantCode:   1,
			wantStderr: []string{"Profile ghost not found"},
		},
		{
			name:       "current with extra arg rejects",
			args:       []string{"current", "extra"},
			wantCode:   1,
			wantStdout: []string{"Usage:"},
		},
		{
			name:       "edit without any flag rejects",
			args:       []string{"edit", "ghost"},
			wantCode:   1,
			wantStdout: []string{"Usage:"},
		},
		{
			name:       "detach missing args",
			args:       []string{"detach"},
			wantCode:   1,
			wantStdout: []string{"Usage:"},
		},
		{
			name:       "detach unknown agent",
			args:       []string{"detach", "bogus", "work"},
			wantCode:   1,
			wantStderr: []string{"agent must be one of"},
		},
		{
			name:       "ls extra arg rejects",
			args:       []string{"ls", "extra"},
			wantCode:   1,
			wantStdout: []string{"Usage:"},
		},
		{
			name:       "doctor extra arg rejects",
			args:       []string{"doctor", "extra"},
			wantCode:   1,
			wantStdout: []string{"Usage:"},
		},
		{
			name:       "invalid output format rejects",
			args:       []string{"ls", "-o", "yaml"},
			wantCode:   1,
			wantStderr: []string{"-o requires value json"},
		},
		{
			name:       "restore unknown agent rejects",
			args:       []string{"restore", "bogus"},
			wantCode:   1,
			wantStderr: []string{"agent must be one of"},
		},
		{
			name:       "backup unknown agent rejects",
			args:       []string{"backup", "bogus"},
			wantCode:   1,
			wantStderr: []string{"agent must be one of"},
		},
		{
			name:       "backup opencode rejects",
			args:       []string{"backup", "opencode"},
			wantCode:   1,
			wantStderr: []string{"agent must be one of"},
		},
		{
			name:       "backup missing args",
			args:       []string{"backup"},
			wantCode:   1,
			wantStdout: []string{"Usage:"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, stdout, stderr, _, _ := newV2Root(t)
			code := root.Execute(tc.args)
			if code != tc.wantCode {
				t.Fatalf("Execute(%v) code=%d want %d (stdout=%q stderr=%q)",
					tc.args, code, tc.wantCode, stdout.String(), stderr.String())
			}
			for _, want := range tc.wantStdout {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("Execute(%v) stdout=%q missing %q", tc.args, stdout.String(), want)
				}
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("Execute(%v) stderr=%q missing %q", tc.args, stderr.String(), want)
				}
			}
		})
	}
}

// TestAddDuplicateRejects exercises the add -> add path, which the
// happy-path test in TestAdapterShortcutCommandsV3 doesn't touch.
func TestAddDuplicateRejects(t *testing.T) {
	root, stdout, stderr, _, _ := newV2Root(t)

	if code := root.Execute([]string{"add", "work", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("first add code=%d stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	if code := root.Execute([]string{"add", "work", "--base-url", "https://other.example/v1", "--api-key", "sk-b"}); code != 1 {
		t.Fatalf("duplicate add code=%d stderr=%q", code, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Profile work already exists") {
		t.Fatalf("duplicate add stderr=%q want 'already exists'", got)
	}
}

// TestRmActiveProfileRejects ensures rm declines to remove the current
// managed profile — the v0.1 contract that v0.2 must preserve.
func TestRmActiveProfileRejects(t *testing.T) {
	root, _, stderr, _, _ := newV2Root(t)

	if code := root.Execute([]string{"add", "work", "--base-url", "https://relay.example/v1", "--api-key", "sk-a"}); code != 0 {
		t.Fatalf("add code=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()

	if code := root.Execute([]string{"use", "work"}); code != 0 {
		t.Fatalf("use code=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()

	if code := root.Execute([]string{"rm", "work"}); code != 1 {
		t.Fatalf("rm current code=%d stderr=%q want 1", code, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "profile work is current") {
		t.Fatalf("rm current stderr=%q want 'is current'", got)
	}
}
