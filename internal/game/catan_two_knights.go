package game

import (
	"errors"
	"fmt"
	"slices"
)

func catanKnightOwnerName(owner int) string {
	if owner < -1 {
		return catanTwoOwnerName(owner)
	}
	return fmt.Sprintf("玩家 %d", owner+1)
}

// Site supplement: the active human controls a neutral knight's mandatory
// retreat and breaks ties between equally weak neutral Treason targets.
func (g *Catan) knightResponseActor(owner, turn int) int {
	if g.twoNeutralKnightOwner(owner) {
		return turn
	}
	return owner
}

func (g *Catan) twoWeakestKnight(owner int) int {
	strength := 4
	for _, n := range g.CitiesKnights.Knights {
		if n.Owner == owner {
			strength = min(strength, n.Strength)
		}
	}
	return strength
}

const CatanTwoKnightsRules = "catan-for-two-knights-2025"

// NewCatanTwoCitiesKnights is a separate, versioned two-player recipe.
// Ordinary city rooms and their options retain their original player limits.
func NewCatanTwoCitiesKnights(n int, options CatanOptions) (*State, error) {
	if n != 2 || options != (CatanOptions{}) {
		return nil, errors.New("双人城市骑士只支持两位玩家及独立配方")
	}
	return newCatanTwoCitiesKnights()
}

func newCatanTwoCitiesKnights() (*State, error) {
	s, err := newCatanTwoCore()
	if err != nil {
		return nil, err
	}
	s.enableCitiesKnights()
	s.Catan.Two.Knights = CatanTwoKnightsRules
	s.Log = append(s.Log, "双人城市骑士：每回合完整进行两次骑士事件与生产；中立骑士不激活、不参与防御，贸易筹码供应有限")
	s.catanScores()
	return s, s.validateCatanTwo()
}

func (g *Catan) twoKnights() bool {
	return g.Two != nil && g.Two.Knights == CatanTwoKnightsRules && g.CitiesKnights != nil && len(g.Players) == 2
}

func (g *Catan) twoNeutralKnightOwner(owner int) bool {
	return g.twoKnights() && (owner == -2 || owner == -3)
}

func (g *Catan) twoKnightTokenVertices(player int) []int {
	out := []int{}
	if g.twoKnights() {
		for _, n := range g.CitiesKnights.Knights {
			if n.Owner == player && g.Two.Bank >= n.Strength {
				out = append(out, n.Vertex)
			}
		}
	}
	return out
}

func (s *State) catanTwoKnightForTokens(player int, a Action) error {
	g := s.Catan
	if a.Choice != "" || g.Two.KnightExchanged || !slices.Contains(g.twoKnightTokenVertices(player), a.Vertex) {
		return errors.New("请选择自己的骑士，且供应需足够支付；每回合一次")
	}
	n := *g.knightAt(a.Vertex)
	g.CitiesKnights.Knights = slices.DeleteFunc(g.CitiesKnights.Knights, func(piece CatanKnight) bool { return piece.Vertex == a.Vertex })
	if err := s.catanTwoEarn(player, n.Strength); err != nil {
		return err
	}
	g.Two.KnightExchanged = true
	s.catanLog(player, "移除交点 #%d 的%d级骑士，换取 %d 枚贸易筹码", a.Vertex+1, n.Strength, n.Strength)
	s.catanScores()
	s.catanVictory()
	return nil
}

func (g *Catan) twoKnightChoices(kind string) []catanTwoNeutralChoice {
	out := []catanTwoNeutralChoice{}
	if !g.twoKnights() {
		return out
	}
	for _, owner := range catanTwoNeutralOwners {
		for _, v := range g.Vertices {
			if kind == "knight" && g.knightRecruitable(owner, v.ID) {
				out = append(out, catanTwoNeutralChoice{Owner: owner, Vertex: v.ID, Edge: -1})
			}
			if n := g.knightAt(v.ID); kind == "knight_promote" && n != nil && n.Owner == owner && n.Strength == 1 && g.knightCanPromote(n) {
				out = append(out, catanTwoNeutralChoice{Owner: owner, Vertex: v.ID, Edge: -1})
			}
		}
	}
	return out
}

