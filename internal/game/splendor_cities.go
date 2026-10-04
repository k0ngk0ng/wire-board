package game

import "fmt"

// A tile has two printed faces. Three distinct tiles are selected during setup,
// with one random face each. Cities stay public and are never claimed/removed.
type GemCity struct {
	Tile   int    `json:"tile"`
	Side   int    `json:"side"`
	Name   string `json:"name"`
	Points int    `json:"points"`
	Cost   [5]int `json:"cost"`
	Any    int    `json:"any,omitempty"`
}

func gemCityEligible(p GemPlayer, c GemCity) bool {
	if p.Eliminated || p.Score < c.Points {
		return false
	}
	counts := p.gemCardCounts()
	for color, need := range c.Cost {
		if counts[color] < need {
			return false
		}
	}
	if c.Any > 0 {
		for color, have := range counts {
			if c.Cost[color] == 0 && have >= c.Any {
				return true
			}
		}
		return false
	}
	return true
}
func (g *Splendor) gemCitiesFor(p GemPlayer) []int {
	result := []int{}
	for i, c := range g.Cities {
		if gemCityEligible(p, c) {
			result = append(result, i)
		}
	}
	return result
}
func (s *State) gemCityTrigger() bool {
	g := s.Splendor
	eligible := g.gemCitiesFor(g.Players[s.Turn])
	if len(eligible) == 0 {
		return false
	}
	if !g.LastRound {
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 满足城市「%s」，完成本轮后结算；只有满足城市条件的玩家参与胜负比较", s.Turn+1, g.Cities[eligible[0]].Name))
	}
	return true
}
