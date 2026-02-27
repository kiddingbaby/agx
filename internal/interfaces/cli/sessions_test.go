package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
	"github.com/kiddingbaby/agx/internal/usecase"
)

type fakeListRuntime struct {
	sessions []domainsession.SessionInfo
	listErr  error
}

func (f *fakeListRuntime) Launch(domainsession.SessionConfig) error {
	return nil
}

func (f *fakeListRuntime) Attach(string) error {
	return nil
}

func (f *fakeListRuntime) ListSessions() ([]domainsession.SessionInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]domainsession.SessionInfo, len(f.sessions))
	copy(out, f.sessions)
	return out, nil
}

func (f *fakeListRuntime) KillSession(string) error {
	return nil
}

func newListRoot(t *testing.T, runtime *fakeListRuntime) (*Root, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	sessionSvc := usecase.NewSessionService(runtime)
	root := New(nil, sessionSvc, nil, Handlers{})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.stdout = stdout
	root.stderr = stderr
	return root, stdout, stderr
}

func TestExecuteListJSONNoSessions(t *testing.T) {
	root, stdout, stderr := newListRoot(t, &fakeListRuntime{})

	if code := root.Execute([]string{"ls", "--json"}); code != 0 {
		t.Fatalf("Execute() code = %d, want 0", code)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
	if got := stdout.String(); got != "{\"sessions\":[]}\n" {
		t.Fatalf("stdout = %q, want %q", got, "{\"sessions\":[]}\\n")
	}
}

func TestExecuteListJSONWithSessions(t *testing.T) {
	root, stdout, stderr := newListRoot(t, &fakeListRuntime{
		sessions: []domainsession.SessionInfo{
			{Name: "ai-claude-myrepo", Windows: 1, Attached: false},
		},
	})

	if code := root.Execute([]string{"ls", "--json"}); code != 0 {
		t.Fatalf("Execute() code = %d, want 0", code)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}

	var payload struct {
		Sessions []struct {
			Name     string `json:"name"`
			Windows  int    `json:"windows"`
			Attached bool   `json:"attached"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(payload.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(payload.Sessions))
	}
	got := payload.Sessions[0]
	if got.Name != "ai-claude-myrepo" || got.Windows != 1 || got.Attached {
		t.Fatalf("session = %+v, want {Name:ai-claude-myrepo Windows:1 Attached:false}", got)
	}
}

func TestExecuteListTextOutputCompatible(t *testing.T) {
	root, stdout, stderr := newListRoot(t, &fakeListRuntime{
		sessions: []domainsession.SessionInfo{
			{Name: "ai-codex-cli", Windows: 2, Attached: true},
		},
	})

	if code := root.Execute([]string{"ls"}); code != 0 {
		t.Fatalf("Execute() code = %d, want 0", code)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}

	want := "Active AI sessions:\n  ai-codex-cli  2 windows (attached)\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestExecuteListRejectsUnknownFlag(t *testing.T) {
	root, stdout, stderr := newListRoot(t, &fakeListRuntime{})

	if code := root.Execute([]string{"ls", "--invalid"}); code != 1 {
		t.Fatalf("Execute() code = %d, want 1", code)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
	if got := stderr.String(); got != "Usage: agx ls [--json]\n" {
		t.Fatalf("stderr = %q, want usage", got)
	}
}
