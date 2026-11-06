// Package engine owns the deterministic, platform-independent PvZ versus rules.
package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
)

// TickMS is the versus simulation clock. It is not an assertion about the
// original binary's display rate. Verified original counters are converted by
// the rule profile documented in docs/research/pvz.
const TickMS = 50
const Rows = 5
const Columns = 9
const MaxZombies = 80
const MaxProjectiles = 256
const MaxEvents = 128
const MaxMoney = 1000000

type Options struct {
	Mode         string `json:"mode"`
	Teams        []int  `json:"teams"`
	RoundSeconds int    `json:"roundSeconds"`
	PrepSeconds  int    `json:"prepSeconds"`
}
type Action struct {
	Type    string `json:"type"`
	Epoch   string `json:"epoch"`
	Seq     int64  `json:"seq"`
	Kind    string `json:"kind,omitempty"`
	Row     int    `json:"row,omitempty"`
	Col     int    `json:"col,omitempty"`
	PlantID int    `json:"plantId,omitempty"`
}
type Player struct {
	Team      int              `json:"team"`
	Side      string           `json:"side"`
	Money     int              `json:"money"`
	Cooldowns map[string]int64 `json:"cooldowns"`
	Seq       int64            `json:"seq"`
}
type Plant struct {
	ID       int    `json:"id"`
	Owner    int    `json:"owner"`
	Kind     string `json:"kind"`
	Row      int    `json:"row"`
	Col      int    `json:"col"`
	HP       int    `json:"hp"`
	MaxHP    int    `json:"maxHP"`
	ReadyAt  int64  `json:"readyAt"`
	ReadyMS  int64  `json:"readyMS,omitempty"`
	AttackMS int64  `json:"attackMS,omitempty"`
	AttackAt int64  `json:"attackAt"`
	BurstAt  int64  `json:"burstAt,omitempty"`
	BornAt   int64  `json:"bornAt"`
}
type Zombie struct {
	ID                int    `json:"id"`
	Owner             int    `json:"owner"`
	Kind              string `json:"kind"`
	Row               int    `json:"row"`
	X                 int    `json:"x"`
	HP                int    `json:"hp"`
	MaxHP             int    `json:"maxHP"`
	Armor             int    `json:"armor"`
	MaxArmor          int    `json:"maxArmor"`
	SlowUntil         int64  `json:"slowUntil"`
	State             string `json:"state"`
	Vaulted           bool   `json:"vaulted"`
	VaultUntil        int64  `json:"vaultUntil,omitempty"`
	VaultX            int    `json:"vaultX,omitempty"`
	BiteAt            int64  `json:"biteAt"`
	BornAt            int64  `json:"bornAt"`
	MoveRemainder     int    `json:"moveRemainder,omitempty"`
	SpeedMilli        int    `json:"speedMilli"`
	SpecialSpeedMilli int    `json:"specialSpeedMilli,omitempty"`
	BiteCarryMS       int    `json:"biteCarryMS,omitempty"`
}
type Projectile struct {
	ID     int    `json:"id"`
	Owner  int    `json:"owner"`
	Row    int    `json:"row"`
	X      int    `json:"x"`
	Kind   string `json:"kind"`
	Damage int    `json:"damage"`
}
type Mower struct {
	Row   int    `json:"row"`
	State string `json:"state"`
	X     int    `json:"x"`
}
type Event struct {
	ID     int64  `json:"id"`
	Type   string `json:"type"`
	Tick   int64  `json:"tick"`
	Row    int    `json:"row"`
	X      int    `json:"x"`
	Owner  int    `json:"owner"`
	Kind   string `json:"kind,omitempty"`
	Target int    `json:"target,omitempty"`
	Amount int    `json:"amount,omitempty"`
}
type RoundResult struct {
	AttackTeam int   `json:"attackTeam"`
	Breached   bool  `json:"breached"`
	ElapsedMS  int64 `json:"elapsedMS"`
}
type State struct {
	Options         Options             `json:"options"`
	InitialRandom   uint32              `json:"initialRandom"`
	RandomState     uint32              `json:"randomState"`
	BaseEpoch       string              `json:"baseEpoch"`
	Epoch           string              `json:"epoch"`
	Round           int                 `json:"round"`
	Phase           string              `json:"phase"`
	Tick            int64               `json:"tick"`
	PhaseEndTick    int64               `json:"phaseEndTick"`
	BattleStartTick int64               `json:"battleStartTick"`
	Players         []Player            `json:"players"`
	Plants          []Plant             `json:"plants"`
	Zombies         []Zombie            `json:"zombies"`
	Projectiles     []Projectile        `json:"projectiles"`
	Mowers          []Mower             `json:"mowers"`
	Events          []Event             `json:"events"`
	Rounds          []RoundResult       `json:"rounds"`
	WinnerTeam      int                 `json:"winnerTeam"`
	NextID          int                 `json:"nextId"`
	EventSerial     int64               `json:"eventSerial"`
	NextSunTick     int64               `json:"nextSunTick"`
	NextBrainTick   int64               `json:"nextBrainTick"`
	IncomeCursor    [2]int              `json:"incomeCursor"`
	TeamCooldowns   [2]map[string]int64 `json:"teamCooldowns"`
	LaneCooldowns   [5]int64            `json:"laneCooldowns"`
}
type Outcome struct {
	WinnerTeam int           `json:"winnerTeam"`
	Draw       bool          `json:"draw"`
	Rounds     []RoundResult `json:"rounds"`
}

