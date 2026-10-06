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

// Identify the printed origins in each connected wagon network. Multiple bits
// expose a merger; we do not guess how “different trains” works after merging.
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
func singleCaravanOrigin(mask int) bool { return mask > 0 && mask&(mask-1) == 0 }

func (c catanCaravans) responseChoices(g *Catan) []catanCaravanWagon {
	choices := c.choices(g)
	q := c.Pending
	if g.Two == nil || q == nil || q.Two == nil || q.Two.First == nil {
		return choices
	}
	leader, _ := q.leaders()
	if leader < 0 {
		return choices
	} // A tie gives each player one placement.
	return slices.DeleteFunc(choices, func(w catanCaravanWagon) bool {
		mask := c.trainOrigins(g, w.From)
		return !singleCaravanOrigin(mask) || mask == q.Two.Train
	})
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
	if t == nil || len(q.Order) != 2 || t.Start < 0 || t.Start%2 != 0 || t.Start > len(c.Wagons) || (q.Kind != "bid" && q.Kind != "place") {
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
		if !singleCaravanOrigin(t.Train) || c.trainOrigins(g, t.First.From) != t.Train {
			return errors.New("双人首辆商队标识无效或会合规则尚待核实")
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
	if first && leader >= 0 {
		t.Train = c.trainOrigins(g, w.From)
		if !singleCaravanOrigin(t.Train) {
			return errors.New("双人会合商队的不同商队规则尚待核实")
		}
	}
	if err := c.place(g, w); err != nil {
		return err
	}
	s.catanScores()
	s.catanVictory()
	// The rulebook ends the game as soon as the active player reaches 12 VP.
	if first && !s.Finished {
		t.First = &w
		if leader >= 0 && c.trainOrigins(g, w.From) != t.Train {
			return errors.New("双人会合商队的不同商队规则尚待核实")
		}
		if len(c.responseChoices(g)) == 0 {
			return errors.New("双人商队没有第二条合法延伸，该边界规则尚待核实")
		}
		if leader < 0 {
			q.Chooser = 1 - q.Active
		}
		s.catanLog(placedBy, "第一辆马车放到路线 #%d，从交点 #%d 出发；待放第二辆", w.Edge+1, w.From+1)
		return nil
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
