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

// takeoverStaleOperation clears any pre-existing operation journal entry so
// the next call to beginOperation does not fail with "unfinished AGX
// operation detected". Recovery commands (agx restore) need to make forward
// progress even when the previous command crashed mid-flight; without this
// the user gets stuck in a loop where doctor recommends `agx restore` and
// `agx restore` refuses to run because the stale entry is still present.
func (s *ProfileService) takeoverStaleOperation() error {
	if s.journal == nil {
		return nil
	}
	current, err := s.journal.Current()
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	return s.journal.Clear(current.ID)
}
