package starfort

import "math"

func (s *State) expectedBaseMaxHP() int {
	if s.tuned() {
		return 30 + s.Wave/10
	}
	return 30
}
func (s *State) quakeCraterFactor() float64 {
	if s.tuned() {
		return 1.8
	}
	return 1.3
}
func (s *State) wetFreezeMS() int64 {
	if s.tuned() {
		return 2000
	}
	return 1000
}
func (s *State) wetSlowBonus() float64 {
	if s.tuned() {
		return .5
	}
	return .2
}
func (s *State) shieldCost() int {
	if s.tuned() {
		return 700
	}
	return 1000
}
func (s *State) revisionHeartBonus() float64 {
	if s.tuned() {
		return 1.3
	}
	return 1.1
}
func (s *State) magePowerRate() float64 {
	if s.tuned() {
		return .6
	}
	return .8
}
func (s *State) mageMagicDescription() string {
	if s.tuned() {
		return "伤害 ×0.8，攻速 ×0.6；"
	}
	return ""
}

// Persist the initial bank selection in the map. Search only the inward half
// plane, keeping the existing road clearance, spacing, IDs and highground count.
// At a radial tangent both banks tie; deterministic candidate order breaks it.
func (s *State) inwardPads(m *Map, r Route) {
	length := pathDistance(r.Points)
	for i := 0; i < 6 && len(m.Pads) < 40; i++ {
		d := length * float64(i+1) / 7
		a, b := pathPosition(r.Points, d), pathPosition(r.Points, d+1)
		dx, dy := b.X-a.X, b.Y-a.Y
		norm := math.Hypot(dx, dy)
		if norm == 0 {
			continue
		}
		nx, ny := -dy/norm, dx/norm
		if nx*(m.Base.X-a.X)+ny*(m.Base.Y-a.Y) < 0 {
			nx, ny = -nx, -ny
		}
		origin := Point{a.X + nx*70, a.Y + ny*70}
		best := math.Inf(1)
		chosen := origin
		for x := 35.; x <= float64(m.Width)-35; x += 15 {
			for y := 35.; y <= float64(m.Height)-35; y += 15 {
				if (x-a.X)*nx+(y-a.Y)*ny < 0 {
					continue
				}
				score := math.Hypot(x-origin.X, y-origin.Y)
				if score >= best {
					continue
				}
				p := Point{x, y}
				distance := roadDistance(*m, p)
				if distance < 48 || distance > 185 || math.Hypot(x-m.Base.X, y-m.Base.Y) < 60 {
					continue
				}
				clash := false
				for _, q := range m.Pads {
					if math.Hypot(q.X-x, q.Y-y) < 60 {
						clash = true
						break
					}
				}
				if !clash {
					best, chosen = score, p
				}
			}
		}
		terrain := "plain"
		if i == 0 {
			terrain = "highground"
		}
		m.Pads = append(m.Pads, Pad{ID: len(m.Pads), X: chosen.X, Y: chosen.Y, Terrain: terrain, Sector: r.Sector, Route: r.ID})
	}
}
