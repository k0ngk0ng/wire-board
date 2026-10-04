package game

import "fmt"

// Fixed setup from the 2025 base 5–6 rulebook, page 2. Kept outside public
// room options until configuration and full expansion acceptance are ready.
type CatanBaseSetup struct {
	Layout       string `json:"layout"`
	Rules        string `json:"rules"`
	Colors       []int  `json:"colors"`
	NeutralColor int    `json:"neutralColor"` // -1 in a six-player game
}

// Row-major, top to bottom. Terrain IDs use the common resource order.
var catanFixedSixTerrain = []seaTerrain{
	{1, 10}, {2, 6}, {5, 0},
	{1, 6}, {3, 2}, {4, 9}, {1, 11},
	{0, 3}, {4, 11}, {0, 5}, {3, 10}, {2, 4},
	{5, 0}, {2, 5}, {3, 4}, {4, 6}, {2, 3}, {3, 8},
	{0, 12}, {4, 10}, {2, 2}, {1, 4}, {0, 11},
	{0, 8}, {3, 3}, {0, 9}, {3, 5},
	{1, 9}, {2, 12}, {4, 8},
}

// Tile, corner and road side for the first and black-outlined second village.
// Corners start at southeast and proceed clockwise, as in makeMap.
// Colors: blue, red, white, orange, purple, green.
var catanFixedSixPieces = [6][2][3]int{
	{{17, 1, 1}, {15, 0, 5}},
	{{3, 2, 1}, {13, 0, 0}},
	{{6, 0, 0}, {9, 0, 5}},
	{{23, 1, 0}, {14, 0, 0}},
	{{26, 1, 1}, {21, 2, 1}},
	{{0, 5, 5}, {9, 2, 2}},
}

// Tile, side, resource. Port types and positions are fixed, not shuffled.
var catanFixedSixPorts = [][3]int{
	{0, 3, -1}, {2, 3, 2}, {2, 5, -1}, {7, 2, 0},
	{17, 5, -1}, {18, 2, -1}, {22, 0, 1}, {23, 2, 3},
	{27, 1, -1}, {28, 0, 4}, {29, 0, 2},
}

func NewCatanFixedFiveSix(n int, options CatanOptions) (*State, error) {
	if n < 5 || n > 6 || !options.FiveSix {
		return nil, fmt.Errorf("此固定新手布局需要启用五至六人扩充并有5至6位玩家")
	}
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	colors := []int{0, 1, 2, 3, 4, 5}
	shuffle(colors)
	if err := s.applyCatanFixedFiveSix(colors); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *State) applyCatanFixedFiveSix(colors []int) error {
	g := s.Catan
	if g == nil || len(g.Players) < 5 || len(g.Players) > 6 || !g.Options.FiveSix || len(g.Tiles) != 30 || len(colors) != 6 || g.Seafarers != nil || g.BaseSetup != nil || g.SetupStep != 0 || s.Phase != "catan_setup_settlement" {
		return fmt.Errorf("固定布局只能用于尚未建设的五至六人基础地图")
	}
	for _, v := range g.Vertices {
		if v.Level > 0 {
			return fmt.Errorf("固定布局不能覆盖已放置的建筑")
		}
	}
	seen := [6]bool{}
	for _, c := range colors {
		if c < 0 || c >= 6 || seen[c] {
			return fmt.Errorf("固定布局颜色必须恰好包含六种颜色")
		}
		seen[c] = true
	}
	g.BaseSetup = &CatanBaseSetup{Layout: "fixed", Rules: "catan-base-5-6-2025", Colors: append([]int{}, colors[:len(g.Players)]...), NeutralColor: -1}
	for i, t := range catanFixedSixTerrain {
		g.Tiles[i].Resource, g.Tiles[i].Number = t.resource, t.number
	}
	// Either desert is nonproducing. Use the upper desert as the initial one.
	g.Robber = 2
	edgeAt := func(tile, side int) int {
		v := g.Tiles[tile].Vertices
		for _, e := range g.Edges {
			if e.A == v[side] && e.B == v[(side+1)%6] || e.B == v[side] && e.A == v[(side+1)%6] {
				return e.ID
			}
		}
		panic("fixed map side missing from topology")
	}
	g.Ports = nil
	for _, port := range catanFixedSixPorts {
		g.Ports = append(g.Ports, CatanPort{Edge: edgeAt(port[0], port[1]), Resource: port[2]})
	}
	s.Log = []string{"卡坦岛五至六人固定新手布局：按官方地图预放村庄和道路，从黑框村庄领取起始资源；采用配对回合"}
	for seat, color := range colors {
		owner := seat
		if seat >= len(g.Players) {
			owner = -1
			g.BaseSetup.NeutralColor = color
		}
		for pass, piece := range catanFixedSixPieces[color] {
			vertex := g.Tiles[piece[0]].Vertices[piece[1]]
			g.Vertices[vertex].Owner, g.Vertices[vertex].Level = owner, 1
			if owner < 0 {
				continue // The unused color keeps both settlements, but no roads.
			}
			g.Edges[edgeAt(piece[0], piece[2])].Owner = owner
			if pass == 1 {
				gain := make([]int, 5)
				for _, tile := range g.Tiles {
					if tile.Resource >= 5 {
						continue
					}
					for _, v := range tile.Vertices {
						if v == vertex {
							gain[tile.Resource]++
						}
					}
				}
				catanMove(g.Bank, g.Players[owner].Resources, gain)
				s.catanLog(owner, "从固定起始村庄获得 %s", catanText(gain))
			}
		}
		if owner >= 0 && g.Options.Helpers {
			id := (owner-g.StartPlayer+len(g.Players))%len(g.Players) + 1
			g.Players[owner].Helper = &CatanHelperSeat{ID: id}
		}
	}
	g.SetupStep, g.SetupVertex, g.TurnSerial = g.SetupLimit(), -1, 1
	s.Turn, s.Phase = g.StartPlayer, "catan_roll"
	s.catanScores()
	return nil
}
