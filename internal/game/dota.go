package game

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

//go:embed dota_data.json
var dotaData []byte

type DotaCardSpec struct {
	Name string `json:"name"`
	Art  string `json:"art"`
	Text string `json:"text"`
}
type DotaHeroSpec struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	HP      int            `json:"hp"`
	Passive string         `json:"passive"`
	Skills  []DotaCardSpec `json:"skills"`
}
type DotaItemSpec struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
	Role  string `json:"role"`
	Text  string `json:"text"`
	Solo  bool   `json:"solo"`
}
type DotaCatalog struct {
	Heroes []DotaHeroSpec `json:"heroes"`
	Items  []DotaItemSpec `json:"items"`
	Common []DotaCardSpec `json:"common"`
}

var dotaCatalog = func() DotaCatalog {
	var c DotaCatalog
	if err := json.Unmarshal(dotaData, &c); err != nil {
		panic(err)
	}
	return c
}()

func dotaHero(id string) DotaHeroSpec {
	for _, v := range dotaCatalog.Heroes {
		if v.ID == id {
			return v
		}
	}
	return DotaHeroSpec{}
}
func dotaItem(id string) DotaItemSpec {
	for _, v := range dotaCatalog.Items {
		if v.ID == id {
			return v
		}
	}
	return DotaItemSpec{}
}

type DotaOrder struct {
	Card   int    `json:"card"`
	Lane   int    `json:"lane"`
	Target int    `json:"target"`
	Buy    string `json:"buy"`
}
type DotaPlayer struct {
	Hero    string   `json:"hero"`
	Team    int      `json:"team"`
	Lane    int      `json:"lane"`
	HP      int      `json:"hp"`
	Gold    int      `json:"gold"`
	Charge  int      `json:"charge"`
	Spent   int      `json:"spent"`
	Gear    []string `json:"gear"`
	Wounded bool     `json:"wounded"`
}

func (p DotaPlayer) has(id string) bool { return slices.Contains(p.Gear, id) }
func (p DotaPlayer) maximum() int {
	return dotaHero(p.Hero).HP + 2*dotaBool(p.has("bracer")) + 2*dotaBool(p.has("vanguard")) + dotaBool(p.has("wraith_band"))
}
func dotaBool(v bool) int {
	if v {
		return 1
	}
	return 0
}
func dotaPrice(p DotaPlayer, id string) int {
	return dotaItem(id).Price - 3*dotaBool(p.has("boots") && (id == "phase_boots" || id == "arcane_boots"))
}

type DotaEvent struct {
	Round    int          `json:"round"`
	Orders   []DotaOrder  `json:"orders"`
	Before   []DotaPlayer `json:"before"`
	After    []DotaPlayer `json:"after"`
	Messages []string     `json:"messages"`
	Core     [2]int       `json:"core"`
	Towers   [2][]int     `json:"towers"`
}
type Dota struct {
	Rules     string       `json:"rules"`
	N         int          `json:"n"`
	Sequence  int          `json:"sequence"`
	Captain   int          `json:"captain"`
	Confirmed []bool       `json:"confirmed"`
	Players   []DotaPlayer `json:"players"`
	Draft     []int        `json:"draft"`
	DraftStep int          `json:"draftStep"`
	First     int          `json:"first"`
	Core      [2]int       `json:"core"`
	Towers    [2][]int     `json:"towers"`
	Track     []int        `json:"track"`
	Kills     [2]int       `json:"kills"`
	Pending   []*DotaOrder `json:"pending"`
	History   []DotaEvent  `json:"history"`
}

