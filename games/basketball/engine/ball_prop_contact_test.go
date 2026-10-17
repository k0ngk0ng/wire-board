package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalBallPropContactInstructions(t *testing.T) {
	f, e := os.Open("testdata/ball-prop-contact.json.gz")
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
		Candidate     int8
		Before, After WorldActionState
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 3072 {
		t.Fatal("incomplete original boundary matrix", len(rows))
	}
	for i, r := range rows {
		s := r.Before
		state := InteractionState{Ball: s.Ball, Holder: s.Holder, LastTouch: s.LastTouch, SelectedHoop: s.SelectedHoop, Auxiliary: s.Auxiliary, SpecialHolder: s.SpecialHolder}
		for j, a := range s.Actors {
			state.Players[j] = CombatActor{Player: a.Player, AttackVariant: a.AttackVariant, Endurance: a.Endurance}
		}
		ctx := InteractionContext{Candidates: [7]int8{-1, -1, -1, -1, -1, -1, r.Candidate}}
		got, e := StepOriginalBallContact(state, ctx)
		if e != nil {
			t.Fatalf("original call %d: %v", i, e)
		}
		s.Ball, s.Holder, s.LastTouch, s.SelectedHoop = got.State.Ball, got.State.Holder, got.State.LastTouch, got.State.SelectedHoop
		s.Auxiliary, s.SpecialHolder = got.State.Auxiliary, got.State.SpecialHolder
		for j, a := range got.State.Players {
			s.Actors[j].Player, s.Actors[j].AttackVariant, s.Actors[j].Endurance = a.Player, a.AttackVariant, a.Endurance
		}
		if s != r.After {
			t.Fatalf("original call %d: %s", i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
		}
		if got.Catch.Caught || len(got.Sounds) != 0 {
			t.Fatal("prop response invented catch or sound", i, got.Catch, got.Sounds)
		}
	}
}

func TestBallPropRecordedRuntime24VideoTicks(t *testing.T) {
	f, e := os.Open("testdata/ball-prop-runtime.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var s RuntimeState
	if e = json.NewDecoder(z).Decode(&s); e != nil {
		t.Fatal(e)
	}
	start := s.VideoFrames
	held := s.Held
	for i := 0; i < 24; i++ {
		r, e := StepVideo(s, held)
		if e != nil {
			t.Fatalf("tick %d video %d logic %d: %v", i, s.VideoFrames, s.LogicSteps, e)
		}
		s = r.State
	}
	if s.VideoFrames != start+24 {
		t.Fatal("changed video clock", s.VideoFrames, start)
	}
	t.Logf("original failing Go checkpoint advanced unchanged from %d to %d", start, s.VideoFrames)
}

func TestBallPropInvalidSpecialHolderIsAtomic(t *testing.T) {
	s := InteractionState{Holder: 6, SpecialHolder: -1, Ball: Ball{Action: BallShot}}
	s.Auxiliary[0].Action = 6
	got, e := StepOriginalBallContact(s, InteractionContext{Candidates: [7]int8{-1, -1, -1, -1, -1, -1, 4}})
	if e == nil || got.State != s {
		t.Fatal("unknown special holder did not reject atomically", got, e)
	}
}
