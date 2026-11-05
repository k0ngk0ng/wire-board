// Package yuyuhakusho adapts platform seat identity to recovered game rules.
package yuyuhakusho

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Options struct {
	Arena      uint16  `json:"arena"`
	TeamBattle bool    `json:"teamBattle"`
	Characters []uint8 `json:"characters"`
}

func strictOptionsObject(raw json.RawMessage, allowed ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("设置须为对象")
	}
	fields := make(map[string]json.RawMessage, len(allowed))
	names := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		names[name] = true
	}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok || !names[name] || fields[name] != nil {
			return nil, errors.New("设置字段未知或重复")
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return nil, err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errors.New("设置字段不能为空")
		}
		fields[name] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("设置格式无效")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, errors.New("设置包含多余内容")
	}
	return fields, nil
}
func defaultOptions(capacity int) Options {
	roster := []uint8{3, 1, 9}
	if capacity == 2 {
		roster = []uint8{3, 9}
	} else if capacity == 4 {
		roster = []uint8{3, 1, 0, 10}
	}
	return Options{Characters: append([]uint8(nil), roster...)}
}
func validateOptions(capacity int, o Options) error {
	if capacity < 2 || capacity > 4 || len(o.Characters) != capacity || o.TeamBattle && capacity != 4 {
		return errors.New("人数与比赛设置不一致")
	}
	if o.Arena > 14 || o.Arena&1 != 0 {
		return errors.New("请选择已恢复的场地")
	}
	for _, character := range o.Characters {
		if character > 10 {
			return errors.New("请选择有效角色")
		}
	}
	return nil
}
func configure(capacity int, raw json.RawMessage) (json.RawMessage, error) {
	if capacity < 2 || capacity > 4 {
		return nil, errors.New("比赛支持 2 至 4 人")
	}
	o := defaultOptions(capacity)
	if len(bytes.TrimSpace(raw)) != 0 {
		if _, err := strictOptionsObject(raw, "arena", "teamBattle", "characters"); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, errors.New("比赛设置格式无效")
		}
	}
	if err := validateOptions(capacity, o); err != nil {
		return nil, err
	}
	return json.Marshal(o)
}
func prepare(ctx gameapi.PreparationContext, raw, action json.RawMessage) (json.RawMessage, error) {
	if ctx.Capacity < 2 || ctx.Capacity > 4 || len(ctx.Bots) == 0 || len(ctx.Bots) > ctx.Capacity || ctx.Actor < 0 || ctx.Actor >= len(ctx.Bots) {
		return nil, errors.New("当前席位不能准备")
	}
	normalized, err := configure(ctx.Capacity, raw)
	if err != nil {
		return nil, err
	}
	var o Options
	if err = json.Unmarshal(normalized, &o); err != nil {
		return nil, err
	}
	fields, err := strictOptionsObject(action, "type", "seat", "character", "arena", "teamBattle")
	if err != nil {
		return nil, err
	}
	var kind string
	if len(fields["type"]) == 0 || json.Unmarshal(fields["type"], &kind) != nil {
		return nil, errors.New("准备操作格式无效")
	}
	switch kind {
	case "character":
		if len(fields["character"]) == 0 || len(fields["arena"]) != 0 || len(fields["teamBattle"]) != 0 {
			return nil, errors.New("角色操作字段无效")
		}
		target := ctx.Actor
		if value := fields["seat"]; value != nil {
			if json.Unmarshal(value, &target) != nil {
				return nil, errors.New("席位无效")
			}
		}
		if target < 0 || target >= len(ctx.Bots) || target != ctx.Actor && !(ctx.Host && ctx.Bots[target]) {
			return nil, errors.New("只能选择自己或电脑席位的角色")
		}
		var character uint8
		if json.Unmarshal(fields["character"], &character) != nil || character > 10 {
			return nil, errors.New("请选择有效角色")
		}
		o.Characters[target] = character
	case "arena":
		if !ctx.Host || len(fields["arena"]) == 0 || len(fields["seat"]) != 0 || len(fields["character"]) != 0 || len(fields["teamBattle"]) != 0 {
			return nil, errors.New("仅房主可设置场地")
		}
		if json.Unmarshal(fields["arena"], &o.Arena) != nil {
			return nil, errors.New("请选择有效场地")
		}
	case "mode":
		if !ctx.Host || len(fields["teamBattle"]) == 0 || len(fields["seat"]) != 0 || len(fields["character"]) != 0 || len(fields["arena"]) != 0 {
			return nil, errors.New("仅房主可设置赛制")
		}
		if json.Unmarshal(fields["teamBattle"], &o.TeamBattle) != nil {
			return nil, errors.New("比赛模式无效")
		}
	default:
		return nil, errors.New("未知准备操作")
	}
	if err := validateOptions(ctx.Capacity, o); err != nil {
		return nil, err
	}
	return json.Marshal(o)
}
func sessionOptions(o Options, automated []bool, seed uint32) (engine.ChuSessionOptions, error) {
	capacity := len(o.Characters)
	if err := validateOptions(capacity, o); err != nil {
		return engine.ChuSessionOptions{}, err
	}
	if len(automated) != capacity {
		return engine.ChuSessionOptions{}, errors.New("席位控制信息不完整")
	}
	slots := []uint8{0, 3}
	if capacity == 3 {
		slots = []uint8{0, 1, 3}
	} else if capacity == 4 {
		slots = []uint8{0, 1, 2, 3}
	}
	out := engine.ChuSessionOptions{Arena: o.Arena, Seed: seed, Difficulty: 6, TeamBattle: o.TeamBattle}
	for seat, slot := range slots {
		out.Players = append(out.Players, engine.ChuSessionPlayer{Slot: slot, CharacterID: o.Characters[seat], Color: uint8(seat), Computer: automated[seat], HumanChannel: uint8(seat)})
	}
	return out, nil
}
