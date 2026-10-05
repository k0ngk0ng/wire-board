package game

import "fmt"

// Internal combination constructor. Scenario-specific variants remain gated
// until their special components and end conditions have been verified.
func NewCatanCitiesKnightsSeafarers(n int, options CatanOptions, setup CatanSeafarersSetup, world *CatanNewWorldMap) (*State, error) {
	if options.Helpers || options.AllHelpers {
		return nil, fmt.Errorf("Helpers与城市骑士组合尚未接入")
	}
	switch setup.Scenario {
	case "shores", "islands", "fog", "desert", "new_world":
	default:
		return nil, fmt.Errorf("该航海家剧本的城市骑士组合规则尚未接入")
	}
	s, err := NewCatanSeafarers(n, options, setup, world)
	if err != nil {
		return nil, err
	}
	logs := append([]string{}, s.Log...)
	phase := s.Phase
	s.enableCitiesKnights()
	g := s.Catan
	k := g.CitiesKnights
	k.PirateStart = g.Seafarers.Pirate
	g.Seafarers.Pirate = -1
	g.Seafarers.VictoryPoints += 2
	s.Phase = phase
	// Keep each map's setup prelude, but replace the two-settlement wording.
	s.Log = []string{logs[0], fmt.Sprintf("加入城市与骑士：顺序放村庄，逆序放城市；普通起始资源；%d分获胜", g.Seafarers.VictoryPoints), "首次蛮族进攻前，强盗与海盗都休眠；金矿只产普通资源"}
	if phase == "catan_world_ports" {
		s.Log = append(s.Log, "先按确认地图轮流放置随机港口，再开始起始建设")
	}
	if n > 4 {
		s.Log = append(s.Log, "采用配对回合，第二位玩家不掷骰、不能自由交易")
	}
	return s, nil
}
func (g *Catan) citySeaSupported() bool {
	if g.Seafarers == nil {
		return true
	}
	switch g.Seafarers.Scenario {
	case "shores", "islands", "six_islands", "fog", "desert", "new_world":
		return true
	}
	return false
}

// Removing a mixed road/ship bridge must not detach a knight from every own
// building. Opponent pieces interrupt movement/scoring, not physical anchoring.
// Existing orphaned pieces in old saves still cannot lose their last connection.
func (g *Catan) preservesKnightConnections(player, removed int) bool {
	if g.CitiesKnights == nil {
		return true
	}
	reachable := func(skip int) []bool {
		seen := make([]bool, len(g.Vertices))
		queue := []int{}
		for _, v := range g.Vertices {
			if v.Owner == player && v.Level > 0 {
				seen[v.ID] = true
				queue = append(queue, v.ID)
			}
		}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			for _, id := range g.touching(v) {
				e := g.Edges[id]
				if id == skip || e.Owner != player {
					continue
				}
				to := e.A
				if to == v {
					to = e.B
				}
				if !seen[to] {
					seen[to] = true
					queue = append(queue, to)
				}
			}
		}
		return seen
	}
	before, after := reachable(-1), reachable(removed)
	for _, n := range g.CitiesKnights.Knights {
		if n.Owner != player {
			continue
		}
		if before[n.Vertex] && !after[n.Vertex] {
			return false
		}
		connected := false
		for _, id := range g.touching(n.Vertex) {
			if id != removed && g.Edges[id].Owner == player {
				connected = true
			}
		}
		if !connected {
			return false
		}
	}
	return true
}
func (g *Catan) canDiplomacyShip(player, edge int) bool {
	if g.Seafarers == nil {
		return false
	}
	copy := *g
	sea := *g.Seafarers
	copy.Seafarers = &sea
	sea.Pirate = -1
	return copy.canShip(player, edge)
}
