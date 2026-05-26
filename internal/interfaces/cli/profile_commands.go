package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/mcpgateway"
	"github.com/kiddingbaby/agx/internal/mcpinject"
	"github.com/kiddingbaby/agx/internal/usecase"
	"github.com/spf13/cobra"
)

type managedProfileView struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Current        bool     `json:"current"`
	BaseURL        string   `json:"base_url,omitempty"`
	APIKey         string   `json:"api_key,omitempty"`
	CredentialRef  string   `json:"credential_ref,omitempty"`
	Model          string   `json:"model,omitempty"`
	CodexWireAPI   string   `json:"codex_wire_api,omitempty"`
	ProviderFamily string   `json:"provider_family,omitempty"`
	Agents         []string `json:"agents,omitempty"`
}

func (r *Root) newProfileAddCommand() *cobra.Command {
	var (
		baseURL      string
		apiKey       string
		model        string
		codexWireAPI string
	)
	cmd := &cobra.Command{
		Use:           "add <profile>",
		Short:         "Create a relay profile",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			profile, err := r.profiles.AddManagedProfile(args[0], domainprofile.Profile{
				Kind:         domainprofile.ProfileKindRelay,
				BaseURL:      baseURL,
				APIKey:       apiKey,
				ModelID:      model,
				CodexWireAPI: domainprofile.CodexWireAPI(codexWireAPI),
			})
			if err != nil {
				return r.reportError(err)
			}
			view := toManagedProfileView(*profile, false)
			if asJSON {
				return r.emitJSON(struct {
					Profile managedProfileView `json:"profile"`
				}{Profile: view})
			}
			fmt.Fprintf(r.stdout, "profile %s created\n", profile.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&baseURL, "base-url", "", "relay base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "inline relay API key")
	cmd.Flags().StringVar(&model, "model", "", "default model for profile launchers")
	cmd.Flags().StringVar(&codexWireAPI, "codex-wire-api", "", "codex wire api: chat | responses (default responses)")
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newProfileEditCommand() *cobra.Command {
	var (
		name         string
		baseURL      string
		apiKey       string
		model        string
		codexWireAPI string
	)
	cmd := &cobra.Command{
		Use:           "edit <profile>",
		Short:         "Update a relay profile",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("base-url") && !cmd.Flags().Changed("api-key") && !cmd.Flags().Changed("model") && !cmd.Flags().Changed("codex-wire-api") {
				return r.usageError(cmd)
			}
			existing, err := r.profiles.ManagedProfile(args[0])
			if err != nil {
				return r.reportError(err)
			}
			if existing.Kind != domainprofile.ProfileKindRelay {
				return r.reportError(fmt.Errorf("only relay profiles can be edited in V0.1"))
			}
			var input usecase.UpdateManagedProfileInput
			if cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if cmd.Flags().Changed("base-url") {
				input.BaseURL = &baseURL
			}
			if cmd.Flags().Changed("api-key") {
				input.APIKey = &apiKey
			}
			if cmd.Flags().Changed("model") {
				input.ModelID = &model
			}
			if cmd.Flags().Changed("codex-wire-api") {
				wire := domainprofile.CodexWireAPI(codexWireAPI)
				input.CodexWireAPI = &wire
			}
			result, err := r.profiles.EditManagedProfile(args[0], input)
			if err != nil {
				return r.reportError(err)
			}
			current, _ := r.profiles.CurrentManagedProfile()
			isCurrent := current != nil && current.Name == result.Profile.Name
			view := toManagedProfileView(*result.Profile, isCurrent)
			if asJSON {
				return r.emitJSON(struct {
					Profile         managedProfileView          `json:"profile"`
					ResyncedTargets []usecase.TargetSyncOutcome `json:"resynced_targets,omitempty"`
					FailedTargets   []usecase.TargetSyncFailure `json:"failed_targets,omitempty"`
				}{Profile: view, ResyncedTargets: result.ResyncedTargets, FailedTargets: result.FailedTargets})
			}
			fmt.Fprintf(r.stdout, "profile %s updated\n", result.Profile.Name)
			for _, outcome := range result.ResyncedTargets {
				fmt.Fprintf(r.stdout, "  resynced %s target %s\n", outcome.Agent, outcome.TargetName)
			}
			for _, failure := range result.FailedTargets {
				fmt.Fprintf(r.stdout, "  failed to resync %s target %s: %s\n", failure.Agent, failure.TargetName, failure.Error)
			}
			if len(result.FailedTargets) > 0 {
				return exitCodeError{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new profile name")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "relay base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "inline relay API key")
	cmd.Flags().StringVar(&model, "model", "", "default model for profile launchers")
	cmd.Flags().StringVar(&codexWireAPI, "codex-wire-api", "", "codex wire api: chat | responses (default responses)")
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newProfileRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "rm <profile>",
		Aliases:       []string{"remove"},
		Short:         "Remove a managed profile",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			profile, err := r.profiles.RemoveManagedProfile(args[0])
			if err != nil {
				return r.reportError(err)
			}
			view := toManagedProfileView(*profile, false)
			if asJSON {
				return r.emitJSON(struct {
					Profile managedProfileView `json:"profile"`
				}{Profile: view})
			}
			fmt.Fprintf(r.stdout, "profile %s removed\n", profile.Name)
			return nil
		},
	}
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newProfileListCommand() *cobra.Command {
	var includeAll bool
	cmd := &cobra.Command{
		Use:           "ls",
		Aliases:       []string{"list"},
		Short:         "List managed profiles",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 0)
			if err != nil {
				return err
			}
			var (
				summaries []usecase.ManagedProfileSummary
				current   string
			)
			if includeAll {
				summaries, current, err = r.profiles.ListAllProfiles()
			} else {
				summaries, current, err = r.profiles.ListManagedProfiles()
			}
			if err != nil {
				return r.reportError(err)
			}
			views := make([]managedProfileView, 0, len(summaries))
			for _, summary := range summaries {
				view := toManagedProfileView(summary.Profile, summary.Profile.Name == current)
				view.Agents = agentsToStrings(summary.BoundAgents)
				views = append(views, view)
			}
			if asJSON {
				return r.emitJSON(struct {
					Current  string               `json:"current,omitempty"`
					Profiles []managedProfileView `json:"profiles"`
				}{Current: current, Profiles: views})
			}
			if len(views) == 0 {
				fmt.Fprintln(r.stdout, "(no profiles)")
				fmt.Fprintln(r.stdout, "")
				fmt.Fprintln(r.stdout, "Get started:")
				fmt.Fprintln(r.stdout, "  agx add <name> --base-url <url> --api-key <key>")
				fmt.Fprintln(r.stdout, "  agx use <name>")
				fmt.Fprintln(r.stdout, "  agx run codex")
				return nil
			}
			tw := tabwriter.NewWriter(r.stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tCURRENT\tBASE URL\tMODEL\tAGENTS")
			for _, row := range views {
				currentMarker := ""
				if row.Current {
					currentMarker = "*"
				}
				nameLabel := row.Name
				if agent, _, derived := usecase.ParseDerivedProfileName(row.Name); derived {
					nameLabel = fmt.Sprintf("%s (derived:%s)", row.Name, agent)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", nameLabel, currentMarker, row.BaseURL, row.Model, strings.Join(row.Agents, ","))
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().BoolVar(&includeAll, "all", false, "include internal derived profiles")
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newProfileShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "show <profile>",
		Short:         "Show one managed profile",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			profile, err := r.profiles.ManagedProfile(args[0])
			if err != nil {
				return r.reportError(err)
			}
			current, _ := r.profiles.CurrentManagedProfile()
			view := toManagedProfileView(*profile, current != nil && current.Name == profile.Name)
			if asJSON {
				return r.emitJSON(struct {
					Profile managedProfileView `json:"profile"`
				}{Profile: view})
			}
			fmt.Fprintf(r.stdout, "name: %s\n", view.Name)
			fmt.Fprintf(r.stdout, "kind: %s\n", view.Kind)
			if view.Current {
				fmt.Fprintln(r.stdout, "current: yes")
			} else {
				fmt.Fprintln(r.stdout, "current: no")
			}
			if view.BaseURL != "" {
				fmt.Fprintf(r.stdout, "base_url: %s\n", view.BaseURL)
			}
			if view.APIKey != "" {
				fmt.Fprintf(r.stdout, "api_key: %s\n", view.APIKey)
			}
			if view.CredentialRef != "" {
				fmt.Fprintf(r.stdout, "credential_ref: %s\n", view.CredentialRef)
			}
			if view.Model != "" {
				fmt.Fprintf(r.stdout, "model: %s\n", view.Model)
			}
			if view.CodexWireAPI != "" {
				fmt.Fprintf(r.stdout, "codex_wire_api: %s\n", view.CodexWireAPI)
			}
			if view.ProviderFamily != "" {
				fmt.Fprintf(r.stdout, "provider_family: %s\n", view.ProviderFamily)
			}
			return nil
		},
	}
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newProfileUseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "use <profile>",
		Short:         "Select the current managed profile",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			profile, err := r.profiles.UseManagedProfile(args[0])
			if err != nil {
				return r.reportError(err)
			}
			view := toManagedProfileView(*profile, true)
			if asJSON {
				return r.emitJSON(struct {
					Profile managedProfileView `json:"profile"`
				}{Profile: view})
			}
			fmt.Fprintf(r.stdout, "current profile: %s\n", profile.Name)
			return nil
		},
	}
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newProfileCurrentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "current",
		Short:         "Show the current managed profile",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 0)
			if err != nil {
				return err
			}
			profile, err := r.profiles.CurrentManagedProfile()
			if err != nil {
				return r.reportError(err)
			}
			if asJSON {
				if profile == nil {
					return r.emitJSON(struct {
						Profile *managedProfileView `json:"profile"`
					}{})
				}
				view := toManagedProfileView(*profile, true)
				return r.emitJSON(struct {
					Profile *managedProfileView `json:"profile"`
				}{Profile: &view})
			}
			if profile == nil {
				fmt.Fprintln(r.stdout, "(no current profile)")
				return nil
			}
			fmt.Fprintf(r.stdout, "%s\n", profile.Name)
			return nil
		},
	}
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newDetachCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "detach <agent> <profile>",
		Short:         "Unbind a profile from one agent without deleting the profile",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 2)
			if err != nil {
				return err
			}
			agent, ok := domainprofile.ParseAgent(args[0])
			if !ok {
				return r.reportError(fmt.Errorf("agent must be one of: %s", agentUsageHuman))
			}
			result, err := r.profiles.RemoveTarget(agent, args[1])
			if err != nil {
				return r.reportError(err)
			}
			if asJSON {
				return r.emitJSON(struct {
					Agent   domainprofile.Agent `json:"agent"`
					Profile string              `json:"profile"`
				}{Agent: result.Agent, Profile: result.Name})
			}
			fmt.Fprintf(r.stdout, "detached %s from profile %s\n", result.Agent, result.Name)
			return nil
		},
	}
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newLauncherCommand(agent domainprofile.Agent) *cobra.Command {
	cmd := &cobra.Command{
		Use:                fmt.Sprintf("%s [profile] [-- <native args...>]", agent),
		Short:              fmt.Sprintf("Launch %s with a managed profile", agent),
		Hidden:             true,
		DisableFlagParsing: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				return cmd.Help()
			}
			profileName, nativeArgs, err := parseLauncherInvocation(args)
			if err != nil {
				r.printUserError(err)
				return exitCodeError{code: 1}
			}
			if err := r.runManagedLaunch(agent, profileName, nativeArgs); err != nil {
				return r.nativeError(err)
			}
			return nil
		},
	}
	return cmd
}

