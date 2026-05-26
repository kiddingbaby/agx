package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/mcpgateway"
	"github.com/kiddingbaby/agx/internal/mcpinject"
	"github.com/spf13/cobra"
)

type mcpServerView struct {
	Name              string            `json:"name"`
	Transport         string            `json:"transport"`
	Command           string            `json:"command,omitempty"`
	Args              []string          `json:"args,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	EnvPassthrough    []string          `json:"env_passthrough,omitempty"`
	URL               string            `json:"url,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	Enabled           bool              `json:"enabled"`
	StartupTimeoutSec int               `json:"startup_timeout_sec,omitempty"`
}

func toMcpServerView(spec mcpgateway.ServerSpec) mcpServerView {
	return mcpServerView{
		Name:              spec.Name,
		Transport:         spec.Transport,
		Command:           spec.Command,
		Args:              spec.Args,
		Env:               spec.Env,
		EnvPassthrough:    spec.EnvPassthrough,
		URL:               spec.URL,
		Headers:           spec.Headers,
		Enabled:           spec.IsEnabled(),
		StartupTimeoutSec: spec.StartupTimeoutSec,
	}
}

func (r *Root) newMcpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "mcp",
		Short:         "Manage the local MCP gateway",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.AddCommand(
		r.newMcpServeCommand(),
		r.newMcpListCommand(),
		r.newMcpShowCommand(),
		r.newMcpRegisterCommand(),
		r.newMcpDeregisterCommand(),
		r.newMcpEnableCommand(),
		r.newMcpDisableCommand(),
		r.newMcpTestCommand(),
		r.newMcpSyncCommand(),
		r.newMcpClearCommand(),
	)
	return cmd
}

