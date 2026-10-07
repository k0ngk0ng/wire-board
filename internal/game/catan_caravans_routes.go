package game

import (
	"errors"
	"slices"
	"sort"
)

const CatanCaravansRules = "catan-caravans-2025"

// Chronological directed wagons can be replayed to validate a saved network.
// Pending holds public bids while the action owner remains State.Turn.
type catanCaravans struct {
	Rules       string                      `json:"rules,omitempty"`
	Map         *catanCaravanMap            `json:"map"`
	Wagons      []catanCaravanWagon         `json:"wagons"`
	Built       bool                        `json:"built"`
	Sequence    int                         `json:"sequence"`
	Pending     *catanCaravanVote           `json:"pending,omitempty"`
	ShortRounds []catanTwoCaravanShortRound `json:"shortRounds,omitempty"`
}

func (c catanCaravans) choices(g *Catan) []catanCaravanWagon {
	choices := []catanCaravanWagon{}
	if c.Map == nil || len(c.Wagons) >= c.Map.Supply {
		return choices
	}
	occupied := map[int]bool{}
	outgoing := map[int]bool{}
	heads := map[int]bool{}
	for _, w := range c.Wagons {
		occupied[w.Edge] = true
		outgoing[w.From] = true
		e := g.Edges[w.Edge]
		to := e.A
		if to == w.From {
			to = e.B
		}
		heads[to] = true
	}
	appendChoice := func(edge, from int) {
		choice := catanCaravanWagon{edge, from}
		if !occupied[edge] && !outgoing[from] && !slices.Contains(choices, choice) {
			choices = append(choices, choice)
		}
	}
	for _, start := range c.Map.Starts {
		appendChoice(start.Edge, start.From)
	}
	for head := range heads {
		if !outgoing[head] {
			for _, edge := range g.touching(head) {
				appendChoice(edge, head)
			}
		}
	}
	sort.Slice(choices, func(i, j int) bool {
		if choices[i].Edge == choices[j].Edge {
			return choices[i].From < choices[j].From
		}
		return choices[i].Edge < choices[j].Edge
	})
	return choices
}

func (c catanCaravans) validate(g *Catan) error {
	if c.Rules != "" && c.Rules != CatanCaravansRules {
		return errors.New("商队规则版本无效")
	}

	if c.Map == nil {
		return errors.New("商队地图缺失")
	}
	if err := c.Map.validate(g); err != nil {
		return err
	}
	if len(c.Wagons) > c.Map.Supply {
		return errors.New("商队马车超出供应")
	}
	replay := catanCaravans{Map: c.Map}
	for _, wagon := range c.Wagons {
		if !slices.Contains(replay.choices(g), wagon) {
			return errors.New("商队马车断开、重复或朝向分叉")
		}
		replay.Wagons = append(replay.Wagons, wagon)
	}
	return nil
}

func (c *catanCaravans) place(g *Catan, wagon catanCaravanWagon) error {
	if err := c.validate(g); err != nil {
		return err
	}
	if !slices.Contains(c.choices(g), wagon) {
		return errors.New("请选择商队起点或队头可延伸的空边及正确朝向")
	}
	c.Wagons = append(c.Wagons, wagon)
	return nil
}

func (c catanCaravans) roadWeight(edge int) int {
	for _, w := range c.Wagons {
		if w.Edge == edge {
			return 2
		}
	}
	return 1
}
func (c catanCaravans) buildingBonus(g *Catan, vertex int) int {
	count := 0
	for _, w := range c.Wagons {
		e := g.Edges[w.Edge]
		if e.A == vertex || e.B == vertex {
			count++
		}
	}
	if count >= 2 {
		return 1
	}
	return 0
}