func (r *Root) newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "run <agent> [profile] [-- <native args...>]",
		Short:              "Launch an agent with a managed profile",
		DisableFlagParsing: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
				return cmd.Help()
			}
			agent, ok := domainprofile.ParseAgent(args[0])
			if !ok {
				r.printUserError(fmt.Errorf("agent must be one of: %s", agentUsageHuman))
				return exitCodeError{code: 1}
			}
			profileName, nativeArgs, err := parseLauncherInvocation(args[1:])
			if err != nil {
				r.printUserError(err)
				return exitCodeError{code: 1}
			}
			if err := r.runManagedLaunch(agent, profileName, nativeArgs); err != nil {
				return r.nativeError(err)
			}
			return nil
		},
	}
	return cmd
}


func toManagedProfileView(profile domainprofile.Profile, current bool) managedProfileView {
	profile = domainprofile.NormalizeProfile(profile)
	view := managedProfileView{
		Name:           profile.Name,
		Kind:           string(profile.Kind),
		Current:        current,
		BaseURL:        profile.BaseURL,
		Model:          profile.ModelID,
		CodexWireAPI:   string(profile.CodexWireAPI),
		ProviderFamily: string(profile.ProviderFamily),
	}
	switch {
	case profile.Kind == domainprofile.ProfileKindRelay && profile.APIKey != "":
		view.APIKey = profile.APIKey
		view.CredentialRef = "api_key"
	}
	return view
}

