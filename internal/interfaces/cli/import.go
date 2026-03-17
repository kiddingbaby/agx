package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kiddingbaby/agx/internal/config"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func (r *Root) handleImport(args []string) int {
	if r.keySvc == nil || r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: import requires key/provider services")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx import claude [--site <site>] [--tags TAGS] [--no-activate] [--dry-run] [-o json]")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Usage: agx import claude [--site <site>] [--tags TAGS] [--no-activate] [--dry-run] [-o json]")
		return 1
	}

	switch args[0] {
	case "claude":
		return r.handleImportClaude(args[1:])
	default:
		fmt.Fprintf(r.stderr, "Error: unknown import target: %s\n", args[0])
		fmt.Fprintln(r.stderr, "Tip: supported: claude")
		return 1
	}
}

func (r *Root) handleImportClaude(args []string) int {
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx import claude [--site <site>] [--tags TAGS] [--no-activate] [--dry-run] [-o json]")
		return 0
	}

	var (
		siteArg     string
		tagsRaw     string
		activate    = true
		dryRun      bool
		asJSON      bool
		reader      = bufio.NewReaderSize(os.Stdin, 64*1024)
		interactive = stdinIsCharDevice() && stderrIsCharDevice()
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--site":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --site requires a value")
				return 1
			}
			siteArg = args[i+1]
			i++
		case "--tags":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --tags requires a value")
				return 1
			}
			tagsRaw = args[i+1]
			i++
		case "--no-activate":
			activate = false
		case "--dry-run":
			dryRun = true
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
		default:
			fmt.Fprintln(r.stderr, "Usage: agx import claude [--site <site>] [--tags TAGS] [--no-activate] [--dry-run] [-o json]")
			return 1
		}
	}

	var tags []string
	if strings.TrimSpace(tagsRaw) != "" {
		tags = normalizeTags(strings.Split(tagsRaw, ","))
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	data, err := loadJSONFile(paths.ClaudeSettingsPath)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: read %s: %v\n", paths.ClaudeSettingsPath, err)
		return 1
	}

	env := map[string]any{}
	if raw, ok := data["env"]; ok {
		if obj, ok := raw.(map[string]any); ok {
			env = obj
		}
	}

	anthropicKey := strings.TrimSpace(stringValue(env["ANTHROPIC_API_KEY"]))
	claudeKey := strings.TrimSpace(stringValue(env["CLAUDE_API_KEY"]))
	apiKey := anthropicKey
	if apiKey == "" {
		apiKey = claudeKey
	}
	if anthropicKey != "" && claudeKey != "" && anthropicKey != claudeKey {
		fmt.Fprintln(r.stderr, "Error: both ANTHROPIC_API_KEY and CLAUDE_API_KEY are set but differ; please fix ~/.claude/settings.json first")
		return 1
	}
	if apiKey == "" {
		fmt.Fprintf(r.stderr, "Error: no Claude API key found in %s (env.ANTHROPIC_API_KEY / env.CLAUDE_API_KEY)\n", paths.ClaudeSettingsPath)
		return 1
	}

	baseURL := strings.TrimSpace(stringValue(env["ANTHROPIC_BASE_URL"]))
	model := strings.TrimSpace(stringValue(data["model"]))

	site := strings.TrimSpace(siteArg)
	if site == "" {
		// Default: import into official when no proxy base-url is set; otherwise require a site name.
		if baseURL == "" {
			site = "claude"
		} else if interactive {
			got, err := promptStringRequired(reader, r.stderr, "Import as site name (required): ")
			if err != nil {
				fmt.Fprintf(r.stderr, "Error: %v\n", err)
				return 1
			}
			site = strings.TrimSpace(got)
		} else {
			site = "claude-import"
		}
	}

	internalName, _ := internalTargetNameFromSiteArg(site)
	target, err := r.providerSvc.GetTarget(internalName)
	if err != nil {
		if domainprovider.IsTargetNotFoundError(err) {
			if baseURL == "" {
				fmt.Fprintf(r.stderr, "Error: site not found: %s (and native config has no base-url to create a proxy site)\n", site)
				fmt.Fprintln(r.stderr, "Tip: use `agx import claude --site claude` to import into the official site, or create a site first then retry.")
				return 1
			}
			if isReservedSiteName(site) {
				fmt.Fprintf(r.stderr, "Error: site name %q is reserved\n", site)
				fmt.Fprintln(r.stderr, "Tip: use official aliases: openai | claude | gemini, or pick another site name.")
				return 1
			}
			if dryRun {
				target = &domainprovider.Target{
					Name:    site,
					Family:  domainprovider.FamilyClaude,
					Kind:    domainprovider.KindClaude,
					Access:  domainprovider.AccessThirdParty,
					Auth:    domainprovider.AuthAPIKey,
					BaseURL: baseURL,
					Model:   model,
				}
			} else {
				created, err := r.providerSvc.SaveTarget(usecase.SaveTargetInput{
					Name:    site,
					Family:  string(domainprovider.FamilyClaude),
					Kind:    string(domainprovider.KindClaude),
					Access:  string(domainprovider.AccessThirdParty),
					Auth:    string(domainprovider.AuthAPIKey),
					BaseURL: baseURL,
					Model:   model,
				})
				if err != nil {
					fmt.Fprintf(r.stderr, "Error: %v\n", err)
					return 1
				}
				target = created
			}
		} else {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
	}

	if target == nil {
		fmt.Fprintf(r.stderr, "Error: site not found: %s\n", site)
		return 1
	}

	keyProvider, keyProfile, err := r.providerSvc.KeyScopeForTarget(*target)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	if dryRun {
		if asJSON {
			payload := struct {
				DryRun   bool     `json:"dry_run"`
				Source   string   `json:"source"`
				Site     siteView `json:"site"`
				KeyScope struct {
					Provider domainkey.Provider `json:"provider"`
					Profile  string             `json:"profile"`
				} `json:"key_scope"`
			}{
				DryRun: true,
				Source: paths.ClaudeSettingsPath,
				Site: siteView{
					Name:    displayNameForTarget(*target),
					Target:  target.Name,
					Family:  target.Family,
					Kind:    target.Kind,
					Access:  target.Access,
					BaseURL: target.BaseURL,
					Model:   target.Model,
				},
			}
			payload.KeyScope.Provider = keyProvider
			payload.KeyScope.Profile = domainkey.NormalizeProfileName(keyProfile)
			_ = json.NewEncoder(r.stdout).Encode(payload)
			return 0
		}
		fmt.Fprintf(r.stdout, "Dry-run: would import Claude key into %s/%s (site=%s)\n", keyProvider, domainkey.NormalizeProfileName(keyProfile), displayNameForTarget(*target))
		fmt.Fprintf(r.stdout, "Next: run `agx use %s`\n", displayNameForTarget(*target))
		return 0
	}

	scopeCount := 0
	for _, existing := range r.keySvc.List() {
		if existing.Provider != keyProvider {
			continue
		}
		if domainkey.NormalizeProfileName(existing.Profile) != domainkey.NormalizeProfileName(keyProfile) {
			continue
		}
		scopeCount++
	}
	desiredName := fmt.Sprintf("%s-%02d", keyProvider, scopeCount+1)
	uniqueName, err := r.uniqueKeyName(keyProvider, keyProfile, desiredName)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	k, err := r.keySvc.Add(keyProvider, keyProfile, uniqueName, apiKey, "", tags)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if activate {
		_ = r.keySvc.Activate(k.ID)
	}

	if asJSON {
		payload := struct {
			Source string            `json:"source"`
			Site   siteView          `json:"site"`
			Key    keysPasteItemView `json:"key"`
		}{
			Source: paths.ClaudeSettingsPath,
			Site: siteView{
				Name:    displayNameForTarget(*target),
				Target:  target.Name,
				Family:  target.Family,
				Kind:    target.Kind,
				Access:  target.Access,
				BaseURL: target.BaseURL,
				Model:   target.Model,
			},
			Key: keysPasteItemView{
				ID:       k.ID,
				Provider: k.Provider,
				Profile:  domainkey.NormalizeProfileName(k.Profile),
				Name:     k.Name,
				Active:   activate,
			},
		}
		_ = json.NewEncoder(r.stdout).Encode(payload)
		return 0
	}

	id := k.ID
	if len(id) >= 8 {
		id = id[:8]
	}
	fmt.Fprintf(r.stdout, "Imported Claude key: %s (%s) -> %s/%s\n", k.Name, id, keyProvider, domainkey.NormalizeProfileName(keyProfile))
	fmt.Fprintf(r.stdout, "Next: run `agx use %s`\n", displayNameForTarget(*target))
	return 0
}

func loadJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}
