package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveHelperCommand returns the most stable command form for AGX self-invocation.
// It prefers an installed `agx` command when available, otherwise falls back to the
// current executable path as long as that path is not a Go build-cache artifact.
func ResolveHelperCommand() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve agx executable: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		exePath = filepath.Clean(exePath)
	}

	if pathEntry, ok := resolvePathCommand("agx"); ok && !isEphemeralExecutable(pathEntry) {
		if pathEntry == exePath {
			return "agx", nil
		}
		if isEphemeralExecutable(exePath) {
			return "agx", nil
		}
	}

	if isEphemeralExecutable(exePath) {
		return "", nil
	}

	return exePath, nil
}

func resolvePathCommand(name string) (string, bool) {
	pathEntry, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	pathEntry, pathErr := filepath.EvalSymlinks(pathEntry)
	if pathErr != nil {
		pathEntry = filepath.Clean(pathEntry)
	}
	return pathEntry, true
}

func isEphemeralExecutable(path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.Contains(path, "/go-build/")
}
