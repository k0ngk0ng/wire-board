package snooker_test

// Before aligning the server turn budget with the local cue phase: consecutive
// computer strokes advance before the previous full clip can finish, building
// a playback backlog; adding metadata invalidates already stored flight hashes.
import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestSnookerTurnBudgetIncludesLocalCuePhase(t *testing.T) {
	mod, ctx, m := begin(t, 2)
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": 0, "power": .06})
	s := state(t, m)
	raw, _ := json.Marshal(s.Flight)
	var f map[string]any
	_ = json.Unmarshal(raw, &f)
	cue, ok := f["cueDuration"].(float64)
	if !ok || cue < 200 || cue > 500 || s.Flight.Start-ctx.Now.UnixMilli() != int64(cue) {
		t.Fatal("server does not reserve the locally played cue phase", f["cueDuration"])
	}
	early := gameapi.Context{Players: 2, Actor: -1, Now: time.UnixMilli(m.SystemDeadline - 1)}
	same, e := mod.OnTimer(early, m)
	if e != nil || !reflect.DeepEqual(same, m) {
		t.Fatal("turn advanced before full clip", e)
	}
	trace(t, m, finishStroke(t, mod, ctx, m))
}
func TestSnookerPreCueFlightRestoresWithoutNewAnimationOrHash(t *testing.T) {
	raw, e := os.ReadFile("testdata/pre-cue-flight.json")
	if e != nil {
		t.Fatal(e)
	}
	var m gameapi.Match
	if e = json.Unmarshal(raw, &m); e != nil {
		t.Fatal(e)
	}
	mod := game.Module()
	if _, e = mod.View(m, 0); e != nil {
		t.Fatal("old flight rejected", e)
	}
	ctx := gameapi.Context{Players: 2, Actor: -1, Now: time.UnixMilli(m.SystemDeadline - 1)}
	restored, e := mod.Restore(ctx, m)
	if e != nil || !reflect.DeepEqual(restored, m) {
		t.Fatal("old flight was retimed or rewritten", e)
	}
	next := finishStroke(t, mod, ctx, m)
	if state(t, next).Phase == "rolling" {
		t.Fatal("old flight could not settle")
	}
	trace(t, m, next)
}