// Smithing can require two successive neutral promotions. Resolve one legal
// response at a time, and skip only when neither color can complete it.
func (s *State) catanTwoQueueBuild(kinds []string, resume string) {
	g, q := s.Catan, s.Catan.Two
	for i, kind := range kinds {
		if len(g.twoNeutralChoices(kind)) == 0 {
			continue
		}
		q.Sequence++
		q.Pending = &CatanTwoPending{Kind: kind, Resume: resume, Remaining: slices.Clone(kinds[i+1:])}
		g.Trade = nil
		s.Phase = "catan_two_build"
		s.catanLog(s.Turn, "请为一家中立势力完成额外建设")
		return
	}
}

func (s *State) catanTwoKnightAfterAction(before *State, a Action) bool {
	g := s.Catan
	if !g.twoKnights() {
		return false
	}
	kinds := []string{}
	if a.Type == "catan_knight_recruit" {
		kinds = append(kinds, "knight")
	}
	if a.Type == "catan_knight_promote" || a.Type == "catan_progress" && a.Card == 8 {
		for _, n := range g.CitiesKnights.Knights {
			old := before.Catan.knightAt(n.Vertex)
			if n.Owner == before.Turn && n.Strength == 2 && old != nil && old.Owner == n.Owner && old.Strength == 1 {
				kinds = append(kinds, "knight_promote")
			}
		}
	}
	if len(kinds) == 0 {
		return false
	}
	s.catanTwoQueueBuild(kinds, s.Phase)
	return true
}

func (s *State) validateTwoKnights() error {
	g, q := s.Catan, s.Catan.Two
	if q.Knights == "" && g.CitiesKnights == nil {
		return nil
	}
	if !g.twoKnights() || g.Seafarers != nil || g.Fishing != nil || g.Rivers != nil || g.Caravans != nil || g.Transport != nil || g.Attack != nil {
		return errors.New("双人城市骑士规则或组合无效")
	}
	k := g.CitiesKnights
	if k.Rules != CatanCitiesKnightsRules || len(k.Players) != 2 || s.Turn < 0 || s.Turn >= 2 || len(g.Vertices) != 54 || len(g.Bank) != 8 || k.ActionSerial == 0 || k.Invasions < 0 || k.BarbarianPosition < 0 || k.BarbarianPosition > 7 || len(g.DevDeck) != 0 || len(g.DevDiscard) != 0 || q.TokensIssued != 0 {
		return errors.New("双人城市骑士组件状态无效")
	}
	counts := map[int][3]int{}
	seen := map[int]bool{}
	check := func(n CatanKnight, board bool) error {
		if (n.Owner < 0 && !g.twoNeutralKnightOwner(n.Owner)) || n.Owner >= 2 || n.Strength < 1 || n.Strength > 3 || n.Vertex < 0 || n.Vertex >= len(g.Vertices) || n.PromotedAt > k.ActionSerial || n.ActivatedAt > k.ActionSerial {
			return errors.New("双人骑士身份或等级无效")
		}
		if n.Owner < 0 && (n.Strength > 2 || n.Active || n.ActivatedAt != 0) {
			return errors.New("中立骑士不能激活或升级三级")
		}
		if board {
			if seen[n.Vertex] || g.Vertices[n.Vertex].Level != 0 {
				return errors.New("骑士位置被占用")
			}
			seen[n.Vertex] = true
		}
		stock := counts[n.Owner]
		stock[n.Strength-1]++
		counts[n.Owner] = stock
		if stock[n.Strength-1] > 2 {
			return errors.New("骑士棋子超过库存")
		}
		return nil
	}
	for _, n := range k.Knights {
		if err := check(n, true); err != nil {
			return err
		}
	}
	if p := k.Pending; p != nil {
		if len(p.Players) == 0 || (q.Pending != nil || q.Trade != nil) {
			return errors.New("双人城市回应冲突")
		}
		for _, player := range p.Players {
			if player < 0 || player >= 2 {
				return errors.New("中立颜色不能作为真实响应玩家")
			}
		}
		if p.Kind == "knight_retreat" {
			if p.Knight == nil {
				return errors.New("缺少退让骑士")
			}
			if err := check(*p.Knight, false); err != nil {
				return err
			}
		}
	}
	return s.validateTwoCityFlow()
}
