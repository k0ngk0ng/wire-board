package game

import (
	"errors"
	"slices"
)

// Village number discs sit on intersections, not in hex centers. Relations
// are recorded once per player, independently of the village's remaining stock.
type CatanClothVillage struct {
	Vertex  int   `json:"vertex"`
	Number  int   `json:"number"`
	Stock   int   `json:"stock"`
	Traders []int `json:"traders"`
}
type CatanClothState struct {
	Villages   []CatanClothVillage `json:"villages"`
	Stock      int                 `json:"stock"`
	Held       []int               `json:"held"`
	HomeTiles  []int               `json:"homeTiles"`
	EmptyLimit int                 `json:"emptyLimit"`
}

// Variable cloth setups let the starting seat choose either large-island 12.
// The generator's legal default remains available for timeout/autoplay.
func (s *State) catanClothStart(player int, a Action) error {
	g := s.Catan
	if g.cloth() == nil || !g.Seafarers.Variable || g.SetupStep != 0 || s.Phase != "catan_cloth_start" || player != s.Turn || a.Type != "catan_cloth_start" || !slices.Contains(g.clothStartTiles(), a.Tile) {
		return errors.New("请由先手选择大岛上的12号地块作为强盗起点")
	}
	g.Robber = a.Tile
	s.Phase = "catan_setup_settlement"
	s.catanLog(player, "选择12号地块 #%d 作为强盗起点，开始放置起始村庄", a.Tile+1)
	return nil
}
func (g *Catan) clothStartTiles() []int {
	result := []int{}
	if c := g.cloth(); c != nil {
		for _, id := range c.HomeTiles {
			if g.Tiles[id].Number == 12 {
				result = append(result, id)
			}
		}
	}
	return result
}

func (g *Catan) cloth() *CatanClothState {
	if g.Seafarers == nil {
		return nil
	}
	return g.Seafarers.Cloth
}
func (g *Catan) clothLand(tile int) bool {
	return g.cloth() == nil || slices.Contains(g.cloth().HomeTiles, tile)
}
func (g *Catan) pirateAllowed(player int) bool {
	if g.Seafarers == nil || g.pirateIslands() != nil || g.wonders() != nil {
		return false
	}
	if c := g.cloth(); c != nil {
		for _, village := range c.Villages {
			if slices.Contains(village.Traders, player) {
				return true
			}
		}
		return false
	}
	return true
}
func (g *Catan) clothShipAnchor(player, vertex int) bool {
	if c := g.cloth(); c != nil {
		for _, village := range c.Villages {
			if village.Vertex == vertex && slices.Contains(village.Traders, player) {
				return true
			}
		}
	}
	return false
}

// Starting at every own building enforces the road/ship junction rule: roads
// alone cannot establish a village trade route. Opponent buildings cannot be
// traversed to establish a new relation; existing relations are retained.
func (s *State) catanClothTrade(player int) {
	g := s.Catan
	c := g.cloth()
	if c == nil {
		return
	}
	reached := map[int]bool{}
	queue := []int{}
	for _, v := range g.Vertices {
		if v.Owner == player && v.Level > 0 {
			reached[v.ID] = true
			queue = append(queue, v.ID)
		}
	}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		if g.Vertices[v].Level > 0 && g.Vertices[v].Owner != player {
			continue
		}
		for _, id := range g.touching(v) {
			e := g.Edges[id]
			if e.Owner != player || !e.Ship {
				continue
			}
			next := e.A
			if next == v {
				next = e.B
			}
			if !reached[next] {
				reached[next] = true
				queue = append(queue, next)
			}
		}
	}
	for i := range c.Villages {
		v := &c.Villages[i]
		if !reached[v.Vertex] || slices.Contains(v.Traders, player) {
			continue
		}
		v.Traders = append(v.Traders, player)
		if v.Stock > 0 {
			v.Stock--
			c.Held[player]++
			s.catanLog(player, "与布匹村落 #%d（%d）建立贸易，领取布匹×1", i+1, v.Number)
		} else {
			s.catanLog(player, "与布匹村落 #%d（%d）建立贸易；村落库存已空", i+1, v.Number)
		}
	}
}