func (s *State) initDota(count int) {
	n := count / 2
	g := &Dota{Rules: "v0.9-full-ancient", N: n, Sequence: 1, Confirmed: make([]bool, count), Players: make([]DotaPlayer, count), Pending: make([]*DotaOrder, count), Track: make([]int, n), History: []DotaEvent{}, Core: [2]int{n + 1, n + 1}}
	teams := make([]int, count)
	for i := range teams {
		teams[i] = i / n
	}
	shuffle(teams)
	first := []int{0, 1}
	shuffle(first)
	g.First = first[0]
	for i := range g.Players {
		g.Players[i] = DotaPlayer{Team: teams[i], Lane: -1, Gear: []string{}}
	}
	for t := 0; t < 2; t++ {
		g.Towers[t] = make([]int, n)
		for l := range g.Towers[t] {
			g.Towers[t][l] = 2
			if n == 1 {
				g.Towers[t][l] = 1
			}
		}
	}
	s.Dota = g
	s.Phase = "dota_teams"
	s.Turn = -1
	s.Log = append(s.Log, "兵线争锋：开始后分队，再轮流选英雄。所有模式均须摧毁敌方遗迹。")
}
func (g *Dota) seats(team int) []int {
	out := []int{}
	for i, p := range g.Players {
		if p.Team == team {
			out = append(out, i)
		}
	}
	return out
}
func (s *State) DotaActors() []int {
	if s.Dota == nil || s.Finished {
		return nil
	}
	g := s.Dota
	out := []int{}
	switch s.Phase {
	case "dota_teams":
		for i, done := range g.Confirmed {
			if !done {
				out = append(out, i)
			}
		}
	case "dota_draft":
		out = append(out, s.Turn)
	case "dota_plan":
		for offset := 0; offset < g.N; offset++ {
			for _, team := range []int{(g.First + s.Round - 1) % 2, 1 - (g.First+s.Round-1)%2} {
				p := g.seats(team)[(offset+s.Round-1)%g.N]
				if g.Pending[p] == nil {
					out = append(out, p)
				}
			}
		}
	}
	return out
}
func (s *State) applyDota(player int, a Action) error {
	g := s.Dota
	if player < 0 || player >= len(g.Players) {
		return errors.New("无效席位")
	}
	if a.Prompt != g.Sequence {
		return errors.New("本阶段已经更新，请按最新局面操作")
	}
	switch a.Type {
	case "dota_teams":
		if s.Phase != "dota_teams" || player != g.Captain {
			return errors.New("分队阶段由房主调整队伍")
		}
		if len(a.Targets) != len(g.Players) {
			return errors.New("队伍人数不正确")
		}
		count := 0
		for _, t := range a.Targets {
			if t != 0 && t != 1 {
				return errors.New("无效队伍")
			}
			count += t
		}
		if count != g.N {
			return errors.New("近卫与天灾的人数必须相等")
		}
		for i, t := range a.Targets {
			g.Players[i].Team = t
			g.Confirmed[i] = false
		}
		g.Sequence++
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 调整了分队，请重新确认", player+1))
	case "dota_confirm":
		if s.Phase != "dota_teams" || g.Confirmed[player] {
			return errors.New("当前无需确认队伍")
		}
		g.Confirmed[player] = true
		if !slices.Contains(g.Confirmed, false) {
			a, b := g.seats(g.First), g.seats(1-g.First)
			shuffle(a)
			shuffle(b)
			g.Draft = []int{}
			for i := 0; i < g.N; i++ {
				g.Draft = append(g.Draft, a[i], b[i])
			}
			s.Phase = "dota_draft"
			s.Turn = g.Draft[0]
			g.Sequence++
			s.Log = append(s.Log, "队伍已确认，双方交替选择不重复的英雄")
		}
	case "dota_pick":
		if s.Phase != "dota_draft" || player != s.Turn {
			return errors.New("尚未轮到你选择英雄")
		}
		spec := dotaHero(a.Choice)
		if spec.ID == "" {
			return errors.New("未知英雄")
		}
		for _, p := range g.Players {
			if p.Hero == a.Choice {
				return errors.New("这位英雄已被选走")
			}
		}
		p := &g.Players[player]
		p.Hero = spec.ID
		p.HP = spec.HP
		p.Gold = 2
		p.Lane = slices.Index(g.seats(p.Team), player)
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 选择了%s", player+1, spec.Name))
		g.DraftStep++
		g.Sequence++
		if g.DraftStep == len(g.Draft) {
			s.Phase = "dota_plan"
			s.Turn = -1
			s.Log = append(s.Log, "第一轮开始：同时暗选，全部锁定后揭牌结算")
		} else {
			s.Turn = g.Draft[g.DraftStep]
		}
	case "dota_plan":
		if s.Phase != "dota_plan" || g.Pending[player] != nil || a.Dota == nil {
			return errors.New("当前无法提交行动")
		}
		if !slices.Contains(g.legal(player, true), *a.Dota) {
			return errors.New("行动、目标或装备不合法")
		}
		order := *a.Dota
		g.Pending[player] = &order
		ready := true
		for _, p := range g.Pending {
			ready = ready && p != nil
		}
		if ready {
			orders := make([]DotaOrder, len(g.Pending))
			for i, p := range g.Pending {
				orders[i] = *p
			}
			s.dotaResolve(orders, true)
			g.Pending = make([]*DotaOrder, len(g.Players))
			g.Sequence++
		}
	case "dota_unlock":
		if s.Phase != "dota_plan" || g.Pending[player] == nil {
			return errors.New("没有可撤回的行动")
		}
		g.Pending[player] = nil
	default:
		return errors.New("未知兵线争锋行动")
	}
	if len(s.Log) > 100 {
		s.Log = s.Log[len(s.Log)-100:]
	}
	return nil
}

