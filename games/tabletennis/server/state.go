// Package tabletennis owns the ball simulation, umpire and computer strategy.
package tabletennis

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
)

const (
	HalfWidth            = .7625
	HalfLength           = 1.37
	TableHeight          = .76
	NetHeight            = .1525
	BallRadius           = .02
	FrameMS        int64 = 50
	LateGraceMS    int64 = 350
	EarlyBufferMS  int64 = 220
	ServeTimeoutMS int64 = 15000
)

type Options struct {
	BestOf     int    `json:"bestOf"`
	Teams      []int  `json:"teams"`
	Difficulty string `json:"difficulty"`
	Court      string `json:"court"`
}
type Vec struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
type Segment struct {
	At    int64 `json:"at"`
	Until int64 `json:"until"`
	P     Vec   `json:"p"`
	V     Vec   `json:"v"`
	A     Vec   `json:"a"`
}
type FlightEvent struct {
	Type string `json:"type"`
	At   int64  `json:"at"`
	P    Vec    `json:"p"`
	Side int    `json:"side"`
}
type Flight struct {
	Segments     []Segment     `json:"segments"`
	Events       []FlightEvent `json:"events"`
	Spin         float64       `json:"spin"`
	StartAt      int64         `json:"startAt"`
	EndAt        int64         `json:"endAt"`
	ReceiveAt    int64         `json:"receiveAt"`
	ContactStart int64         `json:"contactStart"`
	ContactEnd   int64         `json:"contactEnd"`
	Reason       string        `json:"reason"`
	WinnerSide   int           `json:"winnerSide"`
	TouchedNet   bool          `json:"touchedNet"`
	Serve        bool          `json:"serve"`
}
type Event struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	At    int64  `json:"at"`
	Seat  int    `json:"seat"`
	P     Vec    `json:"p"`
	Kind  string `json:"kind,omitempty"`
	Score [2]int `json:"score"`
}
type GameResult struct {
	Score  [2]int `json:"score"`
	Winner int    `json:"winner"`
}
type Paddle struct {
	X      float64 `json:"x"`
	Seq    int64   `json:"seq"`
	At     int64   `json:"at"`
	Rally  int     `json:"rally"`
	Shot   int     `json:"shot"`
	Result string  `json:"result"`
}
type Action struct {
	RacketX float64 `json:"racketX"`
	Type    string  `json:"type"`
	Epoch   string  `json:"epoch"`
	Rally   int     `json:"rally"`
	Shot    int     `json:"shot"`
	Seq     int64   `json:"seq"`
	Kind    string  `json:"kind"`
	Aim     float64 `json:"aim"`
	Power   float64 `json:"power"`
	At      int64   `json:"at"`
}
type PendingSwing struct {
	Action Action `json:"action"`
	Seat   int    `json:"seat"`
	At     int64  `json:"at"`
}
type State struct {
	Paddles       []Paddle        `json:"paddles"`
	Options       Options         `json:"options"`
	Epoch         string          `json:"epoch"`
	Phase         string          `json:"phase"`
	ServerAt      int64           `json:"serverAt"`
	PhaseUntil    int64           `json:"phaseUntil"`
	ServeDeadline int64           `json:"serveDeadline"`
	Score         [2]int          `json:"score"`
	GamesWon      [2]int          `json:"gamesWon"`
	Game          int             `json:"game"`
	Games         []GameResult    `json:"games"`
	Order         []int           `json:"order"`
	InitialOrder  []int           `json:"initialOrder"`
	Ends          int             `json:"ends"`
	EndsSwapped   bool            `json:"endsSwapped"`
	Rally         int             `json:"rally"`
	Shot          int             `json:"shot"`
	Expected      int             `json:"expected"`
	Receiver      int             `json:"receiver"`
	LastHitter    int             `json:"lastHitter"`
	PointWinner   int             `json:"pointWinner"`
	Seq           []int64         `json:"seq"`
	Flight        *Flight         `json:"flight,omitempty"`
	Pending       *PendingSwing   `json:"pending,omitempty"`
	FlightCursor  int             `json:"flightCursor"`
	Events        []Event         `json:"events"`
	EventID       int             `json:"eventId"`
	Log           []string        `json:"log"`
	Result        *gameapi.Result `json:"result,omitempty"`
}
type Controls struct {
	RetrySeq int64 `json:"retrySeq"`
	CanSwing bool  `json:"canSwing"`
	Armed    bool  `json:"armed"`
	Seq      int64 `json:"seq"`
	OpenAt   int64 `json:"openAt"`
	CloseAt  int64 `json:"closeAt"`
}
type View struct {
	Paddles       []Paddle     `json:"paddles"`
	Options       Options      `json:"options"`
	Epoch         string       `json:"epoch"`
	Phase         string       `json:"phase"`
	ServerAt      int64        `json:"serverAt"`
	PhaseUntil    int64        `json:"phaseUntil"`
	ServeDeadline int64        `json:"serveDeadline"`
	Score         [2]int       `json:"score"`
	GamesWon      [2]int       `json:"gamesWon"`
	Game          int          `json:"game"`
	Games         []GameResult `json:"games"`
	Order         []int        `json:"order"`
	Ends          int          `json:"ends"`
	Rally         int          `json:"rally"`
	Shot          int          `json:"shot"`
	Expected      int          `json:"expected"`
	Receiver      int          `json:"receiver"`
	LastHitter    int          `json:"lastHitter"`
	PointWinner   int          `json:"pointWinner"`
	Flight        *Flight      `json:"flight,omitempty"`
	Events        []Event      `json:"events"`
	Controls      Controls     `json:"controls"`
}

