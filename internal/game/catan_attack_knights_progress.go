package game

import (
	"errors"
	"slices"
)

// These kernels run after shared progress-card ownership and phase validation.
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
	if actor < 0 || actor >= len(g.Players) || !g.attackCityKnightOwner(owner) || actor == owner || g.Players[actor].Eliminated || i < 0 || c.Knights[i].Owner != owner {
		return nil, errors.New("叛变须由目标玩家移除己方骑士")
	}
	if !g.attackCityTreasonRemovable(owner, edge) {
		return nil, errors.New("中立势力须移除最低等级骑士")
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

func (s *State) catanAttackCityProgress(player int, a Action) error {
	g := s.Catan
	c := g.Attack.City
	k := g.CitiesKnights
	switch a.Card {
	case 3:
		if len(a.Tokens) > 0 && (len(a.Tokens) != 2 || a.Tokens[0] != 0 || a.Tokens[1] != 0) {
			return errors.New("蛮族地图只交换内陆主数字")
		}
		if err := s.catanAttackCityInvention(a.Tile, a.Target); err != nil {
			return err
		}
	case 8:
		if len(a.Targets) > 2 || len(a.Targets) == 2 && a.Targets[0] == a.Targets[1] {
			return errors.New("锻造最多升级两名不同道路骑士")
		}
		for _, edge := range a.Targets {
			i := c.at(edge)
			if i < 0 || c.Knights[i].Owner != player {
				return errors.New("请选择自己的道路骑士")
			}
			n := c.Knights[i]
			if n.Strength >= 3 || n.PromotedAt == k.ActionSerial || c.count(player, n.Strength+1) >= 2 || n.Strength == 2 && k.Players[player].Improvements[CatanPolitics] < 3 {
				return errors.New("锻造骑士等级、政治或库存不满足")
			}
			c.Knights[i].Strength++
			c.Knights[i].PromotedAt = k.ActionSerial
		}
	case 17:
		for i := range c.Knights {
			if c.Knights[i].Owner == player && !c.Knights[i].Active {
				c.Knights[i].Active = true
				c.Knights[i].ActivatedAt = k.ActionSerial
			}
		}
	case 19:
		if err := s.catanAttackCityIntrigue(player, a.Tile); err != nil {
			return err
		}
	case 21:
		if err := s.catanAttackCityTaxation(player, a.Tile, catanRandom); err != nil {
			return err
		}
	case 22:
		if !g.politicsOpponent(player, a.Target) && !(g.twoAttackKnights() && g.twoNeutralKnightOwner(a.Target)) {
			return errors.New("请选择拥有道路骑士的其他玩家")
		}
		found := false
		for _, n := range c.Knights {
			found = found || n.Owner == a.Target
		}
		if !found {
			return errors.New("目标没有道路骑士")
		}
		if c.Sequence == int(^uint(0)>>1) {
			return errors.New("回应序号超过上限")
		}
		c.Sequence++
		c.Treason = &catanAttackCityTreasonPending{ID: c.Sequence, Actor: player, Owner: a.Target}
		s.Phase = "catan_attack_city_treason_remove"
		return nil
	default:
		return errors.New("不是组合专用进步牌")
	}
	s.catanScores()
	s.catanVictory()
	return nil
}

type catanAttackCityTreasonPending struct {
	ID        int                     `json:"id"`
	Actor     int                     `json:"actor"`
	Owner     int                     `json:"owner"`
	Placement *catanAttackCityTreason `json:"placement,omitempty"`
}

func (s *State) validateAttackCityTreason() error {
	g := s.Catan
	if g == nil || !g.attackKnights() {
		return nil
	}
	c := g.Attack.City
	q := c.Treason
	remove := s.Phase == "catan_attack_city_treason_remove"
	place := s.Phase == "catan_attack_city_treason_place"
	if (q != nil) != (remove || place) {
		return errors.New("叛变回应与阶段不一致")
	}
	if q == nil {
		return nil
	}
	if q.ID < 1 || q.ID != c.Sequence || q.Actor != s.Turn || q.Actor < 0 || q.Actor >= len(g.Players) || !g.attackCityKnightOwner(q.Owner) || q.Owner == q.Actor || g.Players[q.Actor].Eliminated || s.Finished || g.setup() || c.Plan != nil || g.CitiesKnights.Pending != nil || g.CitiesKnights.Event != nil || g.Trade != nil {
		return errors.New("叛变玩家、序号或并行回应无效")
	}
	if q.Placement == nil {
		found := false
		for _, n := range c.Knights {
			found = found || n.Owner == q.Owner
		}
		if !remove || !found {
			return errors.New("叛变缺少可移除骑士")
		}
		return nil
	}
	p := q.Placement
	n := p.Removed
	if !place || p.Player != q.Actor || n.Owner != q.Owner || n.Edge < 0 || n.Edge >= len(g.Edges) || c.at(n.Edge) >= 0 || n.Strength < 1 || n.Strength > 3 || n.ActivatedAt > g.CitiesKnights.ActionSerial || n.PromotedAt > g.CitiesKnights.ActionSerial {
		return errors.New("叛变移除记录无效")
	}
	ranks := []int{}
	for rank := 1; rank <= n.Strength; rank++ {
		if c.count(q.Actor, rank) < 2 {
			ranks = append(ranks, rank)
		}
	}
	if len(ranks) == 0 || !slices.Equal(ranks, p.Ranks) || c.count(q.Owner, n.Strength) >= 2 {
		return errors.New("叛变待放置库存无效")
	}
	return nil
}
func (s *State) catanAttackCityTreasonAction(player int, a Action) error {
	if err := s.validateAttackCityTreason(); err != nil {
		return err
	}
	if s.Catan == nil || !s.Catan.attackKnights() || s.Catan.Attack.City.Treason == nil {
		return errors.New("当前没有叛变回应")
	}
	next := clone(*s)
	g := next.Catan
	c := g.Attack.City
	q := c.Treason
	if a.Type != "catan_attack_city_treason" || a.Prompt != q.ID || a.Skill != "" {
		return errors.New("叛变回应已过期")
	}
	if q.Placement == nil {
		if player != g.knightResponseActor(q.Owner, q.Actor) || a.Choice != "remove" {
			return errors.New("由目标玩家选择移除的道路骑士")
		}
		p, err := c.treasonRemove(g, q.Actor, q.Owner, a.Edge)
		if err != nil {
			return err
		}
		q.Placement = p
		if p != nil {
			next.Phase = "catan_attack_city_treason_place"
		} else {
			c.Treason = nil
			next.Phase = "catan_turn"
		}
	} else {
		if player != q.Actor {
			return errors.New("由使用叛变者选择替代骑士")
		}
		if a.Choice != "skip" {
			if a.Choice != "place" || a.Edge != q.Placement.Removed.Edge {
				return errors.New("必须在被移除骑士原边放置")
			}
			if err := c.treasonPlace(g, q.Placement, player, a.Color); err != nil {
				return err
			}
		}
		c.Treason = nil
		next.Phase = "catan_turn"
	}
	if c.Treason == nil {
		next.catanScores()
		next.catanVictory()
	}
	if err := c.validate(g); err != nil {
		return err
	}
	if err := next.validateAttackCityTreason(); err != nil {
		return err
	}
	*s = next
	return nil
}
