package game

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed rail_maps/*.json
var railMapsJSON embed.FS

type RailMapSpec struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	MinPlayers       int      `json:"minPlayers"`
	MaxPlayers       int      `json:"maxPlayers"`
	Width            int      `json:"width"`
	Height           int      `json:"height"`
	Trains           int      `json:"trains"`
	DoubleMin        int      `json:"doubleMin"`
	SetupTickets     int      `json:"setupTickets"`
	LongTickets      bool     `json:"longTickets"`
	InitialReturn    bool     `json:"initialReturn"`
	AdditionalReturn bool     `json:"additionalReturn"`
	WildSingle       bool     `json:"wildSingle"`
	WildRule         string   `json:"wildRule"`
	Bonus            string   `json:"bonus"`
	Stations         int      `json:"stations"`
	Art              string   `json:"art"`
	Rules            []string `json:"rules"`
}

var railMapSpecs = []RailMapSpec{
	{ID: "usa", Name: "美国", MinPlayers: 2, MaxPlayers: 5, Width: 1744, Height: 1125, Trains: 45, DoubleMin: 4, SetupTickets: 3, InitialReturn: true, AdditionalReturn: true, WildRule: "any", Bonus: "longest", Art: "rail", Rules: []string{"美国基础版；最长连续路线奖励 10 分。", "初始抽 3 张任务至少留 2 张；2–3 人只能使用双线中的一条。"}},
	{ID: "europe", Name: "欧洲", MinPlayers: 2, MaxPlayers: 5, Width: 1744, Height: 1125, Trains: 45, DoubleMin: 4, SetupTickets: 4, LongTickets: true, AdditionalReturn: true, WildRule: "any", Bonus: "longest", Stations: 3, Art: "rail/maps/v1/europe", Rules: []string{"隧道翻 3 张牌检查追加费用；渡轮需要图示数量的万能牌。", "每人 3 座车站，可借用他人路线完成任务；未建车站每座 4 分。", "初始发 1 张长途与 3 张普通任务，至少留 2 张；未留任务移出游戏。", "最长连续路线奖励 10 分；8 节路线计 21 分。"}},
	{ID: "india", Name: "印度", MinPlayers: 2, MaxPlayers: 4, Width: 1125, Height: 1744, Trains: 45, DoubleMin: 4, SetupTickets: 4, InitialReturn: true, AdditionalReturn: true, WildRule: "any", Bonus: "longest", Art: "rail/maps/v1/india", Rules: []string{"最多 4 人；4 人时才可使用双线的两条路线。", "渡轮需要图示数量的万能牌。", "以两条不共用铁路线的路径完成同一任务可获环游奖励：5、10、20、30、40 分。", "初始抽 4 留至少 2；最长连续路线奖励 10 分。"}},
	{ID: "switzerland", Name: "瑞士", MinPlayers: 2, MaxPlayers: 3, Width: 1744, Height: 1125, Trains: 40, DoubleMin: 3, SetupTickets: 5, WildSingle: true, WildRule: "tunnel", Bonus: "longest", Art: "rail/maps/v1/switzerland", Rules: []string{"2–3 人，每人 40 节车厢；3 人可使用双线。", "公开万能牌只计 1 次摸牌，但只能用于隧道。", "国家任务取已连通选项中的最高分；全部未完成则扣最低分。各边境入口互不连通。", "初始抽 5 留至少 2；所有未留任务移出游戏；最长路线奖励 10 分。"}},
	{ID: "nordiccountries", Name: "北欧", MinPlayers: 2, MaxPlayers: 3, Width: 1125, Height: 1744, Trains: 40, DoubleMin: 3, SetupTickets: 5, WildSingle: true, WildRule: "special", Bonus: "tickets", Art: "rail/maps/v1/nordiccountries", Rules: []string{"2–3 人，每人 40 节车厢；3 人可使用双线。", "公开万能牌只计 1 次摸牌，仅用于隧道和渡轮。", "渡轮可用任意 3 张牌替代 1 张必需万能牌；9 节特殊线路可用任意 4 张替代 1 张同色牌。", "初始抽 5 留至少 2；未留任务移出游戏；任务完成最多奖励 10 分，无最长路线奖。"}},
	{ID: "legendaryasia", Name: "传奇亚洲", MinPlayers: 2, MaxPlayers: 5, Width: 1744, Height: 1125, Trains: 45, DoubleMin: 4, SetupTickets: 4, LongTickets: true, AdditionalReturn: true, WildRule: "any", Bonus: "network", Art: "rail/maps/v1/legendaryasia", Rules: []string{"山地每个 × 需额外消耗 1 节车厢，并获 2 分；渡轮需要万能牌。", "初始发 1 张长途与 3 张普通任务，至少留 2 张；未留任务移出游戏。", "最大连通网络包含城市最多者获 10 分，无最长路线奖。", "同分依次比较完成任务数量、已铺山地路线数量。"}},
}

var extraRailData = func() map[string]*RailData {
	out := map[string]*RailData{}
	for _, info := range railMapSpecs[1:] {
		raw, err := railMapsJSON.ReadFile("rail_maps/" + info.ID + ".json")
		if err != nil {
			panic(err)
		}
		var data RailData
		if err = json.Unmarshal(raw, &data); err != nil {
			panic(err)
		}
		out[info.ID] = &data
	}
	return out
}()

func RailMapInfo(id string) (RailMapSpec, bool) {
	if id == "" {
		id = "usa"
	}
	for _, info := range railMapSpecs {
		if info.ID == id {
			return info, true
		}
	}
	return RailMapSpec{}, false
}
func RailMapList() []RailMapSpec { return clone(railMapSpecs) }
func (g *Rail) info() RailMapSpec {
	info, ok := RailMapInfo(g.Map)
	if !ok {
		panic("invalid persisted rail map")
	}
	return info
}
func (g *Rail) data() *RailData {
	if g.Map == "" || g.Map == "usa" {
		return &baseRailData
	}
	if d := extraRailData[g.Map]; d != nil {
		return d
	}
	panic("invalid persisted rail map")
}
func RailCatalog(id string) (map[string]any, bool) {
	info, ok := RailMapInfo(id)
	if !ok {
		return nil, false
	}
	g := Rail{Map: info.ID}
	d := g.data()
	return map[string]any{"map": info, "cities": d.Cities, "routes": d.Routes, "tickets": d.Tickets}, true
}
func NewRailMap(id string, n int) (*State, error) {
	info, ok := RailMapInfo(id)
	if !ok {
		return nil, fmt.Errorf("未知铁路地图")
	}
	if n < info.MinPlayers || n > info.MaxPlayers {
		return nil, fmt.Errorf("%s地图需要 %d–%d 位玩家", info.Name, info.MinPlayers, info.MaxPlayers)
	}
	s := &State{Kind: "rail", Phase: "turn", Round: 1, Log: []string{}}
	s.initRailMap(n, info.ID)
	return s, nil
}
func railRoutePoints(length int) int {
	if length < 1 || length > 9 {
		return 0
	}
	return [...]int{0, 1, 2, 4, 7, 10, 15, 18, 21, 27}[length]
}
