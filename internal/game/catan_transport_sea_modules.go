package game

import (
	"errors"
	"slices"
)

const CatanTransportSeaKnightsRules = "wire-board-transport-sea-knights-v1"
const CatanTransportSeaFishingRules = "wire-board-transport-sea-fishing-v1"
const CatanTransportSeaVariableLayout = "wire-board-transport-sea-variable-v1"

type CatanTransportSeaSetup struct {
	Scenario string `json:"scenario"`
	Layout   string `json:"layout"`
	Rules    string `json:"rules"`
}

func NormalizeCatanTransportSeaSetup(setup CatanTransportSeaSetup) (CatanTransportSeaSetup, error) {
	if setup.Scenario != "shores" && setup.Scenario != "desert" {
		return setup, errors.New("运输海图仅支持新海岸和穿越沙漠")
	}
	if setup.Layout == "" {
		setup.Layout = "fixed"
	}
	if setup.Layout != "fixed" && setup.Layout != "variable" || setup.Rules != "" && setup.Rules != CatanTransportSeafarersRules {
		return setup, errors.New("运输海图布局或版本无效")
	}
	setup.Rules = CatanTransportSeafarersRules
	return setup, nil
}

func (g *Catan) transportSeaKnights() bool {
	return g.transportSea() && g.transportKnights() && g.Transport.SeaKnights == CatanTransportSeaKnightsRules
}

