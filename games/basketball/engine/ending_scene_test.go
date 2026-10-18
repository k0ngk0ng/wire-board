package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

type endingSceneSourceCall struct {
	After    []byte
	Pressed  Buttons
	Requests []EndingSceneRequest
	Reset    bool
	RNGCalls int
}
type endingSceneObservedCall struct {
	Before, AfterText, BeforePhase, After []byte
	TextRequests                          []EndingSceneRequest
	Sounds                                []uint8
	Reset                                 bool
	Frame, EndFrame                       int
	External                              []struct {
		Kind   string
		Frame  int
		Cycles uint64
	}
}
type endingSceneOracle struct {
	Initial  []byte
	Calls    []endingSceneSourceCall
	Observed []endingSceneObservedCall
}

func endingSceneRows(t *testing.T) endingSceneOracle {
	t.Helper()
	var r endingSceneOracle
	readPropAttackFixture(t, "testdata/ending-scene.json.gz", &r)
	if len(r.Calls) != 12745 || len(r.Observed) != 13370 {
		t.Fatal("incomplete logical/whole-console evidence", len(r.Calls), len(r.Observed))
	}
	return r
}
func endingSceneContextRAM(r []byte) EndingSceneContext {
	c := EndingSceneContext{Ending: endingPhaseContextRAM(r), Text: postresultTextProgressRAM(r), SecondUpload: postresultTextUploadsRAM(r, 0x30b)[1]}
	copy(c.PaletteOverrides[:], r[0x307:0x30b])
	return c
}
func TestOriginalEndingSceneContinuousSource(t *testing.T) {
	source := endingSceneRows(t)
	s := openingRAM(source.Initial)
	c := endingSceneContextRAM(source.Initial)
	for tick, row := range source.Calls {
		// External contract: NMI consumes headers only; no expected state reload.
		c.Ending.Upload.Address = 0xff00 | c.Ending.Upload.Address&255
		c.SecondUpload.Address = 0xff00 | c.SecondUpload.Address&255
		s.Transition.CameraScroll[0], s.Transition.CameraScroll[1] = s.Transition.CameraScroll[2], s.Transition.CameraScroll[3]
		s.Projection = resetOriginalWorldDrawList(s.Projection)
		pressed := Buttons(0)
		if s.Transition.Match.Phase == 9 && s.Transition.Match.Entered {
			pressed = ButtonStart
		}
		if pressed != row.Pressed {
			t.Fatal("input", tick)
		}
		got, e := StepOriginalEndingScene(s, c, pressed)
		if e != nil {
			t.Fatal(tick, e)
		}
		want := openingRAM(row.After)
		if got.State != want {
			t.Fatalf("call%d %s", tick, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(want), "state"))
		}
		if got.Context != endingSceneContextRAM(row.After) {
			t.Fatal("context", tick, got.Context, endingSceneContextRAM(row.After))
		}
		if got.Reset != row.Reset || len(got.Requests) != len(row.Requests) || len(row.Requests) > 0 && !reflect.DeepEqual(got.Requests, row.Requests) {
			t.Fatal("ordered requests", tick, got.Requests, row.Requests)
		}
		if row.RNGCalls != 0 || got.State.Transition.Random != s.Transition.Random {
			t.Fatal("outer RNG", tick)
		}
		if tick%64 == 0 || got.Reset {
			data, e := json.Marshal(struct {
				State   OpeningState
				Context EndingSceneContext
			}{s, c})
			if e != nil {
				t.Fatal(e)
			}
			var restored struct {
				State   OpeningState
				Context EndingSceneContext
			}
			if e = json.Unmarshal(data, &restored); e != nil {
				t.Fatal(e)
			}
			replay, e := StepOriginalEndingScene(restored.State, restored.Context, pressed)
			if e != nil || !reflect.DeepEqual(got, replay) {
				t.Fatal("restore", tick, e)
			}
		}
		s, c = got.State, got.Context
	}
	t.Log("matched12745 continuous FD0A+FC83 calls with declared queue consumption, no expected-RAM reload")
}
func TestOriginalEndingSceneWholeConsoleLogicalBoundaries(t *testing.T) {
	source := endingSceneRows(t)
	ordinary, resources, resets := 0, 0, 0
	for i, row := range source.Observed {
		s, c := openingRAM(row.Before), endingSceneContextRAM(row.Before)
		got, e := StepOriginalEndingScene(s, c, Buttons(row.Before[4]))
		if e != nil {
			t.Fatal(i, e)
		}
		// All captured calls validate the text progress entering the phase. Requests
		// issued by actors must wait until the next outer FD0A call.
		if got.Context.Text != postresultTextProgressRAM(row.AfterText) {
			t.Fatal("text order", i)
		}
		if got.Reset != row.Reset || got.State.Transition.Random != s.Transition.Random {
			t.Fatal("reset/RNG", i)
		}
		for j, want := range row.TextRequests {
			if j >= len(got.Requests) || got.Requests[j] != want {
				t.Fatal("text requests must precede presentation", i, j)
			}
		}
		if row.Reset {
			resets++
			continue
		} // F79D capture has no final RAM snapshot.
		if len(row.External) > 0 {
			resources++
			continue
		} // Explicit fade/background/NMI boundary; not kernel parity.
		want := openingRAM(row.After)
		if got.State != want {
			t.Fatalf("observed%d frame%d: %s", i, row.Frame, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(want), "state"))
		}
		if got.Context != endingSceneContextRAM(row.After) {
			t.Fatal("observed context", i, got.Context, endingSceneContextRAM(row.After))
		}
		requests := append([]EndingSceneRequest(nil), row.TextRequests...)
		for _, sound := range row.Sounds {
			requests = append(requests, EndingSceneRequest{Kind: EndingScenePresentation, Presentation: EndingPhaseRequest{Kind: EndingSound, Value: sound}})
		}
		if len(got.Requests) != len(requests) || len(requests) > 0 && !reflect.DeepEqual(got.Requests, requests) {
			t.Fatal("observed request order", i, got.Requests, requests)
		}
		ordinary++
	}
	if ordinary+resources+resets != 13370 || resets != 1 {
		t.Fatal("observation coverage")
	}
	t.Logf("whole-console exactlogical calls%d, declared resource-boundary calls%d, reset boundaries%d; textprogress checked forall", ordinary, resources, resets)
}
func TestOriginalEndingSceneFailuresAtomic(t *testing.T) {
	source := endingSceneRows(t)
	s := openingRAM(source.Initial)
	c := endingSceneContextRAM(source.Initial)
	cases := []struct {
		State   OpeningState
		Context EndingSceneContext
	}{{s, c}, {s, c}, {s, c}}
	cases[0].Context.Ending.TextActive = 0
	cases[0].Context.Ending.Actor.TextRequestFlags = 128
	cases[0].Context.Ending.Actor.TextID = 127
	cases[1].State.Transition.Match.Phase = 11
	cases[2].State.Transition.Match.Phase = 10
	cases[2].State.Transition.Match.Entered = false
	cases[2].State.Transition.Objects[0].Action = 127
	cases[2].State.Transition.Objects[0].Entered = false
	for i, v := range cases {
		got, e := StepOriginalEndingScene(v.State, v.Context, 0)
		if e == nil || got.State != v.State || got.Context != v.Context || got.Reset || len(got.Requests) != 0 {
			t.Fatal("non-atomic error", i, e)
		}
	}
}
