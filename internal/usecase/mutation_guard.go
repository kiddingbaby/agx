package usecase

import (
	"errors"
	"fmt"
	"os"

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
	var (
		snapshot *ports.AgentConfigSnapshot
		err      error
	)

	switch agent {
	case domainprofile.AgentCodex:
		if s.codex == nil {
			return ports.AgentConfigSnapshot{}, false, nil
		}
		snapshot, err = s.codex.Snapshot()
	case domainprofile.AgentClaude:
		if s.claude == nil {
			return ports.AgentConfigSnapshot{}, false, nil
		}
		snapshot, err = s.claude.Snapshot()
	case domainprofile.AgentGemini:
		if s.gemini == nil {
			return ports.AgentConfigSnapshot{}, false, nil
		}
		snapshot, err = s.gemini.Snapshot()
	default:
		return ports.AgentConfigSnapshot{}, false, &InvalidAgentError{Agent: string(agent)}
	}
	if err != nil {
		return ports.AgentConfigSnapshot{}, false, err
	}
	if snapshot == nil {
		return ports.AgentConfigSnapshot{}, false, nil
	}
	return *snapshot, true, nil
}

func (s *ProfileService) restoreAgentSnapshot(agent domainprofile.Agent, snapshot ports.AgentConfigSnapshot) error {
	if !snapshot.Exists {
		switch agent {
		case domainprofile.AgentCodex:
			if s.codex == nil {
				return nil
			}
			_, err := s.codex.RemoveConfig()
			return err
		case domainprofile.AgentClaude:
			if s.claude == nil {
				return nil
			}
			_, err := s.claude.RemoveConfig()
			return err
		case domainprofile.AgentGemini:
			if s.gemini == nil {
				return nil
			}
			_, err := s.gemini.RemoveConfig()
			return err
		default:
			return &InvalidAgentError{Agent: string(agent)}
		}
	}

	tmp, err := os.CreateTemp("", "agx-rollback-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(snapshot.Content); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	switch agent {
	case domainprofile.AgentCodex:
		if s.codex == nil {
			return nil
		}
		_, err = s.codex.Restore(tmpPath)
	case domainprofile.AgentClaude:
		if s.claude == nil {
			return nil
		}
		_, err = s.claude.Restore(tmpPath)
	case domainprofile.AgentGemini:
		if s.gemini == nil {
			return nil
		}
		_, err = s.gemini.Restore(tmpPath)
	default:
		return &InvalidAgentError{Agent: string(agent)}
	}
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
