package engine

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
)

type worldLoopFixture struct {
	Initial WorldLoopState
	Config  struct {
		PursuitReserved                                        *PursuitReservedBytes
		PursuitAlias74                                         *PursuitAlias74
		Actions                                                json.RawMessage
		ScoringBonuses                                         [4]bool
		ObjectKinds                                            [13]uint8
		HoopLinks                                              [6]uint8
		AttackGrades, JumpEnhancements                         [4]uint8
		HumanTactics                                           [2]uint8
		Difficulty, BackboardStrength, LayoutID, TrackedObject uint8
	}
	Records []struct {
		InputFrame, ProjectionFrame int
		Input                       WorldLoopInput
		Expected                    WorldLoopState
	}
}

var loopFixtureOnce sync.Once
var loopFixtureValue worldLoopFixture
var loopFixtureError error

func originalWorldLoopFixture(t *testing.T) worldLoopFixture {
	t.Helper()
	loopFixtureOnce.Do(func() {
		f, err := os.Open("testdata/world-loop.json.gz")
		if err != nil {
			loopFixtureError = err
			return
		}
		defer f.Close()
		z, err := gzip.NewReader(f)
		if err != nil {
			loopFixtureError = err
			return
		}
		defer z.Close()
		loopFixtureError = json.NewDecoder(z).Decode(&loopFixtureValue)
	})
	if loopFixtureError != nil {
		t.Fatal(loopFixtureError)
	}
	if len(loopFixtureValue.Records) != 1998 {
		t.Fatal("missing complete original continuous oracle", len(loopFixtureValue.Records))
	}
	return loopFixtureValue
}

func (f worldLoopFixture) context(t *testing.T) WorldLoopContext {
	t.Helper()
	var a worldActionFixture
	if err := json.Unmarshal(f.Config.Actions, &a.Context); err != nil {
		t.Fatal(err)
	}
	c := WorldLoopContext{Actions: a.context(t), ObjectKinds: f.Config.ObjectKinds, HoopLinks: f.Config.HoopLinks, AttackGrades: f.Config.AttackGrades, JumpEnhancements: f.Config.JumpEnhancements, HumanTactics: f.Config.HumanTactics, Difficulty: f.Config.Difficulty, BackboardStrength: f.Config.BackboardStrength, LayoutID: f.Config.LayoutID, TrackedObject: f.Config.TrackedObject, PursuitAlias74: f.Config.PursuitAlias74, PursuitReserved: f.Config.PursuitReserved}
	for i, bonus := range f.Config.ScoringBonuses {
		c.Actions.Profiles[i].ScoringBonus = bonus
	}
	return c
}

func firstWorldLoopDifference(a, b reflect.Value, path string) string {
	if reflect.DeepEqual(a.Interface(), b.Interface()) {
		return ""
	}
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if d := firstWorldLoopDifference(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name); d != "" {
				return d
			}
		}
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			if d := firstWorldLoopDifference(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i)); d != "" {
				return d
			}
		}
	}
	return fmt.Sprintf("%s: got %v want %v", path, a.Interface(), b.Interface())
}

func TestWorldLoopSingleInitialStateOriginalTrajectory(t *testing.T) {
	f := originalWorldLoopFixture(t)
	ctx := f.context(t)
	state := f.Initial // The only state initialization; expected rows are never inputs.
	calls := 0
	for index, row := range f.Records {
		got, err := StepWorldLoop(state, row.Input, ctx)
		if err != nil {
			t.Fatalf("update %d input frame %d projection frame %d: %v", index, row.InputFrame, row.ProjectionFrame, err)
		}
		if got.State != row.Expected {
			t.Fatalf("update %d input frame %d projection frame %d: %s", index, row.InputFrame, row.ProjectionFrame, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(row.Expected), "state"))
		}
		if (index == 0) != (got.HUD == nil) {
			t.Fatal("initial-input boundary incorrectly executed HUD", index)
		}
		calls += len(got.ActionCalls)
		state = got.State
	}
	if calls != 26093 {
		t.Fatal("wrong cumulative action calls", calls)
	}
	t.Logf("one initial state + controller inputs reproduced all %d core updates, %d object actions, camera and projection; no intermediate state injection", len(f.Records), calls)
}

func TestWorldLoopResumeAndAtomicError(t *testing.T) {
	f := originalWorldLoopFixture(t)
	ctx := f.context(t)
	state := f.Initial
	for _, row := range f.Records[:120] {
		got, err := StepWorldLoop(state, row.Input, ctx)
		if err != nil {
			t.Fatal(err)
		}
		state = got.State
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var restored WorldLoopState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	want, err := StepWorldLoop(state, f.Records[120].Input, ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, err := StepWorldLoop(restored, f.Records[120].Input, ctx)
	if err != nil || got.State != want.State {
		t.Fatal("resume changed core sequence", err)
	}
	ctx.Actions.CourtID = 255
	failed, err := StepWorldLoop(state, f.Records[120].Input, ctx)
	if err == nil || failed.State != state || len(failed.SoundIDs) != 0 || len(failed.ActionCalls) != 0 || failed.HUD != nil {
		t.Fatal("failed update retained partial state or events", err)
	}
}

func TestWorldLoopDrawResetKeepsThirteenthSlot(t *testing.T) {
	s := ProjectionState{DrawCount: 12}
	for i := range s.DrawOrder {
		s.DrawOrder[i] = uint8(i)
	}
	s.Projected[0] = ScreenPosition{X: -17, Y: 25}
	got := resetOriginalWorldDrawList(s)
	for i := 0; i < 12; i++ {
		if got.DrawOrder[i] != 255 {
			t.Fatal(got)
		}
	}
	if got.DrawOrder[12] != 12 || got.DrawCount != 0 || got.Projected != s.Projected {
		t.Fatal("renderer reset destroyed retained cache", got)
	}
}
