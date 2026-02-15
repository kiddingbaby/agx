package session

// SessionConfig holds configuration for creating a session.
type SessionConfig struct {
	Agent   string
	Dir     string
	Command string
	EnvVars map[string]string
}

// SessionInfo holds information about an active session.
type SessionInfo struct {
	Name      string
	Windows   int
	CreatedAt string
	Attached  bool
}
