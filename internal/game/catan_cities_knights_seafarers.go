package game

import "fmt"

// Public three-to-six-player combinations share the accepted sea maps.
func NewCatanCitiesKnightsSeafarers(n int, options CatanOptions, setup CatanSeafarersSetup, world *CatanNewWorldMap) (*State, error) {
	if options.Helpers || options.AllHelpers {
		return nil, fmt.Errorf("Helpers与城市骑士组合尚未接入")
	}
	if !CatanCitiesKnightsSeafarersSupported(setup.Scenario) {
		return nil, fmt.Errorf("该航海家剧本的城市骑士组合规则尚未接入")
	}
	s, err := NewCatanSeafarers(n, options, setup, world)
	if err != nil {
		return nil, err
	}
	if err := s.enableCitiesKnightsSeafarers(); err != nil {
		return nil, err
	}
	return s, nil
}

// Apply the shared sea setup after the map (including optional fishing lakes)
// is prepared, before any building or production has taken place.
func (s *State) enableCitiesKnightsSeafarers() error {
	n := len(s.Catan.Players)
	logs := append([]string{}, s.Log...)
	phase := s.Phase
	s.enableCitiesKnights()
	g := s.Catan
	g.initTribeProgress()
	if p := g.pirateIslands(); p != nil {
		p.KnightsRules = CatanPirateKnightsRules
	}
	k := g.CitiesKnights
	k.PirateStart = g.Seafarers.Pirate
	g.Seafarers.Pirate = -1
	g.Seafarers.VictoryPoints += 2
	s.Phase = phase
	// Keep each map's setup prelude, but replace the two-settlement wording.
	win := fmt.Sprintf("%d分获胜", g.Seafarers.VictoryPoints)
	opening := "顺序放村庄，逆序放城市；普通起始资源"
	bandits := "首次蛮族进攻前，强盗与海盗都休眠；金矿只产普通资源"
	if g.wonders() != nil {
		win = fmt.Sprintf("建成4级奇迹，或%d分且奇迹等级独自领先", g.Seafarers.VictoryPoints)
		bandits = "本剧本不使用海盗；先手选择强盗起点，首次蛮族进攻后强盗入场；金矿只产普通资源"
	}
	if g.cloth() != nil {
		opening = "顺序放村庄，逆序放城市，再顺序放村庄；仅第三座建筑领取普通起始资源"
		win += "；回合结束时5座村落耗尽也会结算；不使用最长路线或最大骑士军队"
	}
	s.Log = []string{logs[0], "加入城市与骑士：" + opening + "；" + win, bandits}
	if g.Seafarers.NumberRecipe != "" {
		s.Log = append(s.Log, catanSeaNumberNotice)
	}
	if g.cloth() != nil {
		s.Log = append(s.Log, catanClothSupplyRule)
	}
	if phase == "catan_world_ports" {
		s.Log = append(s.Log, "先按确认地图轮流放置随机港口，再开始起始建设")
	}
	if n > 4 {
		s.Log = append(s.Log, "采用配对回合，第二位玩家不掷骰、不能自由交易")
	}
	if g.tribe() != nil {
		s.Log = append(s.Log, catanTribeProgressNotice)
	}
	if g.pirateIslands() != nil {
		s.Log = append(s.Log, catanPirateKnightsNotice)
		if err := s.validatePirateKnights(); err != nil {
			return err
		}
	}
	return g.validateTribeProgress()
}

// Recipe IDs only; persisted five/six-player islands use a separate alias.
func CatanCitiesKnightsSeafarersSupported(scenario string) bool {
	switch scenario {
	case "shores", "islands", "fog", "desert", "new_world", "wonders", "cloth", "tribe", "pirate_islands":
		return true
	}
	return false
}

func (g *Catan) citySeaSupported() bool {
	if g.Seafarers == nil {
		return true
	}
	switch g.Seafarers.Scenario {
	case "shores", "islands", "six_islands", "fog", "desert", "new_world", "wonders", "cloth", "tribe", "pirate_islands":
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
