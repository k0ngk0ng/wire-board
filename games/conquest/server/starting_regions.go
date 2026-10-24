package conquest

import (
	"errors"
	"math/rand/v2"
)

// Randomize the entire candidate list, not only the order of preset capitals.
// A small independent-set search preserves separation over both land and sea.
// The private persisted seed makes initial-state replay deterministic.
func startingRegions(g *geography, players int, seed uint64) ([]string, error) {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	order := rng.Perm(len(g.data.Provinces))
	chosen := make([]string, 0, players)
	blocked := make([]int, len(order))
	var search func(int) bool
	search = func(start int) bool {
		if len(chosen) == players {
			return true
		}
		if len(order)-start < players-len(chosen) {
			return false
		}
		for i := start; i < len(order); i++ {
			index := order[i]
			if blocked[index] > 0 {
				continue
			}
			p := g.data.Provinces[index]
			neighbors := append(append([]string{}, p.Neighbors...), p.SeaNeighbors...)
			chosen = append(chosen, p.ID)
			for _, id := range neighbors {
				blocked[g.index[id]]++
			}
			if search(i + 1) {
				return true
			}
			for _, id := range neighbors {
				blocked[g.index[id]]--
			}
			chosen = chosen[:len(chosen)-1]
		}
		return false
	}
	if !search(0) {
		return nil, errors.New("地图无法安排互不相邻的出生点")
	}
	return chosen, nil
}
