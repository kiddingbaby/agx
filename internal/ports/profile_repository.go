package ports

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type ProfileRepository interface {
	List() ([]domainprofile.Profile, error)
	Get(name string) (*domainprofile.Profile, error)
	Upsert(profile domainprofile.Profile) (*domainprofile.Profile, error)
	Delete(name string) error
}
