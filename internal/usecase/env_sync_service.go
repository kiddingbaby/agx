package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/kiddingbaby/agx/internal/config"
	"github.com/kiddingbaby/agx/internal/ports"
)

type EnvSyncOptions struct {
	DryRun bool
}

type EnvAssetsConfig struct {
	SkillsHubHome     string
	SystemPromptPath  string
	SystemPromptLinks []string // codex|claude|gemini

	Skills SkillsAssetsConfig
	MCP    MCPAssetsConfig
}

type SkillsAssetsConfig struct {
	Enabled bool
	Source  string   // absolute path or relative to SkillsHubHome
	Targets []string // codex|claude
	Prune   bool
}

type MCPAssetsConfig struct {
	Enabled bool
	Targets []string // codex|claude
	Prune   bool
	Servers []MCPServerConfig
}

type MCPServerConfig struct {
	Name              string
	Command           []string
	Env               map[string]string
	URL               string
	BearerTokenEnvVar string
}

type EnvSyncService struct {
	paths  config.Paths
	runner ports.CommandRunner
	now    func() time.Time
}

func NewEnvSyncService(paths config.Paths, runner ports.CommandRunner) *EnvSyncService {
	return &EnvSyncService{
		paths:  paths,
		runner: runner,
		now:    time.Now,
	}
}

type EnvSyncFailure struct {
	Component string `json:"component"`
	Target    string `json:"target,omitempty"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
}

type SystemPromptSyncResult struct {
	LinkPath   string `json:"link_path"`
	TargetPath string `json:"target_path"`
	Action     string `json:"action"` // noop|linked|relinked|backed_up_and_linked|skipped|error
	BackupPath string `json:"backup_path,omitempty"`
	Error      string `json:"error,omitempty"`
}

type DirMirrorResult struct {
	Source       string `json:"source"`
	Destination  string `json:"destination"`
	Action       string `json:"action"` // noop|mirrored|skipped|error
	CreatedDirs  int    `json:"created_dirs,omitempty"`
	CopiedFiles  int    `json:"copied_files,omitempty"`
	UpdatedFiles int    `json:"updated_files,omitempty"`
	RemovedPaths int    `json:"removed_paths,omitempty"`
	Error        string `json:"error,omitempty"`
}

type MCPSyncAction struct {
	Op      string `json:"op"` // add|remove|replace|skip|error
	Name    string `json:"name"`
	Details string `json:"details,omitempty"`
	Error   string `json:"error,omitempty"`
}

type MCPSyncResult struct {
	Target  string          `json:"target"`
	Action  string          `json:"action"` // noop|synced|skipped|error
	Actions []MCPSyncAction `json:"actions,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type EnvSyncResult struct {
	SystemPrompts map[string]SystemPromptSyncResult `json:"system_prompts,omitempty"`
	Skills        map[string]DirMirrorResult        `json:"skills,omitempty"`
	MCP           map[string]MCPSyncResult          `json:"mcp,omitempty"`
	Failures      []EnvSyncFailure                  `json:"failures,omitempty"`
	Hints         []string                          `json:"hints,omitempty"`
}

func (s *EnvSyncService) Sync(opts EnvSyncOptions, cfg EnvAssetsConfig) EnvSyncResult {
	res := EnvSyncResult{
		SystemPrompts: map[string]SystemPromptSyncResult{},
		Skills:        map[string]DirMirrorResult{},
		MCP:           map[string]MCPSyncResult{},
	}

	if err := s.validate(); err != nil {
		res.Failures = append(res.Failures, EnvSyncFailure{
			Component: "config",
			Message:   err.Error(),
		})
		res.Hints = append(res.Hints, "Run `agx sync -o json` to inspect failures; re-run after fixing config or missing assets.")
		return res
	}

	cfg = normalizeEnvAssetsConfig(cfg)

	if len(cfg.SystemPromptLinks) > 0 {
		s.syncSystemPrompts(opts, cfg, &res)
	}
	if cfg.Skills.Enabled {
		s.syncSkills(opts, cfg, &res)
	}
	if cfg.MCP.Enabled {
		s.syncMCP(opts, cfg, &res)
	}

	if len(res.Failures) > 0 {
		res.Hints = append(res.Hints, "Run `agx sync -o json` to inspect failures; re-run after fixing config or missing assets.")
	}
	return res
}

