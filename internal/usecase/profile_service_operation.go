package usecase

import (
	"fmt"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

const (
	operationStageStarted       = "started"
	operationStageConfigWritten = "config_written"
)

func (s *ProfileService) lockMutations() (func(), error) {
	if s.mutationLocker == nil {
		return func() {}, nil
	}
	return s.mutationLocker.Lock()
}

func newOperationRecord(command string, agent domainprofile.Agent, profileName string, now time.Time) ports.OperationRecord {
	return ports.OperationRecord{
		ID:        fmt.Sprintf("%s-%s-%s", command, agent, now.UTC().Format("20060102T150405.000000000Z")),
		Command:   command,
		Agent:     agent,
		Profile:   profileName,
		Stage:     operationStageStarted,
		StartedAt: now,
		UpdatedAt: now,
	}
}

func (s *ProfileService) beginOperation(record ports.OperationRecord) error {
	if s.journal == nil {
		return nil
	}
	return s.journal.Begin(record)
}

func (s *ProfileService) updateOperation(record ports.OperationRecord) error {
	if s.journal == nil {
		return nil
	}
	return s.journal.Update(record)
}

func (s *ProfileService) clearOperation(id string) error {
	if s.journal == nil {
		return nil
	}
	return s.journal.Clear(id)
}
