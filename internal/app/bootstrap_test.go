package app

import "testing"

func TestBootstrap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	container, err := Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if container == nil || container.ProfileService == nil {
		t.Fatalf("container = %#v, want ProfileService", container)
	}
}
