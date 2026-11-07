package production

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/games"
	contra "github.com/k0ngk0ng/wire-board/games/contra/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Modules validates native runtime identities before returning any
// modules. Callers must handle initialization errors before opening a server.
// games.Modules remains the compatibility collection for isolated fixtures.
func Modules() ([]gameapi.Module, error) {
	module, err := contra.NewProductionModule()
	if err != nil {
		return nil, fmt.Errorf("initialize contra: %w", err)
	}
	module.Controls = games.ControllerDefinitionFor(module.ID)
	return append(games.Modules(), module), nil
}
