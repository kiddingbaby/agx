package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/usecase"
)

type siteTemplate struct {
	ID                 string
	Label              string
	Family             domainprovider.Family
	Kind               domainprovider.Kind
	Access             domainprovider.AccessMode
	BaseURLDefault     string
	BaseURLRequired    bool
	WireAPI            domainprovider.WireAPI
	RequiresOpenAIAuth *bool
}

const siteTemplateNewAPI = "newapi"

func siteTemplates() []siteTemplate {
	requiresNo := false
	return []siteTemplate{
		{
			ID:              "openai-compatible",
			Label:           "OpenAI-compatible gateway (for codex; enter host/base-url, AGX appends /v1) [Recommended]",
			Family:          domainprovider.FamilyOpenAI,
			Kind:            domainprovider.KindOpenAICompatible,
			Access:          domainprovider.AccessThirdParty,
			BaseURLRequired: true,
			WireAPI:         domainprovider.WireAPIResponses,
		},
		{
			ID:                 siteTemplateNewAPI,
			Label:              "Universal gateway (NewAPI/new-api; one host + one key; creates <name>-codex and optional siblings; use --agents to sync multiple; safe default only syncs codex)",
			Family:             domainprovider.FamilyOpenAI,
			Kind:               domainprovider.KindOpenAICompatible,
			Access:             domainprovider.AccessThirdParty,
			BaseURLRequired:    true,
			WireAPI:            domainprovider.WireAPIResponses,
			RequiresOpenAIAuth: &requiresNo,
		},
		{
			ID:                 "openrouter",
			Label:              "OpenRouter (OpenAI-compatible; base_url=https://openrouter.ai/api/v1)",
			Family:             domainprovider.FamilyOpenAI,
			Kind:               domainprovider.KindOpenAICompatible,
			Access:             domainprovider.AccessThirdParty,
			BaseURLDefault:     "https://openrouter.ai/api/v1",
			BaseURLRequired:    true,
			WireAPI:            domainprovider.WireAPIResponses,
			RequiresOpenAIAuth: &requiresNo,
		},
		{
			ID:              "claude-proxy",
			Label:           "Anthropic (Claude) compatible gateway (Messages API; enter host/base-url; AGX strips trailing /v1 if present)",
			Family:          domainprovider.FamilyClaude,
			Kind:            domainprovider.KindClaude,
			Access:          domainprovider.AccessThirdParty,
			BaseURLRequired: true,
		},
		{
			ID:              "gemini-proxy",
			Label:           "Gemini-compatible gateway (GenerateContent API; enter host/base-url; AGX strips trailing /v1 if present)",
			Family:          domainprovider.FamilyGemini,
			Kind:            domainprovider.KindGemini,
			Access:          domainprovider.AccessThirdParty,
			BaseURLRequired: true,
		},
	}
}

func findSiteTemplate(id string) (siteTemplate, bool) {
	for _, t := range siteTemplates() {
		if t.ID == id {
			return t, true
		}
	}
	return siteTemplate{}, false
}

type siteView struct {
	Name               string                    `json:"name"`
	Target             string                    `json:"target"`
	Family             domainprovider.Family     `json:"family"`
	Kind               domainprovider.Kind       `json:"kind"`
	Access             domainprovider.AccessMode `json:"access"`
	BaseURL            string                    `json:"base_url,omitempty"`
	Model              string                    `json:"model,omitempty"`
	WireAPI            domainprovider.WireAPI    `json:"wire_api,omitempty"`
	RequiresOpenAIAuth *bool                     `json:"requires_openai_auth,omitempty"`
	Bound              bool                      `json:"bound,omitempty"`
	Current            bool                      `json:"current,omitempty"`
	KeyProfile         string                    `json:"key_profile,omitempty"`
	ActiveKeyName      string                    `json:"active_key_name,omitempty"`
	ActiveKeyID        string                    `json:"active_key_id,omitempty"`
}

func displayNameForTarget(target domainprovider.Target) string {
	if target.Name == domainprovider.DefaultTargetName(target.Family) {
		return string(target.Family)
	}
	return target.Name
}

func internalTargetNameFromSiteArg(name string) (string, bool) {
	if family, ok := domainprovider.ParseFamily(name); ok {
		return domainprovider.DefaultTargetName(family), true
	}
	return name, false
}

