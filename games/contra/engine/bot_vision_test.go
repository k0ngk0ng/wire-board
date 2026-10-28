package contra

// Pixel-observer failures precede implementation: reading hidden frame fields,
// treating blank or uniform color as an actor, leaking unknown HUD values,
// metadata/alias ordering changing the same image's result, mutable template
// buffers, offscreen sampling, and mistaking occluded or unrelated imagery for
// a complete known visual. The classifier receives no World/slot/collision.
import (
	"reflect"
	"testing"
)

func botVisionTestTemplate(class string, player int, kind, sprite uint8, blue bool) BotVisionTemplate {
	p := make([]byte, 5*7*4)
	for y := 0; y < 7; y++ {
		for x := 0; x < 5; x++ {
			if (x == 0 || x == 4) && y < 2 || (x+y)%5 == 0 {
				continue
			}
			at := (y*5 + x) * 4
			p[at], p[at+1], p[at+2], p[at+3] = 224, 112, 64, 255
			if (x+y)%3 == 0 {
				p[at], p[at+1], p[at+2] = 30, 70, 220
				if !blue {
					p[at], p[at+1], p[at+2] = 220, 40, 30
				}
			}
		}
	}
	return BotVisionTemplate{Width: 5, Height: 7, AnchorX: 2, AnchorY: 3, RGBA: p, Label: BotVisionLabel{Class: class, Player: player, Kind: kind, Sprite: sprite}}
}
func botVisionTestFrame() BotVisualFrame {
	p := make([]byte, 256*240*4)
	for i := 3; i < len(p); i += 4 {
		p[i] = 255
	}
	return BotVisualFrame{tick: 7, actor: 0, pixels: p, hud: BotPublicHUD{Seats: 2, Phase: "battle", Stage: BotPublicNumber{Known: true, Value: 0}, Scene: BotPublicNumber{Known: true, Value: 0}, Players: [2]BotPublicPlayer{{Occupied: true, Lives: BotPublicNumber{Known: true, Value: 2}}, {Occupied: true}}}}
}
func botVisionPlace(f *BotVisualFrame, v BotVisionTemplate, x, y int) {
	for yy := 0; yy < v.Height; yy++ {
		for xx := 0; xx < v.Width; xx++ {
			at := (yy*v.Width + xx) * 4
			if v.RGBA[at+3] == 0 || x+xx < 0 || x+xx >= 256 || y+yy < 0 || y+yy >= 240 {
				continue
			}
			copy(f.pixels[((y+yy)*256+x+xx)*4:], v.RGBA[at:at+4])
		}
	}
}
func TestBotVisionPixelsAndHUD(t *testing.T) {
	player := botVisionTestTemplate("player", 0, 0, 13, true)
	enemy := botVisionTestTemplate("enemy", -1, 5, 60, false)
	vision, e := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{player, enemy}})
	if e != nil {
		t.Fatal(e)
	}
	frame := botVisionTestFrame()
	blank, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	if len(blank.Enemies) != 0 || blank.Players[0].Visible || blank.Players[0].X != 0 || blank.Players[0].Y != 0 {
		t.Fatal("black frame invented a position")
	}
	if blank.Players[0].Lives != 2 || !blank.Players[0].LivesKnown || blank.Players[1].WeaponKnown || blank.Players[1].AliveKnown {
		t.Fatal("public/unknown HUD differs")
	}
	botVisionPlace(&frame, player, 20, 40)
	botVisionPlace(&frame, enemy, 150, 100)
	got, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	if got.Actor != 0 || got.Tick != 7 || !got.Players[0].Visible || got.Players[0].X != 22 || got.Players[0].Y != 43 {
		t.Fatal("player visual differs", got.Players[0])
	}
	if len(got.Enemies) != 1 || got.Enemies[0].X != 152 || got.Enemies[0].Y != 103 || got.Enemies[0].Kind != 5 {
		t.Fatal("enemy visual differs", got.Enemies)
	}
	// Renderer buffers and caller template storage cannot mutate frozen knowledge.
	player.RGBA[0] ^= 255
	enemy.RGBA[3] = 0
	again, e := ObserveBotFrame(frame, vision)
	if e != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("observer depends on caller buffer", e)
	}
}
func TestBotVisionAliasesAndHiddenInput(t *testing.T) {
	a := botVisionTestTemplate("enemy", -1, 5, 61, false)
	b := a
	b.Label.Sprite = 60
	left, e := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{a, b}})
	if e != nil {
		t.Fatal(e)
	}
	right, e := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{b, a}})
	if e != nil {
		t.Fatal(e)
	}
	frame := botVisionTestFrame()
	botVisionPlace(&frame, a, 150, 100)
	one, e := ObserveBotFrame(frame, left)
	if e != nil {
		t.Fatal(e)
	}
	two, e := ObserveBotFrame(frame, right)
	if e != nil || !reflect.DeepEqual(one, two) || len(one.Enemies) != 1 {
		t.Fatal("visual aliases disclose ordering", e)
	}
	frame.hud.Stage = BotPublicNumber{Value: 7}
	frame.hud.Players[0].Weapon = BotPublicNumber{Value: 4}
	unknown, e := ObserveBotFrame(frame, left)
	if e != nil {
		t.Fatal(e)
	}
	if unknown.StageKnown || unknown.Stage != 0 || unknown.Players[0].WeaponKnown || unknown.Players[0].Weapon != 0 {
		t.Fatal("unknown label values leaked")
	}
	if _, e = ObserveBotFrame(BotVisualFrame{}, left); e == nil {
		t.Fatal("empty frame accepted")
	}
	if _, e = ObserveBotFrame(frame, nil); e == nil {
		t.Fatal("missing immutable knowledge accepted")
	}
}
func TestBotVisionTerrainFromPixels(t *testing.T) {
	pattern := make([]uint8, 256)
	// A 4px-periodic pattern cannot establish a unique 16px cell boundary.
	// This fixed, non-periodic artwork makes the tested alignment observable.
	seed := uint32(0x12345678)
	for i := range pattern {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		pattern[i] = uint8(seed>>24) & 3
	}
	vision, e := NewBotVision(BotVisionKnowledge{Terrain: []BotVisionTerrain{{Pattern: pattern, Kind: TerrainPlatform}}})
	if e != nil {
		t.Fatal(e)
	}
	frame := botVisionTestFrame()
	for y := 7; y+16 <= 224; y += 16 {
		for x := 5; x+16 <= 256; x += 16 {
			for yy := 0; yy < 16; yy++ {
				for xx := 0; xx < 16; xx++ {
					value := pattern[yy*16+xx]
					at := ((y+yy)*256 + x + xx) * 4
					frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = value*40+11, value*20+21, value*50+31
				}
			}
		}
	}
	got, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	if !got.GridKnown || !got.TerrainKnown[1][1] || got.terrainAt(22, 24) != TerrainPlatform || got.ViewX != 11 || got.ViewY != 9 {
		t.Fatal("visible cell alignment differs", got.ViewX, got.ViewY)
	}
	// A solid-color frame gives no unique physical cell alignment or type.
	flat := botVisionTestFrame()
	empty, e := ObserveBotFrame(flat, vision)
	if e != nil {
		t.Fatal(e)
	}
	if empty.GridKnown {
		t.Fatal("uniform pixels invented grid alignment")
	}
	for _, row := range empty.TerrainKnown {
		for _, known := range row {
			if known {
				t.Fatal("uniform pixels invented terrain")
			}
		}
	}
}
