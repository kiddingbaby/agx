package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

type statusBindingView struct {
	Family        domainprovider.Family `json:"family"`
	Target        string                `json:"target"`
	Profile       string                `json:"profile"`
	ActiveKeyName string                `json:"active_key_name,omitempty"`
	ActiveKeyID   string                `json:"active_key_id,omitempty"`
}

func (r *Root) handleStatus(args []string) int {
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx status [-o json]")
		return 0
	}

	asJSON := false
	if len(args) != 0 {
		if len(args) != 2 || args[0] != "-o" {
			fmt.Fprintln(r.stderr, "Usage: agx status [-o json]")
			return 1
		}
		if args[1] != "json" {
			fmt.Fprintf(r.stderr, "Error: unsupported output format: %s\n", args[1])
			return 1
		}
		asJSON = true
	}

	if r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: provider config service is unavailable")
		return 1
	}

	currentTarget := strings.TrimSpace(r.providerSvc.GetCurrentSite())
	currentName := ""
	if currentTarget != "" {
		currentName = currentTarget
		if t, err := r.providerSvc.GetTarget(currentTarget); err == nil && t != nil {
			currentName = displayNameForTarget(*t)
		}
	}

	bindings := r.providerSvc.ListBindings()
	out := make([]statusBindingView, 0, len(bindings))

	for _, binding := range bindings {
		keyProvider, _ := domainkey.ParseProvider(string(binding.Family))
		keyProfile := domainkey.DefaultProfile
		targetLabel := binding.Target
		target, err := r.providerSvc.GetTarget(binding.Target)
		if err == nil && target != nil {
			targetLabel = displayNameForTarget(*target)
			if provider, profile, err := r.providerSvc.KeyScopeForTarget(*target); err == nil {
				keyProvider = provider
				keyProfile = profile
			} else if target.Access == domainprovider.AccessThirdParty {
				keyProfile = domainkey.NormalizeProfileName(target.Name)
			}
		}

		view := statusBindingView{
			Family:  binding.Family,
			Target:  targetLabel,
			Profile: domainkey.NormalizeProfileName(keyProfile),
		}

		if r.keySvc != nil {
			if active, err := r.keySvc.GetActive(keyProvider, keyProfile); err == nil && active != nil {
				view.ActiveKeyName = active.Name
				if len(active.ID) >= 8 {
					view.ActiveKeyID = active.ID[:8]
				} else {
					view.ActiveKeyID = active.ID
				}
			}
		}

		out = append(out, view)
	}

	if asJSON {
		var current *struct {
			Name   string `json:"name"`
			Target string `json:"target"`
		}
		if currentTarget != "" {
			current = &struct {
				Name   string `json:"name"`
				Target string `json:"target"`
			}{Name: currentName, Target: currentTarget}
		}
		payload := struct {
			CurrentSite *struct {
				Name   string `json:"name"`
				Target string `json:"target"`
			} `json:"current_site,omitempty"`
			Bindings []statusBindingView `json:"bindings"`
		}{
			CurrentSite: current,
			Bindings:    out,
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	if currentTarget == "" {
		fmt.Fprintln(r.stdout, "Current site: (none)")
	} else {
		fmt.Fprintf(r.stdout, "Current site: %s\n", currentName)
	}
	fmt.Fprintln(r.stdout, "Bindings:")
	for _, b := range out {
		keyLabel := "(no key)"
		if strings.TrimSpace(b.ActiveKeyName) != "" {
			keyLabel = fmt.Sprintf("%s (%s)", b.ActiveKeyName, b.ActiveKeyID)
		}
		fmt.Fprintf(r.stdout, "  %s -> %s  key=%s\n", b.Family, b.Target, keyLabel)
	}
	return 0
}
