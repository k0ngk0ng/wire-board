package fortressblue

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const (
	MaxPlayers = 8
	MinPlayers = 2

	PhaseSelect   = "select"
	PhasePlaying  = "playing"
	PhaseFinished = "finished"

	TeamRed  = 0
	TeamBlue = 1

	SelectSeconds = 20
	SelectMS      = SelectSeconds * 1000
	SuddenAfterMS = 8 * 60 * 1000
	MatchLimitMS  = 12 * 60 * 1000

	MaxPath       = 400
	MaxEvents     = 64
	MaxLog        = 80
	MaxCraters    = 512
	MaxSpecial    = 2
	MaxAction     = 4096
	MaxStatuses   = 8
	MaxItems      = 4
	StartingItems = 2

	Gravity    = 260.0
	WindAcc    = 14.0
	Projectile = 1.0 / 120.0
)

type Options struct {
	Map         string `json:"map"`
	TurnSeconds int    `json:"turnSeconds"`
}

type Action struct {
	Type   string  `json:"type"`
	Epoch  string  `json:"epoch"`
	Turn   int     `json:"turn"`
	Tank   int     `json:"tank"`
	Dir    int     `json:"dir"`
	Angle  float64 `json:"angle"`
	Power  float64 `json:"power"`
	Facing int     `json:"facing"`
	Weapon string  `json:"weapon"`
	Slot   int     `json:"slot"`
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Status struct {
	Kind   string `json:"kind"`
	Turns  int    `json:"turns"`
	Damage int    `json:"damage"`
	Source int    `json:"source"`
}

type Tank struct {
	SpecialUnlimited  bool     `json:"specialUnlimited,omitempty"`
	PendingShotEffect string   `json:"pendingShotEffect,omitempty"`
	Seat              int      `json:"seat"`
	Team              int      `json:"team"`
	TankID            int      `json:"tankId"`
	Selected          bool     `json:"selected"`
	X                 float64  `json:"x"`
	Y                 float64  `json:"y"`
	Facing            int      `json:"facing"`
	HP                int      `json:"hp"`
	MaxHP             int      `json:"maxHp"`
	Fuel              float64  `json:"fuel"`
	Special           int      `json:"special"`
	Alive             bool     `json:"alive"`
	Damage            int      `json:"damage"`
	Kills             int      `json:"kills"`
	Deaths            int      `json:"deaths"`
	Statuses          []Status `json:"statuses,omitempty"`
	Items             []int    `json:"items,omitempty"`
	ItemTurn          int      `json:"itemTurn,omitempty"`
}

type Crater struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	R    float64 `json:"r"`
	Seat int     `json:"seat"`
}

type Arc struct {
	PathTimes   []float64 `json:"pathTimes,omitempty"`
	ImpactAt    int64     `json:"impactAt,omitempty"`
	OutOfBounds bool      `json:"outOfBounds,omitempty"`
	Path        []Point   `json:"path"`
	Impact      Point     `json:"impact"`
	Hit         int       `json:"hit"`
}

type Hit struct {
	Healing int    `json:"healing,omitempty"`
	Victim  int    `json:"victim"`
	Damage  int    `json:"damage"`
	Kind    string `json:"kind,omitempty"`
}

type Blast struct {
	Hits    []Hit   `json:"hits,omitempty"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	R       float64 `json:"r"`
	Seat    int     `json:"seat"`
	Victim  int     `json:"victim"`
	Damage  int     `json:"damage"`
	Kind    string  `json:"kind,omitempty"`
	Terrain bool    `json:"terrain,omitempty"`
}

type Shot struct {
	CancelledAt int64    `json:"cancelledAt,omitempty"`
	LaunchAt    int64    `json:"launchAt,omitempty"`
	ImpactAt    int64    `json:"impactAt,omitempty"`
	SettleAt    int64    `json:"settleAt,omitempty"`
	ID          int64    `json:"id"`
	Seat        int      `json:"seat"`
	Weapon      string   `json:"weapon"`
	Arcs        []Arc    `json:"arcs"`
	Blasts      []Blast  `json:"blasts"`
	Craters     []Crater `json:"craters,omitempty"`
	At          int64    `json:"at"`
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

type Fall struct {
	Seat   int     `json:"seat"`
	FromX  float64 `json:"fromX"`
	FromY  float64 `json:"fromY"`
	ToX    float64 `json:"toX"`
	ToY    float64 `json:"toY"`
	At     int64   `json:"at"`
	EndAt  int64   `json:"endAt"`
	Landed bool    `json:"landed,omitempty"`
}

type Resolution struct {
	LaunchAt int64  `json:"launchAt,omitempty"`
	Stage    string `json:"stage"`
	ShotID   int64  `json:"shotId"`
	At       int64  `json:"at"`
	ImpactAt int64  `json:"impactAt"`
	SettleAt int64  `json:"settleAt"`
	Falls    []Fall `json:"falls,omitempty"`
}

type PendingShot struct {
	Launched bool   `json:"launched,omitempty"`
	Weapon   Weapon `json:"weapon"`
	Kind     string `json:"kind,omitempty"`
}

type Hazard struct {
	ID         int64   `json:"id"`
	Kind       string  `json:"kind"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	R          float64 `json:"r"`
	At         int64   `json:"at"`
	EndAt      int64   `json:"endAt"`
	NextTickAt int64   `json:"nextTickAt"`
	Source     int     `json:"source"`
}

