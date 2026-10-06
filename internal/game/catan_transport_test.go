package game

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
)

func transportBoard(t *testing.T, n int) (*Catan, *catanTransportMap) {
	t.Helper()
	g, m, err := newCatanTransportBoard(n)
	if err != nil {
		t.Fatal(err)
	}
	return g, m
}
func TestCatanTransportOfficialMapAndRestoration(t *testing.T) {
	for n := 2; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			terrains, ports := map[string]bool{}, map[string]bool{}
			for range 24 {
				g, m := transportBoard(t, n)
				wantTiles, wantVertices, wantEdges, wantPaths, wantBlocked, gold := 19, 57, 84, 12, 9, 100
				if n > 4 {
					wantTiles, wantVertices, wantEdges, wantPaths, wantBlocked, gold = 37, 103, 170, 38, 6, 152
				}
				if len(g.Tiles) != wantTiles || len(g.Vertices) != wantVertices || len(g.Edges) != wantEdges || m.Gold != gold || g.Robber != -1 || g.LongestOwner != -1 {
					t.Fatal("physical board/component inventory", len(g.Tiles), len(g.Vertices), len(g.Edges))
				}
				terrain, numbers := []int{}, []int{}
				for _, tile := range g.Tiles {
					terrain, numbers = append(terrain, tile.Resource), append(numbers, tile.Number)
					if (tile.Resource < 5) != (tile.Number > 0) {
						t.Fatal("non-producing commodity/desert", tile)
					}
				}
				// Independent rows read from the numbered 5–6 diagram, p11.
				if n > 4 {
					rows := [][]int{{0, 6, 10, 0}, {4, 0, 3, 12, 8}, {3, 5, 10, 5, 9, 11}, {0, 8, 12, 0, 2, 6, 0}, {11, 9, 4, 9, 5, 3}, {6, 2, 11, 0, 4}, {0, 10, 8, 0}}
					flat := []int{}
					for _, row := range rows {
						flat = append(flat, row...)
					}
					if !slices.Equal(numbers, flat) {
						t.Fatal("printed number faces", numbers)
					}
				} else {
					counts := map[int]int{}
					for _, v := range numbers {
						counts[v]++
					}
					for _, v := range []int{3, 4, 5, 6, 8, 9, 10, 11} {
						if counts[v] != 2 {
							t.Fatal("base number inventory", counts)
						}
					}
					if counts[2] != 0 || counts[12] != 0 || counts[0] != 3 {
						t.Fatal("base must omit 2 and 12")
					}
				}
				pathCount, blockedCount := 0, 0
				for _, site := range m.Sites {
					pathCount += len(site.Paths)
					blockedCount += len(site.Blocked)
					if len(site.Paths)+2*len(site.Blocked)/3 != 6 {
						t.Fatal("physical four/six-way commodity tile")
					}
					for _, edge := range site.Paths {
						e := g.Edges[edge]
						if e.B != site.Center || !slices.Contains(g.Tiles[site.Tile].Vertices, e.A) || math.Abs(math.Hypot(g.Vertices[e.A].X-g.Vertices[e.B].X, g.Vertices[e.A].Y-g.Vertices[e.B].Y)-g.HexSize) > .001 {
							t.Fatal("interior path must connect a corner to its plaza")
						}
					}
					for _, edge := range site.Blocked {
						if len(g.Edges[edge].Tiles) != 1 {
							t.Fatal("X markers must face frame, not neighbor")
						}
					}
				}
				if pathCount != wantPaths || blockedCount != wantBlocked {
					t.Fatal("extra paths and X markers", pathCount, blockedCount)
				}
				for _, edge := range m.Barbarians {
					if !m.canBuildRoad(g, edge) || len(g.Edges[edge].Tiles) != 2 {
						t.Fatal("starting barbarians are on inland hex sides")
					}
				}
				encoded, _ := json.Marshal(struct {
					Board *Catan
					Map   *catanTransportMap
				}{g, m})
				var restored struct {
					Board *Catan
					Map   *catanTransportMap
				}
				if err := json.Unmarshal(encoded, &restored); err != nil {
					t.Fatal(err)
				}
				if err := restored.Map.validate(restored.Board); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(g, restored.Board) || !reflect.DeepEqual(m, restored.Map) {
					t.Fatal("save round trip")
				}
				terrains[fmt.Sprint(terrain)] = true
				ports[fmt.Sprint(g.Ports)] = true
			}
			if len(terrains) < 2 || len(ports) < 2 {
				t.Fatal("variable setup not randomized")
			}
		})
	}
}

