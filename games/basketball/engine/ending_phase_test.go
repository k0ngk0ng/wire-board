package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

type endingPhaseOracle struct {
	Label         string
	Before, After []byte
	Requests      []EndingPhaseRequest
	Reset         bool
	RNGCalls      int
}

func endingPhaseRows(t *testing.T) []endingPhaseOracle {
	t.Helper()
	var rows []endingPhaseOracle
	readPropAttackFixture(t, "testdata/ending-phase.json.gz", &rows)
	if len(rows) != 1798 {
		t.Fatal("incomplete ending phase evidence", len(rows))
	}
	return rows
}
func endingPhaseContextRAM(r []byte) EndingPhaseContext {
	c := EndingPhaseContext{Actor: resultActorContextRAM(r), TextActive: r[0x6ac], MachineControl: r[0x4d], Background: r[0x3a0], FadeMode: r[0x301], FadeDelay: r[0x302]}
	copy(c.CHRPages[:], r[0x51:0x55])
	c.Upload.Address = uint16(r[0x30b])<<8 | uint16(r[0x30c])
	c.Upload.Length = r[0x30d]
	copy(c.Upload.Data[:], r[0x30e:0x32e])
	return c
}

// Failure inventory precedes implementation in ENDING_PHASE_VERIFICATION.md.
func TestOriginalEndingPhaseSource(t *testing.T) {
	for _, r := range endingPhaseRows(t) {
		before := openingRAM(r.Before)
		c := endingPhaseContextRAM(r.Before)
		got, e := StepOriginalEndingPhase(before, c, Buttons(r.Before[4]))
		if e != nil {
			t.Fatal(r.Label, e)
		}
		want := openingRAM(r.After)
		if got.State != want {
			t.Fatalf("%s: %s", r.Label, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(want), "state"))
		}
		if got.Context != endingPhaseContextRAM(r.After) {
			t.Fatal(r.Label, "context", got.Context, endingPhaseContextRAM(r.After))
		}
		if got.Reset != r.Reset || len(got.Requests) != len(r.Requests) || len(r.Requests) > 0 && !reflect.DeepEqual(got.Requests, r.Requests) {
			t.Fatal(r.Label, "ordered requests/reset", got.Requests, r.Requests, got.Reset, r.Reset)
		}
		if got.State.Transition.Random != before.Transition.Random || r.RNGCalls != 0 {
			t.Fatal(r.Label, "unexpected RNG draw")
		}
		if EndingTimer(got.State) != uint16(r.After[0x6d7])|uint16(r.After[0x6d8])<<8 {
			t.Fatal(r.Label, "timer alias")
		}
		if EndingCredits(got.State) != [5]uint8(r.After[0x6da:0x6df]) {
			t.Fatal(r.Label, "credits alias")
		}
	}
	t.Log("matched all1798 original phase calls, stopping before actors/reset, no RNG")
}
func TestOriginalEndingPhaseRestore(t *testing.T) {
	for _, r := range endingPhaseRows(t) {
		before := struct {
			State   OpeningState
			Context EndingPhaseContext
		}{openingRAM(r.Before), endingPhaseContextRAM(r.Before)}
		raw, e := json.Marshal(before)
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
		a, e := StepOriginalEndingPhase(before.State, before.Context, Buttons(r.Before[4]))
		if e != nil {
			t.Fatal(e)
		}
		b, e := StepOriginalEndingPhase(restored.State, restored.Context, Buttons(r.Before[4]))
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal(r.Label, "restore", e)
		}
	}
}
func TestOriginalEndingPhaseRejectsUnknownAtomically(t *testing.T) {
	rows := endingPhaseRows(t)
	s := openingRAM(rows[0].Before)
	s.Transition.Scene = 7
	c := endingPhaseContextRAM(rows[0].Before)
	cases := []OpeningState{s, s, s, s}
	cases[0].Transition.Scene = 1
	cases[1].Transition.Match.Phase = 11
	cases[2].Transition.Match.Phase = 5
	cases[2].Transition.Match.Entered = true
	cases[2].Transition.Objects[2].Action = 12
	cases[2].Transition.Passages[0] = 255
	cases[2].Transition.Passages[1] = 15
	cases[3].Transition.Match.Phase = 5
	cases[3].Transition.Match.Entered = true
	cases[3].Transition.Objects[2].Action = 0
	cases[3].Transition.Passages[0] = 0
	cases[3].Transition.Passages[1] = 0
	for i, v := range cases {
		got, e := StepOriginalEndingPhase(v, c, 0)
		if e == nil || got.State != v || got.Context != c || got.Reset || len(got.Requests) != 0 {
			t.Fatal(i, "unsupported state not atomic", e)
		}
	}
}
