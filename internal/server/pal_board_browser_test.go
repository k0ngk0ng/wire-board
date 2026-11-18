package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	gomoku "github.com/k0ngk0ng/wire-board/games/gomoku/server"
	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func palFieldCheckpointModule(base gameapi.Module) gameapi.Module {
	module := base
	module.Start = func(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
		match, e := base.Start(c, options)
		if e != nil {
			return match, e
		}
		clock := c
		for i := 0; i < 4096; i++ {
			projected, e := base.View(match, 0)
			if e != nil {
				return match, e
			}
			v := projected.(pal.View)
			if v.Phase == "field" && v.Scene == 2 {
				imported, err := palBrowserImportClock(c, match)
				if err != nil {
					return match, err
				}
				return base.ResumeSaved(c, imported)
			}
			if v.Dialogue != nil {
				for actor := 0; actor < c.Players; actor++ {
					clock.Actor = actor
					action, _ := json.Marshal(pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
					match, e = base.Apply(clock, match, action)
					if e != nil {
						return match, e
					}
				}
			} else {
				if match.SystemDeadline == 0 {
					return match, errors.New("original field checkpoint lost its clock")
				}
				clock.Now = time.UnixMilli(match.SystemDeadline + 200)
				match, e = base.OnTimer(clock, match)
				if e != nil {
					return match, e
				}
			}
		}
		return match, errors.New("original field checkpoint did not finish")
	}
	return module
}

func palBoardModule(t *testing.T) gameapi.Module {
	return palBoardModuleAt(t, ".local/pal/native/rules.wasm")
}
func palBoardModuleAt(t *testing.T, relative string) gameapi.Module {
	t.Helper()
	root := filepath.Join("..", "..")
	wasm, e := os.ReadFile(filepath.Join(root, relative))
	if e != nil {
		t.Fatal(e)
	}
	r, e := engine.NewRuntime(context.Background(), wasm, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return pal.NewModule(r)
}

func TestPALControllerProfilesHTTP(t *testing.T) {
	s, e := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{palBoardModule(t)}}, fstest.MapFS{"index.html": {Data: []byte("PAL controller acceptance")}})
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	c := newClient(t, ts.URL)
	c.register("仙剑操作方案")
	code, settings := c.request("GET", "/api/controllers?game=pal", nil)
	if code != 200 {
		t.Fatalf("PAL controller HTTP returned %d: %v", code, settings)
	}
	models := settings["models"].([]any)
	if len(models) != 1 {
		t.Fatal("missing PAL input model")
	}
	model := models[0].(map[string]any)
	if model["layout"] != "game" || model["gameId"] != "pal" {
		t.Fatal("PAL did not declare its own logical actions", model)
	}
	profiles := settings["profiles"].(map[string]any)
	profile := profiles["game:pal"].(map[string]any)
	keys := profile["keyboard"].(map[string]any)
	for _, key := range []string{"up", "down", "left", "right", "confirm", "cancel"} {
		if keys[key] == nil || keys[key] == "" {
			t.Fatal("missing actual PAL control", key)
		}
	}
	code, all := c.request("GET", "/api/controllers", nil)
	if code != 200 {
		t.Fatal(code, all)
	}
	for id, settings := range all["profiles"].(map[string]any) {
		c.post("/api/controllers/"+id, settings, 200)
	}
}

