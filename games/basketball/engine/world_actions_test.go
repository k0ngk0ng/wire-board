package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
)

type worldActionFixture struct {
	Frame         int
	Before, After WorldActionState
	Calls         []uint8
	Context       struct {
		CachedZones                                                             [4]CourtZone
		ShotEquipment                                                           [4]uint8
		BallShotInputButtons                                                    Buttons
		CourtID                                                                 uint8
		Controls, Characters, RunGrades, JumpGrades, ShotSkills, RecoveryGrades [4]uint8
		Teams                                                                   [2]uint8
		Equipment                                                               uint8
		EnhancementEnabled                                                      bool
		Enhancements                                                            [4]uint8
		OtherObjectArcFlags                                                     [7]uint8
	}
}

var worldFixtureOnce sync.Once
var worldFixtureRows []worldActionFixture
var worldFixtureErr error

func worldActionFixtures(t *testing.T) []worldActionFixture {
	t.Helper()
	worldFixtureOnce.Do(func() {
		f, err := os.Open("testdata/world-actions.json.gz")
		if err != nil {
			worldFixtureErr = err
			return
		}
		defer f.Close()
		reader, err := gzip.NewReader(f)
		if err != nil {
			worldFixtureErr = err
			return
		}
		defer reader.Close()
		worldFixtureErr = json.NewDecoder(reader).Decode(&worldFixtureRows)
	})
	if worldFixtureErr != nil {
		t.Fatal(worldFixtureErr)
	}
	if len(worldFixtureRows) != 1998 {
		t.Fatal("missing original world action stages", len(worldFixtureRows))
	}
	return worldFixtureRows
}

func (r worldActionFixture) context(t *testing.T) WorldActionContext {
	t.Helper()
	c := WorldActionContext{CourtID: r.Context.CourtID, Controls: r.Context.Controls, Characters: r.Context.Characters, Teams: r.Context.Teams, RecoveryGrades: r.Context.RecoveryGrades, Equipment: r.Context.Equipment, EnhancementEnabled: r.Context.EnhancementEnabled, Enhancements: r.Context.Enhancements}
	c.OtherObjectArcFlags = r.Context.OtherObjectArcFlags
	c.CachedZones, c.ShotEquipment, c.BallShotInputButtons = r.Context.CachedZones, r.Context.ShotEquipment, r.Context.BallShotInputButtons
	for i := range c.Profiles {
		profile := KunioProfile()
		var err error
		profile.RunSpeed, err = RunSpeedForGrade(r.Context.RunGrades[i])
		if err != nil {
			t.Fatal(err)
		}
		profile.JumpVelocity, err = JumpVelocityForGrade(r.Context.JumpGrades[i])
		if err != nil {
			t.Fatal(err)
		}
		profile.ShotSkill = r.Context.ShotSkills[i]
		c.Profiles[i] = profile
	}
	return c
}

func TestWorldActionsOriginalInitialPrefix(t *testing.T) {
	assertWorldActionPrefix(t, 59)
}

func TestWorldActionsOriginalExtendedPrefix(t *testing.T) {
	assertWorldActionPrefix(t, 270)
}

func TestWorldActionsEntireCapturedCorpus(t *testing.T) {
	assertWorldActionPrefix(t, 1998)
}

func assertWorldActionPrefix(t *testing.T, count int) {
	t.Helper()
	rows := worldActionFixtures(t)
	calls := 0
	for index, r := range rows[:count] {
		got, err := StepWorldActions(r.Before, r.context(t))
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
			t.Fatalf("phase %d frame %d: calls %v want %v", index, r.Frame, got.Calls, r.Calls)
		}
		calls += len(got.Calls)
	}
	t.Logf("matched the original initial %d complete 13-object action stages, %d actual calls; this is not a whole-world continuous replay", count, calls)
}

func TestWorldActionsUnknownHandlerAndParametersAreAtomic(t *testing.T) {
	r := worldActionFixtures(t)[1]
	original := r.Before
	original.Actors[1].Player.Action = Action(0x7f)
	got, err := StepWorldActions(original, r.context(t))
	if err == nil || got.State != original || len(got.Calls) != 0 {
		t.Fatal("unknown actor left a partial world update", got, err)
	}
	c := r.context(t)
	c.CourtID = 255
	got, err = StepWorldActions(r.Before, c)
	if err == nil || got.State != r.Before || len(got.Calls) != 0 {
		t.Fatal("invalid court left a partial world update", got, err)
	}
}

