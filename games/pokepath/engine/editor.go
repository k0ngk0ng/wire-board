package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

const maxEditorGroups = 256
const maxEditorSpawns = 4096
const maxEditorCodeBytes = 32768

type EditorSlot struct {
	Enemy  string `json:"enemy"`
	Amount int    `json:"amount"`
}

type EditorData struct {
	MapID           int          `json:"mapId"`
	Hearts          int          `json:"hearts"`
	Health          int          `json:"health"`
	Armor           int          `json:"armor"`
	Wave            []EditorSlot `json:"wave"`
	ReturnRoute     int          `json:"returnRoute,omitempty"`
	ReturnWave      int          `json:"returnWave,omitempty"`
	ReturnHealth    int          `json:"returnHealth,omitempty"`
	ReturnMaxHealth int          `json:"returnMaxHealth,omitempty"`
}

type editorInput struct {
	MapID  int          `json:"mapId"`
	Hearts int          `json:"hearts"`
	Health int          `json:"health"`
	Armor  int          `json:"armor"`
	Wave   []EditorSlot `json:"wave"`
}

func (slot EditorSlot) MarshalJSON() ([]byte, error) {
	var enemy *string
	if slot.Enemy != "" {
		value := slot.Enemy
		enemy = &value
	}
	return json.Marshal(struct {
		Enemy  *string `json:"enemy"`
		Amount int     `json:"amount"`
	}{enemy, slot.Amount})
}

