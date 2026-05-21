package usecase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func (s *ProfileService) ManagedAgent(agent domainprofile.Agent) (domainprofile.ManagedAgentState, error) {
	if !agent.Valid() {
		return domainprofile.ManagedAgentState{}, &InvalidAgentError{Agent: string(agent)}
	}
	state, err := s.loadStoredState()
	if err != nil {
		return domainprofile.ManagedAgentState{}, err
	}
	managed := managedAgentState(state, agent)
	managed = s.hydrateManagedAgentDefaults(agent, managed)
	return managed, nil
}

func (s *ProfileService) CurrentTargetContext(agent domainprofile.Agent) (domainprofile.CurrentTarget, string, error) {
	if !agent.Valid() {
		return domainprofile.CurrentTarget{}, "", &InvalidAgentError{Agent: string(agent)}
	}
	managed, err := s.ManagedAgent(agent)
	if err != nil {
		return domainprofile.CurrentTarget{}, "", err
	}
	if managed.CurrentTarget.Kind == "" || strings.TrimSpace(managed.CurrentTarget.Name) == "" {
		return domainprofile.CurrentTarget{}, "", &NoCurrentTargetError{Agent: agent}
	}
	path, err := s.targetContextPath(agent, managed, managed.CurrentTarget.Kind, managed.CurrentTarget.Name)
	if err != nil {
		return domainprofile.CurrentTarget{}, "", err
	}
	return managed.CurrentTarget, path, nil
}

func (s *ProfileService) BackupManagedTarget(agent domainprofile.Agent, kind domainprofile.TargetKind, name string) (domainprofile.ContextBackup, error) {
	if !agent.Valid() {
		return domainprofile.ContextBackup{}, &InvalidAgentError{Agent: string(agent)}
	}

	return withMutationGuard(s,
		func(g *mutationGuard) error { return g.CaptureState() },
		func() (domainprofile.ContextBackup, error) {
			return s.backupManagedTargetLocked(agent, kind, name)
		},
	)
}

func (s *ProfileService) backupManagedTargetLocked(agent domainprofile.Agent, kind domainprofile.TargetKind, name string) (domainprofile.ContextBackup, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return domainprofile.ContextBackup{}, err
	}
	managed := s.hydrateManagedAgentDefaults(agent, managedAgentState(state, agent))
	resolvedKind, resolvedName, root, backups, err := s.resolveManagedTarget(agent, managed, kind, name)
	if err != nil {
		return domainprofile.ContextBackup{}, err
	}
	if len(backups) == 0 {
		backups = []domainprofile.ContextBackup{}
	}
	if err := s.ensureManagedContextRoot(root); err != nil {
		return domainprofile.ContextBackup{}, err
	}

	now := time.Now().UTC()
	backupID := nextContextBackupID(backups, now)
	snapshotRoot := filepath.Join(s.managedBackupsRoot(agent, resolvedKind, resolvedName), backupID)
	if err := copyDir(snapshotRoot, root); err != nil {
		return domainprofile.ContextBackup{}, err
	}
	backup := domainprofile.ContextBackup{
		ID:         backupID,
		TargetKind: resolvedKind,
		TargetName: resolvedName,
		Path:       snapshotRoot,
		CreatedAt:  now,
	}
	if err := s.updateManagedBackups(&state, agent, resolvedKind, resolvedName, backup); err != nil {
		// Best-effort cleanup of the orphaned snapshot directory: state did
		// not pick it up, so leaving it on disk would just waste space.
		_ = os.RemoveAll(snapshotRoot)
		return domainprofile.ContextBackup{}, err
	}
	state.UpdatedAt = now
	if err := s.saveState(state); err != nil {
		_ = os.RemoveAll(snapshotRoot)
		return domainprofile.ContextBackup{}, err
	}
	return backup, nil
}

func (s *ProfileService) RestoreManagedTarget(agent domainprofile.Agent, kind domainprofile.TargetKind, name, backupID string) (domainprofile.ContextBackup, error) {
	if !agent.Valid() {
		return domainprofile.ContextBackup{}, &InvalidAgentError{Agent: string(agent)}
	}

	return withMutationGuard(s,
		func(g *mutationGuard) error { return g.CaptureState() },
		func() (domainprofile.ContextBackup, error) {
			return s.restoreManagedTargetLocked(agent, kind, name, backupID)
		},
	)
}

