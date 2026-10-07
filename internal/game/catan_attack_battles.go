package game

import (
	"errors"
	"slices"
)

// Orders refer to the knight's edge at the start of the end phase. This keeps
// identity stable when another knight subsequently occupies a vacated edge.
type catanAttackMove struct {
	From  int  `json:"from"`
	To    int  `json:"to"`
	Wheat bool `json:"wheat,omitempty"`
}
type catanAttackContestRoll struct {
	Players []int `json:"players"`
	Dice    []int `json:"dice"`
}
type catanAttackBattle struct {
	Tile       int                      `json:"tile"`
	Barbarians int                      `json:"barbarians"`
	Knights    []catanAttackKnight      `json:"knights"`
	Prisoners  []int                    `json:"prisoners"`
	Gold       []int                    `json:"gold"`
	Contests   []catanAttackContestRoll `json:"contests,omitempty"`
	LossDie    int                      `json:"lossDie,omitempty"`
	Lost       []catanAttackKnight      `json:"lost,omitempty"`
}
type catanAttackEndRecord struct {
	ID      int                 `json:"id"`
	Player  int                 `json:"player"`
	Moves   []catanAttackMove   `json:"moves"`
	Battles []catanAttackBattle `json:"battles"`
}

// Transactional end-phase kernel, shared by confirmed plans and internal tests.
func (s *State) catanAttackResolveEnd(moves []catanAttackMove, die func() int) error {
	if s.Catan == nil || s.Catan.Attack == nil || s.Phase != "catan_turn" || s.Finished || s.Turn < 0 || s.Turn >= len(s.Catan.Players) || s.Catan.Players[s.Turn].Eliminated || die == nil {
		return errors.New("当前不能结算蛮族进攻回合末阶段")
	}
	if err := s.validateCatanAttack(); err != nil {
		return err
	}
	next := clone(*s)
	if err := next.catanAttackEndStep(moves, die); err != nil {
		return err
	}
	if err := next.validateCatanAttack(); err != nil {
		return err
	}
	*s = next
	return nil
}

func (s *State) catanAttackEndStep(moves []catanAttackMove, die func() int) error {
	g, a := s.Catan, s.Catan.Attack
	a.EndSequence++
	a.End = &catanAttackEndRecord{ID: a.EndSequence, Player: s.Turn, Moves: slices.Clone(moves), Battles: []catanAttackBattle{}}
	g.Trade = nil
	if err := s.catanAttackMoveKnights(moves, true); err != nil {
		return err
	}
	for _, tile := range a.Map.Coast {
		count := a.Barbarians[tile]
		if count == 0 {
			continue
		}
		battle := catanAttackBattle{Tile: tile, Barbarians: count, Prisoners: make([]int, len(g.Players)), Gold: make([]int, len(g.Players))}
		strength := make([]int, len(g.Players))
		for _, k := range a.Knights {
			if !g.Players[k.Player].Eliminated && slices.Contains(g.Edges[k.Edge].Tiles, tile) {
				battle.Knights = append(battle.Knights, k)
				strength[k.Player]++
			}
		}
		if len(battle.Knights) <= count {
			continue
		}
		if err := battle.distribute(strength, die); err != nil {
			return err
		}
		a.Barbarians[tile] = 0
		for p, n := range battle.Prisoners {
			a.Prisoners[p] += n
		}
		s.catanScores()
		s.catanVictory()
		// Victory on one's own turn is immediate, including restored buildings.
		// No additional casualty die or later coastal battle is required after it.
		if !s.Finished {
			battle.LossDie = die()
			orientation := catanAttackLossOrientation(battle.LossDie)
			if orientation < 0 {
				return errors.New("骑士损失骰子无效")
			}
			for _, k := range battle.Knights {
				if a.Map.edgeOrientation(g, k.Edge) == orientation {
					battle.Lost = append(battle.Lost, k)
					battle.Gold[k.Player] += 3
				}
			}
			a.Knights = slices.DeleteFunc(a.Knights, func(k catanAttackKnight) bool { return slices.Contains(battle.Lost, k) })
		}
		if err := a.ensureGold(sum(battle.Gold)); err != nil {
			return err
		}
		for p, n := range battle.Gold {
			a.Gold[p] += n
			a.GoldBank -= n
		}
		a.End.Battles = append(a.End.Battles, battle)
		s.catanLog(a.End.Player, "地块 #%d 战斗胜利：%d名骑士击退%d个蛮族，恢复生产及被征服建筑", tile+1, len(battle.Knights), count)
		for p, n := range battle.Prisoners {
			if n > 0 {
				s.catanLog(p, "获得%d个俘虏，现有%d个（%d分）", n, a.Prisoners[p], a.Prisoners[p]/2)
			}
		}
		for p, n := range battle.Gold {
			if n > 0 {
				s.catanLog(p, "战斗补偿：金币×%d", n)
			}
		}
		if battle.LossDie > 0 {
			s.catanLog(a.End.Player, "损失骰掷出%d，%d名参战骑士返回供应", battle.LossDie, len(battle.Lost))
		}
		if s.Finished {
			return nil
		}
	}
	s.catanNext()
	s.catanVictory()
	return nil
}

