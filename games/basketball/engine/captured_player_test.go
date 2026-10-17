package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

type capturedOracleCall struct {
	worldActionFixture
	Actor  uint8
	Sounds []uint8
}
type capturedOracle struct {
	Calls    []capturedOracleCall
	Chain    []struct{ Player, Auxiliary capturedOracleCall }
	Requests []struct{ Before, After Ball }
	Devices  []struct {
		Height int
		Calls  []capturedOracleCall
	}
}

func TestOriginalCapturedDeviceWrites(t *testing.T) {
	fixture := capturedFixture(t)
	if len(fixture.Devices) != 2 || len(fixture.Devices[0].Calls) != 1 || len(fixture.Devices[1].Calls) != 15 {
		t.Fatal("incomplete original device oracle", fixture.Devices)
	}
	for _, scenario := range fixture.Devices {
		s := scenario.Calls[0].Before
		for tick, row := range scenario.Calls {
			if s != row.Before {
				t.Fatal("original captured device chain diverged before update", scenario.Height, tick)
			}
			r := WorldActionStep{State: s}
			ctx := row.context(t)
			court, e := originalWorldMotionCourt(&r, ctx, row.Actor)
			if e != nil {
				t.Fatal(e)
			}
			if e = advanceWorldPlayer(&r, row.Actor, ctx, court); e != nil {
				t.Fatal(e)
			}
			if !slices.Equal(r.SoundIDs, row.Sounds) {
				t.Fatal("original device sounds differ", scenario.Height, tick, r.SoundIDs, row.Sounds)
			}
			if r.State != row.After {
				t.Fatalf("height%d tick%d: %s", scenario.Height, tick, firstWorldLoopDifference(reflect.ValueOf(r.State), reflect.ValueOf(row.After), "world"))
			}
			s = r.State
		}
	}
}

func capturedFixture(t *testing.T) capturedOracle {
	t.Helper()
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
	var r capturedOracle
	if e = json.NewDecoder(z).Decode(&r); e != nil {
		t.Fatal(e)
	}
	if len(r.Calls) != 560 || len(r.Chain) != 131 || len(r.Requests) != 50 {
		t.Fatal("incomplete capture oracle")
	}
	return r
}
func stepCapturedOracle(t *testing.T, s WorldActionState, actor uint8, c WorldActionContext) (WorldActionState, []uint8) {
	t.Helper()
	a := s.Actors[actor]
	ctx := CapturedPlayerContext{Actor: actor, Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor), Attachment: s.PlayerLinks[actor], CaptureCounter: s.CaptureCounter}
	pre, e := PrepareCapturedPlayer(CapturedPlayerState{Player: a.Player, ShotCycle: a.ShotCycle}, ctx, c.Profiles[actor])
	if e != nil {
		t.Fatal(e)
	}
	s.Actors[actor].Player, s.Actors[actor].ShotCycle = pre.State.Player, pre.State.ShotCycle
	s.PlayerLinks[actor] = pre.Attachment
	if pre.ChangeSpecialHolder {
		s.SpecialHolder = pre.SpecialHolder
	}
	if pre.ReleaseHolder {
		s.Holder = 6
	}
	s.Ball = ApplyBallCommand(s.Ball, pre.Ball)
	if pre.ChangeBallVelocityX {
		s.Ball.Body.Velocity.X = pre.BallVelocityX
	}
	r := WorldActionStep{State: s, SoundIDs: append([]uint8(nil), pre.SoundIDs...)}
	court, e := originalWorldMotionCourt(&r, c, actor)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Attachment = pre.Attachment
	ctx.HasBall = r.State.Holder == int8(actor)
	done, e := CompleteCapturedPlayer(pre.State, ctx, c.Profiles[actor], court)
	if e != nil {
		t.Fatal(e)
	}
	r.State.Actors[actor].Player, r.State.Actors[actor].ShotCycle = done.State.Player, done.State.ShotCycle
	r.State.PlayerLinks[actor] = done.Attachment
	r.State.Ball = ApplyBallCommand(r.State.Ball, done.Ball)
	if done.ChangeCaptureCounter {
		r.State.CaptureCounter = done.CaptureCounter
	}
	if done.ReleaseContainer {
		r.State.Auxiliary[0].Action, r.State.Auxiliary[0].Entered = 7, false
	}
	return r.State, r.SoundIDs
}
func TestOriginalCapturedPlayerActions(t *testing.T) {
	for i, r := range capturedFixture(t).Calls {
		got, sounds := stepCapturedOracle(t, r.Before, r.Actor, r.context(t))
		if !slices.Equal(sounds, r.Sounds) {
			t.Fatal("captured action sounds differ", i, sounds, r.Sounds)
		}
		if got != r.After {
			t.Fatalf("call%d action%x: %s", i, r.Before.Actors[r.Actor].Player.Action, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(r.After), "world"))
		}
	}
}
func TestOriginalCapturedComputerEscapesContinuously(t *testing.T) {
	fixture := capturedFixture(t)
	s := fixture.Chain[0].Player.Before
	c := fixture.Chain[0].Player.context(t)
	for i, r := range fixture.Chain {
		next, sounds := stepCapturedOracle(t, s, 1, c)
		s = next
		if !slices.Equal(sounds, r.Player.Sounds) {
			t.Fatal("capture chain sounds differ", i, sounds, r.Player.Sounds)
		}
		if s != r.Player.After {
			t.Fatalf("player update%d: %s", i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.Player.After), "world"))
		}
		ctx := AuxiliaryContext{Object: 4, Court: 1, SpecialHolder: s.SpecialHolder}
		if s.SpecialHolder >= 0 && s.SpecialHolder < 4 {
			ctx.SpecialOwner = &s.Actors[s.SpecialHolder].Player
		}
		var step AuxiliaryStep
		var e error
		if s.Auxiliary[0].Action == 7 {
			step, e = StepAuxiliaryRest(s.Auxiliary[0], ctx)
		} else {
			step, e = StepAuxiliaryCarryThrow(s.Auxiliary[0], ctx, auxiliaryNewYorkCourt())
		}
		if e != nil {
			t.Fatal(i, e)
		}
		s.Auxiliary[0] = step.State
		if step.ChangePoseSource {
			s.PoseSource = step.PoseSource
		}
		if step.ClearSpecialHolder {
			s.SpecialHolder = -1
		}
		if s != r.Auxiliary.After {
			t.Fatalf("container update%d: %s", i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.Auxiliary.After), "world"))
		}
	}
	if s.CaptureCounter != 255 || s.SpecialHolder != -1 || s.Actors[1].Player.Action != ActionLand || s.Auxiliary[0].Action != 7 {
		t.Fatal("escape skipped original counter/landing/container handoff")
	}
}
func TestCapturedDropRequestOriginalAcceptance(t *testing.T) {
	for i, r := range capturedFixture(t).Requests {
		got, e := ResolveBallRequest(r.Before)
		if e != nil || got != r.After {
			t.Fatalf("request case%d: got %+v want %+v err %v", i, got, r.After, e)
		}
	}
}
