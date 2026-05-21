package usecase

import (
	"errors"
	"fmt"
	"os"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

const (
	severityError   = "error"
	severityWarning = "warning"
)

func (s *ProfileService) Doctor() (*DoctorReport, error) {
	report := &DoctorReport{
		OK:     true,
		Issues: []DoctorIssue{},
	}

	if err := s.checkCurrentOperation(report); err != nil {
		return nil, err
	}

	profiles, err := s.List()
	if err != nil {
		return nil, err
	}
	profileNames := indexProfileNames(profiles)

	state, codexIssues, err := s.loadDoctorState()
	if err != nil {
		return nil, err
	}
	var runtimeIssues []runtimeStateIssue
	if s.hasRuntimeManagedAgents() {
		runtimeIssues = s.enrichRuntimeState(&state, profiles)
	}

	s.checkCodexBindings(report, state, profileNames)
	s.checkAgentBindings(report, state, profileNames)
	s.checkUnconfiguredRelays(report, profiles, state)
	s.checkOrphanDerivedProfiles(report, profiles, state)
	s.checkOrphanManagedTargets(report, state, profileNames)
	s.checkProfileModelDrift(report, profiles, state)
	s.checkRuntimeStateIssues(report, state, codexIssues)
	s.checkRuntimeStateIssues(report, state, runtimeIssues)
	if err := s.checkAgentBackups(report, state); err != nil {
		return nil, err
	}

	report.OK = !reportHasErrors(report)
	return report, nil
}

func reportHasErrors(report *DoctorReport) bool {
	for _, issue := range report.Issues {
		if issue.Severity == severityError {
			return true
		}
	}
	return false
}

func hasV1ManagedTargets(state domainprofile.State, agent domainprofile.Agent) bool {
	managed, ok := state.ManagedAgents[agent]
	if !ok {
		return false
	}
	return len(managed.Targets) > 0
}

func (s *ProfileService) loadDoctorState() (domainprofile.State, []runtimeStateIssue, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return domainprofile.State{}, nil, err
	}
	if s.codex == nil {
	} else {
		status, err := s.codex.Status()
		if err != nil {
			var incomplete *ports.IncompleteManagedBlockError
			if errors.As(err, &incomplete) {
				return state, []runtimeStateIssue{runtimeSnapshotErrorIssue(domainprofile.AgentCodex, err)}, nil
			}
			return domainprofile.State{}, nil, err
		}

		applyResolvedCodexStatus(&state, status)
	}
	if s.openCode != nil {
		status, err := s.openCode.Status()
		if err != nil {
			return domainprofile.State{}, nil, err
		}
		applyResolvedOpenCodeStatus(&state, status)
	}
	return state, nil, nil
}

func (s *ProfileService) checkCurrentOperation(report *DoctorReport) error {
	if s.journal == nil {
		return nil
	}

	operation, err := s.journal.Current()
	if err != nil {
		return err
	}
	if operation == nil {
		return nil
	}

	report.Operation = operation
	report.addIssue(severityError, "unfinished_operation", fmt.Sprintf("unfinished %s operation %s for %s at stage %s", operation.Command, operation.ID, operation.Agent, operation.Stage), doctorActionForCurrentOperation(operation.Agent))
	return nil
}

func (s *ProfileService) checkCodexBindings(report *DoctorReport, state domainprofile.State, profileNames map[string]struct{}) {
	if state.Codex.SourceProfile != "" {
		if _, ok := profileNames[state.Codex.SourceProfile]; !ok {
			report.addIssue(severityError, "missing_bound_profile", fmt.Sprintf("codex is bound to missing profile %s", state.Codex.SourceProfile), doctorActionForMissingProfile(state.Codex.SourceProfile))
		}
	}
	if !state.Codex.Status.Valid() {
		report.addIssue(severityError, "invalid_binding_status", fmt.Sprintf("codex has invalid status %s", state.Codex.Status), doctorActionForManagedConfig(domainprofile.AgentCodex, state.Codex.SourceProfile))
	}
}

