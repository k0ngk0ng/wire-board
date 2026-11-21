package hotlinemiami

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func object(raw json.RawMessage, allowed ...string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 8192 {
		return nil, errors.New("设置或操作大小无效")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("设置或操作须为对象")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || !slices.Contains(allowed, key) || fields[key] != nil {
			return nil, errors.New("设置或操作字段未知或重复")
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errors.New("设置或操作字段不能为空")
		}
		fields[key] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("设置或操作格式无效")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, errors.New("设置或操作包含多余内容")
	}
	return fields, nil
}

func configure(capacity int, raw json.RawMessage) (json.RawMessage, error) {
	if capacity < 2 || capacity > 4 {
		return nil, errors.New("混战支持 2 至 4 人")
	}
	o := engine.Options{Map: "no-talk", RoundSeconds: 180, Characters: make([]string, capacity), Weapons: make([]string, capacity)}
	characters := engine.Characters()
	for seat := range capacity {
		o.Characters[seat] = characters[seat%len(characters)]
		o.Weapons[seat] = "pistol"
	}
	if len(raw) > 0 {
		fields, err := object(raw, "map", "roundSeconds", "characters", "weapons")
		if err != nil {
			return nil, err
		}
		if json.Unmarshal(raw, &o) != nil {
			return nil, errors.New("比赛设置格式无效")
		}
		if fields["characters"] != nil && len(o.Characters) != capacity || fields["weapons"] != nil && len(o.Weapons) != capacity {
			return nil, errors.New("造型与武器数量须与席位一致")
		}
	}
	if engine.MapByID(o.Map).ID == "" || !slices.Contains([]int{180, 240, 300}, o.RoundSeconds) {
		return nil, errors.New("请选择有效地图与时长")
	}
	if len(o.Characters) != capacity || len(o.Weapons) != capacity {
		return nil, errors.New("比赛席位设置无效")
	}
	for seat := range capacity {
		if !slices.Contains(characters, o.Characters[seat]) {
			return nil, errors.New("请选择有效造型")
		}
		if _, ok := engine.WeaponByID(o.Weapons[seat]); !ok {
			return nil, errors.New("请选择有效起始武器")
		}
	}
	return json.Marshal(o)
}

func prepare(ctx gameapi.PreparationContext, raw, action json.RawMessage) (json.RawMessage, error) {
	if ctx.Actor < 0 || ctx.Actor >= len(ctx.Bots) || len(ctx.Bots) > ctx.Capacity {
		return nil, errors.New("当前席位不能准备")
	}
	configured, err := configure(ctx.Capacity, raw)
	if err != nil {
		return nil, err
	}
	var options engine.Options
	if json.Unmarshal(configured, &options) != nil {
		return nil, errors.New("房间设置无效")
	}
	fields, err := object(action, "type", "seat", "map", "roundSeconds", "character", "weapon")
	if err != nil {
		return nil, err
	}
	var input struct {
		Type         string `json:"type"`
		Seat         *int   `json:"seat"`
		Map          string `json:"map"`
		RoundSeconds int    `json:"roundSeconds"`
		Character    string `json:"character"`
		Weapon       string `json:"weapon"`
	}
	if json.Unmarshal(action, &input) != nil {
		return nil, errors.New("准备操作格式无效")
	}
	allowed := []string{"type"}
	switch input.Type {
	case "map":
		if !ctx.Host || fields["map"] == nil {
			return nil, errors.New("仅房主可选择地图")
		}
		allowed = append(allowed, "map")
		options.Map = input.Map
	case "time":
		if !ctx.Host || fields["roundSeconds"] == nil {
			return nil, errors.New("仅房主可修改时长")
		}
		allowed = append(allowed, "roundSeconds")
		options.RoundSeconds = input.RoundSeconds
	case "character", "weapon":
		seat := ctx.Actor
		if input.Seat != nil {
			seat = *input.Seat
		}
		if seat < 0 || seat >= len(ctx.Bots) || seat != ctx.Actor && !(ctx.Host && ctx.Bots[seat]) {
			return nil, errors.New("只能修改本人或房主管理的电脑")
		}
		allowed = append(allowed, "seat", input.Type)
		if fields[input.Type] == nil {
			return nil, errors.New("请选择造型或武器")
		}
		if input.Type == "character" {
			options.Characters[seat] = input.Character
		} else {
			options.Weapons[seat] = input.Weapon
		}
	default:
		return nil, errors.New("未知准备操作")
	}
	for key := range fields {
		if !slices.Contains(allowed, key) {
			return nil, errors.New("准备操作包含无关字段")
		}
	}
	updated, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	return configure(ctx.Capacity, updated)
}
