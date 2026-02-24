package main

import (
	"fmt"
	"os"

	interfacetui "github.com/kiddingbaby/agx/internal/interfaces/tui"
	"github.com/kiddingbaby/agx/internal/usecase"
)

// runTUI runs the main TUI Dashboard
func runTUI(keySvc *usecase.KeyService, sessionSvc *usecase.SessionService, launchSvc *usecase.LaunchService) {
	if err := interfacetui.RunDashboard(keySvc, sessionSvc, launchSvc); err != nil {
		handleTUIError(err)
	}
}

// runKeyManagerTUI runs the key manager TUI
func runKeyManagerTUI(keySvc *usecase.KeyService) {
	if err := interfacetui.RunKeyManager(keySvc); err != nil {
		handleTUIError(err)
	}
}

func handleTUIError(err error) {
	fmt.Fprintln(os.Stderr, interfacetui.ErrorMessage(err))
	os.Exit(1)
}