func TestCatanTransportRejectCorruptMap(t *testing.T) {
	cases := map[string]func(*Catan, *catanTransportMap){
		"missing-center": func(g *Catan, m *catanTransportMap) { g.Vertices = g.Vertices[:len(g.Vertices)-1] },
		"center-building": func(g *Catan, m *catanTransportMap) {
			g.Vertices[m.Sites[0].Center].Owner = 0
			g.Vertices[m.Sites[0].Center].Level = 1
		},
		"road-on-x":             func(g *Catan, m *catanTransportMap) { g.Edges[m.Sites[0].Blocked[0]].Owner = 0 },
		"spoke-to-other-center": func(g *Catan, m *catanTransportMap) { g.Edges[m.Sites[0].Paths[0]].B = m.Sites[1].Center },
		"duplicate-spoke":       func(g *Catan, m *catanTransportMap) { m.Sites[0].Paths[0] = m.Sites[0].Paths[1] },
		"wrong-kind":            func(g *Catan, m *catanTransportMap) { m.Sites[0].Kind = "castle" },
		"productive-commodity":  func(g *Catan, m *catanTransportMap) { g.Tiles[m.Sites[0].Tile].Number = 6 },
		"wrong-port":            func(g *Catan, m *catanTransportMap) { g.Ports[0].Edge = m.Sites[0].Paths[0] },
		"bad-coordinate":        func(g *Catan, m *catanTransportMap) { g.Vertices[0].X = math.NaN() },
		"wrong-gold":            func(g *Catan, m *catanTransportMap) { m.Gold++ },
		"wrong-barbarian":       func(g *Catan, m *catanTransportMap) { m.Barbarians[0] = m.Barbarians[1] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			g, m := transportBoard(t, 6)
			mutate(g, m)
			if m.validate(g) == nil {
				t.Fatal("corrupt board accepted")
			}
		})
	}
	for _, n := range []int{-1, 0, 1, 7} {
		if _, _, err := newCatanTransportBoard(n); err == nil {
			t.Fatal("invalid size", n)
		}
	}
}

func transportTravelFixture(t *testing.T, level int) (*Catan, *catanTransportMap, *catanTransportTravel, [3]int, []int) {
	t.Helper()
	g, m := transportBoard(t, 4)
	g.Bank = []int{19, 19, 19, 19, 19}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
	}
	q, err := newCatanTransportTravel(g, m, 0, g.Edges[m.Barbarians[0]].A, level)
	if err != nil {
		t.Fatal(err)
	}
	return g, m, q, m.Barbarians, []int{5, 5, 5, 5}
}
func TestCatanTransportMovementCostAndAtomicFailure(t *testing.T) {
	for _, c := range []struct {
		name               string
		owner              int
		barbarian, damaged bool
		mp, toll           int
	}{
		{"no road", -1, false, false, 2, 0}, {"own", 0, false, false, 1, 0}, {"other", 1, false, false, 1, 1},
		{"barbarian roadless", -1, true, false, 4, 0}, {"barbarian own", 0, true, false, 3, 0}, {"barbarian toll", 1, true, false, 3, 1},
		{"earthquake own", 0, false, true, 2, 0}, {"earthquake other", 1, true, true, 4, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, m, q, b, gold := transportTravelFixture(t, 4)
			edge := b[0]
			if !c.barbarian {
				for _, e := range g.Edges {
					if !slices.Contains(b[:], e.ID) && m.canBuildRoad(g, e.ID) {
						edge = e.ID
						break
					}
				}
				q.Position = g.Edges[edge].A
			}
			g.Edges[edge].Owner, g.Edges[edge].Damaged = c.owner, c.damaged
			// A rival building on the destination does not block the wagon.
			g.Vertices[g.Edges[edge].B].Owner = 2
			g.Vertices[g.Edges[edge].B].Level = 2
			old := *q
			quoted, err := q.quote(g, m, b, gold, edge)
			if err != nil || quoted.MP != c.mp || quoted.Toll != c.toll {
				t.Fatal(quoted, err)
			}
			if *q != old || sum(gold) != 20 {
				t.Fatal("quotation mutated state")
			}
			q.Points = c.mp - 1
			before, _ := json.Marshal([]any{q, g, gold, b})
			if _, err = q.move(g, m, b, gold, edge); err == nil {
				t.Fatal("partial-edge movement allowed")
			}
			after, _ := json.Marshal([]any{q, g, gold, b})
			if string(before) != string(after) {
				t.Fatal("failed movement changed state")
			}
			q.Points = 7
			step, err := q.move(g, m, b, gold, edge)
			if err != nil || step != quoted || q.Points != 7-c.mp || q.Position != g.Edges[edge].B || gold[0] != 5-c.toll || gold[1] != 5+c.toll || sum(gold) != 20 {
				t.Fatal(step, q, gold, err)
			}
		})
	}
	g, m, q, b, gold := transportTravelFixture(t, 4)
	g.Edges[b[0]].Owner = 1
	gold[0] = 0
	old := *q
	if _, err := q.move(g, m, b, gold, b[0]); err == nil || *q != old {
		t.Fatal("unpaid toll")
	}
	q.Position = g.Edges[m.Sites[0].Blocked[0]].A
	if _, err := q.move(g, m, b, gold, m.Sites[0].Blocked[0]); err != nil {
		t.Fatal("X forbids road construction, not wagon movement", err)
	}
}

