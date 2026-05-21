package usecase

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func ensureManagedAgents(items map[domainprofile.Agent]domainprofile.ManagedAgentState) map[domainprofile.Agent]domainprofile.ManagedAgentState {
	if items == nil {
		return map[domainprofile.Agent]domainprofile.ManagedAgentState{}
	}
	return items
}

func ensureTargets(items map[string]domainprofile.TargetState) map[string]domainprofile.TargetState {
	if items == nil {
		return map[string]domainprofile.TargetState{}
	}
	return items
}

func managedAgentState(state domainprofile.State, agent domainprofile.Agent) domainprofile.ManagedAgentState {
	if len(state.ManagedAgents) == 0 {
		return domainprofile.ManagedAgentState{}
	}
	managed, ok := state.ManagedAgents[agent]
	if !ok {
		return domainprofile.ManagedAgentState{}
	}
	managed.Targets = ensureTargets(managed.Targets)
	return managed
}

func assignManagedAgentState(state *domainprofile.State, agent domainprofile.Agent, managed domainprofile.ManagedAgentState) {
	if state == nil {
		return
	}
	state.ManagedAgents = ensureManagedAgents(state.ManagedAgents)
	managed.Targets = ensureTargets(managed.Targets)
	state.ManagedAgents[agent] = managed
}

func normalizeManagedAgentRegistries(state *domainprofile.State) {
	if state == nil {
		return
	}
	state.ManagedAgents = ensureManagedAgents(state.ManagedAgents)
	for agent, managed := range state.ManagedAgents {
		managed.Targets = ensureTargets(managed.Targets)
		state.ManagedAgents[agent] = managed
	}
}

func (s *ProfileService) managedTargetContextRoot(agent domainprofile.Agent, name string) string {
	return filepath.Join(s.managedPaths.ContextsDir, string(agent), "targets", normalizeTargetName(name))
}

func (s *ProfileService) managedBackupsRoot(agent domainprofile.Agent, kind domainprofile.TargetKind, name string) string {
	base := filepath.Join(s.managedPaths.BackupsDir, "managed", string(agent), string(kind))
	if name == "" {
		return base
	}
	return filepath.Join(base, normalizeTargetName(name))
}

func (s *ProfileService) ensureManagedContextRoot(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("managed context path is required")
	}
	return os.MkdirAll(path, 0o700)
}

func managedContextConfigPath(agent domainprofile.Agent, root string) string {
	switch agent {
	case domainprofile.AgentCodex:
		return filepath.Join(root, "config.toml")
	case domainprofile.AgentClaude:
		return filepath.Join(root, "settings.json")
	case domainprofile.AgentGemini:
		return filepath.Join(root, ".gemini", "settings.json")
	case domainprofile.AgentOpenCode:
		return filepath.Join(root, "xdg", "opencode", "opencode.json")
	default:
		return ""
	}
}

func (s *ProfileService) managedCodexSyncer(root string) ports.CodexSyncer {
	if s.managedSyncers.NewCodex == nil {
		return nil
	}
	return s.managedSyncers.NewCodex(
		managedContextConfigPath(domainprofile.AgentCodex, root),
		s.managedBackupsRoot(domainprofile.AgentCodex, domainprofile.TargetKindRelay, ""),
		s.managedPaths.HelperCommand,
	)
}

func (s *ProfileService) managedClaudeSyncer(root string) ports.ClaudeSyncer {
	if s.managedSyncers.NewClaude == nil {
		return nil
	}
	return s.managedSyncers.NewClaude(
		managedContextConfigPath(domainprofile.AgentClaude, root),
		s.managedBackupsRoot(domainprofile.AgentClaude, domainprofile.TargetKindRelay, ""),
		s.managedPaths.HelperCommand,
	)
}

func (s *ProfileService) managedGeminiSyncer(root string) ports.GeminiSyncer {
	if s.managedSyncers.NewGemini == nil {
		return nil
	}
	return s.managedSyncers.NewGemini(
		managedContextConfigPath(domainprofile.AgentGemini, root),
		s.managedBackupsRoot(domainprofile.AgentGemini, domainprofile.TargetKindRelay, ""),
	)
}

func (s *ProfileService) managedOpenCodeSyncer(root string) ports.OpenCodeSyncer {
	if s.managedSyncers.NewOpenCode == nil {
		return nil
	}
	return s.managedSyncers.NewOpenCode(
		managedContextConfigPath(domainprofile.AgentOpenCode, root),
		s.managedBackupsRoot(domainprofile.AgentOpenCode, domainprofile.TargetKindRelay, ""),
	)
}

