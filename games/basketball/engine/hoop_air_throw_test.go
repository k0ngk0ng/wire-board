package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type hoopAirThrowOracle struct {
	worldActionFixture
	Actor             uint8
	Held              [4]Buttons
	Prepared          *WorldActionState
	Sounds            []uint8
	BackboardWrite    bool
	BackboardStrength uint8
}

func readHoopAirThrowOracle(t *testing.T) ([]hoopAirThrowOracle, []hoopAirThrowOracle) {
	t.Helper()
	var fixture struct {
		Calls  []hoopAirThrowOracle
		Stages []hoopAirThrowOracle
	}
	readPropAttackFixture(t, "testdata/hoop-air-throw-original.json.gz", &fixture)
	if len(fixture.Calls) != 2048 {
		t.Fatal("missing original actor/entry/owner/air/rider cases", len(fixture.Calls))
	}
	if len(fixture.Stages) != 3 {
		t.Fatal("missing complete3A/3C action stages")
	}
	return fixture.Calls, fixture.Stages
}

func TestHoopAirThrowOriginalCalls(t *testing.T) {
	rows, _ := readHoopAirThrowOracle(t)
	for i, r := range rows {
		result := WorldActionStep{State: r.Before}
		ctx := r.context(t)
		ctx.ActorHeldButtons = r.Held
		court := NewYorkGroundCourt()
		var err error
		if ctx.CourtID != 1 {
			court, err = originalWorldMotionCourt(&result, ctx, r.Actor)
			if err != nil {
				t.Fatal(i, err)
			}
		}
		if err = advanceWorldPlayer(&result, r.Actor, ctx, court); err != nil {
			t.Fatal(i, err)
		}
		if result.State != r.After {
			t.Fatal(i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(result.SoundIDs, r.Sounds) {
			t.Fatal(i, result.SoundIDs, r.Sounds)
		}
		if result.ChangeBackboardStrength != r.BackboardWrite || r.BackboardWrite && result.BackboardStrength != r.BackboardStrength {
			t.Fatal(i, "056D source command", result.ChangeBackboardStrength, result.BackboardStrength, r.BackboardWrite, r.BackboardStrength)
		}
	}
}

func TestHoopAirThrowBackboardContextPersists(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/hoop-air-throw-runtime.json.gz", &s)
	s.Playing.Context.BackboardStrength = 3
	p := &s.Playing.World.Preparation.Actions.Actors[0].Player
	p.Action, p.Entered, p.Request.Pending = 0x3c, false, false
	s.FramesUntilLogic = 1
	next, err := StepVideo(s, [4]Buttons{ButtonUp})
	if err != nil {
		t.Fatal(err)
	}
	if next.State.Playing.Context.BackboardStrength != 0 {
		t.Fatal("056D actor write did not reach canonical context")
	}
	if s.Playing.Context.BackboardStrength != 3 {
		t.Fatal("input context mutated")
	}
	raw, err := json.Marshal(next.State)
	if err != nil {
		t.Fatal(err)
	}
	var restored RuntimeState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Playing.Context.BackboardStrength != 0 {
		t.Fatal("context write lost on restore")
	}
}

func TestHoopAirThrowOriginalWholeStage(t *testing.T) {
	_, rows := readHoopAirThrowOracle(t)
	for i, r := range rows {
		ctx := r.context(t)
		ctx.ActorHeldButtons = r.Held
		got, err := StepWorldActions(r.Before, ctx)
		if err != nil {
			t.Fatal(i, err)
		}
		if got.State != r.After {
			t.Fatal(i, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(got.Calls, r.Calls) || !slices.Equal(got.SoundIDs, r.Sounds) {
			t.Fatal(i, "original scheduler/sounds", got.Calls, r.Calls, got.SoundIDs, r.Sounds)
		}
		if got.ChangeBackboardStrength != r.BackboardWrite || r.BackboardWrite && got.BackboardStrength != r.BackboardStrength {
			t.Fatal(i, "source056D phase write")
		}
	}
}

func TestHoopAirThrowOnlineFailureContinues(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/hoop-air-throw-runtime.json.gz", &s)
	if s.VideoFrames != 8961 {
		t.Fatal("wrong incident boundary")
	}
	for i := 0; i < 180; i++ {
		step, err := StepVideo(s, [4]Buttons{})
		if err != nil {
			t.Fatalf("frame %d: %v", s.VideoFrames+1, err)
		}
		s = step.State
	}
	if s.VideoFrames != 9141 {
		t.Fatal("runtime did not continue", s.VideoFrames)
	}
}
