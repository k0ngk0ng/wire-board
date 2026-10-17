package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type ball19Oracle struct {
	worldActionFixture
	Prepared, AfterMotion *WorldActionState
	Sounds                []uint8
	Label                 string
}
type ball19Chain struct {
	Left, Entered bool
	Latch         uint8
	Records       []ball19Oracle
}

func ball19Oracles(t *testing.T) ([]ball19Oracle, []ball19Chain) {
	t.Helper()
	var f struct {
		Calls  []ball19Oracle
		Chains []ball19Chain
	}
	readPropAttackFixture(t, "testdata/ball19.json.gz", &f)
	if len(f.Calls) != 816 || len(f.Chains) != 8 {
		t.Fatal("incomplete ball19 source fixture", len(f.Calls), len(f.Chains))
	}
	return f.Calls, f.Chains
}
func applyBall19Test(s *WorldActionState, p OriginalBall19Step) error {
	if e := commitWorldTeamSpecial(s, OriginalTeamSpecialBallStep{OriginalShotBallStep: p.OriginalShotBallStep}); e != nil {
		return e
	}
	if p.ChangeContactLatch {
		s.ContactLatch = p.ContactLatch
	}
	return nil
}
func stepBall19Test(s *WorldActionState, c WorldActionContext) ([]uint8, error) {
	p, e := PrepareOriginalBall19(s.Ball, worldTeamSpecialContext(*s, c), PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
	if e != nil {
		return nil, e
	}
	if e = applyBall19Test(s, p); e != nil {
		return nil, e
	}
	q, e := CompleteOriginalBall19(p, NewYorkBallCourt(), func() uint8 { return s.ContactLatch })
	if e != nil {
		return nil, e
	}
	if e = applyBall19Test(s, q); e != nil {
		return nil, e
	}
	return append(p.SoundIDs, q.SoundIDs...), nil
}
func TestOriginalBall19Calls(t *testing.T) {
	rows, _ := ball19Oracles(t)
	for i, r := range rows {
		s := r.Before
		p, e := PrepareOriginalBall19(s.Ball, worldTeamSpecialContext(s, r.context(t)), PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		if e = applyBall19Test(&s, p); e != nil {
			t.Fatal(e)
		}
		if r.Prepared == nil {
			t.Fatal("missing preparation", i)
		}
		if s != *r.Prepared {
			t.Fatalf("%d %s prepare: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(*r.Prepared), "world"))
		}
		q, e := CompleteOriginalBall19(p, NewYorkBallCourt(), func() uint8 { return s.ContactLatch })
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		if e = applyBall19Test(&s, q); e != nil {
			t.Fatal(e)
		}
		if s != r.After {
			t.Fatalf("%d %s complete: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(append(p.SoundIDs, q.SoundIDs...), r.Sounds) {
			t.Fatal("sounds", i)
		}
	}
}
func TestOriginalBall19ContinuousChains(t *testing.T) {
	_, chains := ball19Oracles(t)
	for _, chain := range chains {
		s := chain.Records[0].Before
		c := chain.Records[0].context(t)
		turned := false
		for i, r := range chain.Records {
			var sounds []uint8
			var e error
			if s.Ball.Action == 19 {
				sounds, e = stepBall19Test(&s, c)
			} else {
				sounds, e = advanceWorldBallStep(&s, c, NewYorkBallCourt())
			}
			if e != nil {
				t.Fatal(chain.Left, chain.Entered, chain.Latch, i, e)
			}
			if s != r.After {
				t.Fatalf("left=%v entered=%v latch=%d tick=%d: %s", chain.Left, chain.Entered, chain.Latch, i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
			}
			if !slices.Equal(sounds, r.Sounds) {
				t.Fatal("sounds", i)
			}
			turned = turned || s.Ball.Action == 10
			encoded, e := json.Marshal(s)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(encoded, &s); e != nil {
				t.Fatal(e)
			}
		}
		if s.Ball.Action != 6 {
			t.Fatal("chain failed to land")
		}
		if chain.Entered && chain.Latch == 0 && !turned {
			t.Fatal("chain omitted countdown transition")
		}
	}
}
func TestOriginalBall19RejectInvalidContext(t *testing.T) {
	p := OriginalBall19Step{OriginalShotBallStep: OriginalShotBallStep{Ball: Ball{Action: 18}}}
	if _, e := PrepareOriginalBall19(p.Ball, OriginalShotBallContext{}, PassLaunchContext{}); e == nil {
		t.Fatal("invalid action accepted")
	}
	p.Ball.Action = 19
	if _, e := CompleteOriginalBall19(p, NewYorkBallCourt(), nil); e == nil {
		t.Fatal("missing post-motion latch accepted")
	}
}