func normalizeEnvAssetsConfig(cfg EnvAssetsConfig) EnvAssetsConfig {
	cfg.SkillsHubHome = strings.TrimSpace(cfg.SkillsHubHome)
	cfg.SystemPromptPath = strings.TrimSpace(cfg.SystemPromptPath)
	if cfg.SystemPromptLinks == nil {
		cfg.SystemPromptLinks = []string{"codex", "claude", "gemini"}
	}
	if cfg.SystemPromptPath == "" {
		cfg.SystemPromptPath = "system-prompt"
	}

	cfg.Skills.Source = strings.TrimSpace(cfg.Skills.Source)
	if cfg.Skills.Source == "" {
		cfg.Skills.Source = "skills/tools"
	}
	if cfg.Skills.Targets == nil {
		cfg.Skills.Targets = []string{"codex", "claude"}
	}

	if cfg.MCP.Enabled && cfg.MCP.Targets == nil {
		cfg.MCP.Targets = []string{"codex", "claude"}
	}

	for i := range cfg.MCP.Servers {
		cfg.MCP.Servers[i].Name = strings.TrimSpace(cfg.MCP.Servers[i].Name)
		cfg.MCP.Servers[i].URL = strings.TrimSpace(cfg.MCP.Servers[i].URL)
		cfg.MCP.Servers[i].BearerTokenEnvVar = strings.TrimSpace(cfg.MCP.Servers[i].BearerTokenEnvVar)
		if cfg.MCP.Servers[i].Env == nil {
			cfg.MCP.Servers[i].Env = map[string]string{}
		}
	}

	return cfg
}

func (s *EnvSyncService) syncSystemPrompts(opts EnvSyncOptions, cfg EnvAssetsConfig, res *EnvSyncResult) {
	sourceRoot := cfg.SystemPromptPath
	if !filepath.IsAbs(sourceRoot) {
		if cfg.SkillsHubHome == "" {
			res.Failures = append(res.Failures, EnvSyncFailure{
				Component: "system_prompt",
				Message:   "system-prompt-path is relative but skills-hub-home is empty",
				Detail:    fmt.Sprintf("system-prompt-path=%q", cfg.SystemPromptPath),
			})
			return
		}
		sourceRoot = filepath.Join(cfg.SkillsHubHome, cfg.SystemPromptPath)
	}
	sourceInfo, err := os.Stat(sourceRoot)
	if err != nil {
		res.Failures = append(res.Failures, EnvSyncFailure{
			Component: "system_prompt",
			Message:   "system prompt source path not found",
			Detail:    sourceRoot,
		})
		return
	}
	sourceIsDir := sourceInfo.IsDir()

	for _, t := range cfg.SystemPromptLinks {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" {
			continue
		}
		linkPath, ok := s.systemPromptLinkPathForTarget(t)
		if !ok {
			res.Failures = append(res.Failures, EnvSyncFailure{
				Component: "system_prompt",
				Target:    t,
				Message:   "unknown target (expected codex|claude|gemini)",
			})
			continue
		}
		targetPath := sourceRoot
		if sourceIsDir {
			filename, ok := systemPromptSourceFilenameForTarget(t)
			if !ok {
				res.Failures = append(res.Failures, EnvSyncFailure{
					Component: "system_prompt",
					Target:    t,
					Message:   "unknown target (expected codex|claude|gemini)",
				})
				continue
			}
			targetPath = filepath.Join(sourceRoot, filename)
			if _, err := os.Stat(targetPath); err != nil {
				res.Failures = append(res.Failures, EnvSyncFailure{
					Component: "system_prompt",
					Target:    t,
					Message:   "system prompt source file not found in directory",
					Detail:    targetPath,
				})
				continue
			}
		}
		if opts.DryRun {
			action := "noop"
			if drift, _ := symlinkDrift(linkPath, targetPath); drift {
				action = "skipped"
			}
			res.SystemPrompts[t] = SystemPromptSyncResult{
				LinkPath:   linkPath,
				TargetPath: targetPath,
				Action:     action,
			}
			continue
		}

		action, backupPath, err := ensureSymlink(linkPath, targetPath, s.now)
		out := SystemPromptSyncResult{
			LinkPath:   linkPath,
			TargetPath: targetPath,
			Action:     action,
			BackupPath: backupPath,
		}
		if err != nil {
			out.Action = "error"
			out.Error = err.Error()
			res.Failures = append(res.Failures, EnvSyncFailure{
				Component: "system_prompt",
				Target:    t,
				Message:   "failed to ensure symlink",
				Detail:    err.Error(),
			})
		}
		res.SystemPrompts[t] = out
	}
}

