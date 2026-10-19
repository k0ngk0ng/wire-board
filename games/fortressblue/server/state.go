package fortressblue

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const (
	FieldWidth  = 320.0
	FieldHeight = 180.0
	Columns     = 160
	ColumnWidth = 2.0
	MaxPlayers  = 10
	MinPlayers  = 2

	PhaseSelect   = "select"
	PhasePlaying  = "playing"
	PhaseFinished = "finished"

	TeamRed  = 0
	TeamBlue = 1

	SelectSeconds = 20
	SelectMS      = SelectSeconds * 1000
	SuddenAfterMS = 8 * 60 * 1000
	MatchLimitMS  = 12 * 60 * 1000

	MaxPath    = 400
	MaxEvents  = 64
	MaxLog     = 80
	MaxSpecial = 3
	MaxAction  = 4096

	Gravity = 110.0
	WindAcc = 5.5
)

type Options struct {
	Map         string `json:"map"`
	TurnSeconds int    `json:"turnSeconds"`
}

type ClassSpec struct {
	ID          string
	Name        string
	MaxHP       int
	Move        float64
	Primary     Weapon
	Special     Weapon
	SpecialAmmo int
	Color       string
	Description string
}

type Weapon struct {
	Name   string  `json:"name"`
	Damage float64 `json:"damage"`
	Blast  float64 `json:"blast"`
	Shots  int     `json:"shots"`
	Spread float64 `json:"spread"`
	Heal   bool    `json:"heal,omitempty"`
}

type MapSpec struct {
	ID    string
	Name  string
	Water float64
}

var classOrder = []string{"balanced", "heavy", "scout", "sniper", "support"}

var classes = map[string]ClassSpec{
	"balanced": {
		ID: "balanced", Name: "均衡", MaxHP: 100, Move: 42, Color: "#4d7f72",
		Primary:     Weapon{Name: "标准炮", Damage: 34, Blast: 22, Shots: 1},
		Special:     Weapon{Name: "双联炮", Damage: 21, Blast: 14, Shots: 2, Spread: 2.2},
		SpecialAmmo: 2, Description: "各项均衡，双联炮覆盖更宽。",
	},
	"heavy": {
		ID: "heavy", Name: "重装", MaxHP: 145, Move: 27, Color: "#8b6048",
		Primary:     Weapon{Name: "重炮", Damage: 43, Blast: 27, Shots: 1},
		Special:     Weapon{Name: "重锤", Damage: 62, Blast: 35, Shots: 1},
		SpecialAmmo: 1, Description: "血厚、爆发高，移动缓慢。",
	},
	"scout": {
		ID: "scout", Name: "侦察", MaxHP: 82, Move: 66, Color: "#c49b4a",
		Primary:     Weapon{Name: "速射炮", Damage: 27, Blast: 16, Shots: 1},
		Special:     Weapon{Name: "三连发", Damage: 15, Blast: 12, Shots: 3, Spread: 3.8},
		SpecialAmmo: 2, Description: "机动性最好，三连发适合削血。",
	},
	"sniper": {
		ID: "sniper", Name: "狙击", MaxHP: 88, Move: 31, Color: "#6d7598",
		Primary:     Weapon{Name: "精准炮", Damage: 39, Blast: 14, Shots: 1},
		Special:     Weapon{Name: "穿甲弹", Damage: 64, Blast: 8, Shots: 1},
		SpecialAmmo: 2, Description: "弹道精准，穿甲弹单体伤害极高。",
	},
	"support": {
		ID: "support", Name: "支援", MaxHP: 112, Move: 37, Color: "#81755b",
		Primary:     Weapon{Name: "榴弹炮", Damage: 29, Blast: 21, Shots: 1},
		Special:     Weapon{Name: "维修弹", Damage: 0, Blast: 27, Shots: 1, Heal: true},
		SpecialAmmo: 3, Description: "维修弹为爆炸范围内的队友恢复生命。",
	},
}

