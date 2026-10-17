package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalAuxiliaryLobLaunchAndFlight(t *testing.T) {
	f, err := os.Open("testdata/auxiliary-lob.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Chain, Tick      int
		Object, ArcFlags uint8
		SecondPlayerX    Fixed
		Before, After    WorldActionState
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	var state AuxiliaryState
	chain := -1
	landed := 0
	for i, r := range rows {
		if r.Chain != chain {
			state = r.Before.Auxiliary[r.Object-4]
			chain = r.Chain
		}
		if state != r.Before.Auxiliary[r.Object-4] {
			t.Fatalf("row%d original chain input was not continuous", i)
		}
		court := auxiliaryNewYorkCourt()
		court.passLaunchContext = func() PassLaunchContext { return PassLaunchContext{SecondPlayerX: r.SecondPlayerX} }
		got, err := StepAuxiliaryCarryThrow(state, AuxiliaryContext{Object: r.Object, Court: 1, Owner: &r.Before.Actors[0].Player, Launch: &AuxiliaryLaunchContext{ArcFlags: r.ArcFlags}}, court)
		if err != nil || got.State != r.After.Auxiliary[r.Object-4] {
			t.Fatalf("row%d tick%d: %s err%v", i, r.Tick, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After.Auxiliary[r.Object-4]), "auxiliary"), err)
		}
		if r.Tick == 0 && (got.LaunchPlan == nil || got.LaunchPlan.Target != r.After.LaunchTarget || got.LaunchPlan.ArcHeight != r.After.LaunchArcHeight) {
			t.Fatal("original shared launch target differs", i, got.LaunchPlan, r.After.LaunchTarget, r.After.LaunchArcHeight)
		}
		if r.Tick > 0 && got.LaunchPlan != nil {
			t.Fatal("entered throw recalculated launch")
		}
		state = got.State
		if r.Tick > 0 && state.Action == 0 {
			landed++
		}
	}
	if len(rows) != 308 || landed != 4 {
		t.Fatal("missing original throw calls/chains", len(rows), landed)
	}
	t.Logf("%d original auxiliary throws, including %d uninterrupted full flight/landing chains", len(rows), landed)
}