func (s *EnvSyncService) systemPromptLinkPathForTarget(target string) (string, bool) {
	switch target {
	case "codex":
		return filepath.Join(s.paths.CodexDir, "AGENTS.md"), true
	case "claude":
		return filepath.Join(s.paths.ClaudeDir, "CLAUDE.md"), true
	case "gemini":
		return filepath.Join(s.paths.GeminiDir, "GEMINI.md"), true
	default:
		return "", false
	}
}

func systemPromptSourceFilenameForTarget(target string) (string, bool) {
	switch target {
	case "codex":
		return "AGENTS.md", true
	case "claude":
		return "CLAUDE.md", true
	case "gemini":
		return "GEMINI.md", true
	default:
		return "", false
	}
}

func (s *EnvSyncService) syncSkills(opts EnvSyncOptions, cfg EnvAssetsConfig, res *EnvSyncResult) {
	src := cfg.Skills.Source
	if !filepath.IsAbs(src) {
		if cfg.SkillsHubHome == "" {
			res.Failures = append(res.Failures, EnvSyncFailure{
				Component: "skills",
				Message:   "skills.source is relative but skills-hub-home is empty",
				Detail:    fmt.Sprintf("skills.source=%q", cfg.Skills.Source),
			})
			return
		}
		src = filepath.Join(cfg.SkillsHubHome, cfg.Skills.Source)
	}
	if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
		res.Failures = append(res.Failures, EnvSyncFailure{
			Component: "skills",
			Message:   "skills source directory not found",
			Detail:    src,
		})
		return
	}

	for _, t := range cfg.Skills.Targets {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" {
			continue
		}
		dst, ok := s.skillsToolsDirForTarget(t)
		if !ok {
			res.Failures = append(res.Failures, EnvSyncFailure{
				Component: "skills",
				Target:    t,
				Message:   "unknown target (expected codex|claude)",
			})
			continue
		}
		if opts.DryRun {
			res.Skills[t] = DirMirrorResult{Source: src, Destination: dst, Action: "skipped"}
			continue
		}
		report, err := mirrorDir(src, dst, mirrorOptions{Prune: cfg.Skills.Prune})
		out := DirMirrorResult{
			Source:       src,
			Destination:  dst,
			Action:       "mirrored",
			CreatedDirs:  report.CreatedDirs,
			CopiedFiles:  report.CopiedFiles,
			UpdatedFiles: report.UpdatedFiles,
			RemovedPaths: report.RemovedPaths,
		}
		if err != nil {
			out.Action = "error"
			out.Error = err.Error()
			res.Failures = append(res.Failures, EnvSyncFailure{
				Component: "skills",
				Target:    t,
				Message:   "failed to sync skills",
				Detail:    err.Error(),
			})
		}
		res.Skills[t] = out
	}
}

func (s *EnvSyncService) skillsToolsDirForTarget(target string) (string, bool) {
	switch target {
	case "codex":
		return filepath.Join(s.paths.CodexDir, "skills", "tools"), true
	case "claude":
		return filepath.Join(s.paths.ClaudeDir, "skills", "tools"), true
	default:
		return "", false
	}
}

func (s *EnvSyncService) syncMCP(opts EnvSyncOptions, cfg EnvAssetsConfig, res *EnvSyncResult) {
	for _, t := range cfg.MCP.Targets {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" {
			continue
		}
		switch t {
		case "codex":
			res.MCP[t] = s.syncCodexMCP(opts, cfg)
		case "claude":
			res.MCP[t] = s.syncClaudeMCP(opts, cfg)
		default:
			res.Failures = append(res.Failures, EnvSyncFailure{
				Component: "mcp",
				Target:    t,
				Message:   "unknown target (expected codex|claude)",
			})
		}
	}
}

type codexMCPListItem struct {
	Name      string          `json:"name"`
	Transport json.RawMessage `json:"transport"`
}