func (r *Root) newMcpServeCommand() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Run the MCP gateway daemon",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return r.usageError(cmd)
			}
			path, err := r.resolveMcpConfigPath(cfgPath)
			if err != nil {
				return r.reportError(err)
			}
			logger := slog.New(slog.NewTextHandler(r.stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
			gw, err := mcpgateway.New(path, logger)
			if err != nil {
				return r.reportError(err)
			}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			if err := gw.Serve(ctx); err != nil {
				return r.reportError(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to servers.yaml (defaults to ~/.config/agx/mcp/servers.yaml)")
	return cmd
}

func (r *Root) newMcpListCommand() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:           "list",
		Short:         "List configured MCP servers",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 0)
			if err != nil {
				return err
			}
			cfg, err := r.loadMcpConfig(cfgPath)
			if err != nil {
				return r.reportError(err)
			}
			views := make([]mcpServerView, 0, len(cfg.Servers))
			for _, s := range cfg.Servers {
				views = append(views, toMcpServerView(s))
			}
			if asJSON {
				return r.emitJSON(struct {
					Servers []mcpServerView `json:"servers"`
				}{Servers: views})
			}
			r.printMcpServerTable(views)
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to servers.yaml")
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) printMcpServerTable(views []mcpServerView) {
	if len(views) == 0 {
		fmt.Fprintln(r.stdout, "No MCP servers configured. Use `agx mcp register` to add one.")
		return
	}
	w := tabwriter.NewWriter(r.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTRANSPORT\tENABLED\tENDPOINT")
	for _, v := range views {
		endpoint := v.URL
		if endpoint == "" {
			endpoint = v.Command
			if len(v.Args) > 0 {
				endpoint += " " + strings.Join(v.Args, " ")
			}
		}
		enabled := "yes"
		if !v.Enabled {
			enabled = "no"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", v.Name, v.Transport, enabled, endpoint)
	}
	_ = w.Flush()
}

func (r *Root) newMcpShowCommand() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:           "show <name>",
		Short:         "Show one MCP server's configuration",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			cfg, err := r.loadMcpConfig(cfgPath)
			if err != nil {
				return r.reportError(err)
			}
			spec, ok := cfg.FindServer(args[0])
			if !ok {
				return r.reportError(fmt.Errorf("mcp server %q not found", args[0]))
			}
			view := toMcpServerView(spec)
			if asJSON {
				return r.emitJSON(struct {
					Server mcpServerView `json:"server"`
				}{Server: view})
			}
			r.printMcpServerDetail(view)
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to servers.yaml")
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) printMcpServerDetail(v mcpServerView) {
	fmt.Fprintf(r.stdout, "name:       %s\n", v.Name)
	fmt.Fprintf(r.stdout, "transport:  %s\n", v.Transport)
	fmt.Fprintf(r.stdout, "enabled:    %t\n", v.Enabled)
	if v.URL != "" {
		fmt.Fprintf(r.stdout, "url:        %s\n", v.URL)
	}
	if v.Command != "" {
		fmt.Fprintf(r.stdout, "command:    %s\n", v.Command)
	}
	if len(v.Args) > 0 {
		fmt.Fprintf(r.stdout, "args:       %s\n", strings.Join(v.Args, " "))
	}
	if len(v.Env) > 0 {
		fmt.Fprintln(r.stdout, "env:")
		for k, val := range v.Env {
			fmt.Fprintf(r.stdout, "  %s=%s\n", k, val)
		}
	}
	if len(v.EnvPassthrough) > 0 {
		fmt.Fprintf(r.stdout, "env_passthrough: %s\n", strings.Join(v.EnvPassthrough, ","))
	}
	if len(v.Headers) > 0 {
		fmt.Fprintln(r.stdout, "headers:")
		for k, val := range v.Headers {
			fmt.Fprintf(r.stdout, "  %s: %s\n", k, val)
		}
	}
	if v.StartupTimeoutSec > 0 {
		fmt.Fprintf(r.stdout, "startup_timeout_sec: %d\n", v.StartupTimeoutSec)
	}
}

func (r *Root) newMcpRegisterCommand() *cobra.Command {
	var (
		cfgPath        string
		transport      string
		url            string
		headers        []string
		envKV          []string
		envPassthrough []string
		startupTimeout int
		disabled       bool
	)
	cmd := &cobra.Command{
		Use:                "register <name> [--url URL | -- <command> [args...]]",
		Short:              "Add or update an MCP server entry",
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return r.usageError(cmd)
			}
			name := args[0]
			cmdArgs := args[1:]

			path, err := r.resolveMcpConfigPath(cfgPath)
			if err != nil {
				return r.reportError(err)
			}
			cfg, err := mcpgateway.LoadConfig(path)
			if err != nil {
				return r.reportError(err)
			}

			spec := mcpgateway.ServerSpec{Name: name}
			if disabled {
				v := false
				spec.Enabled = &v
			}
			if startupTimeout > 0 {
				spec.StartupTimeoutSec = startupTimeout
			}
			spec.Headers, err = parseKVList(headers)
			if err != nil {
				return r.reportError(fmt.Errorf("--header: %w", err))
			}
			spec.Env, err = parseKVList(envKV)
			if err != nil {
				return r.reportError(fmt.Errorf("--env: %w", err))
			}
			spec.EnvPassthrough = envPassthrough

			switch {
			case url != "":
				spec.Transport = mcpgateway.TransportHTTP
				spec.URL = url
				if len(cmdArgs) > 0 {
					return r.reportError(fmt.Errorf("--url given; positional command not allowed"))
				}
			case len(cmdArgs) > 0:
				spec.Transport = mcpgateway.TransportStdio
				spec.Command = cmdArgs[0]
				spec.Args = append([]string(nil), cmdArgs[1:]...)
			default:
				return r.reportError(fmt.Errorf("provide either --url or `-- <command> [args...]`"))
			}
			if transport != "" {
				spec.Transport = transport
			}

			cfg.UpsertServer(spec)
			if err := mcpgateway.SaveConfig(path, cfg); err != nil {
				return r.reportError(err)
			}
			fmt.Fprintf(r.stdout, "mcp server %s registered (%s)\n", spec.Name, spec.Transport)
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to servers.yaml")
	cmd.Flags().StringVar(&transport, "transport", "", "force transport (stdio|http); inferred otherwise")
	cmd.Flags().StringVar(&url, "url", "", "downstream MCP HTTP endpoint")
	cmd.Flags().StringSliceVar(&headers, "header", nil, "HTTP header: NAME=VALUE (repeatable)")
	cmd.Flags().StringSliceVar(&envKV, "env", nil, "stdio env var: NAME=VALUE (repeatable)")
	cmd.Flags().StringSliceVar(&envPassthrough, "env-passthrough", nil, "stdio host env to forward (repeatable)")
	cmd.Flags().IntVar(&startupTimeout, "startup-timeout", 0, "connect timeout in seconds (default 30)")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "register but mark disabled")
	return cmd
}

func parseKVList(items []string) (map[string]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(items))
	for _, raw := range items {
		k, v, ok := strings.Cut(raw, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("expected NAME=VALUE, got %q", raw)
		}
		out[k] = v
	}
	return out, nil
}

