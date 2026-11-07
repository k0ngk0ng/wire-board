package contra

// Before all-stage context wiring: continue from one original live Guardian
// initial state, not from per-frame enemy snapshots; compare actual work,
// gameplay, graphics and sprite output after all phases. Only sampled pads and
// original pre-frame entropy are inputs. Audio/media is outside this oracle.
import (
	"encoding/hex"
	"reflect"
	"testing"
)

func guardianWorldFromOriginal(t *testing.T, meta frameOracle, raw string) World {
	b := campaignSceneRAM(t, raw)
	c, s := campaignInitializeFromRAM(t, meta, raw)
	c.Continues, c.DelayLow, c.DelayHigh, c.GameOverDelay, c.Loops = b[0x3a], b[0x2a], b[0x2b], b[0x29], b[0x31]
	s.Work.Value = uint16(b[0x12]) | uint16(b[0x13])<<8
	s.Enemies.Base = BaseEnemyState{b[0x86], b[0x87], b[0x88], b[0x89], b[0x8a]}
	s.Enemies.BossPartsDestroyed, s.Enemies.AlienAimIndex, s.Enemies.JointScroll = b[0x85], b[0x8b], b[0x7f]
	return World{Active: s, Campaign: c}
}

func TestOriginalContinuousGuardianWorld(t *testing.T) {
	rows := readTerrainWorkNative(t, "guardian-world-v1")
	if len(rows) != 320 {
		t.Fatal("incomplete live Guardian run")
	}
	meta := loadFrameOracle(t)
	resources := levelResources(t, "stage-8.json")
	w := guardianWorldFromOriginal(t, meta, rows[0].Before.RAM)
	for i, row := range rows {
		if i > 0 {
			if _, e := w.Active.Presentation.Commit(); e != nil {
				t.Fatal(i, e)
			}
		}
		b := campaignSceneRAM(t, row.Before.RAM)
		a := campaignSceneRAM(t, row.After.RAM)
		w.Active.Random.Value = b[0x34]
		var inputs [2]PlayerInput
		for actor := range inputs {
			inputs[actor] = PlayerInput{Held: Buttons(b[0xf1+actor]), Pressed: Buttons(b[0xf5+actor])}
		}
		if _, e := w.Advance(uint64(i), inputs, resources); e != nil {
			t.Fatal(i, e)
		}
		want := guardianWorldFromOriginal(t, meta, row.After.RAM)
		for name, pair := range map[string][2]any{"combat": {w.Active.Combat, want.Active.Combat}, "terrain": {w.Active.Terrain, want.Active.Terrain}, "header": {w.Active.Header, want.Active.Header}, "enemy-slots": {w.Active.Enemies.Slots, want.Active.Enemies.Slots}, "generator": {w.Active.Enemies.Generator, want.Active.Enemies.Generator}, "scores": {w.Active.Scores, want.Active.Scores}, "animation": {w.Active.Animation, want.Active.Animation}, "sprites": {w.Active.Presentation.Sprites, want.Active.Presentation.Sprites}, "work": {w.Active.Work, want.Active.Work}, "bias": {w.Active.GenerationBias, want.Active.GenerationBias}} {
			if !reflect.DeepEqual(pair[0], pair[1]) {
				t.Fatalf("frame%d %s got%+v want%+v", i, name, pair[0], pair[1])
			}
		}
		if w.Active.Enemies.Base != want.Active.Enemies.Base || w.Active.Enemies.AlienAimIndex != want.Active.Enemies.AlienAimIndex || w.Active.Enemies.ScreenReadOffset != want.Active.Enemies.ScreenReadOffset || w.Active.Random != want.Active.Random {
			t.Fatal(i, "enemy shared state differs")
		}
		p, wp := w.Active.Presentation.Palette, want.Active.Presentation.Palette
		p.Applied = [32]uint8{}
		wp.Applied = [32]uint8{}
		if p != wp || w.Active.Presentation.Patterns.Desired != want.Active.Presentation.Patterns.Desired {
			t.Fatal(i, "palette/CHR differs")
		}
		raw, e := w.Active.Presentation.Queue.Encode()
		if e != nil {
			t.Fatal(e)
		}
		if w.Active.Presentation.Terminated {
			raw = append(raw, 0)
		}
		if hex.EncodeToString(raw) != hex.EncodeToString(a[0x700:0x700+int(a[0x21])]) {
			t.Fatal(i, "queued graphics differ")
		}
	}
	t.Log("320 complete live Guardian frames advanced continuously with real producer wiring")
}
