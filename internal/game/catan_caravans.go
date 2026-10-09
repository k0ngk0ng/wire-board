package game

import (
	"errors"
	"fmt"
	"slices"
)

// Public, irrevocable bids are held aside until the wagon has been placed.
// Turn always remains the action owner; Cursor/Chooser identify responders.
type catanCaravanVote struct {
	Two     *catanTwoCaravanPlacement `json:"two,omitempty"`
	Kind    string                    `json:"kind"`
	Active  int                       `json:"active"`
	Order   []int                     `json:"order"`
	Cursor  int                       `json:"cursor"`
	Chooser int                       `json:"chooser"`
	Bids    [][]int                   `json:"bids"`
	Votes   []*catanCaravanWagon      `json:"votes"`
}

// Public three-to-six-player recipe; extended numbers use the labelled site recipe.
func NewCatanCaravans(n int, options CatanOptions) (*State, error) {
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	m, err := s.Catan.makeCaravansMap()
	if err != nil {
		return nil, err
	}
	s.Catan.Caravans = &catanCaravans{Rules: CatanCaravansRules, Map: m, Wagons: []catanCaravanWagon{}}
	s.Log = append(s.Log, "商队：建设建筑后，行动结束时投票放置一辆马车；自己回合达到12分获胜")
	if n > 4 {
		s.Log = append(s.Log, catanExtendedNumberNotice)
	}
	s.catanScores()
	return s, s.validateCaravans()
}

