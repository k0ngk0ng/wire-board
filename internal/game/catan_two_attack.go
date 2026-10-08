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
	if !g.twoAttack() {
		return out
	}
	for _, from := range g.Attack.captureTargets() {
		for _, to := range g.Attack.Map.Coast {
			if from != to && !g.Attack.conquered(to) {
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
	g.Attack.Barbarians[a.Card]--
	g.Attack.Barbarians[a.Tile]++
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
