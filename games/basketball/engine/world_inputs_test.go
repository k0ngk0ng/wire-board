package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
)

type worldInputFixture struct {
	Frame         int
	Before, After WorldInputState
	Calls         []uint8
	Context       struct {
		PursuitReserved *PursuitReservedBytes
		World           struct {
			CourtID                                                                               uint8
			Controls, Characters, RunGrades, JumpGrades, ShotSkills, RecoveryGrades, Enhancements [4]uint8
			Teams                                                                                 [2]uint8
			Equipment                                                                             uint8
			EnhancementEnabled                                                                    bool
			OtherObjectArcFlags                                                                   [7]uint8
		}
		Frames          [4]ControllerFrame
		Difficulty      uint8
		ScoreDifference uint8
		CachedZones     [4]CourtZone
		ObjectKinds     [13]uint8
		HoopLinks       [6]uint8
		HumanTactics    [2]uint8
		PursuitAlias74  *PursuitAlias74
	}
}

var worldInputsOnce sync.Once
var worldInputsRows []worldInputFixture
var worldInputsError error

func worldInputFixtures(t *testing.T) []worldInputFixture {
	t.Helper()
	worldInputsOnce.Do(func() {
		f, err := os.Open("testdata/world-inputs.json.gz")
		if err != nil {
			worldInputsError = err
			return
		}
		defer f.Close()
		reader, err := gzip.NewReader(f)
		if err != nil {
			worldInputsError = err
			return
		}
		defer reader.Close()
		worldInputsError = json.NewDecoder(reader).Decode(&worldInputsRows)
	})
	if worldInputsError != nil {
		t.Fatal(worldInputsError)
	}
	if len(worldInputsRows) != 1998 {
		t.Fatal("missing complete original input phases", len(worldInputsRows))
	}
	return worldInputsRows
}

func (r worldInputFixture) context(t *testing.T) WorldInputContext {
	t.Helper()
	v := r.Context.World
	c := WorldInputContext{World: WorldActionContext{CourtID: v.CourtID, Controls: v.Controls, Characters: v.Characters, RecoveryGrades: v.RecoveryGrades, Teams: v.Teams, Equipment: v.Equipment, EnhancementEnabled: v.EnhancementEnabled, Enhancements: v.Enhancements, OtherObjectArcFlags: v.OtherObjectArcFlags}, Frames: r.Context.Frames, Difficulty: r.Context.Difficulty, CachedZones: r.Context.CachedZones, ObjectKinds: r.Context.ObjectKinds, HoopLinks: r.Context.HoopLinks, HumanTactics: r.Context.HumanTactics, PursuitAlias74: r.Context.PursuitAlias74, PursuitReserved: r.Context.PursuitReserved}
	c.ScoreDifference = r.Context.ScoreDifference
	for i := range c.World.Profiles {
		p := KunioProfile()
		var err error
		p.RunSpeed, err = RunSpeedForGrade(v.RunGrades[i])
		if err != nil {
			t.Fatal(err)
		}
		p.JumpVelocity, err = JumpVelocityForGrade(v.JumpGrades[i])
		if err != nil {
			t.Fatal(err)
		}
		p.ShotSkill = v.ShotSkills[i]
		c.World.Profiles[i] = p
	}
	return c
}

func TestWorldInputsCompleteOriginalPhase(t *testing.T) {
	rows := worldInputFixtures(t)
	for index, r := range rows {
		got, err := StepWorldInputs(r.Before, r.context(t))
		if err != nil {
			t.Fatalf("phase %d frame %d: %v", index, r.Frame, err)
		}
		if got.State != r.After {
			a, b := reflect.ValueOf(got.State), reflect.ValueOf(r.After)
			for i := 0; i < a.NumField(); i++ {
				if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
					t.Errorf("phase %d frame %d field %s\ngot %+v\nwant %+v", index, r.Frame, a.Type().Field(i).Name, a.Field(i).Interface(), b.Field(i).Interface())
				}
			}
			t.FailNow()
		}
		if !reflect.DeepEqual(got.Calls, r.Calls) {
			t.Fatal("wrong original object input sequence", index, got.Calls, r.Calls)
		}
	}
	t.Log("matched all 1998 complete original input phases / 19980 object input calls; no physics executed")
}