func finite(v float64) bool           { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func sign(side int) float64 {
	if side == 0 {
		return 1
	}
	return -1
}
func side(z float64) int {
	if z >= 0 {
		return 0
	}
	return 1
}
func (s *State) side(team int) int { return team ^ s.Ends }
func decode(m gameapi.Match) (State, error) {
	var s State
	if m.Schema != 1 {
		return s, errors.New("不支持的乒乓球存档版本")
	}
	if e := json.Unmarshal(m.Data, &s); e != nil {
		return s, e
	}
	n := len(s.Options.Teams)
	if (n != 2 && n != 4) || len(s.Order) != n || len(s.Seq) != n || s.Epoch == "" {
		return s, errors.New("乒乓球存档不完整")
	}
	if len(s.Paddles) == 0 {
		s.Paddles = initialPaddles(s.Options.Teams, s.ServerAt)
	}
	if len(s.Paddles) != n {
		return s, errors.New("球拍存档不完整")
	}
	return s, nil
}
func initialPaddles(teams []int, at int64) []Paddle {
	out := make([]Paddle, len(teams))
	count := [2]int{}
	for seat, team := range teams {
		x := 0.
		if len(teams) == 4 {
			x = .35
			if count[team] > 0 {
				x = -.35
			}
		}
		out[seat] = Paddle{X: x, At: at}
		count[team]++
	}
	return out
}
func (s *State) recordPaddle(a Action, seat int, at int64, result string) {
	s.Paddles[seat] = Paddle{X: a.RacketX, Seq: a.Seq, At: at, Rally: a.Rally, Shot: a.Shot, Result: result}
}
func (s *State) emit(kind string, at int64, seat int, p Vec, stroke string) {
	s.EventID++
	s.Events = append(s.Events, Event{ID: s.EventID, Type: kind, At: at, Seat: seat, P: p, Kind: stroke, Score: s.Score})
	if len(s.Events) > 64 {
		s.Events = s.Events[len(s.Events)-64:]
	}
}
func (s *State) match() (gameapi.Match, error) {
	b, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	m := gameapi.Match{Schema: 1, Data: b, Actors: []int{}, Attention: []int{}, TimeoutActors: []int{}, Result: s.Result, Log: append([]string{}, s.Log...), CheckpointKey: fmt.Sprintf("%s/%d/%d/%d", s.Phase, s.Game, s.Rally, s.Shot)}
	if s.Result != nil {
		return m, nil
	}
	for i := range s.Options.Teams {
		m.Actors = append(m.Actors, i)
	}
	m.SystemDeadline = s.ServerAt + FrameMS
	if s.Phase == "serve" && s.ServerAt <= s.ServeDeadline {
		m.Attention = []int{s.Expected}

	}
	return m, nil
}
func (s *State) service() (int, int) {
	total := s.Score[0] + s.Score[1]
	rotation := total / 2
	if s.Score[0] >= 10 && s.Score[1] >= 10 {
		rotation = 10 + total - 20
	}
	i := rotation % len(s.Order)
	return s.Order[i], s.Order[(i+1)%len(s.Order)]
}
func (s *State) nextSeat(seat int) int {
	for i, n := range s.Order {
		if n == seat {
			return s.Order[(i+1)%len(s.Order)]
		}
	}
	return -1
}
func (s *State) beginServe(now int64) {
	s.Phase = "serve"
	s.Expected, s.Receiver = s.service()
	s.ServeDeadline = now + ServeTimeoutMS
	s.PhaseUntil = now
	s.Flight = nil
	s.Pending = nil
	s.FlightCursor = 0
	s.LastHitter = -1
}
func (s *State) newGame(now int64) {
	s.Game++
	s.Score = [2]int{}
	s.Ends ^= 1
	s.EndsSwapped = false
	s.Rally++
	s.Shot = 0
	s.Flight = nil
	s.Pending = nil
	s.PointWinner = -1
	old := s.InitialOrder
	s.Order = make([]int, len(old))
	for i := range old {
		s.Order[i] = old[(1-i+len(old))%len(old)]
	}
	s.InitialOrder = append([]int{}, s.Order...)
	s.Phase = "countdown"
	s.PhaseUntil = now + 1800
	s.Expected, s.Receiver = s.service()
	s.emit("game", now, -1, Vec{}, "")
}
func (s *State) award(team int, reason string, now int64) {
	s.Score[team]++
	s.PointWinner = team
	s.Pending = nil
	s.emit("point", now, -1, Vec{}, reason)
	reasonName := map[string]string{"net": "触网", "wrong_side": "落在己方台", "wrong_service_half": "发球落点不正确", "out": "出界", "double_bounce": "二次落台", "miss": "漏接", "serve_timeout": "发球超时"}[reason]
	if reasonName != "" {
		s.Log = append(s.Log, fmt.Sprintf("%s，%s 队得分（%d:%d）", reasonName, []string{"A", "B"}[team], s.Score[0], s.Score[1]))
	}
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
	if s.Score[team] >= 11 && s.Score[team]-s.Score[1-team] >= 2 {
		s.GamesWon[team]++
		s.Games = append(s.Games, GameResult{s.Score, team})
		s.Log = append(s.Log, fmt.Sprintf("第 %d 局：%d:%d", s.Game, s.Score[0], s.Score[1]))
		if s.GamesWon[team] > s.Options.BestOf/2 {
			s.Phase = "finished"
			r := &gameapi.Result{Ranks: make([]int, len(s.Seq)), Winners: []int{}, Scores: make([]int, len(s.Seq)), Team: len(s.Seq) == 4}
			for i, t := range s.Options.Teams {
				r.Scores[i] = s.GamesWon[t]
				if t == team {
					r.Ranks[i] = 1
					r.Winners = append(r.Winners, i)
				} else {
					r.Ranks[i] = len(s.Seq)/2 + 1
				}
			}
			r.Details, _ = json.Marshal(map[string]any{"games": s.Games, "teams": s.Options.Teams, "gamesWon": s.GamesWon})
			s.Result = r
			s.emit("match", now, -1, Vec{}, "")
			return
		}
		s.Phase = "game_break"
		s.PhaseUntil = now + 3000
		return
	}
	if s.Game == s.Options.BestOf && !s.EndsSwapped && (s.Score[0] >= 5 || s.Score[1] >= 5) {
		s.Ends ^= 1
		s.EndsSwapped = true
		if len(s.Order) == 4 {
			_, receiver := s.service()
			team := s.Options.Teams[receiver]
			indices := []int{}
			for i, seat := range s.Order {
				if s.Options.Teams[seat] == team {
					indices = append(indices, i)
				}
			}
			s.Order[indices[0]], s.Order[indices[1]] = s.Order[indices[1]], s.Order[indices[0]]
		}
		s.emit("ends", now, -1, Vec{}, "")
	}
	s.Phase = "point_break"
	s.PhaseUntil = now + 1000
}