type State struct {
	Hazards        []Hazard `json:"hazards,omitempty"`
	needsAdvance   bool
	resumeMovement bool

	MoveRemainingMS int64        `json:"moveRemainingMs,omitempty"`
	Resolution      *Resolution  `json:"resolution,omitempty"`
	PendingShot     *PendingShot `json:"pendingShot,omitempty"`

	Options     Options  `json:"options"`
	Epoch       string   `json:"epoch"`
	Seed        uint64   `json:"seed"`
	MapID       string   `json:"mapId"`
	Phase       string   `json:"phase"`
	SelectUntil int64    `json:"selectUntil"`
	StartAt     int64    `json:"startAt"`
	SuddenAt    int64    `json:"suddenAt"`
	MatchEndsAt int64    `json:"matchEndsAt"`
	Sudden      bool     `json:"sudden"`
	Turn        int      `json:"turn"`
	Current     int      `json:"current"`
	Wind        int      `json:"wind"`
	Deadline    int64    `json:"deadline"`
	Order       []int    `json:"order"`
	Tanks       []Tank   `json:"tanks"`
	Craters     []Crater `json:"craters"`
	LastShot    *Shot    `json:"lastShot,omitempty"`
	Events      []Event  `json:"events"`
	Log         []string `json:"log"`
	Sequence    int64    `json:"sequence"`
	Finished    bool     `json:"finished"`
	WinnerTeam  int      `json:"winnerTeam"`
	Draw        bool     `json:"draw"`
	Reason      string   `json:"reason"`
	DamageTotal []int    `json:"damageTotal"`
}

type TankView struct {
	SpecialUnlimited  bool     `json:"specialUnlimited,omitempty"`
	PendingShotEffect string   `json:"pendingShotEffect,omitempty"`
	Seat              int      `json:"seat"`
	Team              int      `json:"team"`
	TankID            int      `json:"tankId"`
	Selected          bool     `json:"selected"`
	X                 float64  `json:"x"`
	Y                 float64  `json:"y"`
	Facing            int      `json:"facing"`
	HP                int      `json:"hp"`
	MaxHP             int      `json:"maxHp"`
	Fuel              float64  `json:"fuel"`
	Special           int      `json:"special"`
	Alive             bool     `json:"alive"`
	Damage            int      `json:"damage"`
	Kills             int      `json:"kills"`
	Deaths            int      `json:"deaths"`
	Statuses          []Status `json:"statuses,omitempty"`
	Items             []int    `json:"items,omitempty"`
	Current           bool     `json:"current"`
}

type View struct {
	Hazards     []Hazard    `json:"hazards,omitempty"`
	Resolution  *Resolution `json:"resolution,omitempty"`
	Options     Options     `json:"options"`
	Epoch       string      `json:"epoch"`
	Phase       string      `json:"phase"`
	MapID       string      `json:"mapId"`
	MapName     string      `json:"mapName"`
	MapWidth    int         `json:"mapWidth"`
	MapHeight   int         `json:"mapHeight"`
	MapOffY     int         `json:"mapOffY"`
	SelectUntil int64       `json:"selectUntil"`
	StartAt     int64       `json:"startAt"`
	SuddenAt    int64       `json:"suddenAt"`
	MatchEndsAt int64       `json:"matchEndsAt"`
	Deadline    int64       `json:"deadline"`
	Sudden      bool        `json:"sudden"`
	Turn        int         `json:"turn"`
	Current     int         `json:"current"`
	Wind        int         `json:"wind"`
	Order       []int       `json:"order"`
	TeamSize    int         `json:"teamSize"`
	Tanks       []TankView  `json:"tanks"`
	Craters     []Crater    `json:"craters,omitempty"`
	LastShot    *Shot       `json:"lastShot,omitempty"`
	Events      []Event     `json:"events"`
	Log         []string    `json:"log"`
	You         int         `json:"you"`
	CanSelect   bool        `json:"canSelect"`
	CanMove     bool        `json:"canMove"`
	CanFire     bool        `json:"canFire"`
	Selected    int         `json:"selectedTank,omitempty"`
}

