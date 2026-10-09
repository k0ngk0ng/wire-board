package game

import (
	"errors"
	"slices"
)

const CatanTwoAttackRules = "catan-for-two-barbarian-attack-2025"
const catanAttackNeutral = -2

func (g *Catan) twoAttack() bool {
	return g.Two != nil && g.Two.Rules == CatanTwoRules && len(g.Players) == 2 && g.Attack != nil && g.Attack.TwoRules == CatanTwoAttackRules
}
func NewCatanTwoAttack(n int, options CatanOptions) (*State, error) {
	if n != 2 || options != (CatanOptions{}) {
		return nil, errors.New("双人蛮族进攻需要两位玩家，其他组合尚未接通")
	}
	return newCatanAttackState(n, options)
}
func (s *State) validateTwoAttack() error {
	g := s.Catan
	if g.Attack == nil {
		return nil
	}
	a := g.Attack
	if g.twoAttackKnights() {
		if a.NeutralPrisoners != 0 || a.TwoLanding && (g.Two.Pending == nil || g.Two.Pending.Kind != "settlement" || s.Phase != "catan_two_build") {
			return errors.New("双人道路骑士登陆回应无效")
		}
		return nil
	}
	if !g.twoAttack() || g.Two.KnightExchanged || a.NeutralPrisoners < 0 || a.NeutralPrisoners > a.Map.Barbarians {
		return errors.New("双人蛮族规则或中立俘虏无效")
	}
	neutral := 0
	for _, k := range a.Knights {
		if k.Player == catanAttackNeutral {
			neutral++
		}
	}
	if neutral > 1 || (neutral == 0 && len(a.Knights) > 0 && (a.Pending == nil || !a.Pending.Neutral)) || a.Pending != nil && a.Pending.Neutral && (neutral != 0 || len(a.Knights) != 1) {
		return errors.New("首名骑士的中立骑士接续无效")
	}
	if a.TwoLanding && (g.Two.Pending == nil || g.Two.Pending.Kind != "settlement" || s.Phase != "catan_two_build") {
		return errors.New("双建筑登陆缺少中立建设回应")
	}
	return nil
}
func (s *State) catanTwoAttackLandings(neutralVillage bool) error {
	a := s.Catan.Attack
	a.TwoLanding = false
	count := 1
	if neutralVillage {
		count++
	}
	if s.Catan.attackWonders() {
		return s.startWonderLandings(count, func() [2]int { return [2]int{catanRandom(6) + 1, catanRandom(6) + 1} })
	}
	for i := 0; i < count && !s.Finished; i++ {
		if err := s.catanAttackLanding(func() [2]int { return [2]int{catanRandom(6) + 1, catanRandom(6) + 1} }, catanRandom); err != nil {
			return err
		}
	}
	return nil
}
func (g *Catan) attackRecruitOwner(player int) int {
	if g.twoAttack() && g.Attack.Pending != nil && g.Attack.Pending.Neutral {
		return catanAttackNeutral
	}
	return player
}
func (g *Catan) attackCanMoveKnight(owner, player int) bool {
	return owner == player || g.twoAttack() && owner == catanAttackNeutral
}
func (g *Catan) attackBattleSeats() int {
	if g.twoAttack() {
		return 3
	}
	return len(g.Players)
}
func (g *Catan) attackBattleSeat(owner int) int {
	if g.twoAttack() && owner == catanAttackNeutral {
		return 2
	}
	return owner
}
func (g *Catan) twoAttackMoves() map[int][]int {
	out := map[int][]int{}
	if !g.twoAttack() && !g.twoAttackKnights() {
		return out
	}
	for _, from := range g.attackCaptureTargets() {
		for _, to := range g.attackBattleTiles() {
			if from != to && !g.Attack.conquered(to) {
				if g.attackTransport() {
					legal := false
					for id, piece := range g.AttackTransport.Pieces.Barbarians {
						if piece.Tile != from {
							continue
						}
						for _, edge := range g.AttackTransport.Pieces.edges(g, to, id) {
							next := catanAttackTransportPieces{Barbarians: slices.Clone(g.AttackTransport.Pieces.Barbarians)}
							if next.relocate(g, g.attackTransportBoard(), id, to, edge) == nil {
								legal = true
								break
							}
						}
						break
					}
					if !legal {
						continue
					}
				}
				out[from] = append(out[from], to)
			}
		}
	}
	return out
}
func (s *State) catanTwoMoveBarbarian(player int, a Action, cost int) error {
	g := s.Catan
	if !slices.Contains(g.twoAttackMoves()[a.Card], a.Tile) {
		return errors.New("请选择有蛮族的沿海来源和另一未征服沿海目的地")
	}
	if g.attackTransport() {
		id := -1
		for i, p := range g.AttackTransport.Pieces.Barbarians {
			if p.Tile == a.Card {
				id = i
				break
			}
		}
		if id < 0 {
			return errors.New("来源没有蛮族")
		}
		edges := g.AttackTransport.Pieces.edges(g, a.Tile, id)
		if len(edges) == 0 {
			return errors.New("目的地没有空边")
		}
		if err := g.AttackTransport.Pieces.relocate(g, g.attackTransportBoard(), id, a.Tile, edges[0]); err != nil {
			return err
		}
		g.syncAttackTransportCounts()
	} else {
		g.Attack.Barbarians[a.Card]--
		g.Attack.Barbarians[a.Tile]++
	}
	if g.twoAttackKnights() {
		if m := g.CitiesKnights.Merchant; m != nil && g.Attack.conquered(m.Tile) {
			g.CitiesKnights.Merchant = nil
		}
	}
	s.catanLog(player, "花费%d枚贸易筹码，将1个蛮族从地块 #%d 移至 #%d，不偷牌", cost, a.Card+1, a.Tile+1)
	s.catanScores()
	s.catanVictory()
	return nil
}

func (g *Catan) attackMoveLimit() int {
	if g.twoAttack() {
		return 7
	}
	return 6
}
