package fortressblue

import (
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"math"
	"time"
)

// These are explicitly project tuning parameters. Original source establishes
// ground fire/poison and periodic damage, not these exact radii/lifetimes/damage.
const hazardTickMS int64 = 1000

func addGroundHazard(s *State, source int, kind string, impact Point, at int64) {
	m, _ := fbmaps.ByID(s.MapID)
	ground := groundY(m, maskFor(m, s.Craters), impact.X, impact.Y-4)
	if ground < 0 || ground > float64(m.Height) {
		return
	}
	radius, lifetime := 40.0, int64(8000)
	if kind == "poison" {
		radius, lifetime = 50, 12000
	}
	s.emit("hazard", source, -1, impact.X, ground, 0, kind, at)
	h := Hazard{ID: s.Sequence, Kind: kind, X: impact.X, Y: ground, R: radius, At: at, EndAt: at + lifetime, NextTickAt: at + hazardTickMS, Source: source}
	if len(s.Hazards) >= 64 {
		s.Hazards = append([]Hazard(nil), s.Hazards[len(s.Hazards)-63:]...)
	}
	s.Hazards = append(s.Hazards, h)
}

func (s *State) hazardDeadline() int64 {
	next := int64(0)
	for _, h := range s.Hazards {
		at := min(h.NextTickAt, h.EndAt)
		if next == 0 || at < next {
			next = at
		}
	}
	return next
}

// Ground effects stay attached to their original surface. Removing that
// surface extinguishes them instead of making them hit another floor below.
func extinguishUnsupportedHazards(s *State) {
	if len(s.Hazards) == 0 {
		return
	}
	m, _ := fbmaps.ByID(s.MapID)
	mask := maskFor(m, s.Craters)
	keep := s.Hazards[:0]
	for _, h := range s.Hazards {
		ground := groundY(m, mask, h.X, h.Y-8)
		if math.Abs(ground-h.Y) <= 8 {
			keep = append(keep, h)
		}
	}
	s.Hazards = keep
}

// Mirrors the published falls contract: X is linear, Y follows t². HP is never
// projected from old snapshots, so ongoing hazard damage survives later impact.
func tankPositionAt(s *State, seat int, at int64) Point {
	t := s.Tanks[seat]
	if s.Resolution != nil {
		for _, f := range s.Resolution.Falls {
			if f.Seat != seat || f.Landed || at < f.At {
				continue
			}
			p := clamp(float64(at-f.At)/float64(f.EndAt-f.At), 0, 1)
			return Point{X: f.FromX + (f.ToX-f.FromX)*p, Y: f.FromY + (f.ToY-f.FromY)*p*p}
		}
	}
	return Point{X: t.X, Y: t.Y}
}

func hazardDamage(s *State, h Hazard, seat int, at int64) {
	target := &s.Tanks[seat]
	point := tankPositionAt(s, seat, at)
	if !target.Alive || math.Abs(point.X-h.X) > h.R || point.Y < h.Y-24 || point.Y > h.Y+12 {
		return
	}
	damage := 18
	if h.Kind == "poison" {
		damage = 25
	}
	if h.Source != seat && s.Tanks[h.Source].Team != target.Team {
		damage = shieldedDamage(target, damage)
	}
	damage = min(target.HP, damage)
	if damage == 0 {
		s.emit("shield", h.Source, seat, point.X, point.Y, 0, h.Kind, at)
		return
	}
	target.HP -= damage
	if h.Source != seat && s.Tanks[h.Source].Team != target.Team {
		s.Tanks[h.Source].Damage += damage
		s.DamageTotal[h.Source] += damage
	}
	s.emit("status", h.Source, seat, point.X, point.Y, damage, h.Kind, at)
	if target.HP > 0 {
		return
	}
	target.HP, target.Alive, target.Deaths = 0, false, target.Deaths+1
	target.X, target.Y = point.X, point.Y
	if h.Source != seat && s.Tanks[h.Source].Team != target.Team {
		s.Tanks[h.Source].Kills++
	}
	if s.Resolution != nil {
		for i := range s.Resolution.Falls {
			f := &s.Resolution.Falls[i]
			if f.Seat == seat && !f.Landed {
				f.ToX, f.ToY, f.EndAt, f.Landed = point.X, point.Y, at, true
			}
		}
	}
	s.emit("death", h.Source, seat, point.X, point.Y, 0, h.Kind, at)
	if s.LastShot != nil && s.LastShot.Seat == seat {
		cancelUnlaunchedShot(s, at)
	}
}

func tickHazards(s *State, at int64) {
	extinguishUnsupportedHazards(s)
	keep := s.Hazards[:0]
	for _, h := range s.Hazards {
		if at >= h.EndAt {
			continue
		}
		if at >= h.NextTickAt {
			for seat := range s.Tanks {
				hazardDamage(s, h, seat, h.NextTickAt)
			}
			h.NextTickAt += hazardTickMS
		}
		keep = append(keep, h)
	}
	s.Hazards = keep
	if s.Resolution == nil {
		if s.checkTeamOutcome(at) {
			return
		}
		if !s.Tanks[s.Current].Alive {
			s.needsAdvance = true
			s.resumeMovement = false
		}
	}
}

// Replay due simulation nodes in timestamp order, then grant actionable time
// relative to the observed clock. A late scheduler never shortens the next turn.
func processWorldTimeline(s *State, now time.Time) {
	for count := 0; count < 8192 && !s.Finished; count++ {
		next := s.hazardDeadline()
		resolution := s.resolutionDeadline()
		if resolution > 0 && (next == 0 || resolution < next) {
			next = resolution
		}
		if supply := s.supplyDeadline(); supply > 0 && (next == 0 || supply < next) {
			next = supply
		}
		if next == 0 || next > now.UnixMilli() {
			break
		}
		// At an identical timestamp, land/impact precedes ground contact damage.
		if resolution > 0 && resolution == next {
			processResolution(s, time.UnixMilli(next))
		}
		if s.Finished {
			break
		}
		if hazard := s.hazardDeadline(); hazard > 0 && hazard <= next {
			tickHazards(s, next)
		}
		processSupplies(s, next)
		if s.Resolution == nil && s.checkTeamOutcome(next) {
			break
		}
	}
	processSupplies(s, now.UnixMilli())
	if s.resumeSupply {
		s.Deadline = now.UnixMilli() + int64(s.Options.TurnSeconds)*1000
		s.resumeSupply = false
	}
	if s.Finished || s.Resolution != nil || !s.needsAdvance {
		return
	}
	s.needsAdvance = false
	if s.resumeMovement && s.Tanks[s.Current].Alive {
		s.Deadline = now.UnixMilli() + s.MoveRemainingMS
		s.MoveRemainingMS = 0
		s.resumeMovement = false
		return
	}
	s.resumeMovement = false
	s.MoveRemainingMS = 0
	advanceTurn(s, now)
}
