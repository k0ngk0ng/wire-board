package cs16

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

var terroristCharacters = []string{"terror", "leet", "arctic", "guerilla"}
var counterCharacters = []string{"urban", "gsg9", "sas", "gign"}

func object(raw json.RawMessage, allowed ...string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 8192 {
		return nil, errors.New("设置或操作大小无效")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("设置或操作须为对象")
	}
	fields := map[string]json.RawMessage{}
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

func characterForTeam(team string) string {
	if team == "ct" {
		return "urban"
	}
	return "terror"
}

func validCharacter(team, character string) bool {
	if team == "t" {
		return slices.Contains(terroristCharacters, character)
	}
	return team == "ct" && slices.Contains(counterCharacters, character)
}

func configure(capacity int, raw json.RawMessage) (json.RawMessage, error) {
	if capacity < 2 || capacity > 10 {
		return nil, errors.New("反恐精英支持 2 至 10 人")
	}
	options := engine.Options{Map: "de_dust2", RoundSeconds: 120, MaxRounds: 10, Teams: make([]string, capacity), Characters: make([]string, capacity)}
	for seat := range capacity {
		options.Teams[seat] = "t"
		if seat%2 == 1 {
			options.Teams[seat] = "ct"
		}
		options.Characters[seat] = characterForTeam(options.Teams[seat])
	}
	fields := map[string]json.RawMessage{}
	if len(raw) > 0 {
		var err error
		fields, err = object(raw, "map", "roundSeconds", "maxRounds", "teams", "characters", "friendlyFire")
		if err != nil {
			return nil, err
		}
		if json.Unmarshal(raw, &options) != nil {
			return nil, errors.New("比赛设置格式无效")
		}
		if fields["teams"] != nil && len(options.Teams) != capacity || fields["characters"] != nil && len(options.Characters) != capacity {
			return nil, errors.New("队伍和造型数量须与席位一致")
		}
	}
	if !slices.Contains([]string{"de_dust2", "de_inferno", "de_aztec", "cs_assault", "fy_iceworld", "cs_bloodstrike"}, options.Map) || engine.MapByID(options.Map).ID == "" {
		return nil, errors.New("请选择有效地图")
	}
	if !slices.Contains([]int{60, 90, 120, 180, 300}, options.RoundSeconds) || !slices.Contains([]int{2, 4, 6, 10, 16, 24, 30}, options.MaxRounds) {
		return nil, errors.New("请选择有效回合时长与局数")
	}
	if len(options.Teams) != capacity || len(options.Characters) != capacity {
		return nil, errors.New("比赛席位设置无效")
	}
	counts := map[string]int{}
	for seat, team := range options.Teams {
		if team != "t" && team != "ct" {
			return nil, errors.New("请选择警察或恐怖分子队伍")
		}
		counts[team]++
		if fields["characters"] == nil {
			options.Characters[seat] = characterForTeam(team)
		}
		if !validCharacter(team, options.Characters[seat]) {
			return nil, errors.New("角色造型须属于所在队伍")
		}
	}
	if counts["t"] == 0 || counts["ct"] == 0 || counts["t"] > 5 || counts["ct"] > 5 {
		return nil, errors.New("双方各需 1 至 5 人")
	}
	return json.Marshal(options)
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
	fields, err := object(action, "type", "seat", "map", "roundSeconds", "maxRounds", "team", "character", "friendlyFire")
	if err != nil {
		return nil, err
	}
	var input struct {
		Type         string `json:"type"`
		Seat         *int   `json:"seat"`
		Map          string `json:"map"`
		RoundSeconds int    `json:"roundSeconds"`
		MaxRounds    int    `json:"maxRounds"`
		Team         string `json:"team"`
		Character    string `json:"character"`
		FriendlyFire bool   `json:"friendlyFire"`
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
	case "settings":
		if !ctx.Host || fields["roundSeconds"] == nil && fields["maxRounds"] == nil && fields["friendlyFire"] == nil {
			return nil, errors.New("仅房主可修改赛制")
		}
		allowed = append(allowed, "roundSeconds", "maxRounds", "friendlyFire")
		if fields["roundSeconds"] != nil {
			options.RoundSeconds = input.RoundSeconds
		}
		if fields["maxRounds"] != nil {
			options.MaxRounds = input.MaxRounds
		}
		if fields["friendlyFire"] != nil {
			options.FriendlyFire = input.FriendlyFire
		}
	case "team", "character":
		seat := ctx.Actor
		if input.Seat != nil {
			seat = *input.Seat
		}
		if seat < 0 || seat >= len(ctx.Bots) || seat != ctx.Actor && !(ctx.Host && ctx.Bots[seat]) {
			return nil, errors.New("只能修改本人或房主管理的电脑")
		}
		allowed = append(allowed, "seat", input.Type)
		if fields[input.Type] == nil {
			return nil, errors.New("请选择队伍或角色")
		}
		if input.Type == "team" {
			options.Teams[seat] = input.Team
			options.Characters[seat] = characterForTeam(input.Team)
		} else {
			options.Characters[seat] = input.Character
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
