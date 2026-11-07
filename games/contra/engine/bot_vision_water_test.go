package contra

// Failure cases before the water-entry recipe: a visible splash player becomes
// unknown; blue/red palettes or facing/recoil variants lose seat identity;
// template aliasing invents a hidden logical code instead of a visible family.
import (
	"bytes"
	"fmt"
	"testing"
)

func TestBuiltinBotVisionWaterPlayers(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	vision, err := NewBuiltinBotVision(catalog, bundle)
	if err != nil {
		t.Fatal(err)
	}
	res, err := bundle.Stage(0)
	if err != nil {
		t.Fatal(err)
	}
	info, err := res.Stage(0)
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := res.CHRPreset(info.CHRPreset)
	if err != nil {
		t.Fatal(err)
	}
	palette, err := res.Palette(info.Initial.PaletteIndices)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []uint8{115} {
		for actor := 0; actor < 2; actor++ {
			for _, effect := range []uint8{0, 8, 64, 72} {
				t.Run(fmt.Sprintf("code%d-player%d-effect%d", code, actor, effect), func(t *testing.T) {
					composition, err := NewSpriteCompositionState()
					if err != nil {
						t.Fatal(err)
					}
					attr := effect
					if actor == 1 {
						attr |= 5
					}
					input := SpriteCompositionInput{Level: 0, TwoPlayers: true}
					input.Objects[actor] = LogicalSprite{Code: code, X: 96, Y: 128, Attributes: attr}
					if _, err = composition.Compose(input); err != nil {
						t.Fatal(err)
					}
					presented := PresentedFrame{OAM: composition.OAM, Patterns: mapping, Palette: palette, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 20}}
					hud := BotPublicHUD{Seats: 2, Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: true}}}
					frame, err := NewBotVisualFrame(presented, hud, catalog, 0, 1)
					if err != nil {
						t.Fatal(err)
					}
					observed, err := ObserveBotFrame(frame, vision)
					if err != nil {
						t.Fatal(err)
					}
					player := observed.Players[actor]
					// Water entry aliases bobbing with a one-pixel logical origin.
					// Prove
					// pixel equality before asserting a canonical visible anchor.
					aliasCode, aliasAttr, aliasY := code, attr, uint8(128)
					wantCode, wantY := code, 128
					aliasAttr ^= 8
					if effect&8 == 0 {
						aliasY--
						wantY--
					} else {
						aliasY++
					}
					alias, err := NewSpriteCompositionState()
					if err != nil {
						t.Fatal(err)
					}
					input.Objects[actor] = LogicalSprite{Code: aliasCode, X: 96, Y: aliasY, Attributes: aliasAttr}
					if _, err = alias.Compose(input); err != nil {
						t.Fatal(err)
					}
					aliasView := presented
					aliasView.OAM = alias.OAM
					aliasFrame, err := NewBotVisualFrame(aliasView, hud, catalog, 0, 1)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(frame.Pixels(), aliasFrame.Pixels()) {
						t.Fatal("claimed water alias is not pixel-equivalent")
					}
					if !player.Visible || player.X != 96 || player.Y != wantY || player.Sprite != wantCode {
						t.Fatal("visible water player not recovered", player)
					}
					if observed.Players[1-actor].Visible {
						t.Fatal("water palette crossed player identity")
					}
				})
			}
		}
	}
}
