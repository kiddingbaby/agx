package app

import (
	"fmt"

	"github.com/kiddingbaby/agx/internal/adapters/configfile"
	"github.com/kiddingbaby/agx/internal/adapters/executil"
	"github.com/kiddingbaby/agx/internal/adapters/keyfile"
	"github.com/kiddingbaby/agx/internal/adapters/toolconfig"
	"github.com/kiddingbaby/agx/internal/adapters/undofile"
	"github.com/kiddingbaby/agx/internal/config"
	"github.com/kiddingbaby/agx/internal/usecase"
)

// Bootstrap initializes runtime dependencies and returns app container.
func Bootstrap() (*Container, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}

	secretProvider := config.NewSecretProvider(paths)
	secret, err := secretProvider.Load()
	if err != nil {
		return nil, err
	}

	store, err := keyfile.NewRepository(paths.StorePath, secret)
	if err != nil {
		return nil, fmt.Errorf("initialize key store: %w", err)
	}
	providerRegistry, err := configfile.NewProviderRegistry(paths.ProviderConfigPath)
	if err != nil {
		return nil, fmt.Errorf("initialize provider config: %w", err)
	}

	keySvc := usecase.NewKeyService(store)
	providerSvc := usecase.NewProviderService(providerRegistry)
	configSyncer := toolconfig.NewSyncer(paths)
	undoStore := undofile.NewStore(paths)
	switchSvc := usecase.NewSwitchService(keySvc, providerSvc, configSyncer, undoStore)
	envSyncSvc := usecase.NewEnvSyncService(paths, executil.NewRunner())

	return &Container{
		KeyService:      keySvc,
		ProviderService: providerSvc,
		SwitchService:   switchSvc,
		EnvSyncService:  envSyncSvc,
	}, nil
}
