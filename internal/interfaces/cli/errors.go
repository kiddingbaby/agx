package cli

import (
	"errors"
	"fmt"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
	"github.com/kiddingbaby/agx/internal/usecase"
)

// Exit code conventions (used by reportError and friends in
// cobra_shared_commands.go):
//
//	0  success
//	1  user-recoverable error (missing arg, profile not found,
//	   profile already exists, validation failure, agent error)
//	2  managed-runtime / system not provisioned (T7 typed error)
//	3  doctor reported issues with running state
//	127 unexpected internal error (panic, JSON encode, etc.)
//
// reportError keeps mapping anything it doesn't recognize as exit 1 to
// preserve historical CLI behavior; only the new T7 codes (2, 3) need
// callers to opt in explicitly via reportErrorWithCode.

func (r *Root) printUserError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(r.stderr, "Error: %s\n", r.userErrorMessage(err))
}

func (r *Root) userErrorMessage(err error) string {
	var incompleteManagedBlock *ports.IncompleteManagedBlockError
	if errors.As(err, &incompleteManagedBlock) {
		switch incompleteManagedBlock.Agent {
		case domainprofile.AgentCodex:
			return "Codex config contains an incomplete AGX-managed block. Fix or remove the broken AGX block in `~/.codex/config.toml`, then rerun the command."
		case domainprofile.AgentGemini:
			return "Gemini config contains an incomplete AGX-managed block. Fix or remove the broken AGX block in `~/.gemini/.env`, then rerun the command."
		default:
			return "A managed config block is incomplete. Fix or remove the broken AGX-managed block, then rerun the command."
		}
	}

	var notFound *domainprofile.NotFoundError
	if errors.As(err, &notFound) {
		if strings.TrimSpace(notFound.Name) == "" {
			return "Profile not found. Run `agx ls` to see available profiles."
		}
		return fmt.Sprintf("Profile %s not found. Run `agx ls` to see available profiles.", notFound.Name)
	}

	var alreadyExists *usecase.ProfileAlreadyExistsError
	if errors.As(err, &alreadyExists) {
		if strings.TrimSpace(alreadyExists.Name) == "" {
			return "Profile already exists. Use `agx edit <name> ...` to update it."
		}
		return fmt.Sprintf("Profile %s already exists. Use `agx edit %s ...` to update it.", alreadyExists.Name, alreadyExists.Name)
	}

	var duplicateConfig *usecase.DuplicateRelayConfigError
	if errors.As(err, &duplicateConfig) {
		if strings.TrimSpace(duplicateConfig.Name) != "" && strings.TrimSpace(duplicateConfig.ExistingName) != "" {
			return fmt.Sprintf("Cannot add profile %s: `base_url` and `api_key` already match existing profile %s.", duplicateConfig.Name, duplicateConfig.ExistingName)
		}
		return "Cannot add profile: `base_url` and `api_key` already match an existing profile."
	}

	var inUse *usecase.ProfileInUseError
	if errors.As(err, &inUse) {
		return fmt.Sprintf("%s. Run `agx doctor` for details.", err.Error())
	}

	var conflict *usecase.ConflictingAgentChangesError
	if errors.As(err, &conflict) {
		return err.Error()
	}

	var notBound *usecase.AgentNotBoundToRelayError
	if errors.As(err, &notBound) {
		return err.Error()
	}

	var backupNotFound *usecase.BackupNotFoundError
	if errors.As(err, &backupNotFound) {
		return err.Error()
	}

	var invalidAgent *usecase.InvalidAgentError
	if errors.As(err, &invalidAgent) {
		return "Agent must be one of: " + agentUsageHuman + "."
	}

	if errors.Is(err, errInteractiveCanceled) {
		return "Canceled."
	}

	var noBackup *usecase.NoBackupError
	if errors.As(err, &noBackup) {
		agent := strings.ToLower(string(noBackup.Agent))
		label := agent
		if agent != "" {
			label = strings.ToUpper(agent[:1]) + agent[1:]
		}
		return fmt.Sprintf("No %s backup available. Run `agx %s` at least once before `agx restore %s`.", label, agent, agent)
	}

	var unavailable *usecase.ManagedRuntimeUnavailableError
	if errors.As(err, &unavailable) {
		return err.Error() + ". Reinstall agx or rerun `agx install` to provision the managed runtime."
	}

	var targetNotFound *usecase.TargetNotFoundError
	if errors.As(err, &targetNotFound) {
		if targetNotFound.Agent != "" {
			return fmt.Sprintf("%s. Run `agx ls --all` to inspect known targets.", err.Error())
		}
		return fmt.Sprintf("%s. Run `agx ls --all` to inspect known targets.", err.Error())
	}

	return err.Error()
}
