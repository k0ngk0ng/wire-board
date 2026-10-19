// Package battlecity implements the server-authoritative 1v1/2v2/3v3 tank battle.
package battlecity

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/k0ngk0ng/wire-board/games/battlecity/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const (
	Cols      = maps.Cols
	Rows      = maps.Rows
	Cell      = 16
	SubCols   = Cols * 2
	SubRows   = Rows * 2
	SubSize   = 8
	FrameMS   = int64(100)
	StepMS    = int64(20)
	SelectMS  = int64(15000)
	Countdown = int64(3000)
	MatchMS   = int64(10 * 60 * 1000)
	RespawnMS = int64(4000)
	ShieldMS  = int64(2000)
	SpareMS   = int64(3000)
	IdleMS    = int64(15000)
	LostMS    = int64(750)
	DropMS    = int64(25000)
	ShovelMS  = int64(20000)
	FreezeMS  = int64(8000)

	MaxPlayers    = 6
	MaxBullets    = 32
	MaxPowerUps   = 4
	MaxEvents     = 64
	MaxLog        = 80
	MaxPending    = 64
	MaxActionSize = 8 * 1024
	MaxSeq        = int64(1<<31 - 1)
	MaxSeqJump    = int64(1024)
	MaxCatchUp    = 20
)

const (
	PhaseSelect    = "select"
	PhaseCountdown = "countdown"
	PhasePlaying   = "playing"
	PhaseFinished  = "finished"
)

const (
	TeamTop    = 0
	TeamBottom = 1
)

// Directions are fixed for the server and client.
const (
	DirUp = iota
	DirRight
	DirDown
	DirLeft
)

var dirDX = [4]float64{0, 1, 0, -1}
var dirDY = [4]float64{-1, 0, 1, 0}

var tankOrder = []string{"balanced", "basic", "fast", "power", "armor"}

var tankSpecs = map[string]TankSpec{
	"balanced": {Speed: 44, HP: 1, BulletSpeed: 220, MaxBullets: 1, ReloadMS: 350},
	"basic":    {Speed: 34, HP: 1, BulletSpeed: 200, MaxBullets: 1, ReloadMS: 450},
	"fast":     {Speed: 62, HP: 1, BulletSpeed: 200, MaxBullets: 1, ReloadMS: 450},
	"power":    {Speed: 38, HP: 1, BulletSpeed: 300, MaxBullets: 1, ReloadMS: 280},
	"armor":    {Speed: 28, HP: 4, BulletSpeed: 200, MaxBullets: 1, ReloadMS: 450},
}

type TankSpec struct {
	Speed       float64
	HP          int
	BulletSpeed float64
	MaxBullets  int
	ReloadMS    int64
}

func ValidTank(class string) bool {
	_, ok := tankSpecs[class]
	return ok
}

type Options struct {
	Map string `json:"map"`
}

type Input struct {
	At   int64  `json:"at"`
	Move string `json:"move"`
	Fire bool   `json:"fire"`
}

type Tank struct {
	Seat        int     `json:"seat"`
	Team        int     `json:"team"`
	Class       string  `json:"class,omitempty"`
	Selected    bool    `json:"selected"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Dir         int     `json:"dir"`
	HP          int     `json:"hp"`
	MaxHP       int     `json:"maxHp"`
	Tech        int     `json:"tech"`
	Alive       bool    `json:"alive"`
	Spare       bool    `json:"spare,omitempty"`
	RespawnAt   int64   `json:"respawnAt"`
	ShieldUntil int64   `json:"shieldUntil"`
	FreezeUntil int64   `json:"freezeUntil"`
	LastFireAt  int64   `json:"lastFireAt"`
	LastInputAt int64   `json:"lastInputAt"`
	IdleAt      int64   `json:"idleAt"`
	Kills       int     `json:"kills"`
	Deaths      int     `json:"deaths"`
	SpawnIndex  int     `json:"spawnIndex"`
	Applied     Input   `json:"applied"`
	Pending     []Input `json:"pending,omitempty"`
	Seq         int64   `json:"seq"`
	SlideDir    int     `json:"slideDir"`
	SlideUntil  int64   `json:"slideUntil"`
}

type Bullet struct {
	ID    int     `json:"id"`
	Seat  int     `json:"seat"`
	Team  int     `json:"team"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Dir   int     `json:"dir"`
	Speed float64 `json:"speed"`
	Tech  int     `json:"tech"`
	At    int64   `json:"at"`
}