func (q catanCaravanVote) actor() int {
	if q.Kind == "place" {
		return q.Chooser
	}
	if q.Cursor >= 0 && q.Cursor < len(q.Order) {
		return q.Order[q.Cursor]
	}
	return -1
}
func (g *Catan) caravanOrder(active int) []int {
	order := []int{}
	for step := 0; step < len(g.Players); step++ {
		p := (active + step) % len(g.Players)
		if !g.Players[p].Eliminated {
			order = append(order, p)
		}
	}
	return order
}
func (s *State) validateCaravans() error {
	g := s.Catan
	c := g.Caravans
	if c == nil {
		return nil
	}
	n := len(g.Players)
	if (c.Attack != "" || g.Attack != nil) && !g.caravansAttack() {
		return errors.New("商队蛮族组合标记无效")
	}
	if g.Rivers != nil && !g.riversCaravans() || g.Fishing != nil && !g.fishingCaravans() || g.BaseSetup != nil || g.Seafarers != nil || g.CitiesKnights != nil && !g.caravanKnights() || g.Harbors != nil || g.FriendlyRobber != nil || g.EventDeck == nil && (g.CardEvent != nil || g.RevealedEvent != nil) || g.Options.Helpers || (n > 4) != g.Options.FiveSix || (n > 4) != (g.Paired != nil) || c.Sequence < 0 || g.Robber < -1 || g.Robber >= len(g.Tiles) {
		return errors.New("商队状态或尚未核对的组合无效")
	}
	if err := g.validateCaravanKnights(); err != nil {
		return err
	}
	if err := c.validate(g); err != nil {
		return err
	}
	cards := 5
	if g.caravanKnights() {
		cards = 8
	}
	if len(g.Bank) != cards || (g.setup() && (c.Built || c.Sequence != 0 || len(c.Wagons) != 0)) {
		return errors.New("商队起始状态或银行无效")
	}
	for _, seat := range g.Players {
		if !g.cardBundle(seat.Resources) {
			return errors.New("商队玩家资源无效")
		}
	}
	q := c.Pending
	// Event responses precede production; bidding is strictly after construction.
	// Neither queue may conceal the other's responder after restoring a save.
	if g.CardEvent != nil && (q != nil || c.Built) {
		return errors.New("事件结算不能与商队建设或投票重叠")
	}
	if q != nil && g.CitiesKnights != nil && (g.CitiesKnights.Pending != nil || g.CitiesKnights.Event != nil) {
		return errors.New("城市事件必须先于商队投票完成")
	}
	if q != nil && g.EventDeck != nil && ((g.RevealedEvent == nil && !(g.caravanKnights() && g.EventDeck.alchemyLatest(g.RollID))) || g.RevealedEvent != nil && !g.RevealedEvent.ProductionStarted || g.Two != nil && len(g.Two.Rolls) != 2) {
		return errors.New("商队投票前必须完成本回合事件生产")
	}
	if err := c.validateShortRounds(g); err != nil {
		return err
	}
	if g.Two != nil && !s.Finished && q == nil && (len(c.Wagons)-len(c.ShortRounds))%2 != 0 {
		return errors.New("双人商队尚缺第二辆马车的待处理记录")
	}
	if q == nil {
		if slices.Contains([]string{"catan_caravan_bid", "catan_caravan_vote", "catan_caravan_place"}, s.Phase) {
			return errors.New("商队响应缺失")
		}
		return nil
	}
	if s.Finished || g.setup() || c.Built || c.Sequence < 1 || q.Active != s.Turn || q.Active < 0 || q.Active >= n || g.Players[q.Active].Eliminated || !slices.Equal(q.Order, g.caravanOrder(q.Active)) || len(q.Bids) != n || len(q.Votes) != n || g.Trade != nil || s.Phase != "catan_caravan_"+q.Kind {
		return errors.New("商队投票状态无效")
	}
	if err := s.validateTwoCaravanPlacement(); err != nil {
		return err
	}
	choices := c.responseChoices(g)
	if len(choices) == 0 {
		return errors.New("商队没有可放置位置")
	}
	if !slices.Contains([]string{"bid", "vote", "place"}, q.Kind) || q.actor() < 0 || q.actor() >= n || !slices.Contains(q.Order, q.actor()) || (q.Kind != "place" && (q.Cursor < 0 || q.Cursor >= len(q.Order) || q.Chooser != -1)) {
		return errors.New("商队响应者无效")
	}
	for p := range g.Players {
		index := slices.Index(q.Order, p)
		bid, vote := q.Bids[p], q.Votes[p]
		if index < 0 {
			if bid != nil || vote != nil {
				return errors.New("离场者不能投票")
			}
			continue
		}
		submitted := q.Kind != "bid" || index < q.Cursor
		if submitted {
			if !g.validCaravanBid(bid) {
				return fmt.Errorf("商队出价只能使用%s", g.caravanBidNames())
			}
		} else if bid != nil {
			return errors.New("商队尚未轮到此人出价")
		}
		if vote != nil && (q.Kind == "bid" || sum(bid) == 0 || !slices.Contains(choices, *vote)) {
			return errors.New("商队投票方向无效")
		}
		if q.Kind == "vote" && ((index < q.Cursor && sum(bid) > 0) != (vote != nil)) {
			return errors.New("商队投票顺序无效")
		}
	}
	if q.Kind == "vote" && sum(q.Bids[q.actor()]) == 0 {
		return errors.New("未出价者不能分配选票")
	}
	if q.Kind != "bid" && g.Two == nil {
		majority, largest := q.leaders()
		if majority >= 0 {
			if q.Kind != "place" || q.Chooser != majority {
				return errors.New("商队过半票决定者无效")
			}
			for _, vote := range q.Votes {
				if vote != nil {
					return errors.New("过半票不需要再次分配选票")
				}
			}
		} else if q.Kind == "place" {
			for _, p := range q.Order {
				if sum(q.Bids[p]) > 0 && q.Votes[p] == nil {
					return errors.New("商队选票尚未分配完成")
				}
			}
			if _, unique := q.winningLocation(); unique {
				return errors.New("已有获选位置，不能另选")
			}
			if largest < 0 {
				largest = q.Active
			}
			if q.Chooser != largest {
				return errors.New("商队平票决定者无效")
			}
		}
		if q.Kind == "place" && q.Cursor != len(q.Order) {
			return errors.New("商队出价或投票尚未结束")
		}
	}
	// Include escrow in conservation; submitted cards are public, not in hands.
	stock := 19
	if n > 4 {
		stock = 24
	}
	for color, bank := range g.Bank {
		if color >= 5 {
			stock = 12
			if n > 4 {
				stock = 18
			}
		}
		if bank < 0 || bank > stock {
			return errors.New("商队银行库存无效")
		}
		total := bank
		for p := range g.Players {
			total += g.Players[p].Resources[color]
			if color < len(q.Bids[p]) {
				total += q.Bids[p][color]
			}
		}
		if total != stock {
			return errors.New("商队出价资源不守恒")
		}
	}
	return nil
}

