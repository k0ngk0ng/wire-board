package game

import (
	"errors"
	"slices"
)

// Keep the first placement in the save. Bids remain escrowed until the whole
// round ends; a reconnect must neither repeat the first wagon nor repay bids.
type catanTwoCaravanPlacement struct {
	Start int                `json:"start"`
	First *catanCaravanWagon `json:"first,omitempty"`
	Train int                `json:"train,omitempty"`
}

// Identify a train by all printed origins in its connected network. Trains
// meeting at an intersection merge, so a multi-bit mask is one train.
func (c catanCaravans) trainOrigins(g *Catan, from int) int {
	parent := make([]int, len(g.Vertices))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(v int) int {
		if parent[v] != v {
			parent[v] = root(parent[v])
		}
		return parent[v]
	}
	for _, w := range c.Wagons {
		e := g.Edges[w.Edge]
		parent[root(e.A)] = root(e.B)
	}
	origins := 0
	for i, start := range c.Map.Starts {
		if root(start.From) == root(from) {
			origins |= 1 << i
		}
	}
	return origins
}

// A receipt distinguishes an authorized one-wagon round from a lost pending
// second placement. Replaying its prefix must prove that no two-wagon sequence
// was possible, under the bidding outcome (different trains or a tie).
type catanTwoCaravanShortRound struct {
	Start     int  `json:"start"`
	Different bool `json:"different"`
}

func (c catanCaravans) secondChoices(g *Catan, train int) []catanCaravanWagon {
	return slices.DeleteFunc(c.choices(g), func(w catanCaravanWagon) bool {
		return train != 0 && c.trainOrigins(g, w.From) == train
	})
}

// Look ahead one wagon before offering the first placement. If two are
// possible, do not offer a first move that needlessly prevents the second.
func (c catanCaravans) firstChoices(g *Catan, different bool) ([]catanCaravanWagon, int) {
	choices := c.choices(g)
	pairs := []catanCaravanWagon{}
	for _, w := range choices {
		after := c
		after.Wagons = append(slices.Clone(c.Wagons), w)
		train := 0
		if different {
			train = after.trainOrigins(g, w.From)
		}
		if len(after.secondChoices(g, train)) > 0 {
			pairs = append(pairs, w)
		}
	}
	if len(pairs) > 0 {
		return pairs, 2
	}
	if len(choices) > 0 {
		return choices, 1
	}
	return choices, 0
}

func (c catanCaravans) validateShortRounds(g *Catan) error {
	if len(c.ShortRounds) > c.Sequence {
		return errors.New("双人商队少放记录超出已开始轮数")
	}
	previous := -1
	for i, r := range c.ShortRounds {
		if g.Two == nil || r.Start <= previous || r.Start < i || r.Start >= len(c.Wagons) || (r.Start-i)%2 != 0 {
			return errors.New("双人商队少放记录无效")
		}
		if c.Pending != nil && c.Pending.Two != nil && r.Start >= c.Pending.Two.Start {
			return errors.New("双人商队少放记录与当前轮重叠")
		}
		before := c
		before.Wagons = c.Wagons[:r.Start]
		choices, count := before.firstChoices(g, r.Different)
		if count != 1 || !slices.Contains(choices, c.Wagons[r.Start]) {
			return errors.New("双人商队本可放第二辆，少放记录无效")
		}
		previous = r.Start
	}
	return nil
}

func (c catanCaravans) responseChoices(g *Catan) []catanCaravanWagon {
	q := c.Pending
	if g.Two == nil || q == nil || q.Two == nil || q.Kind != "place" {
		return c.choices(g)
	}
	leader, _ := q.leaders()
	if q.Two.First == nil {
		choices, _ := c.firstChoices(g, leader >= 0)
		return choices
	}
	return c.secondChoices(g, q.Two.Train)
}

func (s *State) validateTwoCaravanPlacement() error {
	g, c := s.Catan, s.Catan.Caravans
	q := c.Pending
	if g.Two == nil {
		if q.Two != nil {
			return errors.New("普通商队不能携带双人放车进度")
		}
		return nil
	}
	t := q.Two
	if t == nil || len(q.Order) != 2 || t.Start < len(c.ShortRounds) || (t.Start-len(c.ShortRounds))%2 != 0 || t.Start > len(c.Wagons) || (q.Kind != "bid" && q.Kind != "place") {
		return errors.New("双人商队响应状态无效")
	}
	for _, v := range q.Votes {
		if v != nil {
			return errors.New("双人商队不分配位置选票")
		}
	}
	if t.First == nil {
		if len(c.Wagons) != t.Start || t.Train != 0 {
			return errors.New("双人首辆马车进度无效")
		}
	} else {
		if q.Kind != "place" || len(c.Wagons) != t.Start+1 || c.Wagons[t.Start] != *t.First {
			return errors.New("双人已放马车记录无效")
		}
	}
	if q.Kind == "bid" {
		return nil
	}
	if q.Cursor != 2 {
		return errors.New("请先完成双方出价")
	}
	leader, _ := q.leaders()
	chooser := leader
	if leader < 0 {
		chooser = q.Active
		if t.First != nil {
			chooser = 1 - chooser
		}
		if t.Train != 0 {
			return errors.New("平票没有独占商队记录")
		}
	} else if t.First != nil {
		if t.Train <= 0 || c.trainOrigins(g, t.First.From) != t.Train {
			return errors.New("双人首辆商队标识无效")
		}
	}
	if q.Chooser != chooser {
		return errors.New("双人商队放车者无效")
	}
	return nil
}

func (s *State) catanTwoPlaceWagon(w catanCaravanWagon) error {
	g, c := s.Catan, s.Catan.Caravans
	q := c.Pending
	t := q.Two
	if t == nil || !slices.Contains(c.responseChoices(g), w) {
		return errors.New("请选择本次允许延伸的商队")
	}
	placedBy := q.actor()
	leader, _ := q.leaders()
	first := t.First == nil
	if err := c.place(g, w); err != nil {
		return err
	}
	s.catanScores()
	s.catanVictory()
	// The rulebook ends the game as soon as the active player reaches 12 VP.
	if first && !s.Finished {
		t.First = &w
		if leader >= 0 {
			t.Train = c.trainOrigins(g, w.From)
		}
		if len(c.responseChoices(g)) > 0 {
			if leader < 0 {
				q.Chooser = 1 - q.Active
			}
			s.catanLog(placedBy, "第一辆马车放到路线 #%d，从交点 #%d 出发；待放第二辆", w.Edge+1, w.From+1)
			return nil
		}
		c.ShortRounds = append(c.ShortRounds, catanTwoCaravanShortRound{Start: t.Start, Different: leader >= 0})
		s.catanLog(placedBy, "本站补充规则：本轮最多只能放1辆马车，放车完成")
	}
	for _, bid := range q.Bids {
		for color, n := range bid {
			g.Bank[color] += n
		}
	}
	c.Pending = nil
	s.catanLog(q.Active, "双人商队放车结束：马车放到路线 #%d，从交点 #%d 出发；所有出价归还银行", w.Edge+1, w.From+1)
	if !s.Finished {
		s.catanNext()
		s.catanVictory()
	}
	return nil
}
