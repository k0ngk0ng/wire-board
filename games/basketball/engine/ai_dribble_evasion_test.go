package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalAIDribbleEvasion(t *testing.T) {
	data, err := os.ReadFile("testdata/ai-dribble-evasion.json")
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		observedAIControl
		Team, Court, JumpPhase, ShotCycle uint8
		TeamHasHuman                      bool
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After state
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatal("missing original mode-26 calls", len(rows))
	}
	for _, r := range rows {
		b, a := r.Before, r.After
		heading := HeadingNone
		if b.Heading != 255 {
			heading = Heading(b.Heading / 32)
		}
		v := AIControlView{Actor: r.Actor, Action: Action(b.Action & 127), ActionEntered: b.Action&128 != 0, Heading: heading, FacingLeft: b.Facing&128 != 0, Team: b.Team, TeamHasHuman: b.TeamHasHuman, Position: Position{X: Fixed(b.X), Depth: Fixed(b.Depth), Height: Fixed(b.Height)}, Random: RandomState(binary.BigEndian.Uint32(b.Random[:]))}
		v.CourtID = b.Court
		got, err := StepOriginalAIControl(aiControlFromObservation(b.observedAIControl), v)
		if err != nil {
			t.Fatalf("frame %d actor %d: %v", r.Frame, r.Actor, err)
		}
		wantHeading := HeadingNone
		if a.Heading != 255 {
			wantHeading = Heading(a.Heading / 32)
		}
		phase := b.JumpPhase
		if got.ShotPlan.ChangeJumpPhase {
			phase = uint8(got.ShotPlan.JumpPhase)
		}
		if got.Control != aiControlFromObservation(a.observedAIControl) || got.Action != Action(a.Action&127) || got.ActionEntered != (a.Action&128 != 0) || got.Heading != wantHeading || got.FacingLeft != (a.Facing&128 != 0) || got.Random != RandomState(binary.BigEndian.Uint32(a.Random[:])) || phase != a.JumpPhase {
			t.Fatalf("frame %d actor %d\nbefore %+v\ngot %+v phase %d\nwant %+v", r.Frame, r.Actor, b, got, phase, a)
		}
		if b.ShotCycle != a.ShotCycle || b.Request != a.Request || b.Holder != a.Holder || b.X != a.X || b.Depth != a.Depth || b.Height != a.Height || got.BindBall || got.BindHoop != -1 || got.Ball.Change || got.ShotPlan.ChangeShotCycle || got.ShotPlan.ClearDepthVelocity {
			t.Fatal("unexpected mutation", r.Frame, got)
		}
	}
	t.Log("matched all 5 original mode-26 calls including random selection and jump command")
}

func TestOriginalAIDribbleEvasionEntryAndFailure(t *testing.T) {
	for actor := uint8(0); actor < 4; actor++ {
		for _, entered := range []bool{false, true} {
			v := AIControlView{Actor: actor, Team: 7, TeamHasHuman: true, Position: Position{X: 200 * Pixel, Depth: 80 * Pixel}}
			got := AIControlDecision{Control: AIControlState{Mode: 26, Entered: entered, Counter: 31, PlanParameter: 1}, Random: 0x12345678, Action: ActionDribbleRun, ActionEntered: true}
			if err := advanceOriginalDribbleEvasion(&got, v, AIEvasionContext{Court: 1}); err != nil {
				t.Fatal(err)
			}
			want := HeadingDownRight
			if actor >= 2 {
				want = HeadingDownLeft
			}
			if got.Random != 0x12345678 || got.Control.Counter != 31 || got.Control.Mode != AIPause || got.Control.Plan != 17 || got.ActionEntered || got.Heading != want {
				t.Fatal(actor, entered, got)
			}
		}
	}
	for _, prior := range []uint8{0, 128} {
		v := AIControlView{Actor: 0, Team: 7, TeamHasHuman: true, Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}
		initial := AIControlDecision{Control: AIControlState{Mode: 26, PlanParameter: prior}, Random: 0x12345678}
		got := initial
		if err := advanceOriginalDribbleEvasion(&got, v, AIEvasionContext{}); err != nil {
			t.Fatal(err)
		}
		wantRandom, value := NextRandom(initial.Random)
		if got.Random != wantRandom || got.Control.PlanParameter != (value&1) {
			t.Fatal("human team did not use original team-zero table", got, value)
		}
	}
	initial := AIControlDecision{Control: AIControlState{Mode: 26, Entered: true, PlanParameter: 14}, Random: 0x12345678}
	got := initial
	if err := advanceOriginalDribbleEvasion(&got, AIControlView{}, AIEvasionContext{}); err == nil || got != initial {
		t.Fatal("unrestored plan must fail atomically", got, err)
	}
}
