package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kiddingbaby/agx/internal/usecase"
	"gopkg.in/yaml.v3"
)

type syncBundle struct {
	Assets *syncAssets `yaml:"assets,omitempty"`
}

type syncAssets struct {
	SkillsHubHome     string   `yaml:"skills-hub-home,omitempty"`
	SystemPromptPath  string   `yaml:"system-prompt-path,omitempty"`
	SystemPromptLinks []string `yaml:"system-prompt-links,omitempty"`

	Skills *syncAssetsSkills `yaml:"skills,omitempty"`
	MCP    *syncAssetsMCP    `yaml:"mcp,omitempty"`
}

type syncAssetsSkills struct {
	Enabled *bool    `yaml:"enabled,omitempty"`
	Source  string   `yaml:"source,omitempty"`
	Targets []string `yaml:"targets,omitempty"`
	Prune   *bool    `yaml:"prune,omitempty"`
}

type syncAssetsMCP struct {
	Enabled *bool           `yaml:"enabled,omitempty"`
	Targets []string        `yaml:"targets,omitempty"`
	Prune   *bool           `yaml:"prune,omitempty"`
	Servers []syncMCPServer `yaml:"servers,omitempty"`
}

type syncMCPServer struct {
	Name              string            `yaml:"name"`
	Command           []string          `yaml:"command,omitempty"`
	Env               map[string]string `yaml:"env,omitempty"`
	URL               string            `yaml:"url,omitempty"`
	BearerTokenEnvVar string            `yaml:"bearer-token-env-var,omitempty"`
}

type resultEnvelope struct {
	SchemaVersion string   `json:"schema_version"`
	Tool          string   `json:"tool"`
	Status        string   `json:"status"`
	Data          any      `json:"data"`
	Failures      []any    `json:"failures,omitempty"`
	Hints         []string `json:"hints,omitempty"`
	Error         any      `json:"error,omitempty"`
}

func (r *Root) handleSync(args []string) int {
	if r.envSyncSvc == nil {
		fmt.Fprintln(r.stderr, "Error: sync service is unavailable")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx sync [skills|system-prompt|mcp] [PATH|DIR] [-o json] [--dry-run] | agx sync [skills|system-prompt|mcp] --stdin [-o json] [--dry-run] | agx sync [skills|system-prompt|mcp] --paste [-o json] [--dry-run]")
		return 0
	}

	component := ""
	if len(args) > 0 {
		switch args[0] {
		case "skills", "system-prompt", "mcp":
			component = args[0]
			args = args[1:]
		}
	}

	var (
		filePath  string
		fromStdin bool
		paste     bool
		asJSON    bool
		dryRun    bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--stdin":
			fromStdin = true
		case "--paste":
			paste = true
		case "-o":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: -o requires a value (json)")
				return 1
			}
			if args[i+1] != "json" {
				fmt.Fprintf(r.stderr, "Error: unsupported output format: %s\n", args[i+1])
				return 1
			}
			asJSON = true
			i++
		case "--dry-run":
			dryRun = true
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintln(r.stderr, "Usage: agx sync [skills|system-prompt|mcp] [PATH|DIR] [-o json] [--dry-run] | agx sync [skills|system-prompt|mcp] --stdin [-o json] [--dry-run] | agx sync [skills|system-prompt|mcp] --paste [-o json] [--dry-run]")
				return 1
			}
			if strings.TrimSpace(filePath) != "" || fromStdin || paste {
				fmt.Fprintln(r.stderr, "Usage: agx sync [skills|system-prompt|mcp] [PATH|DIR] [-o json] [--dry-run] | agx sync [skills|system-prompt|mcp] --stdin [-o json] [--dry-run] | agx sync [skills|system-prompt|mcp] --paste [-o json] [--dry-run]")
				return 1
			}
			filePath = args[i]
		}
	}

	sources := 0
	if strings.TrimSpace(filePath) != "" {
		sources++
	}
	if fromStdin {
		sources++
	}
	if paste {
		sources++
	}
	if sources > 1 {
		fmt.Fprintln(r.stderr, "Error: choose only one input source: PATH|DIR | --stdin | --paste")
		return 1
	}

	if sources == 0 {
		if path, ok := findDefaultConfigPath(r.getwd); ok {
			filePath = path
		} else if stderrIsCharDevice() {
			paste = true
		} else {
			fmt.Fprintln(r.stderr, "Usage: agx sync [skills|system-prompt|mcp] [PATH|DIR] [-o json] [--dry-run] | agx sync [skills|system-prompt|mcp] --stdin [-o json] [--dry-run] | agx sync [skills|system-prompt|mcp] --paste [-o json] [--dry-run]")
			return 1
		}
	}

	var (
		data []byte
		err  error
	)
	sourceLabel := "stdin"
	if fromStdin || paste {
		if fromStdin && stdinIsCharDevice() {
			fmt.Fprintln(r.stderr, "Error: --stdin requires piped stdin")
			return 1
		}
		if paste {
			fmt.Fprintln(r.stderr, "Paste config YAML (with `assets:`), then Ctrl-D:")
		}
		data, err = io.ReadAll(os.Stdin)
	} else {
		resolved, resolveErr := resolveConfigPath(filePath)
		if resolveErr != nil {
			return r.writeSyncEnvelope(asJSON, "error", filePath, dryRun, nil, nil, fmt.Errorf("resolve config path: %w", resolveErr))
		}
		filePath = resolved
		sourceLabel = filePath
		data, err = os.ReadFile(filePath)
	}
	if err != nil {
		return r.writeSyncEnvelope(asJSON, "error", sourceLabel, dryRun, nil, nil, err)
	}

	var bundle syncBundle
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		return r.writeSyncEnvelope(asJSON, "error", sourceLabel, dryRun, nil, nil, fmt.Errorf("parse config: %w", err))
	}

	if bundle.Assets == nil {
		res := usecase.EnvSyncResult{
			Hints: []string{
				"No `assets:` section found in config. Add `assets:` to ~/.config/agx/agx.yml or run `agx init` then fill it.",
			},
		}
		return r.writeSyncEnvelope(asJSON, "degraded", sourceLabel, dryRun, &res, nil, nil)
	}

	cfg := toEnvAssetsConfig(*bundle.Assets)
	if component != "" {
		switch component {
		case "skills":
			cfg.SystemPromptLinks = []string{}
			cfg.MCP.Enabled = false
		case "system-prompt":
			cfg.Skills.Enabled = false
			cfg.MCP.Enabled = false
		case "mcp":
			cfg.SystemPromptLinks = []string{}
			cfg.Skills.Enabled = false
		}
	}
	res := r.envSyncSvc.Sync(usecase.EnvSyncOptions{DryRun: dryRun}, cfg)

	status := "ok"
	if len(res.Failures) > 0 {
		status = "degraded"
	}
	return r.writeSyncEnvelope(asJSON, status, sourceLabel, dryRun, &res, nil, nil)
}

