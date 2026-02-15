package app

import "github.com/kiddingbaby/agx/internal/usecase"

// Container wires application-level use cases.
type Container struct {
	KeyService     *usecase.KeyService
	SessionService *usecase.SessionService
	LaunchService  *usecase.LaunchService
}
