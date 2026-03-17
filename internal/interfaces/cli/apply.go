package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiddingbaby/agx/internal/config"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/usecase"
	"gopkg.in/yaml.v3"
)

type applyBundle struct {
	Keys     []applyKey     `yaml:"keys"`
	Profiles []applyProfile `yaml:"profiles,omitempty"`
	Targets  []applyTarget  `yaml:"targets,omitempty"`
	Bindings []applyBinding `yaml:"bindings,omitempty"`
}

type applyKey struct {
	Provider string    `yaml:"provider"`
	Profile  string    `yaml:"profile,omitempty"`
	Name     string    `yaml:"name"`
	Key      string    `yaml:"key,omitempty"`
	KeyEnv   string    `yaml:"key-env,omitempty"`
	KeyFile  string    `yaml:"key-file,omitempty"`
	BaseURL  *string   `yaml:"base-url,omitempty"`
	Tags     *[]string `yaml:"tags,omitempty"`
	Activate *bool     `yaml:"activate,omitempty"`
}

type applyProfile struct {
	Provider string `yaml:"provider"`
	Profile  string `yaml:"profile,omitempty"`
	Strategy string `yaml:"strategy"`
	FixedKey string `yaml:"fixed-key,omitempty"`
}

type applyTarget struct {
	Name               string            `yaml:"name"`
	Family             string            `yaml:"family"`
	Kind               string            `yaml:"kind"`
	Access             string            `yaml:"access"`
	Auth               string            `yaml:"auth,omitempty"`
	BaseURL            string            `yaml:"base-url,omitempty"`
	Model              string            `yaml:"model,omitempty"`
	Env                map[string]string `yaml:"env,omitempty"`
	WireAPI            string            `yaml:"wire-api,omitempty"`
	RequiresOpenAIAuth *bool             `yaml:"requires-openai-auth,omitempty"`
}

type applyBinding struct {
	Family string `yaml:"family"`
	Target string `yaml:"target"`
}

type applyReport struct {
	Keys struct {
		Created []keyView `json:"created,omitempty"`
		Updated []keyView `json:"updated,omitempty"`
	} `json:"keys,omitempty"`
	Targets struct {
		Upserted []targetView `json:"upserted,omitempty"`
	} `json:"targets,omitempty"`
	Bindings []bindingView `json:"bindings,omitempty"`
	Profiles []profileView `json:"profiles,omitempty"`
}

