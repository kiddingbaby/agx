package cli

import (
	"fmt"
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
)

func (r *Root) handleKeys(args []string) int {
	if len(args) == 0 {
		if r.runKeyManager != nil {
			r.runKeyManager(r.keySvc)
		}
		return 0
	}

	switch args[0] {
	case "ls":
		return r.handleKeysLs(args[1:])
	case "add":
		return r.handleKeysAdd(args[1:])
	case "profile", "profiles":
		return r.handleKeysProfile(args[1:])
	case "activate":
		return r.handleKeysActivate(args[1:])
	case "delete", "rm":
		return r.handleKeysDelete(args[1:])
	default:
		fmt.Fprintf(r.stderr, "Unknown keys subcommand: %s\n", args[0])
		r.printKeysUsage()
		return 1
	}
}

func (r *Root) printKeysUsage() {
	fmt.Fprintln(r.stderr, `Usage: agx keys [command]

Commands:
  ls [--provider P] [--profile N]                          List all keys
  add --provider P --name N --key K [--profile N] [--base-url URL] [--tags T]
  activate <id|name> [--provider P --profile N]           Activate a key
  delete <id|name> [--provider P --profile N]             Delete a key
  profile ls --provider P                                  List provider profiles
  profile set --provider P --profile N --strategy S [--fixed-key ID|NAME]

Without command: Open TUI Key Manager`)
}

func (r *Root) handleKeysLs(args []string) int {
	var providerFilter string
	var profileFilter string
	for i := 0; i < len(args); i++ {
		if args[i] == "--provider" || args[i] == "-P" {
			if i+1 < len(args) {
				providerFilter = args[i+1]
				i++
			}
			continue
		}
		if args[i] == "--profile" || args[i] == "-p" {
			if i+1 < len(args) {
				profileFilter = domainkey.NormalizeProfileName(args[i+1])
				i++
			}
		}
	}

	keys := r.keySvc.List()
	if len(keys) == 0 {
		fmt.Fprintln(r.stdout, "No keys configured. Use 'agx keys add' or TUI (agx keys) to add keys.")
		return 0
	}

	providers := domainkey.SupportedProviders()
	for _, provider := range providers {
		if providerFilter != "" && string(provider) != providerFilter {
			continue
		}

		fmt.Fprintf(r.stdout, "\n%s:\n", strings.ToUpper(string(provider)))

		profiles := make(map[string][]domainkey.Key)
		for _, k := range keys {
			if k.Provider != provider {
				continue
			}
			profile := domainkey.NormalizeProfileName(k.Profile)
			if profileFilter != "" && profile != profileFilter {
				continue
			}
			profiles[profile] = append(profiles[profile], k)
		}

		if len(profiles) == 0 {
			fmt.Fprintln(r.stdout, "  (no keys)")
			continue
		}

		for profile, profileKeys := range profiles {
			fmt.Fprintf(r.stdout, "  [%s]\n", profile)
			for _, k := range profileKeys {
				active := " "
				if k.Active {
					active = "*"
				}
				fmt.Fprintf(r.stdout, "    %s %s  (%s)\n", active, k.Name, k.ID[:8])
			}
		}
	}
	fmt.Fprintln(r.stdout)
	return 0
}

func (r *Root) handleKeysAdd(args []string) int {
	var provider, profile, name, apiKey, baseURL, tagsStr string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-P":
			if i+1 < len(args) {
				provider = args[i+1]
				i++
			}
		case "--profile", "-p":
			if i+1 < len(args) {
				profile = args[i+1]
				i++
			}
		case "--name", "-n":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--key", "-k":
			if i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		case "--base-url", "-b":
			if i+1 < len(args) {
				baseURL = args[i+1]
				i++
			}
		case "--tags", "-t":
			if i+1 < len(args) {
				tagsStr = args[i+1]
				i++
			}
		}
	}

	if provider == "" || name == "" || apiKey == "" {
		fmt.Fprintln(r.stderr, "Error: --provider, --name, and --key are required")
		fmt.Fprintln(r.stderr, "Usage: agx keys add --provider P --name N --key K [--base-url URL] [--tags T]")
		return 1
	}

	providerType, ok := domainkey.ParseProvider(provider)
	if !ok {
		fmt.Fprintf(r.stderr, "Error: invalid provider '%s'. Valid: claude, openai, gemini\n", provider)
		return 1
	}

	profile = domainkey.NormalizeProfileName(profile)

	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	k, err := r.keySvc.Add(providerType, profile, name, apiKey, baseURL, tags)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintf(r.stdout, "Added key: %s (%s) profile=%s\n", k.Name, k.ID[:8], domainkey.NormalizeProfileName(k.Profile))
	return 0
}