type PowerUp struct {
	ID       int     `json:"id"`
	Type     string  `json:"type"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	SpawnAt  int64   `json:"spawnAt"`
	ExpireAt int64   `json:"expireAt"`
}

type Base struct {
	Team        int   `json:"team"`
	Alive       bool  `json:"alive"`
	DestroyedAt int64 `json:"destroyedAt"`
	DestroyedBy int   `json:"destroyedBy"`
	DestroyTick int64 `json:"destroyTick"`
}

type Event struct {
	ID     int     `json:"id"`
	Type   string  `json:"type"`
	At     int64   `json:"at"`
	Seat   int     `json:"seat"`
	Team   int     `json:"team"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Target int     `json:"target,omitempty"`
	Kind   string  `json:"kind,omitempty"`
	Count  int     `json:"count,omitempty"`
}

type State struct {
	Schema      int             `json:"schema"`
	Options     Options         `json:"options"`
	Phase       string          `json:"phase"`
	Epoch       string          `json:"epoch"`
	Seed        uint64          `json:"seed"`
	ServerAt    int64           `json:"serverAt"`
	SelectUntil int64           `json:"selectUntil"`
	StartAt     int64           `json:"startAt"`
	MatchEndsAt int64           `json:"matchEndsAt"`
	Tick        int64           `json:"tick"`
	Players     int             `json:"players"`
	TeamSize    int             `json:"teamSize"`
	MapID       string          `json:"mapId"`
	Broken      []byte          `json:"broken"`
	ShovelUntil [2]int64        `json:"shovelUntil"`
	Tanks       []Tank          `json:"tanks"`
	Bullets     []Bullet        `json:"bullets"`
	PowerUps    []PowerUp       `json:"powerUps"`
	Bases       [2]Base         `json:"bases"`
	Events      []Event         `json:"events"`
	EventID     int             `json:"eventId"`
	BulletID    int             `json:"bulletId"`
	PowerUpID   int             `json:"powerUpId"`
	Log         []string        `json:"log"`
	Result      *gameapi.Result `json:"result,omitempty"`

	cells [Rows][Cols]byte
}

type TankView struct {
	Seat        int     `json:"seat"`
	Team        int     `json:"team"`
	Class       string  `json:"class,omitempty"`
	Selected    bool    `json:"selected"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Dir         int     `json:"dir"`
	HP          int     `json:"hp"`
	MaxHP       int     `json:"maxHp"`
	Tech        int     `json:"tech"`
	Alive       bool    `json:"alive"`
	RespawnAt   int64   `json:"respawnAt"`
	ShieldUntil int64   `json:"shieldUntil"`
	FreezeUntil int64   `json:"freezeUntil"`
	Kills       int     `json:"kills"`
	Deaths      int     `json:"deaths"`
	Seq         int64   `json:"seq"`
}

type BulletView struct {
	ID   int     `json:"id"`
	Seat int     `json:"seat"`
	Team int     `json:"team"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Dir  int     `json:"dir"`
}