func (r *Root) resolveSiteTarget(siteArg string) (*domainprovider.Target, string, error) {
	raw := strings.TrimSpace(siteArg)
	if raw == "" {
		return nil, "", errors.New("site name is required")
	}
	targetName, _ := internalTargetNameFromSiteArg(raw)
	target, err := r.providerSvc.GetTarget(targetName)
	if err != nil {
		return nil, "", err
	}
	return target, displayNameForTarget(*target), nil
}

func (r *Root) handleSiteLs(args []string) int {
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx get sites [-o json]")
		return 0
	}
	if r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: provider config service is unavailable")
		return 1
	}

	asJSON := false
	if len(args) != 0 {
		if len(args) != 2 || args[0] != "-o" {
			fmt.Fprintln(r.stderr, "Usage: agx get sites [-o json]")
			return 1
		}
		if args[1] != "json" {
			fmt.Fprintf(r.stderr, "Error: unsupported output format: %s\n", args[1])
			return 1
		}
		asJSON = true
	}

	bindings := r.providerSvc.ListBindings()
	bound := map[domainprovider.Family]string{}
	for _, b := range bindings {
		bound[b.Family] = b.Target
	}

	current := ""
	if r.providerSvc != nil {
		current = strings.TrimSpace(r.providerSvc.GetCurrentSite())
	}

	targets := r.providerSvc.ListTargets()
	views := make([]siteView, 0, len(targets))
	for _, t := range targets {
		view := siteView{
			Name:               displayNameForTarget(t),
			Target:             t.Name,
			Family:             t.Family,
			Kind:               t.Kind,
			Access:             t.Access,
			BaseURL:            t.BaseURL,
			Model:              t.Model,
			WireAPI:            t.WireAPI,
			RequiresOpenAIAuth: t.RequiresOpenAIAuth,
			Bound:              bound[t.Family] == t.Name,
			Current:            current != "" && current == t.Name,
		}
		if r.keySvc != nil {
			provider, profile, err := r.providerSvc.KeyScopeForTarget(t)
			if err == nil {
				view.KeyProfile = profile
				if active, err := r.keySvc.GetActive(provider, profile); err == nil && active != nil {
					view.ActiveKeyName = active.Name
					if len(active.ID) >= 8 {
						view.ActiveKeyID = active.ID[:8]
					} else {
						view.ActiveKeyID = active.ID
					}
				}
			}
		}
		views = append(views, view)
	}

	sort.Slice(views, func(i, j int) bool {
		if views[i].Family != views[j].Family {
			return views[i].Family < views[j].Family
		}
		return views[i].Name < views[j].Name
	})

	if asJSON {
		payload := struct {
			Sites []siteView `json:"sites"`
		}{Sites: views}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(r.stdout, "Sites:")
	for _, s := range views {
		line := fmt.Sprintf("  %s  family=%s kind=%s access=%s", s.Name, s.Family, s.Kind, s.Access)
		if strings.TrimSpace(s.BaseURL) != "" {
			line += fmt.Sprintf(" base_url=%s", s.BaseURL)
		}
		if strings.TrimSpace(s.Model) != "" {
			line += fmt.Sprintf(" model=%s", s.Model)
		}
		if strings.TrimSpace(string(s.WireAPI)) != "" {
			line += fmt.Sprintf(" wire_api=%s", s.WireAPI)
		}
		if s.RequiresOpenAIAuth != nil {
			line += fmt.Sprintf(" requires_openai_auth=%t", *s.RequiresOpenAIAuth)
		}
		if strings.TrimSpace(s.ActiveKeyName) != "" {
			line += fmt.Sprintf(" key=%s(%s)", s.ActiveKeyName, s.ActiveKeyID)
		} else {
			line += " key=(none)"
		}
		if s.Bound {
			line += " [bound]"
		}
		if s.Current {
			line += " [current]"
		}
		fmt.Fprintln(r.stdout, line)
	}
	return 0
}

func isReservedSiteName(name string) bool {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return true
	}
	if _, ok := domainprovider.ParseFamily(name); ok {
		return true
	}
	return false
}

func (r *Root) pickTemplateInteractive(reader *bufio.Reader) (string, error) {
	templates := siteTemplates()
	fmt.Fprintln(r.stderr, "Pick a template:")
	for i, t := range templates {
		fmt.Fprintf(r.stderr, "  %d) %s\n", i+1, t.Label)
	}
	fmt.Fprintf(r.stderr, "Select [1]: ")
	line, err := readLine(reader)
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return templates[0].ID, nil
	}
	switch strings.ToLower(line) {
	case "q", "quit", "exit":
		return "", nil
	}
	choice, err := parseMenuIndex(line, len(templates))
	if err != nil {
		return "", err
	}
	return templates[choice-1].ID, nil
}

