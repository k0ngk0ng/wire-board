package pokepath_test

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func settingsStart(t *testing.T) *engine.State {
	t.Helper()
	state, err := engine.New(engine.Options{Starter: "charmander"}, 2, 123456, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func settingsAct(t *testing.T, state *engine.State, action engine.Action) {
	t.Helper()
	action.Epoch = state.Epoch
	if err := engine.Apply(state, 1, action); err != nil {
		t.Fatal(err)
	}
}

func TestNativeSettingsCompleteOriginalValuesAndColdRestore(t *testing.T) {
	state := settingsStart(t)
	for _, name := range []string{"autoStop", "autoStopBoss", "indicatorField", "indicatorShape", "fastTarget", "durationCC", "particles", "disableTagColors"} {
		settingsAct(t, state, engine.Action{Type: "settings", Name: name, Enabled: true})
	}
	for _, name := range []string{"autoReset", "displayHealth", "mapEffectsMode"} {
		settingsAct(t, state, engine.Action{Type: "settings", Name: name, Value: 2})
	}
	for _, name := range []string{"projectiles", "areas", "beams", "waves", "auras", "bombardment", "special"} {
		settingsAct(t, state, engine.Action{Type: "settings", Name: "effectOpacity." + name, Value: 35})
	}
	for _, name := range []string{"0", "1", "2", "3", "4"} {
		settingsAct(t, state, engine.Action{Type: "settings", Name: "mute." + name, Enabled: true})
	}
	before, _ := engine.Marshal(state)
	for _, action := range []engine.Action{{Type: "settings", Name: "autoReset", Value: 3}, {Type: "settings", Name: "displayHealth", Value: -1}, {Type: "settings", Name: "mapEffectsMode", Value: 1.5}, {Type: "settings", Name: "effectOpacity.projectiles", Value: 34}, {Type: "settings", Name: "effectOpacity.unknown", Value: 50}, {Type: "settings", Name: "mute.5", Enabled: true}} {
		action.Epoch = state.Epoch
		if err := engine.Apply(state, 0, action); err == nil {
			t.Fatal("accepted invalid original setting")
		}
		after, _ := engine.Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid setting changed the shared state")
		}
	}
	restored, err := engine.Unmarshal(before)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(engine.Project(state).Progression.Settings)
	right, _ := json.Marshal(engine.Project(restored).Progression.Settings)
	if !bytes.Equal(left, right) {
		t.Fatal("cold restore lost original display, mute or automatic settings")
	}
	var values map[string]any
	_ = json.Unmarshal(left, &values)
	if values["autoReset"] != float64(2) || values["displayHealth"] != float64(2) || values["mapEffectsMode"] != float64(2) || values["colorTags"] != false || values["showHealth"] != true || len(values["mute"].([]any)) != 5 || len(values["effectOpacity"].(map[string]any)) != 7 {
		t.Fatal("settings projection omitted an original setting or compatibility value")
	}
}

func TestNativeSettingsAutoStopBossUsesTheOriginalWaveBoundaries(t *testing.T) {
	for _, scenario := range []struct {
		route int
		wave  int
		stop  bool
	}{{0, 99, true}, {0, 49, false}, {12, 49, true}, {12, 48, false}} {
		state := settingsStart(t)
		state.Route = scenario.route
		state.Wave = scenario.wave
		state.AutoWave = true
		settingsAct(t, state, engine.Action{Type: "settings", Name: "autoStopBoss", Enabled: true})
		settingsAct(t, state, engine.Action{Type: "wave"})
		for index := range state.Enemies {
			state.Enemies[index].HP = 0
		}
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
		if state.Wave != scenario.wave+1 || state.AutoWave == scenario.stop || state.WaveActive == scenario.stop {
			t.Fatalf("wrong automatic boss boundary for route=%d wave=%d: wave=%d auto=%v active=%v", scenario.route, scenario.wave, state.Wave, state.AutoWave, state.WaveActive)
		}
	}
}

func TestNativeSettingsAutomaticRestartAndCheckpointRetryUseRealDefeat(t *testing.T) {
	for _, scenario := range []struct {
		mode  float64
		wave  int
		next  int
		lives int
	}{{1, 1, 1, 14}, {1, 51, 1, 14}, {2, 51, 50, 7}, {2, 100, 100, 1}} {
		state := settingsStart(t)
		state.Health = 1
		state.Wave = scenario.wave
		settingsAct(t, state, engine.Action{Type: "settings", Name: "autoReset", Value: scenario.mode})
		settingsAct(t, state, engine.Action{Type: "wave"})
		for index := 0; index < 500 && state.Progression.Profile.Resets == 0; index++ {
			if err := engine.Advance(state, 1000); err != nil {
				t.Fatal(err)
			}
		}
		if state.Progression.Profile.Resets != 1 || state.Wave != scenario.next || state.Health != scenario.lives || state.Phase != "prepare" || state.WaveActive {
			t.Fatalf("automatic defeat handling differs at mode=%v wave=%d: next=%d health=%d phase=%s resets=%d", scenario.mode, scenario.wave, state.Wave, state.Health, state.Phase, state.Progression.Profile.Resets)
		}
	}
}

func TestNativeSettingsAutoStopInvisibleLeavesTheNextWavePrepared(t *testing.T) {
	state := settingsStart(t)
	route := engine.CatalogData().Routes["0"]
	next := 0
	for wave := 2; wave <= 100 && next == 0; wave++ {
		for _, key := range route.Waves[strconv.Itoa(wave)].Preview {
			if key == "absol" || key == "kecleon" || key == "ditto" || key == "sableye" {
				next = wave
				break
			}
		}
	}
	if next == 0 {
		t.Fatal("formal route has no selected invisible preview fixture")
	}
	state.Wave = next - 1
	state.AutoWave = true
	settingsAct(t, state, engine.Action{Type: "settings", Name: "autoStop", Enabled: true})
	settingsAct(t, state, engine.Action{Type: "wave"})
	for index := range state.Enemies {
		state.Enemies[index].HP = 0
	}
	if err := engine.Advance(state, 1000); err != nil {
		t.Fatal(err)
	}
	if state.Wave != next || state.WaveActive || state.AutoWave {
		t.Fatal("automatic waves ignored the next invisible enemy preview")
	}
}

func TestNativeSettingsPermadeathNeverAutomaticallyRestarts(t *testing.T) {
	state := settingsStart(t)
	state.Health = 1
	state.Progression.Challenge = engine.ChallengeRules{Active: true, Permadeath: true}
	settingsAct(t, state, engine.Action{Type: "settings", Name: "autoReset", Value: 1})
	settingsAct(t, state, engine.Action{Type: "wave"})
	for index := 0; index < 500 && state.Phase != "defeat"; index++ {
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
	}
	if state.Phase != "defeat" || state.Progression.Profile.Resets != 0 || state.Health != 0 {
		t.Fatal("automatic retry bypassed a permadeath challenge")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Advance(state, 60000); err != nil {
		t.Fatal(err)
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("a terminal permadeath world advanced or retried in the background")
	}
}

func TestNativeSettingsAutomaticPracticeRetryPreservesCustomRules(t *testing.T) {
	state := settingsStart(t)
	settingsAct(t, state, engine.Action{Type: "custom", Settings: json.RawMessage(`{"mapId":0,"hearts":3,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1}]}`)})
	settingsAct(t, state, engine.Action{Type: "settings", Name: "autoReset", Value: 1})
	settingsAct(t, state, engine.Action{Type: "wave"})
	for index := 0; index < 200 && state.Progression.Profile.Resets == 0; index++ {
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
	}
	if state.Progression.Profile.Resets != 1 || state.Progression.CustomData == nil || state.Health != 3 || state.MaxHealth != 3 || state.Phase != "prepare" || state.Wave != 1 {
		t.Fatal("automatic practice restart lost its original hearts or custom wave")
	}
}
