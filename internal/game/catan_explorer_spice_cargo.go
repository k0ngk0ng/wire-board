package game

import (
	"errors"
	"slices"
)

// Sacks are identical physical pieces. Origin and Owner retain the one-per-
// farm entitlement after delivery/loss; they do not make cargo private.
// Each farm allocates N previously unused IDs on discovery, at most 6*4=24.
type catanExplorerSpiceSack struct {
	Origin int                        `json:"origin"`
	Owner  int                        `json:"owner"`
	At     catanExplorerCargoLocation `json:"at"`
}

func catanExplorerFishScenario(s string) bool {
	return s == "fish-for-catan" || s == "spices-for-catan"
}
func catanExplorerPirateScenario(s string) bool {
	return catanExplorerMissionScenario(s) || s == "spices-for-catan"
}
func (c catanExplorerCargo) farmFriend(player, tile int) bool {
	return player >= 0 && slices.ContainsFunc(c.Spice, func(s catanExplorerSpiceSack) bool { return s.Origin == tile && s.Owner == player })
}
func (c catanExplorerCargo) landVertex(g *Catan, player, vertex int) bool {
	if !catanExplorerLandVertex(g, vertex) {
		return false
	}
	if c.Scenario == "spices-for-catan" {
		for _, t := range g.Tiles {
			if t.Resource == CatanDesert && slices.Contains(t.Vertices, vertex) && !c.farmFriend(player, t.ID) {
				return false
			}
		}
	}
	return true
}
func (c catanExplorerCargo) landEdge(g *Catan, player, edge int) bool {
	if !catanExplorerLandEdge(g, edge) {
		return false
	}
	if c.Scenario == "spices-for-catan" {
		for _, tile := range g.Edges[edge].Tiles {
			if g.Tiles[tile].Resource == CatanDesert && !c.farmFriend(player, tile) {
				return false
			}
		}
	}
	return true
}
func (c catanExplorerCargo) validateSpiceCargo(g *Catan, f *catanExplorerSailing) error {
	if c.Scenario != "spices-for-catan" {
		if len(c.Spice) != 0 {
			return errors.New("本剧本没有香料货物")
		}
		return nil
	}
	if len(c.Spice) != 24 || len(g.Players) > 4 {
		return errors.New("香料实体库存或人数无效")
	}
	counts := map[int]int{}
	claimed := map[[2]int]bool{}
	supply := catanExplorerCargoLocation{"supply", -1}
	for _, s := range c.Spice {
		if s.Origin == -1 {
			if s.Owner != -1 || s.At != supply {
				return errors.New("未分配香料必须留在供应区")
			}
			continue
		}
		if s.Origin < 0 || s.Origin >= len(g.Tiles) || g.Tiles[s.Origin].Resource != CatanDesert || s.Owner < -1 || s.Owner >= len(g.Players) {
			return errors.New("香料来源农场或所属玩家无效")
		}
		counts[s.Origin]++
		if s.Owner == -1 {
			if s.At != (catanExplorerCargoLocation{"farm", s.Origin}) {
				return errors.New("尚未领取的香料须留在原农场")
			}
			continue
		}
		key := [2]int{s.Origin, s.Owner}
		if claimed[key] {
			return errors.New("每位玩家每座农场只能领取一袋香料")
		}
		claimed[key] = true
		if g.Players[s.Owner].Eliminated {
			if s.At != supply {
				return errors.New("离场玩家不能保留香料货物")
			}
		} else {
			crews := 0
			for id := s.Owner*11 + 2; id < (s.Owner+1)*11; id++ {
				if c.Units[id] == (catanExplorerCargoLocation{"farm", s.Origin}) {
					crews++
				}
			}
			if crews != 1 {
				return errors.New("领取香料后必须永久派驻一名船员")
			}
		}
		if s.At != supply && (!c.holder(g, f, s.Owner, s.At) || c.used(s.At) > 2) {
			return errors.New("香料货物位置、归属或容量无效")
		}
	}
	for _, count := range counts {
		if count != len(g.Players) {
			return errors.New("发现农场应按开局人数放置香料")
		}
	}
	return nil
}

// Called inside the discovery transaction, after revealing the farm. The
// authoritative world still settles the two-gold discovery reward separately.
func (c *catanExplorerCargo) discoverSpice(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, tile int) error {
	if err := b.validate(g); err != nil {
		return err
	}
	if err := c.validate(g, f); err != nil {
		return err
	}
	if c.Scenario != "spices-for-catan" || b.Scenario != c.Scenario || !slices.ContainsFunc(b.Hidden, func(h catanExplorerHidden) bool { return h.Tile == tile && h.Revealed && h.Farm != "" }) || slices.ContainsFunc(c.Spice, func(s catanExplorerSpiceSack) bool { return s.Origin == tile }) {
		return errors.New("只能为新发现的农场放置一次香料")
	}
	next := clone(*c)
	remaining := len(g.Players)
	for id, s := range next.Spice {
		if s.Origin == -1 && remaining > 0 {
			next.Spice[id] = catanExplorerSpiceSack{tile, -1, catanExplorerCargoLocation{"farm", tile}}
			remaining--
		}
	}
	if remaining != 0 {
		return errors.New("香料组件不足")
	}
	if err := next.validate(g, f); err != nil {
		return err
	}
	*c = next
	return nil
}
func (c catanExplorerCargo) farmCount(b *catanExplorerBoard, player int, ability string) int {
	n := 0
	for _, h := range b.Hidden {
		if h.Revealed && h.Farm == ability && c.farmFriend(player, h.Tile) {
			n++
		}
	}
	return n
}
func (c catanExplorerCargo) pirateFarmDice(b *catanExplorerBoard, player int) []int {
	var out []int
	for _, h := range b.Hidden {
		if h.Revealed && h.Farm == "pirate" && c.farmFriend(player, h.Tile) {
			out = append(out, h.PirateDie)
		}
	}
	slices.Sort(out)
	return out
}
func (c catanExplorerCargo) spiceContents(at catanExplorerCargoLocation) []int {
	ids := []int{}
	for id, s := range c.Spice {
		if s.At == at {
			ids = append(ids, id)
		}
	}
	return ids
}
