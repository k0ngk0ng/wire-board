package contra

// Fixed-knowledge assembly gate before code: use verified published index pages,
// never ROM bytes, current maps, entity slots or live collision. Known visual
// cells must agree with this independently rendered static-art fixture; a
// missing or mismatched visual catalog must fail instead of borrowing graphics.
import (
	"fmt"
	"testing"
)

func TestBuiltinBotVisionPublishedTerrain(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	if len(vision.terrain) == 0 {
		t.Fatal("no actual visual terrain knowledge")
	}
	res, e := bundle.Stage(0)
	if e != nil {
		t.Fatal(e)
	}
	info, e := res.Stage(0)
	if e != nil {
		t.Fatal(e)
	}
	screen, e := res.Screen(0, RegularMap, 0, false)
	if e != nil {
		t.Fatal(e)
	}
	p := PresentedFrame{Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 0x0a}}
	p.Patterns, e = res.CHRPreset(info.CHRPreset)
	if e != nil {
		t.Fatal(e)
	}
	p.Palette, e = res.Palette(info.Initial.PaletteIndices)
	if e != nil {
		t.Fatal(e)
	}
	copy(p.Background[:], screen.Tiles)
	copy(p.Background[960:], screen.Attributes)
	frame, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 1, Players: [2]BotPublicPlayer{{Occupied: true}}}, catalog, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	observed, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	known := 0
	var collision CollisionWindow
	copy(collision[:], screen.Collision[:])
	for y := 0; y < 224; y += 16 {
		for x := 0; x < 256; x += 16 {
			yy, xx := (y+int(observed.ViewY))/16, (x+int(observed.ViewX))/16
			if yy >= 15 || xx >= 17 || !observed.TerrainKnown[yy][xx] {
				continue
			}
			want := collision.At(uint8(x), uint8(y), p.Scroll).Kind
			if observed.terrainAt(x, y) != want {
				t.Fatalf("known visible cell %d,%d =%v want%v", x, y, observed.terrainAt(x, y), want)
			}
			known++
		}
	}
	if known < 4 {
		t.Fatal("static image did not establish any useful visible cells", known)
	}
	if _, e = NewBuiltinBotVision(nil, bundle); e == nil {
		t.Fatal("missing visual resources accepted")
	}
}

func TestBuiltinBotVisionCartridgeActors(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	res, e := bundle.Stage(0)
	if e != nil {
		t.Fatal(e)
	}
	info, e := res.Stage(0)
	if e != nil {
		t.Fatal(e)
	}
	state, e := NewSpriteCompositionState()
	if e != nil {
		t.Fatal(e)
	}
	input := SpriteCompositionInput{Level: 0}
	input.Objects[0] = LogicalSprite{Code: 13, X: 60, Y: 120, Attributes: 8}
	input.Objects[1] = LogicalSprite{Code: 13, X: 188, Y: 120, Attributes: 77}
	input.Objects[10] = LogicalSprite{Code: 59, X: 124, Y: 180, Attributes: 0}
	if _, e = state.Compose(input); e != nil {
		t.Fatal(e)
	}
	p := PresentedFrame{OAM: state.OAM, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 20}}
	p.Patterns, e = res.CHRPreset(info.CHRPreset)
	if e != nil {
		t.Fatal(e)
	}
	p.Palette, e = res.Palette(info.Initial.PaletteIndices)
	if e != nil {
		t.Fatal(e)
	}
	hud := BotPublicHUD{Seats: 2, Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: true}}}
	frame, e := NewBotVisualFrame(p, hud, catalog, 0, 10)
	if e != nil {
		t.Fatal(e)
	}
	seen, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	for actor, x := range []int{60, 188} {
		player := seen.Players[actor]
		if !player.Visible || absBot(player.X-x) > 1 || absBot(player.Y-120) > 1 {
			t.Fatalf("actual cartridge actor%d not visually recovered: %+v", actor, player)
		}
	}
	found := false
	for _, enemy := range seen.Enemies {
		if enemy.Kind == 5 && absBot(enemy.X-124) <= 1 && absBot(enemy.Y-180) <= 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("actual cartridge runner not visually recovered", seen.Enemies)
	}
	p.Display.AppliedMask = 0
	hidden, e := NewBotVisualFrame(p, hud, catalog, 0, 10)
	if e != nil {
		t.Fatal(e)
	}
	none, e := ObserveBotFrame(hidden, vision)
	if e != nil {
		t.Fatal(e)
	}
	if none.Players[0].Visible || none.Players[1].Visible || len(none.Enemies) != 0 {
		t.Fatal("disabled pixels leaked actor labels")
	}
}

func TestBuiltinBotVisionBackgroundEmplacements(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	resources, e := bundle.Stage(1)
	if e != nil {
		t.Fatal(e)
	}
	info, e := resources.Stage(1)
	if e != nil {
		t.Fatal(e)
	}
	animations, e := OriginalTileAnimations(1)
	if e != nil {
		t.Fatal(e)
	}
	for _, sample := range []struct{ code, want uint8 }{{4, 254}, {5, 254}, {6, 20}, {8, 19}, {3, 0}} {
		t.Run(fmt.Sprint(sample.code), func(t *testing.T) {
			p := PresentedFrame{Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 10}}
			p.Patterns, e = resources.CHRPreset(info.CHRPreset)
			if e != nil {
				t.Fatal(e)
			}
			p.Palette, e = resources.Palette(info.Initial.PaletteIndices)
			if e != nil {
				t.Fatal(e)
			}
			for y, row := range animations[sample.code].Rows {
				copy(p.Background[(10+y)*32+8:], row[:])
			}
			p.Background[960+2*8+2] = 255 // original public Base art uses palette group 3
			frame, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 1, Players: [2]BotPublicPlayer{{Occupied: true}}}, catalog, 0, 1)
			if e != nil {
				t.Fatal(e)
			}
			observed, e := ObserveBotFrame(frame, vision)
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, entity := range observed.Enemies {
				if absBot(entity.X-72) > 1 || absBot(entity.Y-88) > 1 {
					continue
				}
				if sample.want == 0 && (entity.Kind == 19 || entity.Kind == 20 || entity.Kind == 254) {
					t.Fatal("destroyed public art became a live emplacement", entity)
				}
				if sample.want != 0 && entity.Kind == sample.want {
					found = true
				}
				if sample.want == 254 && (entity.Kind == 19 || entity.Kind == 20) {
					t.Fatal("identical closed art leaked a hidden owner kind", entity)
				}
			}
			if sample.want != 0 && !found {
				t.Fatal("public background pattern was not recognized", sample, observed.Enemies)
			}
		})
	}
}
