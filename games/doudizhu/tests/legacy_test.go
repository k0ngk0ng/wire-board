package doudizhu_test

// Replay real pre-v2 opening snapshots through production Apply/View/BotAction.
// Only this test adapter permits creating a legacy room; it never ships.
import (
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/doudizhu/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
)

func legacyModule() gameapi.Module {
	mod := game.Module()
	mod.MinPlayers = 3
	mod.MaxPlayers = 4
	mod.Configure = func(n int, raw json.RawMessage) (json.RawMessage, error) {
		if n != 3 && n != 4 {
			return nil, fmt.Errorf("legacy capacity")
		}
		mode := "classic3"
		if n == 4 {
			mode = "shanghai4"
		}
		o := game.Options{Mode: mode, Rounds: 1, TurnSeconds: 30, AllowStraight: true}
		if len(raw) > 0 {
			var v map[string]json.RawMessage
			if json.Unmarshal(raw, &v) != nil {
				return nil, fmt.Errorf("legacy options")
			}
			for k := range v {
				if k != "mode" && k != "rounds" && k != "turnSeconds" && k != "allowStraight" && k != "personalDoubling" {
					return nil, fmt.Errorf("legacy unknown option")
				}
			}
			if e := json.Unmarshal(raw, &o); e != nil {
				return nil, e
			}
		}
		if o.Mode != mode || o.Rounds < 1 || o.Rounds > 10 || o.TurnSeconds < 10 || o.TurnSeconds > 120 {
			return nil, fmt.Errorf("legacy invalid options")
		}
		return json.Marshal(o)
	}
	mod.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		o, e := mod.Configure(ctx.Players, raw)
		if e != nil {
			return gameapi.Match{}, e
		}
		b, e := os.ReadFile(fmt.Sprintf("testdata/legacy-%d-initial.json", ctx.Players))
		if e != nil {
			return gameapi.Match{}, e
		}
		var m gameapi.Match
		if e = json.Unmarshal(b, &m); e != nil {
			return m, e
		}
		var s game.State
		_ = json.Unmarshal(m.Data, &s)
		_ = json.Unmarshal(o, &s.Options)
		s.DealtAt = ctx.Now.UnixMilli()
		for _, p := range m.Actors {
			s.Due[p] = ctx.Now.UnixMilli() + int64(s.Options.TurnSeconds)*1000
		}
		m.Deadline = s.Due[m.Actors[0]]
		m.Data, _ = json.Marshal(s)
		return m, nil
	}
	return mod
}
