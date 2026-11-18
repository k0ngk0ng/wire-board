package engine

import (
	"errors"
	"math"
	"slices"
	"strings"
)

type EffectOpacity struct {
	Projectiles int `json:"projectiles"`
	Areas       int `json:"areas"`
	Beams       int `json:"beams"`
	Waves       int `json:"waves"`
	Auras       int `json:"auras"`
	Bombardment int `json:"bombardment"`
	Special     int `json:"special"`
}

func effectOpacityValue(settings *ProgressionSettings, key string) *int {
	switch key {
	case "projectiles":
		return &settings.EffectOpacity.Projectiles
	case "areas":
		return &settings.EffectOpacity.Areas
	case "beams":
		return &settings.EffectOpacity.Beams
	case "waves":
		return &settings.EffectOpacity.Waves
	case "auras":
		return &settings.EffectOpacity.Auras
	case "bombardment":
		return &settings.EffectOpacity.Bombardment
	case "special":
		return &settings.EffectOpacity.Special
	}
	return nil
}

func applyOriginalSetting(settings *ProgressionSettings, action Action) (bool, error) {
	booleans := map[string]*bool{"autoStop": &settings.AutoStop, "autoStopBoss": &settings.AutoStopBoss, "indicatorField": &settings.IndicatorField, "indicatorShape": &settings.IndicatorShape, "fastTarget": &settings.FastTarget, "durationCC": &settings.DurationCC, "particles": &settings.Particles, "disableTagColors": &settings.DisableTagColors}
	if target := booleans[action.Name]; target != nil {
		*target = action.Enabled
		if action.Name == "disableTagColors" {
			settings.ColorTags = !action.Enabled
		}
		return true, nil
	}
	integers := map[string]*int{"autoReset": &settings.AutoReset, "displayHealth": &settings.DisplayHealth, "mapEffectsMode": &settings.MapEffectsMode}
	if target := integers[action.Name]; target != nil {
		if !finiteNumber(action.Value) || math.Trunc(action.Value) != action.Value || action.Value < 0 || action.Value > 2 {
			return true, errors.New("请选择有效的自动或显示选项")
		}
		*target = int(action.Value)
		if action.Name == "displayHealth" {
			settings.ShowHealth = action.Value == 2
		}
		if action.Name == "mapEffectsMode" {
			settings.MapEffects = action.Value != 0
		}
		return true, nil
	}
	if strings.HasPrefix(action.Name, "effectOpacity.") {
		target := effectOpacityValue(settings, strings.TrimPrefix(action.Name, "effectOpacity."))
		if target == nil || !finiteNumber(action.Value) || action.Value < 0 || action.Value > 100 || math.Trunc(action.Value) != action.Value || int(action.Value)%5 != 0 {
			return true, errors.New("请选择有效的分类特效透明度")
		}
		*target = int(action.Value)
		return true, nil
	}
	if strings.HasPrefix(action.Name, "mute.") {
		key := strings.TrimPrefix(action.Name, "mute.")
		if len(key) != 1 || key[0] < '0' || key[0] > '4' {
			return true, errors.New("请选择有效的音效分组")
		}
		settings.Mute[int(key[0]-'0')] = action.Enabled
		return true, nil
	}
	return false, nil
}

func validateOriginalSettings(settings ProgressionSettings) error {
	for _, value := range []int{settings.AutoReset, settings.DisplayHealth, settings.MapEffectsMode} {
		if value < 0 || value > 2 {
			return errors.New("存档自动或显示选项无效")
		}
	}
	for _, key := range []string{"projectiles", "areas", "beams", "waves", "auras", "bombardment", "special"} {
		value := *effectOpacityValue(&settings, key)
		if value < 0 || value > 100 || value%5 != 0 {
			return errors.New("存档分类特效透明度无效")
		}
	}
	if settings.ColorTags == settings.DisableTagColors || settings.MapEffects != (settings.MapEffectsMode != 0) || settings.ShowHealth != (settings.DisplayHealth == 2) {
		return errors.New("存档显示选项与兼容字段不一致")
	}
	return nil
}

func automaticWaveAllowed(state *State) bool {
	if !state.AutoWave {
		return false
	}
	settings := state.Progression.Settings
	stop := settings.AutoStopBoss && (state.Wave == 100 || state.Wave == 50 && state.Route >= 12)
	if settings.AutoStop {
		route, _ := routeForState(state)
		wave, _ := waveFor(route, state.Wave)
		hidden := []string{"ditto", "kecleon", "absol", "lunala", "froslass", "mimikyu", "huntail", "octillery", "zorua", "zoroark", "mimeJr", "mrMime", "scraggy", "scrafty", "nickit", "thievul", "sableye", "marshadow", "mew", "duskull", "dusclops", "dusknoir", "spectrier"}
		for _, key := range wave.Preview {
			if slices.Contains(hidden, key) {
				stop = true
				break
			}
		}
	}
	if stop {
		state.AutoWave = false
		return false
	}
	return true
}

func automaticDefeatRetry(state *State) (bool, error) {
	mode := state.Progression.Settings.AutoReset
	practice := state.Phase == "practice_complete" && state.Progression.CustomRoute != nil
	if mode == 0 || state.Phase != "defeat" && !practice || state.Progression.Challenge.Permadeath {
		return false, nil
	}
	autoWave := state.AutoWave
	action := Action{Type: "retry", Mode: "restart"}
	if mode == 2 && state.Wave >= 25 {
		action.Mode = "retry"
	}
	if practice {
		if _, err := handleEditorAction(state, 0, Action{Type: "custom_retry", Epoch: state.Epoch}); err != nil {
			return false, err
		}
	} else {
		if err := applyCoreAction(state, 0, action); err != nil {
			return false, err
		}
	}
	state.AutoWave = autoWave
	if automaticWaveAllowed(state) {
		if err := startWave(state); err != nil {
			return false, err
		}
	}
	return true, nil
}
