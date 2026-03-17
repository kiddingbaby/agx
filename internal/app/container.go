package app

import "github.com/kiddingbaby/agx/internal/usecase"

// Container wires application-level use cases.
type Container struct {
	KeyService      *usecase.KeyService
	ProviderService *usecase.ProviderService
	SwitchService   *usecase.SwitchService
	EnvSyncService  *usecase.EnvSyncService
}
