package tetris

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const (
	SchemaVersion = 1

	BoardWidth  = 10
	HiddenRows  = 2
	VisibleRows = 20
	BoardRows   = HiddenRows + VisibleRows

	FrameMS           int64 = 100
	CountdownMS       int64 = 3000
	IdleMS            int64 = 20000
	MatchCapMS        int64 = 10 * 60 * 1000
	LockDelayMS       int64 = 500
	MaxLockResets           = 15
	MaxBatchEvents          = 16
	MaxActionBytes          = 8192
	MaxSeqJump        int64 = 1024
	MaxSeq            int64 = 1<<31 - 1
	MaxCatchUp              = 20
	MaxPendingGarbage       = 20
	SprintTarget            = 40
)

const (
	ModeSurvival = "survival"
	ModeSprint   = "sprint"
)

type Kind int

const (
	Empty   Kind = 0
	I       Kind = 1
	J       Kind = 2
	L       Kind = 3
	O       Kind = 4
	S       Kind = 5
	T       Kind = 6
	Z       Kind = 7
	Garbage Kind = 8
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Options struct {
	Mode string `json:"mode"`
}

type Piece struct {
	ID        int64 `json:"id"`
	Kind      Kind  `json:"kind"`
	Rot       int   `json:"rot"`
	X         int   `json:"x"`
	Y         int   `json:"y"`
	SpawnedAt int64 `json:"spawnedAt"`
}

type Player struct {
	Seat         int               `json:"seat"`
	Alive        bool              `json:"alive"`
	EliminatedAt int64             `json:"eliminatedAt,omitempty"`
	Board        [BoardRows]string `json:"board"`
	Active       *Piece            `json:"active,omitempty"`
	Queue        []Kind            `json:"queue"`
	Bag          []Kind            `json:"bag,omitempty"`
	Rand         uint64            `json:"rand"`
	Hold         Kind              `json:"hold,omitempty"`
	HoldUsed     bool              `json:"holdUsed,omitempty"`
	Lines        int               `json:"lines"`
	Score        int               `json:"score"`
	Level        int               `json:"level"`
	Pending      []int             `json:"pending,omitempty"`
	TargetSeat   int               `json:"targetSeat"`
	Seq          int64             `json:"seq"`
	IdleAt       int64             `json:"idleAt"`
	LastInputAt  int64             `json:"lastInputAt,omitempty"`
	FallAt       int64             `json:"fallAt"`
	LockAt       int64             `json:"lockAt,omitempty"`
	LockResets   int               `json:"lockResets,omitempty"`
	PieceID      int64             `json:"pieceId,omitempty"`
}

type Event struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	At      int64  `json:"at"`
	Seat    int    `json:"seat"`
	Rows    []int  `json:"rows,omitempty"`
	Count   int    `json:"count,omitempty"`
	Target  int    `json:"target,omitempty"`
	Level   int    `json:"level,omitempty"`
	Kind    Kind   `json:"kind,omitempty"`
	PieceID int64  `json:"pieceId,omitempty"`
	Applied bool   `json:"applied,omitempty"`
}

type State struct {
	Options     Options         `json:"options"`
	Phase       string          `json:"phase"`
	Epoch       string          `json:"epoch"`
	Seed        uint64          `json:"seed"`
	ServerAt    int64           `json:"serverAt"`
	StartAt     int64           `json:"startAt"`
	MatchEndsAt int64           `json:"matchEndsAt"`
	Players     []Player        `json:"players"`
	Events      []Event         `json:"events"`
	EventID     int             `json:"eventId"`
	Log         []string        `json:"log"`
	Result      *gameapi.Result `json:"result,omitempty"`
}

type PieceView struct {
	ID     int64   `json:"id"`
	Kind   Kind    `json:"kind"`
	Rot    int     `json:"rot"`
	X      int     `json:"x"`
	Y      int     `json:"y"`
	Cells  []Point `json:"cells"`
	GhostY *int    `json:"ghostY,omitempty"`
}

type PlayerView struct {
	Seat         int        `json:"seat"`
	Alive        bool       `json:"alive"`
	EliminatedAt int64      `json:"eliminatedAt,omitempty"`
	Board        []string   `json:"board"`
	Active       *PieceView `json:"active,omitempty"`
	Lines        int        `json:"lines"`
	Score        int        `json:"score"`
	Level        int        `json:"level"`
	PendingCount int        `json:"pendingCount"`
	TargetSeat   int        `json:"targetSeat"`
	Queue        []Kind     `json:"queue,omitempty"`
	Hold         Kind       `json:"hold,omitempty"`
	CanHold      bool       `json:"canHold,omitempty"`
	Seq          int64      `json:"seq,omitempty"`
}

type Controls struct {
	CanPlay bool `json:"canPlay"`
	CanHold bool `json:"canHold"`
}

