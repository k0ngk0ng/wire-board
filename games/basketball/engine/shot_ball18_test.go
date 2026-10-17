package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type ball18Oracle struct {
	worldActionFixture
	Prepared, AfterMotion *WorldActionState
	Sounds                []uint8
	Label                 string
}
type ball18Chain struct {
	Left    bool
	Seed    RandomState
	Records []ball18Oracle
}

func ball18Oracles(t *testing.T) ([]ball18Oracle, []ball18Chain) {
	t.Helper()
	var f struct {
		Calls  []ball18Oracle
		Chains []ball18Chain
	}
	readPropAttackFixture(t, "testdata/ball18.json.gz", &f)
	if len(f.Calls) != 436 || len(f.Chains) != 8 {
		t.Fatal("incomplete original ball18 oracle")
	}
	return f.Calls, f.Chains
}
func applyBall18Step(s *WorldActionState, step OriginalBall18Step) {
	dummy := uint8(0)
	applyTeamSpecialStep(s, &dummy, OriginalTeamSpecialBallStep{OriginalShotBallStep: step.OriginalShotBallStep})
	if step.ClearUnownedSpecialShot {
		s.UnownedSpecialShot = 0
	}
	if step.ChangeContactLatch {
		s.ContactLatch = step.ContactLatch
	}
	if step.ChangeRandom {
		s.Random = step.Random
	}
}
func TestOriginalBall18Calls(t *testing.T) {
	rows, _ := ball18Oracles(t)
	for i, r := range rows {
		s := r.Before
		p, e := PrepareOriginalBall18(s.Ball, worldTeamSpecialContext(s, r.context(t)), PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		applyBall18Step(&s, p)
		if r.Prepared == nil || s != *r.Prepared {
			t.Fatalf("case %d %s prepare: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(*r.Prepared), "world"))
		}
		out, e := CompleteOriginalBall18(p, NewYorkBallCourt(), func() Ball18TailContext {
			return Ball18TailContext{ContactLatch: s.ContactLatch, Random: s.Random, SelectedHoop: s.SelectedHoop}
		})
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		applyBall18Step(&s, out)
		if s != r.After {
			t.Fatalf("case %d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(append(p.SoundIDs, out.SoundIDs...), r.Sounds) {
			t.Fatal("sounds", i, p.SoundIDs, out.SoundIDs, r.Sounds)
		}
	}
}
func TestOriginalBall18ContinuousChains(t *testing.T) {
	_, chains := ball18Oracles(t)
	for _, chain := range chains {
		s := chain.Records[0].Before
		c := chain.Records[0].context(t)
		crossed := false
		finished := false
		for i, r := range chain.Records {
			result := WorldActionStep{State: s}
			var e error
			if s.Ball.Action == 18 {
				e = advanceWorldBall18(&result, c, NewYorkBallCourt())
			} else {
				result.SoundIDs, e = advanceWorldBallStep(&result.State, c, NewYorkBallCourt())
			}
			if e != nil {
				t.Fatal(chain.Left, chain.Seed, i, e)
			}
			if result.State != r.After {
				t.Fatalf("left %v seed %x tick %d: %s", chain.Left, chain.Seed, i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
			}
			if !slices.Equal(result.SoundIDs, r.Sounds) {
				t.Fatal("sounds", i, result.SoundIDs, r.Sounds)
			}
			crossed = crossed || result.State.Ball.Action == 18 && result.State.Ball.Animation.Cursor == 7
			finished = finished || result.State.Ball.Action == 10
			encoded, _ := json.Marshal(result.State)
			if e = json.Unmarshal(encoded, &s); e != nil {
				t.Fatal(e)
			}
		}
		if !crossed || !finished || s.Ball.Action != 6 {
			t.Fatal("chain skipped original transition", chain.Left, chain.Seed)
		}
	}
}
func TestOriginalBall18RealFailureContinuation(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/ball18-runtime-before.json.gz", &s)
	if s.VideoFrames != 9449 {
		t.Fatal("wrong retained first failure", s.VideoFrames)
	}
	for i := 0; i < 180; i++ {
		data, _ := json.Marshal(s)
		var restored RuntimeState
		if e := json.Unmarshal(data, &restored); e != nil {
			t.Fatal(e)
		}
		a, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(i, s.VideoFrames, e)
		}
		b, e := StepVideo(restored, [4]Buttons{})
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("JSON replay", i, e)
		}
		s = a.State
	}
}
