package usecase

import (
	"errors"
	"testing"

	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
)

type fakeSessionRuntime struct {
	listErr   error
	attachErr error
	killErr   error

	sessions []domainsession.SessionInfo

	attachCalls []string
	killCalls   []string
}

func (f *fakeSessionRuntime) Launch(cfg domainsession.SessionConfig) error {
	return nil
}

func (f *fakeSessionRuntime) Attach(sessionName string) error {
	f.attachCalls = append(f.attachCalls, sessionName)
	return f.attachErr
}

func (f *fakeSessionRuntime) ListSessions() ([]domainsession.SessionInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]domainsession.SessionInfo, len(f.sessions))
	copy(out, f.sessions)
	return out, nil
}

func (f *fakeSessionRuntime) KillSession(name string) error {
	f.killCalls = append(f.killCalls, name)
	return f.killErr
}

func TestSessionServiceListFiltersManagedSessions(t *testing.T) {
	rt := &fakeSessionRuntime{
		sessions: []domainsession.SessionInfo{
			{Name: "ai-claude", Windows: 2},
			{Name: "project", Windows: 1},
			{Name: "ai-codex-cli", Windows: 1},
		},
	}
	svc := NewSessionService(rt)

	got, err := svc.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() len = %d, want 2", len(got))
	}
	if got[0].Name != "ai-claude" || got[1].Name != "ai-codex-cli" {
		t.Fatalf("List() got = %+v", got)
	}
}

func TestSessionServiceAttachAndKillNormalizeNames(t *testing.T) {
	rt := &fakeSessionRuntime{}
	svc := NewSessionService(rt)

	if err := svc.Attach("codex-cli"); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if len(rt.attachCalls) != 1 || rt.attachCalls[0] != "ai-codex-cli" {
		t.Fatalf("attach calls = %v", rt.attachCalls)
	}

	if err := svc.Kill("ai-codex-cli"); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	if len(rt.killCalls) != 1 || rt.killCalls[0] != "ai-codex-cli" {
		t.Fatalf("kill calls = %v", rt.killCalls)
	}
}

func TestSessionServiceErrors(t *testing.T) {
	rt := &fakeSessionRuntime{
		listErr:   errors.New("list failed"),
		attachErr: errors.New("attach failed"),
		killErr:   errors.New("kill failed"),
	}
	svc := NewSessionService(rt)

	if _, err := svc.List(); err == nil {
		t.Fatal("List() should return error")
	} else {
		if !IsRuntimeError(err) {
			t.Fatalf("List() err = %v, want RuntimeError", err)
		}
		if !errors.Is(err, rt.listErr) {
			t.Fatalf("List() err should unwrap listErr, got %v", err)
		}
	}
	if err := svc.Attach("test"); err == nil {
		t.Fatal("Attach() should return error")
	} else {
		if !IsRuntimeError(err) {
			t.Fatalf("Attach() err = %v, want RuntimeError", err)
		}
		if !errors.Is(err, rt.attachErr) {
			t.Fatalf("Attach() err should unwrap attachErr, got %v", err)
		}
	}
	if err := svc.Kill("test"); err == nil {
		t.Fatal("Kill() should return error")
	} else {
		if !IsRuntimeError(err) {
			t.Fatalf("Kill() err = %v, want RuntimeError", err)
		}
		if !errors.Is(err, rt.killErr) {
			t.Fatalf("Kill() err should unwrap killErr, got %v", err)
		}
	}
}
