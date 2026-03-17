package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

func (r *Root) handleGetKeys(args []string) int {
	if r.keySvc == nil || r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: keys require key/provider services")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx get keys [--site <site>] [-A] [-l TAGS] [-o json]")
		return 0
	}

	var (
		siteArg     string
		allSites    bool
		selectorRaw string
		selectorSet bool
		asJSON      bool
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
		case "-A":
			allSites = true
		case "-l":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: -l requires a value")
				return 1
			}
			selectorRaw = args[i+1]
			selectorSet = true
			i++
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
			fmt.Fprintln(r.stderr, "Usage: agx get keys [--site <site>] [-A] [-l TAGS] [-o json]")
			return 1
		}
	}

	var requiredTags []string
	if selectorSet {
		if strings.TrimSpace(selectorRaw) == "" {
			fmt.Fprintln(r.stderr, "Error: -l selector cannot be empty")
			return 1
		}
		requiredTags = normalizeTags(strings.Split(selectorRaw, ","))
	}

	if allSites {
		type keyRow struct {
			Site string `json:"site"`
			keyView
		}
		rows := make([]keyRow, 0)
		for _, k := range r.keySvc.List() {
			normalizedProfile := domainkey.NormalizeProfileName(k.Profile)
			if !keyHasAllTags(k.Tags, requiredTags) {
				continue
			}

			site := normalizedProfile
			if normalizedProfile == domainkey.DefaultProfile {
				site = string(k.Provider)
			}
			rows = append(rows, keyRow{
				Site:    site,
				keyView: toKeyView(k, normalizedProfile),
			})
		}

		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Site != rows[j].Site {
				return rows[i].Site < rows[j].Site
			}
			if rows[i].Provider != rows[j].Provider {
				return rows[i].Provider < rows[j].Provider
			}
			if rows[i].Profile != rows[j].Profile {
				return rows[i].Profile < rows[j].Profile
			}
			if rows[i].Active != rows[j].Active {
				return rows[i].Active
			}
			return rows[i].Name < rows[j].Name
		})

		if asJSON {
			payload := struct {
				AllSites bool     `json:"all_sites"`
				Keys     []keyRow `json:"keys"`
			}{AllSites: true, Keys: rows}
			if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
				fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
				return 1
			}
			return 0
		}

		if len(rows) == 0 {
			fmt.Fprintln(r.stdout, "(no keys)")
			return 0
		}
		fmt.Fprintln(r.stdout, "Keys:")
		for _, row := range rows {
			active := " "
			if row.Active {
				active = "*"
			}
			tagLabel := "-"
			if len(row.Tags) > 0 {
				tagLabel = strings.Join(row.Tags, ",")
			}
			fmt.Fprintf(r.stdout, "  %s %s  site=%s tags=%s\n", active, row.Name, row.Site, tagLabel)
		}
		return 0
	}

	resolvedSite, err := r.resolveSiteArgOrCurrent(siteArg)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	target, provider, profile, err := r.resolveScopeForSite(resolvedSite)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		if domainprovider.IsTargetNotFoundError(err) {
			fmt.Fprintf(r.stderr, "Tip: run `agx create site %s`\n", resolvedSite)
		}
		return 1
	}

	keys := r.keySvc.List()
	filtered := make([]domainkey.Key, 0)
	for _, k := range keys {
		if k.Provider != provider {
			continue
		}
		if domainkey.NormalizeProfileName(k.Profile) != domainkey.NormalizeProfileName(profile) {
			continue
		}
		if !keyHasAllTags(k.Tags, requiredTags) {
			continue
		}
		filtered = append(filtered, k)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Active != filtered[j].Active {
			return filtered[i].Active
		}
		return filtered[i].Name < filtered[j].Name
	})

	if asJSON {
		views := make([]keyView, 0, len(filtered))
		for _, k := range filtered {
			views = append(views, toKeyView(k, domainkey.NormalizeProfileName(k.Profile)))
		}
		payload := struct {
			Site    string    `json:"site"`
			Target  string    `json:"target"`
			Profile string    `json:"profile"`
			Keys    []keyView `json:"keys"`
		}{
			Site:    displayNameForTarget(*target),
			Target:  target.Name,
			Profile: domainkey.NormalizeProfileName(profile),
			Keys:    views,
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "%s keys (%s/%s):\n", displayNameForTarget(*target), provider, domainkey.NormalizeProfileName(profile))
	if len(filtered) == 0 {
		fmt.Fprintln(r.stdout, "  (no keys)")
		fmt.Fprintf(r.stdout, "Tip: run `agx create key --site %s --stdin`\n", displayNameForTarget(*target))
		return 0
	}
	for _, k := range filtered {
		active := " "
		if k.Active {
			active = "*"
		}
		tagLabel := "-"
		if len(k.Tags) > 0 {
			tagLabel = strings.Join(k.Tags, ",")
		}
		fmt.Fprintf(r.stdout, "  %s %s  tags=%s\n", active, k.Name, tagLabel)
	}
	return 0
}

