package starfort

import "math"

// Fixed lanes require route coverage, not the old board's hard-coded pad IDs.
// All scoring below uses public geometry, tower statistics and team defenses.
func (s *State) centralInvestment(player int) *Action {
	routes := s.World.Map.Routes
	coverage := func(pad Pad, radius float64, route Route) float64 {
		n := 0.
		for d := 35.; d < pathDistance(route.Points); d += 45 {
			p := pathPosition(route.Points, d)
			if math.Hypot(p.X-pad.X, p.Y-pad.Y) <= radius {
				n++
			}
		}
		return n
	}
	strength := make([]float64, len(routes))
	counts := make([]map[string]int, len(routes))
	occupied := map[int]bool{}
	owned := []Tower{}
	for i := range counts {
		counts[i] = map[string]int{}
	}
	for _, tower := range s.Towers {
		occupied[tower.Pad] = true
		if tower.Owner == player {
			owned = append(owned, tower)
		}
		damage, radius, interval := s.towerStats(tower)
		dps := damage * 1000 / float64(interval)
		if tower.Kind == "frost" {
			dps += 25
		}
		dominant, best := 0, -1.
		for j, r := range routes {
			c := coverage(s.World.Map.Pads[tower.Pad], radius, r)
			strength[j] += c * dps
			if c > best {
				dominant, best = j, c
			}
		}
		counts[dominant][tower.Kind]++
	}
	weak := 0
	for i := 1; i < len(routes); i++ {
		if strength[i] < strength[weak] {
			weak = i
		}
	}
	count := 0
	for _, n := range counts[weak] {
		count += n
	}
	desired := 4
	if s.Wave >= 12 {
		desired = 5
	}
	if count < desired {
		kind := "arrow"
		if counts[weak]["arrow"] == 0 {
			kind = "arrow"
		} else if counts[weak]["arcane"] == 0 {
			kind = "arcane"
		} else if counts[weak]["frost"] == 0 {
			kind = "frost"
		} else if counts[weak]["cannon"] == 0 {
			kind = "cannon"
		} else {
			kind = "arrow"
		}
		spec, _ := spec(kind)
		if s.Players[player].Gold >= spec.Cost {
			bestPad := -1
			best := 0.
			for _, pad := range s.World.Map.Pads {
				if occupied[pad.ID] {
					continue
				}
				score := coverage(pad, spec.Range, routes[weak])
				for j, r := range routes {
					if j != weak {
						score += coverage(pad, spec.Range, r) * .12
					}
				}
				if score > best {
					best, bestPad = score, pad.ID
				}
			}
			if bestPad >= 0 {
				return &Action{Type: "build", Tower: kind, Pad: bestPad}
			}
		}
	}
	best := -1
	value := 0.
	for i, tower := range owned {
		cost := upgradeCost(tower)
		if cost > s.Players[player].Gold {
			continue
		}
		damage, radius, interval := s.towerStats(tower)
		impact := 0.
		for j, r := range routes {
			impact += coverage(s.World.Map.Pads[tower.Pad], radius, r) / (1 + strength[j]/500)
		}
		score := impact * damage / float64(interval) / float64(cost) / math.Sqrt(float64(tower.Level))
		if tower.Kind == "frost" {
			score *= .4
		}
		if score > value {
			best, value = i, score
		}
	}
	if best >= 0 {
		t := owned[best]
		a := &Action{Type: "upgrade", TowerID: t.ID}
		if t.Level == 3 {
			a.Branch = "power"
			if t.Kind == "cannon" || t.Kind == "frost" {
				a.Branch = "speed"
			}
		}
		return a
	}
	return nil
}
