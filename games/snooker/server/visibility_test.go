package snooker

// Before adding in-hand visibility / bank escape search: cue.Down suppresses
// all free balls; checking one D position falsely grants them; a sparse grid
// misses narrow clear strips; merely reflecting a ray invents a playable route;
// a fully surrounded cue is always labelled a miss despite having no escape.
import (
	"testing"
)

func screenedD() []Ball {
	bs := []Ball{{ID: 0, Pos: Vec{BaulkX - .15, TableHeight / 2}, Down: true}, testBall(1, 3, TableHeight/2)}
	for i := 0; i < 6; i++ {
		bs = append(bs, testBall(16+i, 1.25, TableHeight/2+(float64(i)-2.5)*.075))
	}
	return bs
}
func TestInHandVisibilityFindsARealWitnessAndDetectsCompleteScreen(t *testing.T) {
	bs := screenedD()
	if _, ok := dVisiblePosition(bs, []int{1}); ok {
		t.Fatal("six-colour screen did not block entire D")
	}
	bs[4].Down = true // remove colour 18; leave only a narrow viewing strip
	bs[3].Pos.Y += .008
	bs[5].Pos.Y -= .008
	p, ok := dVisiblePosition(bs, []int{1})
	if !ok {
		t.Fatal("missed narrow open strip inside D")
	}
	if p.X > BaulkX || p.sub(Vec{BaulkX, TableHeight / 2}).length() > DRadius+1e-9 || !clearSpot(p, bs, 0) {
		t.Fatal("witness is not a legal in-hand placement", p)
	}
	bs[0].Pos = p
	bs[0].Down = false
	if IsSnookered(bs, []int{1}) {
		t.Fatal("witness cannot see both extreme edges")
	}
}
func TestInHandVisibilityDoesNotDependOnArchivedCuePosition(t *testing.T) {
	bs := screenedD()
	bs[4].Down = true
	for _, p := range []Vec{{0, 0}, {3, 1.7}, {BaulkX, .8}} {
		bs[0].Pos = p
		if _, ok := dVisiblePosition(bs, []int{1}); !ok {
			t.Fatal("stored down cue changed D visibility")
		}
	}
}
func TestCushionEscapeRequiresActualReachableFirstContact(t *testing.T) {
	bs := []Ball{testBall(0, .5, .5), testBall(1, 1.5, .5), testBall(21, 1, .5)}
	if !cushionEscape(bs, []int{1}) {
		t.Fatal("missed bank route around central obstruction")
	}
	ring := []Ball{testBall(0, 1, .8), testBall(1, 2, .8)}
	for i, p := range []Vec{{1.0527, .8}, {1.02635, .845639}, {.97365, .845639}, {.9473, .8}, {.97365, .754361}, {1.02635, .754361}} {
		ring = append(ring, testBall(16+i, p.X, p.Y))
	}
	if cushionEscape(ring, []int{1}) {
		t.Fatal("invented a bank route through enclosing colours")
	}
}
