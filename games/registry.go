// Package games is the application's composition root, not a shared game engine.
package games

import (
	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Modules registers independent games. Add the game's server.Module() here;
// its frontend is discovered from games/<id>/web/module.tsx automatically.
func Modules() []gameapi.Module {
	return []gameapi.Module{monopoly.Module(), starfort.Module()}
}

// Legacy lists games that retain the existing room/snapshot protocol adapter.
// Their rules, views, assets and rule tests live in their own directories.
var Legacy = []struct{ ID, Name string }{
	{"splendor", "璀璨宝石"},
	{"rail", "铁路环游"},
	{"catan", "卡坦岛"},
	{"carcassonne", "卡卡颂"},
	{"sanguosha", "三国杀"},
	{"dota", "兵线争锋"},
}