func TestPALBoardBrowserFixture(t *testing.T) {
	if os.Getenv("PAL_BOARD_BROWSER") != "1" {
		t.Skip("explicit native PAL browser acceptance")
	}
	root, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/pal/board-browser")
	battle := os.Getenv("PAL_BOARD_ENTRY") == "battle"
	performance := os.Getenv("PAL_BOARD_ENTRY") == "performance"
	field := os.Getenv("PAL_BOARD_ENTRY") == "field" || performance
	departure := os.Getenv("PAL_BOARD_ENTRY") == "departure"
	injury := os.Getenv("PAL_BOARD_ENTRY") == "injury"
	solo := os.Getenv("PAL_BOARD_ENTRY") == "solo"
	soloInjury := os.Getenv("PAL_BOARD_ENTRY") == "solo-injury"
	soloCompanion := os.Getenv("PAL_BOARD_ENTRY") == "solo-companion"
	soloRescue := os.Getenv("PAL_BOARD_ENTRY") == "solo-rescue"
	soloStoneKey := os.Getenv("PAL_BOARD_ENTRY") == "solo-stone-key"
	soloMedicine := os.Getenv("PAL_BOARD_ENTRY") == "solo-medicine"
	soloJadeBead := os.Getenv("PAL_BOARD_ENTRY") == "solo-jade-bead"
	soloBloodPoolWave := os.Getenv("PAL_BOARD_ENTRY") == "solo-blood-pool-wave"
	soloBloodPool := os.Getenv("PAL_BOARD_ENTRY") == "solo-blood-pool" || soloBloodPoolWave
	soloEarthOrb := os.Getenv("PAL_BOARD_ENTRY") == "solo-earth-orb"
	soloEarthEscape := os.Getenv("PAL_BOARD_ENTRY") == "solo-earth-escape"
	soloGhostMountain := os.Getenv("PAL_BOARD_ENTRY") == "solo-ghost-mountain"
	soloYangzhou := os.Getenv("PAL_BOARD_ENTRY") == "solo-yangzhou"
	soloGaijiao := os.Getenv("PAL_BOARD_ENTRY") == "solo-gaijiao"
	soloCapitalRitual := os.Getenv("PAL_BOARD_ENTRY") == "solo-capital-ritual"
	soloCaiyiSacrifice := os.Getenv("PAL_BOARD_ENTRY") == "solo-caiyi-sacrifice"
	soloShuGifts := os.Getenv("PAL_BOARD_ENTRY") == "solo-shu-gifts"
	soloTowerSage := os.Getenv("PAL_BOARD_ENTRY") == "solo-tower-sage"
	soloTowerBridges := os.Getenv("PAL_BOARD_ENTRY") == "solo-tower-bridges"
	soloTowerEquipment := os.Getenv("PAL_BOARD_ENTRY") == "solo-tower-equipment"
	soloTowerDemons := os.Getenv("PAL_BOARD_ENTRY") == "solo-tower-demons"
	soloTowerCollapse := os.Getenv("PAL_BOARD_ENTRY") == "solo-tower-collapse"
	soloAnuPromise := os.Getenv("PAL_BOARD_ENTRY") == "solo-anu-promise"
	soloKirinHuman := os.Getenv("PAL_BOARD_ENTRY") == "solo-kirin-human"
	soloPastQueen := os.Getenv("PAL_BOARD_ENTRY") == "solo-past-queen"
	soloPastSacrifice := os.Getenv("PAL_BOARD_ENTRY") == "solo-past-sacrifice"
	soloLingyue := os.Getenv("PAL_BOARD_ENTRY") == "solo-lingyue"
	soloWaterOrb := os.Getenv("PAL_BOARD_ENTRY") == "solo-water-orb"
	soloYiru := os.Getenv("PAL_BOARD_ENTRY") == "solo-yiru"
	soloLingRecovery := os.Getenv("PAL_BOARD_ENTRY") == "solo-ling-recovery"
	soloPrayerMother := os.Getenv("PAL_BOARD_ENTRY") == "solo-prayer-mother"
	soloPrayerRitual := os.Getenv("PAL_BOARD_ENTRY") == "solo-prayer-ritual"
	soloPrayerFarewell := os.Getenv("PAL_BOARD_ENTRY") == "solo-prayer-farewell"
	soloHiddenLoot := os.Getenv("PAL_BOARD_ENTRY") == "solo-hidden-loot"
	candidateWave := os.Getenv("PAL_BOARD_VARIANT") == "scene-wave"
	candidateAudio := os.Getenv("PAL_BOARD_VARIANT") == "audio"
	if variant := os.Getenv("PAL_BOARD_VARIANT"); variant != "" && (!candidateWave || !(soloMedicine || soloJadeBead || soloBloodPool || soloEarthOrb || soloEarthEscape || soloGhostMountain || soloYangzhou || soloGaijiao || soloCapitalRitual || soloCaiyiSacrifice || soloShuGifts || soloTowerSage || soloTowerBridges || soloTowerEquipment || soloTowerDemons || soloTowerCollapse || soloAnuPromise || soloKirinHuman || soloPastQueen || soloPastSacrifice || soloLingyue || soloWaterOrb || soloYiru || soloLingRecovery || soloPrayerMother || soloPrayerRitual || soloPrayerFarewell)) && !(candidateAudio && soloPrayerRitual) {
		t.Fatal("unsupported candidate browser entry", variant, os.Getenv("PAL_BOARD_ENTRY"))
	}
	if (soloGhostMountain || soloYangzhou || soloGaijiao || soloCapitalRitual || soloCaiyiSacrifice || soloShuGifts || soloTowerSage || soloTowerBridges || soloTowerEquipment || soloTowerDemons || soloTowerCollapse || soloAnuPromise || soloKirinHuman || soloPastQueen || soloPastSacrifice || soloLingyue || soloWaterOrb || soloYiru || soloLingRecovery || soloPrayerMother || soloPrayerRitual || soloPrayerFarewell) && !candidateWave && !candidateAudio {
		t.Fatal("later-story browser entry requires its actual scene-wave campaign core")
	}
	usesWaveCore := soloBloodPoolWave || candidateWave
	if battle {
		out = filepath.Join(root, ".local/pal/battle-board-browser")
	}
	if field {
		out = filepath.Join(root, ".local/pal/field-board-browser")
	}
	if performance {
		out = filepath.Join(root, ".local/pal/performance-board-browser")
	}
	if departure {
		out = filepath.Join(root, ".local/pal/departure-board-browser")
	}
	if injury {
		out = filepath.Join(root, ".local/pal/injury-board-browser")
	}
	if solo {
		out = filepath.Join(root, ".local/pal/solo-board-browser")
	}
	if soloInjury {
		out = filepath.Join(root, ".local/pal/solo-injury-browser")
	}
	if soloCompanion {
		out = filepath.Join(root, ".local/pal/solo-companion-browser")
	}
	if soloRescue {
		out = filepath.Join(root, ".local/pal/solo-rescue-browser")
	}
	if soloStoneKey {
		out = filepath.Join(root, ".local/pal/solo-stone-key-browser")
	}
	if soloMedicine {
		out = filepath.Join(root, ".local/pal/solo-medicine-browser")
	}
	if soloJadeBead {
		out = filepath.Join(root, ".local/pal/solo-jade-bead-browser")
	}
	if soloBloodPool {
		out = filepath.Join(root, ".local/pal/solo-blood-pool-browser")
	}
	if soloBloodPoolWave {
		out = filepath.Join(root, ".local/pal/solo-blood-pool-wave-browser")
	}
	if soloEarthOrb {
		out = filepath.Join(root, ".local/pal/solo-earth-orb-browser")
	}
	if soloEarthEscape {
		out = filepath.Join(root, ".local/pal/solo-earth-escape-browser")
	}
	if soloGhostMountain {
		out = filepath.Join(root, ".local/pal/solo-ghost-mountain-browser")
	}
	if soloYangzhou {
		out = filepath.Join(root, ".local/pal/solo-yangzhou-browser")
	}
	if soloGaijiao {
		out = filepath.Join(root, ".local/pal/solo-gaijiao-browser")
	}
	if soloCapitalRitual {
		out = filepath.Join(root, ".local/pal/solo-capital-ritual-browser")
	}
	if soloCaiyiSacrifice {
		out = filepath.Join(root, ".local/pal/solo-caiyi-sacrifice-browser")
	}
	if soloShuGifts {
		out = filepath.Join(root, ".local/pal/solo-shu-gifts-browser")
	}
	if soloTowerSage {
		out = filepath.Join(root, ".local/pal/solo-tower-sage-browser")
	}
	if soloTowerBridges {
		out = filepath.Join(root, ".local/pal/solo-tower-bridges-browser")
	}
	if soloTowerEquipment {
		out = filepath.Join(root, ".local/pal/solo-tower-equipment-browser")
	}
	if soloTowerDemons {
		out = filepath.Join(root, ".local/pal/solo-tower-demons-browser")
	}
	if soloTowerCollapse {
		out = filepath.Join(root, ".local/pal/solo-tower-collapse-browser")
	}
	if soloAnuPromise {
		out = filepath.Join(root, ".local/pal/solo-anu-promise-browser")
	}
	if soloKirinHuman {
		out = filepath.Join(root, ".local/pal/solo-kirin-human-browser")
	}
	if soloPastQueen {
		out = filepath.Join(root, ".local/pal/solo-past-queen-browser")
	}
	if soloPastSacrifice {
		out = filepath.Join(root, ".local/pal/solo-past-sacrifice-browser")
	}
	if soloLingyue {
		out = filepath.Join(root, ".local/pal/solo-lingyue-browser")
	}
	if soloWaterOrb {
		out = filepath.Join(root, ".local/pal/solo-water-orb-browser")
	}
	if soloYiru {
		out = filepath.Join(root, ".local/pal/solo-yiru-browser")
	}
	if soloLingRecovery {
		out = filepath.Join(root, ".local/pal/solo-ling-recovery-browser")
	}
	if soloPrayerMother {
		out = filepath.Join(root, ".local/pal/solo-prayer-mother-browser")
	}
	if soloPrayerRitual {
		out = filepath.Join(root, ".local/pal/solo-prayer-ritual-browser")
	}
	if soloPrayerFarewell {
		out = filepath.Join(root, ".local/pal/solo-prayer-farewell-browser")
	}
	if soloHiddenLoot {
		out = filepath.Join(root, ".local/pal/solo-hidden-loot-browser")
	}
	if candidateWave {
		out = filepath.Join(root, ".local/pal/candidate-scene-wave", filepath.Base(out))
	}
	if candidateAudio {
		out = filepath.Join(root, ".local/pal/candidate-audio", filepath.Base(out))
	}
	if os.Getenv("PAL_VISUAL_CDN") == "1" {
		out = filepath.Join(filepath.Dir(out), "cdn-"+filepath.Base(out))
	}
	recoveryScope := os.Getenv("PAL_RECOVERY_SCOPE")
	if recoveryScope != "" {
		if !battle || os.Getenv("PAL_SCREEN_BATTLE_CURRENT") != "1" || !regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`).MatchString(recoveryScope) {
			t.Fatal("invalid battle recovery fixture scope", recoveryScope)
		}
		out = filepath.Join(root, ".local/pal/performance/battle-recovery-"+recoveryScope)
	}
	denseScope := os.Getenv("PAL_DENSE_BATTLE_SCOPE")
	denseNative := os.Getenv("PAL_DENSE_BATTLE_NATIVE")
	if denseNative != "" && (denseScope == "" || (denseNative != "candidate" && denseNative != "summon")) {
		t.Fatal("dense battle candidate requires an isolated fixture")
	}
	if denseScope != "" {
		if recoveryScope != "" || !battle || os.Getenv("PAL_SCREEN_BATTLE_CURRENT") != "1" || !regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`).MatchString(denseScope) {
			t.Fatal("invalid dense battle fixture scope", denseScope)
		}
		out = filepath.Join(root, ".local/pal/performance/dense-battle-"+denseScope)
	}
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(out, "stop")); e != nil && !os.IsNotExist(e) {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(out, "hold-ws")); e != nil && !os.IsNotExist(e) {
		t.Fatal(e)
	}
	var module gameapi.Module
	if candidateAudio {
		var checkpoint []byte
		module, checkpoint = palCampaignBrowserModule(t, "prayer-ritual")
		if err := os.WriteFile(filepath.Join(out, "entry-state.json"), checkpoint, 0600); err != nil {
			t.Fatal(err)
		}
	} else if performance {
		var build string
		module, build = palPerformanceModule(t)
		identity, err := json.Marshal(map[string]any{"buildID": build, "profile": "original-98", "players": 1, "ruleOnlyBundle": true, "entry": "actual new-game opening to original inn"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "identity.json"), identity, 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		module = palBoardModule(t)
	}
	if soloHiddenLoot || usesWaveCore {
		load := func(path string) *engine.Runtime {
			program, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			r, err := engine.NewRuntime(context.Background(), program, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { r.Close(context.Background()) })
			return r
		}
		current := ".local/pal/runtime-candidates/original/rules.wasm"
		retained := ".local/pal/runtime-versions/7ca83b5bcf13e71c1020ef287d9817e5675adf2cbdf7c627644f4cb3566dd03a/rules.wasm"
		if usesWaveCore {
			current = ".local/pal/runtime-candidates/scene-wave/hp-9999/rules.wasm"
			retained = ".local/pal/runtime-versions/68d893938bc623a69a33d9ca402741b04a7be2191c7349f4081b5e94b755bec7/rules.wasm"
		}
		catalog, err := engine.NewRuntimeCatalog(load(current), load(retained))
		if err != nil {
			t.Fatal(err)
		}
		module = pal.NewVersionedModule(catalog)
	}
	if !usesWaveCore && (soloMedicine || soloJadeBead || soloBloodPool || soloEarthOrb || soloEarthEscape) {
		module = palBoardModuleAt(t, ".local/pal/test-health/hp-9999/campaign/rules.wasm")
	}
	if battle {
		core := ".local/pal/battle-fixtures/team-1-party-3-field-18/rules.wasm"
		if os.Getenv("PAL_SCREEN_BATTLE_CURRENT") == "1" {
			core = ".local/pal/screen-input/battle-current/rules.wasm"
		}
		if denseNative == "candidate" {
			core = ".local/pal/dense-battle-candidate/battle/rules.wasm"
		} else if denseNative == "summon" {
			core = ".local/pal/summon-wind/native/rules.wasm"
		}
		module = palBoardModuleAt(t, core)
	}
	if denseScope != "" {
		start := module.Start
		module.Start = func(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
			input, err := json.Marshal(map[string]any{"time": c.Now.Format(time.RFC3339Nano), "version": c.Version, "players": c.Players})
			if err != nil {
				return gameapi.Match{}, err
			}
			if err := os.WriteFile(filepath.Join(out, "start.json"), input, 0600); err != nil {
				return gameapi.Match{}, err
			}
			return start(c, options)
		}
	}
	if denseNative == "summon" {
		module = palSummonBrowserTrace(module, out)
	}
	if field {
		module = palFieldCheckpointModule(module)
	}
	if performance && os.Getenv("PAL_SELECTION_MENU_ENTRY") == "1" {
		module = palPerformanceMenuEntry(module)
	}
	var ticksMu sync.Mutex
	tickSamples := []map[string]any{}
	cadenceTrace := battle && (os.Getenv("PAL_CADENCE_TRACE") == "1" || denseScope != "")
	if performance || cadenceTrace {
		timer := module.OnTimer
		module.OnTimer = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
			begin := time.Now()
			next, err := timer(c, g)
			elapsed := float64(time.Since(begin)) / float64(time.Millisecond)
			sample := map[string]any{"at": begin.UnixMilli(), "elapsedMS": elapsed, "players": c.Players, "failed": err != nil, "nowMS": c.Now.UnixMilli(), "beforeDueMS": g.SystemDeadline, "afterDueMS": next.SystemDeadline}
			if denseNative == "summon" {
				sample["epoch"] = palSummonTimerEpoch(next.Data)
			}
			ticksMu.Lock()
			tickSamples = append(tickSamples, sample)
			ticksMu.Unlock()
			return next, err
		}
	}
	if !candidateAudio && (departure || injury || soloInjury || soloCompanion || soloRescue || soloStoneKey || soloMedicine || soloJadeBead || soloBloodPool || soloEarthOrb || soloEarthEscape || soloGhostMountain || soloYangzhou || soloGaijiao || soloCapitalRitual || soloCaiyiSacrifice || soloShuGifts || soloTowerSage || soloTowerBridges || soloTowerEquipment || soloTowerDemons || soloTowerCollapse || soloAnuPromise || soloKirinHuman || soloPastQueen || soloPastSacrifice || soloLingyue || soloWaterOrb || soloYiru || soloLingRecovery || soloPrayerMother || soloPrayerRitual || soloPrayerFarewell || soloHiddenLoot) {
		name := "campaign-departure-state.json"
		if injury {
			name = "yueru-injury-state.json"
		}
		if soloInjury {
			name = "yueru-injury-state-1.json"
		}
		if soloCompanion {
			name = "tournament-companion-state-1.json"
		}
		if soloRescue {
			name = "hidden-dragon-rescue-plan-state-1.json"
		}
		if soloStoneKey {
			name = "hidden-dragon-key-selected-state-1.json"
		}
		if soloMedicine {
			name = "development-health/hp-9999/white-river-medicine-selected-state-1.json"
		}
		if soloJadeBead {
			name = "development-health/hp-9999/jade-temple-named-state-1.json"
		}
		if soloBloodPool {
			name = "development-health/hp-9999/ghost-tomb-pool-reading-state-1.json"
		}
		if soloEarthOrb {
			name = "development-health/hp-9999/blood-pool-legend-reading-state-1.json"
		}
		if soloEarthEscape {
			name = "development-health/hp-9999/blood-pool-earth-orb-selected-state-1.json"
		}
		if soloGhostMountain {
			name = "development-health/hp-9999/ghost-mountain-ling-leaves-reading-state-1.json"
		}
		if soloYangzhou {
			name = "development-health/hp-9999/yangzhou-freedom-reading-state-1.json"
		}
		if soloGaijiao {
			name = "development-health/hp-9999/toad-mountain-ling-taken-reading-state-1.json"
		}
		if soloCapitalRitual {
			name = "development-health/hp-9999/capital-manor-ritual-start-reading-state-1.json"
		}
		if soloCaiyiSacrifice {
			name = "development-health/hp-9999/caiyi-forest-last-wish-reading-state-1.json"
		}
		if soloShuGifts {
			name = "development-health/hp-9999/shu-tower-wine-god-limit-reading-state-1.json"
		}
		if soloTowerSage {
			name = "development-health/hp-9999/tower-upper-sage-joins-reading-state-1.json"
		}
		if soloTowerBridges {
			name = "development-health/hp-9999/tower-layer-input-source-state-1.json"
		}
		if soloTowerEquipment {
			name = "development-health/hp-9999/tower-equipment-sword-role-selected-state-1.json"
		}
		if soloTowerDemons {
			name = "development-health/hp-9999/tower-bottom-demons-plead-reading-state-1.json"
		}
		if soloTowerCollapse {
			name = "development-health/hp-9999/tower-bottom-pillars-destroyed-reading-state-1.json"
		}
		if soloAnuPromise {
			name = "development-health/hp-9999/sacred-tree-anu-agreement-reading-state-1.json"
		}
		if soloKirinHuman {
			name = "development-health/hp-9999/dali-kirin-human-form-reading-state-1.json"
		}
		if soloPastQueen {
			name = "development-health/hp-9999/temple-past-staff-quest-reading-state-1.json"
		}
		if soloPastSacrifice {
			name = "development-health/hp-9999/temple-past-queen-entrusts-reading-state-1.json"
		}
		if soloLingyue {
			name = "development-health/hp-9999/old-yuhang-cloth-recognized-reading-state-1.json"
		}
		if soloWaterOrb {
			name = "development-health/hp-9999/old-yuhang-exchange-reading-state-1.json"
		}
		if soloYiru {
			name = "development-health/hp-9999/sacred-medicine-yiru-name-reading-state-1.json"
		}
		if soloLingRecovery {
			name = "development-health/hp-9999/trial-worm-ling-rest-reading-state-1.json"
		}
		if soloPrayerMother {
			name = "development-health/hp-9999/prayer-mother-greeting-reading-state-1.json"
		}
		if soloPrayerRitual {
			name = "development-health/hp-9999/prayer-orb-selected-4-state-1.json"
		}
		if soloPrayerFarewell {
			name = "development-health/hp-9999/prayer-pursue-mastermind-reading-state-1.json"
		}
		if soloHiddenLoot {
			name = "hidden-inn-event-5-unpicked-state-1.json"
		}
		if usesWaveCore {
			name = "runtime-candidates/scene-wave/" + name
		}
		checkpoint, err := os.ReadFile(filepath.Join(root, ".local/pal/verification", name))
		if err != nil {
			t.Fatal("run the original campaign E2E first", err)
		}
		if err := os.WriteFile(filepath.Join(out, "entry-state.json"), checkpoint, 0600); err != nil {
			t.Fatal(err)
		}
		resume := module.ResumeSaved
		module.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
			return resume(c, gameapi.Match{Schema: 1, Data: append(json.RawMessage(nil), checkpoint...)})
		}
	}
	if candidateWave || candidateAudio {
		freeze := module.SaveControl
		module.SaveControl = func(c gameapi.Context, g gameapi.Match, paused bool) (gameapi.Match, error) {
			next, err := freeze(c, g, paused)
			if err != nil || !paused {
				return next, err
			}
			visible, err := module.View(next, 0)
			if err != nil {
				return next, err
			}
			data, err := json.Marshal(visible)
			if err == nil {
				err = os.WriteFile(filepath.Join(out, "frozen-view.json"), data, 0600)
			}
			return next, err
		}
		resume := module.ResumeSaved
		module.ResumeSaved = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
			next, err := resume(c, g)
			if err != nil {
				return next, err
			}
			visible, err := module.View(next, 0)
			if err != nil {
				return next, err
			}
			data, err := json.Marshal(visible)
			if err == nil {
				err = os.WriteFile(filepath.Join(out, "restored-boundary-view.json"), data, 0600)
			}
			return next, err
		}
	}
	if battle && os.Getenv("PAL_SCREEN_TIMER_TRACE") == "1" {
		trace, err := os.Create(filepath.Join(out, "timer-trace.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { trace.Close() })
		timer := module.OnTimer
		calls := 0
		module.OnTimer = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
			begin := time.Now()
			next, err := timer(c, g)
			calls++
			if calls <= 400 || calls%20 == 0 {
				v, viewErr := module.View(next, 0)
				if viewErr == nil {
					p := v.(pal.View)
					row, _ := json.Marshal(map[string]any{"call": calls, "now": c.Now.UnixMilli(), "deadlineBefore": g.SystemDeadline, "deadlineAfter": next.SystemDeadline, "controlAt": p.ControlAt, "phase": p.Phase, "battle": p.Battle, "elapsedMS": time.Since(begin).Milliseconds(), "error": fmt.Sprint(err)})
					fmt.Fprintln(trace, string(row))
				}
			}
			return next, err
		}
	}
	webDist := filepath.Join(root, "web/dist")
	if battle || performance {
		switch os.Getenv("PAL_SELECTION_WEB") {
		case "1":
			webDist = filepath.Join(root, ".local/pal/selection-web-dist")
		case "baseline":
			webDist = filepath.Join(root, ".local/pal/selection-baseline-dist")
		}
	}
	if recoveryScope != "" || denseScope != "" {
		webDist = filepath.Join(out, "web-dist")
		if _, err := os.Stat(filepath.Join(webDist, "index.html")); err != nil {
			t.Fatal(err)
		}
	}
	s, e := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", ClientLoading: true, Modules: []gameapi.Module{module, gomoku.Module()}}, os.DirFS(webDist))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	assets := http.StripPrefix("/pal/v1/", http.FileServer(http.FS(os.DirFS(filepath.Join(root, ".local/pal/visual/pal/v1")))))
	mux := http.NewServeMux()
	if performance || cadenceTrace {
		mux.HandleFunc("/pal-performance/metrics", func(w http.ResponseWriter, r *http.Request) {
			ticksMu.Lock()
			defer ticksMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tickSamples)
		})
	}
	if soloBloodPoolWave {
		mux.HandleFunc("/pal-renderer-probe.js", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(out, "renderer-probe.js"))
		})
	}
	mux.HandleFunc("/pal/v1/", func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("PAL_VISUAL_CDN") == "1" {
			http.NotFound(w, r)
			return
		}
		if _, e := os.Stat(filepath.Join(out, "asset-fail")); e == nil {
			http.Error(w, "test asset unavailable", 503)
			return
		}
		assets.ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		for began := time.Now(); time.Since(began) < 15*time.Second; {
			if _, e := os.Stat(filepath.Join(out, "hold-ws")); os.IsNotExist(e) {
				break
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
		s.Handler().ServeHTTP(w, r)
	})
	mux.Handle("/", s.Handler())
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	h := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(listener)
	if e = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); e != nil {
		t.Fatal(e)
	}
	for began := time.Now(); time.Since(began) < 30*time.Minute; {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("PAL browser fixture timed out")
}
