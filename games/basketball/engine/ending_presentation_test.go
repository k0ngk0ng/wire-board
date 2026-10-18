package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

type endingPresentationCall struct {
	After                     []byte
	Pressed                   Buttons
	Requests                  []EndingPhaseRequest
	Actors                    []uint8
	RNGCalls, ProjectionCalls int
	Reset                     bool
}
type endingPresentationOracle struct {
	ConfirmationBefore, Initial []byte
	ConfirmationTail            endingPresentationCall
	Calls                       []endingPresentationCall
}

func TestOriginalEndingPresentationContinuousSource(t *testing.T) {
	var source endingPresentationOracle
	readPropAttackFixture(t, "testdata/ending-presentation.json.gz", &source)
	if len(source.Calls) != 12745 {
		t.Fatal("incomplete full ending chain", len(source.Calls))
	}
	// Execute the actual confirmation actor tail once, not a canned post-tail seed.
	s := openingRAM(source.ConfirmationBefore)
	c := endingPhaseContextRAM(source.ConfirmationBefore)
	var tailRequests []EndingPhaseRequest
	for actor := uint8(0); actor < 4; actor++ {
		r, e := StepOriginalResultActor(s, c.Actor, actor)
		if e != nil {
			t.Fatal("confirmation tail", actor, e)
		}
		s, c.Actor = r.State, r.Context
		for _, sound := range r.SoundIDs {
			tailRequests = append(tailRequests, EndingPhaseRequest{Kind: EndingSound, Value: sound})
		}
	}
	if s != openingRAM(source.Initial) || c != endingPhaseContextRAM(source.Initial) {
		t.Fatal("confirmation tail differs from original")
	}
	if len(tailRequests) != len(source.ConfirmationTail.Requests) || len(tailRequests) > 0 && !reflect.DeepEqual(tailRequests, source.ConfirmationTail.Requests) {
		t.Fatal("confirmation request order")
	}
	phases := []MatchPhase{}
	actorCalls := 0
	for tick, row := range source.Calls {
		// Declared outer boundary only: camera copy and renderer D063 finalization.
		s.Transition.CameraScroll[0], s.Transition.CameraScroll[1] = s.Transition.CameraScroll[2], s.Transition.CameraScroll[3]
		s.Projection = resetOriginalWorldDrawList(s.Projection)
		if len(phases) == 0 || phases[len(phases)-1] != s.Transition.Match.Phase {
			phases = append(phases, s.Transition.Match.Phase)
		}
		pressed := Buttons(0)
		if s.Transition.Match.Phase == 9 && s.Transition.Match.Entered {
			pressed = ButtonStart
		}
		if pressed != row.Pressed {
			t.Fatal("original input mismatch", tick)
		}
		before := s
		got, e := StepOriginalEndingPresentation(s, c, pressed)
		if e != nil {
			t.Fatal(tick, e)
		}
		want := openingRAM(row.After)
		if got.State != want {
			t.Fatalf("call %d: %s", tick, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(want), "state"))
		}
		if got.Context != endingPhaseContextRAM(row.After) {
			t.Fatal("context", tick, got.Context, endingPhaseContextRAM(row.After))
		}
		if got.Reset != row.Reset || len(got.Requests) != len(row.Requests) || len(row.Requests) > 0 && !reflect.DeepEqual(got.Requests, row.Requests) {
			t.Fatal("requests/reset", tick, got.Requests, row.Requests)
		}
		if row.RNGCalls != 0 || got.State.Transition.Random != before.Transition.Random {
			t.Fatal("unexpected ending RNG", tick)
		}
		if row.ProjectionCalls != len(row.Actors) || got.Reset && len(row.Actors) != 0 || !got.Reset && !reflect.DeepEqual(row.Actors, []uint8{0, 1, 2, 3}) {
			t.Fatal("wrong source actor/projection boundary", tick)
		}
		actorCalls += len(row.Actors)
		if tick%64 == 0 || got.Reset {
			raw, e := json.Marshal(struct {
				State   OpeningState
				Context EndingPhaseContext
			}{s, c})
			if e != nil {
				t.Fatal(e)
			}
			var restored struct {
				State   OpeningState
				Context EndingPhaseContext
			}
			if e = json.Unmarshal(raw, &restored); e != nil {
				t.Fatal(e)
			}
			replay, e := StepOriginalEndingPresentation(restored.State, restored.Context, pressed)
			if e != nil || !reflect.DeepEqual(got, replay) {
				t.Fatal("restore", tick, e)
			}
		}
		s, c = got.State, got.Context
	}
	if !reflect.DeepEqual(phases, []MatchPhase{0, 1, 2, 3, 4, 5, 6, 8, 9, 10, 7}) || actorCalls != 50976 {
		t.Fatal("ending choreography", phases, actorCalls)
	}
	t.Logf("matched12745 uninterrupted original ending calls, %d actor+projection calls; reset excluded tail", actorCalls)
}
func TestOriginalEndingPresentationFailureAtomic(t *testing.T) {
	var source endingPresentationOracle
	readPropAttackFixture(t, "testdata/ending-presentation.json.gz", &source)
	s := openingRAM(source.Initial)
	c := endingPhaseContextRAM(source.Initial)
	s.Transition.Match.Phase = 10
	s.Transition.Match.Entered = false
	s.Transition.Objects[0].Action = 127
	s.Transition.Objects[0].Entered = false
	got, e := StepOriginalEndingPresentation(s, c, 0)
	if e == nil || got.State != s || got.Context != c || len(got.Requests) != 0 || got.Reset {
		t.Fatal("actor failure committed phase state", e)
	}
}
