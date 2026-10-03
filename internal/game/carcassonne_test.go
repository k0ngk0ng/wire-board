package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func carTestTile(letter string, x, y, r, owner, f int) CarTile {
	for k, d := range carDefinitions {
		if d.Name == letter {
			return CarTile{X: x, Y: y, Rotation: r, Kind: k, Art: d.Arts[0], Owner: owner, Feature: f}
		}
	}
	panic(letter)
}
func carTestState(tiles ...CarTile) *State {
	return &State{Kind: "carcassonne", Phase: "car_tile", Carcassonne: &Carcassonne{Tiles: tiles, Players: []CarPlayer{{Meeples: 7}, {Meeples: 7}, {Meeples: 7}}, Current: -1, Last: len(tiles) - 1}}
}
func TestCarcassonneCatalogAndRotation(t *testing.T) {
	counts := []int{2, 4, 1, 4, 5, 2, 1, 3, 2, 3, 3, 3, 2, 3, 2, 3, 1, 3, 2, 1, 8, 9, 4, 1}
	arts := map[int]bool{}
	for k, d := range carDefinitions {
		if len(d.Arts) != counts[k] {
			t.Fatal(d.Name, "count")
		}
		ports := map[int]bool{}
		for f, feature := range d.Features {
			for _, p := range feature.Ports {
				if ports[p] || p < 0 || p > 11 {
					t.Fatal(d.Name, "duplicate port", p)
				}
				ports[p] = true
				for r := 0; r < 4; r++ {
					tile := CarTile{Kind: k, Rotation: r}
					if carPortFeature(tile, (p+r*3)%12) != f {
						t.Fatal("rotation")
					}
				}
			}
			for _, city := range feature.Cities {
				if d.Features[city].Kind != "city" {
					t.Fatal("field adjacency")
				}
			}
		}
		for d := 0; d < 4; d++ {
			if !ports[d*3+1] {
				t.Fatal("missing edge center")
			}
		}
		for _, a := range d.Arts {
			if arts[a] {
				t.Fatal("duplicate art")
			}
			arts[a] = true
			if carKind(a) != k {
				t.Fatal("art type")
			}
		}
	}
	if len(arts) != 72 {
		t.Fatal(len(arts))
	}
	for n := 2; n <= 5; n++ {
		s, e := New("carcassonne", n)
		if e != nil || len(s.Carcassonne.Deck) != 70 || s.Carcassonne.Tiles[0].Art != 36 {
			t.Fatal("setup", e)
		}
	}
	for _, n := range []int{1, 6} {
		if _, e := New("carcassonne", n); e == nil {
			t.Fatal("capacity")
		}
	}
}
func TestCarcassonneConnectionsAndScores(t *testing.T) {
	t.Run("city shield and two tile city", func(t *testing.T) {
		s := carTestState(carTestTile("E", 0, 0, 0, 0, 0), carTestTile("E", 0, -1, 2, -1, -1))
		s.carScore(false)
		if s.Carcassonne.Players[0].Score != 4 || s.Carcassonne.Tiles[0].Owner != -1 {
			t.Fatal("two tile city")
		}
		s = carTestState(carTestTile("Q", 0, 0, 0, 0, 0), carTestTile("E", 0, -1, 2, -1, -1), carTestTile("E", 1, 0, 3, -1, -1), carTestTile("E", -1, 0, 1, -1, -1))
		s.carScore(false)
		if s.Carcassonne.Players[0].Score != 10 {
			t.Fatal("shield scoring")
		}
	})
	t.Run("road loop", func(t *testing.T) {
		s := carTestState(carTestTile("V", 0, 0, 3, 0, 0), carTestTile("V", 1, 0, 0, -1, -1), carTestTile("V", 1, 1, 1, -1, -1), carTestTile("V", 0, 1, 2, -1, -1))
		s.carScore(false)
		if s.Carcassonne.Players[0].Score != 4 {
			t.Fatal("loop", s.Carcassonne.Players)
		}
	})
	t.Run("junction roads remain separate", func(t *testing.T) {
		g := carTestState(carTestTile("X", 0, 0, 0, -1, -1)).Carcassonne
		for f := 0; f < 4; f++ {
			c := g.component(carNode{0, f}, g.positions())
			if len(c.Nodes) != 1 || c.Open != 1 {
				t.Fatal(c)
			}
		}
	})
	t.Run("majority tie and all followers returned", func(t *testing.T) {
		s := carTestState(carTestTile("E", 0, 0, 0, 0, 0), carTestTile("E", 0, -1, 2, 1, 0))
		s.carScore(false)
		for i := 0; i < 2; i++ {
			if s.Carcassonne.Players[i].Score != 4 || s.Carcassonne.Tiles[i].Owner != -1 {
				t.Fatal("tie")
			}
		}
		s.carScore(true)
		if s.Carcassonne.Players[0].Score != 4 {
			t.Fatal("double score")
		}
	})
	t.Run("incomplete structures", func(t *testing.T) {
		s := carTestState(carTestTile("Q", 0, 0, 0, 0, 0), carTestTile("B", 2, 0, 0, 1, 0), carTestTile("U", 3, 0, 0, 2, 0))
		s.carScore(false)
		if s.Carcassonne.Players[0].Score != 0 {
			t.Fatal("premature score")
		}
		s.carScore(true)
		if s.Carcassonne.Players[0].Score != 2 || s.Carcassonne.Players[1].Score != 2 || s.Carcassonne.Players[2].Score != 1 {
			t.Fatal(s.Carcassonne.Players)
		}
	})
	t.Run("monastery diagonal completion", func(t *testing.T) {
		s := carTestState(carTestTile("B", 0, 0, 0, 0, 0))
		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				if x != 0 || y != 0 {
					s.Carcassonne.Tiles = append(s.Carcassonne.Tiles, carTestTile("B", x, y, 0, -1, -1))
				}
			}
		}
		s.carScore(false)
		if s.Carcassonne.Players[0].Score != 9 {
			t.Fatal(s.Carcassonne.Players)
		}
	})
}
func TestCarcassonneFarmTopologyAndScoring(t *testing.T) {
	// A city spanning east-west splits north and south farms. A monastery's
	// terminating road does not split its surrounding field. Separate cities do not join.
	for _, tc := range []struct {
		kind   string
		fields int
	}{{"F", 2}, {"A", 1}, {"D", 2}, {"L", 3}, {"X", 4}} {
		tile := carTestTile(tc.kind, 0, 0, 0, -1, -1)
		n := 0
		for _, f := range carDefinitions[tile.Kind].Features {
			if f.Kind == "field" {
				n++
			}
		}
		if n != tc.fields {
			t.Fatal(tc.kind)
		}
	}
	s := carTestState(carTestTile("H", 0, 0, 0, -1, -1))
	g := s.Carcassonne
	if len(g.component(carNode{0, 0}, g.positions()).Nodes) != 1 {
		t.Fatal("separate cities connected")
	}
	// Two farmers on different sides merge around the city. Both get three,
	// although their field touches both tiles of the same completed city.
	s = carTestState(carTestTile("E", 0, 0, 0, 0, 1), carTestTile("E", 0, -1, 2, 1, 1), carTestTile("B", 1, 0, 0, -1, -1), carTestTile("B", 1, -1, 0, -1, -1))
	g = s.Carcassonne
	c := g.component(carNode{0, 1}, g.positions())
	if g.fieldCities(c, g.positions()) != 1 || c.Followers[0] != 1 || c.Followers[1] != 1 {
		t.Fatal("farm join or unique city", c)
	}
	s.carScore(false)
	if g.Tiles[0].Owner != 0 {
		t.Fatal("farmer returned early")
	}
	s.carScore(true)
	if g.Players[0].Score != 3 || g.Players[1].Score != 3 {
		t.Fatal("farm tie", g.Players)
	}
	// Three meeples in one field: 2 beats 1; incomplete cities are worthless.
	s = carTestState(carTestTile("E", 0, 0, 0, 0, 1), carTestTile("E", 0, -1, 2, 1, 1), carTestTile("B", 1, 0, 0, 0, 1), carTestTile("B", 1, -1, 0, -1, -1), carTestTile("E", 2, 0, 0, -1, -1))
	s.carScore(true)
	if s.Carcassonne.Players[0].Score != 3 || s.Carcassonne.Players[1].Score != 0 {
		t.Fatal("farm majority")
	}
}
func TestCarcassonneActionsPrivacyPersistenceAndElimination(t *testing.T) {
	s, _ := New("carcassonne", 3)
	g := s.Carcassonne
	assertRejected := func(p int, a Action) {
		t.Helper()
		before, _ := json.Marshal(s)
		if s.Apply(p, a) == nil {
			t.Fatal("accepted invalid", a)
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("invalid mutation")
		}
	}
	assertRejected(1, Action{Type: "car_place"})
	assertRejected(0, Action{Type: "car_place", X: 100, Y: 100})
	assertRejected(0, Action{Type: "car_place", Rotation: 4})
	assertRejected(0, Action{Type: "car_meeple"})
	for _, p := range []int{-1, 0, 1} {
		v := s.View(p)["carcassonne"].(map[string]any)
		if _, ok := v["deck"]; ok {
			t.Fatal("deck exposed")
		}
		if p != 0 && len(v["legal"].([]CarPlacement)) > 0 {
			t.Fatal("non-player actions")
		}
	}
	a, _ := s.BotAction(0)
	if e := s.Apply(0, a); e != nil {
		t.Fatal(e)
	}
	assertRejected(0, Action{Type: "car_place"})
	assertRejected(0, Action{Type: "car_meeple", Feature: 999})
	assertRejected(0, Action{Type: "car_meeple", Feature: -2})
	data, _ := json.Marshal(s)
	var restored State
	if e := json.Unmarshal(data, &restored); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s.View(0), restored.View(0)) {
		t.Fatal("persistence")
	}
	tiles := len(g.Tiles)
	if e := s.EliminateCarcassonne(0); e != nil || s.Turn != 1 || len(g.Tiles) != tiles {
		t.Fatal("eliminate after placement", e)
	}
	if e := s.EliminateCarcassonne(1); e != nil || !s.Finished || !reflect.DeepEqual(s.Winners, []int{2}) {
		t.Fatal("last player")
	}
}
func TestCarcassonneNoMeepleOnOccupiedFeature(t *testing.T) {
	s := carTestState(carTestTile("E", 0, 0, 0, 1, 0), carTestTile("E", 0, -1, 2, -1, -1))
	s.Phase = "car_meeple"
	if e := s.Apply(0, Action{Type: "car_meeple", Feature: 0}); e == nil {
		t.Fatal("occupied complete city allowed")
	}
	s.Carcassonne.Players[0].Meeples = 0
	if e := s.Apply(0, Action{Type: "car_meeple", Feature: 1}); e == nil {
		t.Fatal("no reserve")
	}
	if e := s.Apply(0, Action{Type: "car_meeple", Feature: -1}); e != nil {
		t.Fatal(e)
	}
}
func TestCarcassonneImpossibleTileAndFinalTies(t *testing.T) {
	s := carTestState(carTestTile("B", 0, 0, 0, -1, -1))
	g := s.Carcassonne
	g.Deck = []int{67, 61}
	s.carDraw()
	if g.Current != 61 || !reflect.DeepEqual(g.Discarded, []int{67}) {
		t.Fatal("unplaceable tile")
	}
	g.Current = -1
	g.Deck = nil
	s.carDraw()
	if !s.Finished || len(s.Winners) != 3 {
		t.Fatal("tie")
	}
}
func TestCarcassonneFullBotGamesAndConservation(t *testing.T) {
	for _, n := range []int{2, 3, 5} {
		s, _ := New("carcassonne", n)
		steps := 0
		for !s.Finished && steps < 150 {
			a, e := s.BotAction(s.Turn)
			if e != nil {
				t.Fatal(e)
			}
			if steps%13 == 0 {
				other := clone(*s)
				shuffle(other.Carcassonne.Deck)
				b, e := other.BotAction(other.Turn)
				if e != nil || !reflect.DeepEqual(a, b) {
					t.Fatal("bot reads future deck")
				}
			}
			if e = s.Apply(s.Turn, a); e != nil {
				t.Fatal(e)
			}
			steps++
			g := s.Carcassonne
			for i, p := range g.Players {
				used := 0
				for _, tile := range g.Tiles {
					if tile.Owner == i {
						used++
					}
				}
				if used+p.Meeples != 7 {
					t.Fatal("meeple conservation")
				}
			}
			remaining := len(g.Deck) + len(g.Tiles) + len(g.Discarded)
			if g.Current >= 0 {
				remaining++
			}
			if remaining != 72 {
				t.Fatal("tile conservation", remaining)
			}
		}
		if !s.Finished || len(s.Winners) == 0 {
			t.Fatal("did not finish", n, steps)
		}
	}
}

