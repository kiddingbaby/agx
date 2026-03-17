package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/kiddingbaby/agx/internal/config"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func (r *Root) handleUse(args []string) int {
	if r.switchSvc == nil || r.providerSvc == nil || r.keySvc == nil {
		fmt.Fprintln(r.stderr, "Error: use requires key/config/switch services")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx use <site> [--agents codex,claude,gemini|all] [--key KEY | -l TAGS] [--dry-run] [-o json]")
		return 0
	}

	var (
		name     string
		keyIdent string
		selector string
		selSet   bool
		agents   []domainprovider.Family
		asJSON   bool
		dryRun   bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agents":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --agents requires a value")
				return 1
			}
			parsed, err := parseAgentsFamilies(args[i+1])
			if err != nil {
				fmt.Fprintf(r.stderr, "Error: %v\n", err)
				return 1
			}
			agents = parsed
			i++
		case "--key":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --key requires a value")
				return 1
			}
			keyIdent = args[i+1]
			i++
		case "-l":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: -l requires a value")
				return 1
			}
			selector = args[i+1]
			selSet = true
			i++
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
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintln(r.stderr, "Usage: agx use <site> [--agents codex,claude,gemini|all] [--key KEY | -l TAGS] [--dry-run] [-o json]")
				return 1
			}
			if strings.TrimSpace(name) != "" {
				fmt.Fprintln(r.stderr, "Usage: agx use <site> [--agents codex,claude,gemini|all] [--key KEY | -l TAGS] [--dry-run] [-o json]")
				return 1
			}
			name = args[i]
		}
	}

	if strings.TrimSpace(keyIdent) != "" && selSet {
		fmt.Fprintln(r.stderr, "Error: --key and -l selector cannot be used together")
		return 1
	}
	var requiredTags []string
	if selSet {
		if strings.TrimSpace(selector) == "" {
			fmt.Fprintln(r.stderr, "Error: -l selector cannot be empty")
			return 1
		}
		requiredTags = normalizeTags(strings.Split(selector, ","))
	}

	if strings.TrimSpace(name) == "" {
		if !stdinIsCharDevice() || !stderrIsCharDevice() {
			fmt.Fprintln(r.stderr, "Usage: agx use <site> [--agents codex,claude,gemini|all] [--key KEY | -l TAGS] [--dry-run] [-o json]")
			return 1
		}
		picked, err := r.pickTargetInteractive(agents)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		if strings.TrimSpace(picked) == "" {
			return 1
		}
		name = picked
	}

	res, err := r.switchSvc.SwitchByName(name, usecase.SwitchOptions{
		KeyIdentifier: keyIdent,
		KeyTags:       requiredTags,
		Families:      agents,
		DryRun:        dryRun,
	})
	if err != nil {
		if !dryRun && usecase.IsNoActiveKeyError(err) && stdinIsCharDevice() && stderrIsCharDevice() {
			if r.tryBootstrapKeysForName(name) {
				res, err = r.switchSvc.SwitchByName(name, usecase.SwitchOptions{
					KeyIdentifier: keyIdent,
					KeyTags:       requiredTags,
					Families:      agents,
					DryRun:        dryRun,
				})
			}
		}
	}
	if err != nil {
		if usecase.IsNoActiveKeyError(err) {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			tipSite := name
			if r.providerSvc != nil {
				if t, _, targetErr := r.resolveSiteTarget(name); targetErr == nil && t != nil && t.Access == domainprovider.AccessThirdParty {
					if _, profile, scopeErr := r.providerSvc.KeyScopeForTarget(*t); scopeErr == nil {
						profile = domainkey.NormalizeProfileName(profile)
						if profile != "" && profile != domainkey.DefaultProfile {
							tipSite = profile
						}
					}
				}
			}
			fmt.Fprintf(r.stderr, "Tip: run `agx create key --site %s --stdin` to paste/import keys, then retry.\n", tipSite)
			return 1
		}
		if usecase.IsKeySelectorNoMatchError(err) || usecase.IsKeySelectorAmbiguousError(err) {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			fmt.Fprintln(r.stderr, "Tip: use `--key <name>` to disambiguate, or activate one key in the site scope.")
			return 1
		}
		if domainprovider.IsTargetNotFoundError(err) {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			suggestions := []string{}
			if r.providerSvc != nil {
				for _, candidate := range []string{name + "-codex", name + "-claude", name + "-gemini"} {
					if t, targetErr := r.providerSvc.GetTarget(candidate); targetErr == nil && t != nil {
						suggestions = append(suggestions, candidate)
					}
				}
			}
			if len(suggestions) > 0 {
				fmt.Fprintf(r.stderr, "Tip: did you mean one of: %s\n", strings.Join(suggestions, ", "))
			} else {
				fmt.Fprintf(r.stderr, "Tip: run `agx create site %s`\n", name)
			}
			return 1
		}
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	if asJSON {
		type appliedView struct {
			Family  domainprovider.Family `json:"family"`
			Target  string                `json:"target"`
			Profile string                `json:"profile"`
			Key     keysPasteItemView     `json:"key"`
			Agent   string                `json:"agent"`
		}
		applied := make([]appliedView, 0, len(res.Applied))
		for _, a := range res.Applied {
			applied = append(applied, appliedView{
				Family:  a.Target.Family,
				Target:  a.Target.Name,
				Profile: a.Profile,
				Key: keysPasteItemView{
					ID:       a.Key.ID,
					Provider: a.Key.Provider,
					Profile:  domainkey.NormalizeProfileName(a.Key.Profile),
					Name:     a.Key.Name,
					Active:   a.Key.Active,
				},
				Agent: a.Agent.Name,
			})
		}
		payload := struct {
			Name    string        `json:"name"`
			DryRun  bool          `json:"dry_run,omitempty"`
			Primary appliedView   `json:"primary"`
			Applied []appliedView `json:"applied"`
		}{Name: name, Primary: appliedView{
			Family:  res.Primary.Target.Family,
			Target:  res.Primary.Target.Name,
			Profile: res.Primary.Profile,
			Key: keysPasteItemView{
				ID:       res.Primary.Key.ID,
				Provider: res.Primary.Key.Provider,
				Profile:  domainkey.NormalizeProfileName(res.Primary.Key.Profile),
				Name:     res.Primary.Key.Name,
				Active:   res.Primary.Key.Active,
			},
			Agent: res.Primary.Agent.Name,
		}, DryRun: dryRun, Applied: applied}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	if len(res.Applied) == 0 {
		fmt.Fprintln(r.stdout, "Using site: (no targets applied)")
		return 0
	}
	if dryRun {
		paths, _ := config.DefaultPaths()
		fmt.Fprintln(r.stdout, "Dry-run (no files written). Would update:")
		seen := map[string]struct{}{}
		for _, a := range res.Applied {
			switch a.Agent.Name {
			case "codex-cli":
				if _, ok := seen[a.Agent.Name]; !ok {
					fmt.Fprintf(r.stdout, "  - codex: %s, %s\n", paths.CodexAuthPath, paths.CodexConfigPath)
					seen[a.Agent.Name] = struct{}{}
				}
			case "claude-code":
				if _, ok := seen[a.Agent.Name]; !ok {
					fmt.Fprintf(r.stdout, "  - claude: %s\n", paths.ClaudeSettingsPath)
					seen[a.Agent.Name] = struct{}{}
				}
			case "gemini-cli":
				if _, ok := seen[a.Agent.Name]; !ok {
					fmt.Fprintf(r.stdout, "  - gemini: %s, %s\n", paths.GeminiSettingsPath, paths.GeminiEnvPath)
					seen[a.Agent.Name] = struct{}{}
				}
			}
		}
	}
	if len(res.Applied) == 1 {
		a := res.Applied[0]
		keyLabel := a.Key.Name
		if len(a.Key.ID) >= 8 {
			keyLabel = fmt.Sprintf("%s (%s)", a.Key.Name, a.Key.ID[:8])
		}
		if dryRun {
			fmt.Fprintf(r.stdout, "Would use %s [%s] key=%s\n", displayNameForTarget(a.Target), a.Target.Family, keyLabel)
		} else {
			fmt.Fprintf(r.stdout, "Using %s [%s] key=%s\n", displayNameForTarget(a.Target), a.Target.Family, keyLabel)
		}
		return 0
	}
	if dryRun {
		fmt.Fprintf(r.stdout, "Would use %s:\n", displayNameForTarget(res.Primary.Target))
	} else {
		fmt.Fprintf(r.stdout, "Using %s:\n", displayNameForTarget(res.Primary.Target))
	}
	for _, a := range res.Applied {
		keyLabel := a.Key.Name
		if len(a.Key.ID) >= 8 {
			keyLabel = fmt.Sprintf("%s (%s)", a.Key.Name, a.Key.ID[:8])
		}
		fmt.Fprintf(r.stdout, "  %s -> %s  key=%s\n", a.Target.Family, displayNameForTarget(a.Target), keyLabel)
	}
	return 0
}

func (r *Root) pickTargetInteractive(families []domainprovider.Family) (string, error) {
	if r.providerSvc == nil {
		return "", fmt.Errorf("provider config service is unavailable")
	}

	targets := r.providerSvc.ListTargets()
	if len(families) > 0 {
		allowed := make(map[domainprovider.Family]struct{}, len(families))
		for _, f := range families {
			allowed[f] = struct{}{}
		}
		filtered := make([]domainprovider.Target, 0, len(targets))
		for _, t := range targets {
			if _, ok := allowed[t.Family]; ok {
				filtered = append(filtered, t)
			}
		}
		targets = filtered
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Family != targets[j].Family {
			return targets[i].Family < targets[j].Family
		}
		return targets[i].Name < targets[j].Name
	})
	if len(targets) == 0 {
		return "", fmt.Errorf("no targets configured")
	}

	for i, t := range targets {
		baseURL := t.BaseURL
		if baseURL == "" {
			baseURL = "-"
		}
		fmt.Fprintf(r.stdout, "[%d] %s  family=%s access=%s base_url=%s\n", i+1, displayNameForTarget(t), t.Family, t.Access, baseURL)
	}
	fmt.Fprintf(r.stderr, "Select site [1-%d]: ", len(targets))
	var choice int
	if _, err := fmt.Fscanln(os.Stdin, &choice); err != nil {
		return "", fmt.Errorf("failed to read selection: %w", err)
	}
	if choice < 1 || choice > len(targets) {
		return "", fmt.Errorf("invalid selection")
	}
	return displayNameForTarget(targets[choice-1]), nil
}

