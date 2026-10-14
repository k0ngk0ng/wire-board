package snooker_test

// Before automatic opening: no choice gate, correct actor/clock in singles and
// doubles; reconnect must not redraw; legacy choice saves retain their draw and
// deadline when upgraded; stale nomination requests cannot select a new opener.
import (
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
	"time"
)

func TestSnookerAutomaticOpeningHTTP(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, game.Module(), "snooker")
			h.Start(nil)
			r := h.People[0].Room()
			d := gametest.Data(r)
			if d["phase"] != "aim" || r["turnDeadline"].(float64) <= 0 {
				t.Fatal("opening waits for a choice", d["phase"])
			}
			first := int(d["turn"].(float64))
			before := d
			h.Restart()
			if !reflect.DeepEqual(before, gametest.Data(h.People[0].Room())) {
				t.Fatal("restore rerolled opening")
			}
			h.People[first].Action(map[string]any{"type": "break_choice", "team": 1, "window": d["window"]}, 400)
			h.People[(first+1)%n].Action(map[string]any{"type": "shoot", "angle": 0, "power": .06, "window": d["window"]}, 400)
			h.People[first].Action(map[string]any{"type": "shoot", "angle": 0, "power": .06, "window": d["window"]}, 200)
			h.Record("random opening directly accepts the drawn player's shot")
		})
	}
}
func TestSnookerRestoreLegacyOpeningChoice(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 4, Now: time.Unix(1700000000, 0)}
	m, e := mod.Start(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	s := state(t, m)
	s.Phase = "start_choice"
	s.Turn = 1
	s.TurnIndex = 1
	s.Due = []int64{0, ctx.Now.Add(time.Minute).UnixMilli(), 0, 0}
	oldWindow := s.Window
	m.Data, _ = json.Marshal(s)
	next, e := mod.Restore(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	after := state(t, next)
	if after.Phase != "aim" || after.Turn != 1 || after.OpeningSeat != 1 || after.OpeningTeam != 1 || after.Window <= oldWindow || !reflect.DeepEqual(after.Due, s.Due) {
		t.Fatal("legacy choice not resumed consistently", after)
	}
	again, e := mod.Restore(ctx, next)
	if e != nil || !reflect.DeepEqual(next, again) {
		t.Fatal("opening migration not idempotent")
	}
}
