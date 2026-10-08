package game

import (
	"errors"
	"slices"
)

// A victorious defense is resolved before production / seven discards. The
// stored continuation prevents reloads and timeouts from moving the fleet twice.
type CatanPirateRaid struct {
	Rewards  []int `json:"rewards"`
	Total    int   `json:"total"`
	Epidemic bool  `json:"epidemic,omitempty"`
}

func (g *Catan) pirateIslands() *CatanPirateIslands {
	if g.Seafarers == nil {
		return nil
	}
	return g.Seafarers.PirateIslands
}
func (g *Catan) warships(player int) int {
	count := 0
	for _, e := range g.Edges {
		if e.Owner == player && e.Ship && e.Warship {
			count++
		}
	}
	return count
}

// catanRaidFleet returns true when it started an asynchronous reward choice.
func (s *State) catanRaidFleet(total int) (bool, error) {
	g := s.Catan
	if g.pirateIslands() == nil || g.Seafarers.Pirate < 0 {
		return false, nil
	}
	if len(g.Dice) != 2 || total != sum(g.Dice) {
		return false, errors.New("海盗巡航骰子无效")
	}
	return s.catanRaidFleetDice(total, false, [2]int{g.Dice[0], g.Dice[1]})
}

func (s *State) catanRaidFleetDice(total int, epidemic bool, dice [2]int) (bool, error) {
	g := s.Catan
	p := g.pirateIslands()
	if p == nil || g.Seafarers.Pirate < 0 {
		return false, nil
	}
	if p.Raid != nil {
		return false, errors.New("海盗进攻尚未结算")
	}
	at := slices.Index(p.FleetPath, g.Seafarers.Pirate)
	if at < 0 || dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 || total < 2 || total > 12 {
		return false, errors.New("海盗巡航状态无效")
	}
	strength := min(dice[0], dice[1])
	tile := p.FleetPath[(at+strength)%len(p.FleetPath)]
	if tile < 0 || tile >= len(g.Tiles) || g.Tiles[tile].Resource != CatanSea {
		return false, errors.New("海盗巡航必须停在海域")
	}
	g.Seafarers.Pirate = tile
	s.catanLog(s.Turn, "海盗舰队前进 %d 格，抵达海域 #%d（力量 %d）", strength, tile+1, strength)
	if tile == p.SafeTile {
		s.Log = append(s.Log, "海盗舰队进入 ! 海域，本次不发动攻击")
		return false, nil
	}
	attacked := map[int]bool{}
	for _, id := range g.Tiles[tile].Vertices {
		v := g.Vertices[id]
		if v.Level > 0 && !g.Players[v.Owner].Eliminated {
			attacked[v.Owner] = true
		}
	}
	q := &CatanPirateRaid{Total: total, Epidemic: epidemic}
	for step := 0; step < len(g.Players); step++ {
		player := (s.Turn + step) % len(g.Players)
		if !attacked[player] {
			continue
		}
		defense := g.warships(player)
		if defense > strength {
			q.Rewards = append(q.Rewards, player)
			s.catanLog(player, "以 %d 艘战舰击退力量 %d 的海盗，可领取一张资源", defense, strength)
		} else if defense == strength {
			s.catanLog(player, "以 %d 艘战舰与海盗打平，无资源损失", defense)
		} else {
			_, _, cities := g.pieces(player)
			hand := g.Players[player].Resources
			count := min(1+cities, sum(hand))
			for range count {
				pick := catanRandom(sum(hand))
				for color, n := range hand {
					if pick < n {
						hand[color]--
						g.Bank[color]++
						break
					}
					pick -= n
				}
			}
			s.catanLog(player, "未挡住力量 %d 的海盗，随机失去 %d 张资源", strength, count)
		}
	}
	if len(q.Rewards) > 0 && sum(g.Bank) > 0 {
		p.Raid = q
		s.Phase = "catan_fleet_reward"
		return true, nil
	}
	if len(q.Rewards) > 0 {
		s.Log = append(s.Log, "银行无剩余资源，本次防守奖励无法领取")
	}
	return false, nil
}
func (s *State) catanFleetReward(player int, a Action) error {
	g := s.Catan
	p := g.pirateIslands()
	if p == nil || p.Raid == nil || len(p.Raid.Rewards) == 0 || p.Raid.Rewards[0] != player || s.Phase != "catan_fleet_reward" || a.Type != "catan_fleet_reward" {
		return errors.New("请等待击退海盗的玩家选择奖励")
	}
	if a.Color < 0 || a.Color >= 5 || g.Bank[a.Color] <= 0 {
		return errors.New("请选择银行仍有库存的一张资源")
	}
	g.Bank[a.Color]--
	g.Players[player].Resources[a.Color]++
	s.catanLog(player, "击退海盗后领取 %s×1", CatanResources[a.Color])
	q := p.Raid
	q.Rewards = q.Rewards[1:]
	for len(q.Rewards) > 0 && g.Players[q.Rewards[0]].Eliminated {
		q.Rewards = q.Rewards[1:]
	}
	if len(q.Rewards) > 0 && sum(g.Bank) > 0 {
		return nil
	}
	if len(q.Rewards) > 0 {
		s.Log = append(s.Log, "银行无剩余资源，其余防守奖励无法领取")
	}
	p.Raid = nil
	return s.catanRollProductionEffect(q.Total, q.Epidemic)
}
func (s *State) catanFleetRewardBot(player int) (Action, error) {
	g := s.Catan
	p := g.pirateIslands()
	if p == nil || p.Raid == nil || len(p.Raid.Rewards) == 0 || p.Raid.Rewards[0] != player {
		return Action{}, errors.New("inactive fleet reward seat")
	}
	best := -1
	for color, count := range g.Bank {
		if count > 0 && (best < 0 || g.Players[player].Resources[color] < g.Players[player].Resources[best]) {
			best = color
		}
	}
	if best < 0 {
		return Action{}, errors.New("海盗奖励资源库存为空")
	}
	return Action{Type: "catan_fleet_reward", Color: best}, nil
}

// The scenario has no robber. After seven discards (and any Thorolf choice),
// the active player may steal from any non-eliminated opponent with resources.
func (s *State) catanPirateSeven() error {
	g := s.Catan
	p := g.pirateIslands()
	if p == nil || !p.SevenPending || s.Phase != "catan_robber" {
		return nil
	}
	p.SevenPending = false
	g.Victims = []int{}
	for i, p := range g.Players {
		if i != s.Turn && !p.Eliminated && sum(p.Resources) > 0 {
			g.Victims = append(g.Victims, i)
		}
	}
	s.Phase = "catan_steal"
	if len(g.Victims) == 0 {
		s.Phase = g.ResumePhase
	}
	return nil
}
