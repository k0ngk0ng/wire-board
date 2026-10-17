package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// World failures: entering captured handling without an actual Kind10 contact;
// exposing old holder/binding/request during motion; using facing instead of
// the entry pose for ball release; clearing binding before aux4's action7;
// resetting the escape countdown on each new idle animation.
func capturedWorldFromContact(t *testing.T, holder int8) (WorldLoopState, WorldLoopContext) {
	w, c := capturedWorldBeforeContact(t, holder, 0)
	pc := WorldPreparationContext{Actions: c.Actions, AttackGrades: c.AttackGrades, HumanTactics: c.HumanTactics, BackboardStrength: c.BackboardStrength}
	copy(pc.Kinds[:], c.ObjectKinds[:7])
	pc.Kinds[4] = 10
	got, e := StepWorldPreparation(w.Preparation, pc)
	if e != nil {
		t.Fatal(e)
	}
	if got.State.Contacts.Candidates[4] != 1 || got.State.Actions.Actors[1].Player.Action != ActionCapturedIdle || got.State.Actions.Auxiliary[0].Action != 6 || got.State.Actions.CaptureCounter != 4 {
		t.Fatal("actual Kind10 search/response did not capture", got.State.Contacts, got.State.Actions.Actors[1], got.State.Actions.Auxiliary[0])
	}
	w.Preparation = got.State
	w.Started = false
	return w, c
}

func capturedWorldBeforeContact(t *testing.T, holder int8, courtID uint8) (WorldLoopState, WorldLoopContext) {
	t.Helper()
	cfg := DefaultRuntimeConfig()
	cfg.Match.Court = courtID
	cfg.Match.Seed = 0x12345678
	runtime, e := NewRuntime(cfg)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; runtime.Playing == nil && n < 400; n++ {
		step, e := StepVideo(runtime, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		runtime = step.State
	}
	if runtime.Playing == nil {
		t.Fatal("opening did not finish")
	}
	w, c := runtime.Playing.World, runtime.Playing.Context
	c.Actions.Controls = [4]uint8{0xc0, 128, 0xc0, 0xc0}
	c.ObjectKinds[4] = 10
	a := &w.Preparation.Actions
	for i := range a.Actors {
		a.Actors[i].Player = Player{Action: ActionIdle, Entered: true, Heading: HeadingNone, Body: Body{Position: Position{X: Fixed(70+i*100) * Pixel, Depth: 40 * Pixel}}}
		a.PlayerLinks[i] = 0
	}
	a.Actors[1].Player.Body.Position.X = 200 * Pixel
	a.Actors[1].Player.Pose.Mirrored = true
	a.Auxiliary[0] = AuxiliaryState{Kind: 10, Action: 3, Entered: true, Owner: -1, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}, Animation: AnimationState{Cursor: 3}}
	a.Auxiliary[1].Body.Position.X = 460 * Pixel
	a.Auxiliary[1].Action = 0
	a.Auxiliary[1].Owner = -1
	a.Auxiliary[1].Attachment = 0
	a.SpecialHolder = -1
	a.CaptureCounter = 0
	a.Holder = holder
	a.Ball = Ball{Action: BallRest, Entered: true, Body: Body{Position: Position{X: 350 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: 77}}}
	if holder >= 0 && holder < 4 {
		a.Actors[holder].Player.Action = ActionDribbleIdle
		a.Ball.Action = BallHeld
	}
	w.Started = true
	return w, c
}

func TestWorldCapturedCommitsBindingAndDropBeforeMotion(t *testing.T) {
	for _, holder := range []int8{1, 0, 6} {
		w, c := capturedWorldFromContact(t, holder)
		r := WorldActionStep{State: w.Preparation.Actions}
		beforeBall := r.State.Ball
		court, e := originalWorldMotionCourt(&r, c.Actions, 1)
		if e != nil {
			t.Fatal(e)
		}
		load := court.originalMotion.load
		seen := false
		court.originalMotion.load = func() CourtFloorState {
			seen = true
			if r.State.SpecialHolder != 1 {
				t.Fatal("motion saw old captured actor binding", r.State.SpecialHolder)
			}
			if holder == 1 {
				if r.State.Holder != 6 || !r.State.Ball.Request.Pending || r.State.Ball.Request.Action != BallDropped || r.State.Ball.Action != beforeBall.Action || r.State.Ball.Body.Velocity.X != -2*Pixel {
					t.Fatal("captured release not committed before motion", r.State.Holder, r.State.Ball)
				}
			} else if r.State.Holder != holder || r.State.Ball != beforeBall {
				t.Fatal("capture released an unrelated or absent ball")
			}
			return load()
		}
		if e = advanceWorldPlayer(&r, 1, c.Actions, court); e != nil {
			t.Fatal(e)
		}
		if !seen {
			t.Fatal("captured player skipped original motion")
		}
	}
}