func NewCatanTransportSea(n int, setup CatanTransportSeaSetup, knights, fishing bool) (*State, error) {
	setup, err := NormalizeCatanTransportSeaSetup(setup)
	if err != nil {
		return nil, err
	}
	s, err := NewCatanTransportSeafarers(n, setup.Scenario)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	if setup.Layout == "variable" {
		g.Transport.Map.SeaLayout = "variable"
		g.Seafarers.Layout, g.Seafarers.Variable = CatanTransportSeaVariableLayout, true
		groups := transportSeaVariableGroups(g)
		for _, ids := range groups {
			resources, numbers := []int{}, []int{}
			for _, id := range ids {
				resources = append(resources, g.Tiles[id].Resource)
				numbers = append(numbers, g.Tiles[id].Number)
			}
			shuffle(resources)
			shuffle(ids)
			for i, id := range ids {
				g.Tiles[id].Resource, g.Tiles[id].Number = resources[i], 0
			}
			reds, others := []int{}, []int{}
			for _, number := range numbers {
				if number == 6 || number == 8 {
					reds = append(reds, number)
				} else {
					others = append(others, number)
				}
			}
			shuffle(reds)
			shuffle(others)
			var place func(int, int) bool
			place = func(start, at int) bool {
				if at == len(reds) {
					return true
				}
				for i := start; i <= len(ids)-(len(reds)-at); i++ {
					id := ids[i]
					valid := true
					for _, edge := range g.Edges {
						if len(edge.Tiles) != 2 || !slices.Contains(edge.Tiles, id) {
							continue
						}
						other := edge.Tiles[0]
						if other == id {
							other = edge.Tiles[1]
						}
						if g.Tiles[other].Number == 6 || g.Tiles[other].Number == 8 {
							valid = false
							break
						}
					}
					if !valid {
						continue
					}
					g.Tiles[id].Number = reds[at]
					if place(i+1, at+1) {
						return true
					}
					g.Tiles[id].Number = 0
				}
				return false
			}
			if !place(0, 0) {
				return nil, errors.New("无法按运输可变布局分开红色数字")
			}
			at := 0
			for _, id := range ids {
				if g.Tiles[id].Number == 0 {
					g.Tiles[id].Number = others[at]
					at++
				}
			}
		}
		s.Log = append(s.Log, "本站运输可变布局：保留海陆、沙漠带、货物地点、港口和双数字格，在各区域内重新分配普通资源与数字")
	}
	if knights {
		logs := slices.Clone(s.Log)
		s.enableCitiesKnights()
		g = s.Catan
		g.Transport.Knights, g.Transport.SeaKnights = CatanTransportKnightsRules, CatanTransportSeaKnightsRules
		g.Transport.DeckRecipe = ""
		// T&B+Knights replaces a forest instead of a field; pin the sea
		// adaptation to a producing mainland forest without moving a site.
		forest := -1
		for _, t := range g.Tiles {
			if t.Resource == 0 && g.Seafarers.Islands[t.ID] == g.Seafarers.StartIslands[0] {
				forest = t.ID
				break
			}
		}
		if forest < 0 {
			return nil, errors.New("运输骑士海图缺少可替换森林")
		}
		g.Tiles[forest].Resource = 3
		g.Transport.Map.SeaForest = &forest
		g.Seafarers.VictoryPoints += 2
		g.CitiesKnights.PirateStart = -1
		if g.Two != nil {
			g.Two.Knights, g.Two.SeaKnights = CatanTwoKnightsRules, CatanTwoSeafarersKnightsRules
		}
		s.Log = append(logs, "本站运输＋航海家＋城市骑士：使用进步牌、商品与城市改良，主岛一森林改粮田；19分获胜。蛮族船和三名道路蛮族分别运作，无强盗海盗，2／12正常生产，骑士可沿己方船路连接")
	}
	if fishing {
		tokens, e := newCatanFishingTokens(n)
		if n == 2 {
			tokens, e = newTwoCatanFishTokens()
		}
		if e != nil {
			return nil, e
		}
		grounds, e := transportSeaFishingGrounds(g)
		if e != nil {
			return nil, e
		}
		f := &CatanFishing{Transport: CatanFishingTransportRules, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
		f.Map.Lakes = []catanFishingLake{}
		f.Map.SeaRecipe = CatanTransportSeaFishingRules
		f.Map.Grounds = grounds
		numbers := []int{4, 5, 6, 8, 9, 10}
		if n > 4 {
			numbers = append(numbers, 5, 9)
		}
		shuffle(numbers)
		for i := range f.Map.Grounds {
			f.Map.Grounds[i].Number = numbers[i]
		}
		if knights {
			f.SeaKnights = CatanFishingSeaKnightsRules
		} else {
			g.Seafarers.VictoryPoints--
		}
		if n == 2 {
			f.Two, f.TwoSea = CatanTwoFishingRules, CatanTwoFishingSeafarersRules
			g.Two.Tokens, g.Two.Bank = []int{0, 0}, 0
			if knights {
				f.TwoKnights = CatanTwoFishingKnightsRules
			}
		}
		g.Fishing = f
		s.Log = append(s.Log, "本站运输捕鱼海图：无湖泊，渔场沿真实海岸布置，避开港口和货物禁建边；2鱼或1粮共用每回合一次马车加2步，旧靴额外加1分。普通组合16分，骑士组合19分；双人使用起始五枚鱼并停用贸易筹码")
	}
	s.catanScores()
	return s, s.validateCatanTransport()
}

func transportSeaVariableGroups(g *Catan) map[int][]int {
	out := map[int][]int{}
	for _, t := range g.Tiles {
		if t.Resource < 0 || t.Resource >= 5 {
			continue
		}
		extra := false
		for _, n := range g.Transport.Map.ExtraNumbers {
			extra = extra || n.Tile == t.ID
		}
		if !extra {
			out[g.Seafarers.Islands[t.ID]] = append(out[g.Seafarers.Islands[t.ID]], t.ID)
		}
	}
	return out
}

// The sea frame's outside edges are water, so find actual land/sea
// boundaries and pair adjacent coastal edges on any island.
func transportSeaFishingGrounds(g *Catan) ([]catanFishingGround, error) {
	blocked := map[int]bool{}
	for _, p := range g.Ports {
		blocked[p.Edge] = true
	}
	for _, site := range g.Transport.Map.Sites {
		for _, id := range site.Blocked {
			blocked[id] = true
		}
	}
	return spreadCoastalFishingGrounds(g, blocked, func(id int) bool {
		return !g.transportInterior(id) && g.edgeTerrain(id, true) && g.edgeTerrain(id, false)
	})
}
