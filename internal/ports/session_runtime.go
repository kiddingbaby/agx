package ports

import domainsession "github.com/kiddingbaby/agx/internal/domain/session"

// SessionRuntime defines session lifecycle operations.
type SessionRuntime interface {
	Launch(cfg domainsession.SessionConfig) error
	Attach(sessionName string) error
	ListSessions() ([]domainsession.SessionInfo, error)
	KillSession(name string) error
}
