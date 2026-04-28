package app

import "github.com/kiddingbaby/agx/internal/usecase"

// Container wires application-level use cases.
type Container struct {
	ProfileService *usecase.ProfileService
}
