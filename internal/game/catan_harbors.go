package game

// Harbors of Catan, 2025 Traders & Barbarians p.4 (formerly Harbormaster).
// Keep ownership in saves: a tie retains the existing holder.
const CatanHarborsRules = "catan-harbors-2025"

type CatanHarbors struct {
	Rules string `json:"rules"`
	Owner int    `json:"owner"`
}

// Internal variant constructor. Waiting-room/public configuration is separate.
func NewCatanHarbors(n int, options CatanOptions, setup CatanBaseConfiguration) (*State, error) {
	s, err := NewCatanConfigured(n, options, setup)
	if err != nil {
		return nil, err
	}
	s.enableCatanHarbors()
	return s, nil
}

func (g *Catan) harborVertex(id int) bool {
	for _, port := range g.Ports {
		if port.Edge >= 0 && port.Edge < len(g.Edges) {
			e := g.Edges[port.Edge]
			if e.A == id || e.B == id {
				return true
			}
		}
	}
	return false
}

// Public-board heuristic shared by construction and relocated port choices.
// Rewards building toward the tile without looking at hidden hands or decks.
func (g *Catan) harborGainValue(player, gain int) int {
	if g.Harbors == nil || gain <= 0 || player < 0 || player >= len(g.Players) {
		return 0
	}
	points := g.harborPoints()
	points[player] += gain
	value := gain * 35
	if g.Harbors.Owner != player && g.awardHolder(g.Harbors.Owner, 3, points) == player {
		value += 160
	}
	return value
}

func (g *Catan) harborVertexValue(player, id int) int {
	if g.Harbors == nil || !g.harborVertex(id) {
		return 0
	}
	v := g.Vertices[id]
	if v.Level == 0 || v.Owner == player && v.Level == 1 {
		return g.harborGainValue(player, 1)
	}
	return 0
}

func (g *Catan) harborPortValue(player, edge int) int {
	if g.Harbors == nil {
		return 0
	}
	gain := 0
	e := g.Edges[edge]
	for _, id := range []int{e.A, e.B} {
		v := g.Vertices[id]
		if v.Owner == player && !g.harborVertex(id) {
			gain += v.Level
		}
	}
	return g.harborGainValue(player, gain)
}

func (s *State) enableCatanHarbors() {
	if s.Catan.Harbors != nil {
		return
	}
	s.Catan.Harbors = &CatanHarbors{Rules: CatanHarborsRules, Owner: -1}
	s.Log = append(s.Log, "加入港口霸主：港口村庄计1点、城市计2点；率先达到3点获得2分奖励，超过持有者可夺取；获胜门槛增加1分")
	if s.Catan.tradersVariants() && s.Catan.Attack != nil {
		s.Log = append(s.Log, "本站蛮族港口组合：被征服建筑暂停港口计点，解放后恢复；原剧本特殊胜利条件保留")
	}
	s.catanScores()
}

func (g *Catan) harborPoints() []int {
	points := make([]int, len(g.Players))
	seen := make([]bool, len(g.Vertices))
	// Only installed ports count, never a Forgotten Tribe reward still on the
	// map or in a player's hand. A vertex beside multiple ports counts once.
	for _, port := range g.Ports {
		if port.Edge < 0 || port.Edge >= len(g.Edges) {
			continue
		}
		e := g.Edges[port.Edge]
		for _, id := range []int{e.A, e.B} {
			if id < 0 || id >= len(g.Vertices) || seen[id] {
				continue
			}
			seen[id] = true
			v := g.Vertices[id]
			if v.Owner >= 0 && v.Owner < len(points) && !g.Players[v.Owner].Eliminated && !(g.tradersVariants() && g.Attack != nil && g.Attack.conqueredBuilding(g, id)) {
				// Official T&B FAQ #37: a metropolis yields two harbor points.
				points[v.Owner] += v.Level
			}
		}
	}
	return points
}

func (s *State) catanHarborsScore() {
	g := s.Catan
	if h := g.Harbors; h != nil {
		points := g.harborPoints()
		previous := h.Owner
		h.Owner = g.awardHolder(previous, 3, points)
		if h.Owner != previous {
			if h.Owner < 0 {
				s.Log = append(s.Log, "港口霸主奖励暂时无人持有")
			} else {
				s.catanLog(h.Owner, "获得港口霸主（港口建筑%d点），奖励2分", points[h.Owner])
			}
		}
	}
}

// Scenario targets stay authored in Seafarers state. Variant adjustments are
// derived once, preserving special endings such as wonders and cloth depletion.
func (g *Catan) victoryTarget() int {
	if g.Explorer != nil {
		return g.Explorer.Board.Target
	}
	if g.attackTransport() {
		if g.Harbors != nil {
			return 15
		}
		return 14
	}
	goal := 10
	if g.Transport != nil {
		goal = 13
		if g.fishingTransport() {
			goal = 12
		}
	}
	if g.Caravans != nil || g.Attack != nil {
		goal = 12
	}
	if g.CitiesKnights != nil && g.wonders() == nil {
		goal = 13
	}
	if g.caravansTransport() || g.caravanKnights() || g.transportKnights() {
		goal = 15
	}
	if g.Seafarers != nil && g.Seafarers.VictoryPoints > 0 {
		goal = g.Seafarers.VictoryPoints
	}
	if g.Harbors != nil {
		goal++
	}
	return goal
}