func (slot *EditorSlot) UnmarshalJSON(raw []byte) error {
	var input struct {
		Enemy  *string `json:"enemy"`
		Amount int     `json:"amount"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return errors.New("自定义波次的敌人和数量无效")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("自定义波次包含多余数据")
	}
	slot.Enemy = ""
	if input.Enemy != nil {
		slot.Enemy = *input.Enemy
	}
	slot.Amount = input.Amount
	return nil
}

func validateEditorData(data EditorData) error {
	catalog := CatalogData()
	if catalog == nil {
		return errors.New("规则目录尚未加载")
	}
	if _, ok := catalog.Routes[strconv.Itoa(data.MapID)]; !ok {
		return errors.New("请选择有效的自定义地图")
	}
	if data.Hearts < 1 || data.Hearts > 14 || data.Health < 0 || data.Health > 200 || data.Health%5 != 0 || data.Armor < 0 || data.Armor > 200 || data.Armor%5 != 0 {
		return errors.New("生命应为 1～14，强化应为 0～200 且按 5 递增")
	}
	if len(data.Wave) == 0 || len(data.Wave) > maxEditorGroups {
		return errors.New("自定义波次需要 1～256 组出兵")
	}
	amount, enemies := 0, 0
	for _, slot := range data.Wave {
		if slot.Amount <= 0 || slot.Amount > maxEditorSpawns-amount {
			return errors.New("自定义波次最多展开 4096 个出兵位置")
		}
		amount += slot.Amount
		if slot.Enemy != "" {
			if _, ok := catalog.Enemies[slot.Enemy]; !ok {
				return errors.New("自定义波次包含未知敌人")
			}
			enemies += slot.Amount
		}
	}
	if enemies == 0 {
		return errors.New("自定义波次至少需要一名敌人")
	}
	return nil
}

func parseEditorInput(raw []byte) (EditorData, error) {
	var input editorInput
	input.Hearts = 14
	if len(raw) == 0 || len(raw) > maxEditorCodeBytes {
		return EditorData{}, errors.New("自定义波次数据为空或过大")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return EditorData{}, errors.New("自定义波次格式无效")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return EditorData{}, errors.New("自定义波次包含多余数据")
	}
	data := EditorData{MapID: input.MapID, Hearts: input.Hearts, Health: input.Health, Armor: input.Armor, Wave: append([]EditorSlot{}, input.Wave...)}
	return data, validateEditorData(data)
}

func DecodeEditorCode(code string) (EditorData, error) {
	code = strings.TrimSpace(code)
	if len(code) == 0 || len(code) > maxEditorCodeBytes {
		return EditorData{}, errors.New("自定义分享码为空或过长")
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(code)
	if err != nil {
		return EditorData{}, errors.New("自定义分享码不是有效编码")
	}
	return parseEditorInput(raw)
}

func EncodeEditorCode(data EditorData) (string, error) {
	if err := validateEditorData(data); err != nil {
		return "", err
	}
	input := editorInput{MapID: data.MapID, Hearts: data.Hearts, Health: data.Health, Armor: data.Armor, Wave: data.Wave}
	raw, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	code := base64.StdEncoding.EncodeToString(raw)
	if len(code) > maxEditorCodeBytes {
		return "", errors.New("自定义分享码过长，请减少出兵分组")
	}
	return code, nil
}

func editorRouteAllowed(state *State, route RouteSpec) bool {
	if !route.IsSecret {
		return state.Stars >= route.UnlockStars
	}
	required := map[int]string{20: "manaphy", 30: "mew", 31: "victini", 32: "marshadow", 33: "farfetchdGalar"}
	key, ok := required[route.ID]
	return ok && state.Progression.Secrets[key]
}

func customWave(data EditorData) WaveSpec {
	preview := []string{}
	sequence := []*string{}
	seen := map[string]bool{}
	for _, slot := range data.Wave {
		if slot.Enemy != "" && !seen[slot.Enemy] {
			preview = append(preview, slot.Enemy)
			seen[slot.Enemy] = true
		}
		for index := 0; index < slot.Amount; index++ {
			if slot.Enemy == "" {
				sequence = append(sequence, nil)
			} else {
				enemy := slot.Enemy
				sequence = append(sequence, &enemy)
			}
		}
	}
	return WaveSpec{Preview: preview, Sequence: sequence}
}

func clearEditorBattle(state *State, keepTowers bool) {
	state.WaveActive = false
	state.Enemies = []Enemy{}
	if !keepTowers {
		state.Towers = []Tower{}
	}
	state.Projectiles = []Projectile{}
	state.WaveKills = 0
	state.WaveGold = 0
	state.WaveHeartsLost = 0
	state.WaveHeartsHealed = 0
	state.AutoWave = false
	state.Progression.WaveRepeat = false
	state.Progression.WaveMirror = false
	state.Progression.ActiveMirror = false
	worldTowers := state.World.Towers
	WorldOnRouteChange(state)
	if keepTowers {
		state.World.Towers = worldTowers
		state.AttackModes.Effects = nil
	} else {
		OnAttackModeRouteChanged(state)
		for index := range state.Team {
			state.Team[index].TileID = -1
		}
	}
}

func handleEditorAction(state *State, actor int, action Action) (bool, error) {
	if action.Type != "custom" && action.Type != "custom_exit" && action.Type != "custom_retry" {
		return false, nil
	}
	if actor < 0 || actor >= state.Players || action.Epoch != state.Epoch {
		return true, errors.New("当前席位或操作代次无效")
	}
	if state.Paused || state.WaveActive || state.Progression.Challenge.Active || state.Progression.Draft.Active {
		return true, errors.New("请先结束当前战斗或挑战，再使用自定义波次")
	}
	if action.Type == "custom_exit" {
		if state.Progression.CustomRoute == nil || state.Progression.CustomData == nil {
			return true, errors.New("当前不在自定义实践中")
		}
		data := *state.Progression.CustomData
		clearEditorBattle(state, false)
		state.Route = data.ReturnRoute
		state.Wave = data.ReturnWave
		state.Health = data.ReturnHealth
		state.MaxHealth = data.ReturnMaxHealth
		state.Progression.CustomRoute = nil
		state.Progression.CustomData = nil
		state.Phase = "prepare"
		if route, ok := routeFor(state.Route); ok {
			state.Weather = route.WeatherID
		}
		addEvent(state, Event{Type: "practice_exit"})
		return true, nil
	}
	if action.Type == "custom_retry" {
		if state.Progression.CustomRoute == nil || state.Progression.CustomData == nil {
			return true, errors.New("当前没有可重试的自定义波次")
		}
		clearEditorBattle(state, true)
		state.Wave = 1
		state.Health = state.Progression.CustomData.Hearts
		state.MaxHealth = state.Progression.CustomData.Hearts
		state.Phase = "prepare"
		state.Progression.Profile.Resets++
		addEvent(state, Event{Type: "practice_retry"})
		return true, nil
	}
	var data EditorData
	var err error
	if len(action.Settings) > 0 {
		data, err = parseEditorInput(action.Settings)
	} else {
		data, err = DecodeEditorCode(action.Name)
	}
	if err != nil {
		return true, err
	}
	route, ok := routeFor(data.MapID)
	if !ok || !editorRouteAllowed(state, route) || len(route.Paths) == 0 {
		return true, errors.New("该地图尚未解锁或无法用于自定义波次")
	}
	data.ReturnRoute, data.ReturnWave, data.ReturnHealth, data.ReturnMaxHealth = state.Route, state.Wave, state.Health, state.MaxHealth
	if previous := state.Progression.CustomData; previous != nil {
		data.ReturnRoute, data.ReturnWave, data.ReturnHealth, data.ReturnMaxHealth = previous.ReturnRoute, previous.ReturnWave, previous.ReturnHealth, previous.ReturnMaxHealth
	}
	route.Waves = map[string]WaveSpec{"1": customWave(data)}
	route.ChallengeRewards = nil
	clearEditorBattle(state, false)
	state.Progression.PreviousRoute = data.ReturnRoute
	state.Progression.CustomData = &data
	state.Progression.CustomRoute = &route
	state.Route = data.MapID
	state.Wave = 1
	state.Health = data.Hearts
	state.MaxHealth = data.Hearts
	state.Weather = route.WeatherID
	state.Phase = "prepare"
	addEvent(state, Event{Type: "practice_start"})
	return true, nil
}

func finishPractice(state *State, victory bool) {
	if victory {
		recordTrainerWave(state, true, state.WaveGold)
	}
	state.WaveActive = false
	state.Phase = "practice_complete"
	state.Enemies = []Enemy{}
	state.Projectiles = []Projectile{}
	key := "defeat"
	if victory {
		key = "victory"
	}
	addEvent(state, Event{Type: "practice_complete", Key: key})
}
