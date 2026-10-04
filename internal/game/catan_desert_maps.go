package game

import "fmt"

// Through the Desert, 2025 Seafarers pages 10–11 and 5–6 extension page 7.
// The desert belt is traversable land but separates exploration regions.
func (g *Catan) makeSeafarersDesertThree() error {
	rows := [][]seaTerrain{
		{{7, 4}, {5, 0}, {6, 0}, {4, 8}},
		{{0, 3}, {5, 0}, {0, 4}, {6, 0}, {2, 12}},
		{{3, 6}, {5, 0}, {1, 5}, {2, 6}, {6, 0}, {6, 0}},
		{{6, 0}, {4, 3}, {0, 10}, {6, 0}, {7, 5}},
		{{0, 11}, {1, 6}, {3, 2}, {1, 9}, {6, 0}, {6, 0}},
		{{4, 10}, {3, 9}, {0, 8}, {6, 0}, {3, 9}},
		{{2, 8}, {2, 4}, {6, 0}, {4, 5}},
	}
	ports := []seaPort{
		{1, 2, 4, 0}, {3, 1, 2, 3}, {4, 0, 1, 2}, {4, 3, 4, -1},
		{5, 2, 5, -1}, {6, 0, 2, 4}, {6, 0, 0, 1}, {6, 1, 5, -1},
	}
	return g.makeSeafarersDesertFixed(rows, ports, [2]int{1, 1}, [2]int{4, 2})
}
func (g *Catan) makeSeafarersDesertFour() error {
	rows := [][]seaTerrain{
		{{7, 10}, {5, 0}, {0, 5}, {6, 0}, {4, 9}},
		{{4, 11}, {5, 0}, {1, 3}, {2, 6}, {6, 0}, {3, 4}},
		{{3, 8}, {5, 0}, {4, 8}, {3, 10}, {0, 4}, {6, 0}, {1, 2}},
		{{6, 0}, {0, 10}, {1, 11}, {2, 9}, {6, 0}, {6, 0}},
		{{1, 12}, {1, 6}, {3, 5}, {0, 8}, {6, 0}, {7, 5}, {2, 3}},
		{{2, 3}, {2, 11}, {4, 4}, {6, 0}, {6, 0}, {6, 0}},
		{{6, 0}, {0, 9}, {6, 0}, {4, 6}, {3, 12}},
	}
	ports := []seaPort{
		{0, 2, 5, 1}, {1, 3, 5, -1}, {2, 4, 0, -1}, {3, 1, 2, 2}, {5, 0, 2, 3},
		{5, 1, 1, 4}, {5, 2, 5, -1}, {6, 1, 5, 0}, {6, 1, 1, -1},
	}
	return g.makeSeafarersDesertFixed(rows, ports, [2]int{1, 1}, [2]int{4, 2})
}
func (g *Catan) makeSeafarersDesertSix() error {
	if len(g.Players) < 5 || len(g.Players) > 6 {
		return fmt.Errorf("此航海家地图需要5至6位玩家")
	}
	rows := [][]seaTerrain{
		{{7, 4}, {4, 6}, {1, 11}, {6, 0}, {0, 12}, {3, 5}, {4, 3}, {1, 6}},
		{{0, 5}, {2, 2}, {5, 0}, {5, 0}, {5, 0}, {5, 0}, {5, 0}, {6, 0}, {6, 0}},
		{{6, 0}, {6, 0}, {6, 0}, {3, 2}, {0, 8}, {2, 9}, {0, 10}, {6, 0}, {6, 0}, {7, 10}},
		{{4, 4}, {0, 5}, {3, 10}, {1, 5}, {2, 10}, {0, 4}, {1, 8}, {6, 0}, {6, 0}},
		{{2, 12}, {3, 3}, {1, 12}, {4, 6}, {1, 11}, {4, 3}, {3, 9}, {6, 0}, {6, 0}, {4, 8}},
		{{1, 9}, {4, 11}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {2, 11}, {0, 3}},
		{{2, 6}, {6, 0}, {2, 8}, {3, 2}, {6, 0}, {7, 4}, {6, 0}, {3, 9}},
	}
	ports := []seaPort{
		{3, 0, 3, -1}, {3, 1, 4, -1}, {2, 6, 5, -1}, {3, 6, 0, -1}, {4, 6, 1, -1},
		{4, 4, 1, 0}, {4, 2, 0, 1}, {4, 0, 3, 2}, {5, 0, 2, 3}, {5, 1, 0, 4}, {6, 0, 1, 2},
	}
	if err := g.makeSeafarersDesertFixed(rows, ports, [2]int{1, 4}, [2]int{4, 4}); err != nil {
		return err
	}
	// This map places the pirate on a sea hex, not on the frame.
	g.Seafarers.Pirate = 61
	g.shuffleSeafarersPorts()
	return nil
}
func (g *Catan) makeSeafarersDesertFixed(rows [][]seaTerrain, ports []seaPort, robber, home [2]int) error {
	if err := g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "desert", 14, robber, [][2]int{home}); err != nil {
		return err
	}
	g.Seafarers.Islands = g.findLandRegions(true)
	id := home[1]
	for r := 0; r < home[0]; r++ {
		id += len(rows[r])
	}
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[id]}
	return nil
}
