package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCatanAttackTransportBoard(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for trial := 0; trial < 12; trial++ {
			g, b, err := newCatanAttackTransportBoard(n)
			if err != nil {
				t.Fatal(n, err)
			}
			if n <= 4 {
				if len(g.Tiles) != 19 || len(g.Vertices) != 57 || len(g.Edges) != 84 || g.Tiles[2].Number != 0 || g.Tiles[7].Number != 12 || g.Tiles[18].Number != 2 || g.Tiles[16].Resource != catanCastle {
					t.Fatal("official diagram differs")
				}
			} else if len(g.Tiles) != 37 || len(b.Transport.Sites) != 7 || len(b.Attack.Castles) != 2 {
				t.Fatal("extended diagram")
			}

			for _, site := range b.Transport.Sites {
				want := 0
				switch site.Kind {
				case "quarry":
					want = CatanDesert
					if g.Tiles[site.Tile].Number != 0 {
						t.Fatal("quarry has number")
					}
				case "castle":
					want = 2
				case "glassworks":
					want = 0
				}
				if b.productionResource(g.Tiles[site.Tile]) != want {
					t.Fatal("commodity produces wrong resource")
				}
				if site.Kind != "quarry" && g.Tiles[site.Tile].Number == 0 {
					t.Fatal("productive commodity has no number")
				}
			}
			p, err := newCatanAttackTransportPieces(g, b)
			if err != nil {
				t.Fatal(err)
			}
			initial := 2
			if n > 4 {
				initial = 3
			}
			if sum(p.counts(g)) != initial {
				t.Fatal("setup invaders", n, p.counts(g))
			}
			for _, x := range p.Barbarians {
				if x.Tile >= 0 && (x.Edge != -1 || !b.productiveCommodity(x.Tile)) {
					t.Fatal("initial piece blocks edge or occupies quarry")
				}
			}
			raw, _ := json.Marshal(b)
			var restored catanAttackTransportBoard
			if err = json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			if err = restored.validate(g); err != nil {
				t.Fatal(err)
			}
			bad := clone(*g)
			bad.Tiles[7].Number = 6
			if b.validate(&bad) == nil {
				t.Fatal("bad numbers accepted")
			}
			bad = clone(*g)
			bad.Edges[len(bad.Edges)-1].A = bad.Edges[len(bad.Edges)-1].B
			if b.validate(&bad) == nil {
				t.Fatal("bad spoke accepted")
			}
		}
	}
}
func TestCatanAttackTransportSharedBarbarians(t *testing.T) {
	for _, n := range []int{2, 6} {
		g, b, err := newCatanAttackTransportBoard(n)
		if err != nil {
			t.Fatal(err)
		}
		p, err := newCatanAttackTransportPieces(g, b)
		if err != nil {
			t.Fatal(err)
		}
		tile := b.Attack.Coast[0]
		var ids []int
		for p.counts(g)[tile] < 3 {
			id, e := p.land(g, b, tile)
			if e != nil {
				t.Fatal(e)
			}
			ids = append(ids, id)
		}
		before := clone(*p)
		if _, err = p.land(g, b, tile); err == nil || !reflect.DeepEqual(before, *p) {
			t.Fatal("conquered landing mutated")
		}
		id := ids[0]
		edge := p.Barbarians[id].Edge
		if edge < 0 || p.blocking(edge) != id {
			t.Fatal("hex piece not blocking edge")
		}
		inland := 9
		if n > 4 {
			inland = 18
		}
		choices := p.edges(g, inland, id)
		if len(choices) == 0 {
			t.Fatal("no inland destinations")
		}
		if err = p.relocate(g, b, id, inland, choices[0]); err != nil {
			t.Fatal(err)
		}
		if p.blocking(edge) >= 0 || p.counts(g)[tile] != 2 || p.counts(g)[inland] != 1 {
			t.Fatal("relocation desynchronized hex and edge")
		}
		if err = p.capture(g, b, id, 0); err != nil {
			t.Fatal(err)
		}
		if p.counts(g)[inland] != 0 || p.blocking(choices[0]) != -1 || p.Barbarians[id].Captor != 0 {
			t.Fatal("capture did not free edge")
		}
		raw, _ := json.Marshal(p)
		var next catanAttackTransportPieces
		if err = json.Unmarshal(raw, &next); err != nil {
			t.Fatal(err)
		}
		if err = next.validate(g, b); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCatanAttackTransportLandingNumbersAndShortage(t *testing.T) {
	for _, n := range []int{3, 6} {
		g, b, err := newCatanAttackTransportBoard(n)
		if err != nil {
			t.Fatal(err)
		}
		p, err := newCatanAttackTransportPieces(g, b)
		if err != nil {
			t.Fatal(err)
		}
		before := clone(*p)
		targets, short, err := p.landNumber(g, b, 7, nil)
		if err != nil || short || len(targets) != 0 || !reflect.DeepEqual(before, *p) {
			t.Fatal("seven moves invader")
		}
		targets, short, err = p.landNumber(g, b, 12, nil)
		want := 1
		if n > 4 {
			want = 2
		}
		if err != nil || short || len(targets) != want {
			t.Fatal("12 rerolled or wrong targets", targets, err)
		}
		for _, tile := range targets {
			if p.counts(g)[tile] != 2 {
				t.Fatal("landing count")
			}
		}
		if n == 6 {
			// Leave one supply piece; all other unused IDs are prisoners.
			spare := false
			for i, x := range p.Barbarians {
				if x.Tile == -1 && x.Captor == -1 {
					if !spare {
						spare = true
						continue
					}
					p.Barbarians[i].Captor = 0
				}
			}
			before = clone(*p)
			if _, _, err = p.landNumber(g, b, 12, func(n int) int { return n }); err == nil || !reflect.DeepEqual(before, *p) {
				t.Fatal("bad random choice mutated")
			}
			targets, short, err = p.landNumber(g, b, 12, func(n int) int { return n - 1 })
			if err != nil || !short || len(targets) != 1 || p.supply() != 0 || p.counts(g)[targets[0]] != 3 {
				t.Fatal("last piece", targets, short, err)
			}
			before = clone(*p)
			targets, short, err = p.landNumber(g, b, 12, nil)
			if err != nil || !short || len(targets) != 0 || !reflect.DeepEqual(before, *p) {
				t.Fatal("empty supply")
			}
		}
	}
}