func TestWorldKind10CaptureToComputerEscape(t *testing.T) {
	s, c := capturedWorldFromContact(t, 6)
	landed := 0
	previous := uint8(4)
	bound := false
	escaped := false
	for n := 0; n < 200; n++ {
		step, e := StepWorldLoop(s, WorldLoopInput{}, c)
		if e != nil {
			t.Fatalf("capture update%d: %v", n, e)
		}
		s = step.State
		a := s.Preparation.Actions
		if a.SpecialHolder == 1 {
			bound = true
		}
		if a.CaptureCounter != previous {
			if a.CaptureCounter != previous-1 {
				t.Fatal("escape counter reset or skipped", previous, a.CaptureCounter)
			}
			previous = a.CaptureCounter
			landed++
		}
		if bound && a.SpecialHolder == -1 {
			if a.CaptureCounter != 255 || landed != 5 || a.Actors[1].Player.Action != ActionLand || a.Auxiliary[0].Action != 7 || !a.Auxiliary[0].Entered {
				t.Fatal("container disappeared before its original release action", a.CaptureCounter, landed, a.Actors[1].Player, a.Auxiliary[0])
			}
			escaped = true
			break
		}
	}
	if !escaped {
		t.Fatal("captured computer never completed original escape chain")
	}
}

func TestWorldCapturedFallingUsesOriginalOwnerScan(t *testing.T) {
	f, e := os.Open("testdata/captured-player.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var oracle struct{ Falls []capturedOracleCall }
	if e = json.NewDecoder(z).Decode(&oracle); e != nil {
		t.Fatal(e)
	}
	if len(oracle.Falls) != 4 {
		t.Fatal(len(oracle.Falls))
	}
	for i, row := range oracle.Falls {
		r := WorldActionStep{State: row.Before}
		c := row.context(t)
		court, e := originalWorldMotionCourt(&r, c, row.Actor)
		if e != nil {
			t.Fatal(e)
		}
		if e = advanceWorldPlayer(&r, row.Actor, c, court); e != nil {
			t.Fatal(i, e)
		}
		if r.State != row.After {
			t.Fatalf("captured fall%d: %s", i, firstWorldLoopDifference(reflect.ValueOf(r.State), reflect.ValueOf(row.After), "world"))
		}
	}
}

func TestWorldCapturedLandingRetainsDeviceJumpReset(t *testing.T) {
	// Both scenarios retain the original controlled contact seed. The old
	// below-surface placement fires the device immediately rather than landing;
	// the 32px placement exercises an actual captured jump/landing on it.
	for _, tc := range []struct {
		name   string
		height Fixed
	}{{"below-surface-original-regression", 0}, {"surface-landing", 32 * Pixel}} {
		t.Run(tc.name, func(t *testing.T) {
			w, c := capturedWorldFromContact(t, 6)
			r := WorldActionStep{State: w.Preparation.Actions}
			court, e := originalWorldMotionCourt(&r, c.Actions, 1)
			if e != nil {
				t.Fatal(e)
			}
			if e = advanceWorldPlayer(&r, 1, c.Actions, court); e != nil {
				t.Fatal(e)
			}
			c.Actions.CourtID = 2
			p := &r.State.Actors[1].Player
			p.Body.Position = Position{X: 280 * Pixel, Depth: 76 * Pixel, Height: tc.height}
			p.Request = ActionRequest{Action: ActionCapturedJump, Pending: true}
			r.State.Actors[1].JumpPhase = 37
			airborne := false
			for tick := 0; tick < 40; tick++ {
				court, e = originalWorldMotionCourt(&r, c.Actions, 1)
				if e != nil {
					t.Fatal(e)
				}
				if e = advanceWorldPlayer(&r, 1, c.Actions, court); e != nil {
					t.Fatal(e)
				}
				if tc.height == 0 {
					if tick != 0 || p.Action != ActionLaunched || !p.Airborne {
						t.Fatal("below-surface original device transition changed", tick, *p)
					}
				} else {
					if p.Airborne {
						airborne = true
						continue
					}
					if !airborne {
						t.Fatal("captured jump did not leave device surface")
					}
					if p.Action != ActionCapturedLand || r.State.CaptureCounter != 3 || p.Body.Position.Height != 32*Pixel {
						t.Fatal("captured landing did not retain original post-motion writes", *p, r.State.CaptureCounter)
					}
				}
				if !court.originalMotion.resetJumpPhase || r.State.Actors[1].JumpPhase != 0 {
					t.Fatal("captured attachment adapter lost device jump reset", r.State.Actors[1].JumpPhase)
				}
				if len(r.SoundIDs) != 1 || r.SoundIDs[0] != 21 {
					t.Fatal("device activation sound missing or repeated", r.SoundIDs)
				}
				return
			}
			t.Fatal("captured jump never landed on original device")
		})
	}
}