type View struct {
	Options     Options      `json:"options"`
	Phase       string       `json:"phase"`
	Epoch       string       `json:"epoch"`
	ServerAt    int64        `json:"serverAt"`
	StartAt     int64        `json:"startAt"`
	MatchEndsAt int64        `json:"matchEndsAt"`
	Players     []PlayerView `json:"players"`
	Events      []Event      `json:"events"`
	Controls    Controls     `json:"controls"`
}

func decode(m gameapi.Match) (*State, error) {
	if m.Schema != SchemaVersion {
		return nil, errors.New("不支持的俄罗斯方块存档版本")
	}
	var s State
	if err := json.Unmarshal(m.Data, &s); err != nil {
		return nil, err
	}
	if len(s.Players) < 1 || len(s.Players) > 4 || s.Epoch == "" {
		return nil, errors.New("俄罗斯方块存档不完整")
	}
	switch s.Phase {
	case "countdown", "playing", "finished":
	default:
		return nil, errors.New("俄罗斯方块存档阶段无效")
	}
	if s.Options.Mode != ModeSurvival && s.Options.Mode != ModeSprint {
		return nil, errors.New("俄罗斯方块模式无效")
	}
	for i := range s.Players {
		p := &s.Players[i]
		p.Seat = i
		if len(p.Board) != BoardRows {
			return nil, fmt.Errorf("玩家 %d 棋盘行数无效", i)
		}
		for _, row := range p.Board {
			if len(row) != BoardWidth || strings.ContainsAny(row, "9") {
				return nil, fmt.Errorf("玩家 %d 棋盘内容无效", i)
			}
			for _, c := range row {
				if c < '0' || c > '9' {
					return nil, fmt.Errorf("玩家 %d 棋盘内容无效", i)
				}
			}
		}
		if len(p.Queue) != 5 || len(p.Bag) > 7 {
			return nil, fmt.Errorf("玩家 %d 队列无效", i)
		}
		for _, k := range append(append([]Kind{}, p.Queue...), p.Bag...) {
			if k < I || k > Z {
				return nil, fmt.Errorf("玩家 %d 方块类型无效", i)
			}
		}
		if p.Hold != Empty && (p.Hold < I || p.Hold > Z) {
			return nil, fmt.Errorf("玩家 %d 暂存方块无效", i)
		}
		if p.Active != nil {
			a := p.Active
			if a.Kind < I || a.Kind > Z || a.Rot < 0 || a.Rot > 3 || a.X < -2 || a.X > BoardWidth+2 || a.Y < 0 || a.Y >= BoardRows {
				return nil, fmt.Errorf("玩家 %d 活动方块无效", i)
			}
		}
		if p.Seq < 0 || p.Seq > MaxSeq || p.IdleAt < 0 || p.FallAt < 0 || p.LockAt < 0 {
			return nil, fmt.Errorf("玩家 %d 计时或序号无效", i)
		}
		if len(p.Pending) > MaxPendingGarbage {
			return nil, fmt.Errorf("玩家 %d 待落垃圾过多", i)
		}
		for _, hole := range p.Pending {
			if hole < 0 || hole >= BoardWidth {
				return nil, fmt.Errorf("玩家 %d 垃圾洞列无效", i)
			}
		}
		if p.Level < 1 || p.Lines < 0 || p.Score < 0 {
			return nil, fmt.Errorf("玩家 %d 分数无效", i)
		}
	}
	if m.Result != nil {
		s.Result = m.Result
	}
	return &s, nil
}

func (s *State) envelope() (gameapi.Match, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	m := gameapi.Match{
		Schema:    SchemaVersion,
		Data:      data,
		Actors:    []int{},
		Attention: []int{},
		Log:       append([]string{}, s.Log...),
		Result:    s.Result,
	}
	for i := range s.Players {
		if !s.Players[i].Alive {
			m.Eliminated = append(m.Eliminated, i)
		}
	}
	if s.Result != nil {
		return m, nil
	}
	switch s.Phase {
	case "countdown":
		m.SystemDeadline = s.StartAt
	case "playing":
		deadline := int64(0)
		for i := range s.Players {
			p := &s.Players[i]
			if !p.Alive {
				continue
			}
			m.Actors = append(m.Actors, i)
			if deadline == 0 || p.IdleAt < deadline {
				deadline = p.IdleAt
			}
		}
		if deadline > 0 {
			m.Deadline = deadline
			for i := range s.Players {
				p := &s.Players[i]
				if p.Alive && p.IdleAt == deadline {
					m.TimeoutActors = append(m.TimeoutActors, i)
				}
			}
		}
		m.SystemDeadline = s.ServerAt + FrameMS
	default:
		return m, errors.New("俄罗斯方块阶段无法封装")
	}
	return m, nil
}

func (s *State) emit(kind string, seat int, at int64) Event {
	s.EventID++
	e := Event{ID: s.EventID, Type: kind, Seat: seat, At: at}
	s.Events = append(s.Events, e)
	if len(s.Events) > 64 {
		s.Events = s.Events[len(s.Events)-64:]
	}
	return e
}

