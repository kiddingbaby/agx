package cli

import (
	"fmt"
	"strings"

	"github.com/kiddingbaby/agx/internal/usecase"
)

func (r *Root) handleAdd(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx add <relay> --base-url URL --api-key KEY [--bind codex,claude,gemini] [-o json]")
		return 0
	}

	const usage = "Usage: agx add <relay> --base-url URL --api-key KEY [--bind codex,claude,gemini] [-o json]"
	parsed, ok := parseProfileMutationArgs(r, args, usage)
	if !ok {
		return 1
	}
	if (strings.TrimSpace(parsed.name) == "" || parsed.baseURL == nil || parsed.apiKey == nil) && !r.canPrompt(parsed.asJSON) {
		fmt.Fprintln(r.stderr, usage)
		return 1
	}
	if strings.TrimSpace(parsed.name) == "" || parsed.baseURL == nil || parsed.apiKey == nil {
		session := newPromptSession(r.stdin)
		var err error
		parsed, err = r.promptForAdd(session, parsed)
		if err != nil {
			r.printUserError(err)
			return 1
		}
	}

	result, err := r.profiles.Add(parsed.name, usecase.AddProfileInput{
		BaseURL: *parsed.baseURL,
		APIKey:  *parsed.apiKey,
		Bind:    parsed.bind,
	})
	if err != nil {
		r.printUserError(err)
		return 1
	}
	if result.Bindings != nil {
		return r.writeBindingsResult(result.Bindings, parsed.asJSON)
	}
	view := toProfileView(*result.Relay)

	if parsed.asJSON {
		return r.writeJSON(struct {
			Relay profileView `json:"relay"`
		}{Relay: view})
	}

	fmt.Fprintf(r.stdout, "Added relay: %s\n", view.Name)
	fmt.Fprintf(r.stdout, "  base_url=%s\n", view.BaseURL)
	fmt.Fprintf(r.stdout, "  api_key=%s\n", view.APIKey)
	return 0
}

func (r *Root) handleEdit(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx edit <relay> [--base-url URL] [--api-key KEY] [--bind codex,claude,gemini] [--unbind codex,claude,gemini] [-o json]")
		return 0
	}

	const usage = "Usage: agx edit <relay> [--base-url URL] [--api-key KEY] [--bind codex,claude,gemini] [--unbind codex,claude,gemini] [-o json]"
	parsed, ok := parseProfileMutationArgs(r, args, usage)
	if !ok {
		return 1
	}
	var session *promptSession
	if r.canPrompt(parsed.asJSON) && (strings.TrimSpace(parsed.name) == "" || parsed.mutationFlags == 0) {
		session = newPromptSession(r.stdin)
	}
	if strings.TrimSpace(parsed.name) == "" && !r.canPrompt(parsed.asJSON) {
		fmt.Fprintln(r.stderr, usage)
		return 1
	}
	if strings.TrimSpace(parsed.name) == "" {
		var err error
		parsed.name, err = r.promptProfileName(session, "Profile name: ", "")
		if err != nil {
			r.printUserError(err)
			return 1
		}
	}
	if parsed.mutationFlags == 0 && !r.canPrompt(parsed.asJSON) {
		fmt.Fprintln(r.stderr, "Error: edit requires at least one of --base-url, --api-key, --bind, or --unbind")
		return 1
	}

	current, err := r.profiles.Get(parsed.name)
	if err != nil {
		r.printUserError(err)
		return 1
	}
	if parsed.mutationFlags == 0 {
		r.printInteractiveEditSummary(*current)
		parsed, err = r.promptForEdit(session, parsed, *current)
		if err != nil {
			r.printUserError(err)
			return 1
		}
		if parsed.mutationFlags == 0 {
			return 0
		}
	}

	result, err := r.profiles.Edit(parsed.name, usecase.EditProfileInput{
		BaseURL: parsed.baseURL,
		APIKey:  parsed.apiKey,
		Bind:    parsed.bind,
		Unbind:  parsed.unbind,
	})
	if err != nil {
		r.printUserError(err)
		return 1
	}
	if result.Bindings != nil {
		return r.writeBindingsResult(result.Bindings, parsed.asJSON)
	}
	view := toProfileView(*result.Relay)

	if parsed.asJSON {
		return r.writeJSON(struct {
			Relay profileView `json:"relay"`
		}{Relay: view})
	}

	fmt.Fprintf(r.stdout, "Edited relay: %s\n", view.Name)
	fmt.Fprintf(r.stdout, "  base_url=%s\n", view.BaseURL)
	fmt.Fprintf(r.stdout, "  api_key=%s\n", view.APIKey)
	return 0
}

func (r *Root) handleRemove(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx rm <relay> [-o json]")
		return 0
	}

	name, asJSON, ok := parseNameWithJSON(r, args, "Usage: agx rm <relay> [-o json]")
	if !ok {
		return 1
	}

	profile, err := r.profiles.Remove(name)
	if err != nil {
		r.printUserError(err)
		return 1
	}
	view := toProfileView(*profile)

	if asJSON {
		return r.writeJSON(struct {
			Relay profileView `json:"relay"`
		}{Relay: view})
	}

	fmt.Fprintf(r.stdout, "Removed relay: %s\n", view.Name)
	return 0
}

func (r *Root) handleInternalAPIKey(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(r.stderr, "Error: internal api key helper requires exactly one relay")
		return 1
	}

	apiKey, err := r.profiles.APIKey(args[0])
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintln(r.stdout, apiKey)
	return 0
}