func (r *Root) handleKeysActivate(args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(r.stderr, "Error: key ID or name required")
		fmt.Fprintln(r.stderr, "Usage: agx keys activate <id|name> [--provider P --profile N]")
		return 1
	}

	identifier := args[0]
	var providerRaw, profileRaw string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-P":
			if i+1 < len(args) {
				providerRaw = args[i+1]
				i++
			}
		case "--profile", "-p":
			if i+1 < len(args) {
				profileRaw = args[i+1]
				i++
			}
		}
	}

	var (
		k   *domainkey.Key
		err error
	)
	if providerRaw == "" {
		k, err = r.keySvc.ActivateByIdentifier(identifier)
	} else {
		provider, ok := domainkey.ParseProvider(providerRaw)
		if !ok {
			fmt.Fprintf(r.stderr, "Error: invalid provider '%s'. Valid: claude, openai, gemini\n", providerRaw)
			return 1
		}
		k, err = r.keySvc.ActivateByIdentifierInScope(provider, profileRaw, identifier)
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: key not found: %s\n", identifier)
		return 1
	}

	fmt.Fprintf(r.stdout, "Activated key: %s [%s/%s]\n", k.Name, k.Provider, domainkey.NormalizeProfileName(k.Profile))
	return 0
}

func (r *Root) handleKeysDelete(args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(r.stderr, "Error: key ID or name required")
		fmt.Fprintln(r.stderr, "Usage: agx keys delete <id|name> [--provider P --profile N]")
		return 1
	}

	identifier := args[0]
	var providerRaw, profileRaw string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-P":
			if i+1 < len(args) {
				providerRaw = args[i+1]
				i++
			}
		case "--profile", "-p":
			if i+1 < len(args) {
				profileRaw = args[i+1]
				i++
			}
		}
	}

	var (
		k   *domainkey.Key
		err error
	)
	if providerRaw == "" {
		k, err = r.keySvc.DeleteByIdentifier(identifier)
	} else {
		provider, ok := domainkey.ParseProvider(providerRaw)
		if !ok {
			fmt.Fprintf(r.stderr, "Error: invalid provider '%s'. Valid: claude, openai, gemini\n", providerRaw)
			return 1
		}
		k, err = r.keySvc.DeleteByIdentifierInScope(provider, profileRaw, identifier)
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: key not found: %s\n", identifier)
		return 1
	}

	fmt.Fprintf(r.stdout, "Deleted key: %s\n", k.Name)
	return 0
}

func (r *Root) handleKeysProfile(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Usage: agx keys profile <ls|set> ...")
		return 1
	}
	switch args[0] {
	case "ls":
		return r.handleKeysProfileLs(args[1:])
	case "set":
		return r.handleKeysProfileSet(args[1:])
	default:
		fmt.Fprintf(r.stderr, "Unknown profile subcommand: %s\n", args[0])
		return 1
	}
}

func (r *Root) handleKeysProfileLs(args []string) int {
	var providerRaw string
	for i := 0; i < len(args); i++ {
		if args[i] == "--provider" || args[i] == "-P" {
			if i+1 < len(args) {
				providerRaw = args[i+1]
				i++
			}
		}
	}
	if providerRaw == "" {
		fmt.Fprintln(r.stderr, "Usage: agx keys profile ls --provider P")
		return 1
	}
	provider, ok := domainkey.ParseProvider(providerRaw)
	if !ok {
		fmt.Fprintf(r.stderr, "Error: invalid provider '%s'. Valid: claude, openai, gemini\n", providerRaw)
		return 1
	}

	profiles := r.keySvc.ListProfiles(provider)
	if len(profiles) == 0 {
		fmt.Fprintln(r.stdout, "No profiles configured.")
		return 0
	}
	for _, p := range profiles {
		fmt.Fprintf(r.stdout, "%s\tstrategy=%s\tfixed_key=%s\n", p.Name, p.Strategy, p.FixedKey)
	}
	return 0
}

func (r *Root) handleKeysProfileSet(args []string) int {
	var providerRaw, profileRaw, strategyRaw, fixedKey string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-P":
			if i+1 < len(args) {
				providerRaw = args[i+1]
				i++
			}
		case "--profile", "-p":
			if i+1 < len(args) {
				profileRaw = args[i+1]
				i++
			}
		case "--strategy", "-s":
			if i+1 < len(args) {
				strategyRaw = args[i+1]
				i++
			}
		case "--fixed-key":
			if i+1 < len(args) {
				fixedKey = args[i+1]
				i++
			}
		}
	}

	if providerRaw == "" || profileRaw == "" || strategyRaw == "" {
		fmt.Fprintln(r.stderr, "Usage: agx keys profile set --provider P --profile N --strategy S [--fixed-key ID|NAME]")
		return 1
	}
	provider, ok := domainkey.ParseProvider(providerRaw)
	if !ok {
		fmt.Fprintf(r.stderr, "Error: invalid provider '%s'. Valid: claude, openai, gemini\n", providerRaw)
		return 1
	}
	strategy, ok := domainkey.ParseRotationStrategy(strategyRaw)
	if !ok {
		fmt.Fprintf(r.stderr, "Error: invalid strategy '%s'. Valid: fixed, round_robin, random\n", strategyRaw)
		return 1
	}

	if err := r.keySvc.SetProfileStrategy(provider, profileRaw, strategy, fixedKey); err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(r.stdout, "Updated profile: %s/%s strategy=%s\n", provider, domainkey.NormalizeProfileName(profileRaw), strategy)
	return 0
}
