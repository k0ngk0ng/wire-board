package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func explorerWorldSnapshot(t *testing.T, g *Catan, x *catanExplorer) string {
	t.Helper()
	b, err := json.Marshal([]any{g, x})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func explorerWorldReject(t *testing.T, g *Catan, x *catanExplorer, action func() error) {
	t.Helper()
	before := explorerWorldSnapshot(t, g, x)
	if err := action(); err == nil {
		t.Fatal("invalid integrated action accepted")
	}
	if explorerWorldSnapshot(t, g, x) != before {
		t.Fatal("rejected action partially changed map, payment or ship")
	}
}
func explorerWorldRestore(t *testing.T, g *Catan, x *catanExplorer) {
	t.Helper()
	before := explorerWorldSnapshot(t, g, x)
	q, next, err := x.copy(g)
	if err != nil {
		t.Fatal(err)
	}
	*g, *x = *q, *next
	if explorerWorldSnapshot(t, g, x) != before {
		t.Fatal("combined persistence changed state")
	}
}
func explorerWorldTurn(t *testing.T, g *Catan, x *catanExplorer, player int, movement bool) uint64 {
	t.Helper()
	sequence := uint64(1)
	if x.Economy.Turn != nil {
		sequence = x.Economy.Turn.Sequence + 1
	}
	if err := x.Economy.beginProduction(g, x.Fleet, x.Cargo, player, sequence); err != nil {
		t.Fatal(err)
	}
	// Fixed non-seven production is intentional for this component test.
	if _, err := x.Economy.resolveProduction(g, x.Fleet, x.Cargo, player, sequence, [2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	if movement {
		if err := x.Cargo.beginMovement(g, x.Fleet, player, sequence, 0); err != nil {
			t.Fatal(err)
		}
	}
	return sequence
}

// Planning reads only public terrain. Its first fog contact must be the end
// of the path; it cannot use the hidden terrain or number order to steer.
func explorerWorldFogPath(g *Catan, start int) []int {
	queue, seen := [][]int{{start}}, map[int]bool{start: true}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		last := path[len(path)-1]
		if len(path) > 1 {
			for _, tile := range g.Tiles {
				if tile.Resource == CatanFog && catanExplorerTouches(g, last, tile.ID) {
					return path[1:]
				}
			}
		}
		for _, edge := range g.Edges {
			if !seen[edge.ID] && catanExplorerSeaEdge(g, edge.ID) && catanExplorerAdjacentEdges(edge, g.Edges[last]) {
				seen[edge.ID] = true
				queue = append(queue, append(slices.Clone(path), edge.ID))
			}
		}
	}
	return nil
}

func TestCatanExplorerWorldPrintedShipsDiscoverAndReceiveRewards(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			g, x, err := newCatanExplorerLandHoWorld(n)
			if err != nil {
				t.Fatal(err)
			}
			done := make([]bool, n)
			regions := [2]bool{}
			for round := 0; round < 8; round++ {
				for player := 0; player < n; player++ {
					sequence := explorerWorldTurn(t, g, x, player, true)
					if !done[player] {
						path := explorerWorldFogPath(g, x.Fleet.Positions[player*3])
						if len(path) == 0 {
							t.Fatal("no route from printed opening ship")
						}
						path = path[:min(4, len(path))]
						hand, gold := slices.Clone(g.Players[player].Resources), x.Economy.Gold[player]
						stacks := [2][]int{slices.Clone(x.Board.Numbers[0]), slices.Clone(x.Board.Numbers[1])}
						result, err := x.sail(g, player, sequence, player*3, path)
						if err != nil {
							t.Fatal(err)
						}
						if len(result.Discoveries) != len(result.Sail.Exploring) {
							t.Fatal("all contacts must reveal and reward together")
						}
						for _, award := range result.Discoveries {
							region := -1
							for r, ids := range x.Board.Regions {
								if slices.Contains(ids, award.Tile) {
									region = r
								}
							}
							if region < 0 {
								t.Fatal("discovery not in official region")
							}
							regions[region] = true
							if award.Resource < 5 {
								if award.Number != stacks[region][0] || sum(award.Resources) != 1 || award.Resources[award.Resource] != 1 || award.Gold != 0 {
									t.Fatal("land resource/number reward", award)
								}
								stacks[region] = stacks[region][1:]
								hand[award.Resource]++
							} else {
								if award.Resource != CatanSea || award.Number != 0 || sum(award.Resources) != 0 || award.Gold != 2 {
									t.Fatal("sea reward", award)
								}
								gold += 2
							}
							if g.Tiles[award.Tile].Resource != award.Resource || g.Tiles[award.Tile].Number != award.Number {
								t.Fatal("board didn't commit reward's tile")
							}
							// Returned arrays cannot mutate actual bank or hands.
							award.Resources[0] = 99
						}
						if !slices.Equal(hand, g.Players[player].Resources) || gold != x.Economy.Gold[player] {
							t.Fatal("reward must transfer to discoverer")
						}
						for r := range stacks {
							if !slices.Equal(stacks[r], x.Board.Numbers[r]) {
								t.Fatal("regional number stack mismatch")
							}
						}
						if len(result.Discoveries) > 0 {
							done[player] = true
							if !x.Fleet.Turn.Ships[player*3].Closed || len(x.Fleet.Turn.Exploring) != 0 {
								t.Fatal("discovery must stop ship and clear response atomically")
							}
							explorerWorldReject(t, g, x, func() error { _, err := x.sail(g, player, sequence, player*3, path); return err })
						}
						explorerWorldRestore(t, g, x)
					}
					if err := x.Cargo.endMovement(g, x.Fleet, player, sequence); err != nil {
						t.Fatal(err)
					}
				}
				if !slices.Contains(done, false) {
					break
				}
			}
			if slices.Contains(done, false) || regions != ([2]bool{true, true}) {
				t.Fatal("every opening ship must reach discovery; both regions covered", done, regions)
			}
		})
	}
}