func (s *ProfileService) checkAgentBindings(report *DoctorReport, state domainprofile.State, profileNames map[string]struct{}) {
	for _, agent := range []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini, domainprofile.AgentOpenCode} {
		binding := currentBinding(state, agent)
		if binding.SourceProfile != "" {
			if _, ok := profileNames[binding.SourceProfile]; !ok {
				report.addIssue(severityError, "missing_bound_profile", fmt.Sprintf("%s is bound to missing profile %s", agent, binding.SourceProfile), doctorActionForMissingProfile(binding.SourceProfile))
			}
		}
		if !binding.Status.Valid() {
			report.addIssue(severityError, "invalid_binding_status", fmt.Sprintf("%s has invalid status %s", agent, binding.Status), doctorActionForManagedConfig(agent, binding.SourceProfile))
		}
	}
}

func (s *ProfileService) checkUnconfiguredRelays(report *DoctorReport, profiles []domainprofile.Profile, state domainprofile.State) {
	for _, profile := range profiles {
		if isReservedManagedProfileName(profile.Name) {
			continue
		}
		if len(boundAgents(profile.Name, state)) == 0 {
			report.addIssue(severityError, "unconfigured_relay", fmt.Sprintf("relay %s exists in AGX store but is not present in any agent config", profile.Name), doctorActionForUnconfiguredRelay(profile.Name))
		}
	}
}

func (s *ProfileService) checkOrphanDerivedProfiles(report *DoctorReport, profiles []domainprofile.Profile, state domainprofile.State) {
	for _, profile := range profiles {
		agent, targetName, ok := parseDerivedProfileName(profile.Name)
		if !ok {
			continue
		}
		managed, present := state.ManagedAgents[agent]
		if present && managed.Targets != nil {
			if _, hit := managed.Targets[targetName]; hit {
				continue
			}
		}
		report.addIssue(severityError, "orphan_derived_profile", fmt.Sprintf("derived profile %s exists in AGX store but no %s target %s references it", profile.Name, agent, targetName), doctorActionForOrphanDerived(profile.Name))
	}
}

func (s *ProfileService) checkOrphanManagedTargets(report *DoctorReport, state domainprofile.State, profileNames map[string]struct{}) {
	for agent, managed := range state.ManagedAgents {
		if !agent.Valid() || managed.Targets == nil {
			continue
		}
		for targetName, target := range managed.Targets {
			if target.Kind != domainprofile.TargetKindRelay {
				continue
			}
			if _, hit := profileNames[targetName]; hit {
				continue
			}
			derivedName := targetProfileName(agent, targetName)
			if _, hit := profileNames[derivedName]; hit {
				continue
			}
			report.addIssue(severityError, "orphan_managed_target", fmt.Sprintf("%s target %s references missing profile %s and derived %s", agent, targetName, targetName, derivedName), doctorActionForOrphanManagedTarget(agent, targetName))
		}
	}
}

func (s *ProfileService) checkProfileModelDrift(report *DoctorReport, profiles []domainprofile.Profile, state domainprofile.State) {
	index := make(map[string]domainprofile.Profile, len(profiles))
	for _, profile := range profiles {
		index[profile.Name] = profile
	}
	for agent, managed := range state.ManagedAgents {
		if !agent.Valid() || managed.Targets == nil {
			continue
		}
		for targetName, target := range managed.Targets {
			if target.Kind != domainprofile.TargetKindRelay {
				continue
			}
			user, ok := index[targetName]
			if !ok {
				continue
			}
			userModel := strings.TrimSpace(user.ModelID)
			targetModel := strings.TrimSpace(target.Relay.ModelID)
			if userModel == "" || targetModel == "" || userModel == targetModel {
				continue
			}
			report.addIssue(severityError, "model_id_drift", fmt.Sprintf("%s target %s model %q differs from profile %s model %q", agent, targetName, targetModel, targetName, userModel), doctorActionForModelDrift(agent, targetName))
		}
	}
}