func (s *State) emitEvent(e Event) {
	s.EventID++
	e.ID = s.EventID
	s.Events = append(s.Events, e)
	if len(s.Events) > 64 {
		s.Events = s.Events[len(s.Events)-64:]
	}
}

func (s *State) log(message string) {
	s.Log = append(s.Log, message)
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
}

type rankMetric struct {
	seat         int
	alive        bool
	lines        int
	score        int
	eliminatedAt int64
}

func metricOf(p Player) rankMetric {
	return rankMetric{seat: p.Seat, alive: p.Alive, lines: p.Lines, score: p.Score, eliminatedAt: p.EliminatedAt}
}

func metricEqual(a, b rankMetric, mode string) bool {
	if mode == ModeSurvival {
		return a.alive == b.alive && a.lines == b.lines && a.score == b.score && a.eliminatedAt == b.eliminatedAt
	}
	return a.lines == b.lines && a.score == b.score && a.alive == b.alive && a.eliminatedAt == b.eliminatedAt
}

func metricLess(a, b rankMetric, mode string) bool {
	if mode == ModeSurvival {
		if a.alive != b.alive {
			return a.alive
		}
		if a.alive {
			if a.lines != b.lines {
				return a.lines > b.lines
			}
			if a.score != b.score {
				return a.score > b.score
			}
			return a.seat < b.seat
		}
		if a.eliminatedAt != b.eliminatedAt {
			return a.eliminatedAt > b.eliminatedAt
		}
		if a.lines != b.lines {
			return a.lines > b.lines
		}
		if a.score != b.score {
			return a.score > b.score
		}
		return a.seat < b.seat
	}
	if a.lines != b.lines {
		return a.lines > b.lines
	}
	if a.score != b.score {
		return a.score > b.score
	}
	if a.alive != b.alive {
		return a.alive
	}
	if a.eliminatedAt != b.eliminatedAt {
		return a.eliminatedAt > b.eliminatedAt
	}
	return a.seat < b.seat
}

func rankPlayers(s *State) ([]int, []int, bool) {
	n := len(s.Players)
	metrics := make([]rankMetric, n)
	for i, p := range s.Players {
		metrics[i] = metricOf(p)
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return metricLess(metrics[order[i]], metrics[order[j]], s.Options.Mode)
	})
	ranks := make([]int, n)
	for i, seat := range order {
		if i == 0 {
			ranks[seat] = 1
			continue
		}
		prev, cur := metrics[order[i-1]], metrics[seat]
		if metricEqual(prev, cur, s.Options.Mode) {
			ranks[seat] = ranks[order[i-1]]
		} else {
			ranks[seat] = i + 1
		}
	}
	winners := []int{}
	for seat, rank := range ranks {
		if rank == 1 {
			winners = append(winners, seat)
		}
	}
	draw := n >= 2 && len(winners) == n
	if draw {
		for i := range ranks {
			ranks[i] = 1
		}
		winners = []int{}
	}
	return ranks, winners, draw
}

// resultFor builds the public result without mutating the phase. It is used by
// finish() and by rule tests that inspect ranking directly.
func resultFor(s *State, now int64, reason string) *gameapi.Result {
	ranks, winners, draw := rankPlayers(s)
	scores := make([]int, len(s.Players))
	players := make([]map[string]any, len(s.Players))
	for i, p := range s.Players {
		scores[i] = p.Score
		players[i] = map[string]any{
			"seat": p.Seat, "lines": p.Lines, "score": p.Score, "level": p.Level,
			"alive": p.Alive, "eliminatedAt": p.EliminatedAt,
		}
	}
	details, _ := json.Marshal(map[string]any{
		"mode": s.Options.Mode, "reason": reason, "players": players,
		"practice": len(s.Players) == 1,
	})
	r := &gameapi.Result{Ranks: ranks, Winners: winners, Scores: scores, Details: details}
	if len(s.Players) == 1 {
		r.Ranks = []int{1}
		r.Winners = []int{0}
		r.Unrated = true
		r.Details = details
	}
	if draw {
		r.Draw = true
		r.Winners = []int{}
		for i := range r.Ranks {
			r.Ranks[i] = 1
		}
	}
	return r
}

func aliveCount(s *State) int {
	n := 0
	for i := range s.Players {
		if s.Players[i].Alive {
			n++
		}
	}
	return n
}

func finish(s *State, now int64, reason string) {
	if s.Result != nil {
		return
	}
	s.Result = resultFor(s, now, reason)
	s.Phase = "finished"
	s.ServerAt = now
	for i := range s.Players {
		s.Players[i].Active = nil
		s.Players[i].Pending = nil
	}
	winner := -1
	if len(s.Result.Winners) > 0 {
		winner = s.Result.Winners[0]
	}
	e := s.emit("finish", winner, now)
	_ = e
	s.log(fmt.Sprintf("俄罗斯方块对局结束（%s）", reason))
}
