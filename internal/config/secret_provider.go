package config

import (
	"crypto/rand"
	"fmt"
	"os"

	"github.com/kiddingbaby/agx/internal/ports"
)

const secretLength = 32

var _ ports.SecretProvider = (*SecretProvider)(nil)

// SecretProvider loads/creates AGX encryption secret from env/file.
type SecretProvider struct {
	paths Paths
}

func NewSecretProvider(paths Paths) *SecretProvider {
	return &SecretProvider{paths: paths}
}

func (p *SecretProvider) Load() ([]byte, error) {
	if secret := os.Getenv("AGX_SECRET"); secret != "" {
		if len(secret) != secretLength {
			return nil, fmt.Errorf("AGX_SECRET must be exactly 32 bytes (got %d)", len(secret))
		}
		return []byte(secret), nil
	}

	if secretBytes, err := os.ReadFile(p.paths.SecretPath); err == nil && len(secretBytes) >= secretLength {
		return secretBytes[:secretLength], nil
	}

	if _, err := os.Stat(p.paths.StorePath); err == nil {
		return nil, fmt.Errorf("found existing keys.yaml but no encryption secret\n" +
			"Migration: echo -n \"$AGX_SECRET\" > ~/.config/agx/secret")
	}

	if err := os.MkdirAll(p.paths.ConfigDir, 0700); err != nil {
		return nil, fmt.Errorf("cannot create config directory: %w", err)
	}

	secret := make([]byte, secretLength)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("cannot generate secret: %w", err)
	}

	if err := os.WriteFile(p.paths.SecretPath, secret, 0600); err != nil {
		return nil, fmt.Errorf("cannot save secret: %w", err)
	}

	return secret, nil
}
