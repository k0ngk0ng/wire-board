package engine

// Failure inventory before enabling Court4: NY motion substituted; conveyor
// direction duplicated instead of shared flag1; wrap loses RNG/kind/link
// changes; stale static kinds reach contact/input/display/period handoff;
// later local auxiliary results overwrite motion writes; invalid future courts
// or invalid dynamic kinds become silently accepted; restore changes motion.
import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func courtFourBindingPlaying(t *testing.T) RuntimeState {
	t.Helper()
	c := DefaultRuntimeConfig()
	c.Match.Court = 4
	c.Match.Seed = 0x12345678
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; s.Playing == nil && n < 400; n++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(n, e)
		}
		s = r.State
	}
	if s.Playing == nil {
		t.Fatal("court4 opening did not hand off")
	}
	return s
}

func TestCourtFourOriginalConveyorMotionBinding(t *testing.T) {
	data, e := os.ReadFile("testdata/court-four-motion.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Object uint8
		Sounds []uint8
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 640 {
		t.Fatal("incomplete original boundary matrix", len(rows))
	}
	f, e := os.Open("testdata/court-four-motion.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	wraps := 0
	for index, row := range rows {
		var raw [4096]byte
		if _, e = io.ReadFull(z, raw[:]); e != nil {
			t.Fatal(e)
		}
		before, want := courtFloorRAM(raw[:2048]), courtFloorRAM(raw[2048:])
		source, e := StepOriginalCourtMotion(before, row.Object, 4)
		if e != nil || source.State != want || len(source.SoundIDs) != len(row.Sounds) || !reflect.DeepEqual(source.SoundIDs, row.Sounds) && len(row.Sounds) != 0 {
			t.Fatalf("original EEFF case%d: %v", index, e)
		}
		if before.Random != want.Random {
			wraps++
		}
		r := WorldActionStep{State: WorldActionState{Random: before.Random, LastShooter: int8(before.LastShooter), Holder: before.Objects[6].Owner}}
		c := WorldActionContext{CourtID: 4, Teams: [2]uint8{0, 1}}
		for i := 0; i < 4; i++ {
			o := before.Objects[i]
			r.State.Actors[i].Player = Player{Body: o.Body, Action: Action(o.Action), Entered: o.Entered, Request: o.Request, Pose: o.Pose, FacingLeft: o.FacingLeft, Airborne: o.Flags&128 != 0, NewYorkPassage: before.CourtFlags[i]}
			r.State.Actors[i].LinkedActor = before.LinkedActors[i]
			r.State.Actors[i].JumpPhase = before.JumpPhases[i]
			r.State.PlayerLinks[i] = o.Attachment
			c.Controls[i] = o.Controller
		}
		for i := 0; i < 2; i++ {
			o := before.Objects[i+4]
			r.State.Auxiliary[i] = AuxiliaryState{Kind: o.Kind, Body: o.Body, Action: o.Action, Entered: o.Entered, Request: o.Request, Pose: o.Pose, FacingLeft: o.FacingLeft, Airborne: o.Flags&128 != 0, Owner: o.Owner, Attachment: o.Attachment, NewYorkPassage: before.CourtFlags[i+4]}
		}
		o := before.Objects[6]
		r.State.Ball = Ball{Body: o.Body, Action: BallAction(o.Action), Entered: o.Entered, Request: BallRequest{Action: BallAction(o.Request.Action), Pending: o.Request.Pending}, Pose: o.Pose, FacingLeft: o.FacingLeft, Airborne: o.Flags&128 != 0, Deflection: before.BallDeflection}
		r.State.BallLink = o.Attachment
		for i := 0; i < 6; i++ {
			o := before.Objects[i+7]
			r.State.Hoops[i] = WorldActionHoop{Position: o.Body.Position, Velocity: o.Body.Velocity, Gravity: o.Body.Gravity, Action: HoopAction(o.Action), Entered: o.Entered, Request: HoopRequest{Action: HoopAction(o.Request.Action), Pending: o.Request.Pending}, Pose: o.Pose, FacingLeft: o.FacingLeft, Airborne: o.Flags&128 != 0, Owner: uint8(o.Owner)}
		}
		court, e := originalWorldMotionCourt(&r, c, row.Object)
		if e != nil {
			t.Fatalf("case%d: %v", index, e)
		}
		a := r.State.Auxiliary[row.Object-4]
		if e = court.originalMotion.auxiliary(&a); e != nil {
			t.Fatal(index, e)
		}
		r.State.Auxiliary[row.Object-4] = a
		got := worldFloorState(r.State, c)
		for i := range want.Objects {
			want.Objects[i].Travel = FloorPoint{}
			want.Objects[i].PreviousHeight = 0
			want.Objects[i].Flags &= 128
		}
		if got != want {
			for i := range want.Objects {
				if got.Objects[i] != want.Objects[i] {
					t.Errorf("case%d object%d got%+v want%+v", index, i, got.Objects[i], want.Objects[i])
				}
			}
			t.Fatalf("case%d motion binding lost original state/RNG", index)
		}
		if a.Kind != want.Objects[row.Object].Kind {
			t.Fatal("selected kind lost")
		}
	}
	if wraps == 0 {
		t.Fatal("source matrix never wrapped")
	}
	t.Logf("matched %d original conveyor motion calls, %d wraps", len(rows), wraps)
}

func TestCourtFourDynamicKindsSurviveWorldConsumersAndRestore(t *testing.T) {
	s := courtFourBindingPlaying(t)
	a := &s.Playing.World.Preparation.Actions
	if a.Auxiliary[0].Kind != 10 || a.Auxiliary[1].Kind != 11 {
		t.Fatal("opening types", a.Auxiliary)
	}
	a.Auxiliary[0].Kind = 14
	a.Auxiliary[1].Kind = 12
	kinds := worldObjectKinds(*a, s.Playing.Context.ObjectKinds)
	if kinds[4] != 14 || kinds[5] != 12 {
		t.Fatal("static kinds replaced current types")
	}
	display := DisplaySourceFromWorld(s.Playing.World, s.Playing.Context, s.Playing.Match)
	if display.Objects[4].Kind != 14 || display.Objects[5].Kind != 12 {
		t.Fatal("display used stale kinds")
	}
	o, e := runtimeOpeningFromPlaying(*s.Playing, s.Config)
	if e != nil {
		t.Fatal(e)
	}
	if o.Transition.Kinds[4] != 14 || o.Transition.Kinds[5] != 12 {
		t.Fatal("period transition lost dynamic kinds")
	}
	raw, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var restored RuntimeState
	if e = json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	got, e := StepVideo(restored, [4]Buttons{})
	if e != nil {
		t.Fatal(e)
	}
	want, e := StepVideo(s, [4]Buttons{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("JSON restore changed world")
	}
	for _, kind := range []uint8{0, 9, 15, 255} {
		bad := cloneRuntime(s)
		bad.Playing.World.Preparation.Actions.Auxiliary[0].Kind = kind
		if e = ValidateRuntimeState(bad); e == nil {
			t.Fatal("invalid/missing conveyor kind restored", kind)
		}
	}
	for _, court := range []uint8{6, 7, 255} {
		c := s.Config
		c.Match.Court = court
		if _, e = NewRuntime(c); e == nil {
			t.Fatal("unverified court accepted", court)
		}
	}
}