func parseMenuIndex(raw string, max int) (int, error) {
	n, err := parsePositiveInt(raw)
	if err != nil {
		return 0, err
	}
	if n < 1 || n > max {
		return 0, fmt.Errorf("invalid selection")
	}
	return n, nil
}

func parsePositiveInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("invalid number")
	}
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid number")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func normalizeNewAPIHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", errors.New("base url is required")
	}
	host = strings.TrimRight(host, "/")
	if strings.HasSuffix(host, "/v1") {
		host = strings.TrimSuffix(host, "/v1")
		host = strings.TrimRight(host, "/")
	}
	if host == "" {
		return "", errors.New("base url is invalid")
	}
	if strings.ContainsAny(host, "\x00\n\r") {
		return "", errors.New("base url contains invalid characters")
	}
	return host, nil
}

func (r *Root) handleSiteAdd(args []string) int {
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx create site <name> [--template T] [--agents codex,claude,gemini|all] [--base-url URL] [--model M] [--wire-api responses] [--requires-openai-auth|--no-requires-openai-auth] [--env KEY=VALUE ...] [--no-keys] [-o json]")
		return 0
	}
	if r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: provider config service is unavailable")
		return 1
	}

	var (
		name               string
		templateID         string
		agentsRaw          string
		agentsSet          bool
		baseURL            string
		baseURLSet         bool
		model              string
		modelSet           bool
		wireAPI            string
		wireAPISet         bool
		requiresOpenAIAuth *bool
		noKeys             bool
		asJSON             bool
		envSet             = map[string]string{}
	)

	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name = args[0]
		args = args[1:]
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--template":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --template requires a value")
				return 1
			}
			templateID = strings.TrimSpace(args[i+1])
			i++
		case "--agents":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --agents requires a value")
				return 1
			}
			agentsRaw = args[i+1]
			agentsSet = true
			i++
		case "--base-url":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --base-url requires a value")
				return 1
			}
			baseURL = args[i+1]
			baseURLSet = true
			i++
		case "--model":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --model requires a value")
				return 1
			}
			model = args[i+1]
			modelSet = true
			i++
		case "--wire-api":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --wire-api requires a value")
				return 1
			}
			wireAPI = args[i+1]
			wireAPISet = true
			i++
		case "--requires-openai-auth":
			v := true
			requiresOpenAIAuth = &v
		case "--no-requires-openai-auth":
			v := false
			requiresOpenAIAuth = &v
		case "--env":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --env requires KEY=VALUE")
				return 1
			}
			raw := args[i+1]
			i++
			k, v, ok := strings.Cut(raw, "=")
			if !ok {
				fmt.Fprintln(r.stderr, "Error: --env requires KEY=VALUE")
				return 1
			}
			envSet[strings.TrimSpace(k)] = strings.TrimSpace(v)
		case "--no-keys":
			noKeys = true
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
			fmt.Fprintln(r.stderr, "Usage: agx create site <name> [--template T] [--agents codex,claude,gemini|all] [--base-url URL] [--model M] [--wire-api responses] [--requires-openai-auth|--no-requires-openai-auth] [--env KEY=VALUE ...] [--no-keys] [-o json]")
			return 1
		}
	}

	if wireAPISet && strings.EqualFold(strings.TrimSpace(wireAPI), "chat_completions") {
		fmt.Fprintln(r.stderr, "Error: unsupported --wire-api chat_completions (codex supports responses only)")
		return 1
	}

	interactive := stdinIsCharDevice() && stderrIsCharDevice()
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)

	name = strings.TrimSpace(name)
	if name == "" {
		if !interactive {
			fmt.Fprintln(r.stderr, "Usage: agx create site <name> [--template T] [--agents codex,claude,gemini|all] [--base-url URL] [--model M] [--wire-api responses] [--requires-openai-auth|--no-requires-openai-auth] [--env KEY=VALUE ...] [--no-keys] [-o json]")
			return 1
		}
		got, err := promptStringRequired(reader, r.stderr, "Site name (required): ")
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		name = strings.TrimSpace(got)
	}
	if isReservedSiteName(name) {
		fmt.Fprintf(r.stderr, "Error: site name %q is reserved\n", name)
		fmt.Fprintln(r.stderr, "Tip: use official aliases: openai | claude | gemini, or pick another site name.")
		return 1
	}
	if _, err := r.providerSvc.GetTarget(name); err == nil {
		fmt.Fprintf(r.stderr, "Error: site already exists: %s\n", name)
		return 1
	}

	if strings.TrimSpace(templateID) == "" {
		if !interactive {
			fmt.Fprintln(r.stderr, "Error: --template is required when not running in a TTY")
			return 1
		}
		picked, err := r.pickTemplateInteractive(reader)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		if strings.TrimSpace(picked) == "" {
			return 1
		}
		templateID = picked
	}

	tpl, ok := findSiteTemplate(templateID)
	if !ok {
		fmt.Fprintf(r.stderr, "Error: unknown template %q\n", templateID)
		return 1
	}

	var agents []domainprovider.Family
	if agentsSet {
		parsed, err := parseAgentsFamilies(agentsRaw)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		agents = parsed
	}

	if agentsSet && tpl.ID != siteTemplateNewAPI {
		fmt.Fprintln(r.stderr, "Error: --agents is only supported with --template newapi")
		return 1
	}

	if !baseURLSet {
		baseURL = tpl.BaseURLDefault
	}
	if tpl.BaseURLRequired && strings.TrimSpace(baseURL) == "" {
		if interactive {
			prompt := "Base URL (required): "
			if strings.TrimSpace(tpl.BaseURLDefault) != "" {
				prompt = fmt.Sprintf("Base URL [%s]: ", tpl.BaseURLDefault)
			}
			got, err := promptStringRequired(reader, r.stderr, prompt)
			if err != nil {
				fmt.Fprintf(r.stderr, "Error: %v\n", err)
				return 1
			}
			baseURL = got
		} else {
			fmt.Fprintln(r.stderr, "Error: --base-url is required for this template")
			return 1
		}
	}

	if !modelSet && interactive {
		got, err := promptString(reader, r.stderr, "Model (optional): ")
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		model = got
	}

	if requiresOpenAIAuth == nil && tpl.RequiresOpenAIAuth != nil {
		v := *tpl.RequiresOpenAIAuth
		requiresOpenAIAuth = &v
	}
	if !wireAPISet && strings.TrimSpace(string(tpl.WireAPI)) != "" {
		wireAPI = string(tpl.WireAPI)
		wireAPISet = true
	}

	var env map[string]string
	if len(envSet) > 0 {
		env = envSet
	}

	if tpl.ID == siteTemplateNewAPI {
		if len(agents) == 0 {
			agents = []domainprovider.Family{
				domainprovider.FamilyOpenAI,
				domainprovider.FamilyClaude,
				domainprovider.FamilyGemini,
			}
		}
		hasCodex := false
		for _, family := range agents {
			if family == domainprovider.FamilyOpenAI {
				hasCodex = true
				break
			}
		}
		if !hasCodex {
			fmt.Fprintln(r.stderr, "Error: --agents must include codex for --template newapi")
			return 1
		}

		return r.handleSiteAddNewAPI(name, baseURL, model, wireAPI, requiresOpenAIAuth, env, agents, noKeys, interactive, asJSON, reader)
	}

	saveIn := usecase.SaveTargetInput{
		Name:               name,
		Family:             string(tpl.Family),
		Kind:               string(tpl.Kind),
		Access:             string(tpl.Access),
		Auth:               string(domainprovider.AuthAPIKey),
		BaseURL:            strings.TrimSpace(baseURL),
		Model:              strings.TrimSpace(model),
		Env:                env,
		WireAPI:            strings.TrimSpace(wireAPI),
		RequiresOpenAIAuth: requiresOpenAIAuth,
	}

	target, err := r.providerSvc.SaveTarget(saveIn)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	var importedKeys []domainkey.Key
	if interactive && !noKeys && r.keySvc != nil {
		provider, profile, scopeErr := r.providerSvc.KeyScopeForTarget(*target)
		if scopeErr != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", scopeErr)
			return 1
		}
		fmt.Fprint(r.stderr, "\nAdd API keys now? [Y/n]: ")
		line, readErr := readLine(reader)
		if readErr != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", readErr)
			return 1
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || line == "y" || line == "yes" {
			keys, err := r.bootstrapImportKeys(reader, provider, profile)
			if err != nil {
				fmt.Fprintf(r.stderr, "Error: %v\n", err)
				return 1
			}
			importedKeys = keys
		}
	}

	if asJSON {
		payload := struct {
			Site     siteView            `json:"site"`
			Imported []keysPasteItemView `json:"imported_keys,omitempty"`
		}{
			Site: siteView{
				Name:               displayNameForTarget(*target),
				Target:             target.Name,
				Family:             target.Family,
				Kind:               target.Kind,
				Access:             target.Access,
				BaseURL:            target.BaseURL,
				Model:              target.Model,
				WireAPI:            target.WireAPI,
				RequiresOpenAIAuth: target.RequiresOpenAIAuth,
			},
		}
		if len(importedKeys) > 0 {
			items := make([]keysPasteItemView, 0, len(importedKeys))
			for _, k := range importedKeys {
				items = append(items, keysPasteItemView{
					ID:       k.ID,
					Provider: k.Provider,
					Profile:  domainkey.NormalizeProfileName(k.Profile),
					Name:     k.Name,
					Active:   k.Active,
				})
			}
			payload.Imported = items
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "Created site: %s [%s/%s access=%s]\n", target.Name, target.Family, target.Kind, target.Access)
	if len(importedKeys) > 0 {
		fmt.Fprintf(r.stdout, "Imported keys: %d\n", len(importedKeys))
	}
	fmt.Fprintf(r.stdout, "Next: run `agx use %s`\n", displayNameForTarget(*target))
	return 0
}

