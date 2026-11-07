package contra

// Failure inventory before implementation: initialization must not advance the
// campaign twice, clear zero-page player/life/weapon state, erase the unused map
// half, skip the base's first palette load, initialize waterfall as horizontal,
// submit desired CHR early, or play music in demo. Errors are atomic even after
// a clear/background/header phase. Whole original entry/return records include
// real dirty transient pools and nonzero preservation sentinels.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type campaignInitializeRecord struct {
	Kind                               string
	Frame                              int
	Before, After, PPUBefore, PPUAfter string
	Sounds                             []int
}
type campaignInitializeOracle struct {
	Stage, Players int
	Demo           bool
	Records        []campaignInitializeRecord
}

func campaignInitializeOracles(t *testing.T) []campaignInitializeOracle {
	t.Helper()
	paths, e := filepath.Glob("../../../docs/research/nes-contra/evidence/campaign-initialize-*-original.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	var out []campaignInitializeOracle
	for _, path := range paths {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		z, e := gzip.NewReader(bytes.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		var d campaignInitializeOracle
		e = json.NewDecoder(z).Decode(&d)
		z.Close()
		if e != nil {
			t.Fatal(e)
		}
		if len(d.Records) != 5 {
			t.Fatal("incomplete initialization record", path)
		}
		out = append(out, d)
	}
	if len(out) != 20 {
		t.Fatalf("expected 16 starts plus 4 preservation probes, got %d", len(out))
	}
	return out
}
func campaignInitializeFromRAM(t *testing.T, meta frameOracle, raw string) (Campaign, ActiveWorld) {
	b := campaignSceneRAM(t, raw)
	w := activeWorldFromOriginal(t, meta, raw)
	for i := range 16 {
		w.Presentation.Sprites.Snow[i] = SnowParticle{X: b[0x1b0+i], Y: b[0x1a0+i], Flags: b[0x7b0+i]}
	}
	for i := range 2 {
		w.Combat.Players[i].IndoorAnchorX = b[0xbe+i]
		w.Combat.Players[i].IndoorAnchorY = b[0xa8+i]
	}
	if b[0x23] == 0 {
		w.Presentation.Queue.Mode = GraphicsText
	}
	w.Presentation.Queue.Used = b[0x21]
	return Campaign{Continues: 3, EndSequence: b[0x2d], AliveAtExit: b[0x194]}, w
}
func TestCampaignInitializeOriginalPhases(t *testing.T) {
	meta := loadFrameOracle(t)
	count := 0
	for run, d := range campaignInitializeOracles(t) {
		for _, r := range d.Records {
			if r.Kind != "initialize" && r.Kind != "prepare" {
				continue
			}
			t.Run(fmt.Sprintf("run%d/%s", run, r.Kind), func(t *testing.T) {
				c, w := campaignInitializeFromRAM(t, meta, r.Before)
				wc, ww := campaignInitializeFromRAM(t, meta, r.After)
				beforeRoutine := w.Combat.World.LevelRoutine
				res := campaignSceneResources(t, d.Stage+1)
				if _, e := c.BeginTick(uint64(r.Frame), &w.Combat.World, CampaignInput{}); e != nil {
					t.Fatal(e)
				}
				var out CampaignSceneResult
				var e error
				if r.Kind == "initialize" {
					out, e = StepCampaignInitialize(&c, &w, res)
				} else {
					out, e = StepCampaignPrepare(&c, &w, res, d.Demo)
				}
				if e != nil {
					t.Fatal(e)
				}
				if w.Combat.World.LevelRoutine != beforeRoutine {
					t.Fatal("phase bypassed AfterPresentation boundary")
				}
				if _, e = c.AfterPresentation(&w.Combat.World, true); e != nil {
					t.Fatal(e)
				}
				a := campaignSceneRAM(t, r.After)
				controls := map[string]uint8{}
				for _, v := range out.Controls {
					controls[v.Kind] = v.Value
					if v.Kind == "ppu-control" {
						w.Combat.World.Scroll.NameTable = v.Value
					}
				}
				for k, v := range map[string]uint8{"display-busy": a[0x20], "ppu-control": a[0xff]} {
					if controls[k] != v {
						t.Fatalf("control %s = %d want %d", k, controls[k], v)
					}
				}
				if r.Kind == "initialize" && controls["display-mask"] != 0x1e {
					t.Fatal("missing initial PPU mask")
				}
				if c.EndSequence != wc.EndSequence || c.AliveAtExit != wc.AliveAtExit {
					t.Fatal("campaign differs", c, wc)
				}
				for key, pair := range map[string][2]any{"combat": {w.Combat, ww.Combat}, "terrain": {w.Terrain, ww.Terrain}, "header": {w.Header, ww.Header}, "enemies": {w.Enemies, ww.Enemies}, "snow": {w.Presentation.Sprites.Snow, ww.Presentation.Sprites.Snow}, "OAM": {w.Presentation.Sprites.OAM, ww.Presentation.Sprites.OAM}, "palette": {w.Presentation.Palette, ww.Presentation.Palette}, "patterns": {w.Presentation.Patterns, ww.Presentation.Patterns}, "scores": {w.Scores, ww.Scores}, "animation": {w.Animation, ww.Animation}, "snow-control": {w.Snow, ww.Snow}, "bias": {w.GenerationBias, ww.GenerationBias}} {
					if !reflect.DeepEqual(pair[0], pair[1]) {
						t.Fatalf("%s got%+v want%+v", key, pair[0], pair[1])
					}
				}
				raw, e := w.Presentation.Queue.Encode()
				if e != nil {
					t.Fatal(e)
				}
				if w.Presentation.Queue.Mode != ww.Presentation.Queue.Mode || !bytes.Equal(raw, a[0x700:0x700+int(a[0x21])]) {
					t.Fatalf("queue %x want %x", raw, a[0x700:0x700+int(a[0x21])])
				}
				var sounds []int
				for _, s := range out.Sounds {
					if s.Kind == "reset" {
						sounds = append(sounds, -1)
					} else {
						sounds = append(sounds, int(s.ID))
					}
				}
				if !reflect.DeepEqual(append([]int{}, sounds...), append([]int{}, r.Sounds...)) {
					t.Fatalf("sound order %v want %v", sounds, r.Sounds)
				}
				ppu, _ := hex.DecodeString(r.PPUBefore)
				wantPPU, _ := hex.DecodeString(r.PPUAfter)
				if len(ppu) != 4096 || len(wantPPU) != 4096 {
					t.Fatal("incomplete PPU capture")
				}
				for _, write := range out.ImmediateBackground {
					for i, v := range write.Values {
						at := (int(write.Address) - 0x2000 + i*int(write.Step)) & 0x7ff
						ppu[at] = v
						ppu[at+0x800] = v
					}
				}
				if !bytes.Equal(ppu, wantPPU) {
					t.Fatal("immediate original background differs")
				}
				count++
			})
		}
	}
	if count != 40 {
		t.Fatalf("only %d initialization phases compared", count)
	}
}

func TestCampaignInitializeFailuresAtomic(t *testing.T) {
	for _, kind := range []string{"initialize", "prepare"} {
		for _, failure := range []string{"nil-campaign", "nil-world", "nil-resources", "wrong-routine", "wrong-game", "bad-level", "unloaded-stage", "missing-map", "missing-palette", "terminated"} {
			if kind == "initialize" && (failure == "missing-map" || failure == "missing-palette") {
				continue
			}
			t.Run(kind+"/"+failure, func(t *testing.T) {
				res := campaignSceneResources(t)
				c := NewCampaign()
				w := ActiveWorld{}
				w.Combat.World = OutdoorWorld{GameRoutine: 5}
				if kind == "prepare" {
					w.Combat.World.LevelRoutine = 2
				}
				w.Presentation.Queue.Mode = GraphicsText
				w.Combat.Players[0] = Player{X: 55, Weapon: 19, Lives: 5}
				cp, wp, rp := &c, &w, res
				switch failure {
				case "nil-campaign":
					cp = nil
				case "nil-world":
					wp = nil
				case "nil-resources":
					rp = nil
				case "wrong-routine":
					w.Combat.World.LevelRoutine = 4
				case "wrong-game":
					w.Combat.World.GameRoutine = 3
				case "bad-level":
					w.Combat.World.Level = 8
				case "unloaded-stage":
					w.Combat.World.Level = 6
				case "missing-map":
					stage := res.stages[0]
					stage.Maps.Entries = nil
					res.stages[0] = stage
				case "missing-palette":
					delete(res.colors, 2)
				case "terminated":
					w.Presentation.Terminated = true
				}
				bc, bw := c, w
				var out CampaignSceneResult
				var e error
				if kind == "initialize" {
					out, e = StepCampaignInitialize(cp, wp, rp)
				} else {
					out, e = StepCampaignPrepare(cp, wp, rp, false)
				}
				if e == nil {
					t.Fatal("invalid initialization accepted")
				}
				if c != bc || !reflect.DeepEqual(w, bw) || !reflect.DeepEqual(out, CampaignSceneResult{}) {
					t.Fatal("failed phase partially changed state/effects")
				}
			})
		}
	}
}
