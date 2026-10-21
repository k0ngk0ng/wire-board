package engine

// Failure inventory before fix: entered dunk wrongly requires the shared target
// to remain a hoop; stale target writes a fabricated durability copy; 0577 byte
// aliases corrupt jumpPhase/attackVariant; action command hits the wrong object;
// relaxing initialization silently permits an invalid mount; continuous source
// chain or JSON restoration diverges. Original full-handler fixtures first.
import (
	"reflect"
	"slices"
	"testing"
)

func TestDunkSharedTargetOriginalCallsAndChains(t *testing.T) {
	var rows struct {
		Calls  []launchedPlayerFixture
		Chains [][]launchedPlayerFixture
	}
	readPropAttackFixture(t, "testdata/dunk-shared-target.json.gz", &rows)
	if len(rows.Calls) != 420 || len(rows.Chains) != 48 {
		t.Fatal("incomplete shared-target source domain")
	}
	run := func(row launchedPlayerFixture, s WorldActionState) WorldActionState {
		t.Helper()
		r := WorldActionStep{State: s}
		c := row.context(t)
		c.ActorHeldButtons = row.Held
		court, e := originalWorldMotionCourt(&r, c, row.Actor)
		if e != nil {
			t.Fatal(e)
		}
		if e = advanceWorldPlayer(&r, row.Actor, c, court); e != nil {
			t.Fatal(row.Actor, s.DunkTargetObject, e)
		}
		if r.State != row.After {
			t.Fatal(firstWorldLoopDifference(reflect.ValueOf(r.State), reflect.ValueOf(row.After), "world"))
		}
		if !slices.Equal(r.SoundIDs, row.Sounds) {
			t.Fatal("source sound differs")
		}
		return r.State
	}
	for _, row := range rows.Calls {
		run(row, row.Before)
	}
	for _, chain := range rows.Chains {
		s := chain[0].Before
		for _, row := range chain {
			if s != row.Before {
				t.Fatal("source chain discontinuity")
			}
			s = run(row, s)
		}
	}
}