func (r *Root) handleApply(args []string) int {
	if r.keySvc == nil || r.providerSvc == nil {
		fmt.Fprintln(r.stderr, "Error: apply requires key/config services")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx apply [PATH|DIR] [-o json] | agx apply --stdin [-o json] | agx apply --paste [-o json]")
		return 0
	}

	var (
		filePath  string
		fromStdin bool
		paste     bool
		asJSON    bool
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--stdin":
			fromStdin = true
		case "--paste":
			paste = true
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
				fmt.Fprintln(r.stderr, "Usage: agx apply [PATH|DIR] [-o json] | agx apply --stdin [-o json] | agx apply --paste [-o json]")
				return 1
			}
			if strings.TrimSpace(filePath) != "" || fromStdin || paste {
				fmt.Fprintln(r.stderr, "Usage: agx apply [PATH|DIR] [-o json] | agx apply --stdin [-o json] | agx apply --paste [-o json]")
				return 1
			}
			filePath = args[i]
		}
	}

	sources := 0
	if strings.TrimSpace(filePath) != "" {
		sources++
	}
	if fromStdin {
		sources++
	}
	if paste {
		sources++
	}
	if sources > 1 {
		fmt.Fprintln(r.stderr, "Usage: agx apply [PATH|DIR] [-o json] | agx apply --stdin [-o json] | agx apply --paste [-o json]")
		return 1
	}

	if sources == 0 {
		if !stdinIsCharDevice() {
			fromStdin = true
		} else if path, ok := findDefaultConfigPath(r.getwd); ok {
			filePath = path
		} else if stderrIsCharDevice() {
			paste = true
		} else {
			fmt.Fprintln(r.stderr, "Usage: agx apply [PATH|DIR] [-o json] | agx apply --stdin [-o json] | agx apply --paste [-o json]")
			return 1
		}
	}

	var (
		data []byte
		err  error
	)
	if fromStdin || paste {
		if fromStdin {
			info, statErr := os.Stdin.Stat()
			if statErr == nil && (info.Mode()&os.ModeCharDevice) != 0 {
				fmt.Fprintln(r.stderr, "Error: --stdin requires piped stdin")
				return 1
			}
		}
		if paste {
			fmt.Fprintln(r.stderr, "Paste config YAML (agx.yml), then Ctrl-D:")
		}
		data, err = io.ReadAll(os.Stdin)
	} else {
		resolved, resolveErr := resolveConfigPath(filePath)
		if resolveErr != nil {
			fmt.Fprintf(r.stderr, "Error: failed to resolve config path: %v\n", resolveErr)
			return 1
		}
		filePath = resolved
		data, err = os.ReadFile(filePath)
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: failed to read config: %v\n", err)
		return 1
	}

	var bundle applyBundle
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		fmt.Fprintf(r.stderr, "Error: failed to parse config: %v\n", err)
		return 1
	}

	var report applyReport

	for _, t := range bundle.Targets {
		target, err := r.providerSvc.SaveTarget(toSaveTargetInput(t))
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		report.Targets.Upserted = append(report.Targets.Upserted, toTargetView(*target, false))
	}

	for _, b := range bundle.Bindings {
		binding, err := r.providerSvc.UseTarget(b.Family, b.Target)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		report.Bindings = append(report.Bindings, toBindingView(*binding))
	}

	for _, k := range bundle.Keys {
		created, updated, view, err := r.applyKey(k)
		if err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		if created {
			report.Keys.Created = append(report.Keys.Created, view)
		}
		if updated {
			report.Keys.Updated = append(report.Keys.Updated, view)
		}
	}

	for _, p := range bundle.Profiles {
		if err := r.applyProfile(p); err != nil {
			fmt.Fprintf(r.stderr, "Error: %v\n", err)
			return 1
		}
		report.Profiles = append(report.Profiles, profileView{
			Provider:  domainkey.Provider(strings.TrimSpace(strings.ToLower(p.Provider))),
			Name:      domainkey.NormalizeProfileName(p.Profile),
			Strategy:  domainkey.RotationStrategy(strings.TrimSpace(strings.ToLower(p.Strategy))),
			FixedKey:  strings.TrimSpace(p.FixedKey),
			NextIndex: 0,
		})
	}

	if asJSON {
		if err := json.NewEncoder(r.stdout).Encode(report); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "Applied: keys(created=%d updated=%d) targets(upserted=%d) bindings=%d profiles=%d\n",
		len(report.Keys.Created), len(report.Keys.Updated), len(report.Targets.Upserted), len(report.Bindings), len(report.Profiles))
	return 0
}

func (r *Root) applyProfile(p applyProfile) error {
	provider, ok := domainkey.ParseProvider(p.Provider)
	if !ok {
		return fmt.Errorf("invalid provider %q", p.Provider)
	}
	strategy, ok := domainkey.ParseRotationStrategy(p.Strategy)
	if !ok {
		return fmt.Errorf("invalid strategy %q", p.Strategy)
	}
	return r.keySvc.SetProfileStrategy(provider, p.Profile, strategy, p.FixedKey)
}

func toSaveTargetInput(t applyTarget) usecase.SaveTargetInput {
	return usecase.SaveTargetInput{
		Name:               t.Name,
		Family:             t.Family,
		Kind:               t.Kind,
		Access:             t.Access,
		Auth:               t.Auth,
		BaseURL:            t.BaseURL,
		Model:              t.Model,
		Env:                t.Env,
		WireAPI:            t.WireAPI,
		RequiresOpenAIAuth: t.RequiresOpenAIAuth,
	}
}

