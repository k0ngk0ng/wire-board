package game

import (
	"errors"
	"slices"
)

// Nil merchant means nobody has played Merchant yet. Desert is legal but has
// no trading benefit. Gold fields, sea and undiscovered tiles are not legal.
type CatanMerchant struct {
	Owner int `json:"owner"`
	Tile  int `json:"tile"`
}

// These powers expire at the end of the acting player's action phase, including
// the first half of a paired turn. Each Commercial Harbor card has its own set
// of remaining offers; building/bank trades may occur between those offers.
type CatanTradePowers struct {
	Player  int     `json:"player"`
	Fleets  []int   `json:"fleets"`
	Harbors [][]int `json:"harbors"`
}

func (k *CatanCitiesKnights) tradePowers(player int) *CatanTradePowers {
	if k.TradePowers == nil {
		k.TradePowers = &CatanTradePowers{Player: player, Fleets: []int{}, Harbors: [][]int{}}
	}
	return k.TradePowers
}
func (g *Catan) merchantTiles(player int) []int {
	out := []int{}
	for _, t := range g.Tiles {
		if t.Resource < 0 || t.Resource > CatanDesert || g.attackKnights() && g.Attack.conquered(t.ID) {
			continue
		}
		for _, v := range t.Vertices {
			if g.Vertices[v].Owner == player && g.Vertices[v].Level > 0 {
				out = append(out, t.ID)
				break
			}
		}
	}
	return out
}
func (g *Catan) guildDuesTargets(player int) []int {
	out := []int{}
	for p, seat := range g.Players {
		if p != player && !seat.Eliminated && seat.Score-g.hiddenVictoryPoints(p) > g.Players[player].Score-g.hiddenVictoryPoints(player) {
			out = append(out, p)
		}
	}
	return out
}
func (s *State) catanTradeProgress(player int, a Action) error {
	g := s.Catan
	k := g.CitiesKnights
	switch a.Card {
	case 10:
		remaining := []int{}
		for p, seat := range g.Players {
			if p != player && !seat.Eliminated {
				remaining = append(remaining, p)
			}
		}
		powers := k.tradePowers(player)
		powers.Harbors = append(powers.Harbors, remaining)
	case 11:
		if !slices.Contains(g.guildDuesTargets(player), a.Target) {
			return errors.New("行会征费只能选择分数高于自己的在场玩家")
		}
		if sum(g.Players[a.Target].Resources) > 0 {
			k.Pending = &CatanCityPending{Kind: "guild_dues", Players: []int{player}, Target: a.Target}
			s.Phase = "catan_guild_dues"
		} else {
			s.catanLog(player, "所选玩家手中没有资源或商品，本次行会征费无收益")
		}
	case 12:
		if !slices.Contains(g.merchantTiles(player), a.Tile) {
			return errors.New("商人必须放在自己建筑旁的陆地，不能放在金矿")
		}
		if k.Merchant != nil && k.Merchant.Owner != player {
			s.catanLog(k.Merchant.Owner, "失去商人控制权和1分")
		}
		k.Merchant = &CatanMerchant{Owner: player, Tile: a.Tile}
		s.catanLog(player, "将商人放在地块 #%d，持有商人获得1分", a.Tile+1)
	case 13:
		if a.Color < 0 || a.Color >= len(g.Bank) {
			return errors.New("请选择商船队要使用的资源或商品种类")
		}
		powers := k.tradePowers(player)
		if !slices.Contains(powers.Fleets, a.Color) {
			powers.Fleets = append(powers.Fleets, a.Color)
		}
		s.catanLog(player, "本行动阶段可以用%s进行二比一银行交易", catanCardName(a.Color))
	case 14, 15:
		color, limit := a.Color, 2
		if a.Card == 14 && (color < 0 || color >= 5) {
			return errors.New("资源垄断只能选择普通资源")
		}
		if a.Card == 15 {
			limit = 1
			if color < 5 || color >= 8 {
				return errors.New("商品垄断只能选择纸张、布料或钱币")
			}
		}
		total := 0
		for p := range g.Players {
			if p == player || g.Players[p].Eliminated {
				continue
			}
			n := min(limit, g.Players[p].Resources[color])
			g.Players[p].Resources[color] -= n
			g.Players[player].Resources[color] += n
			total += n
		}
		s.catanLog(player, "通过垄断获得%s×%d（每位对手最多%d张）", catanCardName(color), total, limit)
	default:
		return errors.New("未知贸易进步牌")
	}
	s.catanScores()
	s.catanVictory()
	return nil
}
func (s *State) catanCommercialOffer(player int, a Action) error {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil || k.TradePowers == nil || k.TradePowers.Player != player || player != s.Turn || s.Phase != "catan_turn" {
		return errors.New("本行动阶段没有可用的商业港")
	}
	powers := k.TradePowers
	if a.Card < 0 || a.Card >= len(powers.Harbors) || !slices.Contains(powers.Harbors[a.Card], a.Target) || g.Players[a.Target].Eliminated {
		return errors.New("这张商业港不能再次向该玩家提出交换")
	}
	if a.Color < 0 || a.Color >= 5 || g.Players[player].Resources[a.Color] == 0 {
		return errors.New("商业港必须给出自己持有的一张普通资源")
	}
	powers.Harbors[a.Card] = slices.DeleteFunc(powers.Harbors[a.Card], func(p int) bool { return p == a.Target })
	g.Trade = nil
	if sum(g.Players[a.Target].Resources[5:]) == 0 {
		s.catanLog(player, "向玩家 %d 使用商业港；对方没有商品，收回资源", a.Target+1)
		return nil
	}
	// The resource stays in the owner's hand while the mandatory response is
	// pending; no other actions can spend it. Exchange both cards atomically.
	k.Pending = &CatanCityPending{Kind: "commercial_harbor", Players: []int{a.Target}, Target: player, Color: a.Color}
	s.Phase = "catan_commercial_harbor"
	s.catanLog(player, "向玩家 %d 给出1张资源，等待对方选择一张商品", a.Target+1)
	return nil
}
func (s *State) catanTradeProgressChoice(player int, a Action) error {
	g := s.Catan
	k := g.CitiesKnights
	q := k.Pending
	switch q.Kind {
	case "guild_dues":
		hand := g.Players[q.Target].Resources
		if s.Phase != "catan_guild_dues" || a.Type != "catan_guild_dues" || a.Choice != "" || !g.cardBundle(a.Take) || sum(a.Take) != min(2, sum(hand)) || !catanHas(hand, a.Take) {
			return errors.New("请选择对方手中的两张资源或商品；不足两张时全部取走")
		}
		catanMove(hand, g.Players[player].Resources, a.Take)
		// Revealed hand and selected types are private; the public log only counts.
		s.catanLog(player, "通过行会征费从玩家 %d 取得%d张卡", q.Target+1, sum(a.Take))
	case "commercial_harbor":
		if s.Phase != "catan_commercial_harbor" || a.Type != "catan_commercial_harbor" || a.Choice != "" || a.Color < 5 || a.Color >= 8 || g.Players[player].Resources[a.Color] == 0 {
			return errors.New("必须选择自己持有的一张商品进行商业港交换")
		}
		g.Players[player].Resources[a.Color]--
		g.Players[q.Target].Resources[a.Color]++
		g.Players[q.Target].Resources[q.Color]--
		g.Players[player].Resources[q.Color]++
		s.catanLog(q.Target, "与玩家 %d 完成商业港交换：1张资源换得1张商品", player+1)
	default:
		return errors.New("未知贸易进步牌回应")
	}
	k.Pending = nil
	s.Phase = "catan_turn"
	return nil
}
