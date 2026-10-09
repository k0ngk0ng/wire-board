package game

import (
	"errors"
	"fmt"
)

// Site extension: preserve the expanded desert frame and exploration regions.
// Two coastal production hexes become castles; the remaining mainland coast
// receives the printed thirteen-face expanded Barbarian Attack landing sequence.
func newCatanAttackDesertExtendedBoard(n int) (*Catan, *catanAttackMap, error) {
	if n < 5 || n > 6 {
		return nil, nil, errors.New("扩大蛮族沙漠需要五六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
	if err != nil {
		return nil, nil, err
	}
	g := s.Catan
	castles := []int{20, 46}
	for _, id := range castles {
		g.Tiles[id].Resource, g.Tiles[id].Number = catanCastle, 0
	}
	coast := []int{}
	for _, tile := range g.Tiles {
		if tile.Resource < 0 || tile.Resource > 4 || g.Seafarers.Islands[tile.ID] != g.Seafarers.StartIslands[0] {
			continue
		}
		water := false
		for _, edge := range g.Edges {
			for _, id := range edge.Tiles {
				if id == tile.ID && g.edgeTerrain(edge.ID, true) {
					water = true
				}
			}
		}
		if water {
			coast = append(coast, tile.ID)
		}
	}
	if len(coast) != 13 {
		return nil, nil, fmt.Errorf("扩大蛮族沙漠沿海配方不是十三格: %v", coast)
	}
	numbers := []int{2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 5, 9, 3}
	for i, id := range coast {
		g.Tiles[id].Number = numbers[i]
	}
	g.Robber, g.Seafarers.Pirate = -1, -1
	g.Seafarers.VictoryPoints = 14
	return g, &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: castles, Coast: coast, Barbarians: 48, Gold: 152}, nil
}