type PowerUpView struct {
	ID       int     `json:"id"`
	Type     string  `json:"type"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	ExpireAt int64   `json:"expireAt"`
}

type BaseView struct {
	Team        int   `json:"team"`
	Alive       bool  `json:"alive"`
	DestroyedAt int64 `json:"destroyedAt"`
	DestroyedBy int   `json:"destroyedBy"`
}

type Controls struct {
	CanSelect bool `json:"canSelect"`
	CanPlay   bool `json:"canPlay"`
}

type View struct {
	Options     Options       `json:"options"`
	Phase       string        `json:"phase"`
	Epoch       string        `json:"epoch"`
	ServerAt    int64         `json:"serverAt"`
	SelectUntil int64         `json:"selectUntil"`
	StartAt     int64         `json:"startAt"`
	MatchEndsAt int64         `json:"matchEndsAt"`
	TeamSize    int           `json:"teamSize"`
	MapID       string        `json:"mapId"`
	Broken      []byte        `json:"broken"`
	ShovelUntil [2]int64      `json:"shovelUntil"`
	Tanks       []TankView    `json:"tanks"`
	Bullets     []BulletView  `json:"bullets"`
	PowerUps    []PowerUpView `json:"powerUps"`
	Bases       [2]BaseView   `json:"bases"`
	Events      []Event       `json:"events"`
	Controls    Controls      `json:"controls"`
}

func strict(raw json.RawMessage, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}

func configure(n int, raw json.RawMessage) (json.RawMessage, error) {
	if n != 2 && n != 4 && n != 6 {
		return nil, errors.New("坦克大战支持 1v1、2v2 或 3v3")
	}
	o := Options{Map: "classic-01"}
	if len(raw) > 0 {
		if err := strict(raw, &o); err != nil {
			return nil, errors.New("无效坦克大战设置")
		}
	}
	if o.Map != "random" && !maps.ValidID(o.Map) {
		return nil, errors.New("请选择有效地图")
	}
	return json.Marshal(o)
}

func newState(ctx gameapi.Context, o Options) (State, error) {
	if o.Map == "random" {
		list := maps.List()
		var b [1]byte
		if _, err := rand.Read(b[:]); err != nil {
			return State{}, err
		}
		o.Map = list[int(b[0])%len(list)].ID
	}
	m, ok := maps.ByID(o.Map)
	if !ok {
		return State{}, maps.ErrUnknown
	}
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		return State{}, err
	}
	var seedBytes [8]byte
	if _, err := rand.Read(seedBytes[:]); err != nil {
		return State{}, err
	}
	var seed uint64
	for _, b := range seedBytes {
		seed = seed<<8 | uint64(b)
	}
	now := ctx.Now.UnixMilli()
	teamSize := ctx.Players / 2
	s := State{
		Schema:      1,
		Options:     o,
		Phase:       PhaseSelect,
		Epoch:       hex.EncodeToString(token[:]),
		Seed:        seed,
		ServerAt:    now,
		SelectUntil: now + SelectMS,
		Players:     ctx.Players,
		TeamSize:    teamSize,
		MapID:       m.ID,
		Broken:      make([]byte, SubCols*SubRows),
		Tanks:       make([]Tank, ctx.Players),
		Events:      []Event{},
		Log:         []string{"坦克大战：选择坦克，准备开战"},
		cells:       m.Cells,
	}
	for team := 0; team < 2; team++ {
		s.Bases[team] = Base{Team: team, Alive: true, DestroyedBy: -1, DestroyTick: -1}
	}
	spawns := [2][]maps.Point{maps.SpawnPoints(teamSize, TeamTop), maps.SpawnPoints(teamSize, TeamBottom)}
	for i := range s.Tanks {
		team := 0
		if i >= teamSize {
			team = 1
		}
		idx := i
		if team == 1 {
			idx = i - teamSize
		}
		p := spawns[team][idx%len(spawns[team])]
		s.Tanks[i] = Tank{
			Seat: i, Team: team, X: float64(p.X * Cell), Y: float64(p.Y * Cell),
			Dir: DirUp, SpawnIndex: idx, IdleAt: now + IdleMS, LastInputAt: now,
			Applied: Input{At: now, Move: "none"},
		}
	}
	return s, nil
}

func (s *State) rand() uint64 {
	s.Seed += 0x9E3779B97F4A7C15
	z := s.Seed
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func (s *State) randN(n int) int {
	if n <= 0 {
		return 0
	}
	return int(s.rand() % uint64(n))
}

func (s *State) emit(kind string, seat, team int, at int64, x, y float64, target int, extraKind string, count int) {
	s.EventID++
	e := Event{ID: s.EventID, Type: kind, At: at, Seat: seat, Team: team, X: x, Y: y, Target: target, Kind: extraKind, Count: count}
	s.Events = append(s.Events, e)
	if len(s.Events) > MaxEvents {
		s.Events = s.Events[len(s.Events)-MaxEvents:]
	}
}

func (s *State) log(msg string) {
	s.Log = append(s.Log, msg)
	if len(s.Log) > MaxLog {
		s.Log = s.Log[len(s.Log)-MaxLog:]
	}
}

func decode(m gameapi.Match) (State, error) {
	var s State
	if m.Schema != 1 {
		return s, errors.New("不支持的坦克大战存档版本")
	}
	if err := json.Unmarshal(m.Data, &s); err != nil {
		return s, err
	}
	if s.Schema != 1 {
		return s, errors.New("存档版本不匹配")
	}
	if s.Players != 2 && s.Players != 4 && s.Players != 6 {
		return s, errors.New("人数无效")
	}
	if s.TeamSize != s.Players/2 || len(s.Tanks) != s.Players {
		return s, errors.New("队伍或坦克数量无效")
	}
	mdata, ok := maps.ByID(s.MapID)
	if !ok {
		return s, errors.New("地图无效")
	}
	s.cells = mdata.Cells
	if len(s.Broken) != SubCols*SubRows {
		return s, errors.New("地形状态长度无效")
	}
	if s.Epoch == "" || s.Phase == "" {
		return s, errors.New("存档缺少 epoch/阶段")
	}
	if s.Phase != PhaseSelect && s.Phase != PhaseCountdown && s.Phase != PhasePlaying && s.Phase != PhaseFinished {
		return s, errors.New("阶段无效")
	}
	if len(s.Tanks) != s.Players || len(s.Bases) != 2 || len(s.Bullets) > MaxBullets || len(s.PowerUps) > MaxPowerUps || len(s.Events) > MaxEvents {
		return s, errors.New("实体数量超出上限")
	}
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if t.Seat != i || t.Team != teamOf(i, s.TeamSize) {
			return s, errors.New("坦克席位/队伍无效")
		}
		if t.Class != "" && !ValidTank(t.Class) {
			return s, errors.New("坦克类型无效")
		}
		if t.Dir < 0 || t.Dir > 3 || t.X < 0 || t.Y < 0 || t.X > Cols*Cell-Cell || t.Y > Rows*Cell-Cell {
			return s, errors.New("坦克位置/方向无效")
		}
		if t.HP < 0 || t.MaxHP < 0 || t.HP > t.MaxHP || t.MaxHP > 4 || t.Tech < 0 || t.Tech > 3 || len(t.Pending) > MaxPending {
			return s, errors.New("坦克数值无效")
		}
		if t.Seq < 0 || t.Seq > MaxSeq {
			return s, errors.New("输入序号无效")
		}
	}
	for _, b := range s.Bullets {
		if b.Seat < 0 || b.Seat >= s.Players || b.Team != teamOf(b.Seat, s.TeamSize) || b.Dir < 0 || b.Dir > 3 || b.Speed <= 0 || b.Speed > 600 {
			return s, errors.New("子弹无效")
		}
	}
	for _, p := range s.PowerUps {
		if !ValidPowerUp(p.Type) {
			return s, errors.New("道具类型无效")
		}
	}
	return s, nil
}

func teamOf(seat, teamSize int) int {
	if seat < teamSize {
		return TeamTop
	}
	return TeamBottom
}

func ValidPowerUp(t string) bool {
	switch t {
	case "star", "grenade", "helmet", "shovel", "tank", "clock":
		return true
	}
	return false
}

func (s *State) match() (gameapi.Match, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	m := gameapi.Match{Schema: 1, Data: data, Actors: []int{}, TimeoutActors: []int{}, Attention: []int{}, Log: append([]string{}, s.Log...), Result: s.Result}
	if s.Result != nil {
		return m, nil
	}
	switch s.Phase {
	case PhaseSelect:
		for i := range s.Tanks {
			if !s.Tanks[i].Selected {
				m.Actors = append(m.Actors, i)
				m.Attention = append(m.Attention, i)
			}
		}
		m.SystemDeadline = s.SelectUntil
		if next := s.ServerAt + FrameMS; next < m.SystemDeadline {
			m.SystemDeadline = next
		}
	case PhaseCountdown:
		m.SystemDeadline = s.StartAt
	case PhasePlaying:
		for i := range s.Tanks {
			m.Actors = append(m.Actors, i)
			if m.Deadline == 0 || s.Tanks[i].IdleAt < m.Deadline {
				m.Deadline = s.Tanks[i].IdleAt
			}
		}
		m.Attention = []int{}
		for i := range s.Tanks {
			if s.Tanks[i].IdleAt == m.Deadline {
				m.TimeoutActors = append(m.TimeoutActors, i)
			}
		}
		m.SystemDeadline = s.ServerAt + FrameMS
	}
	return m, nil
}

func (s *State) tank(seat int) *Tank {
	if seat < 0 || seat >= len(s.Tanks) {
		return nil
	}
	return &s.Tanks[seat]
}

func (s *State) spec(class string) TankSpec { return tankSpecs[class] }

func (s *State) maxHP(t *Tank) int {
	if t.Class == "" {
		return 1
	}
	return tankSpecs[t.Class].HP
}

func (s *State) maxBullets(t *Tank) int {
	n := s.spec(t.Class).MaxBullets
	if t.Tech >= 2 {
		n++
	}
	if n > 4 {
		n = 4
	}
	return n
}

func (s *State) bulletSpeed(t *Tank) float64 {
	speed := s.spec(t.Class).BulletSpeed
	if t.Tech >= 1 {
		speed *= 1.25
	}
	return speed
}

func (s *State) activeBullets(seat int) int {
	n := 0
	for _, b := range s.Bullets {
		if b.Seat == seat {
			n++
		}
	}
	return n
}

func eventSort(events []Event) {
	sort.SliceStable(events, func(i, j int) bool { return events[i].ID < events[j].ID })
}

func (s *State) clampPos(x, y float64) (float64, float64) {
	return math.Max(0, math.Min(x, float64(Cols*Cell-Cell))), math.Max(0, math.Min(y, float64(Rows*Cell-Cell)))
}

func (s *State) finish(draw bool, winner int, reason string) {
	if s.Result != nil {
		return
	}
	s.Phase = PhaseFinished
	r := &gameapi.Result{Team: true, Ranks: make([]int, len(s.Tanks)), Winners: []int{}, Scores: make([]int, len(s.Tanks))}
	for i := range s.Tanks {
		r.Scores[i] = s.Tanks[i].Kills
	}
	if draw {
		r.Draw = true
		for i := range r.Ranks {
			r.Ranks[i] = 1
		}
	} else {
		for i := range s.Tanks {
			if s.Tanks[i].Team == winner {
				r.Ranks[i] = 1
				r.Winners = append(r.Winners, i)
			} else {
				r.Ranks[i] = 2
			}
		}
	}
	details := map[string]any{
		"map": s.MapID, "reason": reason, "teamSize": s.TeamSize,
		"bases": s.Bases, "tanks": s.Tanks, "winner": winner, "draw": draw,
	}
	r.Details, _ = json.Marshal(details)
	s.Result = r
	s.emit("finish", -1, -1, s.ServerAt, 0, 0, 0, reason, 0)
	s.log("对局结束：" + reason)
}

func (s *State) project(seat int) View {
	v := View{
		Options: s.Options, Phase: s.Phase, Epoch: s.Epoch, ServerAt: s.ServerAt,
		SelectUntil: s.SelectUntil, StartAt: s.StartAt, MatchEndsAt: s.MatchEndsAt,
		TeamSize: s.TeamSize, MapID: s.MapID, Broken: append([]byte(nil), s.Broken...),
		ShovelUntil: s.ShovelUntil, Events: append([]Event(nil), s.Events...),
	}
	for i := range s.Tanks {
		t := &s.Tanks[i]
		v.Tanks = append(v.Tanks, TankView{
			Seat: t.Seat, Team: t.Team, Class: t.Class, Selected: t.Selected,
			X: t.X, Y: t.Y, Dir: t.Dir, HP: t.HP, MaxHP: t.MaxHP, Tech: t.Tech,
			Alive: t.Alive, RespawnAt: t.RespawnAt, ShieldUntil: t.ShieldUntil,
			FreezeUntil: t.FreezeUntil, Kills: t.Kills, Deaths: t.Deaths, Seq: t.Seq,
		})
	}
	for _, b := range s.Bullets {
		v.Bullets = append(v.Bullets, BulletView{ID: b.ID, Seat: b.Seat, Team: b.Team, X: b.X, Y: b.Y, Dir: b.Dir})
	}
	for _, p := range s.PowerUps {
		v.PowerUps = append(v.PowerUps, PowerUpView{ID: p.ID, Type: p.Type, X: p.X, Y: p.Y, ExpireAt: p.ExpireAt})
	}
	for i := range s.Bases {
		v.Bases[i] = BaseView{Team: s.Bases[i].Team, Alive: s.Bases[i].Alive, DestroyedAt: s.Bases[i].DestroyedAt, DestroyedBy: s.Bases[i].DestroyedBy}
	}
	if seat >= 0 && seat < len(s.Tanks) && s.Result == nil {
		if s.Phase == PhaseSelect {
			v.Controls.CanSelect = !s.Tanks[seat].Selected
		}
		if s.Phase == PhasePlaying {
			v.Controls.CanPlay = true
		}
	}
	return v
}

var _ = fmt.Sprintf
var _ = eventSort
