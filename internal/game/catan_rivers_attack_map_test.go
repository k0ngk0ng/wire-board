package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func TestCatanRiversAttackOfficialMap(t *testing.T) {
	for n := 2; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			variants := map[string]bool{}
			for range 24 {
				g, m, r, err := newCatanRiversAttackBoard(n)
				if err != nil {
					t.Fatal(err)
				}
				a, err := newCatanAttackPieces(g, m)
				if err != nil {
					t.Fatal(err)
				}
				count, bridges, bank := 19, 7, 100
				wantNumbers := []int{3, 8, 0, 9, 4, 5, 10, 12, 6, 10, 8, 11, 4, 9, 3, 5, 0, 6, 0}
				wantPaths := [][]int{{13, 9, 5, 2}, {11, 15, 18}}
				wantSwamps := []int{2, 18}
				wantCastles := []int{16}
				wantInitial := []int{7, 11}
				wantCounts := []int{3, 3, 3, 4, 3}
				if n > 4 {
					count, bridges, bank = 30, 10, 152
					wantNumbers = []int{12, 0, 3, 4, 5, 10, 8, 5, 11, 6, 3, 5, 0, 8, 10, 4, 6, 9, 0, 3, 8, 11, 9, 6, 4, 9, 10, 2, 0, 11}
					wantPaths = [][]int{{13, 19, 24, 28}, {8, 4, 1}, {15, 16, 17}}
					wantSwamps = []int{28, 1}
					wantCastles = []int{12, 18}
					wantInitial = []int{0, 27}
					wantCounts = []int{4, 5, 6, 6, 5}
				}
				if len(g.Tiles) != count || len(r.Bridges) != bridges || a.GoldBank != bank || g.Robber != -1 || !slices.Equal(r.Swamps, wantSwamps) || !slices.Equal(m.Castles, wantCastles) {
					t.Fatal("wrong map components")
				}
				for i, p := range wantPaths {
					if !slices.Equal(r.Channels[i].Tiles, p) {
						t.Fatal("wrong printed river path")
					}
				}
				counts := make([]int, 5)
				numbers := []int{}
				initial := []int{}
				terrain := []int{}
				for _, tile := range g.Tiles {
					terrain = append(terrain, tile.Resource)
					numbers = append(numbers, tile.Number)
					if tile.Resource < 5 {
						counts[tile.Resource]++
					}
					if a.Barbarians[tile.ID] > 0 {
						initial = append(initial, tile.ID)
					}
				}
				if !slices.Equal(counts, wantCounts) || !slices.Equal(numbers, wantNumbers) || !slices.Equal(initial, wantInitial) {
					t.Fatal("printed numbers, terrain or initial barbarians", counts, numbers, initial)
				}
				if n <= 4 && r.DoubleNumberTile != 7 || n > 4 && r.DoubleNumberTile != -1 {
					t.Fatal("double number placement")
				}
				for _, c := range r.Channels {
					if len(g.Edges[c.Outlet].Tiles) != 1 {
						t.Fatal("inland river mouth")
					}
				}
				for _, p := range g.Ports {
					if slices.Contains(r.Bridges, p.Edge) {
						t.Fatal("port overlaps bridge")
					}
				}
				if a.supply() != m.Barbarians-2 {
					t.Fatal("wrong initial supply")
				}
				data, err := json.Marshal(struct {
					Board  *Catan
					Attack *catanAttack
					Rivers *catanRiversMap
				}{g, a, r})
				if err != nil {
					t.Fatal(err)
				}
				var restored struct {
					Board  *Catan
					Attack *catanAttack
					Rivers *catanRiversMap
				}
				if err = json.Unmarshal(data, &restored); err != nil {
					t.Fatal(err)
				}
				if err = restored.Attack.validate(restored.Board); err != nil {
					t.Fatal(err)
				}
				variants[fmt.Sprint(terrain)] = true
			}
			if len(variants) < 2 {
				t.Fatal("random terrain was fixed")
			}
		})
	}
}

func TestCatanRiversAttackMapRejectsCorruption(t *testing.T) {
	mutations := map[string]func(*Catan, *catanAttackMap){
		"number":   func(g *Catan, m *catanAttackMap) { g.Tiles[7].Number = 2 },
		"river":    func(g *Catan, m *catanAttackMap) { g.Tiles[13].Resource = 0 },
		"coast":    func(g *Catan, m *catanAttackMap) { m.Coast[0] = 13 },
		"castle":   func(g *Catan, m *catanAttackMap) { g.Tiles[16].Resource = 0 },
		"edge":     func(g *Catan, m *catanAttackMap) { g.Edges[0].A = g.Edges[0].B },
		"vertex":   func(g *Catan, m *catanAttackMap) { g.Vertices[0].X += 1 },
		"index":    func(g *Catan, m *catanAttackMap) { g.Tiles[0].Vertices[0] = -1 },
		"port":     func(g *Catan, m *catanAttackMap) { g.Ports[0].Edge = g.Ports[1].Edge },
		"marker":   func(g *Catan, m *catanAttackMap) { m.Rivers = "unknown" },
		"unmarked": func(g *Catan, m *catanAttackMap) { m.Rivers = "" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			g, m, _, err := newCatanRiversAttackBoard(3)
			if err != nil {
				t.Fatal(err)
			}
			mutate(g, m)
			if err = m.validate(g); err == nil {
				t.Fatal("accepted malformed map")
			}
		})
	}
}

func TestCatanRiversAttackLandingNumbers(t *testing.T) {
	g, m, _, err := newCatanRiversAttackBoard(3)
	if err != nil {
		t.Fatal(err)
	}
	for number := 2; number <= 12; number++ {
		count := 0
		for _, id := range m.Coast {
			if m.landingNumber(g, id, number) {
				count++
			}
		}
		want := 1
		if number == 7 {
			want = 0
		}
		if count != want {
			t.Fatalf("number %d: %d landing hexes, want %d", number, count, want)
		}
	}
	// Ordinary Attack's 12 hex must never acquire the combination's extra 2.
	base, bm, err := newCatanAttackBoard(3)
	if err != nil {
		t.Fatal(err)
	}
	if bm.landingNumber(base, 7, 2) || !bm.landingNumber(base, 7, 12) {
		t.Fatal("base landing changed")
	}
	if m.landingNumber(g, -1, 2) || m.landingNumber(g, len(g.Tiles), 2) {
		t.Fatal("invalid target accepted")
	}
}

func TestCatanRiversAttackBoardRequiresRiverComponent(t *testing.T) {
	s, err := NewCatanAttack(3)
	if err != nil {
		t.Fatal(err)
	}
	g, m, _, err := newCatanRiversAttackBoard(3)
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.Tiles, s.Catan.Vertices, s.Catan.Edges, s.Catan.Ports = g.Tiles, g.Vertices, g.Edges, g.Ports
	s.Catan.Attack, err = newCatanAttackPieces(s.Catan, m)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.validateCatanAttack(); err == nil {
		t.Fatal("unfinished combination accepted as session")
	}
}
