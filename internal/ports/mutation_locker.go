package ports

type MutationLocker interface {
	Lock() (func(), error)
}
