package game

import (
	"errors"
	"slices"
)

// E&P + C&K (2025): knights stay on explored islands; ships and mission
// crew are separate inventories. Unlike settlements, knights need no distance
// rule. Do not accidentally treat an undiscovered gold/lair as explored land.
func (g *Catan) explorerKnightSite(vertex int) bool {
	if g.Explorer == nil {
		return true
	}
	if vertex < 0 || vertex >= len(g.Vertices) {
		return false
	}
	land := false
	for _, tile := range g.Tiles {
		if !slices.Contains(tile.Vertices, vertex) {
			continue
		}
		if tile.Resource == CatanFog {
			return false
		}
		land = land || tile.Resource >= 0 && tile.Resource <= CatanDesert || tile.Resource == CatanGold
	}
	return land
}

func (s *State) validateExplorerKnights() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	if k.ActionSerial != g.TurnSerial {
		return errors.New("组合骑士行动序号不一致")
	}
	seen := map[int]bool{}
	counts := make([][3]int, len(g.Players))
	check := func(n CatanKnight, retreat bool) error {
		if n.Owner < 0 || n.Owner >= len(g.Players) || g.Players[n.Owner].Eliminated || n.Strength < 1 || n.Strength > 3 || !g.explorerKnightSite(n.Vertex) || g.Vertices[n.Vertex].Level != 0 || n.ActivatedAt > k.ActionSerial || n.PromotedAt > k.ActionSerial {
			return errors.New("组合骑士位置、所有者、强度或行动记录无效")
		}
		if !retreat {
			if seen[n.Vertex] {
				return errors.New("组合交点不能重叠骑士")
			}
			seen[n.Vertex] = true
		}
		counts[n.Owner][n.Strength-1]++
		if counts[n.Owner][n.Strength-1] > 2 {
			return errors.New("组合每人每级骑士最多两枚")
		}
		return nil
	}
	for _, n := range k.Knights {
		if err := check(n, false); err != nil {
			return err
		}
	}
	if q := k.Pending; q != nil && q.Kind == "knight_retreat" {
		if q.Knight == nil || len(q.Players) != 1 || q.Players[0] != q.Knight.Owner || q.Knight.Owner == s.Turn {
			return errors.New("组合骑士撤退缺少对应玩家或棋子")
		}
		if err := check(*q.Knight, true); err != nil {
			return err
		}
		attacker := g.knightAt(q.Knight.Vertex)
		validDisplacement := q.Source == "" && attacker != nil && attacker.Owner == s.Turn && !attacker.Active && attacker.Strength > q.Knight.Strength
		if q.Source == "intrigue" && attacker == nil {
			for _, edge := range g.touching(q.Knight.Vertex) {
				validDisplacement = validDisplacement || g.Edges[edge].Owner == s.Turn && !g.Edges[edge].Ship
			}
		}
		if !validDisplacement || len(g.knightDestinations(*q.Knight, true)) == 0 {
			return errors.New("组合骑士撤退缺少合法驱逐或退路")
		}
	}
	return nil
}