func parseLauncherInvocation(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, nil
	}
	if args[0] == "--" {
		return "", append([]string(nil), args[1:]...), nil
	}
	if strings.HasPrefix(args[0], "-") {
		return "", append([]string(nil), args...), nil
	}
	if len(args) == 1 {
		return args[0], nil, nil
	}
	if args[1] != "--" {
		return "", nil, fmt.Errorf("native args must follow `--` when a profile name is provided")
	}
	return args[0], append([]string(nil), args[2:]...), nil
}

func (r *Root) runManagedLaunch(agent domainprofile.Agent, profileName string, args []string) error {
	if strings.TrimSpace(profileName) == "" {
		profileName = envProfileOverride()
	}
	if strings.TrimSpace(profileName) == "" {
		current, err := r.profiles.CurrentManagedProfile()
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("no current profile selected; run `agx use <profile>`, set AGX_PROFILE, or pass one explicitly")
		}
		profileName = current.Name
	}
	if _, err := r.profiles.ActivateManagedProfile(agent, profileName); err != nil {
		r.printUserError(err)
		return exitCodeError{code: 1}
	}
	r.maybeAutoBackup(agent)
	r.autoInjectMcpEndpoint(agent)
	return r.runAgentNative(agent, args)
}

// autoInjectMcpEndpoint best-effort updates the agent's managed context
// with the configured MCP gateway endpoint before launch. Failures are
// reported to stderr but never block launch — running an agent without
// the gateway is degraded but valid, and we don't want a broken
// servers.yaml to brick `agx run`.
func (r *Root) autoInjectMcpEndpoint(agent domainprofile.Agent) {
	cfgPath, err := mcpgateway.DefaultConfigPath()
	if err != nil {
		return
	}
	cfg, err := mcpgateway.LoadConfig(cfgPath)
	if err != nil || cfg == nil {
		return
	}
	ep := mcpEndpointFromConfig(cfg)
	if strings.TrimSpace(ep.URL) == "" {
		return
	}
	target, contextPath, err := r.profiles.CurrentTargetContext(agent)
	if err != nil || target.Kind != domainprofile.TargetKindRelay || strings.TrimSpace(contextPath) == "" {
		return
	}
	path := mcpinject.ConfigPath(agent, contextPath)
	if path == "" {
		return
	}
	if err := mcpinject.Inject(agent, path, ep); err != nil {
		fmt.Fprintf(r.stderr, "Warning: mcp endpoint inject failed for %s: %v\n", agent, err)
	}
}