// Explicit midgame fixture: reveal ordinary coastal land and place one paid-
// construction target harbor there. No claim this setup was naturally played.
func explorerWorldBuildDiscoveryFixture(t *testing.T) (*Catan, *catanExplorer, int, int) {
	t.Helper()
	g, x, err := newCatanExplorerLandHoWorld(3)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range slices.Clone(x.Board.Hidden) {
		if hidden.Resource >= 5 {
			continue
		}
		if _, err = x.Board.reveal(g, hidden.Tile); err != nil {
			t.Fatal(err)
		}
		for _, v := range g.Vertices {
			if v.Level != 0 || !catanExplorerLandVertex(g, v.ID) {
				continue
			}
			nearBuilding := false
			for _, edge := range g.Edges {
				if edge.A == v.ID && g.Vertices[edge.B].Level > 0 || edge.B == v.ID && g.Vertices[edge.A].Level > 0 {
					nearBuilding = true
				}
			}
			if nearBuilding {
				continue
			}
			for _, edge := range g.Edges {
				if edge.A != v.ID && edge.B != v.ID || !catanExplorerSeaEdge(g, edge.ID) || slices.Contains(x.Fleet.Positions, edge.ID) {
					continue
				}
				edgeFog, tipFog := false, false
				for _, tile := range edge.Tiles {
					edgeFog = edgeFog || g.Tiles[tile].Resource == CatanFog
				}
				for _, tile := range g.Tiles {
					tipFog = tipFog || tile.Resource == CatanFog && catanExplorerTouches(g, edge.ID, tile.ID)
				}
				if edgeFog || !tipFog {
					continue
				}
				g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level = 0, 2
				g.Players[0].Score += 2
				for resource := 0; resource < 5; resource++ {
					g.Players[0].Resources[resource] += 2
					g.Bank[resource] -= 2
				}
				explorerWorldTurn(t, g, x, 0, false)
				if err = x.validate(g); err != nil {
					t.Fatal(err)
				}
				return g, x, v.ID, edge.ID
			}
		}
	}
	t.Fatal("official board lacks a known edge whose far endpoint touches fog")
	return nil, nil, -1, -1
}

