package contra

// Failure inventory was established before implementation in the independent
// ROM exporter: cart OAM mistaken for scenery; shared/erased panels identified
// as live targets; palette-only alternate selecting CHR0; lost 3-row claws;
// anchors following hidden slots; and caller mutation of global knowledge.
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

type machineVisualCase struct {
	Level                           uint8
	Alternate                       bool
	Table                           string
	Code, Kind                      uint8
	Width, Height, AnchorX, AnchorY int
	Group                           uint8
	CHR, PaletteIndices             [8]uint8
	Backdrop                        uint8
	RGBA                            string
}
type machineArtReference struct {
	ROMSHA256                  string
	VisualCases, NegativeCases []machineVisualCase
	FlamePaletteProof          []struct{ Groups []uint8 }
	LauncherPaletteProof       []struct{ Groups []uint8 }
	ClawPaletteObservations    []struct{ Groups []uint8 }
	ClawPathPaletteProof       []struct{ Groups []uint8 }
	ClawStaticImages           map[string][][3]int
	ClawSpawnAnchors           [][3]int
}

func machineVisualReference(t *testing.T) machineArtReference {
	t.Helper()
	f, e := os.Open("testdata/bot-machine-original-art.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var r machineArtReference
	if e = json.NewDecoder(z).Decode(&r); e != nil {
		t.Fatal(e)
	}
	if r.ROMSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" {
		t.Fatal("wrong original cartridge")
	}
	return r
}

func TestBotMachineBackgroundRecipesOriginalPixels(t *testing.T) {
	r := machineVisualReference(t)
	recipes := BotMachineBackgroundVisionRecipes()
	if len(recipes) != 10 || len(r.VisualCases) != 228 {
		t.Fatal("machine visual coverage changed", len(recipes), len(r.VisualCases))
	}
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	catalog := botBuiltinTestCatalog(t)
	actual, e := buildBotBackgroundVisionTemplates(catalog, bundle, recipes)
	if e != nil {
		t.Fatal(e)
	}
	key := func(w, h, x, y int, kind uint8, raw []byte) string {
		return fmt.Sprintf("%d/%d/%d/%d/%d/%x", w, h, x, y, kind, sha256.Sum256(raw))
	}
	want := map[string]int{}
	for _, c := range r.VisualCases {
		raw, e := hex.DecodeString(c.RGBA)
		if e != nil {
			t.Fatal(e)
		}
		want[key(c.Width, c.Height, c.AnchorX, c.AnchorY, c.Kind, raw)]++
	}
	if len(actual) != len(r.VisualCases) {
		t.Fatal("unexpected palette/CHR Cartesian expansion", len(actual), len(r.VisualCases))
	}
	for _, a := range actual {
		k := key(a.Width, a.Height, a.AnchorX, a.AnchorY, a.Label.Kind, a.RGBA)
		if want[k] == 0 {
			t.Fatal("template differs from independent planar-ROM pixels", a.Label, a.Width, a.Height)
		}
		want[k]--
	}
	for k, n := range want {
		if n != 0 {
			t.Fatal("missing original appearance", k, n)
		}
	}
	t.Logf("%d complete original pixel appearances match", len(actual))
}