type codexMCPTransport struct {
	Type string `json:"type"`

	// stdio
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// streamable_http
	URL               string `json:"url,omitempty"`
	BearerTokenEnvVar string `json:"bearer_token_env_var,omitempty"`
}

func (s *EnvSyncService) syncCodexMCP(opts EnvSyncOptions, cfg EnvAssetsConfig) MCPSyncResult {
	out := MCPSyncResult{Target: "codex", Action: "noop"}
	if s.runner == nil {
		out.Action = "skipped"
		out.Error = "no command runner configured"
		return out
	}

	list := s.runner.Run("codex", []string{"mcp", "list", "--json"}, nil)
	if list.Err != nil {
		out.Action = "error"
		out.Error = strings.TrimSpace(string(list.Stderr))
		if out.Error == "" {
			out.Error = list.Err.Error()
		}
		return out
	}

	var current []codexMCPListItem
	if err := json.Unmarshal(bytes.TrimSpace(list.Stdout), &current); err != nil {
		out.Action = "error"
		out.Error = fmt.Sprintf("parse `codex mcp list --json`: %v", err)
		return out
	}

	currentByName := map[string]codexMCPTransport{}
	for _, item := range current {
		var tr codexMCPTransport
		if err := json.Unmarshal(item.Transport, &tr); err != nil {
			continue
		}
		currentByName[item.Name] = tr
	}

	desired := normalizeMCPServers(cfg.MCP.Servers)
	desiredByName := map[string]MCPServerConfig{}
	for _, s := range desired {
		if strings.TrimSpace(s.Name) == "" {
			continue
		}
		desiredByName[s.Name] = s
	}

	toRemove := []string{}
	toAdd := []MCPServerConfig{}
	for name, want := range desiredByName {
		if got, ok := currentByName[name]; ok {
			if codexMCPTransportMatches(want, got) {
				continue
			}
			toRemove = append(toRemove, name)
			toAdd = append(toAdd, want)
		} else {
			toAdd = append(toAdd, want)
		}
	}

	if cfg.MCP.Prune {
		for name := range currentByName {
			if _, ok := desiredByName[name]; ok {
				continue
			}
			toRemove = append(toRemove, name)
		}
	}

	sort.Strings(toRemove)
	sort.Slice(toAdd, func(i, j int) bool { return toAdd[i].Name < toAdd[j].Name })

	if len(toRemove) == 0 && len(toAdd) == 0 {
		return out
	}
	out.Action = "synced"

	if opts.DryRun {
		for _, name := range toRemove {
			out.Actions = append(out.Actions, MCPSyncAction{Op: "remove", Name: name, Details: "dry-run"})
		}
		for _, srv := range toAdd {
			op := "add"
			if _, ok := currentByName[srv.Name]; ok {
				op = "replace"
			}
			out.Actions = append(out.Actions, MCPSyncAction{Op: op, Name: srv.Name, Details: "dry-run"})
		}
		return out
	}

	for _, name := range toRemove {
		r := s.runner.Run("codex", []string{"mcp", "remove", name}, nil)
		act := MCPSyncAction{Op: "remove", Name: name}
		if r.Err != nil {
			act.Op = "error"
			act.Error = strings.TrimSpace(string(r.Stderr))
			if act.Error == "" {
				act.Error = r.Err.Error()
			}
		}
		out.Actions = append(out.Actions, act)
	}

	for _, srv := range toAdd {
		act := MCPSyncAction{Op: "add", Name: srv.Name}
		if _, ok := currentByName[srv.Name]; ok {
			act.Op = "replace"
		}
		args := buildCodexMCPAddArgs(srv)
		r := s.runner.Run("codex", args, nil)
		if r.Err != nil {
			act.Op = "error"
			act.Error = strings.TrimSpace(string(r.Stderr))
			if act.Error == "" {
				act.Error = r.Err.Error()
			}
		}
		out.Actions = append(out.Actions, act)
	}

	for _, a := range out.Actions {
		if a.Op == "error" {
			out.Action = "error"
			out.Error = "one or more MCP actions failed"
			break
		}
	}
	return out
}

func normalizeMCPServers(in []MCPServerConfig) []MCPServerConfig {
	out := make([]MCPServerConfig, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s.Name = strings.TrimSpace(s.Name)
		if s.Name == "" {
			continue
		}
		if _, ok := seen[s.Name]; ok {
			continue
		}
		seen[s.Name] = struct{}{}
		out = append(out, s)
	}
	return out
}

