package ports

import (
	domainagent "github.com/kiddingbaby/agx/internal/domain/agent"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

type UndoCaptureMeta struct {
	Command string
	Agent   domainagent.Agent
	Target  domainprovider.Target
}

type UndoRestoreResult struct {
	ID       string   `json:"id"`
	Restored []string `json:"restored,omitempty"`
	Deleted  []string `json:"deleted,omitempty"`
}

// UndoStore captures file-level snapshots before a switch and can restore them later.
//
// This is intentionally file-based and local-only.
type UndoStore interface {
	Capture(meta UndoCaptureMeta) (string, error)
	LatestID() (string, error)
	Restore(id string) (UndoRestoreResult, error)
}