func toEnvAssetsConfig(in syncAssets) usecase.EnvAssetsConfig {
	cfg := usecase.EnvAssetsConfig{
		SkillsHubHome:     strings.TrimSpace(in.SkillsHubHome),
		SystemPromptPath:  strings.TrimSpace(in.SystemPromptPath),
		SystemPromptLinks: in.SystemPromptLinks,
	}

	if in.Skills != nil {
		cfg.Skills.Enabled = boolOrDefault(in.Skills.Enabled, true)
		cfg.Skills.Source = strings.TrimSpace(in.Skills.Source)
		cfg.Skills.Targets = in.Skills.Targets
		cfg.Skills.Prune = boolOrDefault(in.Skills.Prune, true)
	}

	if in.MCP != nil {
		cfg.MCP.Enabled = boolOrDefault(in.MCP.Enabled, true)
		cfg.MCP.Targets = in.MCP.Targets
		cfg.MCP.Prune = boolOrDefault(in.MCP.Prune, true)
		for _, s := range in.MCP.Servers {
			cfg.MCP.Servers = append(cfg.MCP.Servers, usecase.MCPServerConfig{
				Name:              strings.TrimSpace(s.Name),
				Command:           s.Command,
				Env:               s.Env,
				URL:               strings.TrimSpace(s.URL),
				BearerTokenEnvVar: strings.TrimSpace(s.BearerTokenEnvVar),
			})
		}
	}

	return cfg
}

func boolOrDefault(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func (r *Root) writeSyncEnvelope(asJSON bool, status string, sourceLabel string, dryRun bool, res *usecase.EnvSyncResult, failures []any, err error) int {
	startedAt := time.Now().UTC()
	payload := map[string]any{
		"source":     sourceLabel,
		"dry_run":    dryRun,
		"started_at": startedAt.Format(time.RFC3339Nano),
	}
	if res != nil {
		payload["result"] = res
		if len(res.Failures) > 0 {
			f := make([]any, 0, len(res.Failures))
			for _, x := range res.Failures {
				f = append(f, x)
			}
			failures = append(failures, f...)
		}
		if len(res.Hints) > 0 {
			payload["hints"] = res.Hints
		}
	}

	env := resultEnvelope{
		SchemaVersion: "agx/sync/v1",
		Tool:          "agx",
		Status:        status,
		Data:          payload,
		Failures:      failures,
	}
	if res != nil && len(res.Hints) > 0 {
		env.Hints = res.Hints
	}
	if err != nil {
		env.Status = "error"
		env.Error = err.Error()
		env.Hints = append(env.Hints, "Fix the error and re-run `agx sync -o json`.")
	}

	if asJSON {
		enc := json.NewEncoder(r.stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(env)
		if env.Status == "ok" {
			return 0
		}
		return 1
	}

	if env.Status == "error" {
		fmt.Fprintf(r.stderr, "Error: %v\n", env.Error)
		return 1
	}

	fmt.Fprintf(r.stdout, "AGX sync: status=%s source=%s dry_run=%t\n", env.Status, sourceLabel, dryRun)
	if res != nil {
		for target, sp := range res.SystemPrompts {
			fmt.Fprintf(r.stdout, "  system_prompt[%s]: %s -> %s (%s)\n", target, sp.LinkPath, sp.TargetPath, sp.Action)
		}
		for target, sk := range res.Skills {
			fmt.Fprintf(r.stdout, "  skills[%s]: %s -> %s (%s copied=%d updated=%d removed=%d)\n",
				target, sk.Source, sk.Destination, sk.Action, sk.CopiedFiles, sk.UpdatedFiles, sk.RemovedPaths)
		}
		for target, m := range res.MCP {
			fmt.Fprintf(r.stdout, "  mcp[%s]: %s\n", target, m.Action)
		}
		if len(res.Failures) > 0 {
			fmt.Fprintln(r.stdout, "Failures:")
			for _, f := range res.Failures {
				target := f.Target
				if strings.TrimSpace(target) == "" {
					target = "-"
				}
				fmt.Fprintf(r.stdout, "  - [%s] target=%s: %s (%s)\n", f.Component, target, f.Message, f.Detail)
			}
		}
		if len(res.Hints) > 0 {
			fmt.Fprintln(r.stdout, "Hints:")
			for _, h := range res.Hints {
				fmt.Fprintf(r.stdout, "  - %s\n", h)
			}
		}
	}
	if env.Status == "ok" {
		return 0
	}
	return 1
}
