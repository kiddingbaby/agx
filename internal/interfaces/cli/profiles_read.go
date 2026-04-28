package cli

import (
	"fmt"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func (r *Root) handleList(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx ls [--agent codex|claude|gemini] [-o json]")
		return 0
	}

	agent, asJSON, hasAgent, ok := parseOptionalAgentFlag(r, args, "Usage: agx ls [--agent codex|claude|gemini] [-o json]")
	if !ok {
		return 1
	}

	profiles, err := r.profiles.List()
	if err != nil {
		r.printUserError(err)
		return 1
	}
	state, err := r.profiles.State()
	if err != nil {
		r.printUserError(err)
		return 1
	}

	views := make([]listProfileView, 0, len(profiles))
	for _, profile := range profiles {
		agents := relayAgents(profile.Name, state)
		current := false
		if hasAgent {
			current = bindingForAgent(state, agent).SourceProfile == profile.Name
		}
		views = append(views, listProfileView{
			Name:    profile.Name,
			BaseURL: profile.BaseURL,
			Agents:  agents,
			Current: current,
		})
	}

	if asJSON {
		if hasAgent {
			agentViews := make([]listAgentRelayView, 0, len(views))
			for _, relay := range views {
				agentViews = append(agentViews, listAgentRelayView{
					Name:    relay.Name,
					BaseURL: relay.BaseURL,
					Current: relay.Current,
				})
			}
			return r.writeJSON(struct {
				Agent        domainprofile.Agent `json:"agent"`
				CurrentRelay string              `json:"current_relay"`
				Relays       []listAgentRelayView `json:"relays"`
			}{
				Agent:        agent,
				CurrentRelay: bindingForAgent(state, agent).SourceProfile,
				Relays:       agentViews,
			})
		}
		return r.writeJSON(struct {
			Relays []listProfileView `json:"relays"`
		}{Relays: views})
	}

	if len(views) == 0 {
		fmt.Fprintln(r.stdout, "(no relays)")
		fmt.Fprintln(r.stdout, "Tip: run `agx add <relay> --base-url URL --api-key KEY`")
		return 0
	}

	if hasAgent {
		current := bindingForAgent(state, agent).SourceProfile
		fmt.Fprintf(r.stdout, "Agent: %s\n", agent)
		fmt.Fprintf(r.stdout, "Current: %s\n", renderCurrentRelay(current))
		for _, relay := range views {
			marker := " "
			if relay.Current {
				marker = "*"
			}
			fmt.Fprintf(r.stdout, "%s %s  base_url=%s\n", marker, relay.Name, relay.BaseURL)
		}
		return 0
	}

	fmt.Fprintln(r.stdout, "Relays:")
	for _, relay := range views {
		fmt.Fprintf(r.stdout, "  %s  base_url=%s agents=%s\n", relay.Name, relay.BaseURL, renderAgents(relay.Agents))
	}
	return 0
}

func (r *Root) handleShow(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx show <relay> [-o json]")
		return 0
	}

	name, asJSON, ok := parseNameWithJSON(r, args, "Usage: agx show <relay> [-o json]")
	if !ok {
		return 1
	}
	profile, err := r.profiles.Get(name)
	if err != nil {
		r.printUserError(err)
		return 1
	}
	state, err := r.profiles.State()
	if err != nil {
		r.printUserError(err)
		return 1
	}

	view := toProfileView(*profile)
	bindings := relayBindings(profile.Name, state)
	agents := relayAgents(profile.Name, state)

	if asJSON {
		return r.writeJSON(struct {
			Relay         profileView           `json:"relay"`
			Agents        []domainprofile.Agent `json:"agents"`
			AgentBindings []bindingView         `json:"agent_bindings"`
		}{
			Relay:         view,
			Agents:        agents,
			AgentBindings: bindings,
		})
	}

	fmt.Fprintf(r.stdout, "Relay: %s\n", view.Name)
	fmt.Fprintf(r.stdout, "  base_url=%s\n", view.BaseURL)
	fmt.Fprintf(r.stdout, "  api_key=%s\n", view.APIKey)
	fmt.Fprintf(r.stdout, "  agents=%s\n", renderAgents(agents))
	fmt.Fprintf(r.stdout, "  created_at=%s\n", view.CreatedAt.Format(timeFormat))
	fmt.Fprintf(r.stdout, "  updated_at=%s\n", view.UpdatedAt.Format(timeFormat))
	for _, binding := range bindings {
		fmt.Fprintf(r.stdout, "  %s status=%s config_path=%s", binding.Agent, binding.Status, binding.ConfigPath)
		if binding.LastBackupID != "" {
			fmt.Fprintf(r.stdout, " last_backup_id=%s", binding.LastBackupID)
		}
		if !binding.LastAppliedAt.IsZero() {
			fmt.Fprintf(r.stdout, " last_applied_at=%s", binding.LastAppliedAt.Format(timeFormat))
		}
		fmt.Fprintln(r.stdout)
	}
	return 0
}