func TestCatanTransportPlazaStopsAndWheatOnce(t *testing.T) {
	g, m, q, b, gold := transportTravelFixture(t, 4)
	g.Players[0].Resources[3] = 2
	g.Bank[3] -= 2
	if err := q.wheat(g, m, b, gold); err != nil {
		t.Fatal(err)
	}
	if q.Points != 9 || g.Players[0].Resources[3] != 1 || g.Bank[3] != 18 {
		t.Fatal("wheat payment")
	}
	if err := q.wheat(g, m, b, gold); err == nil {
		t.Fatal("second wheat")
	}
	path := m.Sites[0].Paths[0]
	q.Position = g.Edges[path].A
	if _, err := q.move(g, m, b, gold, path); err != nil {
		t.Fatal(err)
	}
	if !q.Ended || q.Points != 0 || q.Arrived != 0 || q.Position != m.Sites[0].Center {
		t.Fatal("must stop at plaza")
	}
	if _, err := q.move(g, m, b, gold, path); err == nil {
		t.Fatal("continued after arrival")
	}
	if err := q.wheat(g, m, b, gold); err == nil {
		t.Fatal("wheat after arrival")
	}
	// The next turn may depart normally. Merely starting at a plaza doesn't
	// count as a new arrival or an extra commodity pickup.
	next, err := newCatanTransportTravel(g, m, 0, q.Position, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = next.move(g, m, b, gold, path); err != nil || next.Ended || next.Arrived != -1 {
		t.Fatal("could not depart", err)
	}
	if err = next.stop(g, m, b, gold); err != nil || next.Points != 0 || !next.Ended || next.Arrived != -1 {
		t.Fatal("voluntary stop", err)
	}
}

func TestCatanTransportDriveOffAndRestore(t *testing.T) {
	for level := 1; level <= 4; level++ {
		for die := 1; die <= 6; die++ {
			g, m, q, b, gold := transportTravelFixture(t, level)
			beforeMP := q.Points
			success, err := q.driveOff(g, m, b, gold, 0, die)
			if err != nil || success != (die >= 7-level) || !q.Attempted[0] || q.Points != beforeMP {
				t.Fatal(level, die, success, err)
			}
			if !success {
				if _, err = q.driveOff(g, m, b, gold, 0, 6); err == nil {
					t.Fatal("failed attempt repeated")
				}
				continue
			}
			data, _ := json.Marshal(q)
			var restored catanTransportTravel
			if err = json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			q = &restored
			if err = q.validate(g, m, b, gold); err != nil {
				t.Fatal(err)
			}
			if _, err = q.move(g, m, b, gold, b[0]); err == nil {
				t.Fatal("move before relocation")
			}
			old := b
			if err = q.relocate(g, m, &b, gold, b[1]); err == nil || b != old || q.Pending != 0 {
				t.Fatal("relocated onto occupied edge")
			}
			dest := -1
			for _, e := range g.Edges {
				if m.canBuildRoad(g, e.ID) && !slices.Contains(b[:], e.ID) && (e.A == q.Position || e.B == q.Position) {
					dest = e.ID
					break
				}
			}
			if dest < 0 {
				t.Fatal("fixture needs adjacent unoccupied edge")
			}
			g.Edges[dest].Owner = 1
			g.Players[1].Resources[0] = 1
			g.Bank[0]--
			if err = q.relocate(g, m, &b, gold, dest); err != nil || q.Pending != -1 || b[0] != dest || g.Players[1].Resources[0] != 1 {
				t.Fatal("drive-off movement must not steal", err)
			}
			if _, err = q.driveOff(g, m, b, gold, 0, 6); err == nil {
				t.Fatal("same piece attempted twice at new edge")
			}
			if _, err = q.move(g, m, b, gold, old[0]); err != nil {
				t.Fatal("may continue after successful drive-off", err)
			}
		}
	}
	g, m, q, b, gold := transportTravelFixture(t, 0)
	if _, err := q.driveOff(g, m, b, gold, 0, 6); err == nil {
		t.Fatal("unupgraded wagon cannot fight")
	}
}

func TestCatanTransportUpgradeAndDeliveryRequirements(t *testing.T) {
	wantMP := []int{4, 5, 6, 7, 7}
	for level, mp := range wantMP {
		if catanTransportMovement(level) != mp {
			t.Fatal("wagon MP")
		}
	}
	wantCosts := [][]int{{1, 0, 1, 0, 1}, {1, 0, 1, 0, 1}, {2, 0, 1, 0, 1}, {2, 0, 1, 0, 1}}
	for level, cost := range wantCosts {
		if !slices.Equal(catanTransportUpgradeCost(level), cost) {
			t.Fatal("official upgrade icons")
		}
	}
	if catanTransportUpgradeCost(4) != nil || catanTransportUpgradeCost(-1) != nil || catanTransportMovement(5) != 0 {
		t.Fatal("invalid levels")
	}
	_, m := transportBoard(t, 6)
	for i, s := range m.Sites {
		for _, cargo := range []string{"sand", "marble", "glass", "tools", "wood", ""} {
			want := s.Kind == "quarry" && cargo == "tools" || s.Kind == "glassworks" && cargo == "sand" || s.Kind == "castle" && (cargo == "glass" || cargo == "marble")
			if m.accepts(i, cargo) != want {
				t.Fatal("delivery destination", s.Kind, cargo)
			}
		}
	}
}

func TestCatanTransportRejectCorruptTravelAndInvalidActions(t *testing.T) {
	for name, corrupt := range map[string]func(*catanTransportTravel){
		"wrong-player":     func(q *catanTransportTravel) { q.Player = 9 },
		"wrong-position":   func(q *catanTransportTravel) { q.Position = -1 },
		"wrong-level":      func(q *catanTransportTravel) { q.Level = 5 },
		"extra-mp":         func(q *catanTransportTravel) { q.Points = 8 },
		"negative-mp":      func(q *catanTransportTravel) { q.Points = -1 },
		"ended-with-mp":    func(q *catanTransportTravel) { q.Ended = true },
		"false-arrival":    func(q *catanTransportTravel) { q.Arrived = 0 },
		"unrolled-pending": func(q *catanTransportTravel) { q.Pending = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			g, m, q, b, gold := transportTravelFixture(t, 4)
			corrupt(q)
			if q.validate(g, m, b, gold) == nil {
				t.Fatal("corrupt persisted travel accepted")
			}
			before, _ := json.Marshal([]any{q, g, b, gold})
			if _, err := q.move(g, m, b, gold, b[0]); err == nil {
				t.Fatal("corrupt travel moved")
			}
			after, _ := json.Marshal([]any{q, g, b, gold})
			if string(before) != string(after) {
				t.Fatal("invalid action mutated state")
			}
		})
	}
	g, m, q, b, gold := transportTravelFixture(t, 4)
	before := *q
	for _, die := range []int{0, 7} {
		if _, err := q.driveOff(g, m, b, gold, 0, die); err == nil || *q != before {
			t.Fatal("invalid die consumed attempt")
		}
	}
	if _, err := q.driveOff(g, m, b, gold, 1, 6); err == nil || *q != before {
		t.Fatal("nonadjacent barbarian consumed attempt")
	}
	if err := q.wheat(g, m, b, gold); err == nil || *q != before {
		t.Fatal("free wheat")
	}
	if _, err := q.driveOff(g, m, b, gold, 0, 6); err != nil {
		t.Fatal(err)
	}
	if err := q.stop(g, m, b, gold); err == nil || q.Ended {
		t.Fatal("skipped required relocation")
	}
	blocked := m.Sites[0].Blocked[0]
	if err := q.relocate(g, m, &b, gold, blocked); err != nil || b[0] != blocked {
		t.Fatal("X prohibits roads, not barbarians", err)
	}
	q.Position = g.Edges[blocked].A
	q.Points = 4
	if step, err := q.move(g, m, b, gold, blocked); err != nil || step.MP != 4 {
		t.Fatal("roadless coastal edge plus barbarian costs four", step, err)
	}
}

func TestCatanTransportTwoPlayerRequiresNeutralRoadController(t *testing.T) {
	g, m := transportBoard(t, 2)
	if _, err := newCatanTransportTravel(g, m, 0, 0, 0); err == nil {
		t.Fatal("two-player geometry alone must not bypass the neutral-road controller")
	}
}
