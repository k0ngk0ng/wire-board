package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

type teamSpecialOracleCall struct {
	worldActionFixture
	Prepared                WorldActionState
	ShakeBefore, ShakeAfter uint8
	Sounds                  []uint8
	Action                  BallAction
	Variant                 string
}

func teamSpecialOracles(t *testing.T) ([]teamSpecialOracleCall, []struct {
	Action  BallAction
	Left    bool
	Records []teamSpecialOracleCall
}) {
	t.Helper()
	f, e := os.Open("testdata/team-special-shots.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var out struct {
		Calls  []teamSpecialOracleCall
		Chains []struct {
			Action  BallAction
			Left    bool
			Records []teamSpecialOracleCall
		}
	}
	if e = json.NewDecoder(z).Decode(&out); e != nil {
		t.Fatal(e)
	}
	if len(out.Calls) != 84 || len(out.Chains) != 4 {
		t.Fatal("incomplete original special-shot oracle")
	}
	return out.Calls, out.Chains
}
func teamSpecialContext(s WorldActionState, c WorldActionContext) OriginalShotBallContext {
	ctx := OriginalShotBallContext{Holder: s.Holder, EnhancementEnabled: c.EnhancementEnabled, Equipment: 255, MountFlags: s.MountFlags, SelectedHoop: s.SelectedHoop, Target: s.LaunchTarget, ArcHeight: s.LaunchArcHeight, Landing: s.BallLanding}
	if s.Holder >= 0 && s.Holder < 4 {
		a := s.Actors[s.Holder]
		ctx.ShooterHeight = a.Player.Body.Position.Height
		ctx.ShooterAction = a.Player.Action
		ctx.SpecialShot = a.SpecialShot
		ctx.Team = c.Teams[s.Holder/2]
		ctx.Character = c.Characters[s.Holder]
		ctx.Equipment = c.ShotEquipment[s.Holder]
	}
	return ctx
}
func applyTeamSpecialStep(s *WorldActionState, shake *uint8, step OriginalTeamSpecialBallStep) {
	holder := s.Holder
	s.Ball = step.Ball
	s.SelectedHoop = step.SelectedHoop
	s.LaunchTarget = step.Target
	s.LaunchArcHeight = step.ArcHeight
	s.BallLanding = step.Landing
	if step.ChangeSpecialShot && holder >= 0 && holder < 4 {
		s.Actors[holder].SpecialShot = step.SpecialShot
	}
	if step.ReleaseHolder {
		s.Holder = 6
	}
	if step.ClearExcludedCatcher {
		s.ExcludedCatcher = -1
	}
	if step.ChangeCameraShake {
		*shake = step.CameraShake
	}
}
func TestOriginalTeamSpecialBallCalls(t *testing.T) {
	rows, _ := teamSpecialOracles(t)
	for i, r := range rows {
		s := r.Before
		shake := r.ShakeBefore
		ctx := teamSpecialContext(s, r.context(t))
		prepared, e := PrepareOriginalTeamSpecialBall(s.Ball, ctx)
		if e != nil {
			t.Errorf("call %d action %d %s prepare: %v", i, r.Action, r.Variant, e)
			continue
		}
		applyTeamSpecialStep(&s, &shake, prepared)
		if s != r.Prepared {
			t.Fatalf("call %d prepare: %s", i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.Prepared), "world"))
		}
		done, e := CompleteOriginalTeamSpecialBall(prepared, NewYorkBallCourt())
		if e != nil {
			t.Fatal(i, e)
		}
		applyTeamSpecialStep(&s, &shake, done)
		if s != r.After || shake != r.ShakeAfter {
			t.Fatalf("call %d final: %s shake %d want %d", i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"), shake, r.ShakeAfter)
		}
		sounds := append(append([]uint8(nil), prepared.SoundIDs...), done.SoundIDs...)
		if !slices.Equal(sounds, r.Sounds) {
			t.Fatalf("call %d sound %v want %v", i, sounds, r.Sounds)
		}
	}
}
func TestOriginalTeamSpecialBallContinuousTransitions(t *testing.T) {
	_, chains := teamSpecialOracles(t)
	for _, chain := range chains {
		s := chain.Records[0].Before
		shake := chain.Records[0].ShakeBefore
		cfg := chain.Records[0].context(t)
		for i, r := range chain.Records {
			step, e := PrepareOriginalTeamSpecialBall(s.Ball, teamSpecialContext(s, cfg))
			if e != nil {
				t.Fatalf("action%d left%v tick%d: %v", chain.Action, chain.Left, i, e)
			}
			applyTeamSpecialStep(&s, &shake, step)
			step, e = CompleteOriginalTeamSpecialBall(step, NewYorkBallCourt())
			if e != nil {
				t.Fatal(i, e)
			}
			applyTeamSpecialStep(&s, &shake, step)
			if s != r.After || shake != r.ShakeAfter {
				t.Fatalf("action%d left%v tick%d: %s", chain.Action, chain.Left, i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
			}
		}
		if s.Ball.Action != BallBounce {
			t.Fatal("chain did not reach ordinary bounce")
		}
	}
}

func TestOriginalTeamSpecialBallPlayerContact(t *testing.T) {
	f, e := os.Open("testdata/team-special-shots.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var data struct {
		Contacts []struct {
			teamSpecialOracleCall
			Candidate int8
		}
	}
	if e = json.NewDecoder(z).Decode(&data); e != nil {
		t.Fatal(e)
	}
	if len(data.Contacts) != 32 {
		t.Fatal(len(data.Contacts))
	}
	for i, r := range data.Contacts {
		s := r.Before
		c := r.context(t)
		state := InteractionState{Ball: s.Ball, Holder: s.Holder, LastTouch: s.LastTouch, SelectedHoop: s.SelectedHoop, Auxiliary: s.Auxiliary, SpecialHolder: s.SpecialHolder}
		for j, a := range s.Actors {
			state.Players[j] = CombatActor{Player: a.Player, AttackVariant: a.AttackVariant, Endurance: a.Endurance}
		}
		ctx := InteractionContext{CourtID: c.CourtID, Controls: c.Controls, Attachments: s.PlayerLinks, Candidates: [7]int8{-1, -1, -1, -1, -1, -1, r.Candidate}, ExcludedCatcher: s.ExcludedCatcher, AuxiliaryOwners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}, HoopOwners: s.hoopOwners()}
		got, e := StepOriginalBallContact(state, ctx)
		if e != nil {
			t.Fatal(i, e)
		}
		s.Ball, s.Holder, s.LastTouch, s.SelectedHoop = got.State.Ball, got.State.Holder, got.State.LastTouch, got.State.SelectedHoop
		for j, a := range got.State.Players {
			s.Actors[j].Player, s.Actors[j].AttackVariant, s.Actors[j].Endurance = a.Player, a.AttackVariant, a.Endurance
		}
		if s != r.After {
			t.Fatalf("contact %d: %s", i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
		}
		var sounds []uint8
		for _, sound := range got.Sounds {
			sounds = append(sounds, sound.ID)
		}
		if !slices.Equal(sounds, r.Sounds) {
			t.Fatal(i, sounds, r.Sounds)
		}
		_, e = OriginalContactProfile(ContactProfileKey{Kind: 8, Action: uint8(s.Ball.Action), Frame: s.Ball.Animation.Cursor})
		if e != nil {
			t.Fatal("missing original special ball geometry", e)
		}
	}
}
