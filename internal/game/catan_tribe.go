package game

import (
	"errors"
	"slices"
)

// Forgotten Tribe edge rewards are consumed by the first ship reaching them.
// Card identities stay private until they become part of a player's hand.
type CatanTribeDevelopment struct {
	Edge int `json:"edge"`
	Card int `json:"card"`
}
type CatanTribePortPending struct {
	Player     int                   `json:"player"`
	Resume     string                `json:"resume"`
	AfterRoute *CatanRouteCompletion `json:"afterRoute,omitempty"`
	Helper     bool                  `json:"helper,omitempty"`
}
type CatanTribeState struct {
	AttackRules   string                  `json:"attackRules,omitempty"`
	ProgressRules string                  `json:"progressRules,omitempty"`
	Tokens        []int                   `json:"tokens"`
	Development   []CatanTribeDevelopment `json:"development"`
	Ports         []CatanPort             `json:"ports"`
	Points        []int                   `json:"points"`
	HeldPorts     [][]int                 `json:"heldPorts"`
	Pending       *CatanTribePortPending  `json:"pending,omitempty"`
}

func (g *Catan) tribe() *CatanTribeState {
	if g.Seafarers == nil {
		return nil
	}
	return g.Seafarers.Tribe
}
func (g *Catan) robberAllowed(tile int) bool {
	if tile == -1 {
		return g.friendlyRobberOutsideAllowed()
	}
	if g.Attack != nil {
		return false
	}
	if !g.robberLandAllowed(tile) {
		return false
	}
	if tile != g.Robber && !g.friendlyRobberBlocks(tile) {
		return true
	}
	if g.FriendlyRobber == nil || g.Tiles[tile].Resource != CatanDesert {
		return false
	}
	// The printed fallback takes precedence over adjacency protection. If the
	// robber is already on the only available desert it remains there; the
	// stealing filter still protects players with fewer than three visible VPs.
	for _, t := range g.Tiles {
		if t.ID != g.Robber && g.robberLandAllowed(t.ID) && !g.friendlyRobberBlocks(t.ID) {
			return false
		}
	}
	return true
}
func (g *Catan) robberLandAllowed(tile int) bool {
	if k := g.CitiesKnights; k != nil && (k.Invasions == 0 || k.Chase == "pirate") {
		return false
	}
	if g.pirateIslands() != nil {
		return false
	}
	if tile < 0 || tile >= len(g.Tiles) {
		return false
	}
	t := g.Tiles[tile]
	return t.Resource != CatanSea && t.Resource != CatanFog && g.tribeLand(tile) && g.clothLand(tile)
}

