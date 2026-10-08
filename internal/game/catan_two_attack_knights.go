package game

import (
	"errors"
	"slices"
)

// This separate marker preserves ordinary two-player Attack's shared knight.
const CatanTwoAttackKnightsRules = "catan-for-two-attack-knights-site-v1"

func (g *Catan) twoAttackKnights() bool {
	return g.twoKnights() && g.attackKnights() && g.Attack.TwoRules == CatanTwoAttackKnightsRules
}
func (g *Catan) attackCityKnightOwner(owner int) bool {
	return owner >= 0 && owner < len(g.Players) && !g.Players[owner].Eliminated || g.twoAttackKnights() && g.twoNeutralKnightOwner(owner)
}
func (g *Catan) attackCityCanMove(owner, player int) bool {
	return owner == player || g.twoAttackKnights() && g.twoNeutralKnightOwner(owner)
}
func (g *Catan) attackCityNeutralLimit() int {
	if g.twoAttackKnights() {
		return 8
	}
	return 0
}
func (g *Catan) attackCityMoveLimit() int { return 6 + g.attackCityNeutralLimit() }
func (g *Catan) attackCityTreasonRemovable(owner, edge int) bool {
	c := g.Attack.City
	i := c.at(edge)
	if i < 0 || c.Knights[i].Owner != owner {
		return false
	}
	if g.twoAttackKnights() && g.twoNeutralKnightOwner(owner) {
		for _, k := range c.Knights {
			if k.Owner == owner && k.Strength < c.Knights[i].Strength {
				return false
			}
		}
	}
	return true
}
func (g *Catan) twoAttackKnightChoices(kind string) []catanTwoNeutralChoice {
	out := []catanTwoNeutralChoice{}
	if !g.twoAttackKnights() {
		return out
	}
	c := g.Attack.City
	for _, owner := range catanTwoNeutralOwners {
		if kind == "knight" {
			for _, edge := range c.recruitEdges(g, owner) {
				out = append(out, catanTwoNeutralChoice{Owner: owner, Vertex: -1, Edge: edge})
			}
		}
		if kind == "knight_promote" && c.count(owner, 2) < 2 {
			for _, k := range c.Knights {
				if k.Owner == owner && k.Strength == 1 && k.PromotedAt != g.CitiesKnights.ActionSerial {
					out = append(out, catanTwoNeutralChoice{Owner: owner, Vertex: -1, Edge: k.Edge})
				}
			}
		}
	}
	return out
}
func (s *State) catanTwoAttackKnightAfterAction(before *State, a Action) bool {
	g := s.Catan
	kinds := []string{}
	if a.Type == "catan_attack_knight_recruit" {
		kinds = append(kinds, "knight")
	}
	if a.Type == "catan_attack_knight_promote" || a.Type == "catan_progress" && a.Card == 8 {
		for _, k := range g.Attack.City.Knights {
			i := before.Catan.Attack.City.at(k.Edge)
			if k.Owner == before.Turn && k.Strength == 2 && i >= 0 {
				old := before.Catan.Attack.City.Knights[i]
				if old.Owner == k.Owner && old.Strength == 1 {
					kinds = append(kinds, "knight_promote")
				}
			}
		}
	}
	if len(kinds) == 0 {
		return false
	}
	s.catanTwoQueueBuild(kinds, s.Phase)
	return true
}
func (g *Catan) twoAttackKnightTokenEdges(player int) []int {
	out := []int{}
	if g.twoAttackKnights() {
		for _, k := range g.Attack.City.Knights {
			if k.Owner == player && g.Two.Bank >= k.Strength {
				out = append(out, k.Edge)
			}
		}
	}
	return out
}
func (s *State) catanTwoAttackKnightForTokens(player int, a Action) error {
	g := s.Catan
	if a.Choice != "" || g.Two.KnightExchanged || !slices.Contains(g.twoAttackKnightTokenEdges(player), a.Edge) {
		return errors.New("请选择自己的道路骑士，供应需足额，每回合一次")
	}
	c := g.Attack.City
	i := c.at(a.Edge)
	rank := c.Knights[i].Strength
	c.Knights = slices.Delete(c.Knights, i, i+1)
	if err := s.catanTwoEarn(player, rank); err != nil {
		return err
	}
	g.Two.KnightExchanged = true
	s.catanLog(player, "移除路线 #%d 的%d级骑士，换取 %d 枚贸易筹码", a.Edge+1, rank, rank)
	s.catanScores()
	s.catanVictory()
	return nil
}