func NormalizeOptions(o Options, n int) (Options, error) {
	if n != 2 && n != 4 {
		return o, errors.New("植物大战僵尸支持 2 人或 4 人")
	}
	if o.Mode == "" {
		o.Mode = "duel"
		if n == 4 {
			o.Mode = "team"
		}
	}
	if o.RoundSeconds == 0 {
		o.RoundSeconds = 360
	}
	if o.PrepSeconds == 0 {
		o.PrepSeconds = 20
	}
	if o.Mode != "duel" && o.Mode != "team" || o.Mode == "duel" && n != 2 || o.Mode == "team" && n != 4 {
		return o, errors.New("1v1 需要 2 人，2v2 需要 4 人")
	}
	if !slices.Contains([]int{180, 360, 600}, o.RoundSeconds) || !slices.Contains([]int{10, 20, 30}, o.PrepSeconds) {
		return o, errors.New("请选择有效的攻防与布阵时间")
	}
	if len(o.Teams) == 0 {
		o.Teams = make([]int, n)
		for p := range n {
			o.Teams[p] = p % 2
		}
	} else {
		o.Teams = slices.Clone(o.Teams)
	}
	if len(o.Teams) != n {
		return o, errors.New("分队人数与房间不符")
	}
	count := [2]int{}
	for _, team := range o.Teams {
		if team < 0 || team > 1 {
			return o, errors.New("无效阵营")
		}
		count[team]++
	}
	if count[0] != n/2 || count[1] != n/2 {
		return o, errors.New("两队人数必须相等")
	}
	return o, nil
}
func New(o Options, n int, epoch string) (State, error) {
	o, e := NormalizeOptions(o, n)
	if e != nil {
		return State{}, e
	}
	if epoch == "" || len(epoch) > 128 {
		return State{}, errors.New("无效对局标识")
	}
	sum := sha256.Sum256([]byte(epoch))
	seed := binary.LittleEndian.Uint32(sum[:4])
	if seed == 0 {
		seed = 0x9e3779b9
	}
	s := State{Options: o, InitialRandom: seed, RandomState: seed, BaseEpoch: epoch, Round: 1, WinnerTeam: -2, Players: make([]Player, n), Rounds: []RoundResult{}, Events: []Event{}}
	s.beginRound()
	return s, nil
}
func (s *State) beginRound() {
	s.RandomState = s.InitialRandom
	s.Epoch = s.BaseEpoch + ":" + strconv.Itoa(s.Round)
	s.Phase = "prepare"
	s.PhaseEndTick = s.Tick + int64(s.Options.PrepSeconds*1000/TickMS)
	s.BattleStartTick = 0
	s.Plants = []Plant{}
	s.Zombies = []Zombie{}
	s.Projectiles = []Projectile{}
	s.Mowers = make([]Mower, Rows)
	for row := range Rows {
		s.Mowers[row] = Mower{Row: row, State: "ready", X: -450}
	}
	s.NextSunTick = s.Tick + 100
	s.NextBrainTick = 0
	s.IncomeCursor = [2]int{}
	s.TeamCooldowns = [2]map[string]int64{{}, {}}
	s.LaneCooldowns = [5]int64{}
	for p := range s.Players {
		side := "zombies"
		money := 250
		if s.Options.Teams[p] == (s.Round-1)%2 {
			side = "plants"
			money = 300
		}
		s.Players[p] = Player{Team: s.Options.Teams[p], Side: side, Money: money / (len(s.Players) / 2), Cooldowns: map[string]int64{}}
	}
	s.emit("round", 0, 0, -1, "", 0, s.Round)
}
func Clone(s State) State {
	out := s
	out.Options.Teams = slices.Clone(s.Options.Teams)
	out.Players = slices.Clone(s.Players)
	for p := range out.Players {
		out.Players[p].Cooldowns = cloneMap(s.Players[p].Cooldowns)
	}
	out.Plants = slices.Clone(s.Plants)
	out.Zombies = slices.Clone(s.Zombies)
	out.Projectiles = slices.Clone(s.Projectiles)
	out.Mowers = slices.Clone(s.Mowers)
	out.Events = slices.Clone(s.Events)
	out.Rounds = slices.Clone(s.Rounds)
	for i := range 2 {
		out.TeamCooldowns[i] = cloneMap(s.TeamCooldowns[i])
	}
	return out
}
func cloneMap(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
func Digest(s State) string {
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func Result(s State) *Outcome {
	if s.Phase != "finished" {
		return nil
	}
	return &Outcome{WinnerTeam: s.WinnerTeam, Draw: s.WinnerTeam < 0, Rounds: slices.Clone(s.Rounds)}
}
func (s *State) emit(kind string, row, x, owner int, unit string, target, amount int) {
	s.EventSerial++
	s.Events = append(s.Events, Event{ID: s.EventSerial, Type: kind, Tick: s.Tick, Row: row, X: x, Owner: owner, Kind: unit, Target: target, Amount: amount})
	if len(s.Events) > MaxEvents {
		s.Events = s.Events[len(s.Events)-MaxEvents:]
	}
}
func (s *State) id() int { s.NextID++; return s.NextID }
func PlantX(p Plant) int { return p.Col*1000 + 500 }

// random uses an explicit saved PRNG. Counter ranges are recovered from the PC
// game; this reproducible generator is the independent versus implementation.
func (s *State) random(n int) int {
	x := s.RandomState
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	s.RandomState = x
	return int(x % uint32(n))
}
func counterTick(ms int64) int64 { return (ms + TickMS - 1) / TickMS }
