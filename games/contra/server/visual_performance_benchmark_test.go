package contra

// Opt-in local performance measurement against normalized runtime graphics.
// Uses the real built-in visual knowledge and a standard Session's displayed
// pixels. No rule state is supplied to the observer; no bot victory is claimed.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func benchmarkVisualKnowledge(b *testing.B) (*BotVisualResources, *engine.BotVision, *engine.LevelBundle) {
	b.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		b.Fatal(err)
	}
	public := filepath.Join(root, ".local/contra-browser/build/public")
	raw, err := os.ReadFile(filepath.Join(public, "manifest.json"))
	if os.IsNotExist(err) {
		b.Skip("build normalized runtime assets before this opt-in benchmark")
	}
	if err != nil {
		b.Fatal(err)
	}
	var manifest struct {
		RulesBuildID string `json:"rulesBuildID"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		b.Fatal(err)
	}
	assets, err := readFixtureVisualResources(os.DirFS(public), raw, manifest.RulesBuildID)
	if err != nil {
		b.Fatal(err)
	}
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		b.Fatal(err)
	}
	vision, err := engine.NewBuiltinBotVision(assets.Catalog(), bundle)
	if err != nil {
		b.Fatal(err)
	}
	return assets, vision, bundle
}

// Static published-art baselines for all stages. These compose map zero and
// two representative characters; they do not skip a running campaign or claim
// to cover late bosses, dynamic effects, gameplay completion or worst-case load.
func BenchmarkContraStageArtworkObservation(b *testing.B) {
	assets, vision, bundle := benchmarkVisualKnowledge(b)
	for level := 0; level < 8; level++ {
		b.Run(fmt.Sprintf("stage-%d", level+1), func(b *testing.B) {
			res, err := bundle.Stage(uint8(level))
			if err != nil {
				b.Fatal(err)
			}
			info, err := res.Stage(level)
			if err != nil {
				b.Fatal(err)
			}
			screen, err := res.Screen(level, engine.RegularMap, 0, false)
			if err != nil {
				b.Fatal(err)
			}
			mapping, err := res.CHRPreset(info.CHRPreset)
			if err != nil {
				b.Fatal(err)
			}
			palette, err := res.Palette(info.Initial.PaletteIndices)
			if err != nil {
				b.Fatal(err)
			}
			sprites, err := engine.NewSpriteCompositionState()
			if err != nil {
				b.Fatal(err)
			}
			code := uint8(13)
			if info.SceneType != 0 {
				code = 80
			}
			input := engine.SpriteCompositionInput{Level: uint8(level), TwoPlayers: true}
			input.Objects[0] = engine.LogicalSprite{Code: code, X: 80, Y: 160}
			input.Objects[1] = engine.LogicalSprite{Code: code, X: 176, Y: 160, Attributes: 5}
			if _, err = sprites.Compose(input); err != nil {
				b.Fatal(err)
			}
			p := engine.PresentedFrame{OAM: sprites.OAM, Patterns: mapping, Palette: palette, Scroll: engine.CollisionScroll{NameTable: 0xb0}, Display: engine.DisplayState{AppliedMask: 0x1e}}
			copy(p.Background[:960], screen.Tiles)
			copy(p.Background[960:1024], screen.Attributes)
			hud := engine.BotPublicHUD{Seats: 2, Players: [2]engine.BotPublicPlayer{{Occupied: true}, {Occupied: true}}}
			frame, err := engine.NewBotVisualFrame(p, hud, assets.Catalog(), 0, 1)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err = engine.ObserveBotFrame(frame, vision); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkContraRealVisualObservation(b *testing.B) {
	assets, vision, bundle := benchmarkVisualKnowledge(b)
	s, err := engine.NewSession(42, 2, "visual-performance")
	if err != nil {
		b.Fatal(err)
	}
	res, err := bundle.Stage(0)
	if err != nil {
		b.Fatal(err)
	}
	for frame := uint64(1); frame <= 600; frame++ {
		var pads [2]engine.Buttons
		if frame == 2 {
			pads[0] = engine.ButtonStart
		}
		if _, err = s.StepPads(frame, pads, res); err != nil {
			b.Fatal(err)
		}
	}
	if s.World.Active.Combat.World.LevelRoutine != 4 {
		b.Fatal("visual fixture is not in battle")
	}
	hud := engine.BotPublicHUD{Seats: 2}
	for actor := range hud.Players {
		hud.Players[actor].Occupied = true
	}
	frame, err := engine.NewBotVisualFrame(s.World.Active.Presentation.Snapshot(), hud, assets.Catalog(), 0, 600)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = engine.ObserveBotFrame(frame, vision); err != nil {
			b.Fatal(err)
		}
	}
}

// Includes real frame rendering, classification, both policies and rules for
// a 50ms host batch. The initial state is standard battle with full history.
func BenchmarkContraSharedVisualTimer(b *testing.B) {
	assets, vision, _ := benchmarkVisualKnowledge(b)
	root, err := filepath.Abs("../../..")
	if err != nil {
		b.Fatal(err)
	}
	build, _, err := runtime.ReadRulesBuild(root)
	if err != nil {
		b.Fatal(err)
	}
	m, err := ModuleWithConfig(Config{RulesBuildID: build, Visuals: assets, ObserveBot: func(f engine.BotVisualFrame) (engine.BotObservation, error) {
		return engine.ObserveBotFrame(f, vision)
	}})
	if err != nil {
		b.Fatal(err)
	}
	c := gameapi.Context{Actor: -1, Players: 2, Now: time.UnixMilli(100000), Automated: []bool{false, false}}
	base, err := m.Start(c, json.RawMessage(`{}`))
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if i == 1 || i == 2 {
			base = benchmarkStartInput(b, m, c, base, i == 1)
		}
		c.Now = c.Now.Add(50 * time.Millisecond)
		base, err = m.OnTimer(c, base)
		if err != nil {
			b.Fatal(err)
		}
	}
	var state State
	if err = json.Unmarshal(base.Data, &state); err != nil {
		b.Fatal(err)
	}
	if state.Session.World.Active.Combat.World.LevelRoutine != 4 {
		b.Fatal("timer fixture is not in battle")
	}
	b.Logf("battle fixture frame=%d routine=%d seed=%d", state.Session.Random.Frames, state.Session.World.Active.Combat.World.LevelRoutine, state.Session.Random.Seed)
	c.Now = c.Now.Add(50 * time.Millisecond)
	for _, name := range []string{"one-bot", "two-bots"} {
		b.Run(name, func(b *testing.B) {
			current := c
			current.Automated = []bool{true, name == "two-bots"}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := m.OnTimer(current, base); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
