package game

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
)

func explorerMapRestore(t *testing.T, g *Catan, m *catanExplorerBoard) (*Catan, *catanExplorerBoard) {
	t.Helper()
	data, err := json.Marshal(struct {
		Game  *Catan
		Board *catanExplorerBoard
	}{g, m})
	if err != nil {
		t.Fatal(err)
	}
	var next struct {
		Game  *Catan
		Board *catanExplorerBoard
	}
	if err = json.Unmarshal(data, &next); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, next.Game) || !reflect.DeepEqual(m, next.Board) {
		t.Fatal("board persistence changed state")
	}
	if err = next.Board.validate(next.Game); err != nil {
		t.Fatal(err)
	}
	return next.Game, next.Board
}
func TestCatanExplorerMapsOfficialInventoriesAndGeometry(t *testing.T) {
	for _, scenario := range []string{"land-ho", "pirate-lairs"} {
		for n := 2; n <= 4; n++ {
			for trial := 0; trial < 8; trial++ {
				g, m, err := newCatanExplorerBoard(n, scenario, "fixed")
				if err != nil {
					t.Fatal(err)
				}
				target, total, sea, fog, region := 8, 51, 20, 16, 8
				if scenario == "pirate-lairs" {
					target, total, sea, fog, region = 12, 58, 21, 22, 11
				}
				if len(g.Tiles) != total || m.Target != target || len(m.Starting) != 15 || len(m.Regions[0]) != region || len(m.Regions[1]) != region || len(m.HarborStarts) != 14 {
					t.Fatal("official board footprint", scenario, len(g.Tiles), m.publicView())
				}
				counts := make([]int, 9)
				numbers := []int{}
				for _, tile := range g.Tiles {
					counts[tile.Resource]++
					if tile.Number != 0 {
						numbers = append(numbers, tile.Number)
					}
				}
				if !slices.Equal(counts, []int{4, 2, 4, 2, 3, 0, sea, 0, fog}) {
					t.Fatal("14 loose starting land + printed pasture, sea and hidden regions", counts)
				}
				if !catanExplorerSameInventory(numbers, []int{3, 3, 4, 4, 5, 6, 6, 8, 8, 9, 10, 10, 11, 11, 12}) {
					t.Fatal("15 starting numbers, including printed frame slot", numbers)
				}
				if g.Tiles[m.FramePasture].Resource != 2 || g.Tiles[m.FramePasture].Number != 6 || g.Tiles[m.FrameSea].Resource != CatanSea {
					t.Fatal("fixed frame hexes")
				}
				for i, expected := range []int{11, 9, 3, 8, 4, 10, 6, 12, 8, 10, 4, 11, 6, 3, 5} {
					if g.Tiles[m.Starting[i]].Number != expected {
						t.Fatal("position of starting number", i)
					}
				}
				// Independent expected region column positions, transcribed from figures.
				wantParrot := []int{4, 5, 10, 11, 12, 17, 19, 20}
				wantGoose := []int{34, 36, 37, 42, 43, 44, 49, 50}
				if scenario == "pirate-lairs" {
					wantParrot = []int{4, 5, 6, 11, 12, 13, 14, 19, 20, 22, 23}
					wantGoose = []int{38, 39, 41, 42, 47, 48, 49, 50, 55, 56, 57}
				}
				if !slices.Equal(m.Regions[0], wantParrot) || !slices.Equal(m.Regions[1], wantGoose) {
					t.Fatal("wrong hidden hex positions", m.Regions)
				}
				for region := range 2 {
					hiddenCounts := make([]int, 8)
					for _, h := range m.Hidden {
						if h.Region == region {
							hiddenCounts[h.Resource]++
						}
					}
					want := []int{1, 1, 1, 1, 2, 0, 2, 0}
					if region == 1 {
						want = []int{1, 1, 1, 2, 1, 0, 2, 0}
					}
					if scenario == "pirate-lairs" {
						want[7] = 3
					}
					if !slices.Equal(hiddenCounts, want) {
						t.Fatal("regional physical inventory", region, hiddenCounts)
					}
				}
				for _, edge := range g.Edges {
					if edge.A == edge.B || len(edge.Tiles) < 1 || len(edge.Tiles) > 2 {
						t.Fatal("invalid edge")
					}
				}
				// A simply connected planar hex board includes exactly one outside face.
				if len(g.Vertices)-len(g.Edges)+len(g.Tiles) != 1 {
					t.Fatal("map has holes or duplicate topology")
				}
				explorerMapRestore(t, g, m)
			}
		}
	}
}
func TestCatanExplorerLandHoPrintedStartingPieces(t *testing.T) {
	g, m, err := newCatanExplorerBoard(4, "land-ho", "fixed")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Opening) != 4 {
		t.Fatal("all four printed colors required")
	}
	vertices := []int{}
	for color, p := range m.Opening {
		vertices = append(vertices, p.Settlement, p.Harbor)
		road, ship := g.Edges[p.Road], g.Edges[p.Ship]
		if road.A != p.Settlement && road.B != p.Settlement || ship.A != p.Harbor && ship.B != p.Harbor || !catanExplorerSeaEdge(g, p.Ship) {
			t.Fatal("opening pieces not connected", color, p)
		}
		if !slices.Contains(m.HarborStarts, p.Harbor) {
			t.Fatal("printed harbor outside eastern shore", color)
		}
		resources := make([]int, 5)
		for _, tile := range g.Tiles {
			if tile.Resource < 5 && slices.Contains(tile.Vertices, p.Settlement) {
				resources[tile.Resource]++
			}
		}
		if !slices.Equal(resources, p.Resources) {
			t.Fatal("printed cards must match ordinary settlement, not harbor", color, resources, p.Resources)
		}
	}
	for i, a := range vertices {
		for _, b := range vertices[i+1:] {
			if a == b {
				t.Fatal("overlapping starting pieces")
			}
			for _, edge := range g.Edges {
				if edge.A == a && edge.B == b || edge.A == b && edge.B == a {
					t.Fatal("printed setup violates distance rule", a, b)
				}
			}
		}
	}
	// This constructor records positions only. Main setup must still place
	// active colors and, only for two players, the neutral obstacles.
	for _, v := range g.Vertices {
		if v.Owner != -1 || v.Level != 0 {
			t.Fatal("premature setup mutation")
		}
	}
}
func TestCatanExplorerPirateVariableIslandPreservesFrameAndNumbers(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		g, m, err := newCatanExplorerBoard(3, "pirate-lairs", "variable")
		if err != nil {
			t.Fatal(err)
		}
		land := []int{}
		for _, tile := range m.Starting {
			land = append(land, g.Tiles[tile].Resource)
		}
		seen[fmt.Sprint(land)] = true
		if g.Tiles[m.FramePasture].Resource != 2 || g.Tiles[m.FramePasture].Number != 6 {
			t.Fatal("printed pasture changed")
		}
		explorerMapRestore(t, g, m)
	}
	if len(seen) < 2 {
		t.Fatal("variable starting island never changed")
	}
	for _, args := range []struct {
		n                int
		scenario, layout string
	}{{1, "land-ho", "fixed"}, {5, "land-ho", "fixed"}, {6, "pirate-lairs", "fixed"}, {3, "land-ho", "variable"}, {3, "fish-for-catan", "fixed"}} {
		if _, _, err := newCatanExplorerBoard(args.n, args.scenario, args.layout); err == nil {
			t.Fatal("unverified recipe accepted", args)
		}
	}
}
func TestCatanExplorerMapRevealSeparateNumberStacksAndPrivacy(t *testing.T) {
	for _, scenario := range []string{"land-ho", "pirate-lairs"} {
		g, m, err := newCatanExplorerBoard(3, scenario, "fixed")
		if err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(struct {
			Tiles []CatanTile
			View  catanExplorerBoardView
		}{g.Tiles, m.publicView()})
		// Secret permutations with the same physical inventory must be invisible.
		m.Hidden[0].Resource, m.Hidden[1].Resource = m.Hidden[1].Resource, m.Hidden[0].Resource
		slices.Reverse(m.Numbers[0])
		slices.Reverse(m.Numbers[1])
		after, _ := json.Marshal(struct {
			Tiles []CatanTile
			View  catanExplorerBoardView
		}{g.Tiles, m.publicView()})
		if string(before) != string(after) {
			t.Fatal("secret terrain or number order leaked")
		}
		v := m.publicView()
		v.Starting[0] = -1
		v.Regions[0][0] = -1
		v.HarborStarts[0] = -1
		if len(v.Opening) > 0 {
			v.Opening[0].Resources[0] = 99
		}
		if err = m.validate(g); err != nil {
			t.Fatal("public slices alias stored board", err)
		}
		// Reveal in reverse order, switching regions each step; numbers must
		// follow the corresponding regional stack, never preassigned hidden hexes.
		for j := len(m.Regions[0]) - 1; j >= 0; j-- {
			for region := 0; region < 2; region++ {
				tile := m.Regions[region][j]
				oldCount := [2]int{len(m.Numbers[0]), len(m.Numbers[1])}
				top := 0
				if oldCount[region] > 0 {
					top = m.Numbers[region][0]
				}
				h, err := m.reveal(g, tile)
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				if h.Resource < 5 {
					want = top
					oldCount[region]--
				}
				if h.Number != want || len(m.Numbers[0]) != oldCount[0] || len(m.Numbers[1]) != oldCount[1] {
					t.Fatal("wrong regional number consumption", h, m.Numbers)
				}
				if err = m.validate(g); err != nil {
					t.Fatal(err)
				}
				g, m = explorerMapRestore(t, g, m)
				before, _ := json.Marshal([]any{g, m})
				if _, err = m.reveal(g, tile); err == nil {
					t.Fatal("duplicate discovery")
				}
				after, _ := json.Marshal([]any{g, m})
				if string(before) != string(after) {
					t.Fatal("duplicate discovery partly changed state")
				}
			}
		}
		if m.publicView().Unexplored != ([2]int{}) || len(m.Numbers[0])+len(m.Numbers[1]) != 0 {
			t.Fatal("all ordinary terrain must consume exactly six numbers per region")
		}
	}
}
func TestCatanExplorerMapRejectsCorruption(t *testing.T) {
	for _, change := range []func(*Catan, *catanExplorerBoard){
		func(g *Catan, m *catanExplorerBoard) { g.Tiles[m.FramePasture].Resource = 0 },
		func(g *Catan, m *catanExplorerBoard) { g.Tiles[m.Starting[0]].Number = 5 },
		func(g *Catan, m *catanExplorerBoard) { g.Tiles[0].X = math.NaN() },
		func(g *Catan, m *catanExplorerBoard) { g.HexSize = math.NaN() },
		func(g *Catan, m *catanExplorerBoard) { g.Edges[0].B = g.Edges[0].A },
		func(g *Catan, m *catanExplorerBoard) { m.Hidden[0].Resource = CatanDesert },
		func(g *Catan, m *catanExplorerBoard) { m.Hidden[0].Tile = m.Hidden[1].Tile },
		func(g *Catan, m *catanExplorerBoard) { m.Hidden[0].Region = 1 },
		func(g *Catan, m *catanExplorerBoard) { m.Hidden[0].Number = 3 },
		func(g *Catan, m *catanExplorerBoard) { m.Numbers[0][0] = 2 },
		func(g *Catan, m *catanExplorerBoard) { m.HarborStarts = m.HarborStarts[:1] },
		func(g *Catan, m *catanExplorerBoard) { m.Opening[0].Ship = 0 },
	} {
		g, m, err := newCatanExplorerBoard(3, "land-ho", "fixed")
		if err != nil {
			t.Fatal(err)
		}
		change(g, m)
		if err = m.validate(g); err == nil {
			t.Fatal("accepted damaged map")
		}
	}
}

