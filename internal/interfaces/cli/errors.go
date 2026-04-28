package cli

import (
	"errors"
	"fmt"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func (r *Root) printUserError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(r.stderr, "Error: %s\n", r.userErrorMessage(err))
}

func (r *Root) userErrorMessage(err error) string {
	var notFound *domainprofile.NotFoundError
	if errors.As(err, &notFound) {
		if strings.TrimSpace(notFound.Name) == "" {
			return "Relay not found. Run `agx ls` to see available relays."
		}
		return fmt.Sprintf("Relay %s not found. Run `agx ls` to see available relays.", notFound.Name)
	}

	var alreadyExists *usecase.ProfileAlreadyExistsError
	if errors.As(err, &alreadyExists) {
		if strings.TrimSpace(alreadyExists.Name) == "" {
			return "Relay already exists. Use `agx edit <relay> ...` to update it."
		}
		return fmt.Sprintf("Relay %s already exists. Use `agx edit %s ...` to update it.", alreadyExists.Name, alreadyExists.Name)
	}

	var inUse *usecase.ProfileInUseError
	if errors.As(err, &inUse) {
		return err.Error()
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
		return "Agent must be one of: codex, claude, gemini."
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
		return fmt.Sprintf("No %s backup available. Run `agx edit <relay> --bind %s` before `agx restore --agent %s`.", label, agent, agent)
	}

	return err.Error()
}
