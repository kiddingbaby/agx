package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths contains resolved filesystem paths used by AGX runtime config.
type Paths struct {
	ConfigDir  string
	StorePath  string
	SecretPath string
}

// DefaultPaths resolves ~/.config/agx paths.
func DefaultPaths() (Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("cannot determine home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "agx")
	return Paths{
		ConfigDir:  configDir,
		StorePath:  filepath.Join(configDir, "keys.yaml"),
		SecretPath: filepath.Join(configDir, "secret"),
	}, nil
}