func codexMCPTransportMatches(want MCPServerConfig, got codexMCPTransport) bool {
	if strings.TrimSpace(want.URL) != "" {
		return got.Type == "streamable_http" &&
			strings.TrimSpace(got.URL) == strings.TrimSpace(want.URL) &&
			strings.TrimSpace(got.BearerTokenEnvVar) == strings.TrimSpace(want.BearerTokenEnvVar)
	}
	if len(want.Command) == 0 {
		return false
	}
	wantCmd := want.Command[0]
	wantArgs := []string{}
	if len(want.Command) > 1 {
		wantArgs = want.Command[1:]
	}
	return got.Type == "stdio" &&
		got.Command == wantCmd &&
		reflect.DeepEqual(nilToEmptyStringSlice(got.Args), nilToEmptyStringSlice(wantArgs)) &&
		reflect.DeepEqual(nilToEmptyStringMap(got.Env), nilToEmptyStringMap(want.Env))
}

func nilToEmptyStringSlice(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nilToEmptyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	return in
}

func buildCodexMCPAddArgs(srv MCPServerConfig) []string {
	args := []string{"mcp", "add", srv.Name}
	if strings.TrimSpace(srv.URL) != "" {
		args = append(args, "--url", srv.URL)
		if strings.TrimSpace(srv.BearerTokenEnvVar) != "" {
			args = append(args, "--bearer-token-env-var", srv.BearerTokenEnvVar)
		}
		return args
	}
	envKeys := make([]string, 0, len(srv.Env))
	for k := range srv.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		args = append(args, "--env", k+"="+srv.Env[k])
	}
	args = append(args, "--")
	args = append(args, srv.Command...)
	return args
}

func (s *EnvSyncService) syncClaudeMCP(opts EnvSyncOptions, cfg EnvAssetsConfig) MCPSyncResult {
	out := MCPSyncResult{Target: "claude", Action: "noop"}
	claudeConfigPath := filepath.Join(s.paths.HomeDir, ".claude.json")

	desired := map[string]any{}
	for _, srv := range normalizeMCPServers(cfg.MCP.Servers) {
		if strings.TrimSpace(srv.URL) != "" {
			out.Actions = append(out.Actions, MCPSyncAction{
				Op:      "skip",
				Name:    srv.Name,
				Details: "claude does not support streamable_http in ~/.claude.json (configure manually)",
			})
			continue
		}
		if len(srv.Command) == 0 {
			continue
		}
		env := map[string]any{}
		keys := make([]string, 0, len(srv.Env))
		for k := range srv.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			env[k] = srv.Env[k]
		}
		desired[srv.Name] = map[string]any{
			"type":    "stdio",
			"command": srv.Command[0],
			"args":    srv.Command[1:],
			"env":     env,
		}
	}

	if len(desired) == 0 && cfg.MCP.Prune {
		// Explicit prune with empty desired: clear managed set.
	}

	if opts.DryRun {
		out.Action = "skipped"
		return out
	}

	data, err := loadJSONMap(claudeConfigPath)
	if err != nil {
		out.Action = "error"
		out.Error = err.Error()
		return out
	}

	if cfg.MCP.Prune {
		data["mcpServers"] = desired
	} else {
		current, _ := data["mcpServers"].(map[string]any)
		if current == nil {
			current = map[string]any{}
		}
		for k, v := range desired {
			current[k] = v
		}
		data["mcpServers"] = current
	}

	if err := writeAtomicJSONFile(claudeConfigPath, data, 0600); err != nil {
		out.Action = "error"
		out.Error = err.Error()
		return out
	}

	out.Action = "synced"
	return out
}

type mirrorOptions struct {
	Prune bool
}

type mirrorReport struct {
	CreatedDirs  int
	CopiedFiles  int
	UpdatedFiles int
	RemovedPaths int
}

