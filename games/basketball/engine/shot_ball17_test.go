package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type ball17Oracle struct {
	worldActionFixture
	Prepared, AfterMotion *WorldActionState
	Sounds                []uint8
	Label                 string
}
type ball17Chain struct {
	Left    bool
	Variant string
	Records []ball17Oracle
}

func ball17Oracles(t *testing.T) ([]ball17Oracle, []ball17Chain) {
	t.Helper()
	var f struct {
		Calls  []ball17Oracle
		Chains []ball17Chain
	}
	readPropAttackFixture(t, "testdata/ball17.json.gz", &f)
	if len(f.Calls) != 60 || len(f.Chains) != 6 {
		t.Fatal("incomplete ball17 oracle", len(f.Calls), len(f.Chains))
	}
	return f.Calls, f.Chains
}
func TestOriginalBall17Calls(t *testing.T) {
	rows, _ := ball17Oracles(t)
	for i, r := range rows {
		s := r.Before
		p, e := PrepareOriginalBall17(s.Ball, worldTeamSpecialContext(s, r.context(t)))
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		if e = commitWorldTeamSpecial(&s, OriginalTeamSpecialBallStep{OriginalShotBallStep: p}); e != nil {
			t.Fatal(e)
		}
		if r.Prepared == nil || s != *r.Prepared {
			t.Fatalf("case %d %s prepare: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(*r.Prepared), "world"))
		}
		s.Ball, e = CompleteOriginalBall17(p.Ball, NewYorkBallCourt())
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		if s != r.After {
			t.Fatalf("case %d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(p.SoundIDs, r.Sounds) {
			t.Fatal(i, "sounds", p.SoundIDs, r.Sounds)
		}
	}
}
func TestOriginalBall17ContinuousChains(t *testing.T) {
	_, chains := ball17Oracles(t)
	for _, chain := range chains {
		s := chain.Records[0].Before
		transitioned := false
		for i, r := range chain.Records {
			out := WorldActionStep{State: s}
			var e error
			if s.Ball.Action == 17 {
				e = advanceWorldBall17(&out, r.context(t), NewYorkBallCourt())
			} else if s.Ball.Action == 12 {
				e = advanceWorldTeamSpecialBall(&out, r.context(t), NewYorkBallCourt())
			} else {
				t.Fatal("unexpected chain action", s.Ball.Action)
			}
			if e != nil {
				t.Fatal(chain.Left, chain.Variant, i, e)
			}
			if out.State != r.After {
				t.Fatalf("left%v %s tick%d: %s", chain.Left, chain.Variant, i, firstWorldLoopDifference(reflect.ValueOf(out.State), reflect.ValueOf(r.After), "world"))
			}
			if !slices.Equal(out.SoundIDs, r.Sounds) {
				t.Fatal("sounds", i, out.SoundIDs, r.Sounds)
			}
			transitioned = transitioned || out.State.Ball.Action == 12
			data, e := json.Marshal(out.State)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(data, &s); e != nil {
				t.Fatal(e)
			}
		}
		if !transitioned || s.Ball.Action != 6 {
			t.Fatal("missing original animation transition or bounce")
		}
	}
}
func TestOriginalBall17RejectsInvalidInputs(t *testing.T) {
	rows, _ := ball17Oracles(t)
	s := rows[0].Before
	for _, a := range []BallAction{0, 16, 18, 255} {
		b := s.Ball
		b.Action = a
		got, e := PrepareOriginalBall17(b, worldTeamSpecialContext(s, rows[0].context(t)))
		if e == nil || got.Ball != b {
			t.Fatal("invalid action accepted or mutated", a, e)
		}
	}
	b := s.Ball
	b.Entered = true
	b.Animation.Cursor = 4
	got, e := CompleteOriginalBall17(b, NewYorkBallCourt())
	if e == nil || got != b {
		t.Fatal("invalid cursor accepted or mutated", e)
	}
}