func (s *ProfileService) restoreManagedTargetLocked(agent domainprofile.Agent, kind domainprofile.TargetKind, name, backupID string) (domainprofile.ContextBackup, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return domainprofile.ContextBackup{}, err
	}
	managed := s.hydrateManagedAgentDefaults(agent, managedAgentState(state, agent))
	resolvedKind, resolvedName, root, backups, err := s.resolveManagedTarget(agent, managed, kind, name)
	if err != nil {
		return domainprofile.ContextBackup{}, err
	}
	if len(backups) == 0 {
		return domainprofile.ContextBackup{}, &NoBackupError{Agent: agent}
	}
	backup, err := selectContextBackup(backups, resolvedName, backupID)
	if err != nil {
		return domainprofile.ContextBackup{}, err
	}
	if strings.TrimSpace(backup.Path) == "" {
		return domainprofile.ContextBackup{}, &BackupNotFoundError{ID: backup.ID}
	}
	if err := replaceDirContents(root, backup.Path); err != nil {
		return domainprofile.ContextBackup{}, err
	}

	managed = managedAgentState(state, agent)
	managed = s.hydrateManagedAgentDefaults(agent, managed)
	target, ok := managed.Targets[resolvedName]
	if !ok || target.Kind != resolvedKind {
		return domainprofile.ContextBackup{}, &TargetNotFoundError{Agent: agent, Name: resolvedName}
	}
	target.ContextPath = root
	target.ConfigPath = managedContextConfigPath(agent, root)
	target.UpdatedAt = time.Now().UTC()
	managed.Targets[resolvedName] = target
	managed.UpdatedAt = time.Now().UTC()
	assignManagedAgentState(&state, agent, managed)
	state.UpdatedAt = managed.UpdatedAt
	if err := s.saveState(state); err != nil {
		return domainprofile.ContextBackup{}, err
	}
	return backup, nil
}

func (s *ProfileService) RestoreCurrentTarget(agent domainprofile.Agent) (domainprofile.ContextBackup, error) {
	return s.RestoreManagedTarget(agent, "", "", "")
}

func (s *ProfileService) hydrateManagedAgentDefaults(agent domainprofile.Agent, managed domainprofile.ManagedAgentState) domainprofile.ManagedAgentState {
	managed.Targets = ensureTargets(managed.Targets)
	_ = agent
	return managed
}

func (s *ProfileService) ensureManagedRuntimeReady(agent domainprofile.Agent) error {
	if strings.TrimSpace(s.managedPaths.ContextsDir) == "" {
		return &ManagedRuntimeUnavailableError{Reason: "contexts directory is not configured"}
	}
	switch agent {
	case domainprofile.AgentCodex:
		if s.managedSyncers.NewCodex == nil {
			return &ManagedRuntimeUnavailableError{Agent: agent}
		}
	case domainprofile.AgentClaude:
		if s.managedSyncers.NewClaude == nil {
			return &ManagedRuntimeUnavailableError{Agent: agent}
		}
	case domainprofile.AgentGemini:
		if s.managedSyncers.NewGemini == nil {
			return &ManagedRuntimeUnavailableError{Agent: agent}
		}
	case domainprofile.AgentOpenCode:
		if s.managedSyncers.NewOpenCode == nil {
			return &ManagedRuntimeUnavailableError{Agent: agent}
		}
	}
	return nil
}