func teamSize(players int) int {
	if players < MinPlayers || players > MaxPlayers || players%2 != 0 {
		return 0
	}
	return players / 2
}

func teamOf(seat, players int) int {
	if seat >= players/2 {
		return TeamBlue
	}
	return TeamRed
}

func turnOrder(players int) []int {
	t := players / 2
	out := make([]int, 0, players)
	for i := 0; i < t; i++ {
		out = append(out, i, t+i)
	}
	return out
}

func mapByID(id string) (fbmaps.Map, bool) {
	if id == "random" {
		return fbmaps.Map{}, false
	}
	return fbmaps.ByID(id)
}

func tankSpec(id int) (fbdata.Tank, bool) { return fbdata.TankByID(id) }

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

func (s *State) canAct(seat int) bool {
	return s.Phase == PhasePlaying && s.Resolution == nil && !s.Finished && seat >= 0 && seat < len(s.Tanks) && seat == s.Current && s.Tanks[seat].Alive
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

func teamName(team int) string {
	if team == TeamBlue {
		return "蓝队"
	}
	return "红队"
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
	tanks := make([]map[string]any, len(s.Tanks))
	for i, t := range s.Tanks {
		tanks[i] = map[string]any{"seat": t.Seat, "team": t.Team, "tank": t.TankID, "damage": t.Damage, "kills": t.Kills, "deaths": t.Deaths}
	}
	details, _ := json.Marshal(map[string]any{"map": s.MapID, "reason": s.Reason, "draw": s.Draw, "winnerTeam": s.WinnerTeam, "tanks": tanks, "damage": s.DamageTotal})
	r.Details = details
	return r
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
		if s.Current >= 0 && s.Resolution == nil {
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
	next := s.MatchEndsAt
	if !s.Sudden && s.SuddenAt > 0 {
		next = s.SuddenAt
	}
	if s.Resolution != nil {
		next = s.resolutionDeadline()
	}
	if hazard := s.hazardDeadline(); hazard > 0 && (next == 0 || hazard < next) {
		next = hazard
	}
	return next
}

func (s *State) project(seat int) View {
	m, _ := fbmaps.ByID(s.MapID)
	teamSize := len(s.Tanks) / 2
	v := View{
		Hazards: append([]Hazard(nil), s.Hazards...), Resolution: s.Resolution, Options: s.Options, Epoch: s.Epoch, Phase: s.Phase, MapID: s.MapID, MapName: m.Name,
		MapWidth: m.Width, MapHeight: m.Height, MapOffY: m.OffY,
		SelectUntil: s.SelectUntil, StartAt: s.StartAt, SuddenAt: s.SuddenAt,
		MatchEndsAt: s.MatchEndsAt, Deadline: s.Deadline, Sudden: s.Sudden,
		Turn: s.Turn, Current: s.Current, Wind: s.Wind, Order: append([]int(nil), s.Order...),
		TeamSize: teamSize, Tanks: make([]TankView, len(s.Tanks)),
		Craters:  append([]Crater(nil), s.Craters...),
		LastShot: s.LastShot, Events: append([]Event(nil), s.Events...),
		Log: append([]string(nil), s.Log...), You: seat,
	}
	for i, t := range s.Tanks {
		v.Tanks[i] = TankView{SpecialUnlimited: t.SpecialUnlimited, PendingShotEffect: t.PendingShotEffect, Seat: t.Seat, Team: t.Team, TankID: t.TankID, Selected: t.Selected, X: t.X, Y: t.Y, Facing: t.Facing, HP: t.HP, MaxHP: t.MaxHP, Fuel: t.Fuel, Special: t.Special, Alive: t.Alive, Damage: t.Damage, Kills: t.Kills, Deaths: t.Deaths, Statuses: append([]Status(nil), t.Statuses...), Items: append([]int(nil), t.Items...), Current: s.Phase == PhasePlaying && s.Resolution == nil && i == s.Current}
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
			v.Selected = t.TankID
		}
	}
	return v
}

func validDeadline(now time.Time, turnSeconds int) int64 {
	_ = now
	return int64(turnSeconds) * 1000
}