func (r *Root) handleBackup(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx backup ls --agent codex|claude|gemini [-o json]")
		return 0
	}
	if len(args) == 0 || args[0] != "ls" {
		fmt.Fprintln(r.stderr, "Usage: agx backup ls --agent codex|claude|gemini [-o json]")
		return 1
	}

	agent, asJSON, ok := parseAgentOnlyFlagRequired(r, args[1:], "Usage: agx backup ls --agent codex|claude|gemini [-o json]")
	if !ok {
		return 1
	}
	backups, err := r.profiles.BackupList(agent)
	if err != nil {
		r.printUserError(err)
		return 1
	}
	views := make([]backupView, 0, len(backups))
	for _, backup := range backups {
		views = append(views, toBackupView(backup))
	}

	if asJSON {
		return r.writeJSON(struct {
			Agent   domainprofile.Agent `json:"agent"`
			Backups []backupView        `json:"backups"`
		}{
			Agent:   agent,
			Backups: views,
		})
	}

	if len(views) == 0 {
		fmt.Fprintf(r.stdout, "(no backups for %s)\n", agent)
		return 0
	}
	fmt.Fprintf(r.stdout, "Backups for %s:\n", agent)
	for _, backup := range views {
		fmt.Fprintf(r.stdout, "  backup_id=%s relay=%s restore_mode=%s config_path=%s", backup.ID, backup.AppliedRelay, backup.RestoreMode, backup.ConfigPath)
		if backup.BackupPath != "" {
			fmt.Fprintf(r.stdout, " backup_path=%s", backup.BackupPath)
		}
		if !backup.CreatedAt.IsZero() {
			fmt.Fprintf(r.stdout, " created_at=%s", backup.CreatedAt.Format(timeFormat))
		}
		fmt.Fprintln(r.stdout)
	}
	return 0
}

func (r *Root) handleDoctor(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx doctor [-o json]")
		return 0
	}

	asJSON, ok := parseJSONOnlyArgs(r, args, "Usage: agx doctor [-o json]")
	if !ok {
		return 1
	}

	report, err := r.profiles.Doctor()
	if err != nil {
		r.printUserError(err)
		return 1
	}
	if asJSON {
		if code := r.writeJSON(report); code != 0 {
			return code
		}
		if !report.OK {
			return 1
		}
		return 0
	}

	if report.OK {
		fmt.Fprintln(r.stdout, "Doctor: ok")
		return 0
	}

	fmt.Fprintln(r.stdout, "Doctor: issues found")
	if report.Operation != nil {
		fmt.Fprintf(r.stdout, "  unfinished_operation id=%s command=%s agent=%s stage=%s\n", report.Operation.ID, report.Operation.Command, report.Operation.Agent, report.Operation.Stage)
	}
	for _, issue := range report.Issues {
		fmt.Fprintf(r.stdout, "  [%s] %s: %s\n", issue.Severity, issue.Code, issue.Message)
	}
	return 1
}

func renderCurrentRelay(name string) string {
	if name == "" {
		return "-"
	}
	return name
}
