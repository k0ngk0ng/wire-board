package contra

// Before extended fixed appearances: CHR/palette/retained attributes/backdrop
// must stay paired; a palette-only alternate cannot switch to CHR preset zero;
// all expected pixels come from the independent submitted-frame renderer.
import (
	"bytes"
	"reflect"
	"testing"
)

func backgroundAppearancePixels(t *testing.T, catalog BotVisualCatalog, res *LevelResources, tiles [16]uint8, mapping [8]uint8, indices [8]uint8, attribute, backdrop uint8) []byte {
	t.Helper()
	palette, err := res.Palette(indices)
	if err != nil {
		t.Fatal(err)
	}
	palette[0] = backdrop
	p := PresentedFrame{Patterns: mapping, Palette: palette, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 10}}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			p.Background[(8+y)*32+8+x] = tiles[y*4+x]
		}
	}
	p.Background[960+2*8+2] = attribute
	f, err := NewBotVisualFrame(p, BotPublicHUD{Seats: 1, Players: [2]BotPublicPlayer{{Occupied: true}}}, catalog, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	pixels := f.Pixels()
	out := make([]byte, 32*32*4)
	for y := 0; y < 32; y++ {
		copy(out[y*32*4:], pixels[((64+y)*256+64)*4:((64+y)*256+96)*4])
	}
	return out
}

func TestBotBackgroundBuilderDoesNotMutateEmptyVariantStorage(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	storage := make([]BotBackgroundAppearance, 1)
	storage[0].CHR[0] = 231
	before := append([]BotBackgroundAppearance(nil), storage...)
	recipe := BotVisionBackgroundRecipe{Level: 0, Table: "update", Codes: []uint8{3}, Width: 32, Height: 32, Label: BotVisionLabel{Class: "enemy", Player: -1, Kind: 4}, AppearanceVariants: storage[:0]}
	if _, err = buildBotBackgroundVisionTemplates(catalog, bundle, []BotVisionBackgroundRecipe{recipe}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(storage, before) {
		t.Fatal("default expansion wrote into caller-owned variant storage")
	}
}

func TestBotBackgroundAppearanceKeepsPairedFixedVariants(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	res, _ := bundle.Stage(0)
	info, _ := res.Stage(0)
	mapping, _ := res.CHRPreset(info.CHRPreset)
	graphics, _ := res.Graphics(0, RegularMap)
	indices := info.Initial.PaletteIndices
	changed := indices
	changed[0], changed[1] = changed[1], changed[0]
	recipe := BotVisionBackgroundRecipe{Level: 0, Table: "update", Codes: []uint8{3}, Width: 32, Height: 32, AnchorX: 16, AnchorY: 16, Label: BotVisionLabel{Class: "enemy", Player: -1, Kind: 4}, AppearanceVariants: []BotBackgroundAppearance{
		{CHR: mapping, PaletteIndices: indices, AttributeKnown: true, Attribute: 0, BackdropKnown: true, Backdrop: 15},
		{CHR: mapping, PaletteIndices: changed, AttributeKnown: true, Attribute: 0x55, BackdropKnown: true, Backdrop: 32},
	}}
	templates, err := buildBotBackgroundVisionTemplates(catalog, bundle, []BotVisionBackgroundRecipe{recipe})
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 2 {
		t.Fatal("explicit appearances were expanded into unsupported combinations", len(templates))
	}
	for i, v := range recipe.AppearanceVariants {
		want := backgroundAppearancePixels(t, catalog, res, graphics.Updates[3].Tiles, v.CHR, v.PaletteIndices, v.Attribute, v.Backdrop)
		if !bytes.Equal(templates[i].RGBA, want) {
			t.Fatal("paired fixed appearance differs from submitted rendering", i)
		}
	}
}

func TestBotBackgroundPaletteOnlyAlternatePreservesCHR(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	res, _ := bundle.Stage(6)
	info, _ := res.Stage(6)
	if info.Alternate.HasCHR {
		t.Fatal("fixture no longer exercises palette-only alternate")
	}
	mapping, _ := res.CHRPreset(info.CHRPreset)
	wrongMapping, wrongErr := res.CHRPreset(info.Alternate.CHRPreset)
	graphics, _ := res.Graphics(6, RegularMap)
	code := -1
	var want []byte
	for i, art := range graphics.Updates {
		pixels := backgroundAppearancePixels(t, catalog, res, art.Tiles, mapping, info.Alternate.Palette.PaletteIndices, art.Attribute, 15)
		if wrongErr != nil || !bytes.Equal(pixels, backgroundAppearancePixels(t, catalog, res, art.Tiles, wrongMapping, info.Alternate.Palette.PaletteIndices, art.Attribute, 15)) {
			code, want = i, pixels
			break
		}
	}
	if code < 0 {
		t.Fatal("fixture has no CHR-distinguishing artwork")
	}
	recipe := BotVisionBackgroundRecipe{Level: 6, Alternate: true, Table: "update", Codes: []uint8{uint8(code)}, Width: 32, Height: 32, Label: BotVisionLabel{Class: "enemy", Player: -1, Kind: 16}}
	templates, err := buildBotBackgroundVisionTemplates(catalog, bundle, []BotVisionBackgroundRecipe{recipe})
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) == 0 || !bytes.Equal(templates[0].RGBA, want) {
		t.Fatal("palette-only alternate used a different CHR bank")
	}
}
