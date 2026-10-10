package game

import (
	"errors"
	"slices"
)

const CatanPirateKnightsRules = "wire-board-pirate-knights-v1"
const catanPirateKnightsNotice = "本站补充规则：此前已激活的骑士可转为未激活，将远征线最近的普通船升级战舰；骑士与战舰分别抵御蛮族与舰队。首次蛮族进攻后舰队入场；先城市事件、再舰队、最后生产。炼金术所选骰子同时决定舰队步数；征税选择陆地但不移动强盗，海战失去路线连接的骑士返回库存。"

func (g *Catan) pirateKnightSite(vertex int) bool {
	p := g.pirateIslands()
	return p == nil || !slices.ContainsFunc(p.Fortresses, func(f CatanPirateFortress) bool { return f.Strength > 0 && f.Vertex == vertex })
}
func (g *Catan) knightCanWarship(n *CatanKnight) bool {
	p := g.pirateIslands()
	return p != nil && p.KnightsRules == CatanPirateKnightsRules && g.knightCanAct(n) && g.pirateNextWarship(n.Owner) >= 0
}
func (g *Catan) pirateFortressesRemain() bool {
	p := g.pirateIslands()
	return p != nil && slices.ContainsFunc(p.Fortresses, func(f CatanPirateFortress) bool { return f.Strength > 0 })
}

// Forced sea-battle losses may break anchoring; unlike voluntary ship moves,
// they are not vetoed by knights. Return only pieces losing their own network.
func (s *State) catanPirateStrandedKnights() {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil || g.pirateIslands() == nil {
		return
	}
	k.Knights = slices.DeleteFunc(k.Knights, func(n CatanKnight) bool {
		reachable := make([]bool, len(g.Vertices))
		reachable[n.Vertex] = true
		queue := []int{n.Vertex}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			for _, id := range g.touching(v) {
				e := g.Edges[id]
				if e.Owner != n.Owner {
					continue
				}
				to := e.A
				if to == v {
					to = e.B
				}
				if !reachable[to] {
					reachable[to] = true
					queue = append(queue, to)
				}
			}
		}
		for _, v := range g.Vertices {
			if v.Owner == n.Owner && v.Level > 0 && reachable[v.ID] {
				return false
			}
		}
		s.catanLog(n.Owner, "海战损失切断骑士与己方建筑的连接，交点 #%d 的骑士返回库存", n.Vertex+1)
		return true
	})
}

func (s *State) catanCityFleetProduction(total int, epidemic bool) error {
	g := s.Catan
	if g.EventDeck != nil {
		return s.catanEventProduction(total, epidemic)
	}
	f := g.pirateIslands().CityFleet
	if f == nil || f.RollID != g.RollID || f.Resolved {
		return errors.New("城市骑士舰队骰子缺失或已结算")
	}
	f.Resolved = true
	if pending, err := s.catanRaidFleetDice(total, epidemic, f.Dice); pending || err != nil {
		return err
	}
	return s.catanRollProductionEffect(total, epidemic)
}

func (s *State) validatePirateKnights() error {
	g := s.Catan
	p := g.pirateIslands()
	k := g.CitiesKnights
	if p == nil {
		return nil
	}
	if g.attackSeaKnights() {
		if p.KnightsRules != CatanPirateKnightsRules || p.CityFleet != nil || p.Raid != nil {
			return errors.New("蛮族海图骑士的海盗规则无效")
		}
		for _, n := range k.Knights {
			if n.Vertex < 0 || n.Vertex >= len(g.Vertices) || n.Owner < 0 || n.Owner >= len(g.Players) || !g.pirateKnightSite(n.Vertex) {
				return errors.New("海盗骑士位置或归属无效")
			}
		}
		return nil
	}
	if k == nil {
		if p.KnightsRules != "" || p.CityFleet != nil {
			return errors.New("普通海盗群岛混入城市骑士规则")
		}
		return nil
	}
	if p.KnightsRules != CatanPirateKnightsRules || g.Robber != -1 || k.RobberStart != -1 || k.Chase != "" || len(g.DevDeck)+len(g.DevDiscard) != 0 {
		return errors.New("海盗骑士规则或强盗状态无效")
	}
	if k.Invasions == 0 && g.Seafarers.Pirate != -1 || !g.pirateFortressesRemain() && g.Seafarers.Pirate != -1 {
		return errors.New("海盗舰队休眠或退场状态无效")
	}
	for player := range g.Players {
		if _, ok := g.pirateRouteVertices(player); !ok {
			return errors.New("海盗骑士远征航线无效")
		}
		if sum(g.Players[player].Dev)+sum(g.Players[player].NewDev) != 0 {
			return errors.New("海盗骑士混入发展牌")
		}
	}
	for _, n := range k.Knights {
		if n.Vertex < 0 || n.Vertex >= len(g.Vertices) || n.Owner < 0 || n.Owner >= len(g.Players) {
			return errors.New("海盗骑士位置或归属无效")
		}
		if !g.pirateKnightSite(n.Vertex) {
			return errors.New("骑士不能占据未夺回的要塞")
		}
	}
	for _, e := range g.Edges {
		if e.Warship && (!e.Ship || e.Owner < 0) {
			return errors.New("战舰必须是玩家的船只")
		}
	}
	if g.EventDeck != nil {
		if p.CityFleet != nil {
			return errors.New("事件牌组合包含重复舰队骰子")
		}
		return nil
	}
	f := p.CityFleet
	if g.RollID == 0 {
		if f != nil || p.Raid != nil {
			return errors.New("掷骰前不能有舰队结算")
		}
		return nil
	}
	if f == nil || f.RollID != g.RollID || len(g.Dice) != 2 || f.Dice != [2]int{g.Dice[0], g.Dice[1]} || f.Dice[0] < 1 || f.Dice[0] > 6 || f.Dice[1] < 1 || f.Dice[1] > 6 {
		return errors.New("城市骑士舰队骰子与回合不一致")
	}
	if k.Event != nil && (f.Resolved || p.Raid != nil) || k.Event == nil && !s.Finished && !f.Resolved {
		return errors.New("城市事件与舰队结算顺序不一致")
	}
	if q := p.Raid; q != nil {
		if s.Phase != "catan_fleet_reward" || q.Total != sum(g.Dice) || q.Epidemic || len(q.Rewards) == 0 {
			return errors.New("舰队奖励接续状态无效")
		}
		seen := map[int]bool{}
		for _, player := range q.Rewards {
			if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || seen[player] {
				return errors.New("舰队奖励回应者无效")
			}
			seen[player] = true
		}
	} else if s.Phase == "catan_fleet_reward" {
		return errors.New("舰队奖励队列缺失")
	}
	return nil
}
