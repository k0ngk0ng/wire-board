package engine

// Court6 failure inventory before runtime connection: score-side inversion;
// separate slot progress from board-trigger bytes; shadow RNG/score copies;
// ignored stopped-reel bits; lost FF-cleared NMI queue commands; wrong award
// saturation, palette, or sound order; treating source calls as whole-world proof.
import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestCourtSixObservedGoalsTriggerCanonicalSlotProgress(t *testing.T) {
	raw, e := os.ReadFile("testdata/court-six-slot-goals.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Frame  int
		Sounds []uint8
	}
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 14 {
		t.Fatal("missing observed original Court6 goals")
	}
	f, e := os.Open("testdata/court-six-slot-goals.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	for _, row := range rows {
		var b [4096]byte
		if _, e = io.ReadFull(z, b[:]); e != nil {
			t.Fatal(e)
		}
		before, want := boardContactRAM(b[:2048]), boardContactRAM(b[2048:])
		c := boardContactContextRAM(b[:2048])
		got, e := StepBoardContactObject(before, 6, c)
		if e != nil {
			t.Fatal(row.Frame, e)
		}
		if got.State != want {
			t.Fatal(row.Frame, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(want), "board"))
		}
		if !slices.Equal(got.SoundIDs, row.Sounds) || got.Rim.Outcome != RimGoal || got.State.CourtScoreFlags[(got.Rim.ScoringSide+1)&1] != 1 {
			t.Fatal("original goal did not trigger its opposite slot", row.Frame, got, row.Sounds)
		}
	}
	t.Log("matched 14 observed original Court6 goals and canonical 06DD/06DE triggers")
}
func TestCourtSixObservedSlotCallsIncludeRealAward(t *testing.T) {
	f, e := os.Open("testdata/court-six-slot-observed.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		Frame   int
		Before  CourtAnimationState
		Context CourtAnimationContext
		After   CourtAnimationStep
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 7244 {
		t.Fatal("missing original Court6 slot calls")
	}
	awards, random := 0, 0
	for _, row := range rows {
		got, e := StepCourtAnimation(row.Before, row.Context)
		if e != nil || !reflect.DeepEqual(got, row.After) {
			t.Fatal(row.Frame, e, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(row.After), "slot"))
		}
		if got.Random != row.Context.Random {
			random++
		}
		if got.Scores != row.Context.Scores {
			awards++
			if row.Context.Scores != ([2]uint16{12, 26}) || got.Scores != ([2]uint16{3, 26}) || got.State.OutcomeFlags&128 == 0 || !slices.Contains(got.Sounds, uint8(9)) {
				t.Fatal("observed -9 award changed", row.Frame, got)
			}
		}
	}
	if awards != 1 || random != 165 {
		t.Fatal("real slot activation/award missing", awards, random)
	}
	t.Log("matched 7244 passive original slot calls, 165 shared RNG changes and the real 12:26 -> 3:26 award; full RAM/screenshots match uninstrumented run")
}

func TestCourtSixOriginalGoalToCompletedSlotChains(t *testing.T) {
	f, e := os.Open("testdata/court-six-slot-chains.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var chains []struct {
		GoalIndex             int
		GoalBefore, GoalAfter []byte
		GoalSounds            []uint8
		Calls                 []struct {
			Before  CourtAnimationState
			Context CourtAnimationContext
			After   CourtAnimationStep
		}
	}
	if e = json.NewDecoder(z).Decode(&chains); e != nil {
		t.Fatal(e)
	}
	if len(chains) != 14 {
		t.Fatal("incomplete goal-slot chains")
	}
	calls, awards := 0, 0
	for _, chain := range chains {
		if len(chain.GoalBefore) != 2048 || len(chain.GoalAfter) != 2048 || len(chain.Calls) == 0 {
			t.Fatal("missing source goal state")
		}
		before := boardContactRAM(chain.GoalBefore)
		want := boardContactRAM(chain.GoalAfter)
		goal, e := StepBoardContactObject(before, 6, boardContactContextRAM(chain.GoalBefore))
		if e != nil || goal.State != want || !slices.Equal(goal.SoundIDs, chain.GoalSounds) {
			t.Fatal("source chain goal mismatch", chain.GoalIndex, e)
		}
		slot := CourtAnimationState{SlotProgress: goal.State.CourtScoreFlags, OutcomeFlags: chain.GoalAfter[0x6df]}
		for i, p := range goal.State.Players {
			slot.ReelFrames[i] = p.NewYorkPassage
		}
		for i, p := range goal.State.Auxiliary {
			slot.ReelFrames[i+4] = p.NewYorkPassage
		}
		ctx := chain.Calls[0].Context
		ctx.Random = goal.State.Random
		ctx.Scores = goal.State.Scores
		for i, h := range goal.State.Hoops {
			ctx.HoopLocks[i] = h.ScoreLock
		}
		for tick, row := range chain.Calls {
			if slot != row.Before || ctx != row.Context {
				t.Fatal("chain input diverged without intermediate injection", chain.GoalIndex, tick, slot, row.Before, ctx, row.Context)
			}
			got, e := StepCourtAnimation(slot, ctx)
			if e != nil || !reflect.DeepEqual(got, row.After) {
				t.Fatal(chain.GoalIndex, tick, e, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(row.After), "slot"))
			}
			encoded, e := json.Marshal(struct {
				Slot    CourtAnimationState
				Context CourtAnimationContext
			}{slot, ctx})
			if e != nil {
				t.Fatal(e)
			}
			var restored struct {
				Slot    CourtAnimationState
				Context CourtAnimationContext
			}
			if e = json.Unmarshal(encoded, &restored); e != nil {
				t.Fatal(e)
			}
			again, e := StepCourtAnimation(restored.Slot, restored.Context)
			if e != nil || !reflect.DeepEqual(again, got) {
				t.Fatal("goal-slot JSON replay diverged")
			}
			if got.Scores != ctx.Scores {
				awards++
			}
			slot = got.State
			ctx.Random, ctx.Scores = got.Random, got.Scores
			ctx.VideoFrame++
			calls++
		}
		if slot.SlotProgress != ([2]uint8{}) || slot.OutcomeFlags != 0 {
			t.Fatal("slot chain did not clear original completion flags", chain.GoalIndex, slot)
		}
	}
	if calls != 367 || awards != 1 {
		t.Fatal("missing continuous slot/award coverage", calls, awards)
	}
	t.Log("14 original goal -> completed slot chains: 367 calls and one award, no intermediate RNG/reel/score injection, JSON replay at every boundary")
}