func (b *catanAttackBattle) distribute(strength []int, die func() int) error {
	players := []int{}
	for p, n := range strength {
		if n > 0 {
			players = append(players, p)
		}
	}
	if len(players) == 1 {
		b.Prisoners[players[0]] = b.Barbarians
		return nil
	}
	if b.Barbarians >= len(players) {
		for _, p := range players {
			b.Prisoners[p] = 1
		}
		if b.Barbarians == len(players) {
			return nil
		}
		largest := 0
		leaders := []int{}
		for _, p := range players {
			if strength[p] > largest {
				largest = strength[p]
				leaders = []int{p}
			} else if strength[p] == largest {
				leaders = append(leaders, p)
			}
		}
		if len(leaders) == 1 {
			b.Prisoners[leaders[0]]++
			return nil
		}
		winners, err := b.contest(leaders, 1, die)
		if err != nil {
			return err
		}
		for _, p := range leaders {
			if slices.Contains(winners, p) {
				b.Prisoners[p]++
			} else {
				b.Gold[p] += 3
			}
		}
		return nil
	}
	winners, err := b.contest(players, b.Barbarians, die)
	if err != nil {
		return err
	}
	for _, p := range players {
		if slices.Contains(winners, p) {
			b.Prisoners[p]++
		} else {
			b.Gold[p] += 3
		}
	}
	return nil
}

// Only ties crossing the last available prisoner need another roll. Winners
// and losers already separated by earlier rolls never lose their ranking.
// The two-player rule calls this a tie-breaking die (neutral result 3).
func (b *catanAttackBattle) contest(players []int, places int, die func() int) ([]int, error) {
	winners := []int{}
	for len(winners) < places {
		if len(b.Contests) >= 256 {
			return nil, errors.New("分配俘虏连续同点过多，未提交本次结算")
		}
		roll := catanAttackContestRoll{Players: slices.Clone(players), Dice: make([]int, len(players))}
		groups := [7][]int{}
		for i, p := range players {
			v := die()
			if v < 1 || v > 6 {
				return nil, errors.New("俘虏分配骰子无效")
			}
			roll.Dice[i] = v
			groups[v] = append(groups[v], p)
		}
		b.Contests = append(b.Contests, roll)
		for v := 6; v >= 1; v-- {
			if len(groups[v]) <= places-len(winners) {
				winners = append(winners, groups[v]...)
			} else {
				players = groups[v]
				break
			}
			if len(winners) == places {
				return winners, nil
			}
		}
	}
	return winners, nil
}