func TestCarcassonneJoinedJunctionCountsTileOnce(t *testing.T) {
	s := carTestState(carTestTile("X", 0, 0, 0, 0, 0), carTestTile("V", 0, -1, 3, -1, -1), carTestTile("V", 1, -1, 0, -1, -1), carTestTile("V", 1, 0, 1, -1, -1))
	g := s.Carcassonne
	c := g.component(carNode{0, 0}, g.positions())
	if c.Open != 0 || len(c.Nodes) != 5 || c.Tiles != 4 {
		t.Fatal("merged branches on same tile", c)
	}
	s.carScore(false)
	if g.Players[0].Score != 4 {
		t.Fatal("junction tile counted twice")
	}
}
func TestCarcassonneMatchingEdgesHaveCompatibleFieldFlanks(t *testing.T) {
	for k := range carDefinitions {
		for other := range carDefinitions {
			for r := 0; r < 4; r++ {
				for or := 0; or < 4; or++ {
					a, b := CarTile{Kind: k, Rotation: r}, CarTile{Kind: other, Rotation: or}
					for d := 0; d < 4; d++ {
						af, bf := carPortFeature(a, d*3+1), carPortFeature(b, ((d+2)%4)*3+1)
						if carDefinitions[k].Features[af].Kind != carDefinitions[other].Features[bf].Kind {
							continue
						}
						for _, side := range []int{0, 2} {
							fa, fb := carPortFeature(a, d*3+side), carPortFeature(b, ((d+2)%4)*3+2-side)
							if (fa < 0) != (fb < 0) || (fa >= 0 && carDefinitions[k].Features[fa].Kind != carDefinitions[other].Features[fb].Kind) {
								t.Fatal("mismatched flank", k, other, r, or, d, side)
							}
						}
					}
				}
			}
		}
	}
}
