package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedInteractions struct {
	Players                                                   [4]observedCombatActor
	Ball                                                      observedBall
	Candidates                                                [7]uint8
	Court, Holder, LastTouch, PreviousHolder, ExcludedCatcher uint8
	Controls, Attachments                                     [4]uint8
	AuxiliaryOwners                                           [2]uint8
	HoopOwners                                                [6]uint8
	Random                                                    [4]byte
	AI                                                        [28]uint8
}

func (o observedInteractions) state() InteractionState {
	s := InteractionState{Ball: ballFromObservation(o.Ball), Holder: int8(o.Holder), LastTouch: int8(o.LastTouch), SelectedHoop: int8(o.Ball.Hoop)}
	for i, p := range o.Players {
		s.Players[i] = CombatActor{Player: playerFromObservation(p.observedPlayer), AttackVariant: p.Variant, AttackGrade: p.AttackGrade, Endurance: p.Resistance}
	}
	return s
}

func (o observedInteractions) context() InteractionContext {
	c := InteractionContext{CourtID: o.Court, Controls: o.Controls, Attachments: o.Attachments, PreviousHolder: int8(o.PreviousHolder), ExcludedCatcher: int8(o.ExcludedCatcher)}
	for i, v := range o.Candidates {
		c.Candidates[i] = int8(v)
	}
	for i, v := range o.AuxiliaryOwners {
		c.AuxiliaryOwners[i] = int8(v)
	}
	for i, v := range o.HoopOwners {
		c.HoopOwners[i] = int8(v)
	}
	return c
}

