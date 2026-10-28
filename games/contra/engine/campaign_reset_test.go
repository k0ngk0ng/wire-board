package contra

// Enumerated before implementation: clear all transient fields without
// clearing lives, game-over flags, retained weapons, scores, display buffers,
// pending palette colors, snow particles or 0190..0196; apply authoritative
// command resources without advancing a level or recomputing victory; preserve
// generator bookkeeping and reject malformed command payloads atomically.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type campaignResetRecord struct {
	Kind                               string
	Case, Frame                        int
	Before, After, PPUBefore, PPUAfter string
	Sounds                             []int
}

func campaignResetRecords(t *testing.T) []campaignResetRecord {
	t.Helper()
	f, e := os.Open("../../../docs/research/nes-contra/evidence/campaign-reset-original.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var d struct{ Records []campaignResetRecord }
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	if len(d.Records) != 138 {
		t.Fatal("incomplete reset/menu original", len(d.Records))
	}
	return d.Records
}
func campaignResetFromRAM(t *testing.T, meta frameOracle, raw string) (Campaign, ActiveWorld, CampaignSceneState) {
	b := campaignSceneRAM(t, raw)
	c, w := campaignInitializeFromRAM(t, meta, raw)
	c.Continues, c.Choice, c.DelayLow, c.DelayHigh, c.Loops, c.GameOverDelay = b[0x3a], b[0x6b], b[0x2a], b[0x2b], b[0x31], b[0x29]
	c.Paused = b[0x25] != 0
	w.Enemies.Base = BaseEnemyState{b[0x86], b[0x87], b[0x88], b[0x89], b[0x8a]}
	w.Enemies.BossPartsDestroyed, w.Enemies.AlienAimIndex, w.Enemies.JointScroll = b[0x85], b[0x8b], b[0x7f]
	s := CampaignSceneState{IntroScroll: b[0x70], Reveal: b[0x7e], ExitDelay: b[0x190], ExitBudget: b[0x191], ExitActors: [2]uint8{b[0x192], b[0x193]}, ExitFadeSteps: b[0xfb], Interlude: CampaignInterludeState{FadeDelay: b[0x41], FadePhase: b[0x42], Actor: b[0x43], AnimationPhase: b[0x44], AnimationDelay: b[0x45], TextIndex: b[0x46], TextDelay: b[0x47], TextOrigin: uint16(b[0x48]) | uint16(b[0x49])<<8, TextAddress: uint16(b[0x4a]) | uint16(b[0x4b])<<8, MouthIndex: b[0x418]}}
	return c, w, s
}
func campaignResetInput(w *ActiveWorld, b []byte) CampaignInput {
	in := CampaignInput{Pressed: Buttons(b[0xf5]), CheatLives: b[0x24] != 0}
	for i, p := range w.Combat.Players {
		in.Players[i] = CampaignPlayer{State: p.State, GameOver: p.GameOver, Lives: p.Lives, Weapon: p.Weapon}
	}
	return in
}
func TestCampaignResetOriginalRuntime(t *testing.T) {
	records := campaignResetRecords(t)
	meta := loadFrameOracle(t)
	count := 0
	displays := map[int]string{}
	for _, r := range records {
		if r.Kind == "gameoverDisplay" {
			displays[r.Case] = r.Before
		}
	}
	for _, r := range records {
		if r.Kind != "transition" {
			continue
		}
		t.Run(fmt.Sprintf("case%d", r.Case), func(t *testing.T) {
			b := campaignSceneRAM(t, r.Before)
			c, w, s := campaignResetFromRAM(t, meta, r.Before)
			plan, e := c.BeginTick(uint64(r.Frame), &w.Combat.World, campaignResetInput(&w, b))
			if e != nil {
				t.Fatal(e)
			}
			var reset *CampaignReset
			for _, cmd := range plan.Commands {
				if cmd.Kind == "resetRuntime" {
					reset = cmd.Reset
				}
			}
			if reset == nil {
				t.Fatal("missing authoritative reset")
			}
			expected := r.After
			if reset.Kind == CampaignResetGameOver {
				expected = displays[r.Case]
			}
			_, want, wantScene := campaignResetFromRAM(t, meta, expected)
			// Campaign decisions have already run when this reset command is applied.
			// Loss presentation's captured entry precedes that decision in cartridge.
			if reset.Kind == CampaignResetGameOver {
				want.Combat.World.LevelRoutine = w.Combat.World.LevelRoutine
			}
			decisionWorld := w.Combat.World
			if e = ApplyCampaignRuntimeReset(&w, &s, reset); e != nil {
				t.Fatal(e)
			}
			if w.Combat.World.GameRoutine != decisionWorld.GameRoutine || w.Combat.World.Level != decisionWorld.Level || w.Combat.World.LevelRoutine != decisionWorld.LevelRoutine {
				t.Fatal("reset recomputed progression")
			}
			for key, pair := range map[string][2]any{"combat": {w.Combat, want.Combat}, "terrain": {w.Terrain, want.Terrain}, "header": {w.Header, want.Header}, "enemies": {w.Enemies, want.Enemies}, "presentation": {w.Presentation, want.Presentation}, "scores": {w.Scores, want.Scores}, "scene": {s, wantScene}, "animation": {w.Animation, want.Animation}, "snow": {w.Snow, want.Snow}, "bias": {w.GenerationBias, want.GenerationBias}, "hud": {w.HUD, want.HUD}, "flicker": {w.Flicker, want.Flicker}, "random": {w.Random, want.Random}} {
				if !reflect.DeepEqual(pair[0], pair[1]) {
					t.Fatalf("%s got%+v want%+v", key, pair[0], pair[1])
				}
			}
			count++
		})
	}
	if count != 40 {
		t.Fatal("incomplete runtime reset coverage", count)
	}
}
func TestCampaignResetRejectsAtomically(t *testing.T) {
	for _, name := range []string{"nil-world", "nil-scene", "nil-reset", "kind", "mode", "clear-stage", "clear-scores", "continue-stage", "continue-scores", "player-state"} {
		t.Run(name, func(t *testing.T) {
			w := ActiveWorld{HUD: true}
			s := CampaignSceneState{ExitDelay: 3, IntroScroll: 9}
			reset := CampaignReset{Kind: CampaignResetAdvance, ClearStageRuntime: true, PlayerMode: 1, Players: [2]CampaignPlayer{{Lives: 2, Weapon: 19}, {Lives: 4, Weapon: 2}}}
			w.Combat.World.PlayerMode = 1
			wp, sp, rp := &w, &s, &reset
			switch name {
			case "nil-world":
				wp = nil
			case "nil-scene":
				sp = nil
			case "nil-reset":
				rp = nil
			case "kind":
				reset.Kind = 99
			case "mode":
				reset.PlayerMode = 0
			case "clear-stage":
				reset.ClearStageRuntime = false
			case "clear-scores":
				reset.ClearArcadeScores = true
			case "continue-stage":
				reset.Kind = CampaignResetContinue
				reset.ClearArcadeScores = true
			case "continue-scores":
				reset.Kind = CampaignResetContinue
				reset.ClearStageRuntime = false
			case "player-state":
				reset.Players[1].State = 1
			}
			bw, bs, br := w, s, reset
			if e := ApplyCampaignRuntimeReset(wp, sp, rp); e == nil {
				t.Fatal("invalid reset accepted")
			}
			if !reflect.DeepEqual(w, bw) || s != bs || reset != br {
				t.Fatal("failed reset mutated owner or command")
			}
		})
	}
}

func TestCampaignResetOriginalContinueResources(t *testing.T) {
	meta := loadFrameOracle(t)
	count := 0
	for _, r := range campaignResetRecords(t) {
		if r.Kind != "continueCounters" {
			continue
		}
		c, w, s := campaignResetFromRAM(t, meta, r.Before)
		_, want, wantScene := campaignResetFromRAM(t, meta, r.After)
		b := campaignSceneRAM(t, r.Before)
		plan, e := c.BeginTick(uint64(r.Frame), &w.Combat.World, campaignResetInput(&w, b))
		if e != nil {
			t.Fatal(e)
		}
		var reset *CampaignReset
		for _, cmd := range plan.Commands {
			if cmd.Kind == "restartLevel" {
				reset = cmd.Reset
			}
		}
		if reset == nil {
			t.Fatal("missing continue command")
		}
		want.Combat.World.LevelRoutine = w.Combat.World.LevelRoutine
		if e = ApplyCampaignRuntimeReset(&w, &s, reset); e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(w, want) || s != wantScene {
			t.Fatalf("continue case%d got%+v want%+v scene%+v/%+v", r.Case, w, want, s, wantScene)
		}
		count++
	}
	if count != 8 {
		t.Fatal("incomplete continue resources", count)
	}
}

// Keep original complete bytes accessible for subsequent display tests; these
// helpers do not modify the game state with a recorded output.
func campaignResetQueue(t *testing.T, w *ActiveWorld, raw string) {
	t.Helper()
	b := campaignSceneRAM(t, raw)
	q := GraphicsQueue{Mode: GraphicsText}
	data := b[0x700 : 0x700+int(b[0x21])]
	for len(data) > 0 {
		if len(data) < 4 {
			t.Fatal("truncated text packet")
		}
		end := 3
		for end < len(data) && !(data[end] == 255 && (end+1 == len(data) || data[end+1] < 6)) {
			end++
		}
		if end == len(data) {
			t.Fatal("unterminated text values")
		}
		p := GraphicsTextPacket{Kind: data[0], Address: uint16(data[1])<<8 | uint16(data[2]), Values: append([]byte(nil), data[3:end]...)}
		if e := q.AppendText(p); e != nil {
			t.Fatal(e)
		}
		data = data[end+1:]
	}
	w.Presentation.Queue = q
}
func campaignResetPPU(t *testing.T, raw string) []byte {
	t.Helper()
	b, e := hex.DecodeString(raw)
	if e != nil || len(b) != 4096 {
		t.Fatal("invalid original PPU")
	}
	return b
}
