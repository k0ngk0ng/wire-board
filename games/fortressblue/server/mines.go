package fortressblue

import (
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"math"
)

type Mine struct {
	ID        int64   `json:"id"`
	Source    int     `json:"source"`
	Team      int     `json:"team"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Damage    float64 `json:"damage"`
	At        int64   `json:"at"`
	TriggerAt int64   `json:"triggerAt,omitempty"`
}

// Radius/damage scaling are platform values. Native evidence confirms ground
// persistence, same-team visibility, no team trigger exemption and types 8/11
// exempt from contact, but does not provide the original server configuration.
const mineTriggerRadius = 28.0

func plantMine(s *State, seat int, point Point, damage float64, at int64) {
	m, _ := fbmaps.ByID(s.MapID)
	ground := groundY(m, maskFor(m, s.Craters), point.X, point.Y-2)
	if ground < 0 || ground >= float64(m.Height) {
		return
	}
	if len(s.Mines) >= 32 {
		s.Mines = append([]Mine(nil), s.Mines[1:]...)
	}
	d := Mine{ID: s.Sequence + 1, Source: seat, Team: s.Tanks[seat].Team, X: point.X, Y: ground, Damage: damage, At: at}
	s.Mines = append(s.Mines, d)
	s.emit("mine_plant", seat, -1, point.X, ground, 0, "", at)
	for i, t := range s.Tanks {
		if mineContactEligible(t) && math.Hypot(t.X-d.X, t.Y-17-d.Y) < mineTriggerRadius {
			s.Mines[len(s.Mines)-1].TriggerAt = at
			_ = i
			break
		}
	}
}
func mineContactEligible(t Tank) bool { return t.Alive && t.TankID != 8 && t.TankID != 11 }
func segmentDistance(p, a, b Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	den := dx*dx + dy*dy
	t := 0.0
	if den > 0 {
		t = clamp(((p.X-a.X)*dx+(p.Y-a.Y)*dy)/den, 0, 1)
	}
	return math.Hypot(p.X-a.X-t*dx, p.Y-a.Y-t*dy)
}
func latchMineContact(s *State, seat int, from, to Point, at int64) {
	if seat < 0 || seat >= len(s.Tanks) || !mineContactEligible(s.Tanks[seat]) {
		return
	}
	from.Y -= 17
	to.Y -= 17
	for i := range s.Mines {
		d := &s.Mines[i]
		if d.TriggerAt == 0 && segmentDistance(Point{X: d.X, Y: d.Y}, from, to) < mineTriggerRadius {
			d.TriggerAt = at + 180
		}
	}
}

// Static contact is checked at resolved landing boundaries as well as creation.
func probeMineContacts(s *State, at int64) {
	for i := range s.Mines {
		d := &s.Mines[i]
		if d.TriggerAt != 0 {
			continue
		}
		for _, t := range s.Tanks {
			if mineContactEligible(t) && math.Hypot(t.X-d.X, t.Y-17-d.Y) < mineTriggerRadius {
				d.TriggerAt = at
				break
			}
		}
	}
}
func (s *State) mineDeadline() int64 {
	next := int64(0)
	for _, d := range s.Mines {
		if d.TriggerAt > 0 && (next == 0 || d.TriggerAt < next) {
			next = d.TriggerAt
		}
	}
	return next
}
func tickMines(s *State, at int64) {
	if s.Finished || len(s.Mines) == 0 {
		return
	}
	m, _ := fbmaps.ByID(s.MapID)
	mask := maskFor(m, s.Craters)
	keep := s.Mines[:0]
	due := []Mine{}
	for _, d := range s.Mines {
		unsupported := math.Abs(groundY(m, mask, d.X, d.Y-1)-d.Y) > 4
		if unsupported || (d.TriggerAt > 0 && d.TriggerAt <= at) {
			due = append(due, d)
		} else {
			keep = append(keep, d)
		}
	}
	s.Mines = keep
	for _, d := range due {
		when := at
		if d.TriggerAt > 0 && d.TriggerAt <= at {
			when = d.TriggerAt
		}
		b := applyBlast(s, d.Source, Weapon{Damage: d.Damage, Blast: 14, FriendlyFire: true}, Point{X: d.X, Y: d.Y}, "mine", when, -1)
		s.MineBlasts = append(s.MineBlasts, b)
		s.emit("explode", d.Source, -1, d.X, d.Y, 0, "mine", when)
	}
	if len(s.MineBlasts) > 32 {
		s.MineBlasts = append([]Blast(nil), s.MineBlasts[len(s.MineBlasts)-32:]...)
	}
	if len(due) > 0 {
		tickMines(s, at)
	}
	if len(due) > 0 && s.Resolution == nil {
		shotID, started := int64(0), at
		if s.needsAdvance && !s.resumeMovement && s.LastShot != nil {
			shotID = s.LastShot.ID
			started = s.LastShot.At
		}
		if s.MoveRemainingMS <= 0 {
			s.MoveRemainingMS = max(1, s.Deadline-at)
		}
		beginFalls(s, at, shotID, started)
	}
}