// Called only after the active player explicitly finishes their action phase.
func (s *State) catanBeginCaravanVote() bool {
	g := s.Catan
	c := g.Caravans
	if c == nil || !c.Built {
		return false
	}
	c.Built = false
	if len(c.choices(g)) == 0 {
		return false
	}
	c.Sequence++
	c.Pending = &catanCaravanVote{Kind: "bid", Active: s.Turn, Order: g.caravanOrder(s.Turn), Chooser: -1, Bids: make([][]int, len(g.Players)), Votes: make([]*catanCaravanWagon, len(g.Players))}
	if g.Two != nil {
		c.Pending.Two = &catanTwoCaravanPlacement{Start: len(c.Wagons)}
	}
	g.Trade = nil
	s.Phase = "catan_caravan_bid"
	s.catanLog(s.Turn, "结束建设，开始商队投票：每张%s算一票", g.caravanBidNames())
	return true
}

// Unique largest individual and strict majority are separate tie-break rules.
func (q catanCaravanVote) leaders() (majority, largest int) {
	total, maximum := 0, 0
	largest = -1
	for _, p := range q.Order {
		votes := sum(q.Bids[p])
		total += votes
		if votes > maximum {
			maximum, largest = votes, p
		} else if votes == maximum {
			largest = -1
		}
	}
	majority = -1
	if largest >= 0 && maximum*2 > total {
		majority = largest
	}
	return
}
func (s *State) catanCaravanChooser(player int) {
	q := s.Catan.Caravans.Pending
	q.Kind, q.Chooser = "place", player
	s.Phase = "catan_caravan_place"
	s.catanLog(player, "决定马车放置位置和方向")
}
func (q catanCaravanVote) winningLocation() (catanCaravanWagon, bool) {
	totals := map[catanCaravanWagon]int{}
	for _, p := range q.Order {
		if q.Votes[p] != nil {
			totals[*q.Votes[p]] += sum(q.Bids[p])
		}
	}
	maxVotes, ties := 0, 0
	var winner catanCaravanWagon
	for w, n := range totals {
		if n > maxVotes {
			maxVotes, ties, winner = n, 1, w
		} else if n == maxVotes {
			ties++
		}
	}
	return winner, ties == 1
}
func (s *State) catanCaravanResolve() error {
	q := s.Catan.Caravans.Pending
	if winner, unique := q.winningLocation(); unique {
		return s.catanFinishCaravan(winner)
	}
	_, largest := q.leaders()
	if largest < 0 {
		largest = q.Active
	}
	s.catanCaravanChooser(largest)
	return nil
}
func (s *State) catanCaravanAction(player int, a Action) error {
	g := s.Catan
	q := g.Caravans.Pending
	if q == nil || q.actor() != player || a.Type != "catan_caravan_"+q.Kind {
		return errors.New("请等待对应玩家完成商队响应")
	}
	switch q.Kind {
	case "bid":
		if !g.validCaravanBid(a.Tokens) || !catanHas(g.Players[player].Resources, a.Tokens) {
			return fmt.Errorf("请选择持有的%s，也可不出价", g.caravanBidNames())
		}
		q.Bids[player] = append([]int{}, a.Tokens...)
		for color, n := range a.Tokens {
			g.Players[player].Resources[color] -= n
		}
		if sum(a.Tokens) == 0 {
			s.catanLog(player, "商队投票不出价")
		} else {
			s.catanLog(player, "商队出价 %s，共%d票", catanText(a.Tokens), sum(a.Tokens))
		}
		q.Cursor++
		if q.Cursor < len(q.Order) {
			return nil
		}
		if g.Two != nil {
			leader, _ := q.leaders()
			if leader < 0 {
				leader = q.Active
			}
			s.catanCaravanChooser(leader)
			return nil
		}
		majority, _ := q.leaders()
		if majority >= 0 {
			s.catanCaravanChooser(majority)
			return nil
		}
		q.Kind, q.Cursor = "vote", 0
		s.Phase = "catan_caravan_vote"
		for q.Cursor < len(q.Order) && sum(q.Bids[q.Order[q.Cursor]]) == 0 {
			q.Cursor++
		}
		if q.Cursor == len(q.Order) {
			return s.catanCaravanResolve()
		}
	case "vote", "place":
		w := catanCaravanWagon{Edge: a.Edge, From: a.Vertex}
		if !slices.Contains(g.Caravans.responseChoices(g), w) {
			return errors.New("请选择合法的商队位置与前进方向")
		}
		if q.Kind == "place" {
			return s.catanFinishCaravan(w)
		}
		q.Votes[player] = &w
		s.catanLog(player, "将%d票投给路线 #%d（从交点 #%d 出发）", sum(q.Bids[player]), w.Edge+1, w.From+1)
		q.Cursor++
		for q.Cursor < len(q.Order) && sum(q.Bids[q.Order[q.Cursor]]) == 0 {
			q.Cursor++
		}
		if q.Cursor == len(q.Order) {
			return s.catanCaravanResolve()
		}
	}
	return nil
}
func (s *State) catanFinishCaravan(w catanCaravanWagon) error {
	g := s.Catan
	c := g.Caravans
	q := c.Pending
	if g.Two != nil {
		return s.catanTwoPlaceWagon(w)
	}
	if err := c.place(g, w); err != nil {
		return err
	}
	for _, bid := range q.Bids {
		for color, n := range bid {
			g.Bank[color] += n
		}
	}
	c.Pending = nil
	s.catanLog(q.Active, "商队投票结束：马车放到路线 #%d，从交点 #%d 出发；所有出价归还银行", w.Edge+1, w.From+1)
	s.catanScores()
	s.catanVictory()
	if !s.Finished {
		s.catanNext()
		s.catanVictory()
	}
	return nil
}

