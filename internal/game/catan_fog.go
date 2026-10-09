package game

import (
	"errors"
	"slices"
)

// Terrain and number stacks are shuffled at setup, persisted, and never sent
// to clients. An empty map position stays CatanFog until a route reaches it.
type CatanFogState struct {
	Terrain    []int `json:"terrain"`
	Numbers    []int `json:"numbers"`
	StartTiles []int `json:"startTiles"`
}

// A discovered gold field can interrupt a route action. Keep its remaining
// work separate from the gold's response phase, especially Road Building's
// second route, Helpers' exchange, and the next setup seat.
type CatanRouteCompletion struct {
	Player int  `json:"player"`
	Edge   int  `json:"edge"`
	Free   bool `json:"free,omitempty"`
	Helper bool `json:"helper,omitempty"`
	Setup  bool `json:"setup,omitempty"`
}

func (g *Catan) fogAtRoute(edge int) []int {
	result := []int{}
	if g.Seafarers == nil || g.Seafarers.Fog == nil || edge < 0 || edge >= len(g.Edges) {
		return result
	}
	e := g.Edges[edge]
	for _, t := range g.Tiles {
		if t.Resource == CatanFog && (slices.Contains(t.Vertices, e.A) || slices.Contains(t.Vertices, e.B)) {
			result = append(result, t.ID)
		}
	}
	return result
}

func (s *State) catanDiscover(player, edge int) (int, error) {
	g := s.Catan
	pending := g.fogAtRoute(edge)
	if len(pending) == 0 {
		return 0, nil
	}
	fog := g.Seafarers.Fog
	gold := 0
	for _, id := range pending {
		if len(fog.Terrain) == 0 {
			return 0, errors.New("探索地形堆已空")
		}
		resource := fog.Terrain[len(fog.Terrain)-1]
		fog.Terrain = fog.Terrain[:len(fog.Terrain)-1]
		if resource < 0 || resource > CatanGold {
			return 0, errors.New("探索地形不合法")
		}
		number := 0
		if resource != CatanSea && resource != CatanDesert {
			if len(fog.Numbers) == 0 {
				return 0, errors.New("探索数字牌已空")
			}
			number = fog.Numbers[len(fog.Numbers)-1]
			fog.Numbers = fog.Numbers[:len(fog.Numbers)-1]
			if number < 2 || number > 12 || number == 7 {
				return 0, errors.New("探索数字牌不合法")
			}
		}
		g.Tiles[id].Resource, g.Tiles[id].Number = resource, number
		switch resource {
		case CatanSea:
			s.catanLog(player, "探索地块 #%d，发现海洋", id+1)
		case CatanDesert:
			s.catanLog(player, "探索地块 #%d，发现沙漠", id+1)
		case CatanGold:
			if player < 0 {
				s.Log = append(s.Log, "中立势力发现金矿，不领取资源")
				break
			}
			gold++
			s.catanLog(player, "探索地块 #%d，发现金矿（%d），获得一次资源选择", id+1, number)
		default:
			if player < 0 {
				s.Log = append(s.Log, "中立势力发现资源地块，不领取资源")
				break
			}
			if g.Bank[resource] > 0 {
				g.Bank[resource]--
				g.Players[player].Resources[resource]++
				s.catanLog(player, "探索地块 #%d（%d），获得%s×1", id+1, number, CatanResources[resource])
			} else {
				s.catanLog(player, "探索地块 #%d（%d），但银行没有剩余%s", id+1, number, CatanResources[resource])
			}
		}
	}
	// Fog Islands has no foreign-island bonus. Its setup restriction uses the
	// original visible tiles, so newly connected land cannot open setup sites.
	g.Seafarers.Islands = g.findIslands()
	return gold, nil
}

func (s *State) catanAfterRoute(q CatanRouteCompletion) error {
	if e := s.Catan.Edges[q.Edge]; (!e.Ship || s.Catan.riversSea()) && !e.Bridge && s.Catan.riverEdge(q.Edge) {
		if err := s.catanRiverReward(q.Player, 1); err != nil {
			return err
		}
	}
	if s.Catan.Edges[q.Edge].Ship && !s.Catan.pirateCommitShip(q.Player, q.Edge) {
		return errors.New("远征船线必须按最短路径经自己的登陆点通往要塞，不能分叉")
	}
	s.Catan.Trade = nil
	s.catanClothTrade(q.Player)
	if err := s.catanCollectTribe(q.Player, q.Edge); err != nil {
		return err
	}
	if s.Catan.Attack != nil && s.Catan.Attack.Pending != nil && s.Catan.tribe() != nil && s.Catan.tribe().AttackRules == CatanAttackTribeRewardRules {
		s.Catan.Attack.TribeRoute = &q
		return nil
	}
	return s.catanContinueRouteRewards(q)
}
func (s *State) catanContinueRouteRewards(q CatanRouteCompletion) error {
	if s.Catan.tribe() != nil {
		s.catanScores()
		s.catanVictory()
		if s.Finished {
			return nil
		}
		if s.catanAskTribePort(q.Player, s.Phase, &q, false) {
			return nil
		}
	}
	gold, err := s.catanDiscover(q.Player, q.Edge)
	if err != nil {
		return err
	}
	if gold > 0 {
		s.Catan.GoldPending = &CatanGoldPending{Claims: []CatanGoldClaim{{Player: q.Player, Count: gold}}, Resume: s.Phase, AfterRoute: &q}
		s.catanContinueGold()
	} else {
		s.catanFinishRoute(q)
	}
	return nil
}

func (s *State) catanFinishRoute(q CatanRouteCompletion) {
	g := s.Catan
	if q.Setup {
		s.catanScores()
		s.catanFinishSetupRoute(q.Player, q.Edge)
		return
	}
	if q.Free {
		g.FreeRoads--
		if g.FreeRoads == 0 || !g.hasFreeRouteAction(q.Player) {
			s.Phase = g.ResumePhase
			g.FreeRoads = 0
		}
	}
	g.Trade = nil
	s.catanScores()
	s.catanVictory()
	if q.Helper {
		s.catanHelperComplete(q.Player, "catan_turn")
	}
}