func (r *Root) applyKey(in applyKey) (created bool, updated bool, view keyView, err error) {
	if strings.TrimSpace(in.Provider) == "" || strings.TrimSpace(in.Name) == "" {
		return false, false, keyView{}, errors.New("key requires provider and name")
	}
	provider, ok := domainkey.ParseProvider(in.Provider)
	if !ok {
		return false, false, keyView{}, fmt.Errorf("invalid provider %q", in.Provider)
	}
	profile := domainkey.NormalizeProfileName(in.Profile)

	apiKey, keySet, err := resolveApplyKeySecret(in)
	if err != nil {
		return false, false, keyView{}, err
	}

	existing, findErr := r.keySvc.FindByIdentifierInScope(provider, profile, in.Name)
	if findErr == nil && existing != nil {
		baseURL := existing.BaseURL
		if in.BaseURL != nil {
			baseURL = strings.TrimSpace(*in.BaseURL)
		}
		tags := existing.Tags
		if in.Tags != nil {
			tags = normalizeTags(*in.Tags)
		}
		apiKeyForUpdate := ""
		if keySet {
			apiKeyForUpdate = apiKey
		}
		out, err := r.keySvc.Update(existing.ID, provider, profile, existing.Name, apiKeyForUpdate, baseURL, tags)
		if err != nil {
			return false, false, keyView{}, err
		}
		if in.Activate != nil && *in.Activate {
			if err := r.keySvc.Activate(out.ID); err != nil {
				return false, false, keyView{}, err
			}
		}
		updatedKey, _ := r.keySvc.FindByIdentifier(out.ID)
		if updatedKey != nil {
			updatedKey.Profile = domainkey.NormalizeProfileName(updatedKey.Profile)
			return false, true, toKeyView(*updatedKey, updatedKey.Profile), nil
		}
		out.Profile = domainkey.NormalizeProfileName(out.Profile)
		return false, true, toKeyView(*out, out.Profile), nil
	}

	if !keySet {
		return false, false, keyView{}, fmt.Errorf("missing key material for %s/%s %s (use key/key-env/key-file)", provider, profile, in.Name)
	}

	baseURL := ""
	if in.BaseURL != nil {
		baseURL = strings.TrimSpace(*in.BaseURL)
	}
	var tags []string
	if in.Tags != nil {
		tags = normalizeTags(*in.Tags)
	}
	out, err := r.keySvc.Add(provider, profile, in.Name, apiKey, baseURL, tags)
	if err != nil {
		return false, false, keyView{}, err
	}
	if in.Activate != nil && *in.Activate {
		if err := r.keySvc.Activate(out.ID); err != nil {
			return false, false, keyView{}, err
		}
	}
	createdKey, _ := r.keySvc.FindByIdentifier(out.ID)
	if createdKey != nil {
		createdKey.Profile = domainkey.NormalizeProfileName(createdKey.Profile)
		return true, false, toKeyView(*createdKey, createdKey.Profile), nil
	}
	out.Profile = domainkey.NormalizeProfileName(out.Profile)
	return true, false, toKeyView(*out, out.Profile), nil
}

func resolveApplyKeySecret(in applyKey) (value string, set bool, err error) {
	keyRaw := strings.TrimSpace(in.Key)
	envRaw := strings.TrimSpace(in.KeyEnv)
	fileRaw := strings.TrimSpace(in.KeyFile)

	sources := 0
	if keyRaw != "" {
		sources++
	}
	if envRaw != "" {
		sources++
	}
	if fileRaw != "" {
		sources++
	}
	if sources == 0 {
		return "", false, nil
	}
	if sources > 1 {
		return "", false, fmt.Errorf("multiple key sources for %s: choose one of key/key-env/key-file", in.Name)
	}

	if keyRaw != "" {
		if strings.HasPrefix(keyRaw, "env:") {
			name := strings.TrimSpace(strings.TrimPrefix(keyRaw, "env:"))
			v := strings.TrimSpace(os.Getenv(name))
			if v == "" {
				return "", false, fmt.Errorf("env %s is empty", name)
			}
			return v, true, nil
		}
		if strings.HasPrefix(keyRaw, "file:") {
			path := strings.TrimSpace(strings.TrimPrefix(keyRaw, "file:"))
			data, err := os.ReadFile(path)
			if err != nil {
				return "", false, err
			}
			v := strings.TrimSpace(string(data))
			if v == "" {
				return "", false, fmt.Errorf("key file is empty: %s", path)
			}
			return v, true, nil
		}
		return keyRaw, true, nil
	}

	if envRaw != "" {
		v := strings.TrimSpace(os.Getenv(envRaw))
		if v == "" {
			return "", false, fmt.Errorf("env %s is empty", envRaw)
		}
		return v, true, nil
	}

	data, err := os.ReadFile(fileRaw)
	if err != nil {
		return "", false, err
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "", false, fmt.Errorf("key file is empty: %s", fileRaw)
	}
	return v, true, nil
}

func normalizeTags(in []string) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stdinIsCharDevice() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func stderrIsCharDevice() bool {
	info, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func findDefaultConfigPath(getwd func() (string, error)) (string, bool) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return "", false
	}

	candidates := []string{
		filepath.Join(paths.ConfigDir, "agx.yml"),
		filepath.Join(paths.ConfigDir, "agx.yaml"),
	}

	for _, path := range candidates {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if fi.Mode().IsRegular() {
			return path, true
		}
	}
	return "", false
}

func resolveConfigPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty config path")
	}

	fi, err := os.Stat(path)
	if err != nil {
		// Keep the original path so the caller can surface the underlying read/stat error.
		return path, nil
	}
	if !fi.IsDir() {
		return path, nil
	}

	candidates := []string{
		filepath.Join(path, "agx.yml"),
		filepath.Join(path, "agx.yaml"),
	}
	for _, candidate := range candidates {
		fi, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if fi.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no agx.yml found in directory: %s (expected agx.yml|agx.yaml)", path)
}