func (r *Root) handleSiteAddNewAPI(name, baseURL, model, wireAPI string, requiresOpenAIAuth *bool, env map[string]string, agents []domainprovider.Family, noKeys, interactive, asJSON bool, reader *bufio.Reader) int {
	host, err := normalizeNewAPIHost(baseURL)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: invalid base url: %v\n", err)
		return 1
	}

	if strings.HasSuffix(name, "-codex") || strings.HasSuffix(name, "-claude") || strings.HasSuffix(name, "-gemini") {
		fmt.Fprintf(r.stderr, "Error: newapi site name %q must not end with -codex/-claude/-gemini\n", name)
		return 1
	}

	codexName := name + "-codex"
	claudeName := name + "-claude"
	geminiName := name + "-gemini"

	wantClaude := false
	wantGemini := false
	for _, family := range agents {
		switch family {
		case domainprovider.FamilyClaude:
			wantClaude = true
		case domainprovider.FamilyGemini:
			wantGemini = true
		}
	}

	if _, err := r.providerSvc.GetTarget(codexName); err == nil {
		fmt.Fprintf(r.stderr, "Error: site already exists: %s\n", codexName)
		return 1
	} else if !domainprovider.IsTargetNotFoundError(err) {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if wantClaude {
		if _, err := r.providerSvc.GetTarget(claudeName); err == nil {
			fmt.Fprintf(r.stderr, "Error: site already exists: %s\n", claudeName)
			return 1
		} else if !domainprovider.IsTargetNotFoundError(err) {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
	}
	if wantGemini {
		if _, err := r.providerSvc.GetTarget(geminiName); err == nil {
			fmt.Fprintf(r.stderr, "Error: site already exists: %s\n", geminiName)
			return 1
		} else if !domainprovider.IsTargetNotFoundError(err) {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
	}

	openaiBaseURL := host + "/v1"
	openai, err := r.providerSvc.SaveTarget(usecase.SaveTargetInput{
		Name:               codexName,
		Family:             string(domainprovider.FamilyOpenAI),
		Kind:               string(domainprovider.KindOpenAICompatible),
		Access:             string(domainprovider.AccessThirdParty),
		Auth:               string(domainprovider.AuthAPIKey),
		BaseURL:            openaiBaseURL,
		Model:              strings.TrimSpace(model),
		Env:                env,
		WireAPI:            strings.TrimSpace(wireAPI),
		RequiresOpenAIAuth: requiresOpenAIAuth,
	})
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	createdTargets := []string{openai.Name}
	rollbackTargets := func() {
		for i := len(createdTargets) - 1; i >= 0; i-- {
			_ = r.providerSvc.DeleteTarget(createdTargets[i])
		}
	}

	var claude *domainprovider.Target
	if wantClaude {
		t, err := r.providerSvc.SaveTarget(usecase.SaveTargetInput{
			Name:    claudeName,
			Family:  string(domainprovider.FamilyClaude),
			Kind:    string(domainprovider.KindClaude),
			Access:  string(domainprovider.AccessThirdParty),
			Auth:    string(domainprovider.AuthAPIKey),
			BaseURL: host,
			Model:   "",
			Env:     env,
		})
		if err != nil {
			rollbackTargets()
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		createdTargets = append(createdTargets, t.Name)
		claude = t
	}

	var gemini *domainprovider.Target
	if wantGemini {
		t, err := r.providerSvc.SaveTarget(usecase.SaveTargetInput{
			Name:    geminiName,
			Family:  string(domainprovider.FamilyGemini),
			Kind:    string(domainprovider.KindGemini),
			Access:  string(domainprovider.AccessThirdParty),
			Auth:    string(domainprovider.AuthAPIKey),
			BaseURL: host,
			Model:   "",
			Env:     env,
		})
		if err != nil {
			rollbackTargets()
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		createdTargets = append(createdTargets, t.Name)
		gemini = t
	}

	var importedKeys []domainkey.Key
	if interactive && !noKeys && r.keySvc != nil {
		provider, profile, scopeErr := r.providerSvc.KeyScopeForTarget(*openai)
		if scopeErr != nil {
			rollbackTargets()
			fmt.Fprintf(r.stderr, "Error: %v\n", scopeErr)
			return 1
		}

		sharedLabel := "codex"
		if wantClaude && wantGemini {
			sharedLabel = "codex/claude/gemini"
		} else if wantClaude {
			sharedLabel = "codex/claude"
		} else if wantGemini {
			sharedLabel = "codex/gemini"
		}
		fmt.Fprintf(r.stderr, "\nAdd API keys now (shared for %s)? [Y/n]: ", sharedLabel)
		line, readErr := readLine(reader)
		if readErr != nil {
			rollbackTargets()
			fmt.Fprintf(r.stderr, "Error: %v\n", readErr)
			return 1
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || line == "y" || line == "yes" {
			keys, err := r.bootstrapImportKeys(reader, provider, profile)
			if err != nil {
				rollbackTargets()
				fmt.Fprintf(r.stderr, "Error: %v\n", err)
				return 1
			}
			importedKeys = keys
		}
	}

	if asJSON {
		payload := struct {
			Sites    []siteView          `json:"sites"`
			Imported []keysPasteItemView `json:"imported_keys,omitempty"`
		}{}
		payload.Sites = append(payload.Sites, siteView{
			Name:               displayNameForTarget(*openai),
			Target:             openai.Name,
			Family:             openai.Family,
			Kind:               openai.Kind,
			Access:             openai.Access,
			BaseURL:            openai.BaseURL,
			Model:              openai.Model,
			WireAPI:            openai.WireAPI,
			RequiresOpenAIAuth: openai.RequiresOpenAIAuth,
		})
		if claude != nil {
			payload.Sites = append(payload.Sites, siteView{
				Name:    displayNameForTarget(*claude),
				Target:  claude.Name,
				Family:  claude.Family,
				Kind:    claude.Kind,
				Access:  claude.Access,
				BaseURL: claude.BaseURL,
				Model:   claude.Model,
			})
		}
		if gemini != nil {
			payload.Sites = append(payload.Sites, siteView{
				Name:    displayNameForTarget(*gemini),
				Target:  gemini.Name,
				Family:  gemini.Family,
				Kind:    gemini.Kind,
				Access:  gemini.Access,
				BaseURL: gemini.BaseURL,
				Model:   gemini.Model,
			})
		}
		if len(importedKeys) > 0 {
			items := make([]keysPasteItemView, 0, len(importedKeys))
			for _, k := range importedKeys {
				items = append(items, keysPasteItemView{
					ID:       k.ID,
					Provider: k.Provider,
					Profile:  domainkey.NormalizeProfileName(k.Profile),
					Name:     k.Name,
					Active:   k.Active,
				})
			}
			payload.Imported = items
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "Created sites:\n")
	fmt.Fprintf(r.stdout, "  - %s [openai] base_url=%s\n", openai.Name, openai.BaseURL)
	if claude != nil {
		fmt.Fprintf(r.stdout, "  - %s [claude] base_url=%s\n", claude.Name, claude.BaseURL)
	}
	if gemini != nil {
		fmt.Fprintf(r.stdout, "  - %s [gemini] base_url=%s\n", gemini.Name, gemini.BaseURL)
	}
	if len(importedKeys) > 0 {
		fmt.Fprintf(r.stdout, "Imported keys: %d\n", len(importedKeys))
	}
	fmt.Fprintf(r.stdout, "Next:\n")
	fmt.Fprintf(r.stdout, "  - `agx use %s`\n", openai.Name)
	if claude != nil {
		fmt.Fprintf(r.stdout, "  - `agx use %s`\n", claude.Name)
	}
	if gemini != nil {
		fmt.Fprintf(r.stdout, "  - `agx use %s`\n", gemini.Name)
	}
	if claude != nil || gemini != nil {
		fmt.Fprintf(r.stdout, "  - Multi-sync: `agx use %s --agents codex,claude` (or `--agents all`)\n", openai.Name)
	}
	return 0
}

func (r *Root) handleSiteEdit(args []string) int {
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx patch site <site> [--base-url URL] [--model M] [--wire-api responses] [--requires-openai-auth|--no-requires-openai-auth|--clear-requires-openai-auth] [--env KEY=VALUE ...] [--env-unset KEY ...] [--clear-env] [--reset] [-o json]")
		return 0
	}
	if r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: provider config service is unavailable")
		return 1
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(r.stderr, "Usage: agx patch site <site> [--base-url URL] [--model M] [--wire-api responses] [--requires-openai-auth|--no-requires-openai-auth|--clear-requires-openai-auth] [--env KEY=VALUE ...] [--env-unset KEY ...] [--clear-env] [--reset] [-o json]")
		return 1
	}
	nameArg := args[0]
	args = args[1:]

	var (
		baseURL    string
		baseURLSet bool
		model      string
		modelSet   bool
		wireAPI    string
		wireAPISet bool

		requiresOpenAIAuth      *bool
		clearRequiresOpenAIAuth bool

		clearEnv bool
		envSet   = map[string]string{}
		envUnset []string

		reset  bool
		asJSON bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base-url":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --base-url requires a value")
				return 1
			}
			baseURL = args[i+1]
			baseURLSet = true
			i++
		case "--model":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --model requires a value")
				return 1
			}
			model = args[i+1]
			modelSet = true
			i++
		case "--wire-api":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --wire-api requires a value")
				return 1
			}
			wireAPI = args[i+1]
			wireAPISet = true
			i++
		case "--requires-openai-auth":
			v := true
			requiresOpenAIAuth = &v
		case "--no-requires-openai-auth":
			v := false
			requiresOpenAIAuth = &v
		case "--clear-requires-openai-auth":
			clearRequiresOpenAIAuth = true
		case "--env":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --env requires KEY=VALUE")
				return 1
			}
			raw := args[i+1]
			i++
			k, v, ok := strings.Cut(raw, "=")
			if !ok {
				fmt.Fprintln(r.stderr, "Error: --env requires KEY=VALUE")
				return 1
			}
			envSet[strings.TrimSpace(k)] = strings.TrimSpace(v)
		case "--env-unset":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --env-unset requires a key")
				return 1
			}
			envUnset = append(envUnset, strings.TrimSpace(args[i+1]))
			i++
		case "--clear-env":
			clearEnv = true
		case "--reset":
			reset = true
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
			fmt.Fprintln(r.stderr, "Usage: agx patch site <site> [--base-url URL] [--model M] [--wire-api responses] [--requires-openai-auth|--no-requires-openai-auth|--clear-requires-openai-auth] [--env KEY=VALUE ...] [--env-unset KEY ...] [--clear-env] [--reset] [-o json]")
			return 1
		}
	}

	if wireAPISet && strings.EqualFold(strings.TrimSpace(wireAPI), "chat_completions") {
		fmt.Fprintln(r.stderr, "Error: unsupported --wire-api chat_completions (codex supports responses only)")
		return 1
	}

	internalName, _ := internalTargetNameFromSiteArg(nameArg)
	if reset {
		if err := r.providerSvc.DeleteTarget(internalName); err != nil {
			// Resetting an official site is idempotent: if there's no override, treat as success.
			if strings.Contains(err.Error(), "cannot delete built-in target:") && internalName != nameArg {
				// no-op
			} else {
				fmt.Fprintf(r.stderr, "Error: %v\n", err)
				return 1
			}
		}
		if asJSON {
			_ = json.NewEncoder(r.stdout).Encode(struct {
				Reset string `json:"reset"`
			}{Reset: nameArg})
			return 0
		}
		fmt.Fprintf(r.stdout, "Reset site: %s\n", nameArg)
		return 0
	}

	existing, err := r.providerSvc.GetTarget(internalName)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if existing == nil {
		fmt.Fprintf(r.stderr, "Error: site not found: %s\n", nameArg)
		return 1
	}

	nextEnv := map[string]string{}
	for k, v := range existing.Env {
		nextEnv[k] = v
	}
	if clearEnv {
		nextEnv = map[string]string{}
	}
	for _, k := range envUnset {
		delete(nextEnv, k)
	}
	for k, v := range envSet {
		if strings.TrimSpace(k) == "" {
			continue
		}
		nextEnv[k] = v
	}
	if len(nextEnv) == 0 {
		nextEnv = nil
	}

	next := usecase.SaveTargetInput{
		Name:               existing.Name,
		Family:             string(existing.Family),
		Kind:               string(existing.Kind),
		Access:             string(existing.Access),
		Auth:               string(existing.Auth),
		BaseURL:            existing.BaseURL,
		Model:              existing.Model,
		Env:                nextEnv,
		WireAPI:            string(existing.WireAPI),
		RequiresOpenAIAuth: existing.RequiresOpenAIAuth,
	}
	if baseURLSet {
		next.BaseURL = strings.TrimSpace(baseURL)
	}
	if modelSet {
		next.Model = strings.TrimSpace(model)
	}
	if wireAPISet {
		next.WireAPI = strings.TrimSpace(wireAPI)
	}
	if clearRequiresOpenAIAuth {
		next.RequiresOpenAIAuth = nil
	} else if requiresOpenAIAuth != nil {
		next.RequiresOpenAIAuth = requiresOpenAIAuth
	}

	saved, err := r.providerSvc.SaveTarget(next)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	if asJSON {
		payload := struct {
			Site siteView `json:"site"`
		}{
			Site: siteView{
				Name:               displayNameForTarget(*saved),
				Target:             saved.Name,
				Family:             saved.Family,
				Kind:               saved.Kind,
				Access:             saved.Access,
				BaseURL:            saved.BaseURL,
				Model:              saved.Model,
				WireAPI:            saved.WireAPI,
				RequiresOpenAIAuth: saved.RequiresOpenAIAuth,
			},
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "Updated site: %s\n", displayNameForTarget(*saved))
	return 0
}

