package contra

// Failure inventory is in BOT_VISIBILITY.md before implementation. These module
// boundary tests use a synthetic immutable atlas and a pure observer spy; they
// do not claim strategy quality or original-pixel fidelity (engine acceptance).
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func visualAsset(raw []byte) engine.BotVisualAsset {
	sum := sha256.Sum256(raw)
	return engine.BotVisualAsset{Bytes: len(raw), SHA256: hex.EncodeToString(sum[:])}
}
func syntheticVisualData() (engine.BotVisualCatalogManifest, []byte, map[uint8][]byte) {
	rgb := make([]byte, 192)
	for i := range rgb {
		rgb[i] = byte(i)
	}
	pages := map[uint8][]byte{}
	m := engine.BotVisualCatalogManifest{RGB: visualAsset(rgb), Pages: map[uint8]engine.BotVisualAsset{}}
	for i := uint8(0); i < 128; i++ {
		page := make([]byte, 4096)
		pages[i] = page
		m.Pages[i] = visualAsset(page)
	}
	return m, rgb, pages
}
func syntheticVisuals(t *testing.T) *BotVisualResources {
	t.Helper()
	m, rgb, pages := syntheticVisualData()
	v, e := NewBotVisualResources(m, rgb, pages)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func boundaryObserver(f engine.BotVisualFrame) (engine.BotObservation, error) {
	return engine.BotObservation{Tick: f.Tick(), Actor: f.Actor(), Seats: f.HUD().Seats}, nil
}
func boundaryVisualConfig(t *testing.T, build string) Config {
	t.Helper()
	return Config{RulesBuildID: build, Visuals: syntheticVisuals(t), ObserveBot: boundaryObserver}
}

type countedCatalog struct {
	engine.BotVisualCatalog
	frames, pages int
}

func (c *countedCatalog) RGB() ([]byte, error) { c.frames++; return c.BotVisualCatalog.RGB() }
func (c *countedCatalog) CHRPage(id uint8) ([]byte, error) {
	c.pages++
	return c.BotVisualCatalog.CHRPage(id)
}
func visualModule(t *testing.T, observe func(engine.BotVisualFrame) (engine.BotObservation, error)) (gameapi.Module, *countedCatalog) {
	t.Helper()
	base := testModule(t)
	_, c, g := initialWithModule(t, base, 2)
	_ = c
	s := saved(t, g)
	resources := syntheticVisuals(t)
	count := &countedCatalog{BotVisualCatalog: resources.catalog}
	resources.catalog = count
	m, e := ModuleWithConfig(Config{RulesBuildID: s.RulesBuildID, Visuals: resources, ObserveBot: observe})
	if e != nil {
		t.Fatal(e)
	}
	return m, count
}
func initialWithModule(t *testing.T, m gameapi.Module, n int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	c := gameapi.Context{Actor: 0, Players: n, Now: time.UnixMilli(100000), Automated: make([]bool, n)}
	g, e := m.Start(c, json.RawMessage(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	return m, c, g
}
func TestContraModuleVisualResourceConstruction(t *testing.T) {
	m, rgb, pages := syntheticVisualData()
	v, e := NewBotVisualResources(m, rgb, pages)
	if e != nil {
		t.Fatal(e)
	}
	id, e := engine.BotVisualCatalogIdentity(m)
	if e != nil || v.ID() != id {
		t.Fatal("visual identity differs")
	}
	wantRGB, _ := v.catalog.RGB()
	wantPage, _ := v.catalog.CHRPage(0)
	rgb[0] ^= 255
	pages[0][0] = 3
	delete(m.Pages, 0)
	gotRGB, _ := v.catalog.RGB()
	gotPage, _ := v.catalog.CHRPage(0)
	if !bytes.Equal(wantRGB, gotRGB) || !bytes.Equal(wantPage, gotPage) {
		t.Fatal("source mutation altered frozen assets")
	}
	gotRGB[0] ^= 255
	gotPage[0] = 3
	againRGB, _ := v.catalog.RGB()
	againPage, _ := v.catalog.CHRPage(0)
	if !bytes.Equal(wantRGB, againRGB) || !bytes.Equal(wantPage, againPage) {
		t.Fatal("caller changed retained asset")
	}
	for _, broken := range []string{"missing-page", "sha", "bad-index", "rgb"} {
		t.Run(broken, func(t *testing.T) {
			manifest, rgb, pages := syntheticVisualData()
			switch broken {
			case "missing-page":
				delete(pages, 127)
			case "sha":
				pages[127][0] = 1
			case "bad-index":
				pages[127][0] = 4
				manifest.Pages[127] = visualAsset(pages[127])
			case "rgb":
				rgb[0] ^= 255
			}
			if _, e := NewBotVisualResources(manifest, rgb, pages); e == nil {
				t.Fatal("unverified asset admitted before module setup")
			}
		})
	}
	_, _, g := initial(t, 2)
	build := saved(t, g).RulesBuildID
	for _, cfg := range []Config{{RulesBuildID: build}, {RulesBuildID: build, Visuals: v}, {RulesBuildID: build, ObserveBot: boundaryObserver}, {RulesBuildID: build, Visuals: &BotVisualResources{}, ObserveBot: boundaryObserver}} {
		if _, e := ModuleWithConfig(cfg); e == nil {
			t.Fatal("missing visual capability accepted")
		}
	}
}
func TestContraModuleBotsReceiveOnlySharedPixelsAndPublicHUD(t *testing.T) {
	var seen []engine.BotVisualFrame
	m, count := visualModule(t, func(f engine.BotVisualFrame) (engine.BotObservation, error) {
		seen = append(seen, f)
		pixels := f.Pixels()
		pixels[0] ^= 255
		return boundaryObserver(f)
	})
	_, c, g := initialWithModule(t, m, 2)
	g = ticks(t, m, &c, g, 50)
	if len(seen) != 0 || count.frames != 0 {
		t.Fatal("manual seat invoked visual strategy")
	}
	c.Automated = []bool{true, true}
	g = ticks(t, m, &c, g, 50)
	if len(seen) != 3 || count.frames != 3 {
		t.Fatal("pixels rendered or classified more than once per rule frame", len(seen), count.frames)
	}
	for _, a := range seen {
		if a.Actor() != 0 {
			t.Fatal("shared observation did not use canonical actor")
		}
		h := a.HUD()
		if h.Seats != 2 || !h.Paused.Known || h.Phase != "" || h.Stage.Known || h.Scene.Known || h.Choice.Known {
			t.Fatal("hidden phase/stage leaked", h)
		}
		for _, p := range h.Players {
			if !p.Occupied || !p.Lives.Known || p.Lives.Value != 2 || p.Weapon.Known || p.Alive.Known {
				t.Fatal("unpublished player information leaked", p)
			}
		}
	}
	before := saved(t, g)
	c.Actor = 1
	var e error
	g, e = m.Reclaim(c, g)
	if e != nil {
		t.Fatal(e)
	}
	c.Automated[1] = false
	seen = nil
	g = ticks(t, m, &c, g, 50)
	if len(seen) != 3 {
		t.Fatal("reclaimed seat still observed")
	}
	for _, f := range seen {
		if f.Actor() != 0 {
			t.Fatal("wrong automated actor")
		}
	}
	if saved(t, g).Bots[1].LastTick != before.Bots[1].LastTick {
		t.Fatal("cancelled bot advanced memory")
	}
}
func TestContraModuleHiddenRulesDoNotAlterVisualInput(t *testing.T) {
	var seen []engine.BotVisualFrame
	m, _ := visualModule(t, func(f engine.BotVisualFrame) (engine.BotObservation, error) {
		seen = append(seen, f)
		return boundaryObserver(f)
	})
	_, c, g := initialWithModule(t, m, 2)
	c.Automated = []bool{true, true}
	original := saved(t, g)
	changed := saved(t, g)
	changed.Session.World.Active.Combat.World.Level = 1
	changed.Session.World.Active.Combat.Players[0].X = 222
	changed.Session.World.Active.Combat.Players[0].Weapon = 4
	changed.Session.World.Active.Combat.Players[0].Lives = 0 // pending rules, not submitted HUD
	changed.Session.World.Active.Combat.Players[0].GameOver = 1
	changed.Session.World.Active.Enemies.Slots[0] = engine.Enemy{Kind: 17, X: 99, Y: 99, HP: 255, Sprite: 12}
	changed.Session.World.Active.Combat.Terrain[0] = 255
	other := g
	other.Data, _ = json.Marshal(changed)
	cc := c
	_ = ticks(t, m, &c, g, 17)
	_ = ticks(t, m, &cc, other, 17)
	if len(seen) != 2 {
		t.Fatal("wrong visible samples", len(seen))
	}
	for i := 0; i < 1; i++ {
		if !bytes.Equal(seen[i].Pixels(), seen[i+1].Pixels()) || seen[i].HUD() != seen[i+1].HUD() {
			t.Fatal("same final frame/HUD exposed hidden rules")
		}
	}
	zero := g
	original.Session.PresentedLives[0] = 0
	zero.Data, _ = json.Marshal(original)
	seen = nil
	cc.Now = time.UnixMilli(100000)
	_ = ticks(t, m, &cc, zero, 17)
	if len(seen) != 1 || !seen[0].HUD().Players[0].Lives.Known || seen[0].HUD().Players[0].Lives.Value != 0 {
		t.Fatal("public zero lives treated as unknown")
	}
}
func TestContraModuleVisualFailureIsTransactional(t *testing.T) {
	m, _ := visualModule(t, func(f engine.BotVisualFrame) (engine.BotObservation, error) {
		if f.Actor() == 0 {
			return engine.BotObservation{}, errors.New("classifier unavailable")
		}
		return boundaryObserver(f)
	})
	_, c, g := initialWithModule(t, m, 2)
	c.Automated = []bool{true, true}
	before := string(g.Data)
	c.Actor = -1
	c.Now = c.Now.Add(50 * time.Millisecond)
	if _, e := m.OnTimer(c, g); e == nil {
		t.Fatal("observer error ignored")
	}
	if string(g.Data) != before {
		t.Fatal("partial bot frame committed")
	}
	m, _ = visualModule(t, func(f engine.BotVisualFrame) (engine.BotObservation, error) {
		o, _ := boundaryObserver(f)
		o.Actor = 1 - o.Actor
		return o, nil
	})
	_, c, g = initialWithModule(t, m, 2)
	c.Automated = []bool{true, true}
	c.Actor = -1
	c.Now = c.Now.Add(50 * time.Millisecond)
	if _, e := m.OnTimer(c, g); e == nil {
		t.Fatal("wrong observer actor accepted")
	}
}
func TestContraModuleVisualIdentityRestoreAndLoading(t *testing.T) {
	calls := 0
	m, count := visualModule(t, func(f engine.BotVisualFrame) (engine.BotObservation, error) { calls++; return boundaryObserver(f) })
	_, c, g := initialWithModule(t, m, 2)
	bad := saved(t, g)
	bad.VisualsID = "wrong"
	invalid := g
	invalid.Data, _ = json.Marshal(bad)
	if _, e := m.Restore(c, invalid); e == nil {
		t.Fatal("visual identity mismatch restored")
	}
	g = boundary(t, g, 0)
	g = ticks(t, m, &c, g, 50)
	c.Automated = []bool{true, true}
	before := saved(t, g).Session
	g = ticks(t, m, &c, g, 50)
	if calls != 3 || count.frames != 3 || saved(t, g).Session.Random.Frames != before.Random.Frames+3 || saved(t, g).StageLoading != nil {
		t.Fatal("ready computer seats stalled at the running stage transition")
	}
	calls, count.frames = 0, 0
	_, c, g = initialWithModule(t, m, 2)
	g = ticks(t, m, &c, g, 50)
	g = input(t, m, c, g, engine.ButtonStart)
	g = ticks(t, m, &c, g, 50)
	g = input(t, m, c, g, 0)
	for i := 0; i < 20 && saved(t, g).Session.World.Active.Combat.World.LevelRoutine != 4; i++ {
		g = ticks(t, m, &c, g, 50)
	}
	g = input(t, m, c, g, engine.ButtonStart)
	g = ticks(t, m, &c, g, 50)
	if !g.Paused {
		t.Fatal("did not enter original pause")
	}
	c.Automated = []bool{true, true}
	g = ticks(t, m, &c, g, 100)
	if calls != 0 || count.frames != 0 {
		t.Fatal("paused presentation rendered a strategy frame")
	}
}
