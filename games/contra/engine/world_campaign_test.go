package contra

// Before the campaign/world join: all-dead checked after real combat and before
// enemies; 96 waiting ticks continue environment only; repeated ticks must not
// move clocks; restore keeps pending presentation and campaign counters; sound
// reset occurs at the original wait completion; failed ticks are atomic.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalContinuousCampaignWorld(t *testing.T) {
	meta := loadFrameOracle(t)
	resources := levelResources(t, "stage-1.json")
	for name, count := range map[string]int{"complete": 600, "paused": 180, "loss": 373, "boss": 6472} {
		t.Run(name, func(t *testing.T) {
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
					Before, After string
					Commands      []WorldSoundCommand
				}
			}
			if e = json.NewDecoder(z).Decode(&d); e != nil {
				t.Fatal(e)
			}
			if !d.Original || len(d.Frames) != count {
				t.Fatal("wrong complete run")
			}
			initial, _ := hex.DecodeString(d.Frames[0].Before)
			w := World{Active: activeWorldFromOriginal(t, meta, d.Frames[0].Before), Campaign: Campaign{Continues: initial[0x3a], DelayLow: initial[0x2a], DelayHigh: initial[0x2b], GameOverDelay: initial[0x29], EndSequence: initial[0x2d], Paused: initial[0x25] != 0, Loops: initial[0x31], AliveAtExit: initial[0x194]}}
			for i, row := range d.Frames {
				b, _ := hex.DecodeString(row.Before)
				a, _ := hex.DecodeString(row.After)
				if i > 0 {
					if _, e = w.Active.Presentation.Commit(); e != nil {
						t.Fatal(i, e)
					}
				}
				w.Active.Random.Value = b[0x34]
				inputs := [2]PlayerInput{}
				for actor := range 2 {
					inputs[actor] = PlayerInput{Held: Buttons(b[0xf1+actor]), Pressed: Buttons(b[0xf5+actor])}
				}
				// Restore a real in-progress world, including pending display state.
				before, _ := json.Marshal(w)
				var restored World
				if e = json.Unmarshal(before, &restored); e != nil {
					t.Fatal(e)
				}
				commands, e := w.Advance(uint64(i), inputs, resources)
				if e != nil {
					t.Fatal(i, e)
				}
				other, e := restored.Advance(uint64(i), inputs, resources)
				if e != nil || !reflect.DeepEqual(commands, other) || !reflect.DeepEqual(w, restored) {
					t.Fatal(i, "restore changed progression", e)
				}
				want := activeWorldFromOriginal(t, meta, row.After)
				for field, pair := range map[string][2]any{"combat": {w.Active.Combat, want.Combat}, "terrain": {w.Active.Terrain, want.Terrain}, "header": {w.Active.Header, want.Header}, "slots": {w.Active.Enemies.Slots, want.Enemies.Slots}, "generator": {w.Active.Enemies.Generator, want.Enemies.Generator}, "scores": {w.Active.Scores, want.Scores}, "animation": {w.Active.Animation, want.Animation}, "oam": {w.Active.Presentation.Sprites.OAM, want.Presentation.Sprites.OAM}, "bias": {w.Active.GenerationBias, want.GenerationBias}} {
					if !reflect.DeepEqual(pair[0], pair[1]) {
						t.Fatalf("frame%d %s got%+v want%+v", i, field, pair[0], pair[1])
					}
				}
				c := w.Campaign
				if w.Scene.ExitFadeSteps != a[0xfb] || w.Scene.ExitDelay != a[0x190] || w.Scene.ExitBudget != a[0x191] || w.Scene.ExitActors != [2]uint8{a[0x192], a[0x193]} {
					t.Fatal(i, "exit script counters differ", w.Scene)
				}
				if c.Pending != CampaignNoWork || c.Paused != (a[0x25] != 0) || c.Continues != a[0x3a] || c.GameOverDelay != a[0x29] || c.DelayLow != a[0x2a] || c.DelayHigh != a[0x2b] || c.EndSequence != a[0x2d] || c.AliveAtExit != a[0x194] {
					t.Fatal(i, "campaign state differs", c)
				}
				p, wp := w.Active.Presentation.Palette, want.Presentation.Palette
				p.Applied = [32]uint8{}
				wp.Applied = [32]uint8{}
				if p != wp || w.Active.Presentation.Patterns.Desired != want.Presentation.Patterns.Desired {
					t.Fatalf("frame%d palette got%+v want%+v pattern got%x want%x", i, p, wp, w.Active.Presentation.Patterns.Desired, want.Presentation.Patterns.Desired)
				}
				var queue []uint8
				for _, packet := range w.Active.Presentation.Queue.Packets {
					v, e := packet.Encode()
					if e != nil {
						t.Fatal(e)
					}
					queue = append(queue, v...)
				}
				if w.Active.Presentation.Terminated {
					queue = append(queue, 0)
				}
				if hex.EncodeToString(queue) != hex.EncodeToString(a[0x700:0x700+int(a[0x21])]) {
					t.Fatal(i, "graphics differs")
				}
				if len(commands) != len(row.Commands) {
					t.Fatal(i, "sound count", commands, row.Commands)
				}
				for j := range commands {
					if commands[j] != row.Commands[j] {
						t.Fatal(i, "sound order", commands, row.Commands)
					}
				}
				after, _ := json.Marshal(w)
				if repeated, e := w.Advance(uint64(i), inputs, resources); e != nil || len(repeated) != 0 {
					t.Fatal(i, "duplicate tick", e)
				}
				unchanged, _ := json.Marshal(w)
				if string(after) != string(unchanged) {
					t.Fatal(i, "duplicate changed world")
				}
				if i > 0 {
					if _, e = w.Advance(uint64(i-1), inputs, resources); e == nil {
						t.Fatal("backwards tick accepted")
					}
					failed, _ := json.Marshal(w)
					if string(failed) != string(after) {
						t.Fatal("failed tick changed world")
					}
				}
			}
			if name == "loss" && w.Active.Combat.World.LevelRoutine != 5 {
				t.Fatal("did not reach real continue transition")
			}
		})
	}
}