func TestWorldInputsLaterSeatsSeePickupAndDoNotMove(t *testing.T) {
	r := worldInputFixtures(t)[1]
	s, c := r.Before, r.context(t)
	position := Position{X: 200 * Pixel, Depth: 40 * Pixel}
	s.World.Holder = 6
	s.World.Ball = Ball{Action: BallRest, Body: Body{Position: position}}
	for _, actor := range []int{1, 2} {
		s.World.Actors[actor].Player = Player{Action: ActionIdle, Entered: true, Body: Body{Position: position}}
		s.Team.Players[actor] = AIControlState{Mode: AIPursueBall, Entered: true, Plan: AIPursueBall, Counter: 2}
	}
	c.World.Controls[3] = 0
	s.World.Actors[3].Player.Action = ActionIdle
	got, err := StepWorldInputs(s, c)
	if err != nil || got.State.World.Holder != 1 || got.State.World.Actors[1].Player.Action != ActionPickUp || got.State.Team.Players[2].Mode != 7 || got.State.Team.Players[2].TrackedObject != 1 {
		t.Fatal("later AI read stale pre-pickup ownership", got, err)
	}
	if got.State.World.Ball.Action != BallHeld || got.State.World.Ball.Request != (BallRequest{Action: BallRest, Pending: true}) {
		t.Fatal("owned-ball input was not applied after all actors", got.State.World.Ball)
	}
	for actor := range s.World.Actors {
		if got.State.World.Actors[actor].Player.Body.Position != s.World.Actors[actor].Player.Body.Position || got.State.World.Actors[actor].Player.Animation != s.World.Actors[actor].Player.Animation {
			t.Fatal("input phase ran actor physics or animation", actor)
		}
	}
	if got.State.World.Ball.Body.Position != position {
		t.Fatal("input phase moved ball")
	}
}

func TestWorldInputsDisabledComputerAndAtomicFailure(t *testing.T) {
	r := worldInputFixtures(t)[1]
	s, c := r.Before, r.context(t)
	s.Team.Players[2].Mode = 127
	c.World.Controls[2] = 0xc0
	got, err := StepWorldInputs(s, c)
	if err != nil || got.State.Team.Players[2] != s.Team.Players[2] {
		t.Fatal("disabled computer still ran strategy", got, err)
	}
	c.World.Controls[2] = 0x80
	got, err = StepWorldInputs(s, c)
	if err == nil || got.State != s || len(got.Calls) != 0 {
		t.Fatal("failed later actor retained partial earlier inputs", got, err)
	}
}

func TestWorldInputsExpiredLinksAndJumpPhase(t *testing.T) {
	r := worldInputFixtures(t)[1]
	s, c := r.Before, r.context(t)
	c.World.Controls = [4]uint8{0xc0, 0xc0, 0xc0, 0xc0}
	s.World.Actors[0].LinkedActor = 1
	s.World.PlayerLinks[1] = 64
	s.World.Actors[0].JumpPhase = -1
	s.World.Actors[0].Player.Airborne = false
	s.World.Actors[2].JumpPhase = -1
	s.World.Actors[2].Player.Airborne = true
	s.World.CarriedPlayer = 3
	s.World.PlayerLinks[3] = 64
	s.World.Actors[0].Player.Request = ActionRequest{Action: ActionAttackA, Pending: false}
	got, err := StepWorldInputs(s, c)
	if err != nil || got.State.World.Actors[0].LinkedActor != -1 || got.State.World.Actors[0].JumpPhase != 0 || got.State.World.Actors[2].JumpPhase != -1 || got.State.World.CarriedPlayer != -1 || got.State.World.Actors[0].Player.Request != (ActionRequest{Action: ActionIdle, Pending: true}) {
		t.Fatal(got, err)
	}
}
