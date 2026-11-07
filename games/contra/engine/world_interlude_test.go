package contra

// Failure inventory before full-chain acceptance: nested F8DB falsely ends a
// direct-background frame; source header addresses 40..4B or projectile Timer0
// are compared as gameplay while the interlude owns them; stage reset clears
// retained exit resources; stage resources switch before/after the real boundary;
// display work or sound is lost across scenes; restoring a save replays progress.
// One original initial state is used. Only recorded pads and the explicitly
// external original idle-time entropy byte enter subsequent World ticks.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type interludeWorldRecord struct {
	Index, EmulatorFrame               int
	Before, After, BeforePPU, AfterPPU string
	Commands                           []WorldSoundCommand
}

func interludeWorldOriginal(t *testing.T) []interludeWorldRecord {
	t.Helper()
	f, e := os.Open("../../../docs/research/nes-contra/evidence/world-interlude-original.json.gz")
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
		Original                   bool `json:"source_is_original_execution"`
		OriginalBossRouteUnchanged bool
		OriginalBossPrefixFrames   int
		Frames                     []interludeWorldRecord
	}
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	if !d.Original || !d.OriginalBossRouteUnchanged || d.OriginalBossPrefixFrames != 6472 || len(d.Frames) != 7997 {
		t.Fatalf("incomplete or changed original route: %d", len(d.Frames))
	}
	return d.Frames
}
func interludeStateFromRAM(b []byte) CampaignInterludeState {
	return CampaignInterludeState{FadeDelay: b[0x41], FadePhase: b[0x42], Actor: b[0x43], AnimationPhase: b[0x44], AnimationDelay: b[0x45], TextIndex: b[0x46], TextDelay: b[0x47], MouthIndex: b[0x418], TextOrigin: uint16(b[0x48]) | uint16(b[0x49])<<8, TextAddress: uint16(b[0x4a]) | uint16(b[0x4b])<<8}
}
func TestOriginalContinuousInterludeWorld(t *testing.T) {
	rows := interludeWorldOriginal(t)
	meta := loadFrameOracle(t)
	b := campaignSceneRAM(t, rows[0].Before)
	w := guardianWorldFromOriginal(t, meta, rows[0].Before)
	w.Campaign.Choice, w.Campaign.Paused = b[0x6b], b[0x25] != 0
	w.Scene = CampaignSceneState{IntroScroll: b[0x70], Reveal: b[0x7e], ExitFadeSteps: b[0xfb], ExitDelay: b[0x190], ExitBudget: b[0x191], ExitActors: [2]uint8{b[0x192], b[0x193]}}
	w.Active.Presentation.Display = DisplayState{Busy: b[0x20], Mask: b[0xfe]}
	ppu, e := hex.DecodeString(rows[0].BeforePPU)
	if e != nil || len(ppu) != 2080 {
		t.Fatal("invalid initial PPU")
	}
	copy(w.Active.Presentation.Background[:], ppu[:2048])
	copy(w.Active.Presentation.Palette.Applied[:], ppu[2048:])
	resources := map[uint8]*LevelResources{0: levelResources(t, "stage-1.json"), 1: levelResources(t, "stage-2.json")}
	aliases := false
	phases := map[string]int{}
	for i, row := range rows {
		b, a := campaignSceneRAM(t, row.Before), campaignSceneRAM(t, row.After)
		if i > 0 {
			if _, e = w.Active.Presentation.Commit(); e != nil {
				t.Fatal(i, e)
			}
		}
		prefix := fmt.Sprintf("tick%d original%d stage%d routine%d seq%d", i, row.EmulatorFrame, b[0x30]+1, b[0x2c], b[0x2d])
		if w.Active.Presentation.Display.Busy != b[0x20] {
			t.Fatalf("%s boundary busy got%d want%d", prefix, w.Active.Presentation.Display.Busy, b[0x20])
		}
		beforePPU, _ := hex.DecodeString(row.BeforePPU)
		if w.Active.Presentation.Background != [2048]uint8(beforePPU[:2048]) || w.Active.Presentation.Palette.Applied != [32]uint8(beforePPU[2048:]) {
			t.Fatalf("%s submitted PPU differs", prefix)
		}
		w.Active.Random.Value = b[0x34]
		var inputs [2]PlayerInput
		for actor := range inputs {
			inputs[actor] = PlayerInput{Held: Buttons(b[0xf1+actor]), Pressed: Buttons(b[0xf5+actor])}
		}
		if resources[w.Active.Combat.World.Level] == nil {
			t.Fatal(prefix, "world selected unavailable stage")
		}
		before, _ := json.Marshal(w)
		var restored World
		if e = json.Unmarshal(before, &restored); e != nil {
			t.Fatal(e)
		}
		commands, e := w.Advance(uint64(i), inputs, resources[w.Active.Combat.World.Level])
		if e != nil {
			t.Fatal(prefix, e)
		}
		other, e := restored.Advance(uint64(i), inputs, resources[restored.Active.Combat.World.Level])
		if e != nil || !reflect.DeepEqual(w, restored) || !reflect.DeepEqual(commands, other) {
			t.Fatal(prefix, "save/restore changed world", e)
		}
		// Alias ownership is a sequence invariant, not a comparison against the
		// implementation. It lasts through the outgoing routine5 frame and ends
		// only when the original stage5 reset executes on the following tick.
		if b[0x2c] == 10 {
			aliases = true
		}
		if b[0x2c] == 5 {
			aliases = false
		}
		want := guardianWorldFromOriginal(t, meta, row.After)
		gotActive, wantActive := w.Active, want.Active
		if aliases {
			if w.Scene.Interlude != interludeStateFromRAM(a) {
				t.Fatalf("%s interlude locals got%+v want%+v", prefix, w.Scene.Interlude, interludeStateFromRAM(a))
			}
			if a[0x40] != 0 {
				t.Fatal(prefix, "first-stage interlude group changed")
			}
			// Retired gameplay owners have no source byte in this domain. Every
			// excluded byte is verified above through the actual cinematic owner.
			gotActive.Combat.World.SceneType, wantActive.Combat.World.SceneType = 0, 0
			gotActive.Combat.World.ScrollType, wantActive.Combat.World.ScrollType = 0, 0
			gotActive.Combat.Weapons.Projectiles[0].Timer, wantActive.Combat.Weapons.Projectiles[0].Timer = 0, 0
			gotActive.Terrain.AlternateGraphicsScreen, wantActive.Terrain.AlternateGraphicsScreen = 0, 0
			gotActive.Header.CollisionLimits, wantActive.Header.CollisionLimits = [3]uint8{}, [3]uint8{}
		} else if b[0x2c] == 5 && w.Scene.Interlude != (CampaignInterludeState{}) {
			t.Fatal(prefix, "stage reset retained cinematic locals")
		}
		for _, field := range []struct {
			name      string
			got, want any
		}{
			{"combat", gotActive.Combat, wantActive.Combat}, {"terrain", gotActive.Terrain, wantActive.Terrain}, {"header", gotActive.Header, wantActive.Header}, {"enemy-slots", gotActive.Enemies.Slots, wantActive.Enemies.Slots}, {"enemy-base", gotActive.Enemies.Base, wantActive.Enemies.Base}, {"generator", gotActive.Enemies.Generator, wantActive.Enemies.Generator}, {"scores", gotActive.Scores, wantActive.Scores}, {"animation", gotActive.Animation, wantActive.Animation}, {"snow", gotActive.Snow, wantActive.Snow}, {"sprites", gotActive.Presentation.Sprites, wantActive.Presentation.Sprites}, {"bias", gotActive.GenerationBias, wantActive.GenerationBias},
		} {
			if !reflect.DeepEqual(field.got, field.want) {
				t.Fatalf("%s %s got%+v want%+v", prefix, field.name, field.got, field.want)
			}
		}
		if w.Active.Enemies.ScreenReadOffset != want.Active.Enemies.ScreenReadOffset || w.Active.Random != want.Active.Random || w.Active.HUD != want.Active.HUD {
			t.Fatalf("%s persistent work/HUD: enemy offset %d/%d random %+v/%+v HUD %v/%v", prefix, w.Active.Enemies.ScreenReadOffset, want.Active.Enemies.ScreenReadOffset, w.Active.Random, want.Active.Random, w.Active.HUD, want.Active.HUD)
		}
		c := w.Campaign
		if c.Pending != CampaignNoWork || c.EndSequence != a[0x2d] || c.Continues != a[0x3a] || c.DelayLow != a[0x2a] || c.DelayHigh != a[0x2b] || c.GameOverDelay != a[0x29] || c.Paused != (a[0x25] != 0) || c.Loops != a[0x31] || c.AliveAtExit != a[0x194] {
			t.Fatal(prefix, "campaign differs", c)
		}
		if w.Scene.IntroScroll != a[0x70] || w.Scene.Reveal != a[0x7e] || w.Scene.ExitFadeSteps != a[0xfb] || w.Scene.ExitDelay != a[0x190] || w.Scene.ExitBudget != a[0x191] || w.Scene.ExitActors != [2]uint8{a[0x192], a[0x193]} {
			t.Fatal(prefix, "scene counters differ", w.Scene)
		}
		if w.Active.Presentation.Display.Busy != a[0x20] || w.Active.Presentation.Display.Mask != a[0xfe] || w.Active.Flicker != (a[0x7eb]&1 != 0) {
			t.Fatal(prefix, "display controls differ", w.Active.Presentation.Display)
		}
		pal, wpal := w.Active.Presentation.Palette, want.Active.Presentation.Palette
		pal.Applied, wpal.Applied = [32]uint8{}, [32]uint8{}
		if pal != wpal || w.Active.Presentation.Patterns.Desired != want.Active.Presentation.Patterns.Desired {
			t.Fatal(prefix, "pending palette/CHR differs")
		}
		raw, e := w.Active.Presentation.Queue.Encode()
		if e != nil {
			t.Fatal(prefix, e)
		}
		if w.Active.Presentation.Terminated {
			raw = append(raw, 0)
		}
		if hex.EncodeToString(raw) != hex.EncodeToString(a[0x700:0x700+int(a[0x21])]) || w.Active.Presentation.Queue.Mode != want.Active.Presentation.Queue.Mode {
			t.Fatalf("%s queue got%x want%x", prefix, raw, a[0x700:0x700+int(a[0x21])])
		}
		afterPPU, _ := hex.DecodeString(row.AfterPPU)
		if w.Active.Presentation.Background != [2048]uint8(afterPPU[:2048]) || w.Active.Presentation.Palette.Applied != [32]uint8(afterPPU[2048:]) {
			t.Fatal(prefix, "immediate PPU differs")
		}
		if len(commands) != len(row.Commands) {
			t.Fatal(prefix, "sound count", commands, row.Commands)
		}
		for j := range commands {
			if commands[j] != row.Commands[j] {
				t.Fatal(prefix, "sound order", commands, row.Commands)
			}
		}
		after, _ := json.Marshal(w)
		if repeated, e := w.Advance(uint64(i), inputs, resources[w.Active.Combat.World.Level]); e != nil || len(repeated) != 0 {
			t.Fatal(prefix, "duplicate tick", e)
		}
		unchanged, _ := json.Marshal(w)
		if string(after) != string(unchanged) {
			t.Fatal(prefix, "duplicate changed state")
		}
		phases[fmt.Sprintf("%d:%d", b[0x30]+1, b[0x2c])]++
	}
	if w.Active.Combat.World.Level != 1 || w.Active.Combat.World.LevelRoutine != 4 || w.Campaign.Outcome != CampaignOngoing || phases["1:10"] != 701 || phases["1:5"] != 1 || phases["2:1"] != 669 || phases["2:3"] != 32 || phases["2:4"] != 120 {
		t.Fatal("incomplete real campaign transition", phases)
	}
	t.Logf("7997 continuous original World calls, including Boss defeat, interlude, reset, second-stage intro/entrance and 120 battle calls; RAM/PPU/audio and per-frame JSON restore matched; phases=%v", phases)
}
