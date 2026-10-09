package game

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"testing"
)

func TestCatanRiversTransportMap(t *testing.T) {
	for n := 2; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			variations := map[string]bool{}
			for range 24 {
				g, m, r, err := newCatanRiversTransportBoard(n)
				if err != nil {
					t.Fatal(err)
				}
				count, sites, bridges := 19, 3, 7
				if n > 4 {
					count, sites, bridges = 37, 7, 10
				}
				if len(g.Tiles) != count || len(m.Sites) != sites || len(r.Bridges) != bridges || len(r.Swamps) != 2 {
					t.Fatal("wrong component count")
				}
				if r.DoubleNumberTile < 0 || g.Tiles[r.DoubleNumberTile].Number != 12 {
					t.Fatal("missing double number")
				}
				for _, site := range m.Sites {
					if g.Tiles[site.Tile].Number == 0 {
						t.Fatal("commodity site cannot produce")
					}
					for _, edge := range site.Blocked {
						if slices.Contains(r.Bridges, edge) {
							t.Fatal("crossed edge overlaps bridge")
						}
					}
				}
				for _, port := range g.Ports {
					if slices.Contains(r.Bridges, port.Edge) {
						t.Fatal("port overlaps bridge")
					}
				}
				for i, ch := range r.Channels {
					for _, other := range r.Channels[:i] {
						for _, a := range ch.Tiles {
							for _, b := range other.Tiles {
								if math.Hypot(g.Tiles[a].X-g.Tiles[b].X, g.Tiles[a].Y-g.Tiles[b].Y) < math.Sqrt(3)*g.HexSize+.01 {
									t.Fatal("river components touch")
								}
							}
						}
					}
					first, last := g.Tiles[ch.Tiles[0]], g.Tiles[ch.Tiles[len(ch.Tiles)-1]]
					for j, id := range ch.Tiles {
						v := g.Tiles[id]
						scale := float64(j) / float64(len(ch.Tiles)-1)
						if math.Hypot(v.X-first.X-(last.X-first.X)*scale, v.Y-first.Y-(last.Y-first.Y)*scale) > .01 {
							t.Fatal("artwork cannot align")
						}
					}
				}
				raw, _ := json.Marshal(struct {
					G *Catan
					M *catanTransportMap
					R *catanRiversMap
				}{g, m, r})
				variations[string(raw)] = true
				var restored struct {
					G *Catan
					M *catanTransportMap
					R *catanRiversMap
				}
				if err = json.Unmarshal(raw, &restored); err != nil {
					t.Fatal(err)
				}
				if err = restored.M.validate(restored.G); err != nil {
					t.Fatal(err)
				}
				for _, mutate := range []func(*Catan, *catanTransportMap){
					func(g *Catan, m *catanTransportMap) { g.Tiles[r.Channels[0].Tiles[0]].Resource = 0 },
					func(g *Catan, m *catanTransportMap) { g.Tiles[r.Swamps[0]].Number = 6 },
					func(g *Catan, m *catanTransportMap) { g.Tiles[m.Sites[0].Tile].Number = 0 },
					func(g *Catan, m *catanTransportMap) { m.Rivers = "invalid" },
					func(g *Catan, m *catanTransportMap) { m.Rivers = "" },
					func(g *Catan, m *catanTransportMap) { g.Edges[m.Sites[0].Paths[0]].A++ },
				} {
					bad, board := clone(*m), clone(*g)
					mutate(&board, &bad)
					if bad.validate(&board) == nil {
						t.Fatal("bad combined map accepted")
					}
				}
			}
			if len(variations) < 2 {
				t.Fatal("terrain never randomized")
			}
		})
	}
}
