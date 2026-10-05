package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"testing"
)

func caravanBoard(t *testing.T, n int) (*Catan, *catanCaravanMap) {
	t.Helper()
	s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Catan.makeCaravansMap()
	if err != nil {
		t.Fatal(err)
	}
	return s.Catan, f
}
func TestCatanCaravansPrintedMaps(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			varied := map[string]bool{}
			for range 24 {
				s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
				if err != nil {
					t.Fatal(err)
				}
				before := clone(*s.Catan)
				f, err := s.Catan.makeCaravansMap()
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if err = f.validate(g); err != nil {
					t.Fatal(err)
				}
				wantHoles := []int{9}
				wantSupply := 22
				wantPorts := 9
				wantCounts := []int{4, 3, 4, 4, 3}
				if n > 4 {
					wantHoles = []int{8, 21}
					wantSupply = 33
					wantPorts = 11
					wantCounts = []int{6, 5, 6, 6, 5}
				}
				if !slices.Equal(f.WateringHoles, wantHoles) || f.Supply != wantSupply || len(f.Starts) != 3*len(wantHoles) || g.Robber != -1 || len(g.Ports) != wantPorts {
					t.Fatal("printed layout", f)
				}
				terrain := []int{}
				counts := make([]int, 5)
				numbers := make([]int, 13)
				for _, tile := range g.Tiles {
					terrain = append(terrain, tile.Resource)
					if tile.Resource < 5 {
						counts[tile.Resource]++
						numbers[tile.Number]++
					} else if !slices.Contains(wantHoles, tile.ID) || tile.Number != 0 {
						t.Fatal("unexpected nonproductive hex")
					}
				}
				wantNumbers := []int{0, 0, 1, 2, 2, 2, 2, 0, 2, 2, 2, 2, 1}
				if n > 4 {
					wantNumbers = []int{0, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2}
				}
				if !slices.Equal(counts, wantCounts) || !slices.Equal(numbers, wantNumbers) {
					t.Fatal("inventory")
				}
				for _, e := range g.Edges {
					if len(e.Tiles) == 2 {
						a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
						if (a == 6 || a == 8) && (b == 6 || b == 8) {
							t.Fatal("adjacent red numbers")
						}
					}
				}
				for _, root := range f.Starts {
					edge := g.Edges[root.Edge]
					if edge.A != root.From && edge.B != root.From {
						t.Fatal("start direction not an endpoint")
					}
					for _, h := range wantHoles {
						if slices.Contains(g.Tiles[h].Vertices, root.From) {
							if slices.Contains(edge.Tiles, h) {
								t.Fatal("start follows watering perimeter")
							}
							corner := slices.Index(g.Tiles[h].Vertices, root.From)
							if !slices.Contains([]int{1, 3, 5}, corner) {
								t.Fatal("unprinted origin")
							}
						}
					}
				}
				if !reflect.DeepEqual(before.Players, g.Players) || !reflect.DeepEqual(before.Bank, g.Bank) || !reflect.DeepEqual(before.DevDeck, g.DevDeck) || !reflect.DeepEqual(before.Paired, g.Paired) || g.Options != before.Options {
					t.Fatal("map changed non-map state")
				}
				data, _ := json.Marshal(struct {
					Game *Catan
					Map  *catanCaravanMap
				}{g, f})
				var restored struct {
					Game *Catan
					Map  *catanCaravanMap
				}
				if err = json.Unmarshal(data, &restored); err != nil {
					t.Fatal(err)
				}
				if err = restored.Map.validate(restored.Game); err != nil {
					t.Fatal(err)
				}
				key, _ := json.Marshal(terrain)
				varied[string(key)] = true
			}
			if len(varied) < 2 {
				t.Fatal("ordinary terrain not randomized")
			}
		})
	}
}