func mirrorDir(src string, dst string, opts mirrorOptions) (mirrorReport, error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	srcEntries := map[string]fs.FileMode{}
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		srcEntries[rel] = info.Mode()
		return nil
	})
	if err != nil {
		return mirrorReport{}, err
	}

	report := mirrorReport{}

	// First pass: ensure directories exist.
	for rel, mode := range srcEntries {
		if mode.IsDir() {
			if err := os.MkdirAll(filepath.Join(dst, rel), 0755); err != nil {
				return report, err
			}
			report.CreatedDirs++
		}
	}

	// Second pass: copy files and symlinks.
	for rel, mode := range srcEntries {
		srcPath := filepath.Join(src, rel)
		dstPath := filepath.Join(dst, rel)

		if mode.IsDir() {
			continue
		}
		if mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return report, err
			}
			if err := replaceSymlink(dstPath, target); err != nil {
				return report, err
			}
			report.UpdatedFiles++
			continue
		}

		// regular file
		same, err := filesEqual(srcPath, dstPath)
		if err != nil {
			return report, err
		}
		if same {
			continue
		}

		_, statErr := os.Stat(dstPath)
		dstExisted := statErr == nil

		if err := copyFile(srcPath, dstPath); err != nil {
			return report, err
		}
		if dstExisted {
			report.UpdatedFiles++
		} else {
			report.CopiedFiles++
		}
	}

	if !opts.Prune {
		return report, nil
	}

	// Prune: remove any dst entries not in src.
	dstEntries := []string{}
	if err := filepath.WalkDir(dst, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if path == dst {
			return nil
		}
		rel, err := filepath.Rel(dst, path)
		if err != nil {
			return err
		}
		dstEntries = append(dstEntries, rel)
		return nil
	}); err != nil {
		return report, err
	}

	sort.Slice(dstEntries, func(i, j int) bool { return len(dstEntries[i]) > len(dstEntries[j]) })
	for _, rel := range dstEntries {
		if _, ok := srcEntries[rel]; ok {
			continue
		}
		p := filepath.Join(dst, rel)
		if err := os.RemoveAll(p); err != nil {
			return report, err
		}
		report.RemovedPaths++
	}

	return report, nil
}

func replaceSymlink(path string, target string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return os.Symlink(target, path)
}

func filesEqual(a string, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if ai.Size() != bi.Size() {
		return false, nil
	}
	ab, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ab, bb), nil
}

func copyFile(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".agx-sync-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	return nil
}

func symlinkDrift(linkPath string, targetPath string) (bool, error) {
	st, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return true, err
	}
	if st.Mode()&os.ModeSymlink == 0 {
		return true, nil
	}
	got, err := os.Readlink(linkPath)
	if err != nil {
		return true, err
	}
	return filepath.Clean(got) != filepath.Clean(targetPath), nil
}

func ensureSymlink(linkPath string, targetPath string, now func() time.Time) (action string, backupPath string, err error) {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return "error", "", err
	}

	st, lerr := os.Lstat(linkPath)
	if lerr != nil {
		if os.IsNotExist(lerr) {
			if err := os.Symlink(targetPath, linkPath); err != nil {
				return "error", "", err
			}
			return "linked", "", nil
		}
		return "error", "", lerr
	}

	if st.Mode()&os.ModeSymlink != 0 {
		got, err := os.Readlink(linkPath)
		if err != nil {
			return "error", "", err
		}
		if filepath.Clean(got) == filepath.Clean(targetPath) {
			return "noop", "", nil
		}
		if err := os.Remove(linkPath); err != nil {
			return "error", "", err
		}
		if err := os.Symlink(targetPath, linkPath); err != nil {
			return "error", "", err
		}
		return "relinked", "", nil
	}

	backupPath = linkPath + ".bak." + now().UTC().Format("20060102T150405Z")
	if err := os.Rename(linkPath, backupPath); err != nil {
		return "error", "", err
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		return "error", backupPath, err
	}
	return "backed_up_and_linked", backupPath, nil
}

func loadJSONMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse json %s: %w", path, err)
	}
	return payload, nil
}

func writeAtomicJSONFile(path string, payload map[string]any, perm os.FileMode) error {
	data, err := marshalSortedJSON(payload)
	if err != nil {
		return err
	}
	return writeAtomicFile(path, data, perm)
}

// marshalSortedJSON marshals maps with stable key ordering for diff-friendly output.
func marshalSortedJSON(payload any) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeAtomicFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".agx-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

func (s *EnvSyncService) validate() error {
	if s.paths.HomeDir == "" {
		return errors.New("missing HomeDir in config.Paths")
	}
	return nil
}
