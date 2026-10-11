package game

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

// Failure scenarios, before changing validation:
//   - ARM64 saves fail on amd64 because generated display coordinates differ by
//     a few ULPs; validation must accept those without rewriting the saved board.
//   - Tolerating display roundoff must not admit NaN/Inf, visible coordinate
//     movement, tile IDs/numbers/terrain, vertex IDs/harbors or edge rewiring.
//   - The same comparison is used by fixed, variable and extended river maps.
func TestCatanRiversGeometryRestore(t *testing.T) {
	recipes := map[string]func() (*State, error){
		"shores":     func() (*State, error) { return newCatanRiversShores(4) },
		"fog":        func() (*State, error) { return newCatanRiversPrintedSea(4, "fog") },
		"desert":     func() (*State, error) { return newCatanRiversPrintedSea(4, "desert") },
		"tribe":      func() (*State, error) { return newCatanRiversTribe(4) },
		"world":      func() (*State, error) { return newCatanRiversWorld(4) },
		"shores-six": func() (*State, error) { return newCatanRiversShoresExtended(6) },
		"fog-six":    func() (*State, error) { return newCatanRiversFogExtended(6) },
		"desert-six": func() (*State, error) { return newCatanRiversDesertExtended(6, "rivers-across") },
		"tribe-six":  func() (*State, error) { return newCatanRiversTribeExtended(6) },
	}
	for name, create := range recipes {
		t.Run(name, func(t *testing.T) {
			s, err := create()
			if err != nil {
				t.Fatal(err)
			}
			for i := range s.Catan.Tiles {
				s.Catan.Tiles[i].X = math.Nextafter(s.Catan.Tiles[i].X, math.Inf(1))
				s.Catan.Tiles[i].Y = math.Nextafter(s.Catan.Tiles[i].Y, math.Inf(-1))
			}
			for i := range s.Catan.Vertices {
				s.Catan.Vertices[i].X = math.Nextafter(s.Catan.Vertices[i].X, math.Inf(1))
				s.Catan.Vertices[i].Y = math.Nextafter(s.Catan.Vertices[i].Y, math.Inf(-1))
			}
			before, _ := json.Marshal(s)
			if err := s.Catan.validateRivers(); err != nil {
				t.Fatal("display roundoff rejected", err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("validation rewrote the saved board")
			}
		})
	}
}

func TestCatanRiversGeometryRejectsCorruption(t *testing.T) {
	mutations := map[string]func(*Catan){
		"tile-x":        func(g *Catan) { g.Tiles[0].X += .01 },
		"tile-y":        func(g *Catan) { g.Tiles[0].Y += .01 },
		"tile-nan":      func(g *Catan) { g.Tiles[0].X = math.NaN() },
		"tile-inf":      func(g *Catan) { g.Tiles[0].Y = math.Inf(1) },
		"tile-id":       func(g *Catan) { g.Tiles[0].ID++ },
		"terrain":       func(g *Catan) { g.Tiles[12].Resource = (g.Tiles[12].Resource + 1) % 5 },
		"number":        func(g *Catan) { g.Tiles[12].Number++ },
		"tile-vertices": func(g *Catan) { g.Tiles[0].Vertices[0] = g.Tiles[0].Vertices[1] },
		"vertex-x":      func(g *Catan) { g.Vertices[0].X += .01 },
		"vertex-y":      func(g *Catan) { g.Vertices[0].Y += .01 },
		"vertex-nan":    func(g *Catan) { g.Vertices[0].X = math.NaN() },
		"vertex-inf":    func(g *Catan) { g.Vertices[0].Y = math.Inf(-1) },
		"vertex-id":     func(g *Catan) { g.Vertices[0].ID++ },
		"vertex-harbor": func(g *Catan) { g.Vertices[0].Harbor = !g.Vertices[0].Harbor },
		"edge":          func(g *Catan) { g.Edges[0].A = g.Edges[0].B },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			s, err := newCatanRiversShores(4)
			if err != nil {
				t.Fatal(err)
			}
			mutate(s.Catan)
			if s.Catan.validateRivers() == nil {
				t.Fatal("corrupt board accepted")
			}
		})
	}
}

func TestCatanRiversFrozenGeometryDiagnostic(t *testing.T) {
	s := catanTwoRiversFrozenState(t)
	ref, err := newCatanTwoRiversSeafarers(s.Catan.Seafarers.Scenario, s.Catan.Rivers.SeaLayout)
	if err != nil {
		t.Fatal(err)
	}
	differences := 0
	for i, a := range s.Catan.Tiles {
		b := ref.Catan.Tiles[i]
		if !reflect.DeepEqual(a, b) {
			differences++
			if differences <= 3 {
				t.Logf("tile %d: saved=(%.17g,%.17g) generated=(%.17g,%.17g); delta=(%g,%g)", i, a.X, a.Y, b.X, b.Y, a.X-b.X, a.Y-b.Y)
			}
		}
	}
	t.Logf("tiles with differing coordinates: %d", differences)
	if err := s.validateTwoRiversSea(); err != nil {
		t.Fatal(err)
	}
}

// Integer conversion failures: rounding (including negative values) must be
// deterministic, adjacent cells stay distinct, and nonfinite/overflowing values
// cannot collapse to the same int64 and accidentally pass validation.
func TestCatanGeometryIntegerGrid(t *testing.T) {
	for _, tc := range []struct {
		name  string
		a, b  float64
		equal bool
	}{
		{"recorded arm amd pair", 453.37059831359926, 453.3705983135992, true},
		{"same positive cell", 1.0000001, 1.0000004, true},
		{"adjacent positive cells", 1.00000049, 1.00000051, false},
		{"same negative cell", -1.0000001, -1.0000004, true},
		{"adjacent negative cells", -1.00000049, -1.00000051, false},
		{"signed zero", 0, math.Copysign(0, -1), true},
		{"nan", math.NaN(), math.NaN(), false},
		{"positive infinity", math.Inf(1), math.Inf(1), false},
		{"negative infinity", math.Inf(-1), math.Inf(-1), false},
		{"overflow", math.MaxFloat64, math.MaxFloat64, false},
		{"finite integer overflow", 1e20, 1e20, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameCatanCoordinate(tc.a, tc.b); got != tc.equal {
				t.Fatalf("same coordinate = %t, want %t", got, tc.equal)
			}
		})
	}
}
