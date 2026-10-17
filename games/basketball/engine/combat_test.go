package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedCombatActor struct {
	observedPlayer
	Variant, AttackGrade, Resistance uint8
}
type observedCombatState struct {
	Players                                                           [4]observedCombatActor
	Candidates                                                        [6]uint8
	Court, Holder, PreviousHolder, LastTouch, BallAction, BallRequest uint8
	HoopOwners                                                        [6]uint8
	Random                                                            [4]byte
	AI                                                                [28]uint8
}

func (o observedCombatState) state() CombatState {
	var result CombatState
	for i, p := range o.Players {
		result.Players[i] = CombatActor{Player: playerFromObservation(p.observedPlayer), AttackVariant: p.Variant, AttackGrade: p.AttackGrade, Endurance: p.Resistance}
	}
	return result
}

func TestOriginalCompleteCombatPhase(t *testing.T) {
	data, err := os.ReadFile("testdata/combat-phases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame         int
		Before, After observedCombatState
		Sounds        []uint8
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1197 {
		t.Fatal("missing complete combat phases", len(rows))
	}
	hits := 0
	for _, r := range rows {
		var candidates [6]int8
		for i, c := range r.Before.Candidates {
			candidates[i] = int8(c)
			if candidates[i] >= 0 {
				hits++
			}
		}
		got, err := StepOriginalCombat(r.Before.state(), CombatContext{CourtID: r.Before.Court, Candidates: candidates})
		if err != nil || got.State != r.After.state() {
			t.Fatalf("frame %d\nbefore %+v\ngot %+v\nwant %+v\nerror %v", r.Frame, r.Before.state(), got, r.After.state(), err)
		}
		if len(got.Sounds) != len(r.Sounds) {
			t.Fatal("wrong sound count", r.Frame, got.Sounds, r.Sounds)
		}
		for i, sound := range got.Sounds {
			if sound.ID != r.Sounds[i] {
				t.Fatal("wrong sound order", r.Frame, got.Sounds, r.Sounds)
			}
		}
		unchanged := r.Before
		unchanged.Players = r.After.Players
		if unchanged != r.After {
			t.Fatal("original combat mutated unsupported world context", r.Frame)
		}
	}
	if hits != 9 {
		t.Fatal("missing contact responses", hits)
	}
	t.Log("matched all 1197 original combat phases, 9 actual ordinary-player contacts, and sound requests")
}

func oneCombatCandidate(source, target int8) CombatContext {
	c := CombatContext{Candidates: [6]int8{-1, -1, -1, -1, -1, -1}}
	c.Candidates[source] = target
	return c
}

func TestOriginalCombatSaturationAndGroundFraction(t *testing.T) {
	s := CombatState{}
	s.Players[0] = CombatActor{AttackVariant: 8, AttackGrade: 3}
	s.Players[1] = CombatActor{Endurance: 127, Player: Player{Action: ActionWalk, Entered: true, Airborne: true, Request: ActionRequest{Action: ActionPass, Pending: true}, Body: Body{Position: Position{X: 12345, Height: 200}, Velocity: Velocity{Depth: 51, Height: 77}, Gravity: 99}}}
	got, err := StepOriginalCombat(s, oneCombatCandidate(0, 1))
	p := got.State.Players[1]
	if err != nil || p.Endurance != 0 || p.Player.Action != Action(0x48) || p.Player.Entered || p.Player.Body.Velocity.X != 3*Pixel || p.Player.Body.Velocity.Depth != 51 || p.Player.Body.Velocity.Height != 77 || p.Player.Body.Gravity != 99 || p.Player.Body.Position != s.Players[1].Player.Body.Position || !p.Player.Airborne || p.Player.Request != s.Players[1].Player.Request {
		t.Fatal(got, err)
	}
	if len(got.Sounds) != 1 || got.Sounds[0].ID != 19 {
		t.Fatal(got.Sounds)
	}
}

func TestOriginalCombatLaunchAndFrameFeedback(t *testing.T) {
	for _, mirrored := range []bool{false, true} {
		s := CombatState{}
		s.Players[0] = CombatActor{AttackVariant: 14, AttackGrade: 3, Player: Player{FacingLeft: mirrored, Animation: AnimationState{Cursor: 254, Elapsed: 9}}}
		s.Players[1].Player.Body = Body{Position: Position{Depth: 72 * Pixel}, Velocity: Velocity{X: 123, Depth: 456, Height: 789}, Gravity: 44}
		ctx := oneCombatCandidate(0, 1)
		ctx.CourtID = 2
		got, err := StepOriginalCombat(s, ctx)
		wantX := -6 * Pixel
		if mirrored {
			wantX = -wantX
		}
		p := got.State.Players[1]
		if err != nil || p.Endurance != 255 || p.Player.Action != Action(0x49) || p.Player.Airborne || p.Player.Body.Velocity != (Velocity{X: wantX, Depth: -Pixel, Height: 10 * Pixel}) || p.Player.Body.Gravity != Pixel || got.State.Players[0].Player.Animation.Cursor != 0 || got.State.Players[0].Player.Animation.Elapsed != 0 {
			t.Fatal(got, err)
		}
	}
}

func TestOriginalCombatRecoilAsymmetry(t *testing.T) {
	for _, velocity := range []Fixed{100, -5} {
		s := CombatState{}
		s.Players[0] = CombatActor{AttackVariant: 1, AttackGrade: 3, Player: Player{Body: Body{Velocity: Velocity{X: velocity}}, Animation: AnimationState{Elapsed: 7}}}
		got, err := StepOriginalCombat(s, oneCombatCandidate(0, 1))
		if err != nil {
			t.Fatal(err)
		}
		wantVelocity, wantClock := Fixed(-384), uint8(255)
		if velocity < 0 {
			wantVelocity, wantClock = 384, 7
		}
		if got.State.Players[0].Player.Body.Velocity.X != wantVelocity || got.State.Players[0].Player.Animation.Elapsed != wantClock {
			t.Fatal(got)
		}
	}
}

func TestOriginalCombatForcedLaunchAndAtomicFailure(t *testing.T) {
	for _, special := range []bool{false, true} {
		s := CombatState{}
		s.Players[0] = CombatActor{AttackVariant: 8, AttackGrade: 7}
		if special {
			s.Players[0].AttackGrade = 0
			s.Players[1].Player.Action = Action(0x47)
		}
		got, err := StepOriginalCombat(s, oneCombatCandidate(0, 1))
		if err != nil || got.State.Players[1].Player.Action != Action(0x49) {
			t.Fatal(got, err)
		}
	}
	s := CombatState{}
	s.Players[0] = CombatActor{AttackVariant: 8, AttackGrade: 3}
	ctx := oneCombatCandidate(0, 1)
	ctx.Candidates[2] = 13
	got, err := StepOriginalCombat(s, ctx)
	if err == nil || got.State != s || len(got.Sounds) != 0 {
		t.Fatal("invalid target must fail atomically", got, err)
	}
	ctx = oneCombatCandidate(4, 1)
	got, err = StepOriginalCombat(s, ctx)
	if err == nil || got.State != s {
		t.Fatal("unrestored prop attack must fail atomically", got, err)
	}
}
