package game

import (
	"errors"
	"slices"
)

// Private mission controller. The six printed token numbers are an explicit
// verified-component input, not guessed constants; no public recipe uses this
// constructor until the full 2025 token inventory has been checked.
type catanExplorerLairs struct {
	Inventory []int                    `json:"inventory"`
	Deck      []int                    `json:"deck"`
	Sites     []catanExplorerLair      `json:"sites"`
	Progress  []int                    `json:"progress"`
	Arrival   []uint64                 `json:"arrival"` // Physical stack: earlier arrival is lower.
	Serial    uint64                   `json:"serial"`
	Battle    *catanExplorerLairBattle `json:"battle,omitempty"`
}
type catanExplorerLair struct {
	Tile          int     `json:"tile"`
	Number        int     `json:"number"` // Secret until resolved.
	Ready         uint64  `json:"ready,omitempty"`
	Resolved      uint64  `json:"resolved,omitempty"`
	Captor        int     `json:"captor"`
	Hero          int     `json:"hero"`
	Contributions []int   `json:"contributions,omitempty"`
	Rounds        [][]int `json:"rounds,omitempty"`
}
type catanExplorerLairBattle struct {
	Tile       int    `json:"tile"`
	Player     int    `json:"player"`
	Sequence   uint64 `json:"sequence"`
	Candidates []int  `json:"candidates"`
}

var catanExplorerLairPoints = [...]int{0, 1, 1, 2, 2, 2, 3, 3} // 2025 rulebook p4.

func newCatanExplorerLairs(players int, numbers []int) (*catanExplorerLairs, error) {
	if players < 2 || players > 4 || len(numbers) != 6 || slices.ContainsFunc(numbers, func(n int) bool { return n < 2 || n > 12 || n == 7 }) {
		return nil, errors.New("巢穴需要二至四人及六枚已核验数字")
	}
	return &catanExplorerLairs{Inventory: slices.Clone(numbers), Deck: slices.Clone(numbers), Sites: []catanExplorerLair{}, Progress: make([]int, players), Arrival: make([]uint64, players)}, nil
}
func (l catanExplorerLairs) site(tile int) int {
	return slices.IndexFunc(l.Sites, func(s catanExplorerLair) bool { return s.Tile == tile })
}
func (l catanExplorerLairs) leader() int {
	best := -1
	for p, step := range l.Progress {
		if step > 0 && (best < 0 || step > l.Progress[best] || step == l.Progress[best] && l.Arrival[p] < l.Arrival[best]) {
			best = p
		}
	}
	return best
}
func (l catanExplorerLairs) scores() []int {
	result := make([]int, len(l.Progress))
	leader := l.leader()
	for p, step := range l.Progress {
		result[p] = catanExplorerLairPoints[step]
		if p == leader {
			result[p]++
		}
	}
	return result
}
func (l *catanExplorerLairs) advance(player int) {
	// A marker at the last printed space stays at the bottom of its existing
	// stack; another success cannot lift it above later arrivals.
	if l.Progress[player] == len(catanExplorerLairPoints)-1 {
		return
	}
	l.Progress[player]++
	l.Serial++
	l.Arrival[player] = l.Serial
}
func (l catanExplorerLairs) validate(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy) error {
	if g == nil || b == nil || f == nil || c == nil || e == nil || b.Scenario != "pirate-lairs" || c.Scenario != b.Scenario || len(l.Inventory) != 6 || len(l.Deck)+len(l.Sites) != 6 || len(l.Progress) != len(g.Players) || len(l.Arrival) != len(l.Progress) {
		return errors.New("巢穴地图、组件或人数无效")
	}
	if err := b.validate(g); err != nil {
		return err
	}
	if err := e.validate(g, f, c); err != nil {
		return err
	}
	all := slices.Clone(l.Deck)
	tiles := []int{}
	seenArrival := map[uint64]bool{}
	for p, step := range l.Progress {
		if step < 0 || step >= len(catanExplorerLairPoints) || (step == 0) != (l.Arrival[p] == 0) || l.Arrival[p] > l.Serial || l.Arrival[p] > 0 && seenArrival[l.Arrival[p]] {
			return errors.New("巢穴任务轨道或叠放顺序无效")
		}
		if l.Arrival[p] > 0 {
			seenArrival[l.Arrival[p]] = true
		}
	}
	for _, s := range l.Sites {
		if s.Tile < 0 || s.Tile >= len(g.Tiles) || slices.Contains(tiles, s.Tile) || g.Tiles[s.Tile].Resource != CatanGold || s.Number < 2 || s.Number > 12 || s.Number == 7 {
			return errors.New("巢穴地形、数字或唯一性无效")
		}
		tiles = append(tiles, s.Tile)
		all = append(all, s.Number)
		ids := c.contents(catanExplorerCargoLocation{"lair", s.Tile})
		if s.Resolved == 0 {
			if b.Liberated[s.Tile] != 0 || g.Tiles[s.Tile].Number != 0 || (len(ids) == 3) != (s.Ready > 0) || s.Hero != -1 {
				return errors.New("未结算巢穴不能产金或提前获胜")
			}
		} else if s.Ready != s.Resolved || s.Hero < 0 || s.Hero >= len(g.Players) || len(ids) > 2 || b.Liberated[s.Tile] != s.Number || g.Tiles[s.Tile].Number != s.Number {
			return errors.New("已解放巢穴或英雄记录无效")
		}
		if s.Ready > 0 && (e.Turn == nil || s.Ready > e.Turn.Sequence) || s.Resolved > 0 && (len(s.Contributions) != len(g.Players) || sum(s.Contributions) != 3 || s.Contributions[s.Hero] == 0) {
			return errors.New("巢穴结算回合或参战人数无效")
		}
	}
	if !catanExplorerSameInventory(all, l.Inventory) || slices.ContainsFunc(l.Inventory, func(n int) bool { return n < 2 || n > 12 || n == 7 }) {
		return errors.New("六枚巢穴数字不守恒")
	}
	for tile := range b.Liberated {
		i := l.site(tile)
		if i < 0 || l.Sites[i].Resolved == 0 {
			return errors.New("金矿没有对应已结算巢穴")
		}
	}
	for _, loc := range c.Units {
		if loc.Kind == "lair" && l.site(loc.Index) < 0 {
			return errors.New("船员没有对应巢穴")
		}
	}
	if battle := l.Battle; battle != nil {
		i := l.site(battle.Tile)
		if i < 0 || e.Turn == nil || battle.Player != e.Turn.Player || battle.Sequence != e.Turn.Sequence || c.Turn == nil || c.Turn.Phase != "ended" || l.Sites[i].Ready != battle.Sequence || l.Sites[i].Resolved != 0 || len(battle.Candidates) < 2 || len(l.Sites[i].Contributions) != len(g.Players) {
			return errors.New("巢穴英雄待定阶段无效")
		}
		for j, p := range battle.Candidates {
			if p < 0 || p >= len(g.Players) || slices.Contains(battle.Candidates[:j], p) || l.Sites[i].Contributions[p] == 0 {
				return errors.New("英雄重掷玩家无效")
			}
		}
	}
	return l.validateHistory(c)
}

