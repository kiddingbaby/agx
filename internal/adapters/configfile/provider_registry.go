package configfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/ports"
	"gopkg.in/yaml.v3"
)

var _ ports.ProviderConfigRepository = (*ProviderRegistry)(nil)

type providerFileModel struct {
	Targets     []domainprovider.Target  `yaml:"targets,omitempty"`
	Bindings    []domainprovider.Binding `yaml:"bindings,omitempty"`
	CurrentSite string                   `yaml:"current-site,omitempty"`
}

// ProviderRegistry persists custom targets and active family bindings.
type ProviderRegistry struct {
	path     string
	lockPath string

	mu          sync.Mutex
	targets     []domainprovider.Target
	bindings    []domainprovider.Binding
	currentSite string
}

func NewProviderRegistry(path string) (*ProviderRegistry, error) {
	r := &ProviderRegistry{
		path:        path,
		lockPath:    path + ".lock",
		targets:     []domainprovider.Target{},
		bindings:    []domainprovider.Binding{},
		currentSite: "",
	}
	if err := r.loadIfExists(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *ProviderRegistry) ListTargets() []domainprovider.Target {
	targets := make([]domainprovider.Target, 0)
	if err := r.withReadLock(func() error {
		targets = r.mergeTargets()
		return nil
	}); err != nil {
		return r.mergeTargets()
	}
	return targets
}

func (r *ProviderRegistry) GetTarget(name string) (*domainprovider.Target, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("target name is required")
	}

	var target *domainprovider.Target
	err := r.withReadLock(func() error {
		for _, candidate := range r.mergeTargets() {
			if candidate.Name == name {
				copyTarget := candidate
				target = &copyTarget
				return nil
			}
		}
		return &domainprovider.TargetNotFoundError{Name: name}
	})
	if err != nil {
		return nil, err
	}
	return target, nil
}

