package snooker

// New referee helpers specified before implementation: removing a nominated
// free ball hides its role when a farther ball also blocks; nearest/equidistant
// blockers matter; different reds can have no common effective blocker; the
// red full-diameter clause differs from mere centre-line availability; all
// colours that could be on after a red count for Miss eligibility.
import (
	"reflect"
	"testing"
)

func TestEffectiveSnookeringBallIsNearestNotOnlyNecessary(t *testing.T) {
	bs := []Ball{testBall(0, .4, .8), testBall(1, 2, .8), testBall(19, 1, .8), testBall(21, 1.3, .8)}
	if got := effectiveSnookeringBalls(bs, []int{1}); !reflect.DeepEqual(got, []int{19}) {
		t.Fatal(got)
	}
	bs[2].Pos = Vec{1, .77}
	bs[3].Pos = Vec{1, .83}
	if got := effectiveSnookeringBalls(bs, []int{1}); !reflect.DeepEqual(got, []int{19, 21}) {
		t.Fatal("equidistant blockers", got)
	}
	bs = []Ball{testBall(0, .4, .8), testBall(1, 2, .5), testBall(2, 2, 1.1), testBall(19, 1, .67), testBall(21, 1, .93)}
	if got := effectiveSnookeringBalls(bs, []int{1, 2}); len(got) != 0 {
		t.Fatal("different reds blocked by different colours", got)
	}
}
func TestMissFullRedRequiresUnobstructedDiameter(t *testing.T) {
	c := RuleContext{Target: "red", Clear: 2, Balls: []Ball{testBall(0, .4, .8), testBall(1, 2, .8), testBall(21, 1, .858)}}
	any, full, _ := missContact(c, Strike{}, []int{1})
	if !any || full {
		t.Fatal("partial red access counted as full diameter", any, full)
	}
	c.Balls = InitialBalls()
	on := onBalls(c, 0)
	any, full, _ = missContact(c, Strike{}, on)
	if !any || !full {
		t.Fatal("reds themselves incorrectly obstructed full diameter", any, full)
	}
}
func TestMissAfterRedConsidersEveryColourThatCouldBeOn(t *testing.T) {
	c := RuleContext{Target: "colour", Clear: 2, Balls: []Ball{testBall(0, 1, .889), testBall(1, 1.4, .889), testBall(19, 1.7845, .889), testBall(18, .737, .889)}}
	any, full, targets := missContact(c, Strike{Nominate: 5}, []int{19})
	if !any || !full || len(targets) != 2 {
		t.Fatal("ignored accessible brown after choosing blocked blue", any, full, targets)
	}
}