func normalizeTargetName(name string) string {
	return domainprofile.NormalizeTargetName(name)
}

func validateTargetName(name string) error {
	return domainprofile.ValidateTargetName(name)
}

func targetProfileName(agent domainprofile.Agent, name string) string {
	name = normalizeTargetName(name)
	if name == "" {
		return ""
	}
	return string(agent) + "." + name
}

func cloneContextBackups(items []domainprofile.ContextBackup) []domainprofile.ContextBackup {
	return append([]domainprofile.ContextBackup(nil), items...)
}

func prependContextBackup(backups *[]domainprofile.ContextBackup, backup domainprofile.ContextBackup) error {
	*backups = append([]domainprofile.ContextBackup{backup}, (*backups)...)
	if len(*backups) <= backupHistoryLimit {
		return nil
	}
	// Drop the trimmed directories from disk first, then commit the in-memory
	// slice mutation. An earlier version sliced the metadata before any
	// RemoveAll ran, so a mid-loop failure left orphan directories on disk
	// with no record in state.
	trimmed := append([]domainprofile.ContextBackup(nil), (*backups)[backupHistoryLimit:]...)
	var removalErrs []error
	for _, item := range trimmed {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		if err := os.RemoveAll(item.Path); err != nil {
			removalErrs = append(removalErrs, err)
		}
	}
	if len(removalErrs) > 0 {
		return errors.Join(removalErrs...)
	}
	*backups = (*backups)[:backupHistoryLimit]
	return nil
}

func copyDir(dst, src string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("copy dir source is not a directory: %s", src)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		// Re-create symlinks instead of dereferencing them. Earlier the walk
		// fell through to copyFile for symlinks, materializing the link
		// target's content as a regular file in the backup — destroying
		// link semantics and potentially leaking secrets the link pointed
		// at outside the context root.
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			//nolint:gosec // G122: symlink is recreated literally (linkTarget
			// comes from os.Readlink on the source); we never follow it, so
			// there is no TOCTOU traversal risk here.
			return os.Symlink(linkTarget, target)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(target, path, info.Mode().Perm())
	})
}

func copyFile(dst, src string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// replaceDirContents swaps dst with a copy of src in a way that preserves the
// previous dst contents until the copy fully succeeds. Earlier the routine
// did RemoveAll(dst) followed by copyDir(dst, src); if copyDir failed mid-way
// the original dst was already gone and the user lost both the current
// context and the backup.
func replaceDirContents(dst, src string) error {
	// If dst doesn't exist yet, fall straight through to a plain copy.
	if _, err := os.Lstat(dst); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return copyDir(dst, src)
	}

	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	backup, err := os.MkdirTemp(parent, ".agx-restore-")
	if err != nil {
		return err
	}
	// MkdirTemp creates the directory; remove it so Rename can move dst into
	// the same slot.
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(dst, backup); err != nil {
		return err
	}
	if err := copyDir(dst, src); err != nil {
		// Restore the previous contents on failure.
		_ = os.RemoveAll(dst)
		_ = os.Rename(backup, dst)
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}

func nextContextBackupID(backups []domainprofile.ContextBackup, now time.Time) string {
	candidate := now.UTC().Format("20060102T150405Z")
	if !hasContextBackupID(backups, candidate) {
		return candidate
	}
	for i := 1; i < 1000; i++ {
		next := fmt.Sprintf("%s-%03d", candidate, i)
		if !hasContextBackupID(backups, next) {
			return next
		}
	}
	return fmt.Sprintf("%s-%d", candidate, now.UTC().UnixNano())
}

func hasContextBackupID(backups []domainprofile.ContextBackup, id string) bool {
	for _, item := range backups {
		if item.ID == id {
			return true
		}
	}
	return false
}

func selectContextBackup(backups []domainprofile.ContextBackup, targetName, backupID string) (domainprofile.ContextBackup, error) {
	filtered := make([]domainprofile.ContextBackup, 0, len(backups))
	for _, backup := range backups {
		if targetName == "" || backup.TargetName == targetName {
			filtered = append(filtered, backup)
		}
	}
	if len(filtered) == 0 {
		return domainprofile.ContextBackup{}, &BackupNotFoundError{ID: backupID}
	}
	if strings.TrimSpace(backupID) == "" {
		return filtered[0], nil
	}
	for _, backup := range filtered {
		if backup.ID == backupID {
			return backup, nil
		}
	}
	return domainprofile.ContextBackup{}, &BackupNotFoundError{ID: backupID}
}
