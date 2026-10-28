package contra

// Whole-world stage gate before claiming eight-stage integration. Each stage
// starts once from its declared original initialization and then advances 800
// complete calls with only pads and pre-frame entropy. Partial/nested NMI ends
// cannot be accepted as expected after-state; no per-frame rule state injection.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestOriginalContinuousEightStageWorld(t *testing.T) {
	meta := loadFrameOracle(t)
	for stage := 1; stage <= 8; stage++ {
		t.Run(fmt.Sprint(stage), func(t *testing.T) {
			f, e := os.Open(fmt.Sprintf("../../../docs/research/nes-contra/evidence/world-eight-stage-%d-original.json.gz", stage))
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
				Stage    int
				Frames   []struct {
					Before, After     string
					BeforeSP, AfterSP uint8
					Commands          []WorldSoundCommand
				}
			}
			if e = json.NewDecoder(z).Decode(&d); e != nil {
				t.Fatal(e)
			}
			if !d.Original || len(d.Frames) != 800 || d.Stage != stage {
				t.Fatal("wrong stage oracle")
			}
			resources := levelResources(t, fmt.Sprintf("stage-%d.json", stage))
			w := guardianWorldFromOriginal(t, meta, d.Frames[0].Before)
			for i, row := range d.Frames {
				if row.AfterSP != row.BeforeSP+2 {
					t.Fatal(i, "nested completion")
				}
				if i > 0 {
					if _, e = w.Active.Presentation.Commit(); e != nil {
						t.Fatal(i, e)
					}
				}
				b, a := campaignSceneRAM(t, row.Before), campaignSceneRAM(t, row.After)
				w.Active.Random.Value = b[0x34]
				var in [2]PlayerInput
				for actor := range in {
					in[actor] = PlayerInput{Held: Buttons(b[0xf1+actor]), Pressed: Buttons(b[0xf5+actor])}
				}
				commands, e := w.Advance(uint64(i), in, resources)
				if e != nil {
					t.Fatal(i, e)
				}
				want := guardianWorldFromOriginal(t, meta, row.After)
				for name, pair := range map[string][2]any{"combat": {w.Active.Combat, want.Active.Combat}, "terrain": {w.Active.Terrain, want.Active.Terrain}, "header": {w.Active.Header, want.Active.Header}, "slots": {w.Active.Enemies.Slots, want.Active.Enemies.Slots}, "base": {w.Active.Enemies.Base, want.Active.Enemies.Base}, "generator": {w.Active.Enemies.Generator, want.Active.Enemies.Generator}, "scores": {w.Active.Scores, want.Active.Scores}, "animation": {w.Active.Animation, want.Active.Animation}, "snow": {w.Active.Snow, want.Active.Snow}, "sprites": {w.Active.Presentation.Sprites, want.Active.Presentation.Sprites}, "work": {w.Active.Work, want.Active.Work}, "bias": {w.Active.GenerationBias, want.Active.GenerationBias}} {
					if !reflect.DeepEqual(pair[0], pair[1]) {
						t.Fatalf("frame%d %s got%+v want%+v", i, name, pair[0], pair[1])
					}
				}
				if w.Active.Enemies.AlienAimIndex != want.Active.Enemies.AlienAimIndex || w.Active.Enemies.BossPartsDestroyed != want.Active.Enemies.BossPartsDestroyed || w.Active.Enemies.JointScroll != want.Active.Enemies.JointScroll || w.Active.Enemies.ScreenReadOffset != want.Active.Enemies.ScreenReadOffset || w.Active.Random != want.Active.Random {
					t.Fatal(i, "shared enemy state differs")
				}
				p, wp := w.Active.Presentation.Palette, want.Active.Presentation.Palette
				p.Applied = [32]uint8{}
				wp.Applied = [32]uint8{}
				if p != wp || w.Active.Presentation.Patterns.Desired != want.Active.Presentation.Patterns.Desired {
					t.Fatal(i, "palette/pattern differs", p, wp)
				}
				raw, e := w.Active.Presentation.Queue.Encode()
				if e != nil {
					t.Fatal(e)
				}
				if w.Active.Presentation.Terminated {
					raw = append(raw, 0)
				}
				if hex.EncodeToString(raw) != hex.EncodeToString(a[0x700:0x700+int(a[0x21])]) {
					t.Fatal(i, "graphics queue differs")
				}
				if len(commands) != len(row.Commands) {
					t.Fatal(i, "sound count differs", commands, row.Commands)
				}
				for j := range commands {
					if commands[j] != row.Commands[j] {
						t.Fatal(i, "sound order differs", commands, row.Commands)
					}
				}
			}
		})
	}
}
