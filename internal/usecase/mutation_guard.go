package usecase

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

type mutationGuard struct {
	service        *ProfileService
	profilesBefore map[string]*domainprofile.Profile
	stateBefore    *domainprofile.State
	agentSnapshots map[domainprofile.Agent]ports.AgentConfigSnapshot
	active         bool
}

func newMutationGuard(service *ProfileService) *mutationGuard {
	return &mutationGuard{
		service:        service,
		profilesBefore: map[string]*domainprofile.Profile{},
		agentSnapshots: map[domainprofile.Agent]ports.AgentConfigSnapshot{},
		active:         true,
	}
}

func (g *mutationGuard) CaptureProfile(name string) error {
	name = domainprofile.NormalizeProfileName(name)
	if name == "" {
		return nil
	}
	if _, ok := g.profilesBefore[name]; ok {
		return nil
	}

	profile, err := g.service.profiles.Get(name)
	if err != nil {
		if IsProfileNotFoundError(err) {
			g.profilesBefore[name] = nil
			return nil
		}
		return err
	}

	copied := *profile
	g.profilesBefore[name] = &copied
	return nil
}

func (g *mutationGuard) CaptureState() error {
	if g.stateBefore != nil {
		return nil
	}

	state, err := g.service.loadStoredState()
	if err != nil {
		return err
	}
	copied := cloneState(state)
	g.stateBefore = &copied
	return nil
}

func (g *mutationGuard) CaptureAgents(agents ...domainprofile.Agent) error {
	for _, agent := range agents {
		if !agent.Valid() {
			continue
		}
		if _, ok := g.agentSnapshots[agent]; ok {
			continue
		}

		snapshot, ok, err := g.service.snapshotAgentConfig(agent)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		snapshot.Content = append([]byte(nil), snapshot.Content...)
		g.agentSnapshots[agent] = snapshot
	}
	return nil
}

// captureDerivedProfilesFor captures every per-agent derived profile YAML
// that is rooted in the given user profile name (e.g. "work" expands to
// "codex.work", "claude.work", "gemini.work", "opencode.work"). Without
// this the mutation guard's profile rollback cannot undo Upsert/Delete
// mutations to the derived profile store, which several managed-profile
// flows (Add / Edit / Remove / resync / rename) touch in a loop.
func (g *mutationGuard) captureDerivedProfilesFor(name string) error {
	name = domainprofile.NormalizeProfileName(name)
	if name == "" {
		return nil
	}
	for _, agent := range domainprofile.SupportedAgents() {
		derived := targetProfileName(agent, name)
		if derived == "" {
			continue
		}
		if err := g.CaptureProfile(derived); err != nil {
			return err
		}
	}
	return nil
}

func (g *mutationGuard) Commit() {
	g.active = false
}

func (g *mutationGuard) Rollback() error {
	if !g.active {
		return nil
	}

	var rollbackErrs []error
	for name, profile := range g.profilesBefore {
		if err := g.restoreProfile(name, profile); err != nil {
			rollbackErrs = append(rollbackErrs, err)
		}
	}
	for agent, snapshot := range g.agentSnapshots {
		if err := g.service.restoreAgentSnapshot(agent, snapshot); err != nil {
			rollbackErrs = append(rollbackErrs, err)
		}
	}
	if g.stateBefore != nil {
		if err := g.service.saveState(cloneState(*g.stateBefore)); err != nil {
			rollbackErrs = append(rollbackErrs, err)
		}
	}
	if err := g.service.clearCurrentOperationJournal(); err != nil {
		rollbackErrs = append(rollbackErrs, err)
	}

	g.active = false
	if len(rollbackErrs) == 0 {
		return nil
	}
	return fmt.Errorf("rollback failed: %w", errors.Join(rollbackErrs...))
}

func (g *mutationGuard) restoreProfile(name string, profile *domainprofile.Profile) error {
	if profile == nil {
		err := g.service.profiles.Delete(name)
		if err == nil || IsProfileNotFoundError(err) {
			return nil
		}
		return err
	}

	_, err := g.service.profiles.Upsert(*profile)
	return err
}

func (s *ProfileService) snapshotAgentConfig(agent domainprofile.Agent) (ports.AgentConfigSnapshot, bool, error) {
	if !agent.Valid() {
		return ports.AgentConfigSnapshot{}, false, &InvalidAgentError{Agent: string(agent)}
	}
	syncer := s.resolveAgentSyncer(agent)
	if syncer == nil {
		return ports.AgentConfigSnapshot{}, false, nil
	}
	snapshot, err := syncer.Snapshot()
	if err != nil {
		return ports.AgentConfigSnapshot{}, false, err
	}
	if snapshot == nil {
		return ports.AgentConfigSnapshot{}, false, nil
	}
	return *snapshot, true, nil
}

func (s *ProfileService) restoreAgentSnapshot(agent domainprofile.Agent, snapshot ports.AgentConfigSnapshot) error {
	if !agent.Valid() {
		return &InvalidAgentError{Agent: string(agent)}
	}
	syncer := s.resolveAgentSyncer(agent)
	if syncer == nil {
		return nil
	}
	if !snapshot.Exists {
		_, err := syncer.RemoveConfig()
		return err
	}

	tmpDir := filepath.Dir(snapshot.ConfigPath)
	if strings.TrimSpace(tmpDir) == "" || tmpDir == "." {
		tmpDir = ""
	}
	if tmpDir != "" {
		if err := os.MkdirAll(tmpDir, 0o700); err != nil {
			return err
		}
	}
	// Keep the rollback snapshot next to the destination config instead of
	// /tmp: the snapshot may contain an API key, and /tmp on a multi-user
	// host is world-traversable. Using the config's parent directory keeps
	// the credential inside the agx-owned 0700 tree.
	tmp, err := os.CreateTemp(tmpDir, ".agx-rollback-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(snapshot.Content); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	_, err = syncer.Restore(tmpPath)
	return err
}

func (s *ProfileService) clearCurrentOperationJournal() error {
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

// withMutationGuard wraps the lock/capture/commit/rollback dance shared by
// every mutating ProfileService entry point. captureFn records the pre-image
// the guard must be able to roll back to; actionFn performs the mutation.
// On error from actionFn (or panic via deferred Rollback) the guard restores
// the captured state and the original error is returned, joined with any
// rollback error.
func withMutationGuard[T any](
	s *ProfileService,
	captureFn func(*mutationGuard) error,
	actionFn func() (T, error),
) (result T, err error) {
	unlock, err := s.lockMutations()
	if err != nil {
		return result, err
	}
	defer unlock()

	guard := newMutationGuard(s)
	if err := captureFn(guard); err != nil {
		return result, err
	}

	defer finishMutationGuard(guard, &err)

	return actionFn()
}

// finishMutationGuard commits or rolls back a mutationGuard based on the
// captured error pointer. It also recovers panics so a runtime fault inside
// the mutation closure cannot leave the guarded state half-applied: any
// recovered panic triggers Rollback first and is then re-raised so the
// process still terminates with a non-zero exit.
func finishMutationGuard(guard *mutationGuard, errPtr *error) {
	if r := recover(); r != nil {
		if rollbackErr := guard.Rollback(); rollbackErr != nil && errPtr != nil {
			*errPtr = errors.Join(*errPtr, rollbackErr)
		}
		panic(r)
	}
	if errPtr != nil && *errPtr == nil {
		guard.Commit()
		return
	}
	if rollbackErr := guard.Rollback(); rollbackErr != nil && errPtr != nil {
		*errPtr = errors.Join(*errPtr, rollbackErr)
	}
}
