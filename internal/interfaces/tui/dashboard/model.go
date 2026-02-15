package dashboard

import (
	basetui "github.com/kiddingbaby/agx/internal/tui"
	"github.com/kiddingbaby/agx/internal/usecase"
)

type Agent = basetui.Agent
type Callbacks = basetui.DashboardCallbacks
type Model = basetui.DashboardModel

func NewModel(sessionSvc *usecase.SessionService, keySvc *usecase.KeyService, cb Callbacks) Model {
	return basetui.NewDashboardModel(sessionSvc, keySvc, cb)
}
