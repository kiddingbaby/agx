package mcpgateway

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	"gopkg.in/yaml.v3"
)

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"

	DefaultListen            = "127.0.0.1:8765"
	DefaultStartupTimeoutSec = 30
)

type Config struct {
	Servers []ServerSpec `yaml:"servers"`
	Gateway GatewaySpec  `yaml:"gateway,omitempty"`
}

type ServerSpec struct {
	Name              string            `yaml:"name"`
	Transport         string            `yaml:"transport,omitempty"`
	Command           string            `yaml:"command,omitempty"`
	Args              []string          `yaml:"args,omitempty"`
	Env               map[string]string `yaml:"env,omitempty"`
	EnvPassthrough    []string          `yaml:"env_passthrough,omitempty"`
	URL               string            `yaml:"url,omitempty"`
	Headers           map[string]string `yaml:"headers,omitempty"`
	Enabled           *bool             `yaml:"enabled,omitempty"`
	StartupTimeoutSec int               `yaml:"startup_timeout_sec,omitempty"`
}

type GatewaySpec struct {
	Listen string    `yaml:"listen,omitempty"`
	Auth   AuthSpec  `yaml:"auth,omitempty"`
	Audit  AuditSpec `yaml:"audit,omitempty"`
}

type AuthSpec struct {
	BearerTokenEnv string `yaml:"bearer_token_env,omitempty"`
}

type AuditSpec struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	Path    string `yaml:"path,omitempty"`
}

func (s ServerSpec) IsEnabled() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "agx", "mcp", "servers.yaml"), nil
}

func DefaultAuditPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "agx", "mcp", "audit.log"), nil
}

func LoadConfig(path string) (*Config, error) {
	body, exists, err := fileutil.ReadIfExists(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if !exists {
		return cfg, nil
	}
	if err := yaml.Unmarshal([]byte(body), cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	body, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(path, body, 0o600)
}

func (c *Config) Normalize() {
	if c == nil {
		return
	}
	for i := range c.Servers {
		c.Servers[i].normalize()
	}
	if strings.TrimSpace(c.Gateway.Listen) == "" {
		c.Gateway.Listen = DefaultListen
	}
}

func (s *ServerSpec) normalize() {
	s.Name = strings.TrimSpace(s.Name)
	s.Transport = strings.ToLower(strings.TrimSpace(s.Transport))
	if s.Transport == "" {
		switch {
		case strings.TrimSpace(s.URL) != "":
			s.Transport = TransportHTTP
		case strings.TrimSpace(s.Command) != "":
			s.Transport = TransportStdio
		}
	}
	if s.StartupTimeoutSec <= 0 {
		s.StartupTimeoutSec = DefaultStartupTimeoutSec
	}
}

func (c *Config) Validate() error {
	seen := map[string]struct{}{}
	for _, s := range c.Servers {
		if s.Name == "" {
			return fmt.Errorf("mcp: server name required")
		}
		if _, dup := seen[s.Name]; dup {
			return fmt.Errorf("mcp: duplicate server name %q", s.Name)
		}
		seen[s.Name] = struct{}{}
		switch s.Transport {
		case TransportStdio:
			if strings.TrimSpace(s.Command) == "" {
				return fmt.Errorf("mcp: server %q stdio transport requires command", s.Name)
			}
		case TransportHTTP:
			if strings.TrimSpace(s.URL) == "" {
				return fmt.Errorf("mcp: server %q http transport requires url", s.Name)
			}
		default:
			return fmt.Errorf("mcp: server %q has unknown transport %q (want stdio|http)", s.Name, s.Transport)
		}
	}
	return nil
}

// UpsertServer replaces a server by name, or appends if not present.
// Returns true when a previous entry was replaced.
func (c *Config) UpsertServer(spec ServerSpec) bool {
	spec.normalize()
	for i, s := range c.Servers {
		if s.Name == spec.Name {
			c.Servers[i] = spec
			return true
		}
	}
	c.Servers = append(c.Servers, spec)
	return false
}

// RemoveServer removes a server by name. Returns true when found.
func (c *Config) RemoveServer(name string) bool {
	for i, s := range c.Servers {
		if s.Name == name {
			c.Servers = append(c.Servers[:i], c.Servers[i+1:]...)
			return true
		}
	}
	return false
}

// SetEnabled flips the enabled flag of a named server. Returns false when not found.
func (c *Config) SetEnabled(name string, enabled bool) bool {
	for i, s := range c.Servers {
		if s.Name == name {
			v := enabled
			c.Servers[i].Enabled = &v
			return true
		}
	}
	return false
}

// FindServer looks up by name.
func (c *Config) FindServer(name string) (ServerSpec, bool) {
	for _, s := range c.Servers {
		if s.Name == name {
			return s, true
		}
	}
	return ServerSpec{}, false
}
