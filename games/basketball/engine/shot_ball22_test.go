package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type ball22Oracle struct {
	worldActionFixture
	Prepared *WorldActionState
	Sounds   []uint8
	Label    string
}
type ball22Chain struct {
	Left    bool
	Height  int
	Records []ball22Oracle
}

func ball22Oracles(t *testing.T) ([]ball22Oracle, []ball22Chain) {
	t.Helper()
	var f struct {
		Calls  []ball22Oracle
		Chains []ball22Chain
	}
	readPropAttackFixture(t, "testdata/ball22.json.gz", &f)
	if len(f.Calls) != 340 || len(f.Chains) != 8 {
		t.Fatal("incomplete ball22 oracle", len(f.Calls), len(f.Chains))
	}
	return f.Calls, f.Chains
}
func applyBall22Step(s *WorldActionState, p OriginalShotBallStep) {
	dummy := uint8(0)
	applyTeamSpecialStep(s, &dummy, OriginalTeamSpecialBallStep{OriginalShotBallStep: p})
}
func runBall22Oracle(t *testing.T, s WorldActionState, r ball22Oracle) WorldActionState {
	t.Helper()
	p, e := PrepareOriginalBall22(s.Ball, worldTeamSpecialContext(s, r.context(t)))
	if e != nil {
		t.Fatal(r.Label, e)
	}
	applyBall22Step(&s, p)
	if r.Prepared == nil || s != *r.Prepared {
		t.Fatalf("%s prepare: %s", r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(*r.Prepared), "world"))
	}
	done, e := CompleteOriginalBall22(p, NewYorkBallCourt(), func() int8 { return s.SelectedHoop })
	if e != nil {
		t.Fatal(r.Label, e)
	}
	applyBall22Step(&s, done)
	if s != r.After {
		t.Fatalf("%s result: %s", r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
	}
	if !slices.Equal(append(p.SoundIDs, done.SoundIDs...), r.Sounds) {
		t.Fatal("sounds", r.Label, p.SoundIDs, done.SoundIDs, r.Sounds)
	}
	return s
}
func TestOriginalBall22Calls(t *testing.T) {
	rows, _ := ball22Oracles(t)
	for _, r := range rows {
		runBall22Oracle(t, r.Before, r)
	}
}
func TestOriginalBall22ContinuousChains(t *testing.T) {
	_, chains := ball22Oracles(t)
	for _, chain := range chains {
		s := chain.Records[0].Before
		crossed := false
		for i, r := range chain.Records {
			if s != r.Before {
				t.Fatal("chain continuity", chain.Left, chain.Height, i)
			}
			if s.Ball.Action == 22 {
				s = runBall22Oracle(t, s, r)
			} else {
				result := WorldActionStep{State: s}
				e := advanceWorldTeamSpecialBall(&result, r.context(t), NewYorkBallCourt())
				s = result.State
				sounds := result.SoundIDs
				if e != nil {
					t.Fatal(i, e)
				}
				if s != r.After {
					t.Fatalf("chain %d: %s", i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
				}
				if !slices.Equal(sounds, r.Sounds) {
					t.Fatal("chain sounds", i, sounds, r.Sounds)
				}
			}
			crossed = crossed || s.Ball.Animation.Cursor < 4
			encoded, e := json.Marshal(s)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(encoded, &s); e != nil {
				t.Fatal(e)
			}
		}
		if !crossed || s.Ball.Action != 6 {
			t.Fatal("missing turn/landing", chain.Left, chain.Height)
		}
	}
}
