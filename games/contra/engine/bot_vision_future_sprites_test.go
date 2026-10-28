package contra

// Failure inventory before recipes: decimal42/43 misread as sniper hex42/43;
// all attributes mixed with every pose; a gun delay attribute mistaken for OAM
// palette; a controlled noninitial sprite value admitted as a natural pose;
// Giant art used before its CHR switch; equal enemy/friendly bullet pixels
// promoted to an enemy; runtime level/slot choosing the static knowledge.
import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestBotFutureSpriteRecipesLegalAppearanceSets(t *testing.T) {
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	before, _ := json.Marshal(bundle.Manifest())
	rs, e := BotFutureSpriteVisionRecipes(bundle)
	if e != nil {
		t.Fatal(e)
	}
	if len(rs) != 73 {
		t.Fatal("unexpected recipe Cartesian product", len(rs))
	}
	counts := map[string]int{}
	for _, r := range rs {
		if r.Label.Stage.Known || r.Label.Scene.Known || r.Label.Player != -1 || r.Label.Phase != "" {
			t.Fatal("recipe carries a runtime selector", r.Label)
		}
		if r.Label.Sprite != r.Code {
			t.Fatal("visual identity differs")
		}
		switch r.Code {
		case 189, 190:
			if r.Attributes != 0 || r.Label.Class != "enemy" || r.Label.Kind != 14 {
				t.Fatal(r)
			}
			counts["gunner"]++
		case 34:
			if r.Attributes != 0 || r.Label.Class != "ambiguous" {
				t.Fatal("snow bomb inferred allegiance", r)
			}
			counts["neutral34"]++
		case 31:
			if r.Attributes != 0 || r.Label.Class != "ambiguous" {
				t.Fatal("same-image allegiance inferred", r)
			}
			counts["neutral31"]++
		case 32:
			if r.Attributes != 6 || r.Label.Class != "ambiguous" {
				t.Fatal("mortar palette or allegiance differs", r)
			}
			counts["neutral32"]++
		case 42, 43:
			if r.Attributes != 0 || r.Label.Kind != BotVisualCartKind || r.Label.Class != "enemy" || r.Mapping != [8]uint8{1, 4, 18, 19, 88, 89, 90, 91} {
				t.Fatal("cart mislabelled as logical kind/sniper", r)
			}
			counts["cart"]++
		case 184, 185, 186, 195:
			if r.Attributes != 0 && r.Attributes != 64 || r.Label.Kind != 19 || r.Mapping != [8]uint8{1, 4, 22, 23, 84, 85, 86, 87} {
				t.Fatal("unreachable Giant graphics variant", r)
			}
			counts["giant"]++
		case 187, 188:
			if r.Attributes != 0 || r.Label.Kind != 20 || r.Mapping != [8]uint8{1, 4, 22, 23, 84, 85, 86, 87} {
				t.Fatal("disk has invented flip or resource set", r)
			}
			counts["disk"]++
		case 116, 117, 118, 119:
			if r.Attributes != 0 && r.Attributes != 32 || r.Label.Kind != 17 {
				t.Fatal("invented grenade flip", r)
			}
			counts["grenade"]++
		case 196:
			if r.Attributes != 0 || r.Label.Class != "scenery" {
				t.Fatal("invulnerable ice joint became enemy", r)
			}
			counts["joint"]++
		case 124, 125, 126:
			if r.Attributes != 0 || r.Label.Kind != 21 || r.Mapping[6] != 107 {
				t.Fatal("UFO not bound to carrier art", r)
			}
			counts["ufo"]++
		case 172, 173, 174, 175:
			if r.Attributes != 0 && r.Attributes != 192 || r.Label.Kind != 17 {
				t.Fatal("invented fetus orientation", r)
			}
			counts["fetus"]++
		case 176, 177, 178:
			if r.Attributes != 0 || r.Label.Kind != 19 {
				t.Fatal("invented blob flip", r)
			}
			counts["blob"]++
		case 179, 180, 181, 182:
			if r.Attributes != 0 && r.Attributes != 128 || r.Label.Kind != 20 {
				t.Fatal("invented spider horizontal flip", r)
			}
			counts["spider"]++
		default:
			t.Fatal("unreviewed sprite code", r.Code)
		}
	}
	for k, want := range map[string]int{"gunner": 6, "neutral31": 3, "neutral32": 1, "cart": 2, "giant": 24, "disk": 2, "grenade": 8, "joint": 1, "ufo": 6, "neutral34": 1, "fetus": 8, "blob": 3, "spider": 8} {
		if counts[k] != want {
			t.Fatal(k, counts)
		}
	}
	after, _ := json.Marshal(bundle.Manifest())
	if !bytes.Equal(before, after) {
		t.Fatal("knowledge construction mutated resources")
	}
	if _, e = BotFutureSpriteVisionRecipes(nil); e == nil {
		t.Fatal("missing resources accepted")
	}
}
func TestBotFutureSpriteBulletAllegianceIsUnobservable(t *testing.T) {
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	catalog := botBuiltinTestCatalog(t)
	rs, e := BotFutureSpriteVisionRecipes(bundle)
	if e != nil {
		t.Fatal(e)
	}
	checked := 0
	for _, r := range rs {
		if r.Code != 31 && r.Code != 32 && r.Code != 34 {
			continue
		}
		friend := r
		friend.Attributes = 0
		friend.Label = BotVisionLabel{Class: "friendly-projectile", Player: -1, Sprite: r.Code}
		pair, e := BuildBotVisualSpriteTemplates(catalog, []BotVisualSpriteRecipe{r, friend})
		if e != nil || len(pair) != 2 {
			t.Fatal(e, len(pair))
		}
		if pair[0].Width != pair[1].Width || pair[0].Height != pair[1].Height || pair[0].AnchorX != pair[1].AnchorX || pair[0].AnchorY != pair[1].AnchorY || !bytes.Equal(pair[0].RGBA, pair[1].RGBA) {
			t.Fatal("claimed image alias is not exact", r.Code)
		}
		vision, e := NewBotVision(BotVisionKnowledge{Sprites: pair})
		if e != nil {
			t.Fatal(e)
		}
		frame := botVisionTestFrame()
		botVisionPlace(&frame, pair[0], 120, 100)
		observed, e := ObserveBotFrame(frame, vision)
		if e != nil || len(observed.Enemies) != 0 {
			t.Fatal("hidden projectile owner became an enemy", e, observed.Enemies)
		}
		checked++
	}
	if checked != 5 {
		t.Fatal("incomplete alias assertions", checked)
	}
}
func TestBotFutureSpriteRecipesUseExactDamagePalettes(t *testing.T) {
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	rs, e := BotFutureSpriteVisionRecipes(bundle)
	if e != nil {
		t.Fatal(e)
	}
	r, e := bundle.Stage(5)
	if e != nil {
		t.Fatal(e)
	}
	info, e := r.Stage(5)
	if e != nil {
		t.Fatal(e)
	}
	want := map[[32]uint8]bool{}
	for _, id := range []uint8{86, 81, 82} {
		p := info.Alternate.Palette.PaletteIndices
		p[7] = id
		colors, e := r.Palette(p)
		if e != nil {
			t.Fatal(e)
		}
		want[colors] = true
	}
	seen := map[[32]uint8]bool{}
	for _, v := range rs {
		if v.Label.Kind == 19 && v.Code >= 184 {
			if !want[v.Palette] {
				t.Fatal("invented Giant palette")
			}
			seen[v.Palette] = true
		}
	}
	if len(seen) != 3 {
		t.Fatal("missing visible damage colors", len(seen))
	}
}

