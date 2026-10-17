package engine

import (
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"reflect"
	"testing"
)

func boardContactRAM(r []byte) BoardContactState {
	word := func(a, b int) Fixed { return Fixed(r[a]) | Fixed(r[b])<<8 }
	body := func(i int) observedBall {
		return observedBall{Action: int32(r[0x512+i]), Request: int32(r[0x51f+i]), Pose: int32(r[0x5b+i]), Facing: int32(r[0x46b+i]), Clock: int32(r[0x437+i]), Cursor: int32(r[0x444+i]), RepeatCount: int32(r[0x451+i]), NextAction: int32(r[0x52c+i]), Air: int32(r[0x5f7+i]), X: int32(word(0x68+i, 0x75+i) + Fixed(r[0x82+i])<<16), Depth: int32(word(0x8f+i, 0x9c+i)), Height: int32(word(0xa9+i, 0xb6+i) + Fixed(r[0xc3+i])<<16), VX: int32(word(0x478+i, 0x485+i)), VY: int32(word(0x492+i, 0x49f+i)), VZ: int32(word(0x4ac+i, 0x4b9+i)), Gravity: int32(word(0x4c6+i, 0x4d3+i))}
	}
	s := BoardContactState{Holder: int8(r[0x53f]), Random: RandomState(binary.BigEndian.Uint32(r[0x48:0x4c])), Scores: [2]uint16{uint16(word(0x166, 0x168)), uint16(word(0x167, 0x169))}, ScoreSoundDelay: r[0xe3], DetachedCount: r[0xe4], PreviousBallAction: BallAction(r[0x6cf]), ObservedBallAction: BallAction(r[0x6d0]), SelectedBoard: r[0x6f6], DisplayFlags: r[0x6d6], CourtToggle: r[0x6d8], CourtScoreFlags: [2]uint8{r[0x6dd], r[0x6de]}}
	copy(s.MountFlags[:], r[0x6f0:0x6f6])
	for i := 0; i < 4; i++ {
		b := ballFromObservation(body(i))
		s.Players[i] = Player{Body: b.Body, Action: Action(b.Action), Entered: b.Entered, Pose: b.Pose, Animation: b.Animation, Airborne: b.Airborne, FacingLeft: b.FacingLeft, Request: ActionRequest{Action: Action(b.Request.Action), Pending: b.Request.Pending}, NewYorkPassage: r[0x6d7+i], Heading: HeadingNone}
		if r[0x4ec+i] != 255 {
			s.Players[i].Heading = Heading(r[0x4ec+i] / 32)
		}
	}
	for i := 4; i < 6; i++ {
		b := ballFromObservation(body(i))
		s.Auxiliary[i-4] = AuxiliaryState{Body: b.Body, Action: uint8(b.Action), Entered: b.Entered, Pose: b.Pose, Animation: b.Animation, Airborne: b.Airborne, FacingLeft: b.FacingLeft, Owner: int8(r[0x539+i]), Attachment: r[0x642+i], Request: ActionRequest{Action: Action(b.Request.Action), Pending: b.Request.Pending}, NewYorkPassage: r[0x6d7+i], ObjectLink: int8(r[0x56b+i])}
	}
	b := body(6)
	b.Bounces = int32(r[0x511])
	b.Deflection = int32(r[0x564])
	s.Ball = ballFromObservation(b)
	for i := 7; i < 13; i++ {
		s.Hoops[i-7] = hoopFromObservation(observedHoop{observedBall: body(i), Link: int32(r[0x539+i]), Cooldown: int32(r[0x626+i]), Durability: int32(r[0x577+i])})
	}
	return s
}
func boardContactContextRAM(r []byte) BoardContactContext {
	shooter := int(r[0x6d1])
	selected := int(r[0x55d]) - 7
	if selected < 0 || selected >= 6 {
		selected = -1
	}
	bonus := false
	if shooter < 2 {
		bonus = r[0x6e6+3*int(r[0x3e3+shooter])] == 27
	}
	return BoardContactContext{CourtID: r[0x58b], BackboardStrength: r[0x56d] >> 6, AuxiliaryKinds: [2]uint8{r[0x3da], r[0x3db]}, Rim: RimContext{SelectedHoop: selected, ShotOriginX: Fixed(uint16(r[0x55e])+uint16(r[0x55f])<<8) * Pixel, ShooterAction: Action(r[0x512+shooter] & 127), ShotSkill: r[0x61d+shooter], ScoringBonus: bonus}}
}
func TestBoardContactEveryOriginalObject(t *testing.T) {
	f, err := os.Open("testdata/board-contact.ram.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var counts [13]int
	var record [4101]byte
	var phase BoardContactState
	var frames int
	for n := 0; ; n++ {
		_, err = io.ReadFull(z, record[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		frame := binary.LittleEndian.Uint32(record[:4])
		object := record[4]
		if object != uint8(12-n%13) {
			t.Fatalf("call %d order %d", n, object)
		}
		before, after := boardContactRAM(record[5:2053]), boardContactRAM(record[2053:])
		ctx := boardContactContextRAM(record[5:2053])
		got, e := StepBoardContactObject(before, object, ctx)
		if e != nil {
			t.Fatalf("frame %d object %d: %v", frame, object, e)
		}
		if got.State != after {
			a, b := reflect.ValueOf(got.State), reflect.ValueOf(after)
			for i := 0; i < a.NumField(); i++ {
				if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
					t.Errorf("frame %d object %d field %s\ngot %+v\nwant %+v", frame, object, a.Type().Field(i).Name, a.Field(i).Interface(), b.Field(i).Interface())
				}
			}
			t.FailNow()
		}
		if object == 12 {
			whole, e := StepBoardContacts(before, ctx)
			if e != nil {
				t.Fatal(frame, e)
			}
			phase = whole.State
			frames++
		}
		if object == 0 && phase != after {
			t.Fatal("full descending phase differs", frame)
		}
		counts[object]++
	}
	for i, n := range counts {
		if n != 1997 {
			t.Fatalf("object %d has %d calls, want 1997", i, n)
		}
	}
	t.Logf("matched %d complete descending phases and all %v object calls", frames, counts)
}
func TestBoardContactErrorsAtomic(t *testing.T) {
	s := BoardContactState{ScoreSoundDelay: 8}
	got, err := StepBoardContactObject(s, 13, BoardContactContext{})
	if err == nil || got.State != s {
		t.Fatal(got, err)
	}
}

func TestBoardContactBoundsAndFractionPreservation(t *testing.T) {
	for _, tc := range []struct {
		x, depth, height Fixed
		wantX            Fixed
		hit              bool
	}{
		{48*Pixel + 7, 40 * Pixel, 72 * Pixel, 62*Pixel + 7, true},
		{47*Pixel + 7, 40 * Pixel, 72 * Pixel, 47*Pixel + 7, false},
		{60*Pixel + 7, 40 * Pixel, 72 * Pixel, 60*Pixel + 7, false},
		{52*Pixel + 7, 56 * Pixel, 72 * Pixel, 52*Pixel + 7, false},
		{468*Pixel + 7, 40 * Pixel, 72 * Pixel, 467*Pixel + 7, true},
		{480*Pixel + 7, 40 * Pixel, 72 * Pixel, 480*Pixel + 7, false},
	} {
		s := BoardContactState{Holder: 6}
		s.Players[0].Body = Body{Position: Position{X: tc.x, Depth: tc.depth, Height: tc.height}, Velocity: Velocity{X: 987, Depth: 123, Height: 456}}
		got, err := StepBoardContactObject(s, 0, BoardContactContext{})
		if err != nil {
			t.Fatal(err)
		}
		wantV := Fixed(987)
		if tc.hit {
			wantV = 0
		}
		if got.State.Players[0].Body.Position.X != tc.wantX || got.State.Players[0].Body.Velocity != (Velocity{X: wantV, Depth: 123, Height: 456}) {
			t.Fatal(tc, got)
		}
	}
}
func TestBoardContactHeldLocksAndHoopRemount(t *testing.T) {
	s := BoardContactState{Holder: 1}
	for i := range s.Hoops {
		s.Hoops[i].ScoreLock = 16
	}
	h := Hoop{Action: HoopThrown, Entered: true, Airborne: true, Position: Position{X: 50*Pixel + 19, Depth: 43*Pixel + 5, Height: 80*Pixel + 3}, Velocity: Velocity{X: 567}}
	s.Hoops[0] = h
	got, err := StepBoardContactObject(s, 7, BoardContactContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got.State.MountFlags[0] != 1 || got.State.Hoops[0].Position != (Position{X: 65*Pixel + 19, Depth: 43*Pixel + 5, Height: 80*Pixel + 3}) || got.State.Hoops[0].Action != HoopReady || got.State.Hoops[0].Entered || got.State.Hoops[0].Airborne || got.State.Hoops[0].Durability != 8 || got.State.Hoops[0].Velocity != h.Velocity {
		t.Fatal(got)
	}
	for _, h := range got.State.Hoops {
		if h.ScoreLock != 0 {
			t.Fatal("held ball did not clear locks")
		}
	}
	s.MountFlags[0] = 1
	got, err = StepBoardContactObject(s, 7, BoardContactContext{})
	if err != nil || got.State.MountFlags[0] != 3 || got.State.Hoops[0].Position.Height != 112*Pixel+3 {
		t.Fatal(got, err)
	}
	s.MountFlags[0] = 3
	got, err = StepBoardContactObject(s, 7, BoardContactContext{})
	if err != nil || got.State.Hoops[0].Action != HoopThrown || got.State.Hoops[0].Position.X != 62*Pixel+19 {
		t.Fatal(got, err)
	}
}
func TestBoardContactStuckHistoryAndSoundDelay(t *testing.T) {
	s := BoardContactState{Holder: 6, ScoreSoundDelay: 1, ObservedBallAction: BallShot, Ball: Ball{Action: BallStuck, Body: Body{Position: Position{X: 256*Pixel + 5, Depth: 90*Pixel + 6, Height: 84*Pixel + 7}, Velocity: Velocity{X: 123, Depth: 456}, Gravity: 99}}}
	s.Hoops[0].Position = Position{X: 65 * Pixel, Depth: 43 * Pixel, Height: 80 * Pixel}
	got, err := StepBoardContactObject(s, 6, BoardContactContext{Rim: RimContext{SelectedHoop: 0}})
	if err != nil || got.State.PreviousBallAction != BallShot || got.State.ObservedBallAction != BallStuck || got.State.Ball.Body.Position != (Position{X: 321*Pixel + 5, Depth: 45*Pixel + 6, Height: 84*Pixel + 7}) || got.State.Ball.Body.Velocity != (Velocity{Height: 257}) || got.State.Ball.Body.Gravity != 0 || got.State.Ball.BouncesRemaining != 1 || !reflect.DeepEqual(got.SoundIDs, []uint8{50}) {
		t.Fatal(got, err)
	}
	s.Ball.Action = BallRest
	s.ScoreSoundDelay = 0
	got, err = StepBoardContactObject(s, 6, BoardContactContext{})
	if err != nil || got.State.ScoreSoundDelay != 255 || len(got.SoundIDs) != 0 {
		t.Fatal(got, err)
	}
	s.Ball.Action = BallStuck
	got, err = StepBoardContactObject(s, 6, BoardContactContext{Rim: RimContext{SelectedHoop: -1}})
	if err == nil || got.State != s {
		t.Fatal("invalid stuck target left partial state", got, err)
	}
}