func TestCatanExplorerWorldNewShipDiscoveryStopsBeforeMovement(t *testing.T) {
	g, x, _, edge := explorerWorldBuildDiscoveryFixture(t)
	before := slices.Clone(g.Players[0].Resources)
	gold := x.Economy.Gold[0]
	awards, err := x.buildShip(g, 0, 1, 1, edge)
	if err != nil {
		t.Fatal(err)
	}
	if len(awards) == 0 || !slices.Equal(x.Cargo.Turn.BuildStopped, []int{1}) || x.Fleet.Positions[1] != edge {
		t.Fatal("new vessel must resolve tip discovery during construction")
	}
	before[0]--
	before[2]--
	for _, a := range awards {
		for r, n := range a.Resources {
			before[r] += n
		}
		gold += a.Gold
	}
	if !slices.Equal(before, g.Players[0].Resources) || gold != x.Economy.Gold[0] {
		t.Fatal("building cost and discovery award must both apply once")
	}
	explorerWorldRestore(t, g, x)
	if err = x.Cargo.beginMovement(g, x.Fleet, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if !x.Fleet.Turn.Ships[1].Closed || x.Fleet.Turn.Ships[1].Remaining != 0 || x.Fleet.Turn.Ships[0].Remaining != 4 {
		t.Fatal("only newly discovering vessel is stopped")
	}
	explorerWorldReject(t, g, x, func() error { return x.Fleet.wool(g, 0, 1, 1) })
	explorerWorldReject(t, g, x, func() error { _, err := x.sail(g, 0, 1, 1, []int{edge}); return err })
	explorerWorldRestore(t, g, x)
	if err = x.Cargo.endMovement(g, x.Fleet, 0, 1); err != nil {
		t.Fatal(err)
	}
	explorerWorldTurn(t, g, x, 0, true)
	if len(x.Cargo.Turn.BuildStopped) != 0 || x.Fleet.Turn.Ships[1].Remaining != 4 || x.Fleet.Turn.Ships[1].Closed {
		t.Fatal("next turn restores ordinary movement budget")
	}
	explorerWorldRestore(t, g, x)
}

func TestCatanExplorerWorldBuildDiscoveryRollsBackPaymentAndMap(t *testing.T) {
	g, x, _, edge := explorerWorldBuildDiscoveryFixture(t)
	// Force its still-hidden contact to be sea by swapping two unrevealed
	// pieces in the same region; official component inventory stays unchanged.
	contact := -1
	for i, h := range x.Board.Hidden {
		if !h.Revealed && catanExplorerTouches(g, edge, h.Tile) {
			contact = i
			break
		}
	}
	if contact < 0 {
		t.Fatal("missing fog tip")
	}
	if x.Board.Hidden[contact].Resource != CatanSea {
		other := -1
		for i, h := range x.Board.Hidden {
			if !h.Revealed && h.Region == x.Board.Hidden[contact].Region && h.Resource == CatanSea {
				other = i
				break
			}
		}
		if other < 0 {
			t.Fatal("missing reserve sea")
		}
		x.Board.Hidden[contact].Resource, x.Board.Hidden[other].Resource = x.Board.Hidden[other].Resource, x.Board.Hidden[contact].Resource
	}
	x.Economy.Gold[1] += x.Economy.GoldBank - 1
	x.Economy.GoldBank = 1
	if err := x.validate(g); err != nil {
		t.Fatal(err)
	}
	explorerWorldReject(t, g, x, func() error { _, err := x.buildShip(g, 0, 1, 1, edge); return err })
	if x.Fleet.Positions[1] != -1 || x.Board.Hidden[contact].Revealed {
		t.Fatal("shortage must not pay, place ship or reveal map")
	}
	// Restore exactly one coin from the holder to the bank and retry the
	// identical action: it should succeed once, not award a second reveal.
	x.Economy.Gold[1]--
	x.Economy.GoldBank++
	if _, err := x.buildShip(g, 0, 1, 1, edge); err != nil {
		t.Fatal(err)
	}
	explorerWorldReject(t, g, x, func() error { _, err := x.buildShip(g, 0, 1, 1, edge); return err })
	explorerWorldRestore(t, g, x)
}

func TestCatanExplorerWorldDiscoveryCanImmediatelyFoundSettlement(t *testing.T) {
	g, x, err := newCatanExplorerLandHoWorld(3)
	if err != nil {
		t.Fatal(err)
	}
	// Deterministic hidden fixture, preserving the printed region inventory.
	// Navigation below still sees only fog; this is not a random natural game.
	for region, ids := range x.Board.Regions {
		resources := catanExplorerRegionResources(region, false)
		for j, tile := range ids {
			for i := range x.Board.Hidden {
				if x.Board.Hidden[i].Tile == tile {
					x.Board.Hidden[i].Resource = resources[j]
				}
			}
		}
	}
	settled := false
	for step := 0; step < 10 && !settled; step++ {
		sequence := explorerWorldTurn(t, g, x, 0, true)
		path := explorerWorldFogPath(g, x.Fleet.Positions[0])
		if len(path) == 0 {
			t.Fatal("no more unexplored shore")
		}
		result, err := x.sail(g, 0, sequence, 0, path[:min(4, len(path))])
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Discoveries) > 0 {
			explorerWorldRestore(t, g, x)
			edge := g.Edges[x.Fleet.Positions[0]]
			for _, vertex := range []int{edge.A, edge.B} {
				if !catanExplorerLandVertex(g, vertex) {
					continue
				}
				before := explorerWorldSnapshot(t, g, x)
				hand, bank, score := slices.Clone(g.Players[0].Resources), slices.Clone(g.Bank), g.Players[0].Score
				if err = x.Cargo.settle(g, x.Fleet, 0, sequence, 0, vertex); err != nil {
					if explorerWorldSnapshot(t, g, x) != before {
						t.Fatal("invalid founding changed state")
					}
					continue
				}
				if !slices.Equal(hand, g.Players[0].Resources) || !slices.Equal(bank, g.Bank) || g.Players[0].Score != score+1 || x.Fleet.Positions[0] != -1 || x.Cargo.Units[0] != (catanExplorerCargoLocation{"supply", -1}) {
					t.Fatal("founding after discovery should return ship/settler without charging twice")
				}
				settled = true
				explorerWorldRestore(t, g, x)
				break
			}
		}
		if err = x.Cargo.endMovement(g, x.Fleet, 0, sequence); err != nil {
			t.Fatal(err)
		}
	}
	if !settled {
		t.Fatal("printed settler ship never founded on its newly discovered coast")
	}
}

