package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalMarkingTrace(t *testing.T) {
	type state struct {
		observedAIControl
		Court, Difficulty uint8
		PlayerActions     [4]Action
		TeamHasHuman      bool
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After state
	}
	b, err := os.ReadFile("testdata/ai-marking.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 181 {
		t.Fatal("missing original marking decisions", len(rows))
	}
	for _, r := range rows {
		b, a := r.Before, r.After
		h := HeadingNone
		if b.Heading != 255 {
			h = Heading(b.Heading / 32)
		}
		v := AIControlView{Actor: r.Actor, Action: Action(b.Action & 127), ActionEntered: b.Action&128 != 0, Heading: h, FacingLeft: b.Facing&128 != 0, HasBall: b.Holder == r.Actor, TeamHasHuman: b.TeamHasHuman, Players: b.Players, Position: Position{X: Fixed(b.X), Depth: Fixed(b.Depth), Height: Fixed(b.Height)}, Random: RandomState(binary.BigEndian.Uint32(b.Random[:])), Difficulty: b.Difficulty, PlayerActions: b.PlayerActions}
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
	t.Logf("matched every %d original mode 4 call; no mode 12 calls in source", len(rows))
}

func TestOriginalMarkingBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		targetX, targetDepth int
		special              bool
		human                bool
		mode                 AIMode
		action               Action
		heading              Heading
		left                 bool
	}{
		{"near bot", 247, 40, false, false, 19, ActionRun, HeadingRight, false},
		{"near human", 247, 40, false, true, 39, ActionRun, HeadingRight, false},
		{"exact stop boundary", 248, 40, false, false, 4, ActionWalk, HeadingRight, false},
		{"beyond stop boundary", 249, 40, false, false, 4, ActionRun, HeadingRight, false},
		{"depth alignment", 210, 48, false, false, 4, ActionWalk, HeadingUp, false},
		{"special retreat", 230, 40, true, false, 4, ActionWalk, HeadingLeft, false},
		{"special distance boundary", 264, 40, true, false, 4, ActionIdle, HeadingRight, false},
		{"special left", 170, 40, true, false, 4, ActionWalk, HeadingRight, true},
		{"special far left", 100, 40, true, false, 4, ActionIdle, HeadingLeft, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := AIControlView{Actor: 0, Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Action: ActionRun, ActionEntered: true, TeamHasHuman: tc.human}
			v.Players[2] = Position{X: Fixed(tc.targetX) * Pixel, Depth: Fixed(tc.targetDepth) * Pixel}
			if tc.special {
				v.PlayerActions[2] = 0x49
			}
			got := AIControlDecision{Control: AIControlState{Mode: 4, Entered: true, TrackedObject: 2, Destination: 48}, Action: ActionRun, ActionEntered: true}
			err := advanceOriginalMarking(&got, v)
			if err != nil || got.Control.Mode != tc.mode || got.Action != tc.action || got.Heading != tc.heading || got.FacingLeft != tc.left {
				t.Fatal(got, err)
			}
		})
	}
}
func TestOriginalMarkingMode12AndInvalidTarget(t *testing.T) {
	v := AIControlView{Actor: 1, Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Random: 12345}
	v.Players[2] = Position{X: 400 * Pixel, Depth: 40 * Pixel}
	got := AIControlDecision{Control: AIControlState{Mode: 12}, Action: ActionRun, ActionEntered: true, Random: v.Random}
	wantRandom, _ := NextRandom(v.Random)
	if err := advanceOriginalMarking(&got, v); err != nil || got.Control.TrackedObject != 2 || got.Control.Counter != 32 || got.Control.Plan != 12 || got.Random != wantRandom || got.ActionEntered {
		t.Fatal(got, err)
	}
	got.Control.TrackedObject = 6
	if err := advanceOriginalMarking(&got, v); err == nil {
		t.Fatal("invalid target accepted")
	}
}