func TestWorldActionsStateRoundTrip(t *testing.T) {
	r := worldActionFixtures(t)[30]
	data, err := json.Marshal(r.Before)
	if err != nil {
		t.Fatal(err)
	}
	var restored WorldActionState
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored != r.Before {
		t.Fatal("serializing the world changed its authoritative state")
	}
	first, err := StepWorldActions(r.Before, r.context(t))
	if err != nil {
		t.Fatal(err)
	}
	second, err := StepWorldActions(restored, r.context(t))
	if err != nil || first.State != second.State || !reflect.DeepEqual(first.Calls, second.Calls) {
		t.Fatal("restored world did not reproduce the action stage", err)
	}
}

func TestWorldActionsHoopThrowArcPrefix(t *testing.T) {
	r := worldActionFixtures(t)[665]
	got, err := StepWorldActions(r.Before, r.context(t))
	if err != nil || got.State != r.After || !reflect.DeepEqual(got.Calls, r.Calls) {
		t.Fatalf("original frame %d throw differs: hoop %+v want %+v; arc %d want %d: %v", r.Frame, got.State.Hoops[2], r.After.Hoops[2], got.State.LaunchArcHeight, r.After.LaunchArcHeight, err)
	}
}

func TestWorldActionsOriginalDunkStage(t *testing.T) {
	r := worldActionFixtures(t)[270]
	got, err := StepWorldActions(r.Before, r.context(t))
	if err != nil || got.State != r.After || !reflect.DeepEqual(got.Calls, r.Calls) {
		t.Fatalf("frame %d complete dunk action stage differs: %v", r.Frame, err)
	}
}

func TestWorldActionsHoopMountIdentity(t *testing.T) {
	rows := worldActionFixtures(t)
	for _, index := range []int{1546, 1674} {
		r := rows[index]
		got, err := StepWorldActions(r.Before, r.context(t))
		if err != nil || got.State != r.After || !reflect.DeepEqual(got.Calls, r.Calls) {
			t.Fatalf("frame %d remounted hoop lost original identity: got %v want %v: %v", r.Frame, got.State.hoopOwners(), r.After.hoopOwners(), err)
		}
	}
}

func TestWorldHeldEntryClearsExclusion(t *testing.T) {
	r := worldActionFixtures(t)[48]
	s := r.Before
	s.Ball.Action, s.Ball.Entered = BallHeld, false
	s.ExcludedCatcher = 0
	owner := s.Holder
	if err := advanceWorldBall(&s, r.context(t)); err != nil {
		t.Fatal(err)
	}
	if s.ExcludedCatcher != -1 || s.Holder != owner {
		t.Fatal("held entry did not clear exclusion independently of ownership", s.ExcludedCatcher, s.Holder)
	}
	s.ExcludedCatcher = 1
	if err := advanceWorldBall(&s, r.context(t)); err != nil {
		t.Fatal(err)
	}
	if s.ExcludedCatcher != 1 {
		t.Fatal("held flight repeated entry-only exclusion reset")
	}
}

func TestWorldPassLaunchMetadata(t *testing.T) {
	data, err := os.ReadFile("testdata/pass-launch.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ X, Depth, Height, TargetX, TargetDepth, TargetHeight, ArcHeight int32 }
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		// Original launch parameters, independently recorded before flight;
		// this checks adapter commands, not a whole-world natural phase.
		target := Position{X: Fixed(r.TargetX) * Pixel, Depth: Fixed(r.TargetDepth) * Pixel, Height: Fixed(r.TargetHeight) * Pixel}
		s := WorldActionState{Holder: 0, Ball: Ball{Action: BallPass, Body: Body{Position: Position{X: Fixed(r.X) * Pixel, Depth: Fixed(r.Depth) * Pixel, Height: Fixed(r.Height) * Pixel}}}}
		s.Actors[1].Player.Body.Position = target
		if err := advanceWorldBall(&s, WorldActionContext{}); err != nil {
			t.Fatal(err)
		}
		if s.LaunchArcHeight != Fixed(r.ArcHeight)*Pixel || s.LaunchTarget != target || s.BallLanding != (Position{X: target.X, Depth: target.Depth}) || s.Holder != 6 || s.ExcludedCatcher != 0 {
			t.Fatalf("original pass metadata differs: arc %d want %d", s.LaunchArcHeight, Fixed(r.ArcHeight)*Pixel)
		}
	}
}
