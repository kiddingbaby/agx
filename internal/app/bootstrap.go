package app

import (
	"fmt"

	"github.com/kiddingbaby/agx/internal/adapters/claudeconfig"
	"github.com/kiddingbaby/agx/internal/adapters/codexconfig"
	"github.com/kiddingbaby/agx/internal/adapters/geminiconfig"
	"github.com/kiddingbaby/agx/internal/adapters/lockfile"
	"github.com/kiddingbaby/agx/internal/adapters/opjournal"
	"github.com/kiddingbaby/agx/internal/adapters/profilefile"
	"github.com/kiddingbaby/agx/internal/config"
	"github.com/kiddingbaby/agx/internal/usecase"
)

// Bootstrap runs startup preparation, initializes runtime dependencies, and returns the app container.
func Bootstrap() (*Container, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	startupLock := lockfile.New(paths.LockPath)

	helperCommand, err := config.ResolveHelperCommand()
	if err != nil {
		return nil, err
	}

	profiles, err := profilefile.NewRepository(paths.ProfilesDir)
	if err != nil {
		return nil, fmt.Errorf("initialize profile store: %w", err)
	}

	state := profilefile.NewStateRepository(paths.StatePath)
	codexSyncer := codexconfig.NewSyncer(paths.CodexConfigPath, paths.BackupsDir, helperCommand)
	claudeSyncer := claudeconfig.NewSyncer(paths.ClaudeSettingsPath, paths.BackupsDir, helperCommand)
	geminiSyncer := geminiconfig.NewSyncer(paths.GeminiEnvPath, paths.BackupsDir)
	profileSvc := usecase.NewProfileService(profiles, state, codexSyncer, claudeSyncer, geminiSyncer)
	profileSvc.SetMutationLocker(startupLock)
	profileSvc.SetOperationJournal(opjournal.New(paths.OperationPath))

	return &Container{
		ProfileService: profileSvc,
	}, nil
}
