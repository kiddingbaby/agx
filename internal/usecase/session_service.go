package usecase

import (
	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
	"github.com/kiddingbaby/agx/internal/ports"
)

// SessionService encapsulates session lifecycle use cases.
type SessionService struct {
	runtime ports.SessionRuntime
}

func NewSessionService(runtime ports.SessionRuntime) *SessionService {
	return &SessionService{runtime: runtime}
}

func (s *SessionService) List() ([]domainsession.SessionInfo, error) {
	sessions, err := s.runtime.ListSessions()
	if err != nil {
		return nil, WrapRuntimeError("list", err)
	}

	managed := make([]domainsession.SessionInfo, 0, len(sessions))
	for _, info := range sessions {
		if domainsession.IsManagedSessionName(info.Name) {
			managed = append(managed, info)
		}
	}
	return managed, nil
}

func (s *SessionService) Attach(sessionName string) error {
	return WrapRuntimeError("attach", s.runtime.Attach(domainsession.NormalizeSessionName(sessionName)))
}

func (s *SessionService) Kill(sessionName string) error {
	return WrapRuntimeError("kill", s.runtime.KillSession(domainsession.NormalizeSessionName(sessionName)))
}