// A depleted village does not draw from the common stock. When a nonempty
// village produces, everyone with a relation is entitled to one token.
func (s *State) catanProduceCloth(number int) error {
	g := s.Catan
	c := g.cloth()
	if c == nil {
		return nil
	}
	for i := range c.Villages {
		v := &c.Villages[i]
		if v.Number != number || v.Stock <= 0 {
			continue
		}
		traders := []int{}
		for _, p := range v.Traders {
			if p < 0 || p >= len(g.Players) || p >= len(c.Held) {
				return errors.New("布匹村落的贸易座位数据不完整")
			}
			if !g.Players[p].Eliminated && !slices.Contains(traders, p) {
				traders = append(traders, p)
			}
		}
		needed := max(0, len(traders)-v.Stock)
		if needed > c.Stock {
			// This fail-closed invariant is not a verified shortage rule. Resolve
			// common-supply exhaustion before exposing the scenario (see checklist).
			return errors.New("布匹公共库存不足以补足本次产出")
		}
		v.Stock -= min(v.Stock, len(traders))
		c.Stock -= needed
		for _, p := range traders {
			c.Held[p]++
			s.catanLog(p, "从布匹村落 #%d（%d）获得布匹×1", i+1, number)
		}
	}
	return nil
}

func (s *State) catanClothEnd() bool {
	g := s.Catan
	c := g.cloth()
	if c == nil || g.setup() || s.Finished {
		return s.Finished
	}
	empty := 0
	for _, v := range c.Villages {
		if v.Stock == 0 {
			empty++
		}
	}
	if c.EmptyLimit <= 0 || empty < c.EmptyLimit {
		return false
	}
	s.catanScores()
	best, cloth := -1, -1
	winners := []int{}
	for i, p := range g.Players {
		if p.Eliminated {
			continue
		}
		if p.Score > best || (p.Score == best && c.Held[i] > cloth) {
			best, cloth = p.Score, c.Held[i]
			winners = []int{i}
		} else if p.Score == best && c.Held[i] == cloth {
			winners = append(winners, i)
		}
	}
	s.Finished, s.Phase, s.Winners = true, "finished", winners
	g.Trade = nil
	s.Log = append(s.Log, "本轮结束时布匹村落已耗尽至终局条件：比较总分，同分时比较布匹数量")
	return true
}

func (s *State) catanClothSteal(player int, a Action) error {
	g := s.Catan
	c := g.cloth()
	if c == nil || s.Phase != "catan_cloth_steal" || player != s.Turn || !slices.Contains(g.Victims, a.Target) {
		return errors.New("请选择海盗旁有资源或布匹的对手")
	}
	switch a.Choice {
	case "resource":
		if sum(g.Players[a.Target].Resources) == 0 {
			return errors.New("该玩家没有可偷取的资源")
		}
		s.Phase = "catan_steal"
		return s.catanSteal(player, a.Target)
	case "cloth":
		if c.Held[a.Target] <= 0 {
			return errors.New("该玩家没有可偷取的布匹")
		}
		c.Held[a.Target]--
		c.Held[player]++
		s.catanLog(player, "通过海盗从玩家 %d 偷取布匹×1", a.Target+1)
		g.Victims = []int{}
		s.Phase = g.ResumePhase
		s.catanScores()
		s.catanVictory()
	default:
		return errors.New("请选择偷取一张资源或一枚布匹")
	}
	return nil
}

func (s *State) catanClothStealBot(player int) (Action, error) {
	g := s.Catan
	c := g.cloth()
	if c == nil || player != s.Turn || s.Phase != "catan_cloth_steal" {
		return Action{}, errors.New("inactive cloth theft seat")
	}
	choices := []botChoice{}
	for _, target := range g.Victims {
		if c.Held[target] > 0 {
			value := 50 + (c.Held[player]%2)*20
			if c.Held[target]%2 == 0 {
				value += 10
			}
			choices = append(choices, botChoice{Action{Type: "catan_cloth_steal", Target: target, Choice: "cloth"}, value})
		}
		if count := sum(g.Players[target].Resources); count > 0 {
			choices = append(choices, botChoice{Action{Type: "catan_cloth_steal", Target: target, Choice: "resource"}, 20 + min(count, 5)})
		}
	}
	return s.botLegal(player, choices)
}
