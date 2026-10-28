package contra

// Before template generation: blank code1 is never a target; static attributes
// and recoil/shared parts use source composition; anchor survives transparent
// trimming; catalogue/template buffers never alias; no runtime slot/Kind is an
// input; only the caller's bounded recipe set is expanded.
import (
	"bytes"
	"testing"
)

func TestBotVisualSpriteTemplatesMatchVisibleProvider(t *testing.T) {
	c := &visualTestCatalog{pages: map[uint8][]uint8{0: make([]byte, 4096)}, rgb: make([]byte, 192)}
	for i := range c.rgb {
		c.rgb[i] = uint8(i)
	}
	for i := range c.pages[0] {
		c.pages[0][i] = uint8((i + i/64) % 4)
	}
	var pal [32]uint8
	for i := range pal {
		pal[i] = uint8(i)
	}
	var recipes []BotVisualSpriteRecipe
	for _, code := range []uint8{2, 8, 13, 30, 72, 184} {
		for _, attr := range []uint8{0, 5, 8, 64, 128, 192} {
			recipes = append(recipes, BotVisualSpriteRecipe{Palette: pal, Code: code, Attributes: attr, Label: BotVisionLabel{Class: "art", Player: -1, Sprite: code, Appearance: attr}})
		}
	}
	got, e := BuildBotVisualSpriteTemplates(c, recipes)
	if e != nil {
		t.Fatal(e)
	}
	if len(got) != len(recipes) {
		t.Fatal("recipe unexpectedly dropped", len(got))
	}
	for i, recipe := range recipes {
		state, e := NewSpriteCompositionState()
		if e != nil {
			t.Fatal(e)
		}
		input := SpriteCompositionInput{}
		input.Objects[0] = LogicalSprite{Code: recipe.Code, X: 128, Y: 112, Attributes: recipe.Attributes}
		if _, e = state.Compose(input); e != nil {
			t.Fatal(e)
		}
		p := PresentedFrame{OAM: state.OAM, Palette: pal, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 0x14}}
		f, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 1}, c, 0, 1)
		if e != nil {
			t.Fatal(e)
		}
		want := f.Pixels()
		v := got[i]
		replay := make([]byte, len(want))
		for at := 0; at < len(replay); at += 4 {
			copy(replay[at:at+3], c.rgb[:3])
			replay[at+3] = 255
		}
		for y := 0; y < v.Height; y++ {
			for x := 0; x < v.Width; x++ {
				at := (y*v.Width + x) * 4
				if v.RGBA[at+3] == 0 {
					continue
				}
				xx, yy := 128-v.AnchorX+x, 112-v.AnchorY+y
				copy(replay[(yy*256+xx)*4:(yy*256+xx)*4+4], v.RGBA[at:at+4])
			}
		}
		if !bytes.Equal(replay, want) {
			t.Fatalf("recipe%d code%d attr%d mismatches displayed pixels", i, recipe.Code, recipe.Attributes)
		}
	}
	keep := append([]byte(nil), got[0].RGBA...)
	c.pages[0][0] ^= 3
	c.rgb[16*3] ^= 5
	recipes[0].Palette[17] = 2
	if !bytes.Equal(keep, got[0].RGBA) {
		t.Fatal("generated template aliases input")
	}
}
func TestBotVisualSpriteTemplatesSkipBlankAndAbsentPixels(t *testing.T) {
	c := &visualTestCatalog{pages: map[uint8][]uint8{0: make([]byte, 4096)}, rgb: make([]byte, 192)}
	for i := range c.rgb {
		c.rgb[i] = uint8(i)
	}
	got, e := BuildBotVisualSpriteTemplates(c, []BotVisualSpriteRecipe{{Code: 0}, {Code: 1}, {Code: 2}})
	if e != nil || len(got) != 0 {
		t.Fatal("blank/sentinel became visual target", e, got)
	}
	if _, e = BuildBotVisualSpriteTemplates(nil, []BotVisualSpriteRecipe{{Code: 2}}); e == nil {
		t.Fatal("nil catalog accepted")
	}
}

func TestBotVisualSpriteTemplatesAliasesKeepLabelsAndOwnership(t *testing.T) {
	c := &visualTestCatalog{pages: map[uint8][]uint8{0: make([]byte, 4096), 1: make([]byte, 4096)}, rgb: make([]byte, 192)}
	for i := range c.rgb {
		c.rgb[i] = uint8(i)
	}
	for i := range c.pages[0] {
		c.pages[0][i] = uint8((i + i/64) % 4)
	}
	copy(c.pages[1], c.pages[0])
	var p [32]uint8
	for i := range p {
		p[i] = uint8(i)
	}
	p2 := p
	for i := 17; i < 32; i++ {
		copy(c.rgb[(i+32)*3:(i+33)*3], c.rgb[i*3:(i+1)*3])
		p2[i] = uint8(i + 32)
	}
	recipes := []BotVisualSpriteRecipe{{Code: 2, Palette: p, Label: BotVisionLabel{Class: "player", Player: 0}}, {Code: 2, Palette: p2, Mapping: [8]uint8{1, 1, 1, 1, 1, 1, 1, 1}, Label: BotVisionLabel{Class: "enemy", Kind: 5}}}
	got, e := BuildBotVisualSpriteTemplates(c, recipes)
	if e != nil || len(got) != 2 {
		t.Fatal(e, len(got))
	}
	if !bytes.Equal(got[0].RGBA, got[1].RGBA) || got[0].AnchorX != got[1].AnchorX || got[0].AnchorY != got[1].AnchorY || got[0].Label != recipes[0].Label || got[1].Label != recipes[1].Label {
		t.Fatal("visual alias changed drawing or label")
	}
	keep := append([]byte(nil), got[1].RGBA...)
	got[0].RGBA[0] ^= 255
	if !bytes.Equal(got[1].RGBA, keep) {
		t.Fatal("cached aliases share mutable output")
	}
}
