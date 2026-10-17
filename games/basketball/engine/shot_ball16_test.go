package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type ball16Oracle struct {
	worldActionFixture
	Prepared, AfterMotion *WorldActionState
	Sounds                []uint8
	Label                 string
}
type ball16Chain struct {
	Left    bool
	Height  int
	Records []ball16Oracle
}

func ball16Oracles(t *testing.T) ([]ball16Oracle, []ball16Chain) {
	t.Helper()
	var f struct {
		Calls  []ball16Oracle
		Chains []ball16Chain
	}
	readPropAttackFixture(t, "testdata/ball16.json.gz", &f)
	if len(f.Calls) != 370 || len(f.Chains) != 6 {
		t.Fatal("incomplete original ball16 oracle", len(f.Calls), len(f.Chains))
	}
	return f.Calls, f.Chains
}
func applyBall16Step(s *WorldActionState, step OriginalShotBallStep) {
	dummy := uint8(0)
	applyTeamSpecialStep(s, &dummy, OriginalTeamSpecialBallStep{OriginalShotBallStep: step})
	if step.ClearUnownedSpecialShot {
		s.UnownedSpecialShot = 0
	}
}
func runBall16Oracle(t *testing.T, s WorldActionState, r ball16Oracle) WorldActionState {
	t.Helper()
	p, e := PrepareOriginalBall16(s.Ball, worldTeamSpecialContext(s, r.context(t)), PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
	if e != nil {
		t.Fatal(r.Label, e)
	}
	applyBall16Step(&s, p)
	if r.Prepared == nil {
		t.Fatal("missing prepared original state")
	}
	if s != *r.Prepared {
		t.Fatal(r.Label, "prepare", firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(*r.Prepared), "world"))
	}
	out, e := CompleteOriginalBall16(p, NewYorkBallCourt(), func() int8 { return s.SelectedHoop })
	if e != nil {
		t.Fatal(r.Label, e)
	}
	applyBall16Step(&s, out)
	if s != r.After {
		t.Fatal(r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
	}
	if !slices.Equal(append(p.SoundIDs, out.SoundIDs...), r.Sounds) {
		t.Fatal("sounds", r.Label, p.SoundIDs, out.SoundIDs, r.Sounds)
	}
	return s
}
func TestOriginalBall16Calls(t *testing.T) {
	rows, _ := ball16Oracles(t)
	for _, r := range rows {
		t.Run(r.Label, func(t *testing.T) { runBall16Oracle(t, r.Before, r) })
	}
}
func TestOriginalBall16ContinuousChains(t *testing.T) {
	_, chains := ball16Oracles(t)
	for _, chain := range chains {
		s := chain.Records[0].Before
		turned := false
		for i, r := range chain.Records {
			s = runBall16Oracle(t, s, r)
			turned = turned || s.Ball.Body.Velocity.Height < 0
			data, e := json.Marshal(s)
			if e != nil {
				t.Fatal(e)
			}
			var restored WorldActionState
			if e = json.Unmarshal(data, &restored); e != nil || restored != s {
				t.Fatal("restore", chain.Left, chain.Height, i, e)
			}
			s = restored
		}
		if !turned || s.Ball.Action != 6 {
			t.Fatal("missing turn/landing", chain.Left, chain.Height)
		}
	}
}