func TestBotFutureSpriteRecipesOriginalStaticPixels(t *testing.T) {
	f, e := os.Open("../../../docs/research/nes-contra/evidence/bot-future-sprites-original-art.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var d struct {
		ROMSHA256 string
		Cases     []struct {
			Mapping                         [8]uint8
			Palette                         [32]uint8
			Code, Attributes, Kind          uint8
			Class                           string
			Width, Height, AnchorX, AnchorY int
			RGBA                            []uint8
		}
	}
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	if d.ROMSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" || len(d.Cases) != 73 {
		t.Fatal("wrong original art source")
	}
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	recipes, e := BotFutureSpriteVisionRecipes(bundle)
	if e != nil {
		t.Fatal(e)
	}
	actual, e := BuildBotVisualSpriteTemplates(botBuiltinTestCatalog(t), recipes)
	if e != nil {
		t.Fatal(e)
	}
	if len(actual) != len(d.Cases) {
		t.Fatal("incomplete original art", len(actual))
	}
	for i, w := range d.Cases {
		r, a := recipes[i], actual[i]
		if r.Mapping != w.Mapping || r.Palette != w.Palette || r.Code != w.Code || r.Attributes != w.Attributes || r.Label.Class != w.Class || r.Label.Kind != w.Kind {
			t.Fatal("source pairing differs", i, r.Code, w.Code)
		}
		if a.Width != w.Width || a.Height != w.Height || a.AnchorX != w.AnchorX || a.AnchorY != w.AnchorY || !bytes.Equal(a.RGBA, w.RGBA) {
			t.Fatal("original planar-ROM pixels differ", i, r.Code)
		}
	}
}