var mapOrder = []string{"hills", "canyon", "islands", "snow", "volcano"}
var maps = map[string]MapSpec{
	"hills":   {ID: "hills", Name: "层叠丘陵", Water: 7},
	"canyon":  {ID: "canyon", Name: "回声峡谷", Water: 6},
	"islands": {ID: "islands", Name: "浮岛浅滩", Water: 12},
	"snow":    {ID: "snow", Name: "雪原斜面", Water: 6},
	"volcano": {ID: "volcano", Name: "火山口", Water: 8},
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Tank struct {
	Seat     int     `json:"seat"`
	Team     int     `json:"team"`
	Class    string  `json:"class"`
	Selected bool    `json:"selected"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Facing   int     `json:"facing"`
	HP       int     `json:"hp"`
	MaxHP    int     `json:"maxHp"`
	Fuel     float64 `json:"fuel"`
	Special  int     `json:"special"`
	Shield   int     `json:"shield"`
	Damage   int     `json:"damage"`
	Kills    int     `json:"kills"`
	Deaths   int     `json:"deaths"`
	Alive    bool    `json:"alive"`
}

type Arc struct {
	Path   []Point `json:"path"`
	Impact Point   `json:"impact"`
	Hit    int     `json:"hit"`
}

type Blast struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	R       float64 `json:"r"`
	Seat    int     `json:"seat"`
	Victim  int     `json:"victim"`
	Damage  int     `json:"damage"`
	Heal    int     `json:"heal,omitempty"`
	Terrain bool    `json:"terrain,omitempty"`
}

type Shot struct {
	ID     int64   `json:"id"`
	Seat   int     `json:"seat"`
	Weapon string  `json:"weapon"`
	Arcs   []Arc   `json:"arcs"`
	Blasts []Blast `json:"blasts"`
	At     int64   `json:"at"`
}

type Event struct {
	ID     int64   `json:"id"`
	Type   string  `json:"type"`
	Seat   int     `json:"seat"`
	Target int     `json:"target"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Amount int     `json:"amount,omitempty"`
	At     int64   `json:"at"`
	Text   string  `json:"text,omitempty"`
}

type State struct {
	Options     Options   `json:"options"`
	Epoch       string    `json:"epoch"`
	Seed        uint64    `json:"seed"`
	MapID       string    `json:"mapId"`
	Phase       string    `json:"phase"`
	SelectUntil int64     `json:"selectUntil"`
	StartAt     int64     `json:"startAt"`
	SuddenAt    int64     `json:"suddenAt"`
	MatchEndsAt int64     `json:"matchEndsAt"`
	Sudden      bool      `json:"sudden"`
	Turn        int       `json:"turn"`
	Current     int       `json:"current"`
	Wind        int       `json:"wind"`
	Deadline    int64     `json:"deadline"`
	Order       []int     `json:"order"`
	Heights     []float64 `json:"heights"`
	Tanks       []Tank    `json:"tanks"`
	LastShot    *Shot     `json:"lastShot,omitempty"`
	Events      []Event   `json:"events"`
	Log         []string  `json:"log"`
	Sequence    int64     `json:"sequence"`
	Finished    bool      `json:"finished"`
	WinnerTeam  int       `json:"winnerTeam"`
	Draw        bool      `json:"draw"`
	Reason      string    `json:"reason"`
	DamageTotal []int     `json:"damageTotal"`
}

type TankView struct {
	Seat     int     `json:"seat"`
	Team     int     `json:"team"`
	Class    string  `json:"class"`
	Selected bool    `json:"selected"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Facing   int     `json:"facing"`
	HP       int     `json:"hp"`
	MaxHP    int     `json:"maxHp"`
	Fuel     float64 `json:"fuel"`
	Special  int     `json:"special"`
	Shield   int     `json:"shield"`
	Damage   int     `json:"damage"`
	Kills    int     `json:"kills"`
	Deaths   int     `json:"deaths"`
	Alive    bool    `json:"alive"`
}

type View struct {
	Options     Options    `json:"options"`
	Epoch       string     `json:"epoch"`
	Phase       string     `json:"phase"`
	MapID       string     `json:"mapId"`
	SelectUntil int64      `json:"selectUntil"`
	StartAt     int64      `json:"startAt"`
	SuddenAt    int64      `json:"suddenAt"`
	MatchEndsAt int64      `json:"matchEndsAt"`
	Deadline    int64      `json:"deadline"`
	Sudden      bool       `json:"sudden"`
	Turn        int        `json:"turn"`
	Current     int        `json:"current"`
	Wind        int        `json:"wind"`
	Order       []int      `json:"order"`
	TeamSize    int        `json:"teamSize"`
	Tanks       []TankView `json:"tanks"`
	Heights     []float64  `json:"heights"`
	LastShot    *Shot      `json:"lastShot,omitempty"`
	Events      []Event    `json:"events"`
	Log         []string   `json:"log"`
	You         int        `json:"you"`
	CanSelect   bool       `json:"canSelect"`
	CanMove     bool       `json:"canMove"`
	CanFire     bool       `json:"canFire"`
	Selected    string     `json:"selected,omitempty"`
}

type Action struct {
	Type   string  `json:"type"`
	Epoch  string  `json:"epoch"`
	Turn   int     `json:"turn,omitempty"`
	Class  string  `json:"class,omitempty"`
	Dir    int     `json:"dir,omitempty"`
	Angle  float64 `json:"angle,omitempty"`
	Power  float64 `json:"power,omitempty"`
	Facing int     `json:"facing,omitempty"`
	Weapon string  `json:"weapon,omitempty"`
}

func classByID(id string) (ClassSpec, bool) {
	c, ok := classes[id]
	return c, ok
}

func mapByID(id string) (MapSpec, bool) {
	m, ok := maps[id]
	return m, ok
}

func teamSize(players int) int {
	if players%2 != 0 || players < MinPlayers || players > MaxPlayers {
		return 0
	}
	return players / 2
}

func teamOf(seat, players int) int {
	if seat < players/2 {
		return TeamRed
	}
	return TeamBlue
}

func turnOrder(players int) []int {
	t := players / 2
	order := make([]int, 0, players)
	for i := 0; i < t; i++ {
		order = append(order, i, t+i)
	}
	return order
}

func (s *State) canAct(seat int) bool {
	return s.Phase == PhasePlaying && !s.Finished && seat >= 0 && seat < len(s.Tanks) && seat == s.Current && s.Tanks[seat].Alive
}

func (s *State) note(text string) {
	s.Log = append(s.Log, text)
	if len(s.Log) > MaxLog {
		s.Log = append([]string(nil), s.Log[len(s.Log)-MaxLog:]...)
	}
}

func (s *State) emit(kind string, seat, target int, x, y float64, amount int, text string, at int64) {
	s.Sequence++
	ev := Event{ID: s.Sequence, Type: kind, Seat: seat, Target: target, X: x, Y: y, Amount: amount, At: at, Text: text}
	s.Events = append(s.Events, ev)
	if len(s.Events) > MaxEvents {
		s.Events = append([]Event(nil), s.Events[len(s.Events)-MaxEvents:]...)
	}
}

func hashString(parts ...string) uint64 {
	h := fnv.New64a()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

func windFor(seed uint64, turn int, sudden bool) int {
	h := hashString(fmt.Sprint(seed), fmt.Sprint(turn), fmt.Sprint(sudden))
	wind := int(h%21) - 10
	if sudden {
		if wind >= 0 {
			wind = min(20, wind*2+2)
		} else {
			wind = max(-20, wind*2-2)
		}
	}
	return wind
}

func (s *State) teamAlive(team int) int {
	n := 0
	for _, t := range s.Tanks {
		if t.Team == team && t.Alive {
			n++
		}
	}
	return n
}

func (s *State) teamHP(team int) int {
	n := 0
	for _, t := range s.Tanks {
		if t.Team == team && t.Alive {
			n += max(0, t.HP)
		}
	}
	return n
}

func (s *State) finish(draw bool, winner int, reason string, at int64) {
	if s.Finished {
		return
	}
	s.Finished = true
	s.Draw = draw
	s.WinnerTeam = winner
	s.Reason = reason
	s.Phase = PhaseFinished
	s.Deadline = 0
	s.emit("finish", -1, -1, 0, 0, 0, reason, at)
	if draw {
		s.note("对局结束：" + reason + "，双方和局")
	} else {
		s.note("对局结束：" + reason + "，胜方为" + teamName(winner))
	}
}

func teamName(team int) string {
	if team == TeamBlue {
		return "蓝队"
	}
	return "红队"
}

func (s *State) checkTeamOutcome(at int64) bool {
	red, blue := s.teamAlive(TeamRed), s.teamAlive(TeamBlue)
	if red == 0 && blue == 0 {
		s.finish(true, -1, "双方同时全灭", at)
		return true
	}
	if red == 0 {
		s.finish(false, TeamBlue, "红队全灭", at)
		return true
	}
	if blue == 0 {
		s.finish(false, TeamRed, "蓝队全灭", at)
		return true
	}
	return false
}

func (s *State) timeOutcome(at int64) {
	redHP, blueHP := s.teamHP(TeamRed), s.teamHP(TeamBlue)
	if redHP == blueHP {
		s.finish(true, -1, "时间到，双方剩余战力相同", at)
	} else if redHP > blueHP {
		s.finish(false, TeamRed, "时间到，红队剩余战力领先", at)
	} else {
		s.finish(false, TeamBlue, "时间到，蓝队剩余战力领先", at)
	}
}

func (s *State) result() *gameapi.Result {
	if !s.Finished {
		return nil
	}
	r := &gameapi.Result{Team: true, Ranks: make([]int, len(s.Tanks)), Winners: []int{}, Scores: make([]int, len(s.Tanks))}
	for i := range s.Tanks {
		r.Scores[i] = s.Tanks[i].Damage + s.Tanks[i].Kills*25
	}
	if s.Draw {
		r.Draw = true
		for i := range r.Ranks {
			r.Ranks[i] = 1
		}
	} else {
		for i, t := range s.Tanks {
			if t.Team == s.WinnerTeam {
				r.Ranks[i] = 1
				r.Winners = append(r.Winners, i)
			} else {
				r.Ranks[i] = 2
			}
		}
	}
	details, _ := json.Marshal(map[string]any{"map": s.MapID, "reason": s.Reason, "draw": s.Draw, "winnerTeam": s.WinnerTeam, "classes": classList(s.Tanks), "damage": s.DamageTotal})
	r.Details = details
	return r
}

func classList(tanks []Tank) []string {
	out := make([]string, len(tanks))
	for i, t := range tanks {
		out[i] = t.Class
	}
	return out
}

func (s *State) envelope() (gameapi.Match, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	m := gameapi.Match{Schema: 1, Data: raw, Log: append([]string(nil), s.Log...)}
	if s.Finished {
		m.Result = s.result()
		return m, nil
	}
	switch s.Phase {
	case PhaseSelect:
		actors := []int{}
		for i, t := range s.Tanks {
			if !t.Selected {
				actors = append(actors, i)
			}
		}
		m.Actors = actors
		m.Attention = append([]int(nil), actors...)
		m.TimeoutActors = append([]int(nil), actors...)
		m.Deadline = s.SelectUntil
		m.SystemDeadline = s.SelectUntil
	case PhasePlaying:
		if s.Current >= 0 {
			m.Actors = []int{s.Current}
			m.Attention = []int{s.Current}
			m.TimeoutActors = []int{s.Current}
			m.Deadline = s.Deadline
		}
		m.Eliminated = s.deadSeats()
		m.SystemDeadline = s.systemDeadline()
	}
	return m, nil
}

func (s *State) deadSeats() []int {
	out := []int{}
	for i, t := range s.Tanks {
		if !t.Alive {
			out = append(out, i)
		}
	}
	return out
}

func (s *State) systemDeadline() int64 {
	if s.Finished {
		return 0
	}
	if s.Phase == PhaseSelect {
		return s.SelectUntil
	}
	if s.Phase != PhasePlaying {
		return 0
	}
	if !s.Sudden {
		if s.SuddenAt > 0 {
			return s.SuddenAt
		}
		return s.MatchEndsAt
	}
	return s.MatchEndsAt
}

func (s *State) project(seat int) View {
	teamSize := len(s.Tanks) / 2
	v := View{
		Options: s.Options, Epoch: s.Epoch, Phase: s.Phase, MapID: s.MapID,
		SelectUntil: s.SelectUntil, StartAt: s.StartAt, SuddenAt: s.SuddenAt,
		MatchEndsAt: s.MatchEndsAt, Deadline: s.Deadline, Sudden: s.Sudden,
		Turn: s.Turn, Current: s.Current, Wind: s.Wind, Order: append([]int(nil), s.Order...),
		TeamSize: teamSize, Tanks: make([]TankView, len(s.Tanks)),
		Heights:  append([]float64(nil), s.Heights...),
		LastShot: s.LastShot, Events: append([]Event(nil), s.Events...),
		Log: append([]string(nil), s.Log...), You: seat,
	}
	for i, t := range s.Tanks {
		v.Tanks[i] = TankView{Seat: t.Seat, Team: t.Team, Class: t.Class, Selected: t.Selected, X: t.X, Y: t.Y, Facing: t.Facing, HP: t.HP, MaxHP: t.MaxHP, Fuel: t.Fuel, Special: t.Special, Shield: t.Shield, Damage: t.Damage, Kills: t.Kills, Deaths: t.Deaths, Alive: t.Alive}
	}
	if seat >= 0 && seat < len(s.Tanks) {
		t := s.Tanks[seat]
		if s.Phase == PhaseSelect {
			v.CanSelect = !t.Selected
		}
		if s.canAct(seat) {
			v.CanMove = t.Fuel > 0
			v.CanFire = true
		}
		if t.Selected {
			v.Selected = t.Class
		}
	}
	return v
}

func (s *State) validDeadline(now time.Time) int64 {
	if s.Phase != PhasePlaying || s.Finished {
		return 0
	}
	if s.Deadline <= now.UnixMilli() {
		return now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
	}
	return s.Deadline
}
