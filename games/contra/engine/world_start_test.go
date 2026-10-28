package contra

// Full startup before wiring scenes: initialization must complete across nested
// NMI; palettes may be written by the text queue; immediate backgrounds must not
// wait another frame; busy countdown suppresses display without stopping rules;
// intro object writes are not ordinary player motion; music/reset emit once.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalContinuousStartupWorld(t *testing.T) {
	checkOriginalContinuousSceneWorld(t, "start", 1077)
}

func TestOriginalContinuousContinueWorld(t *testing.T) {
	w := checkOriginalContinuousSceneWorld(t, "continue", 898)
	if w.Campaign.Continues != 2 || w.Campaign.Outcome != CampaignOngoing || w.Active.Combat.World.LevelRoutine != 4 {
		t.Fatal("continue did not resume original battle", w.Campaign)
	}
}

func TestOriginalContinuousQuitWorld(t *testing.T) {
	w := checkOriginalContinuousSceneWorld(t, "quit", 386)
	if w.Campaign.Outcome != CampaignFailed || w.Active.Combat.World.GameRoutine != 0 {
		t.Fatal("end choice did not fail campaign")
	}
	before, _ := json.Marshal(w)
	if commands, err := w.Advance(999, [2]PlayerInput{}, levelResources(t, "stage-1.json")); err != nil || len(commands) != 0 {
		t.Fatal("terminal world advanced", err)
	}
	after, _ := json.Marshal(w)
	if string(before) != string(after) {
		t.Fatal("terminal world changed")
	}
}

func TestDiscardedGraphicsRetainPaletteBudget(t *testing.T) {
	p := WorldPresentation{Queue: GraphicsQueue{}, Palette: PaletteOutput{RequestedColors: 16}}
	if err := p.Queue.Append(GraphicsPacket{Kind: 1, Length: 65, Segments: []GraphicsSegment{{Address: 0x2000, Values: make([]uint8, 65)}}}); err != nil {
		t.Fatal(err)
	}
	p.DiscardGraphics(GraphicsText)
	if err := p.Seal(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Commit(); err != nil {
		t.Fatal(err)
	}
	if p.Palette.RequestedColors != 16 || p.DiscardedGraphicsBytes != 0 || p.Queue.Mode != GraphicsText {
		t.Fatal("discard changed native palette gate or parser")
	}
}

func checkOriginalContinuousSceneWorld(t *testing.T, name string, count int) World {
	t.Helper()
	f, e := os.Open("../../../docs/research/nes-contra/evidence/world-" + name + "-original.json.gz")
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
		Original bool `json:"source_is_original_execution"`
		Frames   []struct {
			Before, After, BeforePPU, AfterPPU string
			Commands                           []WorldSoundCommand
		}
	}
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	if !d.Original || len(d.Frames) != count {
		t.Fatal("incomplete startup trace")
	}
	meta := loadFrameOracle(t)
	resources := levelResources(t, "stage-1.json")
	initial := campaignSceneRAM(t, d.Frames[0].Before)
	c, a := campaignInitializeFromRAM(t, meta, d.Frames[0].Before)
	c.DelayLow, c.DelayHigh, c.GameOverDelay, c.Choice = initial[0x2a], initial[0x2b], initial[0x29], initial[0x6b]
	w := World{Active: a, Campaign: c, Scene: CampaignSceneState{IntroScroll: initial[0x70], Reveal: initial[0x7e], ExitFadeSteps: initial[0xfb]}}
	w.Active.Presentation.Display = DisplayState{Busy: initial[0x20], Mask: initial[0xfe]}
	ppu, _ := hex.DecodeString(d.Frames[0].BeforePPU)
	copy(w.Active.Presentation.Background[:], ppu[:2048])
	copy(w.Active.Presentation.Palette.Applied[:], ppu[2048:])
	for i, row := range d.Frames {
		b, a := campaignSceneRAM(t, row.Before), campaignSceneRAM(t, row.After)
		if i > 0 {
			if _, e = w.Active.Presentation.Commit(); e != nil {
				t.Fatal(i, e)
			}
		}
		if w.Active.Presentation.Display.Busy != b[0x20] {
			t.Fatal(i, "boundary busy differs", w.Active.Presentation.Display.Busy, b[0x20])
		}
		beforePPU, _ := hex.DecodeString(row.BeforePPU)
		if w.Active.Presentation.Background != [2048]uint8(beforePPU[:2048]) || w.Active.Presentation.Palette.Applied != [32]uint8(beforePPU[2048:]) {
			t.Fatal(i, "submitted PPU differs")
		}
		w.Active.Random.Value = b[0x34]
		inputs := [2]PlayerInput{}
		for actor := range 2 {
			inputs[actor] = PlayerInput{Held: Buttons(b[0xf1+actor]), Pressed: Buttons(b[0xf5+actor])}
		}
		commands, e := w.Advance(uint64(i), inputs, resources)
		if e != nil {
			t.Fatal(i, e)
		}
		_, want := campaignInitializeFromRAM(t, meta, row.After)
		for name, pair := range map[string][2]any{"combat": {w.Active.Combat, want.Combat}, "terrain": {w.Active.Terrain, want.Terrain}, "header": {w.Active.Header, want.Header}, "slots": {w.Active.Enemies.Slots, want.Enemies.Slots}, "generator": {w.Active.Enemies.Generator, want.Enemies.Generator}, "scores": {w.Active.Scores, want.Scores}, "animation": {w.Active.Animation, want.Animation}, "sprites": {w.Active.Presentation.Sprites, want.Presentation.Sprites}, "bias": {w.Active.GenerationBias, want.GenerationBias}} {
			if !reflect.DeepEqual(pair[0], pair[1]) {
				t.Fatalf("frame%d routine%d %s got%+v want%+v", i, b[0x2c], name, pair[0], pair[1])
			}
		}
		if w.Campaign.Pending != CampaignNoWork || w.Campaign.EndSequence != a[0x2d] || w.Campaign.DelayLow != a[0x2a] || w.Scene.IntroScroll != a[0x70] || w.Scene.Reveal != a[0x7e] {
			t.Fatal(i, "campaign or intro state differs", w.Campaign, w.Scene)
		}
		if w.Campaign.Continues != a[0x3a] || w.Campaign.Choice != a[0x6b] || w.Campaign.DelayHigh != a[0x2b] || w.Campaign.GameOverDelay != a[0x29] {
			t.Fatal(i, "menu/progression differs", w.Campaign)
		}
		if w.Active.Presentation.Display.Busy != a[0x20] || w.Active.Presentation.Display.Mask != a[0xfe] || w.Active.Flicker != (a[0x7eb]&1 != 0) {
			t.Fatal(i, "display controls differ")
		}
		p, wp := w.Active.Presentation.Palette, want.Presentation.Palette
		p.Applied = [32]uint8{}
		wp.Applied = [32]uint8{}
		if p != wp || w.Active.Presentation.Patterns.Desired != want.Presentation.Patterns.Desired {
			t.Fatal(i, "palette/CHR differs", p, wp)
		}
		raw, e := w.Active.Presentation.Queue.Encode()
		if e != nil {
			t.Fatal(e)
		}
		if w.Active.Presentation.Terminated {
			raw = append(raw, 0)
		}
		if hex.EncodeToString(raw) != hex.EncodeToString(a[0x700:0x700+int(a[0x21])]) || w.Active.Presentation.Queue.Mode != want.Presentation.Queue.Mode {
			t.Fatalf("frame%d queue got%x want%x", i, raw, a[0x700:0x700+int(a[0x21])])
		}
		afterPPU, _ := hex.DecodeString(row.AfterPPU)
		if w.Active.Presentation.Background != [2048]uint8(afterPPU[:2048]) || w.Active.Presentation.Palette.Applied != [32]uint8(afterPPU[2048:]) {
			t.Fatal(i, "immediate PPU differs")
		}
		if len(commands) != len(row.Commands) {
			t.Fatal(i, "sound count", commands, row.Commands)
		}
		for j := range commands {
			if commands[j] != row.Commands[j] {
				t.Fatal(i, "sound order", commands, row.Commands)
			}
		}
	}
	t.Logf("%s: %d continuous world frames with actual PPU match without state injection", name, count)
	return w
}