func (r *Root) handleDescribeKey(args []string) int {
	if r.keySvc == nil || r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: keys require key/provider services")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx describe key <key> [--site <site>] [-o json]")
		return 0
	}

	var (
		siteArg  string
		keyIdent string
		asJSON   bool
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
				fmt.Fprintln(r.stderr, "Usage: agx describe key <key> [--site <site>] [-o json]")
				return 1
			}
			if strings.TrimSpace(keyIdent) != "" {
				fmt.Fprintln(r.stderr, "Usage: agx describe key <key> [--site <site>] [-o json]")
				return 1
			}
			keyIdent = args[i]
		}
	}
	if strings.TrimSpace(keyIdent) == "" {
		fmt.Fprintln(r.stderr, "Usage: agx describe key <key> [--site <site>] [-o json]")
		return 1
	}

	resolvedSite, err := r.resolveSiteArgOrCurrent(siteArg)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	target, provider, profile, err := r.resolveScopeForSite(resolvedSite)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	k, err := r.keySvc.FindByIdentifierInScope(provider, profile, keyIdent)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if k == nil {
		fmt.Fprintln(r.stderr, "Error: key not found")
		return 1
	}

	view := toKeyView(*k, domainkey.NormalizeProfileName(k.Profile))
	if asJSON {
		payload := struct {
			Site   string  `json:"site"`
			Target string  `json:"target"`
			Key    keyView `json:"key"`
		}{
			Site:   displayNameForTarget(*target),
			Target: target.Name,
			Key:    view,
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "Key: %s\n", view.Name)
	fmt.Fprintf(r.stdout, "  id=%s\n", view.ID)
	fmt.Fprintf(r.stdout, "  site=%s provider=%s profile=%s\n", displayNameForTarget(*target), view.Provider, view.Profile)
	fmt.Fprintf(r.stdout, "  active=%t\n", view.Active)
	if strings.TrimSpace(view.BaseURL) != "" {
		fmt.Fprintf(r.stdout, "  base_url=%s\n", view.BaseURL)
	}
	if len(view.Tags) > 0 {
		fmt.Fprintf(r.stdout, "  tags=%s\n", strings.Join(view.Tags, ","))
	}
	if !view.CreatedAt.IsZero() {
		fmt.Fprintf(r.stdout, "  created_at=%s\n", view.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	if !view.UpdatedAt.IsZero() {
		fmt.Fprintf(r.stdout, "  updated_at=%s\n", view.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	return 0
}

func (r *Root) handleCreateKey(args []string) int {
	if r.keySvc == nil || r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: keys require key/provider services")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx create key [name] [--site <site>] (--stdin | --api-key ... | --api-key-env ... | --api-key-file ...) [--activate] [--tags TAGS] [-o json]")
		return 0
	}

	var (
		siteArg    string
		name       string
		apiKeyRaw  string
		apiKeyEnv  string
		apiKeyFile string
		fromStdin  bool
		activate   bool
		tagsRaw    string
		asJSON     bool
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
		case "--api-key":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --api-key requires a value")
				return 1
			}
			apiKeyRaw = args[i+1]
			i++
		case "--api-key-env":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --api-key-env requires a value")
				return 1
			}
			apiKeyEnv = args[i+1]
			i++
		case "--api-key-file":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --api-key-file requires a value")
				return 1
			}
			apiKeyFile = args[i+1]
			i++
		case "--stdin":
			fromStdin = true
		case "--activate":
			activate = true
		case "--tags":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --tags requires a value")
				return 1
			}
			tagsRaw = args[i+1]
			i++
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
				fmt.Fprintln(r.stderr, "Usage: agx create key [name] [--site <site>] (--stdin | --api-key ... | --api-key-env ... | --api-key-file ...) [--activate] [--tags TAGS] [-o json]")
				return 1
			}
			if strings.TrimSpace(name) != "" {
				fmt.Fprintln(r.stderr, "Usage: agx create key [name] [--site <site>] (--stdin | --api-key ... | --api-key-env ... | --api-key-file ...) [--activate] [--tags TAGS] [-o json]")
				return 1
			}
			name = args[i]
		}
	}

	resolvedSite, err := r.resolveSiteArgOrCurrent(siteArg)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		if strings.Contains(err.Error(), "--site") {
			fmt.Fprintln(r.stderr, "Tip: if you haven't used a site yet (no current site), pass --site explicitly.")
		}
		return 1
	}
	target, provider, profile, err := r.resolveScopeForSite(resolvedSite)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		if domainprovider.IsTargetNotFoundError(err) {
			fmt.Fprintf(r.stderr, "Tip: run `agx create site %s`\n", resolvedSite)
		}
		return 1
	}

	var tags []string
	if strings.TrimSpace(tagsRaw) != "" {
		tags = normalizeTags(strings.Split(tagsRaw, ","))
	}

	keySources := 0
	if strings.TrimSpace(apiKeyRaw) != "" {
		keySources++
	}
	if strings.TrimSpace(apiKeyEnv) != "" {
		keySources++
	}
	if strings.TrimSpace(apiKeyFile) != "" {
		keySources++
	}
	if fromStdin {
		if keySources > 0 || strings.TrimSpace(name) != "" {
			fmt.Fprintln(r.stderr, "Error: --stdin cannot be combined with key material flags or a name")
			return 1
		}
		keys, err := r.importKeysForScopeFromStdinOrInteractive(provider, profile, tags)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		if len(keys) == 0 {
			fmt.Fprintln(r.stderr, "Error: no keys imported")
			return 1
		}

		if asJSON {
			items := make([]keysPasteItemView, 0, len(keys))
			for _, k := range keys {
				items = append(items, keysPasteItemView{
					ID:       k.ID,
					Provider: k.Provider,
					Profile:  domainkey.NormalizeProfileName(k.Profile),
					Name:     k.Name,
					Active:   k.Active,
				})
			}
			payload := struct {
				Site   string              `json:"site"`
				Target string              `json:"target"`
				Keys   []keysPasteItemView `json:"keys"`
			}{
				Site:   displayNameForTarget(*target),
				Target: target.Name,
				Keys:   items,
			}
			if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
				fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
				return 1
			}
			return 0
		}

		fmt.Fprintf(r.stdout, "Imported keys: %d (%s/%s)\n", len(keys), provider, domainkey.NormalizeProfileName(profile))
		return 0
	}

	if keySources == 0 {
		// Interactive paste (TTY) when no secret flags were provided.
		if !stdinIsCharDevice() || !stderrIsCharDevice() {
			fmt.Fprintln(r.stderr, "Error: missing key material (use --api-key/--api-key-env/--api-key-file or --stdin)")
			return 1
		}
		keys, err := r.importKeysForScopeFromStdinOrInteractive(provider, profile, tags)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		if len(keys) == 0 {
			fmt.Fprintln(r.stderr, "Error: no keys imported")
			return 1
		}
		fmt.Fprintf(r.stdout, "Imported keys: %d (%s/%s)\n", len(keys), provider, domainkey.NormalizeProfileName(profile))
		return 0
	}

	if keySources > 1 {
		fmt.Fprintln(r.stderr, "Error: multiple key sources; choose one of --api-key/--api-key-env/--api-key-file")
		return 1
	}

	secret, err := resolveKeyMaterial(apiKeyRaw, apiKeyEnv, apiKeyFile)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	keyName := strings.TrimSpace(name)
	if keyName == "" {
		scopeCount := 0
		for _, existing := range r.keySvc.List() {
			if existing.Provider != provider {
				continue
			}
			if domainkey.NormalizeProfileName(existing.Profile) != domainkey.NormalizeProfileName(profile) {
				continue
			}
			scopeCount++
		}
		keyName, err = r.uniqueKeyName(provider, profile, fmt.Sprintf("%s-%02d", provider, scopeCount+1))
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
	} else {
		keyName, err = r.uniqueKeyName(provider, profile, keyName)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
	}

	k, err := r.keySvc.Add(provider, profile, keyName, secret, "", tags)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if activate {
		if err := r.keySvc.Activate(k.ID); err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
	}

	if asJSON {
		kv := keysPasteItemView{
			ID:       k.ID,
			Provider: k.Provider,
			Profile:  domainkey.NormalizeProfileName(k.Profile),
			Name:     k.Name,
			Active:   k.Active || activate,
		}
		if err := json.NewEncoder(r.stdout).Encode(struct {
			Site   string            `json:"site"`
			Target string            `json:"target"`
			Key    keysPasteItemView `json:"key"`
		}{
			Site:   displayNameForTarget(*target),
			Target: target.Name,
			Key:    kv,
		}); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	id := k.ID
	if len(id) >= 8 {
		id = id[:8]
	}
	fmt.Fprintf(r.stdout, "Added key: %s (%s) site=%s\n", k.Name, id, displayNameForTarget(*target))
	return 0
}

