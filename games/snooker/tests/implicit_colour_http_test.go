package snooker_test

// Directed physical layouts: each of the six colours is a real straight pot
// after a red. A stale client's default black nomination must not override it.
import (
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"math"
	"testing"
)

func TestSnookerAnyColourAfterRedHTTP(t *testing.T) {
	for id := 16; id <= 21; id++ {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			mod := game.Module()
			start := mod.Start
			mod.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
				m, e := start(ctx, raw)
				if e != nil {
					return m, e
				}
				s := state(t, m)
				s.Phase = "aim"
				s.Target = "colour"
				s.InHand = false
				for i := range s.Balls {
					s.Balls[i].Down = i >= 2 && i <= 15
				}
				s.Balls[0].Pos = game.Vec{X: game.TableWidth / 2, Y: .6}
				s.Balls[id].Pos = game.Vec{X: game.TableWidth / 2, Y: .2}
				s.Balls[1].Pos = game.Vec{X: .4, Y: 1.3}
				s.Logs = append(s.Logs, "验收定点：红球后自由击打彩球")
				m.Data, _ = json.Marshal(s)
				return m, nil
			}
			h := gametest.NewHTTP(t, 2, mod, "snooker")
			h.Start(nil)
			d := gametest.Data(h.People[0].Room())
			actor := int(d["turn"].(float64))
			team := int(d["teams"].([]any)[actor].(float64))
			a := map[string]any{"type": "shoot", "window": d["window"], "angle": -math.Pi / 2, "power": .25}
			if id%2 == 0 {
				a["nominate"] = 7
			} // old UI request, ignored by the automatic rule
			h.People[actor].Action(a, 200)
			done := gametest.Data(awaitStopped(t, h))
			if done["phase"] != "aim" || done["turn"] != float64(actor) || done["scores"].([]any)[team] != float64(game.BallValue(id)) || done["scores"].([]any)[1-team] != float64(0) || done["target"] != "red" {
				t.Fatal("legal colour was penalized", done["phase"], done["scores"])
			}
			h.Record("real colour pot accepted without manual nomination")
		})
	}
}
