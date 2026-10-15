// Package games is the application's composition root, not a shared game engine.
package games

import (
	chess "github.com/k0ngk0ng/wire-board/games/chess/server"
	doudizhu "github.com/k0ngk0ng/wire-board/games/doudizhu/server"
	gomoku "github.com/k0ngk0ng/wire-board/games/gomoku/server"
	guandan "github.com/k0ngk0ng/wire-board/games/guandan/server"
	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
	pairs "github.com/k0ngk0ng/wire-board/games/pairs/server"
	racing "github.com/k0ngk0ng/wire-board/games/racing/server"
	shengji "github.com/k0ngk0ng/wire-board/games/shengji/server"
	snooker "github.com/k0ngk0ng/wire-board/games/snooker/server"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	xiangqi "github.com/k0ngk0ng/wire-board/games/xiangqi/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Modules registers independent games. Add the game's server.Module() here;
// its frontend is discovered from games/<id>/web/module.tsx automatically.
func Modules() []gameapi.Module {
	return []gameapi.Module{monopoly.Module(), starfort.Module(), doudizhu.Module(), guandan.Module(), shengji.Module(), snooker.Module(), pairs.Module(), gomoku.Module(), chess.Module(), xiangqi.Module(), racing.Module()}
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