func (r *Root) handleSiteRm(args []string) int {
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx delete site <site> [-o json]")
		return 0
	}
	if r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: provider config service is unavailable")
		return 1
	}
	var (
		nameArg string
		asJSON  bool
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
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
				fmt.Fprintln(r.stderr, "Usage: agx delete site <site> [-o json]")
				return 1
			}
			if strings.TrimSpace(nameArg) != "" {
				fmt.Fprintln(r.stderr, "Usage: agx delete site <site> [-o json]")
				return 1
			}
			nameArg = strings.TrimSpace(args[i])
		}
	}
	if strings.TrimSpace(nameArg) == "" {
		fmt.Fprintln(r.stderr, "Usage: agx delete site <site> [-o json]")
		return 1
	}

	internalName, _ := internalTargetNameFromSiteArg(nameArg)
	if err := r.providerSvc.DeleteTarget(internalName); err != nil {
		// Official sites are reset-only; deleting when no override exists should be a no-op.
		if strings.Contains(err.Error(), "cannot delete built-in target:") && internalName != nameArg {
			if asJSON {
				_ = json.NewEncoder(r.stdout).Encode(struct {
					Action string `json:"action"`
					Site   string `json:"site"`
				}{Action: "reset", Site: nameArg})
				return 0
			}
			fmt.Fprintf(r.stdout, "Reset site: %s\n", nameArg)
			return 0
		}
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if _, isAlias := domainprovider.ParseFamily(nameArg); isAlias {
		if asJSON {
			_ = json.NewEncoder(r.stdout).Encode(struct {
				Action string `json:"action"`
				Site   string `json:"site"`
			}{Action: "reset", Site: nameArg})
			return 0
		}
		fmt.Fprintf(r.stdout, "Reset site: %s\n", nameArg)
		return 0
	}
	if asJSON {
		_ = json.NewEncoder(r.stdout).Encode(struct {
			Action string `json:"action"`
			Site   string `json:"site"`
		}{Action: "removed", Site: nameArg})
		return 0
	}
	fmt.Fprintf(r.stdout, "Removed site: %s\n", nameArg)
	return 0
}