func (s *ProfileService) syncRelayContext(agent domainprofile.Agent, root string, profile domainprofile.Profile, binding *domainprofile.OpenCodeProfileBinding) (string, *domainprofile.OpenCodeProfileBinding, error) {
	switch agent {
	case domainprofile.AgentCodex:
		syncer := s.managedCodexSyncer(root)
		if syncer == nil {
			return "", nil, &ManagedRuntimeUnavailableError{Agent: agent}
		}
		result, err := syncer.Sync(profile, ports.CodexSyncOptions{
			DefaultProfileName: profile.Name,
			WireAPI:            profile.CodexWireAPI,
		})
		if err != nil {
			return "", nil, err
		}
		return result.ConfigPath, nil, nil
	case domainprofile.AgentClaude:
		syncer := s.managedClaudeSyncer(root)
		if syncer == nil {
			return "", nil, &ManagedRuntimeUnavailableError{Agent: agent}
		}
		result, err := syncer.Sync(profile)
		if err != nil {
			return "", nil, err
		}
		return result.ConfigPath, nil, nil
	case domainprofile.AgentGemini:
		syncer := s.managedGeminiSyncer(root)
		if syncer == nil {
			return "", nil, &ManagedRuntimeUnavailableError{Agent: agent}
		}
		result, err := syncer.Sync(profile)
		if err != nil {
			return "", nil, err
		}
		return result.ConfigPath, nil, nil
	case domainprofile.AgentOpenCode:
		syncer := s.managedOpenCodeSyncer(root)
		if syncer == nil {
			return "", nil, &ManagedRuntimeUnavailableError{Agent: agent}
		}
		if binding != nil && strings.TrimSpace(binding.ModelID) == "" {
			target := profile.Name
			if _, t, ok := parseDerivedProfileName(profile.Name); ok {
				target = t
			}
			return "", nil, fmt.Errorf("opencode model is required; run `agx edit %s --model <id>` before launching opencode", target)
		}
		normalized := normalizeOpenCodeProfileBinding(profile.Name, binding)
		if err := domainprofile.ValidateOpenCodeModelID(normalized.ModelID); err != nil {
			target := profile.Name
			if _, t, ok := parseDerivedProfileName(profile.Name); ok {
				target = t
			}
			return "", nil, fmt.Errorf("%w; run `agx edit %s --model <id>` before launching opencode", err, target)
		}
		result, err := syncer.Sync(profile, ports.OpenCodeSyncOptions{
			ProviderFamily: normalized.ProviderFamily,
			ModelID:        normalized.ModelID,
			ModelName:      normalized.ModelName,
			ProviderName:   profile.Name,
			SetAsCurrent:   true,
		})
		if err != nil {
			return "", nil, err
		}
		return result.ConfigPath, &normalized, nil
	default:
		return "", nil, &InvalidAgentError{Agent: string(agent)}
	}
}

func (s *ProfileService) targetContextPath(agent domainprofile.Agent, managed domainprofile.ManagedAgentState, kind domainprofile.TargetKind, name string) (string, error) {
	managed.Targets = ensureTargets(managed.Targets)
	name = normalizeTargetName(name)
	if target, ok := managed.Targets[name]; ok && target.Kind == kind {
		if target.ContextPath != "" {
			return target.ContextPath, nil
		}
		return s.managedTargetContextRoot(agent, name), nil
	}
	switch kind {
	case domainprofile.TargetKindRelay:
		return "", &TargetNotFoundError{Agent: agent, Name: name}
	default:
		return "", &NoCurrentTargetError{Agent: agent}
	}
}

func (s *ProfileService) resolveManagedTarget(agent domainprofile.Agent, managed domainprofile.ManagedAgentState, kind domainprofile.TargetKind, name string) (domainprofile.TargetKind, string, string, []domainprofile.ContextBackup, error) {
	managed.Targets = ensureTargets(managed.Targets)
	resolvedKind := kind
	resolvedName := normalizeTargetName(name)
	if resolvedKind == "" && resolvedName != "" {
		target, ok := managed.Targets[resolvedName]
		if !ok {
			return "", "", "", nil, &TargetNotFoundError{Agent: agent, Name: resolvedName}
		}
		resolvedKind = target.Kind
	} else if resolvedKind == "" {
		if managed.CurrentTarget.Kind == "" || strings.TrimSpace(managed.CurrentTarget.Name) == "" {
			return "", "", "", nil, &NoCurrentTargetError{Agent: agent}
		}
		resolvedKind = managed.CurrentTarget.Kind
		resolvedName = managed.CurrentTarget.Name
	}
	root, err := s.targetContextPath(agent, managed, resolvedKind, resolvedName)
	if err != nil {
		return "", "", "", nil, err
	}
	if resolvedKind == domainprofile.TargetKindRelay {
		if target, ok := managed.Targets[resolvedName]; ok {
			return resolvedKind, resolvedName, root, cloneContextBackups(target.Backups), nil
		}
		return "", "", "", nil, &TargetNotFoundError{Agent: agent, Name: resolvedName}
	}
	return "", "", "", nil, &NoCurrentTargetError{Agent: agent}
}

func (s *ProfileService) updateManagedBackups(state *domainprofile.State, agent domainprofile.Agent, kind domainprofile.TargetKind, name string, backup domainprofile.ContextBackup) error {
	managed := s.hydrateManagedAgentDefaults(agent, managedAgentState(*state, agent))
	if target, ok := managed.Targets[normalizeTargetName(name)]; ok && target.Kind == kind {
		if err := prependContextBackup(&target.Backups, backup); err != nil {
			return err
		}
		target.UpdatedAt = time.Now().UTC()
		managed.Targets[normalizeTargetName(name)] = target
		managed.UpdatedAt = time.Now().UTC()
		assignManagedAgentState(state, agent, managed)
		return nil
	}
	return fmt.Errorf("%s target %s not found", agent, normalizeTargetName(name))
}