func (r *Root) handlePatchKey(args []string) int {
	if r.keySvc == nil || r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: keys require key/provider services")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx patch key <key> [--site <site>] [--name NEW] [--tags TAGS] [--activate] [-o json]")
		return 0
	}

	var (
		siteArg  string
		keyIdent string
		newName  string
		tagsRaw  string
		tagsSet  bool
		activate bool
		asJSON   bool
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
		case "--name":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --name requires a value")
				return 1
			}
			newName = args[i+1]
			i++
		case "--tags":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --tags requires a value")
				return 1
			}
			tagsRaw = args[i+1]
			tagsSet = true
			i++
		case "--activate":
			activate = true
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
				fmt.Fprintln(r.stderr, "Usage: agx patch key <key> [--site <site>] [--name NEW] [--tags TAGS] [--activate] [-o json]")
				return 1
			}
			if strings.TrimSpace(keyIdent) != "" {
				fmt.Fprintln(r.stderr, "Usage: agx patch key <key> [--site <site>] [--name NEW] [--tags TAGS] [--activate] [-o json]")
				return 1
			}
			keyIdent = args[i]
		}
	}

	if strings.TrimSpace(keyIdent) == "" {
		fmt.Fprintln(r.stderr, "Usage: agx patch key <key> [--site <site>] [--name NEW] [--tags TAGS] [--activate] [-o json]")
		return 1
	}
	if strings.TrimSpace(newName) == "" && !tagsSet && !activate {
		fmt.Fprintln(r.stderr, "Error: nothing to patch (use --name, --tags, and/or --activate)")
		return 1
	}
	if tagsSet && strings.TrimSpace(tagsRaw) == "" {
		// Allow clearing tags via: --tags ""
		tagsRaw = ""
	}

	resolvedSite, err := r.resolveSiteArgOrCurrent(siteArg)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	target, provider, profile, err := r.resolveScopeForSite(resolvedSite)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	k, err := r.keySvc.FindByIdentifierInScope(provider, profile, keyIdent)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if k == nil {
		fmt.Fprintln(r.stderr, "Error: key not found")
		return 1
	}

	updatedName := k.Name
	if strings.TrimSpace(newName) != "" {
		if strings.TrimSpace(newName) == k.Name {
			updatedName = k.Name
		} else {
			unique, err := r.uniqueKeyName(provider, profile, newName)
			if err != nil {
				fmt.Fprintf(r.stderr, "Error: %v\n", err)
				return 1
			}
			updatedName = unique
		}
	}

	updatedTags := k.Tags
	if tagsSet {
		if strings.TrimSpace(tagsRaw) == "" {
			updatedTags = nil
		} else {
			updatedTags = normalizeTags(strings.Split(tagsRaw, ","))
		}
	}

	out := k
	if strings.TrimSpace(updatedName) != k.Name || tagsSet {
		next, err := r.keySvc.Update(k.ID, k.Provider, k.Profile, updatedName, "", k.BaseURL, updatedTags)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		out = next
	}

	if activate {
		if err := r.keySvc.Activate(out.ID); err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		out.Active = true
	}

	view := toKeyView(*out, domainkey.NormalizeProfileName(out.Profile))
	if asJSON {
		payload := struct {
			Site   string  `json:"site"`
			Target string  `json:"target"`
			Key    keyView `json:"key"`
		}{
			Site:   displayNameForTarget(*target),
			Target: target.Name,
			Key:    view,
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "Patched key: %s site=%s\n", view.Name, displayNameForTarget(*target))
	return 0
}