func TestCatanCaravansDirectionsMergesAndSupply(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			g, f := caravanBoard(t, n)
			rng := rand.New(rand.NewSource(2041))
			coverage := map[string]bool{}
			// Deterministic choices on the real hex graph cover convergence, return to
			// the watering hole, coastal paths and finite inventory. No branch fixtures.
			for sample := 0; sample < 120; sample++ {
				c := &catanCaravans{Map: f}
				if !reflect.DeepEqual(c.choices(g), c.choices(g)) {
					t.Fatal("unstable choice order")
				}
				for step := 0; step < f.Supply; step++ {
					choices := c.choices(g)
					if len(choices) == 0 {
						coverage["blocked"] = true
						break
					}
					before, _ := json.Marshal(c)
					for _, w := range c.Wagons {
						if err := c.place(g, w); err == nil {
							t.Fatal("repeat wagon accepted")
						}
					}
					after, _ := json.Marshal(c)
					if string(before) != string(after) {
						t.Fatal("rejected placement mutated routes")
					}
					selected := choices[rng.Intn(len(choices))]
					e := g.Edges[selected.Edge]
					to := e.A
					if to == selected.From {
						to = e.B
					}
					incoming, outgoing := map[int]int{}, map[int]int{}
					for _, w := range c.Wagons {
						edge := g.Edges[w.Edge]
						end := edge.A
						if end == w.From {
							end = edge.B
						}
						incoming[end]++
						outgoing[w.From]++
					}
					if outgoing[selected.From] > 0 {
						t.Fatal("branch advertised")
					}
					if incoming[to] > 0 && outgoing[to] == 0 {
						coverage["head-merge"] = true
					}
					if incoming[to] > 0 && outgoing[to] > 0 {
						coverage["merge-existing"] = true
					}
					if incoming[selected.From] == 2 {
						coverage["shared-extension"] = true
					}
					if len(e.Tiles) == 1 {
						coverage["coast"] = true
					}
					for _, h := range f.WateringHoles {
						if slices.Contains(e.Tiles, h) {
							coverage["watering-perimeter"] = true
						}
					}
					// Roads and all players' buildings do not obstruct this public network.
					g.Edges[selected.Edge].Owner = sample % n
					g.Vertices[selected.From].Owner = (sample + 1) % n
					g.Vertices[selected.From].Level = 1
					if err := c.place(g, selected); err != nil {
						t.Fatal(err)
					}
					if c.roadWeight(selected.Edge) != 2 {
						t.Fatal("parallel road weight")
					}
					for _, next := range c.choices(g) {
						if next.From == selected.From {
							t.Fatal("branch after placement")
						}
					}
					for _, v := range g.Vertices {
						degree := 0
						for _, w := range c.Wagons {
							e := g.Edges[w.Edge]
							if e.A == v.ID || e.B == v.ID {
								degree++
							}
						}
						want := 0
						if degree >= 2 {
							want = 1
						}
						if c.buildingBonus(g, v.ID) != want {
							t.Fatal("building between wagons bonus")
						}
					}
					copy := clone(*c)
					if err := copy.validate(g); err != nil {
						t.Fatal("restore", err)
					}
					if !reflect.DeepEqual(c.choices(g), copy.choices(g)) {
						t.Fatal("restore lost direction")
					}
				}
				if len(c.Wagons) == f.Supply {
					coverage["supply"] = true
					if len(c.choices(g)) != 0 {
						t.Fatal("overdraw")
					}
				}
			}
			for _, event := range []string{"head-merge", "merge-existing", "shared-extension", "coast", "watering-perimeter", "supply"} {
				if !coverage[event] {
					t.Fatal("uncovered direction", event, coverage)
				}
			}
			t.Logf("coverage=%v", coverage)
		})
	}
}

func TestCatanCaravansRejectCorruptMapsAndRoutes(t *testing.T) {
	g, f := caravanBoard(t, 4)
	for _, mutate := range []func(*Catan, *catanCaravanMap){
		func(g *Catan, f *catanCaravanMap) { f.Supply++ },
		func(g *Catan, f *catanCaravanMap) { f.WateringHoles[0] = 8 },
		func(g *Catan, f *catanCaravanMap) { f.Starts[0].From = -1 },
		func(g *Catan, f *catanCaravanMap) { g.Tiles[9].Number = 5 },
		func(g *Catan, f *catanCaravanMap) { g.Tiles[9].Resource = 5 },
		func(g *Catan, f *catanCaravanMap) { g.Vertices[0].X++ },
		func(g *Catan, f *catanCaravanMap) { g.Edges[0].A = -1 },
		func(g *Catan, f *catanCaravanMap) { g.Ports[0].Resource = 9 },
	} {
		copyG, copyF := clone(*g), clone(*f)
		mutate(&copyG, &copyF)
		if copyF.validate(&copyG) == nil {
			t.Fatal("corrupt map accepted")
		}
	}
	c := catanCaravans{Map: f}
	start := f.Starts[0]
	e := g.Edges[start.Edge]
	reverse := e.A
	if reverse == start.From {
		reverse = e.B
	}
	for _, bad := range []catanCaravanWagon{{start.Edge, reverse}, {-1, start.From}, {start.Edge, -1}, {0, g.Edges[0].A}} {
		before := clone(c)
		if c.place(g, bad) == nil {
			t.Fatal("bad initial direction accepted", bad)
		}
		if !reflect.DeepEqual(c, before) {
			t.Fatal("rejection changed route")
		}
		saved := catanCaravans{Map: f, Wagons: []catanCaravanWagon{bad}}
		if saved.validate(g) == nil {
			t.Fatal("bad saved route accepted")
		}
	}
	if err := c.place(g, start); err != nil {
		t.Fatal(err)
	}
	dup := clone(c)
	dup.Wagons = append(dup.Wagons, start)
	if dup.validate(g) == nil {
		t.Fatal("duplicate saved wagon accepted")
	}
	for _, mode := range []string{"building", "road", "helpers", "repeat", "fishing", "rivers"} {
		s, err := NewCatan(4, CatanOptions{})
		if err != nil {
			t.Fatal(err)
		}
		switch mode {
		case "building":
			s.Catan.Vertices[0].Level = 1
		case "road":
			s.Catan.Edges[0].Owner = 0
		case "helpers":
			s.Catan.Options.Helpers = true
		case "repeat":
			_, err = s.Catan.makeCaravansMap()
		case "fishing":
			s.Catan.Fishing = &CatanFishing{}
		case "rivers":
			s.Catan.Rivers = &CatanRivers{}
		}
		if err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(s.Catan)
		if _, err = s.Catan.makeCaravansMap(); err == nil {
			t.Fatal("incompatible board", mode)
		}
		after, _ := json.Marshal(s.Catan)
		if string(before) != string(after) {
			t.Fatal("rejected map mutated", mode)
		}
	}
}
