package contra

// Original pattern/raster evidence predates these declarations. Failure cases:
// current hidden level chooses knowledge; closed/shared jaw or rubble becomes
// a target; cropped pattern omits pulse colors; hidden HP/phase is asserted;
// an anchor uses slot coordinates; or a caller mutates global fixed knowledge.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type alienArtReference struct {
	ROMSHA256 string `json:"romSHA256"`
	Patterns  []struct {
		Table     string
		Code      int
		Tiles     []uint8
		Attribute uint8
	}
	Contexts []struct {
		Alternate      bool
		Mapping        [8]uint8
		PaletteIndices [8]uint8
		SourcePhase    int
	}
	Rasters []struct {
		Context, Code     int
		RetainedAttribute uint8
		RGBA              string
	}
	Scenes []struct {
		Context          int
		Appearance, RGBA string
	}
}

func botAlienArt(t *testing.T) alienArtReference {
	t.Helper()
	f, e := os.Open("testdata/bot-alien-original-art.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var ref alienArtReference
	if e = json.NewDecoder(z).Decode(&ref); e != nil {
		t.Fatal(e)
	}
	if ref.ROMSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" || len(ref.Rasters) != 1224 || len(ref.Scenes) != 18 {
		t.Fatal("original Alien artwork coverage")
	}
	return ref
}
func botAlienCrop(t *testing.T, raw string, x, y, w, h int) []uint8 {
	t.Helper()
	b, e := hex.DecodeString(raw)
	if e != nil || len(b) != 32*32*4 {
		t.Fatal("bad original block raster")
	}
	out := make([]uint8, 0, w*h*4)
	for row := 0; row < h; row++ {
		out = append(out, b[((y+row)*32+x)*4:((y+row)*32+x+w)*4]...)
	}
	return out
}
func botAlienTemplates(t *testing.T, ref alienArtReference) []BotVisionTemplate {
	t.Helper()
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	templates, err := buildBotBackgroundVisionTemplates(botBuiltinTestCatalog(t), bundle, BotAlienBackgroundVisionRecipes())
	if err != nil {
		t.Fatal(err)
	}
	for _, recipe := range BotAlienBackgroundVisionRecipes() {
		for _, r := range ref.Rasters {
			context := ref.Contexts[r.Context]
			if context.Alternate != recipe.Alternate {
				continue
			}
			include := false
			for _, code := range recipe.Codes {
				include = include || int(code) == r.Code
			}
			if !include {
				continue
			}
			want := botAlienCrop(t, r.RGBA, recipe.CropX, recipe.CropY, recipe.Width, recipe.Height)
			found := false
			for _, actual := range templates {
				if actual.Label == recipe.Label && actual.Width == recipe.Width && actual.Height == recipe.Height && actual.AnchorX == recipe.AnchorX-recipe.CropX && actual.AnchorY == recipe.AnchorY-recipe.CropY && bytes.Equal(actual.RGBA, want) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("public template construction misses original code%02x/context%d", r.Code, r.Context)
			}
		}
	}
	return templates
}

func TestBotAlienRecipesOriginalArtAndContexts(t *testing.T) {
	ref := botAlienArt(t)
	recipes := BotAlienBackgroundVisionRecipes()
	if len(recipes) != 2 {
		t.Fatal("expected Guardian opening and Heart core families")
	}
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	resources, e := bundle.Stage(7)
	if e != nil {
		t.Fatal(e)
	}
	graphics, e := resources.Graphics(7, RegularMap)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, recipe := range recipes {
		if recipe.Level != 7 || recipe.Variant != RegularMap || recipe.Table != "update" || recipe.Label.Class != "enemy" || recipe.Label.Player != -1 || recipe.Label.Stage.Known || recipe.Label.Scene.Known || recipe.Label.Choice.Known || recipe.Label.Phase != "" {
			t.Fatal("recipe claims hidden runtime facts", recipe)
		}
		for _, code := range recipe.Codes {
			found := false
			for _, p := range ref.Patterns {
				if p.Table == "update" && p.Code == int(code) {
					found = true
					if p.Attribute != graphics.Updates[code].Attribute || !bytes.Equal(p.Tiles, graphics.Updates[code].Tiles[:]) {
						t.Fatal("recipe artwork differs from pinned ROM", code)
					}
				}
			}
			if !found {
				t.Fatal("missing source", code)
			}
			count++
		}
		switch recipe.Label.Kind {
		case 16:
			if recipe.Label.Family != "alien-guardian" {
				t.Fatal("missing visible Guardian family")
			}
			if recipe.Alternate || !bytes.Equal(recipe.Codes, []uint8{0x12}) || recipe.CropX != 0 || recipe.CropY != 8 || recipe.Width != 32 || recipe.Height != 24 || recipe.AnchorX != 8 || recipe.AnchorY != 24 {
				t.Fatal("Guardian must use visible opening, not common forehead/closed jaw", recipe)
			}
		case 22:
			if recipe.Label.Family != "alien-heart" {
				t.Fatal("missing visible Heart family")
			}
			if !recipe.Alternate || !bytes.Equal(recipe.Codes, []uint8{7, 11}) || recipe.CropX != 8 || recipe.CropY != 0 || recipe.Width != 24 || recipe.Height != 24 || recipe.AnchorX != 24 || recipe.AnchorY != 8 {
				t.Fatal("Heart must use visible core with alternate public art", recipe)
			}
		default:
			t.Fatal("unverified visual family", recipe.Label.Kind)
		}
	}
	if count != 3 {
		t.Fatal("unexpected target pattern coverage", count)
	}
	// The chosen Guardian crop is identical across all sixteen legal ordinary
	// contexts. The Heart crops each have exactly two visible palette variants,
	// invariant under the four legal CHR phases. No current scene selects them.
	for _, recipe := range recipes {
		for _, code := range recipe.Codes {
			distinct := map[string]bool{}
			contexts := 0
			for _, r := range ref.Rasters {
				if r.Code == int(code) && ref.Contexts[r.Context].Alternate == recipe.Alternate {
					distinct[string(botAlienCrop(t, r.RGBA, recipe.CropX, recipe.CropY, recipe.Width, recipe.Height))] = true
					contexts++
				}
			}
			want, quantity := 1, 16
			if recipe.Alternate {
				want, quantity = 2, 8
			}
			if len(distinct) != want || contexts != quantity {
				t.Fatal("legal public appearance coverage", code, len(distinct), contexts)
			}
		}
	}
}