func TestCatanExplorerWorldResourceDiscoveryFailureRollsBackMove(t *testing.T) {
	g, x, err := newCatanExplorerLandHoWorld(3)
	if err != nil {
		t.Fatal(err)
	}
	// Find a single legal sea step from no-fog contact to a hidden tile.
	// Relocating the ship before that step is an explicit midgame fixture.
	from, to := -1, -1
	contacts := []int{}
	for _, a := range g.Edges {
		if !catanExplorerSeaEdge(g, a.ID) {
			continue
		}
		safe := true
		for _, h := range x.Board.Hidden {
			if catanExplorerTouches(g, a.ID, h.Tile) {
				safe = false
			}
		}
		if !safe {
			continue
		}
		for _, b := range g.Edges {
			if !catanExplorerSeaEdge(g, b.ID) || !catanExplorerAdjacentEdges(a, b) {
				continue
			}
			ids := []int{}
			for _, h := range x.Board.Hidden {
				if catanExplorerTouches(g, b.ID, h.Tile) {
					ids = append(ids, h.Tile)
				}
			}
			if len(ids) == 1 {
				from, to, contacts = a.ID, b.ID, ids
				break
			}
		}
		if from >= 0 {
			break
		}
	}
	if from < 0 {
		t.Fatal("missing exploration geometry")
	}
	slices.Sort(contacts)
	// Assign brick by a swap within its region, preserving physical inventory.
	for _, tile := range contacts {
		index := -1
		for i, h := range x.Board.Hidden {
			if h.Tile == tile {
				index = i
			}
		}
		other := -1
		for i, h := range x.Board.Hidden {
			if h.Region == x.Board.Hidden[index].Region && h.Resource == 1 {
				other = i
				break
			}
		}
		if other < 0 {
			t.Fatal("missing unique regional resource")
		}
		x.Board.Hidden[index].Resource, x.Board.Hidden[other].Resource = x.Board.Hidden[other].Resource, x.Board.Hidden[index].Resource
	}
	x.Fleet.Positions[0] = from
	sequence := explorerWorldTurn(t, g, x, 0, true)
	// Every brick is in one hand: the discovery cannot finish. Its movement,
	// revealed terrain and regional number draw must all roll back together.
	for p := range g.Players {
		g.Bank[1] += g.Players[p].Resources[1]
		g.Players[p].Resources[1] = 0
	}
	g.Players[1].Resources[1], g.Bank[1] = 19, 0
	if err = x.validate(g); err != nil {
		t.Fatal(err)
	}
	explorerWorldReject(t, g, x, func() error { _, err := x.sail(g, 0, sequence, 0, []int{to}); return err })
	if x.Fleet.Positions[0] != from || g.Tiles[contacts[0]].Resource != CatanFog {
		t.Fatal("failed reward leaked reveal or move")
	}
	g.Players[1].Resources[1]--
	g.Bank[1]++
	result, err := x.sail(g, 0, sequence, 0, []int{to})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Discoveries) != 1 || result.Discoveries[0].Resource != 1 {
		t.Fatal("retry should complete the original reward once")
	}
	explorerWorldRestore(t, g, x)
}