func TestPresentationMemoryAndDisplayBoundary(t *testing.T) {
	p := WorldPresentation{Display: DisplayState{Busy: 2, Mask: 0x1e}, Queue: GraphicsQueue{Mode: GraphicsText}}
	if e := p.Queue.AppendText(GraphicsTextPacket{Kind: 1, Address: 0x3f10, Values: []uint8{0x21}}); e != nil {
		t.Fatal(e)
	}
	if e := p.ApplyImmediate([]BackgroundWrite{{Address: 0x2800, Step: 1, Values: []uint8{7}}}); e != nil {
		t.Fatal(e)
	}
	if p.Background[0] != 7 {
		t.Fatal("nametable mirror differs")
	}
	if e := p.Seal(); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Commit(); e != nil {
		t.Fatal(e)
	}
	if p.Display.Busy != 1 || p.Display.AppliedMask != 0 || p.Palette.Applied[0] != 0x21 || p.Palette.Applied[16] != 0x21 {
		t.Fatal("palette text or blanking differs", p.Display)
	}
	if _, e := p.Commit(); e != nil {
		t.Fatal(e)
	}
	if p.Display.Busy != 0 || p.Display.AppliedMask != 0x1e {
		t.Fatal("display did not resume")
	}
	before := p
	if e := p.ApplyImmediate([]BackgroundWrite{{Address: 0x2000, Step: 1, Values: []uint8{9}}, {Address: 0x1000, Step: 1, Values: []uint8{3}}}); e == nil || !reflect.DeepEqual(p, before) {
		t.Fatal("invalid write partially changed presentation")
	}
}
