package contra

// Before global integration: independently verified later-stage artwork must
// remain identifiable in the complete immutable catalog, and the Hangar cart
// must not inherit the old broad sniper label. This uses pixels, never World.
import "testing"

func TestBuiltinBotVisionReviewedFutureBackgrounds(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	vision, err := NewBuiltinBotVision(catalog, bundle)
	if err != nil {
		t.Fatal(err)
	}
	recipes := append(BotMachineBackgroundVisionRecipes(), BotAlienBackgroundVisionRecipes()...)
	recipes = append(recipes, BotSnowBackgroundVisionRecipes()...)
	for _, recipe := range recipes {
		templates, err := buildBotBackgroundVisionTemplates(catalog, bundle, []BotVisionBackgroundRecipe{recipe})
		if err != nil {
			t.Fatal(err)
		}
		if len(templates) == 0 {
			t.Fatal("reviewed recipe produced no visual template", recipe.Label.Family)
		}
		art := templates[0]
		frame := botVisionTestFrame()
		botVisionPlace(&frame, art, 64, 80)
		observed, err := ObserveBotFrame(frame, vision)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range observed.Enemies {
			found = found || e.Kind == recipe.Label.Kind && e.Family == recipe.Label.Family && e.X == 64+art.AnchorX && e.Y == 80+art.AnchorY
		}
		if !found {
			t.Fatal("complete global knowledge lost a reviewed visible target", recipe.Label, recipe.Codes, observed.Enemies)
		}
	}
}

func TestBuiltinBotVisionSnowCarrierInRecordedScene(t *testing.T) {
	seen := botVisionObservePNG(t, "testdata/bot-snow-carrier-scene.png")
	found := false
	for _, e := range seen.Enemies {
		// The source patch is visibly aligned at (96,48); (28,16) is its
		// inner silver-axis anchor. It is not the logical entity origin.
		found = found || e.Family == "snow-carrier" && e.X == 124 && e.Y == 64
	}
	if !found {
		t.Fatal("actual visible carrier core missing from complete global knowledge", seen.Enemies)
	}
}

func TestBuiltinBotVisionHangarCartIsNotSniper(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	vision, err := NewBuiltinBotVision(catalog, bundle)
	if err != nil {
		t.Fatal(err)
	}
	recipes, err := BotFutureSpriteVisionRecipes(bundle)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, r := range recipes {
		if r.Label.Kind != BotVisualCartKind {
			continue
		}
		art, err := BuildBotVisualSpriteTemplates(catalog, []BotVisualSpriteRecipe{r})
		if err != nil {
			t.Fatal(err)
		}
		frame := botVisionTestFrame()
		botVisionPlace(&frame, art[0], 80, 100)
		seen, err := ObserveBotFrame(frame, vision)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range seen.Enemies {
			if e.Kind == 6 {
				t.Fatal("cart artwork was called a sniper", e)
			}
			found = found || e.Kind == BotVisualCartKind
		}
		if !found {
			t.Fatal("reviewed cart not present in the global observer", seen.Enemies)
		}
		checked++
	}
	if checked != 2 {
		t.Fatal("missing reviewed cart poses", checked)
	}
}

func TestBuiltinBotVisionFutureGroupsUseReviewedAppearances(t *testing.T) {
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := BotFutureSpriteVisionRecipes(bundle)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[BotVisualSpriteRecipe]bool{}
	for _, r := range reviewed {
		allowed[r] = true
	}
	all, err := botBuiltinSpriteRecipes(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range all {
		code, kind := r.Code, r.Label.Kind
		futureGroup := code == 196 || kind == 17 && (code >= 116 && code <= 119 || code >= 172 && code <= 175) || kind == 21 && code >= 124 && code <= 126 || kind == 19 && code >= 176 && code <= 178 || kind == 20 && code >= 179 && code <= 182
		if futureGroup && !allowed[r] {
			t.Fatal("unreviewed broad future sprite combination remains in global knowledge", r.Label, code, r.Attributes, r.Mapping)
		}
	}
}