func (g *Dota) legal(player int, movement bool) []DotaOrder {
	p := g.Players[player]
	out := []DotaOrder{}
	add := func(card, lane, target int, buy string) {
		o := DotaOrder{card, lane, target, buy}
		if !slices.Contains(out, o) {
			out = append(out, o)
		}
	}
	if p.Lane < 0 {
		for l := 0; l < g.N; l++ {
			add(0, l, -1, "")
		}
		return out
	}
	add(4, -1, -1, "")
	for _, item := range dotaCatalog.Items {
		if !p.has(item.ID) && dotaPrice(p, item.ID) <= p.Gold && (g.N > 1 || item.Solo) {
			add(4, -1, -1, item.ID)
		}
	}
	if p.Spent&1 == 0 {
		for l := 0; l < g.N; l++ {
			add(0, l, -1, "")
		}
	}
	for _, card := range []int{1, 2, 3, 5, 6, 7} {
		if p.Spent&(1<<card) != 0 || card == 7 && p.Charge < 3 {
			continue
		}
		untargeted := card == 2 || card == 3 || p.Hero == "axe" && card == 5 || p.Hero == "crystal_maiden" && (card == 5 || card == 7) || p.Hero == "earthshaker" && card == 7 || p.Hero == "sven" && card == 6 || p.Hero == "juggernaut" && card >= 5 || p.Hero == "drow" && card == 6 || p.Hero == "lina" && (card == 5 || card == 6) || p.Hero == "sniper" && card == 5
		found := false
		if !untargeted {
			for i, t := range g.Players {
				if t.Team != p.Team && t.Lane >= 0 && (t.Lane == p.Lane || p.Hero == "sniper" && card == 7) {
					add(card, -1, i, "")
					found = true
				}
			}
		}
		if untargeted || !found {
			add(card, -1, -1, "")
		}
	}
	if movement && (p.has("force_staff") || p.has("blink") && !p.Wounded) {
		for l := 0; l < g.N; l++ {
			if l == p.Lane {
				continue
			}
			copyG := *g
			copyG.Players = slices.Clone(g.Players)
			copyG.Players[player].Lane = l
			for _, a := range copyG.legal(player, false) {
				if a.Card == 0 || a.Card == 4 {
					continue
				}
				distance := l - p.Lane
				if distance < 0 {
					distance = -distance
				}
				if p.has("force_staff") && distance == 1 || p.has("blink") && !p.Wounded && a.Card >= 5 {
					add(a.Card, l, a.Target, "")
				}
			}
		}
	}
	return out
}

func (s *State) dotaView(player int) map[string]any {
	b, _ := json.Marshal(s)
	v := map[string]any{}
	_ = json.Unmarshal(b, &v)
	g := s.Dota
	d := v["dota"].(map[string]any)
	delete(d, "pending")
	locked := make([]bool, len(g.Players))
	plans := map[int]DotaOrder{}
	for i, p := range g.Pending {
		locked[i] = p != nil
		if p != nil && player >= 0 && player < len(g.Players) && g.Players[player].Team == g.Players[i].Team {
			plans[i] = *p
		}
	}
	d["locked"] = locked
	d["plans"] = plans
	d["catalog"] = dotaCatalog
	d["actors"] = s.DotaActors()
	d["legal"] = []DotaOrder{}
	maximum := make([]int, len(g.Players))
	for i, p := range g.Players {
		maximum[i] = p.maximum()
	}
	d["maximum"] = maximum
	if player >= 0 && player < len(g.Players) && s.Phase == "dota_plan" && !s.Finished {
		d["legal"] = g.legal(player, true)
	}
	return v
}