// Called by the authoritative discovery transaction immediately after a real
// gold-field reveal; rewards are still settled by that discovery controller.
func (l *catanExplorerLairs) discover(g *Catan, b *catanExplorerBoard, tile int) error {
	if err := b.validate(g); err != nil {
		return err
	}
	if tile < 0 || tile >= len(g.Tiles) || g.Tiles[tile].Resource != CatanGold || g.Tiles[tile].Number != 0 || l.site(tile) >= 0 || len(l.Deck) == 0 {
		return errors.New("此金矿不能重复放置巢穴")
	}
	l.Sites = append(l.Sites, catanExplorerLair{Tile: tile, Number: l.Deck[0], Hero: -1, Captor: -1})
	l.Deck = l.Deck[1:]
	return nil
}

// land/pickup are movement actions; begin/roll are end-of-movement responses.
// Random dice are provided by the trusted dispatcher, never by client input.
func (l *catanExplorerLairs) apply(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy, player int, sequence uint64, kind string, tile, ship int, units []int, randN func(int) int) error {
	if err := l.validate(g, b, f, c, e); err != nil {
		return err
	}
	if e.Turn == nil || e.Turn.Player != player || e.Turn.Sequence != sequence || e.Turn.Phase != "ready" || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return errors.New("巢穴行动玩家或回合序号无效")
	}
	base := *g
	base.Explorer = nil
	ng, nb, nf, nc, ne, nl := clone(base), clone(*b), clone(*f), clone(*c), clone(*e), clone(*l)
	if err := nl.applyUnchecked(&ng, &nb, &nf, &nc, &ne, player, sequence, kind, tile, ship, units, randN); err != nil {
		return err
	}
	if err := nl.validate(&ng, &nb, &nf, &nc, &ne); err != nil {
		return err
	}
	ng.Explorer = g.Explorer
	*g, *b, *f, *c, *e, *l = ng, nb, nf, nc, ne, nl
	return nil
}
func (l *catanExplorerLairs) applyUnchecked(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy, player int, sequence uint64, kind string, tile, ship int, units []int, randN func(int) int) error {
	i := l.site(tile)
	if i < 0 {
		return errors.New("请选择已探索的巢穴")
	}
	s := &l.Sites[i]
	switch kind {
	case "land", "pickup":
		if l.Battle != nil {
			return errors.New("先完成巢穴战斗")
		}
		if err := c.allowed(g, f, player, sequence, "movement"); err != nil {
			return err
		}
		if ship < 0 || ship >= len(f.Positions) || ship/3 != player || f.Positions[ship] < 0 || !catanExplorerTouches(g, f.Positions[ship], tile) || len(units) == 0 || len(units) > 2 {
			return errors.New("请选择接触巢穴的己方船及一至二名船员")
		}
		from, to := catanExplorerCargoLocation{"ship", ship}, catanExplorerCargoLocation{"lair", tile}
		if kind == "land" {
			if s.Ready > 0 || s.Resolved > 0 || len(c.contents(to))+len(units) > 3 {
				return errors.New("只能向未攻陷巢穴派驻，最多三人")
			}
		} else {
			if s.Resolved == 0 || sequence <= s.Resolved || c.used(from)+len(units) > 2 {
				return errors.New("仅后续回合可接回已解放金矿的己方船员，船需有空位")
			}
			from, to = to, from
		}
		for j, id := range units {
			if id < 0 || id >= len(c.Units) || id/11 != player || id%11 < 2 || slices.Contains(units[:j], id) || c.Units[id] != from {
				return errors.New("船员来源、归属或重复选择无效")
			}
		}
		for _, id := range units {
			c.Units[id] = to
		}
		if kind == "land" && len(c.contents(to)) == 3 {
			s.Ready, s.Captor = sequence, player
		}
	case "begin":
		if l.Battle != nil || c.Turn == nil || c.Turn.Phase != "ended" || s.Ready != sequence || s.Captor != player || s.Resolved > 0 {
			return errors.New("结束全部航行后才能结算本回合攻陷的巢穴")
		}
		counts := make([]int, len(g.Players))
		for _, id := range c.contents(catanExplorerCargoLocation{"lair", tile}) {
			counts[id/11]++
		}
		order := []int{}
		for offset := 0; offset < len(g.Players); offset++ {
			p := (player + offset) % len(g.Players)
			if counts[p] > 0 {
				if g.Players[p].Eliminated {
					return errors.New("离场参战船员须先由主控制器清理")
				}
				order = append(order, p)
			}
		}
		if e.GoldBank < 2*len(order) {
			return errors.New("巢穴奖金金币耗尽规则尚待核验")
		}
		s.Contributions = counts
		for _, p := range order {
			e.GoldBank -= 2
			e.Gold[p] += 2
			l.advance(p)
		}
		l.Battle = &catanExplorerLairBattle{Tile: tile, Player: player, Sequence: sequence, Candidates: order}
		if len(order) == 1 {
			return l.finishBattle(g, b, c, order[0])
		}
	case "roll":
		if l.Battle == nil || l.Battle.Tile != tile || randN == nil {
			return errors.New("当前没有巢穴英雄掷骰")
		}
		dice := make([]int, len(g.Players))
		for _, p := range l.Battle.Candidates {
			dice[p] = randN(6) + 1
		}
		winners, err := catanExplorerLairWinners(s.Contributions, l.Battle.Candidates, dice)
		if err != nil {
			return err
		}
		s.Rounds = append(s.Rounds, dice)
		if len(winners) == 1 {
			return l.finishBattle(g, b, c, winners[0])
		}
		l.Battle.Candidates = winners
	default:
		return errors.New("未知巢穴行动")
	}
	return nil
}
func (l *catanExplorerLairs) finishBattle(g *Catan, b *catanExplorerBoard, c *catanExplorerCargo, hero int) error {
	s := &l.Sites[l.site(l.Battle.Tile)]
	l.advance(hero)
	id := -1
	for _, unit := range c.contents(catanExplorerCargoLocation{"lair", s.Tile}) {
		if unit/11 == hero {
			id = unit
			break
		}
	}
	if id < 0 {
		return errors.New("英雄的参战船员缺失")
	}
	c.Units[id] = catanExplorerCargoLocation{"supply", -1}
	s.Hero, s.Resolved = hero, l.Battle.Sequence
	if b.Liberated == nil {
		b.Liberated = map[int]int{}
	}
	b.Liberated[s.Tile] = s.Number
	g.Tiles[s.Tile].Number = s.Number
	l.Battle = nil
	return nil
}

