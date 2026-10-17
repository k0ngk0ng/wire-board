package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalDribbleAttackTrace(t *testing.T) {
	type state struct {
		observedAIControl
		Court, Difficulty uint8
		TeamHasHuman      bool
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After state
	}
	b, err := os.ReadFile("testdata/ai-dribble-attack.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 124 {
		t.Fatal("missing original dribble decisions", len(rows))
	}
	for _, r := range rows {
		b, a := r.Before, r.After
		h := HeadingNone
		if b.Heading != 255 {
			h = Heading(b.Heading / 32)
		}
		v := AIControlView{Actor: r.Actor, Action: Action(b.Action & 127), ActionEntered: b.Action&128 != 0, Heading: h, FacingLeft: b.Facing&128 != 0, HasBall: b.Holder == r.Actor, TeamHasHuman: b.TeamHasHuman, Players: b.Players, Position: Position{X: Fixed(b.X), Depth: Fixed(b.Depth), Height: Fixed(b.Height)}, Random: RandomState(binary.BigEndian.Uint32(b.Random[:])), Difficulty: b.Difficulty, CourtID: b.Court}
		got, err := StepOriginalAIControl(aiControlFromObservation(b.observedAIControl), v)
		if err != nil {
			t.Fatalf("frame %d actor %d: %v", r.Frame, r.Actor, err)
		}
		wantH := HeadingNone
		if a.Heading != 255 {
			wantH = Heading(a.Heading / 32)
		}
		if got.Control != aiControlFromObservation(a.observedAIControl) || got.Action != Action(a.Action&127) || got.ActionEntered != (a.Action&128 != 0) || got.Heading != wantH || got.FacingLeft != (a.Facing&128 != 0) || got.Random != RandomState(binary.BigEndian.Uint32(a.Random[:])) {
			t.Fatalf("frame %d actor %d before %+v\ngot %+v\nafter %+v", r.Frame, r.Actor, b, got, a)
		}
		if got.BindBall || got.Ball.Change || got.BindHoop != -1 || b.Request != a.Request || b.Holder != a.Holder || b.X != a.X || b.Depth != a.Depth || b.Height != a.Height {
			t.Fatal("unexpected world mutation", r.Frame)
		}
	}
	t.Logf("matched every %d original mode 17 call; no mode 16 calls in source", len(rows))
}

func TestOriginalDribbleObstacleBoundaries(t *testing.T) {
	// Fixed rule cases from $B15B, separate from the natural trace oracle.
	for _, tc := range []struct {
		name      string
		dx, dz    int
		left, hit bool
		parameter uint8
	}{
		{"ahead 47", 47, 0, false, true, 0}, {"ahead 48", 48, 0, false, false, 0},
		{"behind 15", -15, 0, false, true, 255}, {"behind 16", -16, 0, false, false, 0},
		{"left ahead 47", -47, 0, true, true, 0}, {"left ahead 48", -48, 0, true, false, 0},
		{"left behind 15", 15, 0, true, true, 255}, {"left behind depth edge", 15, 8, true, true, 255},
		{"depth positive edge", 10, -8, false, false, 0}, {"depth negative edge", 10, 8, false, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := AIControlView{Actor: 0, Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, FacingLeft: tc.left}
			v.Players[2] = Position{X: Fixed(200+tc.dx) * Pixel, Depth: Fixed(40+tc.dz) * Pixel}
			v.Players[3] = Position{X: 400 * Pixel}
			got := AIControlDecision{Control: AIControlState{PlanParameter: 33}}
			hit := originalDribbleObstacle(&got, v)
			if hit != tc.hit || hit && got.Control.PlanParameter != tc.parameter || !hit && got.Control.PlanParameter != 33 {
				t.Fatal(hit, got)
			}
		})
	}
}

func TestOriginalDribbleAttackTransitions(t *testing.T) {
	for _, mode := range []AIMode{16, 17} {
		v := AIControlView{Actor: 0, HasBall: true, Action: ActionDribbleRun, ActionEntered: true, Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, TeamHasHuman: true}
		v.Players[2] = Position{X: 400 * Pixel}
		v.Players[3] = v.Players[2]
		s := AIControlDecision{Control: AIControlState{Mode: mode, Entered: true, Counter: 0}, Action: v.Action, ActionEntered: true}
		if err := advanceOriginalDribbleAttack(&s, v, AIDribbleContext{}); err != nil || s.Control.Counter != 0 || s.Control.Mode != mode {
			t.Fatal(s, err)
		}
		v.TeamHasHuman = false
		if err := advanceOriginalDribbleAttack(&s, v, AIDribbleContext{}); err != nil || s.Control.Mode != 6 || s.Control.Entered {
			t.Fatal(s, err)
		}
		v.HasBall = false
		if err := advanceOriginalDribbleAttack(&s, v, AIDribbleContext{}); err != nil || s.Control.Mode != 2 {
			t.Fatal(s, err)
		}
	}
}
