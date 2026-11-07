package contra

// Failure inventory is in docs/research/nes-contra/bot-background-recipes.md.
// Expected artwork comes from independently retained ROM bytes and original
// queue traces, exported before these recipe declarations. No live World input.
import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type backgroundArtReference struct {
	ROMSHA256 string `json:"romSHA256"`
	Patterns  []struct {
		Level, Variant, Code, Width, Height int
		Table                               string
		Tiles                               []uint8
		Attribute                           uint8
	}
	BasePaletteObservations []struct {
		Level, VisualOwner, Code, PaletteGroup int
		Anchor                                 [2]int
	}
}

func backgroundReference(t *testing.T) backgroundArtReference {
	t.Helper()
	f, e := os.Open("testdata/bot-background-original-art.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var ref backgroundArtReference
	if e = json.NewDecoder(z).Decode(&ref); e != nil {
		t.Fatal(e)
	}
	if ref.ROMSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" {
		t.Fatal("wrong original cartridge")
	}
	return ref
}
func TestBotBackgroundRecipesMatchOriginalArtwork(t *testing.T) {
	ref := backgroundReference(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	matched := 0
	for _, recipe := range BotBackgroundVisionRecipes() {
		if recipe.Level >= 8 || recipe.Width <= 0 || recipe.Height <= 0 || len(recipe.Codes) == 0 || recipe.Label.Class != "enemy" || recipe.Label.Stage.Known || recipe.Label.Scene.Known || recipe.Label.Choice.Known || recipe.Label.Phase != "" {
			t.Fatal("recipe contains hidden runtime meaning", recipe)
		}
		r, e := bundle.Stage(recipe.Level)
		if e != nil {
			t.Fatal(e)
		}
		_, tiles, e := r.mapSet(int(recipe.Level), recipe.Variant)
		if e != nil {
			t.Fatal(e)
		}
		if recipe.Alternate {
			stage, _ := r.Stage(int(recipe.Level))
			if recipe.Variant != RegularMap || !stage.Alternate.HasCHR {
				t.Fatal("invalid static alternate source")
			}
		}
		for _, code := range recipe.Codes {
			var actual []uint8
			attribute := uint8(0)
			width, height := 4, 4
			switch recipe.Table {
			case "base":
				if int(code) >= len(tiles.Supertiles) {
					t.Fatal("base ID outside source")
				}
				b := tiles.Supertiles[code]
				actual = append([]uint8(nil), b.Tiles[:]...)
				attribute = b.Attribute
			case "update":
				g, e := r.Graphics(int(recipe.Level), recipe.Variant)
				if e != nil || int(code) >= len(g.Updates) {
					t.Fatal("update ID outside source", e)
				}
				b := g.Updates[code]
				actual = append([]uint8(nil), b.Tiles[:]...)
				attribute = b.Attribute
			case "tile-animation":
				a, e := OriginalTileAnimations(recipe.Level)
				if e != nil || int(code) >= len(a) {
					t.Fatal("animation outside source", e)
				}
				width, height = 2, len(a[code].Rows)
				for _, row := range a[code].Rows {
					actual = append(actual, row[:]...)
				}
				attribute = a[code].Attribute
			default:
				t.Fatal("unknown art table")
			}
			if recipe.CropX < 0 || recipe.CropY < 0 || recipe.CropX+recipe.Width > width*8 || recipe.CropY+recipe.Height > height*8 {
				t.Fatal("recipe crop exceeds public artwork")
			}
			found := false
			for _, p := range ref.Patterns {
				if p.Level == int(recipe.Level) && p.Variant == int(recipe.Variant) && p.Table == recipe.Table && p.Code == int(code) {
					found = true
					if p.Width != width || p.Height != height || p.Attribute != attribute || !reflect.DeepEqual(p.Tiles, actual) {
						t.Fatal("artwork differs from independent ROM reference", recipe.Level, recipe.Table, code)
					}
				}
			}
			if !found {
				t.Fatal("recipe has no original source")
			}
			matched++
		}
	}
	if matched < 70 {
		t.Fatal("missing background families", matched)
	}
	t.Logf("%d immutable recipe patterns equal pinned original artwork", matched)
}
func TestBotBackgroundRecipesAmbiguityAndVisibleAnchors(t *testing.T) {
	refs := backgroundReference(t)
	seen := map[string]uint8{}
	for _, r := range BotBackgroundVisionRecipes() {
		for _, code := range r.Codes {
			key := fmt.Sprintf("%d/%d/%t/%s/%d", r.Level, r.Variant, r.Alternate, r.Table, code)
			if old, ok := seen[key]; ok && old != r.Label.Kind {
				t.Fatal("same artwork claims contradictory identity")
			}
			seen[key] = r.Label.Kind
			if r.Table == "tile-animation" {
				want := uint8(20)
				switch code {
				case 4, 5:
					want = 254
				case 8, 9:
					want = 19
				case 3:
					t.Fatal("destroyed wall labeled enemy")
				}
				if r.Label.Kind != want || r.TilePalette != 3 || r.AnchorX != 8 || r.AnchorY != 8 || r.Width != 16 || r.Height != 16 {
					t.Fatal("base ambiguity/palette/anchor differs", r, code)
				}
			}
			if r.Level == 0 && r.Label.Kind == 17 && (r.Table != "base" || !r.Alternate || code != 0x35 || r.AnchorX != 0 || r.AnchorY != 16) {
				t.Fatal("destroyed tunnel or wrong hatch anchor treated as door")
			}
			if r.Table == "update" {
				switch r.Label.Kind {
				case 4:
					if code < 3 || code > 16 {
						t.Fatal("rotator erased/background pattern")
					}
				case 7:
					if code == 0x16 || code == 0x17 {
						t.Fatal("hidden red turret claimed visible")
					}
				case 8:
					if code > 2 {
						t.Fatal("destroyed boss cannon")
					}
				case 10:
					if code != 3 && code != 4 {
						t.Fatal("destroyed boss plating")
					}
				case 16:
					if code == 0x28 || code == 0x2b {
						t.Fatal("destroyed jungle cannon")
					}
				}
			}
		}
	}
	for _, row := range refs.BasePaletteObservations {
		if row.PaletteGroup != 3 || row.Anchor != [2]int{8, 8} {
			t.Fatal("original visible context differs")
		}
		if row.Code == 3 {
			continue
		}
		key := fmt.Sprintf("%d/0/false/tile-animation/%d", row.Level, row.Code)
		want := uint8(row.VisualOwner)
		if row.Code == 4 || row.Code == 5 {
			want = 254
		}
		if seen[key] != want {
			t.Fatal("missing observed base appearance", key)
		}
	}
}
func TestBotBackgroundRecipesAreDetachedGlobalKnowledge(t *testing.T) {
	a := BotBackgroundVisionRecipes()
	b := BotBackgroundVisionRecipes()
	if len(a) == 0 || !reflect.DeepEqual(a, b) {
		t.Fatal("unstable recipe catalog")
	}
	a[0].Codes[0] ^= 255
	a[0].Label.Kind = 0
	if !reflect.DeepEqual(b, BotBackgroundVisionRecipes()) {
		t.Fatal("caller mutated global visual knowledge")
	}
	var visit func(reflect.Type)
	visit = func(typ reflect.Type) {
		switch typ.Kind() {
		case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Func:
			t.Fatal("runtime-capable recipe field", typ)
		case reflect.Slice, reflect.Array:
			visit(typ.Elem())
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				switch f.Name {
				case "HP", "Timer", "Routine", "Slot", "World", "Collision", "State":
					t.Fatal("hidden rule field in visual knowledge", f.Name)
				}
				visit(f.Type)
			}
		}
	}
	visit(reflect.TypeOf(BotVisionBackgroundRecipe{}))
}
