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
  ls [--provider P]              List all keys
  add --provider P --name N --key K [--base-url URL] [--tags T]  Add a new key
  activate <id|name>             Activate a key
  delete <id|name>               Delete a key

Without command: Open TUI Key Manager`)
}

func (r *Root) handleKeysLs(args []string) int {
	var providerFilter string
	for i := 0; i < len(args); i++ {
		if args[i] == "--provider" || args[i] == "-p" {
			if i+1 < len(args) {
				providerFilter = args[i+1]
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
		hasKeys := false
		for _, k := range keys {
			if k.Provider == provider {
				hasKeys = true
				active := " "
				if k.Active {
					active = "*"
				}
				fmt.Fprintf(r.stdout, "  %s %s  (%s)\n", active, k.Name, k.ID[:8])
			}
		}
		if !hasKeys {
			fmt.Fprintln(r.stdout, "  (no keys)")
		}
	}
	fmt.Fprintln(r.stdout)
	return 0
}

func (r *Root) handleKeysAdd(args []string) int {
	var provider, name, apiKey, baseURL, tagsStr string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-p":
			if i+1 < len(args) {
				provider = args[i+1]
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

	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	k, err := r.keySvc.Add(providerType, name, apiKey, baseURL, tags)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintf(r.stdout, "Added key: %s (%s)\n", k.Name, k.ID[:8])
	return 0
}

func (r *Root) handleKeysActivate(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Error: key ID or name required")
		fmt.Fprintln(r.stderr, "Usage: agx keys activate <id|name>")
		return 1
	}

	identifier := args[0]
	k, err := r.keySvc.ActivateByIdentifier(identifier)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: key not found: %s\n", identifier)
		return 1
	}

	fmt.Fprintf(r.stdout, "Activated key: %s [%s]\n", k.Name, k.Provider)
	return 0
}

func (r *Root) handleKeysDelete(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.stderr, "Error: key ID or name required")
		fmt.Fprintln(r.stderr, "Usage: agx keys delete <id|name>")
		return 1
	}

	identifier := args[0]
	k, err := r.keySvc.DeleteByIdentifier(identifier)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: key not found: %s\n", identifier)
		return 1
	}

	fmt.Fprintf(r.stdout, "Deleted key: %s\n", k.Name)
	return 0
}