func TestBotMachineBackgroundRecipesVisibleTargetsAndNegatives(t *testing.T) {
	r := machineVisualReference(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	templates, e := buildBotBackgroundVisionTemplates(botBuiltinTestCatalog(t), bundle, BotMachineBackgroundVisionRecipes())
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBotVision(BotVisionKnowledge{Sprites: templates})
	if e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	positive := 0
	check := func(c machineVisualCase, visible bool) {
		raw, e := hex.DecodeString(c.RGBA)
		if e != nil {
			t.Fatal(e)
		}
		f := BotVisualFrame{tick: 1, actor: 0, pixels: make([]byte, 256*240*4), hud: BotPublicHUD{Seats: 1, Phase: "battle", Players: [2]BotPublicPlayer{{Occupied: true}}}}
		for at := 0; at < len(f.pixels); at += 4 {
			copy(f.pixels[at:at+4], []byte{9, 9, 9, 255})
		}
		for y := 0; y < c.Height; y++ {
			copy(f.pixels[((80+y)*256+80)*4:], raw[y*c.Width*4:(y+1)*c.Width*4])
		}
		o, e := ObserveBotFrame(f, vision)
		if e != nil {
			t.Fatal(e)
		}
		if o.StageKnown || o.SceneKnown {
			t.Fatal("machine art revealed hidden stage/scene")
		}
		if !visible {
			if len(o.Enemies) != 0 {
				t.Fatalf("erased/shared/unlicensed-palette image %d/%s/%02x claimed target %+v", c.Level, c.Table, c.Code, o.Enemies)
			}
			return
		}
		found := false
		for _, v := range o.Enemies {
			if v.Kind == c.Kind && v.X == 80+c.AnchorX && v.Y == 80+c.AnchorY {
				found = true
			}
		}
		if !found {
			t.Fatalf("visible %d/%s/%02x lacks anchor %d,%d: %+v", c.Level, c.Table, c.Code, c.AnchorX, c.AnchorY, o.Enemies)
		}
	}
	for _, c := range r.VisualCases {
		k := fmt.Sprintf("%d/%s/%d", c.Level, c.Table, c.Code)
		if !seen[k] {
			check(c, true)
			seen[k] = true
			positive++
		}
	}
	for _, c := range r.NegativeCases {
		check(c, false)
	}
	if positive != 16 || len(r.NegativeCases) != 14 {
		t.Fatal("visible/negative coverage changed", positive, len(r.NegativeCases))
	}
	t.Logf("%d visible anchors and %d erased/shared/other-palette negatives match", positive, len(r.NegativeCases))
}

func TestBotMachineBackgroundRecipesProvenanceAndIsolation(t *testing.T) {
	r := machineVisualReference(t)
	if len(r.FlamePaletteProof) != 211 || len(r.LauncherPaletteProof) != 2 || len(r.ClawPaletteObservations) != 360 || len(r.ClawStaticImages["0"]) != 34 || len(r.ClawSpawnAnchors) != 34 || len(r.ClawStaticImages["6"]) != 6 || len(r.ClawPathPaletteProof) != 161 {
		t.Fatal("source palette/placement coverage changed")
	}
	for _, p := range r.FlamePaletteProof {
		for _, g := range p.Groups {
			if g != 3 {
				t.Fatal("flame palette")
			}
		}
	}
	for _, p := range r.LauncherPaletteProof {
		for _, g := range p.Groups {
			if g != 2 {
				t.Fatal("launcher palette")
			}
		}
	}
	for _, p := range r.ClawPaletteObservations {
		for _, g := range p.Groups {
			if g != 0 {
				t.Fatal("claw palette")
			}
		}
	}
	for _, p := range r.ClawPathPaletteProof {
		for _, g := range p.Groups {
			if g != 0 {
				t.Fatal("claw path crosses a different retained palette")
			}
		}
	}
	a := BotMachineBackgroundVisionRecipes()
	before := BotMachineBackgroundVisionRecipes()
	for _, p := range a {
		family := "machine-flame"
		if p.Level == 6 {
			family = map[uint8]string{16: "machine-claw", 23: "machine-mortar-launcher", 22: "machine-armored-door"}[p.Label.Kind]
		}
		if p.Label.Family != family {
			t.Fatal("missing visible machine family", p.Label)
		}
		if p.Variant != RegularMap || p.Label.Class != "enemy" || p.Label.Player != -1 || p.Label.Stage.Known || p.Label.Scene.Known || p.Label.Choice.Known || p.Label.Phase != "" || p.CropX != 0 || p.CropY != 0 {
			t.Fatal("hidden runtime claim in machine knowledge", p)
		}
		if len(p.AppearanceVariants) == 0 {
			t.Fatal("missing grouped source appearance")
		}
		for _, v := range p.AppearanceVariants {
			if !v.BackdropKnown || p.Table == "tile-animation" && (!v.AttributeKnown || v.Attribute != p.TilePalette*0x55) {
				t.Fatal("retained palette/backdrop missing", p)
			}
		}
	}
	a[0].Codes[0] = 255
	a[0].AppearanceVariants[0].CHR[4] ^= 255
	if !reflect.DeepEqual(before, BotMachineBackgroundVisionRecipes()) {
		t.Fatal("caller mutated global machine knowledge")
	}
	// One caller must not mutate another recipe's independently owned variants.
	if !reflect.DeepEqual(a[1].AppearanceVariants, before[1].AppearanceVariants) {
		t.Fatal("recipe variants alias one another")
	}
}
