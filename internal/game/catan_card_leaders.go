package game

// Event rankings are public information, computed before C&K event dice
// activate/deactivate knights or pillage cities. Persist only the winning
// response queue; later responses must not re-evaluate eligibility.
func (g *Catan) cardEventLeaders(kind string, start int) []int {
	counts := make([]int, len(g.Players))
	switch kind {
	case "calm_seas":
		for _, v := range g.Vertices {
			if v.Owner >= 0 && v.Owner < len(counts) && v.Level > 0 && g.harborVertex(v.ID) {
				// Settlement, city and metropolis each count as one building,
				// even if the intersection serves more than one installed port.
				counts[v.Owner]++
			}
		}
	case "tournament":
		if g.attackKnights() {
			for _, knight := range g.Attack.City.Knights {
				if knight.Active && !g.Players[knight.Owner].Eliminated {
					counts[knight.Owner] += knight.Strength
				}
			}
		} else if k := g.CitiesKnights; k != nil {
			for _, knight := range k.Knights {
				if knight.Active && knight.Owner >= 0 && knight.Owner < len(counts) {
					counts[knight.Owner] += knight.Strength
				}
			}
		} else {
			for p := range g.Players {
				counts[p] = g.Players[p].Knights // Played, face-up knights only.
			}
		}
	default:
		return []int{}
	}
	best := -1
	for p, count := range counts {
		if !g.Players[p].Eliminated && count > best {
			best = count
		}
	}
	winners := []int{}
	for offset := range len(counts) {
		p := (start + offset) % len(counts)
		// Printed 2025 rules impose no minimum or unique-leader condition for
		// these two rewards: tied zero counts qualify, just like other ties.
		if !g.Players[p].Eliminated && counts[p] == best {
			winners = append(winners, p)
		}
	}
	return winners
}
