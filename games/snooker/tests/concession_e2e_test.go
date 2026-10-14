package snooker_test

// Before concession/claim handling: the only quit button forfeits the entire
// match; premature frame concessions or unilateral doubles offers are accepted;
// the other side cannot choose to play on; a restored cue can move away again.
import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestSnookerFrameConcessionNeedsEligibilityTeamAndOpponent(t *testing.T) {
	mod, ctx, m := begin(t, 4)
	ctx.Actor = 1
	raw := mustAction(map[string]any{"type": "concede_frame", "window": state(t, m).Window})
	if _, e := mod.Apply(ctx, m, raw); e == nil {
		t.Fatal("premature frame concession")
	}
	s := state(t, m)
	s.Scores = [2]int{200, 0}
	m.Data, _ = json.Marshal(s)
	m = action(t, mod, ctx, m, map[string]any{"type": "concede_frame"})
	if state(t, m).Phase != "aim" {
		t.Fatal("one partner conceded for team")
	}
	ctx.Actor = 3
	m = action(t, mod, ctx, m, map[string]any{"type": "concede_frame"})
	if state(t, m).Phase != "concede" || m.Result != nil {
		t.Fatal("concession didn't await opponent")
	}
	raw = mustAction(map[string]any{"type": "cancel_vote", "window": state(t, m).Window})
	bad, e := mod.Apply(ctx, m, raw)
	if e == nil || !reflect.DeepEqual(m, bad) {
		t.Fatal("submitted concession withdrawn")
	}
	ctx.Actor = 0
	m = action(t, mod, ctx, m, map[string]any{"type": "concede_reply", "choice": "continue"})
	if state(t, m).Phase != "aim" || state(t, m).Turn != 0 {
		t.Fatal("opponent could not play on")
	}
	for _, p := range []int{1, 3} {
		ctx.Actor = p
		m = action(t, mod, ctx, m, map[string]any{"type": "concede_frame"})
	}
	ctx.Actor = 0
	m = action(t, mod, ctx, m, map[string]any{"type": "concede_reply", "choice": "accept"})
	if m.Result == nil || len(m.Result.Winners) != 2 || m.Result.Winners[0] != 0 || state(t, m).Results[0].Reason != "conceded" {
		t.Fatal("wrong accepted team concession")
	}
	trace(t, m)
}
func TestSnookerRemainingBlackCanOnlyBeClaimedWithMoreThanSevenLead(t *testing.T) {
	mod, ctx, m := begin(t, 2)
	m = onlyBlack(t, m, [2]int{7, 0})
	raw := mustAction(map[string]any{"type": "claim_frame", "window": state(t, m).Window})
	if _, e := mod.Apply(ctx, m, raw); e == nil {
		t.Fatal("seven-point lead claimed")
	}
	s := state(t, m)
	s.Scores[0] = 8
	m.Data, _ = json.Marshal(s)
	m = action(t, mod, ctx, m, map[string]any{"type": "claim_frame"})
	if m.Result == nil || state(t, m).Results[0].Reason != "claimed" {
		t.Fatal("valid black claim not accepted")
	}
	trace(t, m)
}
func TestSnookerOriginalPositionRestoreLocksCuePlacement(t *testing.T) {
	mod, ctx, m := begin(t, 2)
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": 0, "power": .06})
	m = finishStroke(t, mod, ctx, m)
	ctx.Actor = state(t, m).Turn
	ctx.Now = ctx.Now.Add(time.Minute)
	m = action(t, mod, ctx, m, map[string]any{"type": "foul_choice", "choice": "restore"})
	if state(t, m).InHand {
		t.Fatal("restored original cue is still freely placeable")
	}
	ctx.Actor = 0
	raw := mustAction(map[string]any{"type": "place", "window": state(t, m).Window, "x": .6, "y": .8})
	bad, e := mod.Apply(ctx, m, raw)
	if e == nil || !reflect.DeepEqual(bad, m) {
		t.Fatal("original-position retry moved white")
	}
	trace(t, m)
}
