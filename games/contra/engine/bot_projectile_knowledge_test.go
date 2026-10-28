package contra

// Before narrowing projectile recipes: no legal player laser orientation may
// become a hostile bullet because only its unflipped friendly pose was listed.
// Interior beam art is also fixed public knowledge; ownership-aliased pixels
// must remain neutral rather than acquire a native dispatch identity.
import "testing"

func TestBuiltinBotVisionPlayerLaserIsNeverHostile(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	vision, err := NewBuiltinBotVision(catalog, bundle)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := bundle.Stage(0)
	info, _ := r.Stage(0)
	mapping, _ := r.CHRPreset(info.CHRPreset)
	palette, _ := r.Palette(info.Initial.PaletteIndices)
	seenPose := map[[2]uint8]bool{}
	for _, pose := range laserPoses {
		if seenPose[pose] {
			continue
		}
		seenPose[pose] = true
		art, err := BuildBotVisualSpriteTemplates(catalog, []BotVisualSpriteRecipe{{Mapping: mapping, Palette: palette, Code: pose[0], Attributes: pose[1], Label: BotVisionLabel{Class: "friendly-projectile", Player: -1, Sprite: pose[0]}}})
		if err != nil {
			t.Fatal(err)
		}
		frame := botVisionTestFrame()
		botVisionPlace(&frame, art[0], 80, 100)
		o, err := ObserveBotFrame(frame, vision)
		if err != nil {
			t.Fatal(err)
		}
		if len(o.Enemies) != 0 {
			t.Fatal("legal player beam became a hostile visual target", pose, o.Enemies)
		}
	}
}

func TestBuiltinBotVisionContainsInteriorLaserAppearances(t *testing.T) {
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	recipes, err := botBuiltinSpriteRecipes(bundle)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := bundle.Stage(1)
	info, _ := r.Stage(1)
	mapping, _ := r.CHRPreset(info.CHRPreset)
	for _, code := range []uint8{130, 131, 132, 146} {
		for _, attribute := range []uint8{0, 64} {
			found := false
			for _, recipe := range recipes {
				found = found || recipe.Mapping == mapping && recipe.Code == code && recipe.Attributes == attribute && recipe.Label.Class == "friendly-projectile"
			}
			if !found {
				t.Fatal("missing legal interior laser pose", code, attribute)
			}
		}
	}
}
