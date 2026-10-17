package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func possessedIdleHTTPState(t *testing.T) RuntimeState {
	t.Helper()
	f, err := os.Open("testdata/runtime-http-possessed-idle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var s RuntimeState
	if err = json.NewDecoder(z).Decode(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPossessedIdleOriginalRequestDispatch(t *testing.T) {
	s := possessedIdleHTTPState(t)
	before := s.Playing.World.Preparation.Actions
	if before.Holder != 1 || before.Actors[1].Player.Action != ActionDribbleIdle || !before.Actors[1].Player.Request.Pending || before.Actors[1].Player.Request.Action != ActionIdle {
		t.Fatal("fixture no longer represents the reported dispatch")
	}
	got := WorldActionStep{State: before}
	if err := advanceWorldPlayer(&got, 1, s.Playing.Context.Actions, NewYorkGroundCourt()); err != nil {
		t.Fatal(err)
	}
	p := got.State.Actors[1].Player
	if p.Action != ActionIdle || !p.Entered || p.Request.Pending || p.Body.Position != before.Actors[1].Player.Body.Position || got.State.Holder != 1 {
		t.Fatalf("original accepted idle path changed: %+v holder %d", p, got.State.Holder)
	}
}

func TestPossessedIdleHTTPFixedState24VideoTicks(t *testing.T) {
	s := possessedIdleHTTPState(t)
	start := s.VideoFrames
	for i := 0; i < 24; i++ {
		next, err := StepVideo(s, [4]Buttons{})
		if err != nil {
			t.Fatalf("tick %d video %d logic %d: %v", i, s.VideoFrames, s.LogicSteps, err)
		}
		s = next.State
		if i == 0 && s.Playing.World.Preparation.Actions.Holder != 1 {
			t.Fatal("idle dispatch released possession")
		}
	}
	if s.VideoFrames != start+24 {
		t.Fatal("video clock changed", s.VideoFrames, start)
	}
	t.Logf("24 video ticks from unchanged HTTP state: video %d logic %d", s.VideoFrames, s.LogicSteps)
}

func TestPossessedIdleHeldOffsetOriginalTable(t *testing.T) {
	for _, mirrored := range []bool{false, true} {
		p := Player{Action: ActionIdle, Pose: Pose{Mirrored: mirrored}, Body: Body{Position: Position{X: 200*Pixel + 15, Depth: 40*Pixel + 25, Height: 10*Pixel + 35}}}
		b := Ball{Action: BallHeld, Entered: true, Body: Body{Position: Position{X: 230*Pixel + 91, Depth: 42*Pixel + 92, Height: 20*Pixel + 93}}}
		got, err := StepHeldBall(b, p)
		if err != nil {
			t.Fatal(err)
		}
		x := 196*Pixel + 91
		if mirrored {
			x = 204*Pixel + 91
		}
		if got.Body.Position != (Position{X: x, Depth: 39*Pixel + 92, Height: 17*Pixel + 93}) {
			t.Fatal("idle offset differs from fixed $ED85", got.Body.Position)
		}
	}
}

func TestPossessedIdlePassageRetainsOwnership(t *testing.T) {
	s := possessedIdleHTTPState(t)
	before := s.Playing.World.Preparation.Actions
	p := &before.Actors[1].Player
	p.Action = ActionIdle
	p.Entered = true
	p.Request.Pending = false
	p.NewYorkPassage = 1
	p.Body.Position = Position{X: 380 * Pixel, Depth: 75 * Pixel, Height: -192 * Pixel}
	got := WorldActionStep{State: before}
	if err := advanceWorldPlayer(&got, 1, s.Playing.Context.Actions, NewYorkGroundCourt()); err != nil {
		t.Fatal(err)
	}
	p = &got.State.Actors[1].Player
	if p.Action != ActionCarrierJump || p.NewYorkPassage != 0 || got.State.Holder != 1 || got.State.Ball.Action != BallHeld || !got.State.Ball.Request.Pending || got.State.Ball.Request.Action != BallHeld {
		t.Fatal("held passage used free exit or lost ball command", *p, got.State.Ball)
	}
}

func TestPossessedFreeActionsOriginalInstructionCalls(t *testing.T) {
	f, err := os.Open("testdata/possessed-free-actions.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		worldActionFixture
		Actor             uint8
		Action            Action
		Computer, Entered bool
		Variant           string
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 116 {
		t.Fatal("missing isolated cases", len(rows))
	}
	for _, r := range rows {
		got := WorldActionStep{State: r.Before}
		err := advanceWorldPlayer(&got, r.Actor, r.context(t), NewYorkGroundCourt())
		if err != nil {
			t.Errorf("action %d cpu %v entered %v %s: %v", r.Action, r.Computer, r.Entered, r.Variant, err)
			continue
		}
		if got.State != r.After {
			t.Errorf("action %d cpu %v entered %v %s: %s", r.Action, r.Computer, r.Entered, r.Variant, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "world"))
		}
	}
}

func TestHeldPoseOffsetsEntireOriginalTable(t *testing.T) {
	data, err := os.ReadFile("testdata/held-pose-offsets.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Action Action
		Bytes  [4]uint8
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 106 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		for _, mirror := range []bool{false, true} {
			p := Player{Action: r.Action, Pose: Pose{Mirrored: mirror}, Body: Body{Position: Position{X: 200*Pixel + 3, Depth: 40*Pixel + 5, Height: 30*Pixel + 7}}}
			b := Ball{Action: BallHeld, Entered: true, Body: Body{Position: Position{X: 200*Pixel + 11, Depth: 40*Pixel + 13, Height: 40*Pixel + 17}}}
			got, e := StepHeldBall(b, p)
			if e != nil {
				t.Errorf("action %d mirror %v: %v", r.Action, mirror, e)
				continue
			}
			dx := Fixed(int16(uint16(r.Bytes[0])+uint16(r.Bytes[1])*256)) * Pixel
			if mirror {
				dx = -dx
			}
			want := Position{X: 200*Pixel + dx + 11, Depth: Fixed(uint8(40-r.Bytes[3]))*Pixel + 13, Height: (30+Fixed(r.Bytes[2]))*Pixel + 17}
			if got.Body.Position != want {
				t.Errorf("action %d mirror %v: got %+v want %+v", r.Action, mirror, got.Body.Position, want)
			}
		}
	}
}