// envProfileOverride lets the user pin a relay profile for a single
// directory (or session) without flipping the global default. Pairs well
// with direnv / .envrc:
//
//	echo 'export AGX_PROFILE=staging' > .envrc
//	direnv allow
//
// Direnv's allow-list is the trust boundary, so agx itself never has to
// prompt for an unknown override source. Whitespace-only values are
// treated as unset.
func envProfileOverride() string {
	return strings.TrimSpace(os.Getenv("AGX_PROFILE"))
}

// maybeAutoBackup snapshots the agent's current managed target before launch
// when the user opts in via AGX_AUTO_BACKUP. This is intentionally opt-in: CLI
// users do not appreciate hidden filesystem activity on every launch. When
// enabled, snapshot failures degrade to a stderr warning and do not block
// the launch — the user can still recover via the previous snapshot.
// opencode has no per-target context, so the hook is a no-op for it.
func (r *Root) maybeAutoBackup(agent domainprofile.Agent) {
	if !autoBackupEnabled() {
		return
	}
	if agent == domainprofile.AgentOpenCode {
		return
	}
	if _, err := r.profiles.BackupManagedTarget(agent, "", ""); err != nil {
		fmt.Fprintf(r.stderr, "warning: AGX_AUTO_BACKUP enabled but snapshot failed: %v\n", err)
	}
}

func autoBackupEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("AGX_AUTO_BACKUP"))
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
