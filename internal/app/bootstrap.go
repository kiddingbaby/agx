package app

import (
	"fmt"

	"github.com/kiddingbaby/agx/internal/adapters/keyfile"
	tmuxruntime "github.com/kiddingbaby/agx/internal/adapters/tmux"
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

	runtime, err := tmuxruntime.NewRuntime()
	if err != nil {
		return nil, err
	}

	keySvc := usecase.NewKeyService(store)
	sessionSvc := usecase.NewSessionService(runtime)
	launchSvc := usecase.NewLaunchService(keySvc, runtime)

	return &Container{
		KeyService:     keySvc,
		SessionService: sessionSvc,
		LaunchService:  launchSvc,
	}, nil
}
