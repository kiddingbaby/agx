package tui

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kiddingbaby/agx/internal/interfaces/tui/dashboard"
	"github.com/kiddingbaby/agx/internal/interfaces/tui/keymgr"
	"github.com/kiddingbaby/agx/internal/usecase"
)

type LaunchError struct {
	Err error
}

func (e *LaunchError) Error() string {
	return e.Err.Error()
}

func (e *LaunchError) Unwrap() error {
	return e.Err
}

func RunDashboard(keySvc *usecase.KeyService, sessionSvc *usecase.SessionService, launchSvc *usecase.LaunchService) error {
	for {
		var postAction func() error

		callbacks := dashboard.Callbacks{
			OnAttach: func(sessionName string) {
				postAction = func() error {
					return sessionSvc.Attach(sessionName)
				}
			},
			OnLaunch: func(agent dashboard.Agent) {
				postAction = func() error {
					if err := launchSvc.Launch(agent.Name, getCwd(), ""); err != nil {
						return &LaunchError{Err: err}
					}
					return nil
				}
			},
			OnKill: func(sessionName string) {
				_ = sessionSvc.Kill(sessionName)
			},
		}

		finalModel, err := tea.NewProgram(dashboard.NewModel(sessionSvc, keySvc, callbacks), tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}

		dm, ok := finalModel.(dashboard.Model)
		if !ok {
			return fmt.Errorf("unexpected dashboard model type: %T", finalModel)
		}

		if dm.ShouldSwitchToKeys() {
			if err := RunKeyManager(keySvc); err != nil {
				return err
			}
			continue
		}

		if postAction != nil {
			if err := postAction(); err != nil {
				return err
			}
		}
		return nil
	}
}

func RunKeyManager(keySvc *usecase.KeyService) error {
	_, err := tea.NewProgram(keymgr.NewModel(keySvc), tea.WithAltScreen()).Run()
	return err
}

func ErrorMessage(err error) string {
	var noActive *usecase.NoActiveKeyError
	if errors.As(err, &noActive) {
		return fmt.Sprintf("No active key for %s. Use 'agx keys' to add one.", noActive.Provider)
	}

	var launchErr *LaunchError
	if errors.As(err, &launchErr) {
		return fmt.Sprintf("Error launching session: %v", launchErr.Err)
	}

	return fmt.Sprintf("Error: %v", err)
}

func getCwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
