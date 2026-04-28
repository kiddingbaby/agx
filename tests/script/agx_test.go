//go:build contract

package script

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestAGXContracts(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	root = filepath.Clean(filepath.Join(root, "..", ".."))

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata",
		RequireExplicitExec: true,
		RequireUniqueNames:  true,
		Setup: func(env *testscript.Env) error {
			home := filepath.Join(env.WorkDir, "home")
			if err := os.MkdirAll(home, 0o700); err != nil {
				return err
			}
			env.Setenv("HOME", home)
			env.Setenv("AGX_REPO_ROOT", root)
			env.Setenv("AGX_BIN", agxBinaryPath())
			return nil
		},
	})
}

func agxBinaryPath() string {
	if cacheDir := os.Getenv("AGX_CACHE_DIR"); cacheDir != "" {
		return filepath.Join(cacheDir, "bin", "agx")
	}
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, "agx", "bin", "agx")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("agx")
	}
	return filepath.Join(home, ".cache", "agx", "bin", "agx")
}
