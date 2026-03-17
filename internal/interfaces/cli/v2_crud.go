package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

func (r *Root) handleGet(args []string) int {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprintln(r.stdout, "Usage: agx get <resource>")
		fmt.Fprintln(r.stdout, "Resources: sites | keys")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Usage: agx get <resource>")
		fmt.Fprintln(r.stderr, "Resources: sites | keys")
		return 1
	}
	switch args[0] {
	case "sites":
		return r.handleSiteLs(args[1:])
	case "keys":
		return r.handleGetKeys(args[1:])
	default:
		fmt.Fprintf(r.stderr, "Error: unknown resource: %s\n", args[0])
		fmt.Fprintln(r.stderr, "Resources: sites | keys")
		return 1
	}
}

func (r *Root) handleDescribe(args []string) int {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprintln(r.stdout, "Usage: agx describe <resource> <name>")
		fmt.Fprintln(r.stdout, "Resources: site | key")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Usage: agx describe <resource> <name>")
		fmt.Fprintln(r.stderr, "Resources: site | key")
		return 1
	}
	switch args[0] {
	case "site":
		return r.handleDescribeSite(args[1:])
	case "key":
		return r.handleDescribeKey(args[1:])
	default:
		fmt.Fprintf(r.stderr, "Error: unknown resource: %s\n", args[0])
		fmt.Fprintln(r.stderr, "Resources: site | key")
		return 1
	}
}

func (r *Root) handleCreate(args []string) int {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprintln(r.stdout, "Usage: agx create <resource> ...")
		fmt.Fprintln(r.stdout, "Resources: site | key")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Usage: agx create <resource> ...")
		fmt.Fprintln(r.stderr, "Resources: site | key")
		return 1
	}
	switch args[0] {
	case "site":
		return r.handleSiteAdd(args[1:])
	case "key":
		return r.handleCreateKey(args[1:])
	default:
		fmt.Fprintf(r.stderr, "Error: unknown resource: %s\n", args[0])
		fmt.Fprintln(r.stderr, "Resources: site | key")
		return 1
	}
}

func (r *Root) handlePatch(args []string) int {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprintln(r.stdout, "Usage: agx patch <resource> ...")
		fmt.Fprintln(r.stdout, "Resources: site | key")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Usage: agx patch <resource> ...")
		fmt.Fprintln(r.stderr, "Resources: site | key")
		return 1
	}
	switch args[0] {
	case "site":
		return r.handleSiteEdit(args[1:])
	case "key":
		return r.handlePatchKey(args[1:])
	default:
		fmt.Fprintf(r.stderr, "Error: unknown resource: %s\n", args[0])
		fmt.Fprintln(r.stderr, "Resources: site | key")
		return 1
	}
}

func (r *Root) handleDelete(args []string) int {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprintln(r.stdout, "Usage: agx delete <resource> <name>")
		fmt.Fprintln(r.stdout, "Resources: site | key")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Usage: agx delete <resource> <name>")
		fmt.Fprintln(r.stderr, "Resources: site | key")
		return 1
	}
	switch args[0] {
	case "site":
		return r.handleSiteRm(args[1:])
	case "key":
		return r.handleDeleteKey(args[1:])
	default:
		fmt.Fprintf(r.stderr, "Error: unknown resource: %s\n", args[0])
		fmt.Fprintln(r.stderr, "Resources: site | key")
		return 1
	}
}

func (r *Root) handleDescribeSite(args []string) int {
	if r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: provider config service is unavailable")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx describe site <site> [-o json]")
		return 0
	}

	var (
		siteArg string
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
				fmt.Fprintln(r.stderr, "Usage: agx describe site <site> [-o json]")
				return 1
			}
			if strings.TrimSpace(siteArg) != "" {
				fmt.Fprintln(r.stderr, "Usage: agx describe site <site> [-o json]")
				return 1
			}
			siteArg = args[i]
		}
	}
	if strings.TrimSpace(siteArg) == "" {
		fmt.Fprintln(r.stderr, "Usage: agx describe site <site> [-o json]")
		return 1
	}

	target, displayName, err := r.resolveSiteTarget(siteArg)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		if domainprovider.IsTargetNotFoundError(err) {
			fmt.Fprintf(r.stderr, "Tip: run `agx create site %s`\n", siteArg)
		}
		return 1
	}

	bound := false
	for _, b := range r.providerSvc.ListBindings() {
		if b.Family == target.Family && b.Target == target.Name {
			bound = true
			break
		}
	}

	current := strings.TrimSpace(r.providerSvc.GetCurrentSite())
	isCurrent := current != "" && current == target.Name

	view := siteView{
		Name:               displayName,
		Target:             target.Name,
		Family:             target.Family,
		Kind:               target.Kind,
		Access:             target.Access,
		BaseURL:            target.BaseURL,
		Model:              target.Model,
		WireAPI:            target.WireAPI,
		RequiresOpenAIAuth: target.RequiresOpenAIAuth,
		Bound:              bound,
	}
	if isCurrent {
		view.Current = true
	}

	keyProviderLabel := domainkey.Provider(view.Family)
	if r.keySvc != nil {
		provider, profile, scopeErr := r.providerSvc.KeyScopeForTarget(*target)
		if scopeErr == nil {
			keyProviderLabel = provider
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

	if asJSON {
		payload := struct {
			Site siteView `json:"site"`
		}{Site: view}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "Site: %s\n", view.Name)
	fmt.Fprintf(r.stdout, "  target=%s family=%s kind=%s access=%s\n", view.Target, view.Family, view.Kind, view.Access)
	if strings.TrimSpace(view.BaseURL) != "" {
		fmt.Fprintf(r.stdout, "  base_url=%s\n", view.BaseURL)
	}
	if strings.TrimSpace(view.Model) != "" {
		fmt.Fprintf(r.stdout, "  model=%s\n", view.Model)
	}
	if strings.TrimSpace(string(view.WireAPI)) != "" {
		fmt.Fprintf(r.stdout, "  wire_api=%s\n", view.WireAPI)
	}
	if view.RequiresOpenAIAuth != nil {
		fmt.Fprintf(r.stdout, "  requires_openai_auth=%t\n", *view.RequiresOpenAIAuth)
	}
	if strings.TrimSpace(view.KeyProfile) != "" {
		keyLabel := "(none)"
		if strings.TrimSpace(view.ActiveKeyName) != "" {
			keyLabel = fmt.Sprintf("%s (%s)", view.ActiveKeyName, view.ActiveKeyID)
		}
		fmt.Fprintf(r.stdout, "  keys=%s/%s active=%s\n", keyProviderLabel, view.KeyProfile, keyLabel)
	}
	flags := make([]string, 0, 2)
	if view.Bound {
		flags = append(flags, "bound")
	}
	if view.Current {
		flags = append(flags, "current")
	}
	if len(flags) > 0 {
		sort.Strings(flags)
		fmt.Fprintf(r.stdout, "  flags=%s\n", strings.Join(flags, ","))
	}
	return 0
}