func TestCatanExplorerPrintedHarborsSailToBothHiddenRegions(t *testing.T) {
	for players := 2; players <= 4; players++ {
		g, m, err := newCatanExplorerBoard(players, "land-ho", "fixed")
		if err != nil {
			t.Fatal(err)
		}
		fleet, _ := newCatanExplorerSailing(players)
		for p := 0; p < players; p++ {
			fleet.Positions[3*p] = m.Opening[p].Ship
		}
		gold, bank := make([]int, players), 100
		found := [2]bool{}
		sequence := uint64(0)
		for player := 0; player < players; player++ {
			discovered := false
			for turn := 0; turn < 12 && !discovered; turn++ {
				sequence++
				if err = fleet.begin(g, player, sequence, 0); err != nil {
					t.Fatal(err)
				}
				// Find the nearest unexplored shoreline using public terrain only.
				// No production, rewards, cargo or score is simulated here.
				start := fleet.Positions[player*3]
				queue, seen := [][]int{{start}}, map[int]bool{start: true}
				var path []int
				for len(queue) > 0 && path == nil {
					current := queue[0]
					queue = queue[1:]
					last := current[len(current)-1]
					if len(current) > 1 {
						for _, tile := range g.Tiles {
							if tile.Resource == CatanFog && catanExplorerTouches(g, last, tile.ID) {
								path = current[1:]
								break
							}
						}
					}
					if path != nil {
						break
					}
					for edge, e := range g.Edges {
						if !seen[edge] && catanExplorerSeaEdge(g, edge) && catanExplorerAdjacentEdges(e, g.Edges[last]) {
							seen[edge] = true
							queue = append(queue, append(slices.Clone(current), edge))
						}
					}
				}
				if len(path) == 0 {
					t.Fatal("printed harbor has no route to either exploration region", player)
				}
				path = path[:min(4, len(path))]
				q, err := fleet.sail(g, player, sequence, player*3, path, -1, -1, gold, &bank)
				if err != nil {
					t.Fatal(err)
				}
				fleet = explorerSailingRestore(t, g, fleet)
				if len(q.Exploring) > 0 {
					for _, tile := range q.Exploring {
						h, err := m.reveal(g, tile)
						if err != nil {
							t.Fatal(err)
						}
						found[h.Region] = true
					}
					g, m = explorerMapRestore(t, g, m)
					if err = fleet.discovered(g, player, sequence); err != nil {
						t.Fatal(err)
					}
					if !fleet.Turn.Ships[player*3].Closed {
						t.Fatal("discovery must not restore the ship's movement")
					}
					discovered = true
				}
				if err = fleet.end(g, player, sequence); err != nil {
					t.Fatal(err)
				}
				fleet = explorerSailingRestore(t, g, fleet)
			}
			if !discovered {
				t.Fatal("printed starting ship failed to discover", player)
			}
		}
		if found != ([2]bool{true, true}) {
			t.Fatal("printed harbors should reach parrot and goose regions", players, found)
		}
	}
}
