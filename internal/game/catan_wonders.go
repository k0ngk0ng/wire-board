package game

import (
	"errors"
	"slices"
)

// IDs and costs follow the 2025 Seafarers component tiles. Costs use the
// engine's wood, brick, wool, grain, ore order and apply to EACH of four levels.
type CatanWonderRule struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Requirement string `json:"requirement"`
	Cost        [5]int `json:"cost"`
}

var catanWonderRules = [...]CatanWonderRule{
	{0, "大城堡", "至少一座城市且达到6分", [5]int{0, 1, 0, 1, 3}},
	{1, "大纪念碑", "至少一座港口城市，且拥有至少5段道路或船只组成的连续贸易路线", [5]int{0, 0, 0, 3, 2}},
	{2, "大剧院", "至少两座城市", [5]int{1, 1, 3, 0, 0}},
	{3, "大桥", "在任一大桥标记处拥有建筑", [5]int{3, 0, 1, 1, 0}},
	{4, "长城", "在任一长城标记处拥有建筑", [5]int{1, 3, 0, 1, 0}},
	{5, "灯塔", "在任一灯塔标记处拥有建筑", [5]int{3, 0, 1, 1, 0}},
	{6, "大图书馆", "至少两座城市", [5]int{1, 1, 3, 0, 0}},
}

type CatanWonder struct {
	ID    int `json:"id"`
	Owner int `json:"owner"` // -1 unclaimed; claiming does not build level 1
	Level int `json:"level"`
}

type CatanWonderMarker struct {
	Card   int `json:"card"`
	Vertex int `json:"vertex"`
}

type CatanWonders struct {
	Cards        []CatanWonder       `json:"cards"`
	Markers      []CatanWonderMarker `json:"markers"`
	SetupBlocked []int               `json:"setupBlocked"`
}

func (g *Catan) wonders() *CatanWonders {
	if g.Seafarers == nil {
		return nil
	}
	return g.Seafarers.Wonders
}

func (g *Catan) wonderOwned(player int) int {
	if w := g.wonders(); w != nil {
		for _, card := range w.Cards {
			if card.Owner == player {
				return card.ID
			}
		}
	}
	return -1
}

func (g *Catan) wonderClaimable(player, id int) bool {
	w := g.wonders()
	if w == nil || id < 0 || id >= len(w.Cards) || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || w.Cards[id].Owner >= 0 || g.wonderOwned(player) >= 0 {
		return false
	}
	_, _, cities := g.pieces(player)
	switch id {
	case 0:
		return cities >= 1 && g.Players[player].Score >= 6
	case 1:
		if g.roadLength(player) < 5 {
			return false
		}
		// The printed card requires both a port city and a five-segment
		// trade route; it does not require that route to start at that city.
		for _, port := range g.Ports {
			e := g.Edges[port.Edge]
			for _, v := range []int{e.A, e.B} {
				if g.Vertices[v].Owner == player && g.Vertices[v].Level == 2 {
					return true
				}
			}
		}
	case 2, 6:
		return cities >= 2
	default:
		for _, marker := range w.Markers {
			v := g.Vertices[marker.Vertex]
			if marker.Card == id && v.Owner == player && v.Level > 0 {
				return true
			}
		}
	}
	return false
}

func (s *State) catanWonderAction(player int, a Action) error {
	g := s.Catan
	w := g.wonders()
	if s.Phase != "catan_turn" || w == nil || a.Card < 0 || a.Card >= len(w.Cards) {
		return errors.New("当前不能领取或建造该奇迹")
	}
	card := &w.Cards[a.Card]
	rule := catanWonderRules[a.Card]
	if a.Type == "catan_wonder_claim" {
		if !g.wonderClaimable(player, a.Card) {
			return errors.New("尚未满足条件、奇迹已被领取，或你已拥有另一座奇迹")
		}
		card.Owner = player
		s.catanLog(player, "领取奇迹「%s」，尚未开始建造", rule.Name)
		return nil
	}
	if card.Owner != player || card.Level >= 4 {
		return errors.New("只能建造自己已领取且尚未完成的奇迹")
	}
	if !catanHas(g.Players[player].Resources, rule.Cost[:]) {
		return errors.New("建造奇迹的资源不足")
	}
	// Requirements are checked when claiming, not again on every level.
	catanMove(g.Players[player].Resources, g.Bank, rule.Cost[:])
	card.Level++
	s.catanLog(player, "支付%s，将奇迹「%s」建至第 %d 级", catanText(rule.Cost[:]), rule.Name, card.Level)
	s.catanVictory()
	return nil
}

func (g *Catan) wonderVictory(player int) bool {
	id := g.wonderOwned(player)
	if id < 0 {
		return false
	}
	w := g.wonders()
	level := w.Cards[id].Level
	if level == 4 {
		return true
	}
	if level == 0 || g.Players[player].Score < g.victoryTargetFor(player) {
		return false
	}
	for _, other := range w.Cards {
		if other.Owner >= 0 && other.Owner != player && other.Level >= level {
			return false
		}
	}
	return true
}

func (g *Catan) wonderBotChoices(player int) []botChoice {
	choices := []botChoice{}
	if w := g.wonders(); w != nil {
		for _, card := range w.Cards {
			if g.wonderClaimable(player, card.ID) {
				// Prefer a wonder whose recurring cost matches our own hand.
				score := 900
				for color, cost := range catanWonderRules[card.ID].Cost {
					score += min(cost, g.Players[player].Resources[color]) * 10
				}
				choices = append(choices, botChoice{Action{Type: "catan_wonder_claim", Card: card.ID}, score})
			}
			if card.Owner == player && card.Level < 4 {
				choices = append(choices, botChoice{Action{Type: "catan_wonder_build", Card: card.ID}, 800 + card.Level*50})
			}
		}
	}
	return choices
}

func (g *Catan) wonderVertexValue(player, vertex int) int {
	if w := g.wonders(); w != nil && !g.setup() && g.wonderOwned(player) < 0 {
		for _, marker := range w.Markers {
			if marker.Vertex == vertex && w.Cards[marker.Card].Owner < 0 {
				return 300
			}
		}
	}
	return 0
}

func catanBotBuildCost(a Action) []int {
	if a.Type == "catan_wonder_build" && a.Card >= 0 && a.Card < len(catanWonderRules) {
		return catanWonderRules[a.Card].Cost[:]
	}
	return catanPrices[a.Type]
}

// The official fixed and variable setups both allow any desert. Let the
// starting seat resolve that shared setup choice before placing any pieces.
func (g *Catan) wonderStartTiles() []int {
	tiles := []int{}
	if g.wonders() != nil {
		for _, tile := range g.Tiles {
			if tile.Resource == CatanDesert {
				tiles = append(tiles, tile.ID)
			}
		}
	}
	return tiles
}

func (s *State) catanWondersStart(player int, a Action) error {
	g := s.Catan
	if g.wonders() == nil || g.SetupStep != 0 || s.Phase != "catan_wonders_start" || player != s.Turn || a.Type != "catan_wonders_start" || !slices.Contains(g.wonderStartTiles(), a.Tile) {
		return errors.New("请由先手选择沙漠地块作为强盗起点")
	}
	if k := g.CitiesKnights; k != nil {
		// Select the future origin without waking the robber before invasion.
		k.RobberStart = a.Tile
		g.Robber = -1
	} else {
		g.Robber = a.Tile
	}
	s.Phase = "catan_setup_settlement"
	s.catanLog(player, "选择沙漠地块 #%d 作为强盗起点，开始放置起始村庄", a.Tile+1)
	return nil
}