func (r *Root) tryBootstrapKeysForName(name string) bool {
	if r.keySvc == nil || r.providerSvc == nil {
		return false
	}

	target, err := r.providerSvc.GetTarget(name)
	if err != nil {
		if family, ok := domainprovider.ParseFamily(name); ok {
			target, err = r.providerSvc.GetTarget(domainprovider.DefaultTargetName(family))
		}
	}
	if err != nil || target == nil {
		return false
	}

	provider, profile, err := r.providerSvc.KeyScopeForTarget(*target)
	if err != nil {
		return false
	}

	reader := bufio.NewReader(os.Stdin)
	keys, err := r.bootstrapImportKeys(reader, provider, profile)
	if err != nil || len(keys) == 0 {
		return false
	}
	_ = r.keySvc.Activate(keys[0].ID)
	return true
}

func parseAgentsFamilies(raw string) ([]domainprovider.Family, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("--agents cannot be empty")
	}
	if strings.EqualFold(raw, "all") {
		return []domainprovider.Family{
			domainprovider.FamilyOpenAI,
			domainprovider.FamilyClaude,
			domainprovider.FamilyGemini,
		}, nil
	}

	set := map[domainprovider.Family]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(strings.ToLower(part))
		if token == "" {
			continue
		}
		switch token {
		case "codex":
			set[domainprovider.FamilyOpenAI] = struct{}{}
		case "claude":
			set[domainprovider.FamilyClaude] = struct{}{}
		case "gemini":
			set[domainprovider.FamilyGemini] = struct{}{}
		default:
			return nil, fmt.Errorf("unknown agent %q (use codex,claude,gemini or all)", token)
		}
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("--agents cannot be empty")
	}

	out := make([]domainprovider.Family, 0, len(set))
	for _, family := range []domainprovider.Family{
		domainprovider.FamilyOpenAI,
		domainprovider.FamilyClaude,
		domainprovider.FamilyGemini,
	} {
		if _, ok := set[family]; ok {
			out = append(out, family)
		}
	}
	return out, nil
}
