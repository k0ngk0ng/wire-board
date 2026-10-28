package contra

// Written before the Snow declarations. Failure inventory and independently
// rendered original-ROM evidence: docs/research/nes-contra/bot-snow-background-recipes.md.
import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type snowVisualCase struct {
	Code, Kind           uint8
	Family, RGBA, Reason string
	Anchor               [2]int
	CHR, PaletteIndices  [8]uint8
	Backdrop             uint8
}
type snowArtReference struct {
	ROMSHA256                  string
	VisualCases, NegativeCases []snowVisualCase
	Patterns                   []struct {
		Tiles     [16]uint8
		Attribute uint8
	}
	OriginalControlledObservations []json.RawMessage
	REA                            []json.RawMessage
}

func snowVisualReference(t *testing.T) snowArtReference {
	t.Helper()
	f, err := os.Open("testdata/bot-snow-original-art.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var r snowArtReference
	if err = json.NewDecoder(z).Decode(&r); err != nil {
		t.Fatal(err)
	}
	if r.ROMSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" || len(r.OriginalControlledObservations) != 95 || len(r.REA) != 33 {
		t.Fatal("wrong or incomplete original evidence")
	}
	return r
}

func TestBotSnowBackgroundRecipesOriginalPixels(t *testing.T) {
	r := snowVisualReference(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	res, err := bundle.Stage(4)
	if err != nil {
		t.Fatal(err)
	}
	graphics, err := res.Graphics(4, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Patterns) != len(graphics.Updates) {
		t.Fatal("original replacement table differs")
	}
	for i, p := range r.Patterns {
		if p.Tiles != graphics.Updates[i].Tiles || p.Attribute != graphics.Updates[i].Attribute {
			t.Fatal("original art bytes or quadrant attribute differ", i)
		}
	}
	recipes := BotSnowBackgroundVisionRecipes()
	actual, err := buildBotBackgroundVisionTemplates(botBuiltinTestCatalog(t), bundle, recipes)
	if err != nil {
		t.Fatal(err)
	}
	if len(recipes) != 4 || len(actual) != 64 || len(r.VisualCases) != 64 {
		t.Fatal("missing artwork or invented palette Cartesian expansion", len(recipes), len(actual), len(r.VisualCases))
	}
	key := func(w, h, x, y int, kind uint8, family string, raw []byte) string {
		return fmt.Sprintf("%d/%d/%d/%d/%d/%s/%x", w, h, x, y, kind, family, sha256.Sum256(raw))
	}
	want := map[string]int{}
	for _, c := range r.VisualCases {
		raw, err := hex.DecodeString(c.RGBA)
		if err != nil {
			t.Fatal(err)
		}
		want[key(32, 32, c.Anchor[0], c.Anchor[1], c.Kind, c.Family, raw)]++
	}
	for _, a := range actual {
		k := key(a.Width, a.Height, a.AnchorX, a.AnchorY, a.Label.Kind, a.Label.Family, a.RGBA)
		if want[k] == 0 {
			t.Fatal("template differs from independent original CHR/palette/anchor", a.Label, a.AnchorX, a.AnchorY)
		}
		want[k]--
	}
	for k, count := range want {
		if count != 0 {
			t.Fatal("missing evidenced appearance", k, count)
		}
	}
	t.Log("64 complete original pixel appearances and 28 source patterns agree")
}

func TestBotSnowBackgroundRecipesVisibleAnchorsAndNegatives(t *testing.T) {
	r := snowVisualReference(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	templates, err := buildBotBackgroundVisionTemplates(botBuiltinTestCatalog(t), bundle, BotSnowBackgroundVisionRecipes())
	if err != nil {
		t.Fatal(err)
	}
	vision, err := NewBotVision(BotVisionKnowledge{Sprites: templates})
	if err != nil {
		t.Fatal(err)
	}
	check := func(c snowVisualCase, positive bool) {
		t.Helper()
		raw, err := hex.DecodeString(c.RGBA)
		if err != nil || len(raw) != 32*32*4 {
			t.Fatal("invalid independent image", err)
		}
		f := BotVisualFrame{tick: 1, actor: 0, pixels: make([]byte, 256*240*4), hud: BotPublicHUD{Seats: 1, Phase: "battle", Players: [2]BotPublicPlayer{{Occupied: true}}}}
		for at := 0; at < len(f.pixels); at += 4 {
			copy(f.pixels[at:at+4], []byte{9, 9, 9, 255})
		}
		for y := 0; y < 32; y++ {
			copy(f.pixels[((80+y)*256+80)*4:], raw[y*32*4:(y+1)*32*4])
		}
		o, err := ObserveBotFrame(f, vision)
		if err != nil {
			t.Fatal(err)
		}
		if o.StageKnown || o.SceneKnown {
			t.Fatal("artwork revealed hidden stage/scene")
		}
		if !positive {
			if len(o.Enemies) != 0 {
				t.Fatal("erased/scenery/wrong appearance became a target", c.Code, c.Reason, o.Enemies)
			}
			return
		}
		found := false
		for _, v := range o.Enemies {
			if v.Kind == c.Kind && v.Family == c.Family && v.X == 80+c.Anchor[0] && v.Y == 80+c.Anchor[1] {
				found = true
			}
		}
		if !found {
			t.Fatal("visible target/anchor not recovered from pixels", c.Code, c.Anchor, o.Enemies)
		}
	}
	for _, c := range r.VisualCases {
		check(c, true)
	}
	for _, c := range r.NegativeCases {
		check(c, false)
	}
	if len(r.NegativeCases) != 15 {
		t.Fatal("missing negative coverage")
	}
	t.Log("64 positive visible anchors and 15 erased/scenery/wrong-CHR negatives agree")
}

func TestBotSnowBackgroundRecipesAreDetachedAndVisualOnly(t *testing.T) {
	a, b := BotSnowBackgroundVisionRecipes(), BotSnowBackgroundVisionRecipes()
	if len(a) != 4 || !reflect.DeepEqual(a, b) {
		t.Fatal("unstable Snow catalog")
	}
	for _, r := range a {
		if r.Level != 4 || r.Variant != RegularMap || r.Table != "update" || r.CropX != 0 || r.CropY != 0 || r.Width != 32 || r.Height != 32 || r.Label.Class != "enemy" || r.Label.Player != -1 || r.Label.Stage.Known || r.Label.Scene.Known || r.Label.Choice.Known || r.Label.Phase != "" {
			t.Fatal("hidden or unsupported meaning in Snow artwork", r)
		}
		if r.AnchorX < 0 || r.AnchorX >= 32 || r.AnchorY < 0 || r.AnchorY >= 32 || len(r.AppearanceVariants) == 0 {
			t.Fatal("nonvisual anchor or unbound appearance")
		}
		for _, code := range r.Codes {
			switch r.Label.Family {
			case "snow-tank":
				if r.Label.Kind != 18 || r.Alternate || code != 15 && code != 18 && code != 19 {
					t.Fatal("wheel or scenery treated as tank")
				}
			case "snow-carrier":
				if r.Label.Kind != 20 || !r.Alternate || code != 9 && code != 10 && code != 11 && code != 13 {
					t.Fatal("exit, erased or unrelated art treated as carrier")
				}
			default:
				t.Fatal("unknown visual family")
			}
		}
		for _, appearance := range r.AppearanceVariants {
			if appearance.AttributeKnown || !appearance.BackdropKnown {
				t.Fatal("source quadrants overridden or backdrop missing")
			}
		}
	}
	a[0].Codes[0] = 255
	a[0].AppearanceVariants[0].CHR[4] = 255
	a[0].AppearanceVariants[0].PaletteIndices[2] = 255
	if !reflect.DeepEqual(b, BotSnowBackgroundVisionRecipes()) || !reflect.DeepEqual(a[1].AppearanceVariants, b[1].AppearanceVariants) {
		t.Fatal("caller changed shared/global knowledge")
	}
}