type catanExplorerLairView struct {
	Tile          int     `json:"tile"`
	Number        int     `json:"number,omitempty"`
	Ready         uint64  `json:"ready,omitempty"`
	Resolved      uint64  `json:"resolved,omitempty"`
	Hero          int     `json:"hero"`
	Contributions []int   `json:"contributions,omitempty"`
	Rounds        [][]int `json:"rounds,omitempty"`
}

type catanExplorerLairsView struct {
	Sites    []catanExplorerLairView  `json:"sites"`
	Progress []int                    `json:"progress"`
	Scores   []int                    `json:"scores"`
	Leader   int                      `json:"leader"`
	Left     int                      `json:"left"`
	Battle   *catanExplorerLairBattle `json:"battle,omitempty"`
}

func (l catanExplorerLairs) publicView() catanExplorerLairsView {
	sites := make([]catanExplorerLairView, 0, len(l.Sites))
	for _, s := range l.Sites {
		number := 0
		if s.Resolved > 0 {
			number = s.Number
		}
		sites = append(sites, catanExplorerLairView{s.Tile, number, s.Ready, s.Resolved, s.Hero, slices.Clone(s.Contributions), clone(s.Rounds)})
	}
	return catanExplorerLairsView{sites, slices.Clone(l.Progress), l.scores(), l.leader(), len(l.Deck), clone(l.Battle)}
}