func (s *State) catanCaravanBot(player int) (Action, error) {
	g := s.Catan
	c := g.Caravans
	q := c.Pending
	if q == nil || q.actor() != player {
		return Action{}, errors.New("inactive caravan responder")
	}
	choices := c.responseChoices(g)
	if len(choices) == 0 {
		return Action{}, errors.New("no caravan placement")
	}
	best := choices[0]
	bestValue := -1 << 30
	for _, w := range choices {
		// Board-only evaluation: no rival resources, development cards or deck.
		value := 0
		e := g.Edges[w.Edge]
		if e.Owner == player {
			value += 12
		} else if e.Owner >= 0 && !g.Players[e.Owner].Eliminated {
			value -= 6
		}
		for _, v := range []int{e.A, e.B} {
			owner := g.Vertices[v].Owner
			if owner < 0 || g.Players[owner].Eliminated {
				continue
			}
			benefit := 3
			if c.buildingBonus(g, v) == 0 {
				for _, old := range c.Wagons {
					edge := g.Edges[old.Edge]
					if edge.A == v || edge.B == v {
						benefit += 40
						break
					}
				}
			}
			if owner == player {
				value += benefit
			} else {
				value -= benefit / 2
			}
		}
		if q.Kind == "vote" {
			for p, v := range q.Votes {
				if v != nil && *v == w {
					value += min(8, sum(q.Bids[p]))
				}
			}
		}
		if value > bestValue {
			best, bestValue = w, value
		}
	}
	a := Action{Type: "catan_caravan_" + q.Kind, Edge: best.Edge, Vertex: best.From}
	if q.Kind == "bid" {
		a.Tokens = make([]int, 5)
		// Keep building resources; spend surplus only on a useful public route.
		if bestValue > 0 {
			limit := 1
			if bestValue >= 40 {
				limit = 3
			}
			for _, color := range g.caravanBidColors() {
				a.Tokens[color] = min(limit, max(0, g.Players[player].Resources[color]-2))
				limit -= a.Tokens[color]
			}
		}
	}
	return a, nil
}
