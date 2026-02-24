package ports

// SecretProvider provides master secret bytes for key encryption.
type SecretProvider interface {
	Load() ([]byte, error)
}