func (r *ProviderRegistry) UpsertTarget(target domainprovider.Target) (*domainprovider.Target, error) {
	if err := domainprovider.ValidateTarget(target); err != nil {
		return nil, err
	}
	if builtIn, ok := r.builtInTargetForName(target.Name); ok {
		// Allow overriding built-in official targets for advanced options (model/env/wire-api...),
		// but keep their semantics stable (still official, same family/kind).
		if target.Family != builtIn.Family {
			return nil, fmt.Errorf("built-in target %s belongs to family %s, not %s", target.Name, builtIn.Family, target.Family)
		}
		if target.Access != domainprovider.AccessOfficial {
			return nil, fmt.Errorf("built-in target %s must remain access=%s", target.Name, domainprovider.AccessOfficial)
		}
		if target.Kind != builtIn.Kind {
			return nil, fmt.Errorf("built-in target %s must remain kind=%s", target.Name, builtIn.Kind)
		}
	}

	var out domainprovider.Target
	err := r.withWriteLock(func() error {
		now := time.Now()
		target.CreatedAt = now
		target.UpdatedAt = now
		for i := range r.targets {
			if r.targets[i].Name != target.Name {
				continue
			}
			target.CreatedAt = r.targets[i].CreatedAt
			if target.CreatedAt.IsZero() {
				target.CreatedAt = now
			}
			r.targets[i] = target
			out = target
			return nil
		}
		r.targets = append(r.targets, target)
		out = target
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ProviderRegistry) DeleteTarget(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("target name is required")
	}
	if r.isBuiltInName(name) {
		// Built-in targets cannot be deleted, but their overrides can be removed (reset to defaults).
		return r.withWriteLock(func() error {
			for i := range r.targets {
				if r.targets[i].Name == name {
					r.targets = append(r.targets[:i], r.targets[i+1:]...)
					return nil
				}
			}
			return fmt.Errorf("cannot delete built-in target: %s", name)
		})
	}

	return r.withWriteLock(func() error {
		for _, binding := range r.bindings {
			if binding.Target == name {
				return fmt.Errorf("target is bound by family %s", binding.Family)
			}
		}
		for i := range r.targets {
			if r.targets[i].Name == name {
				r.targets = append(r.targets[:i], r.targets[i+1:]...)
				if r.currentSite == name {
					r.currentSite = ""
				}
				return nil
			}
		}
		return &domainprovider.TargetNotFoundError{Name: name}
	})
}

func (r *ProviderRegistry) ListBindings() []domainprovider.Binding {
	bindings := make([]domainprovider.Binding, 0)
	if err := r.withReadLock(func() error {
		bindings = r.resolvedBindings()
		return nil
	}); err != nil {
		return r.resolvedBindings()
	}
	return bindings
}

func (r *ProviderRegistry) GetBinding(family domainprovider.Family) (*domainprovider.Binding, error) {
	if !family.Valid() {
		return nil, fmt.Errorf("invalid family %q", family)
	}

	var binding *domainprovider.Binding
	err := r.withReadLock(func() error {
		for _, candidate := range r.resolvedBindings() {
			if candidate.Family == family {
				copyBinding := candidate
				binding = &copyBinding
				return nil
			}
		}
		return fmt.Errorf("binding not found for family %s", family)
	})
	if err != nil {
		return nil, err
	}
	return binding, nil
}

func (r *ProviderRegistry) SetBinding(family domainprovider.Family, targetName string) (*domainprovider.Binding, error) {
	if !family.Valid() {
		return nil, fmt.Errorf("invalid family %q", family)
	}
	targetName = strings.TrimSpace(targetName)
	if targetName == "" {
		return nil, errors.New("target name is required")
	}

	var out domainprovider.Binding
	err := r.withWriteLock(func() error {
		target, err := r.lookupTargetUnlocked(targetName)
		if err != nil {
			return err
		}
		if target.Family != family {
			return fmt.Errorf("target %s belongs to family %s, not %s", target.Name, target.Family, family)
		}

		now := time.Now()
		for i := range r.bindings {
			if r.bindings[i].Family != family {
				continue
			}
			r.bindings[i].Target = targetName
			r.bindings[i].UpdatedAt = now
			out = r.bindings[i]
			return nil
		}
		r.bindings = append(r.bindings, domainprovider.Binding{
			Family:    family,
			Target:    targetName,
			UpdatedAt: now,
		})
		out = r.bindings[len(r.bindings)-1]
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ProviderRegistry) GetCurrentSite() string {
	current := ""
	if err := r.withReadLock(func() error {
		current = r.currentSite
		return nil
	}); err != nil {
		return strings.TrimSpace(r.currentSite)
	}
	return strings.TrimSpace(current)
}

func (r *ProviderRegistry) SetCurrentSite(targetName string) error {
	targetName = strings.TrimSpace(targetName)
	if targetName != "" && strings.IndexFunc(targetName, unicode.IsSpace) >= 0 {
		return fmt.Errorf("current-site cannot contain whitespace: %q", targetName)
	}

	return r.withWriteLock(func() error {
		if targetName == "" {
			r.currentSite = ""
			return nil
		}
		if _, err := r.lookupTargetUnlocked(targetName); err != nil {
			return err
		}
		r.currentSite = targetName
		return nil
	})
}

func (r *ProviderRegistry) withReadLock(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()

	if err := r.loadIfExists(); err != nil {
		return err
	}
	return fn()
}

func (r *ProviderRegistry) withWriteLock(fn func() error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	unlock, err := r.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()

	if err := r.loadIfExists(); err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	return r.saveAtomic()
}

func (r *ProviderRegistry) acquireFileLock() (func(), error) {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(r.lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

func (r *ProviderRegistry) mergeTargets() []domainprovider.Target {
	mergedMap := map[string]domainprovider.Target{}
	for _, t := range domainprovider.DefaultTargets() {
		mergedMap[t.Name] = t
	}
	for _, t := range r.targets {
		mergedMap[t.Name] = t
	}

	merged := make([]domainprovider.Target, 0, len(mergedMap))
	for _, t := range mergedMap {
		merged = append(merged, t)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Name < merged[j].Name
	})
	return merged
}

func (r *ProviderRegistry) resolvedBindings() []domainprovider.Binding {
	resolved := make([]domainprovider.Binding, 0, len(domainprovider.SupportedFamilies()))
	now := time.Now()
	for _, family := range domainprovider.SupportedFamilies() {
		binding := domainprovider.Binding{
			Family:    family,
			Target:    domainprovider.DefaultTargetName(family),
			UpdatedAt: now,
		}
		for _, candidate := range r.bindings {
			if candidate.Family == family {
				binding = candidate
				break
			}
		}
		resolved = append(resolved, binding)
	}
	sort.Slice(resolved, func(i, j int) bool {
		return resolved[i].Family < resolved[j].Family
	})
	return resolved
}

func (r *ProviderRegistry) lookupTargetUnlocked(name string) (*domainprovider.Target, error) {
	for _, target := range r.mergeTargets() {
		if target.Name == name {
			copyTarget := target
			return &copyTarget, nil
		}
	}
	return nil, &domainprovider.TargetNotFoundError{Name: name}
}

func (r *ProviderRegistry) isBuiltInName(name string) bool {
	for _, target := range domainprovider.DefaultTargets() {
		if target.Name == name {
			return true
		}
	}
	return false
}

func (r *ProviderRegistry) builtInTargetForName(name string) (domainprovider.Target, bool) {
	for _, target := range domainprovider.DefaultTargets() {
		if target.Name == name {
			return target, true
		}
	}
	return domainprovider.Target{}, false
}

func (r *ProviderRegistry) loadIfExists() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			r.targets = []domainprovider.Target{}
			r.bindings = []domainprovider.Binding{}
			r.currentSite = ""
			return nil
		}
		return err
	}
	var model providerFileModel
	if err := yaml.Unmarshal(data, &model); err != nil {
		return err
	}

	r.targets = model.Targets
	r.bindings = model.Bindings
	r.currentSite = strings.TrimSpace(model.CurrentSite)

	now := time.Now()
	for i := range r.targets {
		r.targets[i].Name = strings.TrimSpace(r.targets[i].Name)
		r.targets[i].BaseURL = strings.TrimSpace(r.targets[i].BaseURL)
		r.targets[i].Model = strings.TrimSpace(r.targets[i].Model)
		r.targets[i].WireAPI = domainprovider.WireAPI(strings.TrimSpace(strings.ToLower(string(r.targets[i].WireAPI))))
		if err := domainprovider.ValidateTarget(r.targets[i]); err != nil {
			name := r.targets[i].Name
			if name == "" {
				name = fmt.Sprintf("targets[%d]", i)
			}
			return fmt.Errorf("invalid target in providers.yaml (%s): %w", name, err)
		}
		if r.targets[i].UpdatedAt.IsZero() {
			r.targets[i].UpdatedAt = now
		}
		if r.targets[i].CreatedAt.IsZero() {
			r.targets[i].CreatedAt = r.targets[i].UpdatedAt
		}
	}

	for i := range r.bindings {
		family, ok := domainprovider.ParseFamily(string(r.bindings[i].Family))
		if !ok {
			return fmt.Errorf("invalid binding family in providers.yaml: %q", r.bindings[i].Family)
		}
		r.bindings[i].Family = family
		r.bindings[i].Target = strings.TrimSpace(r.bindings[i].Target)
		if r.bindings[i].Target == "" {
			return fmt.Errorf("binding target is required for family %s", family)
		}
		if strings.IndexFunc(r.bindings[i].Target, unicode.IsSpace) >= 0 {
			return fmt.Errorf("binding target cannot contain whitespace: %q", r.bindings[i].Target)
		}
		if r.bindings[i].UpdatedAt.IsZero() {
			r.bindings[i].UpdatedAt = now
		}
	}

	if len(r.bindings) > 0 {
		targets := r.mergeTargets()
		byName := make(map[string]domainprovider.Target, len(targets))
		for _, t := range targets {
			byName[t.Name] = t
		}
		for _, b := range r.bindings {
			target, ok := byName[b.Target]
			if !ok {
				return fmt.Errorf("binding for family %s refers to unknown target %s", b.Family, b.Target)
			}
			if target.Family != b.Family {
				return fmt.Errorf("binding for family %s refers to target %s (family %s)", b.Family, b.Target, target.Family)
			}
		}
	}

	if strings.TrimSpace(r.currentSite) != "" {
		if strings.IndexFunc(r.currentSite, unicode.IsSpace) >= 0 {
			r.currentSite = ""
		} else if _, err := r.lookupTargetUnlocked(r.currentSite); err != nil {
			r.currentSite = ""
		}
	}
	return nil
}

func (r *ProviderRegistry) saveAtomic() error {
	model := providerFileModel{
		Targets:     r.targets,
		Bindings:    r.bindings,
		CurrentSite: r.currentSite,
	}
	data, err := yaml.Marshal(model)
	if err != nil {
		return err
	}

	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", r.path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, r.path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
