package ports

// AgentSyncer captures the 5 operations every agent-specific syncer must
// implement: pre-mutation snapshot, named on-disk backup, restore from
// backup, full config removal, and backup file deletion. Agent-specific
// interfaces (CodexSyncer / ClaudeSyncer / GeminiSyncer / OpenCodeSyncer)
// embed this and add their own Sync / Status surface on top.
//
// This is the resolver target: usecase code that doesn't care which agent
// it's talking to can take an AgentSyncer and call only these methods.
type AgentSyncer interface {
	Snapshot() (*AgentConfigSnapshot, error)
	CreateBackup(id string, content []byte) (string, error)
	Restore(backupPath string) (string, error)
	RemoveConfig() (string, error)
	DeleteBackup(backupPath string) error
}
