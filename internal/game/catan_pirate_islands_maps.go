package game

import "fmt"

// NewCatanPirateIslands builds the fixed official scenario with the normal
// setup, Helpers and paired-turn machinery. Public availability is controlled
// by the room catalogue, separately from the full rules catalogue.
func NewCatanPirateIslands(n int, options CatanOptions) (*State, error) {
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	if n > 4 {
		err = s.Catan.makeSeafarersPirateIslandsSix()
	} else {
		err = s.Catan.makeSeafarersPirateIslandsFour()
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Public board state for The Pirate Islands (Seafarers scenario 7). The
// fortresses are separate from buildings until recaptured: they neither
// produce resources nor score as settlements.
type CatanPirateFortress struct {
	Root        int   `json:"root"`
	Route       []int `json:"route"`
	StartVertex int   `json:"startVertex"`
	StartShip   int   `json:"startShip"`
	Beachhead   int   `json:"beachhead"`
	Vertex      int   `json:"vertex"`
	Strength    int   `json:"strength"`
}
type CatanPirateIslands struct {
	KnightsRules string                `json:"knightsRules,omitempty"`
	CityFleet    *CatanEventFleet      `json:"cityFleet,omitempty"`
	Battle       *CatanPirateBattle    `json:"battle,omitempty"`
	SevenPending bool                  `json:"sevenPending,omitempty"`
	Raid         *CatanPirateRaid      `json:"raid,omitempty"`
	Colors       []int                 `json:"colors"` // indices in the shared blue/red/white/orange/purple/green palette
	HomeTiles    []int                 `json:"homeTiles"`
	FleetPath    []int                 `json:"fleetPath"`  // cyclic, in printed arrow order; first is the initial tile
	SafeTile     int                   `json:"safeTile"`   // -1 in 3–4; the printed ! sea hex in 5–6
	Fortresses   []CatanPirateFortress `json:"fortresses"` // indexed by seat, matching the six player colors
}

// A printed marker uses row, column, corner; a ship uses row, column, side.
// Corners run SE, S, SW, NW, N, NE; sides run SE, SW, W, NW, NE, E.
type pirateSeatMap struct{ start, ship, beach, fort [3]int }

// 2025 Seafarers pages 16–17. White's locations are unused with three players.
func (g *Catan) makeSeafarersPirateIslandsFour() error {
	if len(g.Players) < 3 || len(g.Players) > 4 {
		return fmt.Errorf("此海盗群岛地图需要3至4位玩家")
	}
	rows := [][]seaTerrain{
		{{7, 11}, {4, 6}, {6, 0}, {6, 0}, {3, 4}, {1, 5}},
		{{1, 0}, {6, 0}, {6, 0}, {5, 0}, {6, 0}, {4, 9}, {0, 10}},
		{{3, 4}, {6, 0}, {6, 0}, {5, 0}, {6, 0}, {0, 3}, {2, 8}, {0, 5}},
		{{6, 0}, {4, 8}, {6, 0}, {6, 0}, {3, 6}, {1, 9}, {2, 12}},
		{{3, 10}, {6, 0}, {6, 0}, {5, 0}, {6, 0}, {2, 11}, {0, 8}, {2, 9}},
		{{1, 0}, {6, 0}, {6, 0}, {2, 0}, {6, 0}, {4, 5}, {0, 2}},
		{{7, 3}, {4, 6}, {6, 0}, {6, 0}, {3, 10}, {1, 4}},
	}
	ports := []seaPort{{0, 4, 3, 0}, {0, 5, 3, 1}, {0, 5, 5, 2}, {1, 6, 5, 3}, {3, 6, 5, 4}, {4, 7, 0, -1}, {5, 6, 0, -1}, {6, 4, 0, -1}}
	path := [][2]int{{6, 3}, {6, 2}, {5, 2}, {4, 2}, {3, 2}, {2, 2}, {1, 2}, {0, 2}, {0, 3}, {1, 4}, {2, 4}, {3, 3}, {4, 4}, {5, 4}}
	// Blue, red, orange, white: omit the printed white slot with three players.
	seats := []pirateSeatMap{
		{[3]int{3, 4, 1}, [3]int{3, 4, 1}, [3]int{3, 1, 1}, [3]int{4, 0, 0}},
		{[3]int{0, 4, 1}, [3]int{0, 4, 1}, [3]int{0, 1, 5}, [3]int{0, 0, 4}},
		{[3]int{6, 4, 4}, [3]int{6, 4, 3}, [3]int{6, 1, 0}, [3]int{6, 0, 1}},
		{[3]int{3, 4, 4}, [3]int{3, 4, 3}, [3]int{3, 1, 4}, [3]int{2, 0, 5}},
	}
	return g.makeSeafarersPirateIslandsFixed(rows, ports, path, [2]int{-1, -1}, seats)
}

// 2025 Seafarers 5–6 extension page 10. Purple is absent with five players;
// the sea hex marked ! moves the fleet normally but suppresses its attack.
func (g *Catan) makeSeafarersPirateIslandsSix() error {
	if len(g.Players) < 5 || len(g.Players) > 6 {
		return fmt.Errorf("此海盗群岛地图需要5至6位玩家")
	}
	rows := [][]seaTerrain{
		{{7, 3}, {6, 0}, {4, 6}, {6, 0}, {6, 0}, {2, 5}, {3, 4}, {1, 9}},
		{{6, 0}, {6, 0}, {6, 0}, {6, 0}, {5, 0}, {6, 0}, {4, 3}, {1, 10}, {0, 11}},
		{{7, 11}, {5, 0}, {4, 8}, {6, 0}, {6, 0}, {6, 0}, {0, 12}, {0, 6}, {2, 4}, {0, 10}},
		{{6, 0}, {6, 0}, {6, 0}, {5, 0}, {6, 0}, {3, 6}, {3, 4}, {4, 5}, {2, 8}},
		{{7, 11}, {5, 0}, {4, 6}, {6, 0}, {6, 0}, {6, 0}, {2, 2}, {2, 10}, {1, 3}, {2, 5}},
		{{6, 0}, {6, 0}, {6, 0}, {6, 0}, {5, 0}, {6, 0}, {3, 11}, {4, 9}, {1, 8}},
		{{7, 3}, {6, 0}, {4, 8}, {6, 0}, {6, 0}, {0, 9}, {3, 5}, {0, 4}},
	}
	ports := []seaPort{{0, 6, 3, 0}, {0, 7, 3, 1}, {0, 7, 5, 2}, {1, 8, 5, 3}, {3, 8, 5, 4}, {5, 8, 5, -1}, {6, 7, 5, -1}, {6, 7, 1, -1}, {6, 6, 1, -1}}
	path := [][2]int{{6, 3}, {5, 3}, {4, 3}, {3, 2}, {2, 3}, {1, 3}, {0, 3}, {0, 4}, {1, 5}, {2, 5}, {3, 4}, {4, 5}, {5, 5}, {6, 4}}
	seats := []pirateSeatMap{
		{[3]int{3, 5, 4}, [3]int{3, 5, 3}, [3]int{2, 2, 0}, [3]int{2, 0, 0}},
		{[3]int{0, 5, 3}, [3]int{0, 4, 4}, [3]int{0, 2, 5}, [3]int{0, 0, 3}},
		{[3]int{3, 5, 1}, [3]int{3, 5, 1}, [3]int{4, 2, 5}, [3]int{4, 0, 4}},
		{[3]int{0, 5, 1}, [3]int{0, 5, 1}, [3]int{0, 2, 1}, [3]int{0, 0, 1}},
		{[3]int{6, 5, 4}, [3]int{6, 5, 3}, [3]int{6, 2, 4}, [3]int{6, 0, 4}},
		{[3]int{6, 5, 2}, [3]int{6, 4, 0}, [3]int{6, 2, 0}, [3]int{6, 0, 2}},
	}
	return g.makeSeafarersPirateIslandsFixed(rows, ports, path, [2]int{3, 2}, seats)
}

func (g *Catan) makeSeafarersPirateIslandsFixed(rows [][]seaTerrain, ports []seaPort, path [][2]int, safe [2]int, seats []pirateSeatMap) error {
	home := [2]int{0, len(rows[0]) - 1}
	if err := g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "pirate_islands", 10, home, [][2]int{home}); err != nil {
		return err
	}
	g.Robber = -1
	g.Seafarers.IslandBonus = 0
	g.shuffleSeafarersPorts()
	p := &CatanPirateIslands{SafeTile: -1, Colors: []int{0, 1, 3, 2, 5, 4}[:len(g.Players)]}
	g.Seafarers.PirateIslands = p
	tileID := func(row, col int) int {
		for r := 0; r < row; r++ {
			col += len(rows[r])
		}
		return col
	}
	for _, t := range g.Tiles {
		if g.Seafarers.Islands[t.ID] == g.Seafarers.StartIslands[0] {
			p.HomeTiles = append(p.HomeTiles, t.ID)
		}
	}
	for _, pos := range path {
		p.FleetPath = append(p.FleetPath, tileID(pos[0], pos[1]))
	}
	g.Seafarers.Pirate = p.FleetPath[0]
	if safe[0] >= 0 {
		p.SafeTile = tileID(safe[0], safe[1])
	}
	vertex := func(pos [3]int) int { return g.Tiles[tileID(pos[0], pos[1])].Vertices[pos[2]] }
	for player, spec := range seats[:len(g.Players)] {
		v := vertex(spec.start)
		t := g.Tiles[tileID(spec.ship[0], spec.ship[1])]
		a, b := t.Vertices[spec.ship[2]], t.Vertices[(spec.ship[2]+1)%6]
		edge := -1
		for _, e := range g.Edges {
			if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
				edge = e.ID
				break
			}
		}
		if edge < 0 || (v != a && v != b) || !g.edgeTerrain(edge, true) {
			return fmt.Errorf("海盗群岛预设船只未连接起始村庄")
		}
		g.Vertices[v].Owner, g.Vertices[v].Level = player, 1
		g.Edges[edge].Owner, g.Edges[edge].Ship = player, true
		p.Fortresses = append(p.Fortresses, CatanPirateFortress{Root: v, Route: []int{edge}, StartVertex: v, StartShip: edge, Beachhead: vertex(spec.beach), Vertex: vertex(spec.fort), Strength: 3})
	}
	deck := []int{}
	for _, card := range g.DevDeck {
		if card == 4 && len(g.Players) == 3 {
			continue
		}
		deck = append(deck, card)
	}
	g.DevDeck = deck
	return nil
}
