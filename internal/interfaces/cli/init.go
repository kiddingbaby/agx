package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiddingbaby/agx/internal/config"
)

const bundleTemplateYAML = `# AGX agx.yml (template)
#
# Apply:
#   agx apply ~/.config/agx/agx.yml
#   agx use <site>
#   codex | claude | gemini
#
# Notes:
# - Official sites are built-in by default:
#     openai / claude / gemini
#   (internal target names: openai-official / claude-official / gemini-official)
# - You can keep keys out of this file and manage them via:
#     agx create key --site <site> --stdin
#
# This template is safe to apply as-is (no changes).

keys: []
targets: []
bindings: []
profiles: []

# Example: add an OpenAI key from env
#
# keys:
#   - provider: openai
#     profile: default
#     name: oai-01
#     key-env: OPENAI_API_KEY
#     activate: true
#
# Example: bind OpenAI family to OpenRouter (OpenAI-compatible)
#
# targets:
#   - name: openrouter
#     family: openai
#     kind: openai-compatible
#     access: third_party
#     base-url: https://openrouter.ai/api/v1
#     wire-api: responses
#     requires-openai-auth: false
#
# bindings:
#   - family: openai
#     target: openrouter

# Example: global assets sync (system prompts / skills / MCP)
#
# assets:
#   skills-hub-home: "/path/to/skills-hub"
#   # file mode: "system-prompt/AGENTS.md"
#   # dir mode (recommended): "system-prompt" (contains AGENTS.md/CLAUDE.md/GEMINI.md)
#   system-prompt-path: "system-prompt"
#   # Empty means "disabled" (set to [codex, claude, gemini] when ready)
#   system-prompt-links: []
#   skills:
#     enabled: true
#     source: "skills/tools"
#     targets: [codex, claude]
#     prune: true
#   mcp:
#     enabled: false
#     targets: [codex, claude]
#     prune: true
#     servers:
#       - name: playwright
#         command: ["npx", "-y", "@playwright/mcp@latest"]
`

func (r *Root) handleInit(args []string) int {
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx init [--path PATH] [--force] [--print]")
		return 0
	}

	var (
		path  string
		force bool
		print bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--path":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --path requires a value")
				return 1
			}
			path = args[i+1]
			i++
		case "--force", "-f":
			force = true
		case "--print":
			print = true
		default:
			fmt.Fprintln(r.stderr, "Usage: agx init [--path PATH] [--force] [--print]")
			return 1
		}
	}

	content := strings.TrimSpace(bundleTemplateYAML) + "\n"
	if print {
		fmt.Fprint(r.stdout, content)
		return 0
	}

	if strings.TrimSpace(path) == "" {
		paths, err := config.DefaultPaths()
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		path = filepath.Join(paths.ConfigDir, "agx.yml")
	}

	if !force {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(r.stdout, "Config already exists: %s\n", path)
			return 0
		} else if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintf(r.stdout, "Wrote config template: %s\n", path)
	fmt.Fprintln(r.stdout, "Next: add keys via `agx create key --site <site> --stdin` (or fill keys in agx.yml), then run: `agx use <site>`")
	return 0
}