func TestBotAlienRecipesVisibleTargetsAndNegatives(t *testing.T) {
	ref := botAlienArt(t)
	vision, e := NewBotVision(BotVisionKnowledge{Sprites: botAlienTemplates(t, ref)})
	if e != nil {
		t.Fatal(e)
	}
	positive, negative := 0, 0
	for _, scene := range ref.Scenes {
		pixels, e := hex.DecodeString(scene.RGBA)
		if e != nil || len(pixels) != 256*224*4 {
			t.Fatal("bad original scene")
		}
		frame := BotVisualFrame{tick: 1, actor: 0, pixels: make([]uint8, 256*240*4), hud: BotPublicHUD{Seats: 1, Phase: "battle", Players: [2]BotPublicPlayer{{Occupied: true}}}}
		for i := 3; i < len(frame.pixels); i += 4 {
			frame.pixels[i] = 255
		}
		copy(frame.pixels, pixels)
		observation, e := ObserveBotFrame(frame, vision)
		if e != nil {
			t.Fatal(e)
		}
		visible := scene.Appearance == "open" || scene.Appearance == "pulse0" || scene.Appearance == "pulse1"
		if !visible {
			negative++
			if len(observation.Enemies) != 0 {
				t.Fatalf("original %s context%d invented a weak-point candidate: %+v", scene.Appearance, scene.Context, observation.Enemies)
			}
			continue
		}
		positive++
		kind := uint8(16)
		family := "alien-guardian"
		x, y := 168, 88
		if ref.Contexts[scene.Context].Alternate {
			kind = 22
			family = "alien-heart"
			x, y = 184, 104
		}
		matched := false
		for _, enemy := range observation.Enemies {
			if enemy.Kind == kind && enemy.Family == family && enemy.X == x && enemy.Y == y {
				matched = true
			}
		}
		if !matched {
			t.Fatalf("original %s context%d missing visible anchor %d,%d: %+v", scene.Appearance, scene.Context, x, y, observation.Enemies)
		}
		// The same rendered image may persist after an unseen damage decision.
		// enemy means potential visual target, never Alive/HP/vulnerability.
		frame.hud.Stage = BotPublicNumber{Value: 7}
		again, e := ObserveBotFrame(frame, vision)
		if e != nil || !reflect.DeepEqual(observation.Enemies, again.Enemies) || again.StageKnown {
			t.Fatal("same image was disambiguated by an unknown stage", e)
		}
	}
	if positive != 8 || negative != 10 {
		t.Fatal("source scene coverage", positive, negative)
	}
}

func TestBotAlienRecipesSharedJawAndDetachedKnowledge(t *testing.T) {
	ref := botAlienArt(t)
	var closed, open string
	for _, r := range ref.Rasters {
		if r.Context == 12 {
			if r.Code == 0x11 {
				closed = r.RGBA
			}
			if r.Code == 0x13 {
				open = r.RGBA
			}
		}
	}
	if !bytes.Equal(botAlienCrop(t, closed, 0, 0, 32, 16), botAlienCrop(t, open, 0, 0, 32, 16)) {
		t.Fatal("retained common teeth no longer share the same pixels")
	}
	a, b := BotAlienBackgroundVisionRecipes(), BotAlienBackgroundVisionRecipes()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("unstable global recipe catalog")
	}
	for _, r := range a {
		for _, code := range r.Codes {
			if code == 0x10 || code == 0x11 || code == 0x13 || code >= 0x0c && code <= 0x0f || code >= 0x15 {
				t.Fatal("closed/shared/decorative/destroyed art asserted as candidate", code)
			}
		}
	}
	a[0].Codes[0] ^= 255
	a[0].Label.Kind = 0
	if !reflect.DeepEqual(b, BotAlienBackgroundVisionRecipes()) {
		t.Fatal("caller changed global knowledge")
	}
}