func (g *Catan) tribeLand(tile int) bool {
	if g.caravansSea() && slices.Contains(g.Caravans.Map.WateringHoles, tile) {
		return true
	}
	if g.tribe() == nil || g.Tiles[tile].Number > 0 || g.riversSea() && slices.Contains(g.Rivers.Map.Swamps, tile) {
		return true
	}
	// The replacement lake has four printed production numbers rather than
	// an ordinary disc. It remains part of the buildable/robbable mainland.
	return g.Fishing != nil && slices.ContainsFunc(g.Fishing.Map.Lakes, func(l catanFishingLake) bool { return l.Tile == tile })
}
func (g *Catan) tribePortEdges(player int) []int {
	result := []int{}
	if g.tribe() == nil || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return result
	}
	blocked := map[int]bool{}
	ports := append(append([]CatanPort{}, g.Ports...), g.tribe().Ports...)
	for _, p := range ports {
		e := g.Edges[p.Edge]
		blocked[e.A] = true
		blocked[e.B] = true
	}
	for _, e := range g.Edges {
		coast := *g
		if g.riversSea() {
			coast.Rivers = nil
		}
		if blocked[e.A] || blocked[e.B] || g.fishingGroundEdge(e.ID) || !coast.edgeTerrain(e.ID, true) || !coast.edgeTerrain(e.ID, false) {
			continue
		}
		a, b := g.Vertices[e.A], g.Vertices[e.B]
		if (a.Owner == player && a.Level > 0) || (b.Owner == player && b.Level > 0) {
			result = append(result, e.ID)
		}
	}
	return result
}
func (s *State) catanCollectTribe(player, edge int) error {
	g := s.Catan
	t := g.tribe()
	if t == nil || edge < 0 || edge >= len(g.Edges) || !g.Edges[edge].Ship {
		return nil
	}
	if len(t.Points) != len(g.Players) || len(t.HeldPorts) != len(g.Players) {
		return errors.New("部族奖励座位数据不完整")
	}
	for i := len(t.Tokens) - 1; i >= 0; i-- {
		if t.Tokens[i] == edge {
			t.Tokens = append(t.Tokens[:i], t.Tokens[i+1:]...)
			t.Points[player]++
			s.catanLog(player, "从遗忘部族领取胜利点，额外获得1分")
		}
	}
	for i := len(t.Development) - 1; i >= 0; i-- {
		reward := t.Development[i]
		if reward.Edge == edge {
			if g.CitiesKnights != nil {
				if t.ProgressRules != CatanTribeProgressRules || player != s.Turn || reward.Card < 0 || reward.Card >= len(catanProgressRules) {
					return errors.New("部族进步牌奖励数据或领取者不合法")
				}
				t.Development = append(t.Development[:i], t.Development[i+1:]...)
				s.catanLog(player, "从遗忘部族领取一张进步牌")
				s.catanGrantProgress(player, reward.Card)
			} else {
				if reward.Card < 0 || reward.Card >= len(g.Players[player].Dev) {
					return errors.New("部族发展卡数据不合法")
				}
				g.Players[player].Dev[reward.Card]++
				g.Players[player].NewDev[reward.Card]++
				t.Development = append(t.Development[:i], t.Development[i+1:]...)
				s.catanLog(player, "从遗忘部族领取一张发展卡")
			}
		}
	}
	for i := len(t.Ports) - 1; i >= 0; i-- {
		port := t.Ports[i]
		if port.Edge == edge {
			if port.Resource < -1 || port.Resource > 4 {
				return errors.New("部族港口数据不合法")
			}
			t.HeldPorts[player] = append(t.HeldPorts[player], port.Resource)
			t.Ports = append(t.Ports[:i], t.Ports[i+1:]...)
			s.catanLog(player, "从遗忘部族领取%s", catanPortName(port.Resource))
		}
	}
	return nil
}
func catanPortName(resource int) string {
	if resource < 0 {
		return "通用3:1港口"
	}
	return CatanResources[resource] + "2:1港口"
}
func (s *State) catanAskTribePort(player int, resume string, route *CatanRouteCompletion, helper bool) bool {
	g := s.Catan
	t := g.tribe()
	if s.Finished || t == nil || player < 0 || player >= len(t.HeldPorts) || len(t.HeldPorts[player]) == 0 || len(g.tribePortEdges(player)) == 0 {
		return false
	}
	t.Pending = &CatanTribePortPending{Player: player, Resume: resume, AfterRoute: route, Helper: helper}
	g.Trade = nil
	s.Phase = "catan_port"
	return true
}
func (s *State) catanPlaceTribePort(player int, a Action) error {
	g := s.Catan
	t := g.tribe()
	if t == nil || t.Pending == nil || s.Phase != "catan_port" || t.Pending.Player != player || a.Type != "catan_port" {
		return errors.New("请由领取者完成港口安放")
	}
	if player >= len(t.HeldPorts) || a.Slot < 0 || a.Slot >= len(t.HeldPorts[player]) || !slices.Contains(g.tribePortEdges(player), a.Edge) {
		return errors.New("请选择暂存的港口及己方海岸建筑旁的空位置，港口之间至少间隔一条边")
	}
	resource := t.HeldPorts[player][a.Slot]
	if resource < -1 || resource > 4 {
		return errors.New("港口种类不合法")
	}
	g.Ports = append(g.Ports, CatanPort{Edge: a.Edge, Resource: resource})
	t.HeldPorts[player] = append(t.HeldPorts[player][:a.Slot], t.HeldPorts[player][a.Slot+1:]...)
	s.catanLog(player, "在海岸 #%d 安放%s，可立即使用", a.Edge+1, catanPortName(resource))
	s.catanScores()
	s.catanVictory()
	if s.Finished {
		t.Pending = nil
		return nil
	}
	if len(t.HeldPorts[player]) > 0 && len(g.tribePortEdges(player)) > 0 {
		return nil
	}
	q := t.Pending
	t.Pending = nil
	s.Phase = q.Resume
	if q.AfterRoute != nil {
		s.catanFinishRoute(*q.AfterRoute)
	} else if q.Helper {
		s.catanHelperComplete(player, q.Resume)
	}
	return nil
}
func (s *State) catanTribePortBot(player int) (Action, error) {
	g := s.Catan
	t := g.tribe()
	if t == nil || t.Pending == nil || t.Pending.Player != player {
		return Action{}, errors.New("inactive port choice seat")
	}
	choices := []botChoice{}
	for slot, resource := range t.HeldPorts[player] {
		value := 0
		for _, v := range g.Vertices {
			if v.Owner != player || v.Level == 0 {
				continue
			}
			for _, tile := range g.Tiles {
				if tile.Number > 0 && slices.Contains(tile.Vertices, v.ID) && (resource < 0 || tile.Resource == resource) {
					value += (6 - absCatan(7-tile.Number)) * v.Level
				}
			}
		}
		for _, edge := range g.tribePortEdges(player) {
			choices = append(choices, botChoice{Action{Type: "catan_port", Slot: slot, Edge: edge}, value + g.harborPortValue(player, edge)})
		}
	}
	return s.botLegal(player, choices)
}