func (r *Root) handleDeleteKey(args []string) int {
	if r.keySvc == nil || r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: keys require key/provider services")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx delete key <key> [--site <site>] [-o json]")
		return 0
	}

	var (
		siteArg  string
		keyIdent string
		asJSON   bool
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
				fmt.Fprintln(r.stderr, "Usage: agx delete key <key> [--site <site>] [-o json]")
				return 1
			}
			if strings.TrimSpace(keyIdent) != "" {
				fmt.Fprintln(r.stderr, "Usage: agx delete key <key> [--site <site>] [-o json]")
				return 1
			}
			keyIdent = args[i]
		}
	}

	if strings.TrimSpace(keyIdent) == "" {
		fmt.Fprintln(r.stderr, "Usage: agx delete key <key> [--site <site>] [-o json]")
		return 1
	}

	resolvedSite, err := r.resolveSiteArgOrCurrent(siteArg)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	target, provider, profile, err := r.resolveScopeForSite(resolvedSite)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	deleted, err := r.keySvc.DeleteByIdentifierInScope(provider, profile, keyIdent)
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if deleted == nil {
		fmt.Fprintln(r.stderr, "Error: key not found")
		return 1
	}

	if asJSON {
		view := toKeyView(*deleted, domainkey.NormalizeProfileName(deleted.Profile))
		payload := struct {
			Site   string  `json:"site"`
			Target string  `json:"target"`
			Key    keyView `json:"key"`
		}{
			Site:   displayNameForTarget(*target),
			Target: target.Name,
			Key:    view,
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	id := deleted.ID
	if len(id) >= 8 {
		id = id[:8]
	}
	fmt.Fprintf(r.stdout, "Deleted key: %s (%s) site=%s\n", deleted.Name, id, displayNameForTarget(*target))
	return 0
}

func (r *Root) resolveSiteArgOrCurrent(siteArg string) (string, error) {
	siteArg = strings.TrimSpace(siteArg)
	if siteArg != "" {
		return siteArg, nil
	}
	if r.providerSvc == nil {
		return "", errors.New("provider config service is unavailable")
	}
	current := strings.TrimSpace(r.providerSvc.GetCurrentSite())
	if current == "" {
		return "", fmt.Errorf("site is required (run `agx use <site>` or pass --site <site>)")
	}
	return current, nil
}

func keyHasAllTags(tags []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		set[t] = struct{}{}
	}
	for _, r := range required {
		if _, ok := set[r]; !ok {
			return false
		}
	}
	return true
}