func (r *Root) newMcpDeregisterCommand() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:           "deregister <name>",
		Short:         "Remove an MCP server entry",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return r.usageError(cmd)
			}
			path, err := r.resolveMcpConfigPath(cfgPath)
			if err != nil {
				return r.reportError(err)
			}
			cfg, err := mcpgateway.LoadConfig(path)
			if err != nil {
				return r.reportError(err)
			}
			if !cfg.RemoveServer(args[0]) {
				return r.reportError(fmt.Errorf("mcp server %q not found", args[0]))
			}
			if err := mcpgateway.SaveConfig(path, cfg); err != nil {
				return r.reportError(err)
			}
			fmt.Fprintf(r.stdout, "mcp server %s removed\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to servers.yaml")
	return cmd
}

func (r *Root) newMcpEnableCommand() *cobra.Command {
	return r.newMcpToggleCommand("enable", true)
}

func (r *Root) newMcpDisableCommand() *cobra.Command {
	return r.newMcpToggleCommand("disable", false)
}

func (r *Root) newMcpToggleCommand(verb string, enable bool) *cobra.Command {
	var cfgPath string
	short := "Enable an MCP server"
	if !enable {
		short = "Disable an MCP server without removing it"
	}
	cmd := &cobra.Command{
		Use:           verb + " <name>",
		Short:         short,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return r.usageError(cmd)
			}
			path, err := r.resolveMcpConfigPath(cfgPath)
			if err != nil {
				return r.reportError(err)
			}
			cfg, err := mcpgateway.LoadConfig(path)
			if err != nil {
				return r.reportError(err)
			}
			if !cfg.SetEnabled(args[0], enable) {
				return r.reportError(fmt.Errorf("mcp server %q not found", args[0]))
			}
			if err := mcpgateway.SaveConfig(path, cfg); err != nil {
				return r.reportError(err)
			}
			fmt.Fprintf(r.stdout, "mcp server %s %sd\n", args[0], verb)
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to servers.yaml")
	return cmd
}

func (r *Root) newMcpTestCommand() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:           "test <name>",
		Short:         "Connect to one MCP server once and list its tools",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			cfg, err := r.loadMcpConfig(cfgPath)
			if err != nil {
				return r.reportError(err)
			}
			spec, ok := cfg.FindServer(args[0])
			if !ok {
				return r.reportError(fmt.Errorf("mcp server %q not found", args[0]))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			result, err := mcpgateway.Probe(ctx, spec)
			if err != nil {
				return r.reportError(fmt.Errorf("%s: %w", spec.Name, err))
			}
			if asJSON {
				return r.emitJSON(result)
			}
			fmt.Fprintf(r.stdout, "%s: %d tool(s), %d prompt(s) (%.0fms)\n",
				spec.Name, len(result.Tools), len(result.Prompts), result.Latency.Seconds()*1000)
			for _, t := range result.Tools {
				fmt.Fprintf(r.stdout, "  tool   %s\n", t)
			}
			for _, p := range result.Prompts {
				fmt.Fprintf(r.stdout, "  prompt %s\n", p)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to servers.yaml")
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) resolveMcpConfigPath(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return override, nil
	}
	return mcpgateway.DefaultConfigPath()
}

