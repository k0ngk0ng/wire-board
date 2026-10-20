package fortressblue

// Isolated physics failure scenarios, specified before implementation: damage
// credited to victim/self; fuel rounding gives free movement; wall penetration;
// out-of-world shell converted into a clamped edge explosion.
import (
	"encoding/json"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"math"
	"testing"
	"time"
)

func TestDamageCreditExcludesVictimAndSelf(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	_, s = selectAll(t, s, m, []int{1, 1})
	target := tankCenter(s.Tanks[1])
	hp := s.Tanks[1].HP
	applyBlast(s, 0, Weapon{Damage: 300, Blast: 14}, target, "", 1, 1)
	if s.Tanks[1].Damage != 0 || s.Tanks[0].Damage != hp-s.Tanks[1].HP {
		t.Fatal("damage credited to victim or incorrect shooter")
	}
	before := s.Tanks[0].Damage
	applyBlast(s, 0, Weapon{Damage: 300, Blast: 14}, tankCenter(s.Tanks[0]), "", 2, 0)
	if s.Tanks[0].Damage != before {
		t.Fatal("self damage scored")
	}
}
func TestMoveNeverExceedsRemainingFuel(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	_, s = selectAll(t, s, m, []int{1, 1})
	s.Tanks[0].X = 300
	s.Tanks[0].Y = 808
	s.Tanks[0].Fuel = .2
	x := s.Tanks[0].X
	moveTank(s, 0, 1, 1)
	if math.Abs(s.Tanks[0].X-x) > .200001 {
		t.Fatal("movement exceeded fuel")
	}
}
func TestOutOfBoundsShotNeverCreatesEdgeBlast(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	_, s = selectAll(t, s, m, []int{1, 1})
	s.Tanks[0].X = 4
	s.Tanks[0].Y = 100
	s.Tanks[1].X = 10
	s.Tanks[1].Y = 100
	s.Wind = 0
	hp := s.Tanks[1].HP
	resolveShot(s, 0, Weapon{Damage: 300, Blast: 14}, "", 0, 100, -1, time.UnixMilli(1700000002000))
	for i := 0; i < 10; i++ {
		m = mustMatch(t, s)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(m.Data, &raw)
		if string(raw["resolution"]) == "" || string(raw["resolution"]) == "null" {
			break
		}
		n, e := onTimer(ctx(2, -1, time.UnixMilli(m.SystemDeadline)), m)
		if e != nil {
			t.Fatal(e)
		}
		s, _ = decode(n)
	}
	if len(s.Craters) != 0 {
		t.Fatal("out-of-bounds shell created crater")
	}
	for _, e := range s.Events {
		if e.Type == "hit" || e.Type == "explode" {
			t.Fatal("out-of-bounds shell hit an edge target")
		}
	}
	_ = hp
}
func TestWalkDoesNotEnterHighWall(t *testing.T) {
	m := fbmaps.Map{Width: 160, Height: 160, MaskCols: 40, MaskRows: 40}
	mask := make([]byte, 200)
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if y >= 30 || (x >= 20 && y >= 15) {
				idx := y*40 + x
				mask[idx>>3] |= 1 << uint(7-(idx&7))
			}
		}
	}
	if _, ok := walkGround(m, mask, 77, 120, 81); ok {
		t.Fatal("walked into a sixty-pixel wall")
	}
	if y, ok := walkGround(m, mask, 40, 120, 41); !ok || y != 120 {
		t.Fatal("flat ground was blocked")
	}
}
func TestMoveStartsTimedFallWithoutInstantDamage(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	_, s = selectAll(t, s, m, []int{1, 1})
	s.Tanks[0].X = 300
	s.Tanks[0].Y = 700
	s.Tanks[0].Fuel = 10
	hp := s.Tanks[0].HP
	if !moveTank(s, 0, 1, 1700000002000) {
		t.Fatal("move rejected")
	}
	if s.Tanks[0].Y != 700 || s.Tanks[0].HP != hp {
		t.Fatal("move teleported or damaged before landing")
	}
	raw, _ := json.Marshal(s)
	var w struct {
		Resolution *struct {
			Stage string
			Falls []struct {
				Seat       int
				FromY, ToY float64
				At, EndAt  int64
			}
		}
	}
	_ = json.Unmarshal(raw, &w)
	if w.Resolution == nil || w.Resolution.Stage != "settling" || len(w.Resolution.Falls) == 0 {
		t.Fatal("move omitted timed falling path")
	}
}
