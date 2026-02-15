package keymgr

import (
	basetui "github.com/kiddingbaby/agx/internal/tui"
	"github.com/kiddingbaby/agx/internal/usecase"
)

type Model = basetui.KeyManagerModel

func NewModel(keySvc *usecase.KeyService) Model {
	return basetui.NewKeyManagerModel(keySvc)
}
