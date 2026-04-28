package usecase

import (
	"fmt"
	"os"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
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

	state, err := s.State()
	if err != nil {
		return nil, err
	}

	s.checkCodexBindings(report, state, profileNames)
	s.checkAgentBindings(report, state, profileNames)
	if err := s.checkAgentBackups(report, state); err != nil {
		return nil, err
	}

	report.OK = len(report.Issues) == 0
	return report, nil
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
	report.addIssue("error", "unfinished_operation", fmt.Sprintf("unfinished %s operation %s for %s at stage %s", operation.Command, operation.ID, operation.Agent, operation.Stage))
	return nil
}

func (s *ProfileService) checkCodexBindings(report *DoctorReport, state domainprofile.State, profileNames map[string]struct{}) {
	if state.Codex.SourceProfile != "" {
		if _, ok := profileNames[state.Codex.SourceProfile]; !ok {
			report.addIssue("error", "missing_bound_profile", fmt.Sprintf("codex is bound to missing profile %s", state.Codex.SourceProfile))
		}
	}
	if !state.Codex.Status.Valid() {
		report.addIssue("error", "invalid_binding_status", fmt.Sprintf("codex has invalid status %s", state.Codex.Status))
	}

	for name, binding := range state.CodexProfiles {
		if _, ok := profileNames[name]; !ok {
			report.addIssue("error", "missing_bound_profile", fmt.Sprintf("codex manages missing profile %s", name))
		}
		if !binding.Status.Valid() {
			report.addIssue("error", "invalid_binding_status", fmt.Sprintf("codex profile %s has invalid status %s", name, binding.Status))
		}
	}
}

func (s *ProfileService) checkAgentBindings(report *DoctorReport, state domainprofile.State, profileNames map[string]struct{}) {
	for _, agent := range []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini} {
		binding := currentBinding(state, agent)
		if binding.SourceProfile != "" {
			if _, ok := profileNames[binding.SourceProfile]; !ok {
				report.addIssue("error", "missing_bound_profile", fmt.Sprintf("%s is bound to missing profile %s", agent, binding.SourceProfile))
			}
		}
		if !binding.Status.Valid() {
			report.addIssue("error", "invalid_binding_status", fmt.Sprintf("%s has invalid status %s", agent, binding.Status))
		}
	}
}

func (s *ProfileService) checkAgentBackups(report *DoctorReport, state domainprofile.State) error {
	for _, agent := range []domainprofile.Agent{domainprofile.AgentCodex, domainprofile.AgentClaude, domainprofile.AgentGemini} {
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
		report.addIssue("error", "invalid_restore_mode", fmt.Sprintf("%s backup %s has invalid restore mode %s", agent, backup.ID, backup.RestoreMode))
		return nil
	}
	if backup.RestoreMode != domainprofile.RestoreModeRestoreFile {
		return nil
	}
	if strings.TrimSpace(backup.BackupPath) == "" {
		report.addIssue("error", "missing_backup_path", fmt.Sprintf("%s backup %s is missing backup_path", agent, backup.ID))
		return nil
	}
	if _, err := os.Stat(backup.BackupPath); err != nil {
		if os.IsNotExist(err) {
			report.addIssue("error", "missing_backup_file", fmt.Sprintf("%s backup %s file is missing: %s", agent, backup.ID, backup.BackupPath))
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

func (r *DoctorReport) addIssue(severity, code, message string) {
	r.Issues = append(r.Issues, DoctorIssue{
		Severity: severity,
		Code:     code,
		Message:  message,
	})
}
