package snooker

// Written before adjudication. See the complete failure inventory in
// docs/snooker-implementation.md. These isolate official edge cases; match and
// HTTP tests separately prove that decisions are committed once and restored.
import (
	"reflect"
	"testing"
)

func rulePosition(target string, clear int, active ...int) RuleContext {
	bs := InitialBalls()
	for i := range bs {
		bs[i].Down = true
	}
	for _, id := range append([]int{0}, active...) {
		bs[id].Down = false
	}
	return RuleContext{Balls: bs, Target: target, Clear: clear, Scores: [2]int{0, 0}, Team: 0}
}
func ruleMotion(c RuleContext, first, pots []int) Motion {
	bs := append([]Ball{}, c.Balls...)
	for _, id := range pots {
		bs[id].Down = true
	}
	return Motion{Balls: bs, First: first, Potted: pots, Off: []int{}}
}
func TestRulesRedColourAndLastRedClearance(t *testing.T) {
	c := rulePosition("red", 2, 1, 2, 16, 17, 18, 19, 20, 21)
	r := Judge(c, Strike{}, ruleMotion(c, []int{1}, []int{1, 2}))
	if r.Foul != 0 || r.Points != 2 || !r.KeepTurn || r.Target != "colour" {
		t.Fatal(r)
	}
	c = rulePosition("colour", 2, 16, 17, 18, 19, 20, 21)
	r = Judge(c, Strike{Nominate: 7}, ruleMotion(c, []int{21}, []int{21}))
	if r.Foul != 0 || r.Points != 7 || r.Target != "clear" || r.Clear != 2 || !reflect.DeepEqual(r.Respot, []int{21}) {
		t.Fatal(r)
	}
	c = rulePosition("clear", 2, 16, 17, 18, 19, 20, 21)
	r = Judge(c, Strike{}, ruleMotion(c, []int{16}, []int{16}))
	if r.Points != 2 || r.Clear != 3 || len(r.Respot) != 0 || !r.KeepTurn {
		t.Fatal(r)
	}
}
func TestRulesFoulDiscardsStrokePointsAndTakesHighestPenalty(t *testing.T) {
	c := rulePosition("red", 2, 1, 2, 21)
	r := Judge(c, Strike{}, ruleMotion(c, []int{21}, []int{1, 21, 0}))
	if r.Foul != 7 || r.Points != 0 || !r.InHand || r.KeepTurn || !reflect.DeepEqual(r.Respot, []int{21}) {
		t.Fatal(r)
	}
	c = rulePosition("colour", 2, 1, 21)
	r = Judge(c, Strike{Nominate: 7}, ruleMotion(c, []int{21}, []int{1}))
	if r.Foul != 7 || r.Points != 0 || len(r.Respot) != 0 {
		t.Fatal(r)
	}
}
func TestRulesFreeBallScoresTargetAndReturnsToSpot(t *testing.T) {
	c := rulePosition("red", 2, 1, 2, 19, 21)
	c.FreeBall = true
	r := Judge(c, Strike{FreeBall: 19}, ruleMotion(c, []int{19}, []int{19, 1}))
	if r.Foul != 0 || r.Points != 2 || r.Target != "colour" || !reflect.DeepEqual(r.Respot, []int{19}) {
		t.Fatal(r)
	}
	c = rulePosition("clear", 2, 16, 19, 21)
	c.FreeBall = true
	r = Judge(c, Strike{FreeBall: 19}, ruleMotion(c, []int{19}, []int{19}))
	if r.Points != 2 || r.Clear != 2 || !r.KeepTurn || !reflect.DeepEqual(r.Respot, []int{19}) {
		t.Fatal(r)
	}
	r = Judge(c, Strike{FreeBall: 19}, ruleMotion(c, []int{19, 16}, []int{19, 16}))
	if r.Points != 2 || r.Clear != 3 || r.Foul != 0 {
		t.Fatal(r)
	}
}
func TestRulesMissAndTouchingBallExceptions(t *testing.T) {
	c := rulePosition("red", 2, 1, 21)
	c.Balls[0].Pos = Vec{.5, .5}
	c.Balls[1].Pos = Vec{1, .5}
	r := Judge(c, Strike{Power: .3}, ruleMotion(c, nil, nil))
	if r.Foul != 4 || !r.Miss || !r.FullBall {
		t.Fatal("clear path miss", r)
	}
	c.Scores = [2]int{0, 80}
	r = Judge(c, Strike{Power: .3}, ruleMotion(c, nil, nil))
	if r.Miss {
		t.Fatal("miss despite needing penalty points", r)
	}
	c.Scores = [2]int{}
	c.Balls[1].Pos = Vec{.5 + 2*BallRadius, .5}
	r = Judge(c, Strike{Angle: 3.141592653589793, Power: .1}, ruleMotion(c, nil, nil))
	if r.Foul != 0 {
		t.Fatal("moving away from touching on-ball", r)
	}
	moved := ruleMotion(c, nil, nil)
	moved.Balls[1].Pos.X += .02
	r = Judge(c, Strike{Power: .1}, moved)
	if r.Foul == 0 {
		t.Fatal("pushed touching ball")
	}
}
func TestRulesSnookerRequiresBothExtremeEdges(t *testing.T) {
	bs := []Ball{testBall(0, .4, .8), testBall(1, 1.8, .8), testBall(21, 1.0, .8)}
	if !IsSnookered(bs, []int{1}) {
		t.Fatal("clear obstruction ignored")
	}
	bs[2].Pos.Y = 1.2
	if IsSnookered(bs, []int{1}) {
		t.Fatal("open ball called snookered")
	}
	// A red does not count as a ball not on when evaluating a snooker on reds.
	bs[2] = testBall(2, 1.0, .8)
	if IsSnookered(bs, []int{1, 2}) {
		t.Fatal("legal red treated as blocking colour")
	}
}
func TestRulesSpottingUsesHighestVacantSpotWithoutTouching(t *testing.T) {
	bs := InitialBalls()
	bs[21].Down = true
	bs[1].Pos = ColourSpot(7)
	bs[20].Down = true
	out, e := SpotColours(bs, []int{21})
	if e != nil {
		t.Fatal(e)
	}
	if out[21].Down || out[21].Pos != ColourSpot(6) || !bs[21].Down {
		t.Fatal("failed occupied-spot rule or mutated original")
	}
	// All six spots are occupied; find the closest point toward the top cushion.
	bs = InitialBalls()
	bs[21].Down = true
	bs[1].Pos = ColourSpot(7)
	out, e = SpotColours(bs, []int{21})
	if e != nil {
		t.Fatal(e)
	}
	if out[21].Pos.X <= ColourSpot(7).X || out[21].Pos.Y != TableHeight/2 {
		t.Fatal(out[21])
	}
	for _, b := range out {
		if !b.Down && b.ID != 21 && b.Pos.sub(out[21].Pos).length() <= 2*BallRadius {
			t.Fatal("respotted touching another ball")
		}
	}
}
