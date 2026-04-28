package ports

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type StateRepository interface {
	Load() (domainprofile.State, error)
	Save(state domainprofile.State) (domainprofile.State, error)
}
