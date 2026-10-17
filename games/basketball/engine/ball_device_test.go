package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

type ballDeviceObserved struct {
	observedBall
	Holder         uint8
	UnownedSpecial uint8
	Target         Position
	Arc            Fixed
	Landing        Position
}

func TestOriginalDeviceShotAndStuckBallInstructions(t *testing.T) {
	f, err := os.Open("testdata/ball-device.json.gz")
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
		Kind          string
		Before, After ballDeviceObserved
		HoopHeight    Fixed
		MountFlags    [6]uint8
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1054 {
		t.Fatal("missing original action probes", len(rows))
	}
	counts := map[string]int{}
	for i, r := range rows {
		before, want := ballFromObservation(r.Before.observedBall), ballFromObservation(r.After.observedBall)
		var got Ball
		holder := r.Before.Holder
		if r.Kind == "owner6" {
			ctx := OriginalShotBallContext{Holder: 6, EnhancementEnabled: true, Equipment: 255, MountFlags: r.MountFlags, UnownedSpecialShot: r.Before.UnownedSpecial}
			step, e := StepOriginalShotBall(before, ctx, NewYorkBallCourt())
			err = e
			got = step.Ball
			if step.ReleaseHolder {
				holder = 6
			}
			flag := r.Before.UnownedSpecial
			if step.ClearUnownedSpecialShot {
				flag = 0
			}
			if step.Target != r.After.Target || step.ArcHeight != r.After.Arc || step.Landing != r.After.Landing || step.SelectedHoop != int8(r.After.Hoop) || flag != r.After.UnownedSpecial || step.ChangeSpecialShot {
				t.Fatalf("row%d launch metadata got%+v want%+v", i, step, r.After)
			}
		} else {
			step, e := StepStuckBall(before, r.HoopHeight, NewYorkBallCourt())
			err = e
			got = step.Ball
			if step.ReleaseHolder {
				holder = 6
			}
		}
		if err != nil || got != want || holder != r.After.Holder {
			t.Fatalf("row%d %s before%+v\ngot%+v\nwant%+v holder%d/%d err%v", i, r.Kind, before, got, want, holder, r.After.Holder, err)
		}
		counts[r.Kind]++
	}
	t.Logf("matched original full ball action instructions %v", counts)
}

func TestCourtThreeRecordedDeviceAndRimFailuresAdvance(t *testing.T) {
	f, err := os.Open("testdata/ball-device-runtime.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var cases []struct {
		Failure string
		State   RuntimeState
	}
	if err = json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		got, err := StepVideo(c.State, [4]Buttons{})
		if err != nil {
			t.Fatalf("case%d original failure %s: %v", i, c.Failure, err)
		}
		if !got.LogicAdvanced || got.State.LogicSteps != c.State.LogicSteps+1 {
			t.Fatal("failure skipped logical update")
		}
		if i == 0 {
			b := got.State.Playing.World.Preparation.Actions.Ball
			if b.Action != BallShot || !b.Entered || !b.Airborne {
				t.Fatal("owner-six launch did not execute", b)
			}
		}
		if i == 1 {
			a := got.State.Playing.World.Preparation.Actions
			if a.Ball.Action != BallStuck || !a.Ball.Entered || a.Ball.BouncesRemaining != 95 || a.Holder != 6 {
				t.Fatal("stuck ball did not enter original handler", a.Ball, a.Holder)
			}
		}
	}
}

func TestWorldUnownedShotFlagAndDynamicStuckHeight(t *testing.T) {
	fixture := worldActionFixtures(t)[0]
	s, ctx := fixture.Before, fixture.context(t)
	for i := range s.Actors {
		s.Actors[i].Player.Action = ActionIdle
		s.Actors[i].Player.Entered = false
		s.Actors[i].Player.Heading = HeadingNone
		s.Actors[i].Player.Request = ActionRequest{}
		s.Actors[i].SpecialShot = uint8(i + 1)
	}
	s.Holder = 6
	s.UnownedSpecialShot = 128
	s.Ball = Ball{Action: BallShot, Body: Body{Position: Position{X: 200*Pixel + 5, Depth: 40*Pixel + 7, Height: 152*Pixel + 9}}}
	got, err := StepWorldActions(s, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.UnownedSpecialShot != 0 || got.State.LaunchTarget.Height != 272*Pixel {
		t.Fatal("owner-six flag not consumed", got.State.UnownedSpecialShot, got.State.LaunchTarget)
	}
	for i, a := range got.State.Actors {
		if a.SpecialShot != s.Actors[i].SpecialShot {
			t.Fatal("owner-six flag changed a player", i)
		}
	}
	s.UnownedSpecialShot = 0
	s.SelectedHoop = 9
	s.Hoops[2].Position.Height = 160*Pixel + 199
	s.Holder = 2
	s.Ball = Ball{Action: BallStuck, Body: Body{Position: Position{X: 62*Pixel + 5, Depth: 40*Pixel + 7, Height: 100*Pixel + 9}}}
	got, err = StepWorldActions(s, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Holder != 6 || got.State.Ball.Body.Position.Height != 174*Pixel+9 || got.State.Ball.Body.Position.Depth != 42*Pixel+7 || got.State.Ball.BouncesRemaining != 95 {
		t.Fatal("stuck entry ignored live selected hoop/fractions", got.State.Ball, got.State.Holder)
	}
}
