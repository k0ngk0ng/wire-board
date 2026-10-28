package contra

// Cartridge water24..29 use identical visible colors for both actor labels.
// Public occupancy narrows candidate seats but cannot rule out an NPC with the
// same head artwork. Identity requires visible history or controlled feedback.
import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestBotWaterArtworkHasCrossSeatAliases(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
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
	for code := uint8(24); code <= 29; code++ {
		for _, effect := range []uint8{0, 8, 64, 72} {
			recipes := []BotVisualSpriteRecipe{
				{Mapping: mapping, Palette: palette, Code: code, Attributes: effect, Label: BotVisionLabel{Class: "player", Player: 0}},
				{Mapping: mapping, Palette: palette, Code: code, Attributes: effect | 5, Label: BotVisionLabel{Class: "player", Player: 1}},
			}
			art, err := BuildBotVisualSpriteTemplates(catalog, recipes)
			if err != nil {
				t.Fatal(err)
			}
			if len(art) != 2 || art[0].Width != art[1].Width || art[0].Height != art[1].Height || !bytes.Equal(art[0].RGBA, art[1].RGBA) {
				t.Fatal("expected exact shared water artwork", code, effect)
			}
		}
	}
}

func TestBuiltinBotVisionSoloWaterPublicOccupancy(t *testing.T) {
	file, err := os.Open("testdata/bot-water-submitted-frame.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var sample struct {
		Frame PresentedFrame
		HUD   BotPublicHUD
		Tick  uint64
	}
	if err = json.NewDecoder(z).Decode(&sample); err != nil {
		t.Fatal(err)
	}
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	vision, err := NewBuiltinBotVision(catalog, bundle)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := NewBotVisualFrame(sample.Frame, sample.HUD, catalog, 0, sample.Tick)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := ObserveBotFrame(frame, vision)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Players[0].Visible {
		t.Fatal("a head alone was promoted to player identity", observed.Players[0])
	}
	if observed.Players[1].Visible {
		t.Fatal("invented second player in solo room")
	}
	found := false
	for _, candidate := range observed.PlayerCandidates {
		if candidate.Mask == 1 && candidate.Player.X == 128 && candidate.Player.Sprite >= 24 && candidate.Player.Sprite <= 29 {
			found = true
		}
	}
	if !found {
		t.Fatal("visible water head lost its public candidate", observed.PlayerCandidates)
	}
}