func (s *ProfileService) checkRuntimeStateIssues(report *DoctorReport, state domainprofile.State, issues []runtimeStateIssue) {
	for _, issue := range issues {
		if strings.TrimSpace(issue.Code) == "" || strings.TrimSpace(issue.Message) == "" {
			continue
		}
		severity := severityError
		if issue.Code == "runtime_config_unreadable" && issue.Agent.Valid() && hasV1ManagedTargets(state, issue.Agent) {
			severity = severityWarning
		}
		report.addIssue(severity, issue.Code, issue.Message, issue.Action)
	}
}

func (s *ProfileService) checkAgentBackups(report *DoctorReport, state domainprofile.State) error {
	for _, agent := range []domainprofile.Agent{domainprofile.AgentCodex, domainprofile.AgentClaude, domainprofile.AgentGemini, domainprofile.AgentOpenCode} {
		binding := currentBinding(state, agent)
		for _, backup := range binding.Backups {
			if err := checkBackupMetadata(report, agent, backup); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkBackupMetadata(report *DoctorReport, agent domainprofile.Agent, backup domainprofile.Backup) error {
	if !backup.RestoreMode.Valid() {
		report.addIssue(severityError, "invalid_restore_mode", fmt.Sprintf("%s backup %s has invalid restore mode %s", agent, backup.ID, backup.RestoreMode), doctorActionForBackupIssue(agent))
		return nil
	}
	if backup.RestoreMode != domainprofile.RestoreModeRestoreFile {
		return nil
	}
	if strings.TrimSpace(backup.BackupPath) == "" {
		report.addIssue(severityError, "missing_backup_path", fmt.Sprintf("%s backup %s is missing backup_path", agent, backup.ID), doctorActionForBackupIssue(agent))
		return nil
	}
	if _, err := os.Stat(backup.BackupPath); err != nil {
		if os.IsNotExist(err) {
			report.addIssue(severityError, "missing_backup_file", fmt.Sprintf("%s backup %s file is missing: %s", agent, backup.ID, backup.BackupPath), doctorActionForBackupIssue(agent))
			return nil
		}
		return err
	}
	if agent != domainprofile.AgentCodex {
		return nil
	}
	return nil
}

func indexProfileNames(profiles []domainprofile.Profile) map[string]struct{} {
	names := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		names[profile.Name] = struct{}{}
	}
	return names
}

func (r *DoctorReport) addIssue(severity, code, message, action string) {
	r.Issues = append(r.Issues, DoctorIssue{
		Severity: severity,
		Code:     code,
		Message:  message,
		Action:   strings.TrimSpace(action),
	})
}

func doctorActionForCurrentOperation(agent domainprofile.Agent) string {
	return fmt.Sprintf("retry the interrupted %s operation or run `agx restore %s` if the config changed", agent, agent)
}

func doctorActionForMissingProfile(profile string) string {
	return fmt.Sprintf("recreate profile %s or remove the stale binding, then rerun `agx doctor`", profile)
}

func doctorActionForManagedConfig(agent domainprofile.Agent, profile string) string {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return fmt.Sprintf("run `agx restore %s` and rerun `agx doctor`", agent)
	}
	return fmt.Sprintf("run `agx %s %s` again to refresh the managed config, then rerun `agx doctor`", agent, profile)
}

func doctorActionForUnconfiguredRelay(profile string) string {
	return fmt.Sprintf("bind relay %s to at least one agent, then rerun `agx doctor`", profile)
}

func doctorActionForOrphanDerived(profile string) string {
	return fmt.Sprintf("derived profile %s has no managed target; will be cleanable via `agx repair` once available", profile)
}

func doctorActionForOrphanManagedTarget(agent domainprofile.Agent, name string) string {
	return fmt.Sprintf("%s target %s references no profile in store; run `agx add %s` to recreate it or remove the stale target", agent, name, name)
}

func doctorActionForModelDrift(agent domainprofile.Agent, name string) string {
	return fmt.Sprintf("run `agx %s %s` to re-sync target %s with the current profile model", agent, name, name)
}

func doctorActionForBackupIssue(agent domainprofile.Agent) string {
	return fmt.Sprintf("recreate the %s backup by relaunching the agent, then rerun `agx doctor`", agent)
}
