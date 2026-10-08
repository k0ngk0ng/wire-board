package game

import (
	"errors"
	"slices"
)

// These kernels are called after progress-card ownership/phase validation by
// the future combined action controller, not from ordinary city-card dispatch.
func (s *State) catanAttackCityIntrigue(player, tile int) error {
	g := s.Catan
	if !g.attackKnights() || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || !slices.Contains(g.Attack.captureTargets(), tile) {
		return errors.New("请选择一个有蛮族的沿海地块")
	}
	g.Attack.Barbarians[tile]--
	g.Attack.Prisoners[player]++
	return nil
}
func (s *State) catanAttackCityTaxation(player, tile int, random func(int) int) error {
	g := s.Catan
	if !g.attackKnights() || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || tile < 0 || tile >= len(g.Tiles) || random == nil {
		return errors.New("请选择征税地块")
	}
	// Validate all random picks before committing any transfer.
	next := clone(*s)
	g = next.Catan
	seen := map[int]bool{}
	for _, id := range g.Tiles[tile].Vertices {
		v := g.Vertices[id]
		other := v.Owner
		if v.Level == 0 || other < 0 || other >= len(g.Players) || other == player || seen[other] || g.Players[other].Eliminated || g.Attack.conqueredBuilding(g, id) && !g.attackCityMetropolis(id) {
			continue
		}
		seen[other] = true
		hand := g.Players[other].Resources
		if sum(hand) == 0 {
			continue
		}
		pick := random(sum(hand))
		if pick < 0 || pick >= sum(hand) {
			return errors.New("征税随机牌选择无效")
		}
		for color, count := range hand {
			if pick < count {
				hand[color]--
				g.Players[player].Resources[color]++
				break
			}
			pick -= count
		}
	}
	*s = next
	return nil
}

type catanAttackCityTreason struct {
	Player  int                   `json:"player"`
	Removed catanAttackCityKnight `json:"removed"`
	Ranks   []int                 `json:"ranks"`
}

func (c *catanAttackCity) treasonRemove(g *Catan, actor, owner, edge int) (*catanAttackCityTreason, error) {
	i := c.at(edge)
	if actor < 0 || actor >= len(g.Players) || owner < 0 || owner >= len(g.Players) || actor == owner || g.Players[actor].Eliminated || g.Players[owner].Eliminated || i < 0 || c.Knights[i].Owner != owner {
		return nil, errors.New("叛变须由目标玩家移除己方骑士")
	}
	old := c.Knights[i]
	c.Knights = slices.Delete(c.Knights, i, i+1)
	ranks := []int{}
	for rank := 1; rank <= old.Strength; rank++ {
		if c.count(actor, rank) < 2 {
			ranks = append(ranks, rank)
		}
	}
	if len(ranks) == 0 {
		return nil, nil
	}
	return &catanAttackCityTreason{Player: actor, Removed: old, Ranks: ranks}, nil
}
func (c *catanAttackCity) treasonPlace(g *Catan, q *catanAttackCityTreason, player, rank int) error {
	if q == nil || q.Player != player || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || !slices.Contains(q.Ranks, rank) || rank < 1 || rank > q.Removed.Strength || c.count(player, rank) >= 2 || q.Removed.Edge < 0 || q.Removed.Edge >= len(g.Edges) || c.at(q.Removed.Edge) >= 0 {
		return errors.New("请选择有库存的同级或低级骑士，在被移除骑士原道路放置")
	}
	c.Knights = append(c.Knights, catanAttackCityKnight{Owner: player, Edge: q.Removed.Edge, Strength: rank, Active: q.Removed.Active})
	return nil
}

func (s *State) catanAttackCityInvention(left, right int) error {
	g := s.Catan
	if !g.attackKnights() || g.setup() || left == right || !slices.Contains(g.attackCityInventionTiles(), left) || !slices.Contains(g.attackCityInventionTiles(), right) {
		return errors.New("发明只能交换内陆的两枚合适数字圆片")
	}
	before := [2]int{g.Tiles[left].Number, g.Tiles[right].Number}
	g.Attack.City.NumberSwaps = append(g.Attack.City.NumberSwaps, CatanNumberSwap{Left: CatanNumberToken{Tile: left}, Right: CatanNumberToken{Tile: right}, Before: before})
	g.Tiles[left].Number, g.Tiles[right].Number = before[1], before[0]
	return nil
}
func (c *catanAttackCity) validateNumbers(g *Catan) error {
	numbers := map[CatanNumberToken]int{}
	for _, t := range g.Tiles {
		numbers[CatanNumberToken{Tile: t.ID}] = t.Number
	}
	for _, swap := range c.NumberSwaps {
		if swap.Left.Slot != 0 || swap.Right.Slot != 0 || slices.Contains(g.Attack.Map.Coast, swap.Left.Tile) || slices.Contains(g.Attack.Map.Coast, swap.Right.Tile) {
			return errors.New("不能交换沿海数字")
		}
	}
	original, err := rewindCatanInvention(g, numbers, c.NumberSwaps)
	if err != nil {
		return err
	}
	want := catanAttackBoardRecipe(len(g.Players) > 4).numbers
	if len(g.Tiles) != len(want) {
		return errors.New("组合地图数字数量错误")
	}
	for id, n := range want {
		if original[CatanNumberToken{Tile: id}] != n {
			return errors.New("组合数字交换无法恢复官方配方")
		}
	}
	return nil
}