func (r *Root) loadMcpConfig(override string) (*mcpgateway.Config, error) {
	path, err := r.resolveMcpConfigPath(override)
	if err != nil {
		return nil, err
	}
	return mcpgateway.LoadConfig(path)
}

func (r *Root) newMcpSyncCommand() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:           "sync",
		Short:         "Inject the gateway endpoint into every managed agent context",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return r.usageError(cmd)
			}
			cfg, err := r.loadMcpConfig(cfgPath)
			if err != nil {
				return r.reportError(err)
			}
			ep := mcpEndpointFromConfig(cfg)
			return r.applyMcpToAllTargets(true, ep)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "", "path to servers.yaml")
	return cmd
}

func (r *Root) newMcpClearCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "clear",
		Short:         "Remove the gateway endpoint from every managed agent context",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return r.usageError(cmd)
			}
			return r.applyMcpToAllTargets(false, mcpinject.Endpoint{})
		},
	}
	return cmd
}

func mcpEndpointFromConfig(cfg *mcpgateway.Config) mcpinject.Endpoint {
	listen := normalizeListenForClient(cfg.Gateway.Listen)
	return mcpinject.Endpoint{
		URL:       "http://" + listen + "/mcp",
		BearerEnv: strings.TrimSpace(cfg.Gateway.Auth.BearerTokenEnv),
	}
}

// normalizeListenForClient turns a server-side listen spec into something
// an agent on the same host can dial. "0.0.0.0:8765" and ":8765" both mean
// "listen on all interfaces" — agents would refuse those as hostnames, so
// rewrite to loopback. Hostnames and explicit IPs pass through.
func normalizeListenForClient(listen string) string {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return mcpgateway.DefaultListen
	}
	if strings.HasPrefix(listen, ":") {
		return "127.0.0.1" + listen
	}
	if strings.HasPrefix(listen, "0.0.0.0:") {
		return "127.0.0.1:" + strings.TrimPrefix(listen, "0.0.0.0:")
	}
	if strings.HasPrefix(listen, "[::]:") {
		return "127.0.0.1:" + strings.TrimPrefix(listen, "[::]:")
	}
	return listen
}

func (r *Root) applyMcpToAllTargets(inject bool, ep mcpinject.Endpoint) error {
	if r.profiles == nil {
		return r.reportError(fmt.Errorf("profile service unavailable"))
	}
	if inject && strings.TrimSpace(ep.URL) == "" {
		return r.reportError(fmt.Errorf("gateway URL is empty"))
	}
	agents := domainprofile.SupportedAgents()
	var touched int
	var firstErr error
	for _, agent := range agents {
		state, err := r.profiles.ManagedAgent(agent)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, target := range state.Targets {
			if target.Kind != domainprofile.TargetKindRelay {
				continue
			}
			if strings.TrimSpace(target.ContextPath) == "" {
				continue
			}
			path := mcpinject.ConfigPath(agent, target.ContextPath)
			if path == "" {
				continue
			}
			var opErr error
			if inject {
				opErr = mcpinject.Inject(agent, path, ep)
			} else {
				opErr = mcpinject.Clear(agent, path)
			}
			if opErr != nil {
				fmt.Fprintf(r.stderr, "  %s %s: %v\n", agent, targetName(target), opErr)
				if firstErr == nil {
					firstErr = opErr
				}
				continue
			}
			verb := "injected"
			if !inject {
				verb = "cleared"
			}
			fmt.Fprintf(r.stdout, "  %s %-12s %s -> %s\n", agent, targetName(target), verb, path)
			touched++
		}
	}
	if touched == 0 && firstErr == nil {
		fmt.Fprintln(r.stdout, "No managed targets found. Run `agx use <profile>` first.")
		return nil
	}
	if firstErr != nil {
		return r.reportError(firstErr)
	}
	return nil
}

func targetName(t domainprofile.TargetState) string {
	if t.Relay.ProfileName != "" {
		return t.Relay.ProfileName
	}
	return string(t.Kind)
}