func (s *State) validateCatanAttackEnd() error {
	g, a := s.Catan, s.Catan.Attack
	if a.EndSequence < 0 || (a.End == nil) != (a.EndSequence == 0) {
		return errors.New("回合末战斗记录序号无效")
	}
	q := a.End
	if q == nil {
		return nil
	}
	if g.setup() || q.ID != a.EndSequence || q.Player < 0 || q.Player >= len(g.Players) || len(q.Moves) > 6 || len(q.Battles) > len(a.Map.Coast) {
		return errors.New("回合末战斗记录无效")
	}
	used := map[int]bool{}
	for _, m := range q.Moves {
		if m.From < 0 || m.From >= len(g.Edges) || m.To < 0 || m.To >= len(g.Edges) || m.From == m.To || used[m.From] || a.castleEdge(g, m.To) {
			return errors.New("骑士移动记录无效")
		}
		used[m.From] = true
	}
	previous := -1
	for i, b := range q.Battles {
		coast := slices.Index(a.Map.Coast, b.Tile)
		if coast <= previous || b.Barbarians < 1 || b.Barbarians > 3 || len(b.Knights) <= b.Barbarians || len(b.Knights) > 6 || len(b.Prisoners) != len(g.Players) || len(b.Gold) != len(g.Players) || sum(b.Prisoners) != b.Barbarians || b.LossDie < 0 || b.LossDie > 6 || b.LossDie == 0 && (!s.Finished || i != len(q.Battles)-1) {
			return errors.New("战斗顺序、力量或俘虏记录无效")
		}
		previous = coast
		used = map[int]bool{}
		involved := map[int]bool{}
		for _, k := range b.Knights {
			if k.Player < 0 || k.Player >= len(g.Players) || k.Edge < 0 || k.Edge >= len(g.Edges) || used[k.Edge] || !slices.Contains(g.Edges[k.Edge].Tiles, b.Tile) {
				return errors.New("参战骑士记录无效")
			}
			used[k.Edge] = true
			involved[k.Player] = true
		}
		used = map[int]bool{}
		for _, k := range b.Lost {
			if used[k.Edge] || !slices.Contains(b.Knights, k) || b.LossDie == 0 || a.Map.edgeOrientation(g, k.Edge) != catanAttackLossOrientation(b.LossDie) {
				return errors.New("骑士损失记录无效")
			}
			used[k.Edge] = true
		}
		for p, n := range b.Prisoners {
			if n < 0 || b.Gold[p] < 0 || b.Gold[p]%3 != 0 || (n > 0 || b.Gold[p] > 0) && !involved[p] {
				return errors.New("俘虏或金币奖励记录无效")
			}
		}
		if len(b.Contests) > 256 {
			return errors.New("俘虏掷骰记录过长")
		}
		for _, r := range b.Contests {
			if len(r.Players) < 2 || len(r.Players) > len(involved) || len(r.Dice) != len(r.Players) {
				return errors.New("俘虏掷骰记录无效")
			}
			for j, p := range r.Players {
				if !involved[p] || slices.Contains(r.Players[:j], p) || r.Dice[j] < 1 || r.Dice[j] > 6 {
					return errors.New("俘虏掷骰参与者或点数无效")
				}
			}
		}
	}
	return nil
}

// Replayed on a clone for drafts; only confirmation changes public pieces.
func (s *State) catanAttackMoveKnights(moves []catanAttackMove, requireDeparture bool) error {
	g, a := s.Catan, s.Catan.Attack
	originals := map[int]int{}
	for i, k := range a.Knights {
		originals[k.Edge] = i
	}
	moved := map[int]bool{}
	for _, move := range moves {
		i, ok := originals[move.From]
		if !ok || moved[i] || a.Knights[i].Player != s.Turn || move.From == move.To {
			return errors.New("每名己方骑士只能移动一次，请使用其阶段开始时的位置")
		}
		steps := 3
		if move.Wheat {
			steps = 5
			if g.Players[s.Turn].Resources[3] < 1 {
				return errors.New("延长骑士移动需要独立支付1张粮食")
			}
		}
		if _, ok := a.knightDestinations(g, i, steps)[move.To]; !ok {
			return errors.New("骑士目的地超出移动距离、已被占据或属于城堡")
		}
		if move.Wheat {
			g.Players[s.Turn].Resources[3]--
			g.Bank[3]++
		}
		a.Knights[i].Edge = move.To
		moved[i] = true
		s.catanLog(s.Turn, "骑士从路线 #%d 移至 #%d（最多%d步）", move.From+1, move.To+1, steps)
	}
	for _, k := range a.Knights {
		if requireDeparture && k.Player == s.Turn && a.castleEdge(g, k.Edge) {
			return errors.New("必须先将自己的所有城堡骑士移出")
		}
	}
	return nil
}
