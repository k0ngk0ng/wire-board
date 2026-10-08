package game

import (
	"errors"
	"slices"
)

// T&B allows the scenarios retaining Largest Army, but publishes no neutral
// sea-board recipe. Keep this supplement separate from multiplayer maps.
const CatanTwoSeafarersRules = "wire-board-two-seafarers-v1"
const catanTwoSeafarersNotice = "本站双人航海家规则：使用四人地图，两家中立势力各从一处相隔较远的合法海岸村庄开始；真人每回合两次生产。造船后补造中立船，无合法船位才改造中立道路；中立船不移动。中立探索翻开迷雾但不领奖，中立不领取部落奖励、不与布匹村落贸易；不占部落奖励边。无沙漠时贸易筹码可将强盗退至场外，不影响海盗"

func CatanTwoSeafarersScenario(scenario string) bool {
	return slices.Contains([]string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"}, scenario)
}
func NormalizeCatanTwoSeafarersSetup(setup CatanSeafarersSetup) (CatanSeafarersSetup, error) {
	if !CatanTwoSeafarersScenario(setup.Scenario) {
		return setup, errors.New("双人航海家只适用于保留最大骑士军队的八种剧本")
	}
	return NormalizeCatanSeafarersSetup(4, setup)
}
func (g *Catan) twoSeaRecipe() bool {
	return g.Two != nil && g.Two.Seafarers == CatanTwoSeafarersRules && len(g.Players) == 2
}
func (g *Catan) twoSeafarers() bool {
	return g.twoSeaRecipe() && g.Seafarers != nil && CatanTwoSeafarersScenario(g.Seafarers.Scenario)
}

func NewCatanTwoSeafarers(n int, options CatanOptions, setup CatanSeafarersSetup, world *CatanNewWorldMap) (*State, error) {
	setup, err := NormalizeCatanTwoSeafarersSetup(setup)
	if err != nil {
		return nil, err
	}
	o, err := NormalizeCatanOptions(options)
	if err != nil || n != 2 || !CatanTwoHelpersOptions(setup.Scenario, o) {
		return nil, errors.New("双人航海家需要两位真人，不使用五六人扩充")
	}
	if setup.Scenario != "new_world" && world != nil {
		return nil, errors.New("只有新世界可以指定地形")
	}
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	g := s.Catan
	g.Two = &CatanTwo{Rules: CatanTwoRules, Seafarers: CatanTwoSeafarersRules, Rolls: []int{}, Tokens: []int{5, 5}, Bank: 10}
	switch setup.Scenario {
	case "shores":
		err = g.makeSeafarersShoresFour()
	case "islands":
		err = g.makeSeafarersIslandsFour()
	case "fog":
		err = g.makeSeafarersFogFour()
	case "desert":
		err = g.makeSeafarersDesertFour()
	case "tribe":
		err = g.makeSeafarersTribeFour()
	case "cloth":
		err = g.makeSeafarersClothFour()
	case "wonders":
		err = g.makeSeafarersWondersFour()
		s.Phase = "catan_wonders_start"
	case "new_world":
		var geometry *Catan
		geometry, err = catanNewWorldMapGeometry(4, world)
		if err == nil {
			err = g.makeNewWorldMap()
		}
		if err == nil {
			g.Tiles, g.Vertices, g.Edges, g.HexSize = geometry.Tiles, geometry.Vertices, geometry.Edges, geometry.HexSize
			g.Ports, g.Robber = []CatanPort{}, -1
			g.Seafarers.Islands = g.findIslands()
			s.Phase = "catan_world_ports"
		}
	}
	if err != nil {
		return nil, err
	}
	if setup.Layout == "variable" {
		if err = s.randomizeCatanSeafarersMap(); err != nil {
			return nil, err
		}
	}
	g = s.Catan // Variable setup atomically replaces the copied state.
	g.Seafarers.Rules, g.Seafarers.Layout = setup.Rules, setup.Layout
	if err = g.prepareTwoSeaNeutrals(); err != nil {
		return nil, err
	}
	g.StartPlayer = catanRandom(2)
	s.Turn = g.StartPlayer
	s.Log = []string{catanTwoSeafarersNotice}
	if g.cloth() != nil {
		s.Log = append(s.Log, "布匹沿用三轮真人起始建设，第三座领取资源；不使用最长路线奖励", catanClothSupplyRule)
	}
	s.catanScores()
	return s, s.enableTwoHelpers(o)
}

func (g *Catan) prepareTwoSeaNeutrals() error {
	// Select by public geometry only, before any real settlement. Coastal sites
	// permit both neutral route types; maximally separated sites avoid one choke.
	candidates := []int{}
	for _, v := range g.Vertices {
		if !g.canSettlement(-2, v.ID, true) {
			continue
		}
		coast := false
		for _, edge := range g.touching(v.ID) {
			coast = coast || g.edgeTerrain(edge, true) && g.edgeTerrain(edge, false)
		}
		if coast {
			candidates = append(candidates, v.ID)
		}
	}
	best := -1.0
	first, second := -1, -1
	for _, a := range candidates {
		for _, b := range candidates {
			if b <= a {
				continue
			}
			adjacent := false
			for _, edge := range g.touching(a) {
				e := g.Edges[edge]
				adjacent = adjacent || e.A == b || e.B == b
			}
			if adjacent {
				continue
			}
			dx, dy := g.Vertices[a].X-g.Vertices[b].X, g.Vertices[a].Y-g.Vertices[b].Y
			d := dx*dx + dy*dy
			if d > best {
				best, first, second = d, a, b
			}
		}
	}
	if first < 0 {
		return errors.New("地图缺少两处中立海岸开局位置")
	}
	g.Two.SeaStarts = []int{first, second}
	for i, id := range g.Two.SeaStarts {
		g.Vertices[id].Owner, g.Vertices[id].Level = catanTwoNeutralOwners[i], 1
	}
	return nil
}

func (s *State) validateTwoSeafarers() error {
	g, q := s.Catan, s.Catan.Two
	if q.Seafarers == "" && g.Seafarers == nil {
		if len(q.SeaStarts) != 0 || q.AfterRoute != "" {
			return errors.New("基础双人局不能包含航海家记录")
		}
		return nil
	}
	if !g.twoSeafarers() || g.Rivers != nil || g.Caravans != nil || g.Attack != nil || g.Transport != nil || g.Explorer != nil || g.Fishing != nil && !g.twoFishingSeafarers() || g.CitiesKnights != nil {
		return errors.New("双人航海家版本或尚未接通的组合无效")
	}
	sea := g.Seafarers
	setup := CatanSeafarersSetup{Scenario: sea.Scenario, Layout: sea.Layout, Rules: sea.Rules}
	normal, err := NormalizeCatanTwoSeafarersSetup(setup)
	if err != nil || normal != setup || len(sea.Seats) != 2 || len(sea.Islands) != len(g.Tiles) || len(q.SeaStarts) != 2 || q.SeaStarts[0] == q.SeaStarts[1] {
		return errors.New("双人海图配置或开局记录无效")
	}
	counts := map[string]int{"shores": 42, "islands": 35, "fog": 42, "desert": 42, "tribe": 49, "cloth": 42, "wonders": 49, "new_world": 42}
	// Map dimensions are bound to the four-player recipe, not the base island.
	if count := counts[sea.Scenario]; count > 0 && len(g.Tiles) != count {
		return errors.New("双人海图尺寸不匹配")
	}
	targets := map[string]int{"shores": 14, "islands": 13, "fog": 12, "desert": 14, "tribe": 13, "cloth": 14, "wonders": 10, "new_world": 12}
	bonuses := map[string]int{"shores": 2, "islands": 2, "desert": 2, "wonders": 1, "new_world": 1}
	if sea.VictoryPoints != targets[sea.Scenario] || sea.IslandBonus != bonuses[sea.Scenario] || sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || g.Robber < -1 || g.Robber >= len(g.Tiles) {
		return errors.New("双人海图胜利条件或强盗海盗位置无效")
	}
	for i, v := range g.Vertices {
		if v.ID != i {
			return errors.New("双人海图交点编号无效")
		}
	}
	for i, tile := range g.Tiles {
		if tile.ID != i || len(tile.Vertices) != 6 {
			return errors.New("双人海图地块编号无效")
		}
		for _, v := range tile.Vertices {
			if v < 0 || v >= len(g.Vertices) {
				return errors.New("双人海图地块交点无效")
			}
		}
	}
	for i, e := range g.Edges {
		if e.ID != i || e.A < 0 || e.B < 0 || e.A == e.B || e.A >= len(g.Vertices) || e.B >= len(g.Vertices) || e.Ship && e.Owner == -1 {
			return errors.New("双人海图航路无效")
		}
		for _, id := range e.Tiles {
			if id < 0 || id >= len(g.Tiles) || !slices.Contains(g.Tiles[id].Vertices, e.A) || !slices.Contains(g.Tiles[id].Vertices, e.B) {
				return errors.New("双人海图航路地块无效")
			}
		}
	}
	for i, v := range q.SeaStarts {
		if v < 0 || v >= len(g.Vertices) || g.Vertices[v].Owner != catanTwoNeutralOwners[i] || g.Vertices[v].Level != 1 {
			return errors.New("双人海图中立起点被移除")
		}
	}
	if (sea.Scenario == "fog") != (sea.Fog != nil) || (sea.Scenario == "tribe") != (sea.Tribe != nil) || (sea.Scenario == "cloth") != (sea.Cloth != nil) || (sea.Scenario == "wonders") != (sea.Wonders != nil) || (sea.Scenario == "new_world") != (sea.NewWorld != nil) || sea.PirateIslands != nil {
		return errors.New("双人海图组件不匹配")
	}
	if q.AfterRoute != "" {
		if !slices.Contains([]string{"road", "ship"}, q.AfterRoute) || q.AfterHelper != "" || q.Pending != nil || q.Trade != nil || g.setup() || s.Finished || !g.twoSeaRouteWaiting() {
			return errors.New("双人航路后续建设记录无效")
		}
	}
	if g.GoldPending != nil {
		gold := g.GoldPending
		if s.Phase != "catan_gold" || len(gold.Claims) == 0 || q.Pending != nil || q.Trade != nil || g.HelperPending != nil {
			return errors.New("双人金矿回应冲突")
		}
		seen := map[int]bool{}
		for _, claim := range gold.Claims {
			if claim.Player < 0 || claim.Player >= 2 || claim.Count < 1 || seen[claim.Player] {
				return errors.New("双人金矿领取人无效")
			}
			seen[claim.Player] = true
		}
	}
	return nil
}
func (g *Catan) twoSeaRouteWaiting() bool {
	return g.GoldPending != nil || g.HelperPending != nil || g.tribe() != nil && g.tribe().Pending != nil
}
func (g *Catan) twoNeutralShipChoices() []catanTwoNeutralChoice {
	choices := []catanTwoNeutralChoice{}
	for _, owner := range catanTwoNeutralOwners {
		if g.shipCount(owner) >= 15 {
			continue
		}
		for _, e := range g.Edges {
			if !g.canShip(owner, e.ID) {
				continue
			}
			// Neutral colors have no mission inventories; preserve reward edges for
			// humans instead of stranding a reward underneath a permanent neutral ship.
			if t := g.tribe(); t != nil {
				if slices.Contains(t.Tokens, e.ID) || slices.ContainsFunc(t.Development, func(c CatanTribeDevelopment) bool { return c.Edge == e.ID }) || slices.ContainsFunc(t.Ports, func(p CatanPort) bool { return p.Edge == e.ID }) {
					continue
				}
			}
			choices = append(choices, catanTwoNeutralChoice{owner, -1, e.ID})
		}
	}
	return choices
}

// Avoid a permanently landlocked autoplay opening on crowded two-player maps.
func (g *Catan) twoSeaCoast(vertex int) bool {
	for _, id := range g.touching(vertex) {
		e := g.Edges[id]
		if (e.Owner == -1 || e.Owner == g.Vertices[vertex].Owner) && g.edgeTerrain(id, true) && !g.pirateBlocks(id) {
			return true
		}
	}
	return false
}
func (g *Catan) twoSeaNeedsCoast(player int) bool {
	for _, v := range g.Vertices {
		if v.Owner == player && v.Level > 0 && g.twoSeaCoast(v.ID) {
			return false
		}
	}
	return true
}