func TestOriginalCompleteInteractionPhase(t *testing.T) {
	data, err := os.ReadFile("testdata/interactions.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame         int
		Before, After observedInteractions
		Sounds        []uint8
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1197 {
		t.Fatal("missing complete response phases", len(rows))
	}
	transfers, sounds := 0, 0
	for _, r := range rows {
		got, err := StepOriginalInteractions(r.Before.state(), r.Before.context())
		if err != nil || got.State != r.After.state() {
			t.Fatalf("frame %d\nbefore %+v\ncontext %+v\ngot %+v\nwant %+v\nerror %v", r.Frame, r.Before.state(), r.Before.context(), got, r.After.state(), err)
		}
		if len(got.Sounds) != len(r.Sounds) {
			t.Fatal("wrong sounds", r.Frame, got.Sounds, r.Sounds)
		}
		for i, s := range got.Sounds {
			if s.ID != r.Sounds[i] {
				t.Fatal("wrong sound order", r.Frame, got.Sounds, r.Sounds)
			}
		}
		if r.Before.context() != r.After.context() || r.Before.Random != r.After.Random || r.Before.AI != r.After.AI {
			t.Fatal("unexpected mutation outside interactions", r.Frame)
		}
		if got.State.Holder != int8(r.Before.Holder) {
			transfers++
		}
		sounds += len(got.Sounds)
	}
	if transfers != 11 || sounds != 9 {
		t.Fatal(transfers, sounds)
	}
	t.Log("matched all 1197 original combat+catch phases, including 11 ownership transfers and 9 sound requests")
}

func TestOriginalPlayerCatchFlags(t *testing.T) {
	data, err := os.ReadFile("testdata/player-catch-flags.json")
	if err != nil {
		t.Fatal(err)
	}
	var flags []uint8
	if err = json.Unmarshal(data, &flags); err != nil {
		t.Fatal(err)
	}
	if len(flags) != 96 {
		t.Fatal(len(flags))
	}
	for action, flag := range flags {
		if originalPlayerCatchFlags(Action(action)) != flag&16 {
			t.Fatalf("original catch flag differs for action %x", action)
		}
	}
	for action := 96; action < 256; action++ {
		if originalPlayerCatchFlags(Action(action)) != 0 {
			t.Fatal("attack or invalid action can catch", action)
		}
	}
}

func interactionContext() InteractionContext {
	return InteractionContext{Candidates: [7]int8{-1, -1, -1, -1, -1, -1, -1}, ExcludedCatcher: -1, PreviousHolder: 6, AuxiliaryOwners: [2]int8{-1, -1}, HoopOwners: [6]int8{-1, -1, -1, -1, -1, -1}}
}

func TestInteractionsRecomputeCatchEligibilityAfterHit(t *testing.T) {
	s := InteractionState{Holder: 6, LastTouch: 3, Ball: Ball{Action: BallPass, Entered: true}}
	s.Players[0] = CombatActor{AttackVariant: 8, AttackGrade: 3}
	s.Players[1] = CombatActor{Player: Player{Action: ActionRun, Entered: true}, Endurance: 255}
	c := interactionContext()
	c.Candidates[0] = 1
	c.Candidates[6] = 1
	got, err := StepOriginalInteractions(s, c)
	if err != nil || got.State.Players[1].Player.Action != Action(0x48) || got.State.Holder != 6 || got.State.LastTouch != 3 || got.State.Ball != s.Ball || got.Catch.Caught {
		t.Fatal("old running-catch flag survived a hit", got, err)
	}
	// A later recipient cannot take a held ball before the hurt actor's action
	// phase actually drops it, even when the ball-contact candidate is valid.
	s.Holder = 1
	s.Ball.Action = BallHeld
	c.Candidates[6] = 2
	got, err = StepOriginalInteractions(s, c)
	if err != nil || got.State.Holder != 1 || got.State.Ball != s.Ball || got.Catch.Caught {
		t.Fatal("combat released ball before the hurt action", got, err)
	}
}

func TestInteractionsCatchWritesOnlyOriginalCommands(t *testing.T) {
	s := InteractionState{Holder: 6, LastTouch: 1, Ball: Ball{Action: BallDropped, Entered: true, Body: Body{Position: Position{X: 123, Height: 456}}}}
	s.Players[3].Player = Player{Action: ActionIdle, Entered: true, Animation: AnimationState{Elapsed: 7}}
	c := interactionContext()
	c.Candidates[6] = 3
	got, err := StepOriginalInteractions(s, c)
	if err != nil || got.State.Holder != 3 || got.State.LastTouch != 3 || got.State.Players[3].Player.Action != ActionReceiveBall || got.State.Players[3].Player.Entered || got.State.Ball.Action != BallHeld || got.State.Ball.Entered || got.State.Ball.Body != s.Ball.Body || got.State.Players[3].Player.Animation != s.Players[3].Player.Animation || len(got.Sounds) != 0 {
		t.Fatal(got, err)
	}
	s.Ball.Action = BallPass
	s.Players[3].Player.Action = Action(0x20)
	got, err = StepOriginalInteractions(s, c)
	if err != nil || got.State.Holder != 6 || got.State.LastTouch != 3 || got.State.Players[3].Player.Action != Action(0x48) || got.State.Ball.Action != BallKnockback || got.Catch.Caught || len(got.Sounds) != 0 {
		t.Fatal("occupied-hands pass changed ownership", got, err)
	}
}

func TestInteractionsUnsupportedCatchRollsBackCombat(t *testing.T) {
	s := InteractionState{Holder: 6, LastTouch: 3, SelectedHoop: 8, Ball: Ball{Action: 13}}
	s.Players[0] = CombatActor{AttackVariant: 8, AttackGrade: 3}
	s.Players[1].Endurance = 200
	c := interactionContext()
	c.Candidates[0] = 1
	// First prove that this prefix changes gameplay state and emits a sound.
	prefix, err := StepOriginalInteractions(s, c)
	if err != nil || prefix.State == s || len(prefix.Sounds) != 1 {
		t.Fatal("rollback fixture never executed earlier combat", prefix, err)
	}
	// Object 13 is outside the original world (0..12). Ball 13 contacting a
	// real player is now supported and must not be used as the failure trigger.
	c.Candidates[6] = 13
	got, err := StepOriginalInteractions(s, c)
	if err == nil || got.State != s || len(got.Sounds) != 0 {
		t.Fatal("invalid catch candidate retained earlier combat effects", got, err)
	}
}

func TestInteractionsSpecialBallHitRetainsEarlierCombat(t *testing.T) {
	for _, target := range []int8{1, 2} {
		name := "same recipient"
		if target == 2 {
			name = "different recipient"
		}
		t.Run(name, func(t *testing.T) {
			s := InteractionState{Holder: 6, LastTouch: 3, SelectedHoop: 8, Ball: Ball{Action: 13, Entered: true}}
			s.Players[0] = CombatActor{AttackVariant: 8, AttackGrade: 3}
			s.Players[1] = CombatActor{Player: Player{Action: ActionIdle, Entered: true}, Endurance: 200}
			s.Players[2] = CombatActor{Player: Player{Action: ActionRun, Entered: true, FacingLeft: true}, Endurance: 91}
			c := interactionContext()
			c.CourtID = 1
			c.Candidates[0], c.Candidates[6] = 1, target
			got, err := StepOriginalInteractions(s, c)
			want := s
			// Grade-three ordinary impact removes 128 endurance first.
			want.Players[1].Endurance = 72
			want.Players[1].Player.Action, want.Players[1].Player.Entered = 0x48, false
			want.Players[1].Player.Body.Velocity.X = 3 * Pixel
			// The later special-ball impact overrides movement/action, preserving
			// the endurance resulting from the earlier combat, possession and ball.
			receiver := &want.Players[target].Player
			receiver.Action, receiver.Entered = 0x49, false
			vx := 4 * Pixel
			if target == 2 {
				vx = -4 * Pixel
			}
			receiver.Body.Velocity = Velocity{X: vx, Height: 8 * Pixel}
			receiver.Body.Gravity = Pixel
			if err != nil || got.State != want || got.Catch.Caught {
				t.Fatal("supported special-ball response lost combat ordering", got, want, err)
			}
			sounds := [2]CombatSound{{Source: 0, Target: 1, ID: 19}, {Source: 6, Target: uint8(target), ID: 21}}
			if len(got.Sounds) != len(sounds) || got.Sounds[0] != sounds[0] || got.Sounds[1] != sounds[1] {
				t.Fatal("wrong combat/special-ball sound order", got.Sounds, sounds)
			}
		})
	}
}