func catanExplorerLairWinners(counts, candidates, dice []int) ([]int, error) {
	if len(counts) != len(dice) || len(candidates) == 0 {
		return nil, errors.New("英雄骰子人数无效")
	}
	winners := []int{}
	best, bestCount := -1, -1
	for p, die := range dice {
		if slices.Contains(candidates, p) {
			if die < 1 || die > 6 {
				return nil, errors.New("英雄骰子无效")
			}
		} else if die != 0 {
			return nil, errors.New("未参与重掷的玩家不能有骰子")
		}
	}
	for i, p := range candidates {
		if p < 0 || p >= len(counts) || counts[p] <= 0 || slices.Contains(candidates[:i], p) {
			return nil, errors.New("英雄候选无效")
		}
		score := dice[p] + counts[p]
		if score > best || score == best && counts[p] > bestCount {
			winners = []int{p}
			best, bestCount = score, counts[p]
		} else if score == best && counts[p] == bestCount {
			winners = append(winners, p)
		}
	}
	return winners, nil
}
func (l catanExplorerLairs) validateHistory(c *catanExplorerCargo) error {
	expected := make([]int, len(l.Progress))
	for _, s := range l.Sites {
		pending := l.Battle != nil && l.Battle.Tile == s.Tile
		if s.Ready == 0 {
			if s.Captor != -1 {
				return errors.New("未攻陷巢穴不能已有攻陷玩家")
			}
		} else if s.Captor < 0 || s.Captor >= len(expected) {
			return errors.New("攻陷玩家无效")
		}
		if s.Resolved == 0 && !pending {
			if len(s.Contributions)+len(s.Rounds) != 0 {
				return errors.New("未开始结算不能发奖或掷骰")
			}
			continue
		}
		if len(s.Contributions) != len(expected) || sum(s.Contributions) != 3 || slices.ContainsFunc(s.Contributions, func(n int) bool { return n < 0 || n > 3 }) {
			return errors.New("战斗参战船员无效")
		}
		actual := make([]int, len(expected))
		for _, id := range c.contents(catanExplorerCargoLocation{"lair", s.Tile}) {
			actual[id/11]++
		}
		candidates := []int{}
		for offset := 0; offset < len(expected); offset++ {
			p := (s.Captor + offset) % len(expected)
			count := s.Contributions[p]
			if count > 0 {
				expected[p]++
				candidates = append(candidates, p)
			}
			available := count
			if p == s.Hero {
				available--
			}
			if actual[p] > available || pending && actual[p] != count {
				return errors.New("战斗记录和船员实体不一致")
			}
		}
		for _, dice := range s.Rounds {
			if len(candidates) < 2 {
				return errors.New("已经决出英雄不能重掷")
			}
			var err error
			candidates, err = catanExplorerLairWinners(s.Contributions, candidates, dice)
			if err != nil {
				return err
			}
		}
		if pending {
			if l.Battle.Player != s.Captor || !slices.Equal(candidates, l.Battle.Candidates) {
				return errors.New("重掷候选与骰子历史不一致")
			}
		} else {
			if len(candidates) != 1 || candidates[0] != s.Hero {
				return errors.New("英雄与骰子历史不一致")
			}
			expected[s.Hero]++
		}
	}
	serial := 0
	for p, n := range expected {
		if l.Progress[p] != min(7, n) {
			return errors.New("任务进度与巢穴战果不一致")
		}
		serial += l.Progress[p]
	}
	if l.Serial != uint64(serial) {
		return errors.New("任务轨道移动序号不一致")
	}
	return nil
}
