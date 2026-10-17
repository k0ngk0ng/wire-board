package engine

// Before implementation, cover descent across/onto the floor, subpixel height,
// ascent/no contact, signed horizontal speed, facing versus old pose flip, spin timing,
// and clearing the excluded catcher only after a grounded pass. The oracle
// runs unchanged original instructions with explicitly isolated inputs, not a
// natural-match claim. Existing natural pass and world traces remain required.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestPassFloorAgainstOriginalFunction(t *testing.T) {
	f, err := os.Open("testdata/pass-floor-function.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	type observation struct {
		observedBall
		Excluded uint8
	}
	var rows []struct{ Before, After observation }
	if err := json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 216 {
		t.Fatal("unexpected isolated original pass coverage", len(rows))
	}
	landings := 0
	for i, row := range rows {
		before, want := ballFromObservation(row.Before.observedBall), ballFromObservation(row.After.observedBall)
		state := WorldActionState{Ball: before, Holder: int8(row.Before.Holder), ExcludedCatcher: int8(row.Before.Excluded)}
		sounds, err := advanceWorldBallStep(&state, WorldActionContext{}, NewYorkBallCourt())
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if state.Ball != want || state.ExcludedCatcher != int8(row.After.Excluded) || state.Holder != int8(row.After.Holder) || len(sounds) != 0 {
			t.Fatalf("case %d before=%+v\ngot=%+v excluded=%d\nwant=%+v excluded=%d sounds=%v", i, before, state.Ball, state.ExcludedCatcher, want, int8(row.After.Excluded), sounds)
		}
		if want.Action == BallBounce {
			landings++
			if want.Entered || want.Airborne || state.ExcludedCatcher != -1 {
				t.Fatal("floor must request bounce entry and release catcher restriction")
			}
		}
	}
	if landings == 0 || landings == len(rows) {
		t.Fatal("missing grounded or airborne branches", landings)
	}
	t.Logf("matched %d isolated original pass updates, %d floor-to-bounce transitions", len(rows), landings)
}
