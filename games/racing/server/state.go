// Package racing owns the complete pure racing rules, vehicle simulation and AI.
package racing

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"sort"
)

const maxInputSequence int64 = 1<<31 - 1
const maxSequenceJump int64 = 1024
const frameMS int64 = 100
const stepMS int64 = 20
const CarRadius = 13.0
const MaxSpeed = 310.0
const idleMS int64 = 15000
const lostInputMS int64 = 750

type Options struct {
	Track string `json:"track"`
	Laps  int    `json:"laps"`
}
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Scenery struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Height float64 `json:"height"`
	Kind   string  `json:"kind"`
}
type Track struct {
	Props     []Scenery `json:"props,omitempty"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Width     float64   `json:"width"`
	Height    float64   `json:"height"`
	RoadWidth float64   `json:"roadWidth"`
	Points    []Point   `json:"points"`
}
type Input struct {
	At       int64   `json:"at"`
	Steer    float64 `json:"steer"`
	Throttle float64 `json:"throttle"`
	Brake    bool    `json:"brake"`
	Drift    bool    `json:"drift"`
}
type Car struct {
	Seat        int     `json:"seat"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	VX          float64 `json:"vx"`
	VY          float64 `json:"vy"`
	Angle       float64 `json:"angle"`
	Speed       float64 `json:"speed"`
	Steer       float64 `json:"steer"`
	Throttle    float64 `json:"throttle"`
	Brake       bool    `json:"brake"`
	Drift       bool    `json:"drift"`
	Skin        int     `json:"skin"`
	Seq         int64   `json:"seq"`
	Lap         int     `json:"lap"`
	Checkpoint  int     `json:"checkpoint"`
	Progress    float64 `json:"progress"`
	Rank        int     `json:"rank"`
	FinishAt    int64   `json:"finishAt"`
	Offroad     bool    `json:"offroad"`
	LastInputAt int64   `json:"lastInputAt"`
	IdleAt      int64   `json:"idleAt"`
	ResetUntil  int64   `json:"resetUntil"`
	Gate        int     `json:"gate"`
	LastReset   int64   `json:"lastReset"`
	Applied     Input   `json:"applied"`
	Pending     []Input `json:"pending,omitempty"`
}
type Event struct {
	ID        int     `json:"id"`
	Type      string  `json:"type"`
	At        int64   `json:"at"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Seat      int     `json:"seat"`
	Intensity float64 `json:"intensity"`
}
type Controls struct {
	CanDrive bool `json:"canDrive"`
	CanReset bool `json:"canReset"`
	CanSkin  bool `json:"canSkin"`
}
type View struct {
	Options        Options  `json:"options"`
	Phase          string   `json:"phase"`
	Epoch          string   `json:"epoch"`
	ServerAt       int64    `json:"serverAt"`
	StartAt        int64    `json:"startAt"`
	FinishDeadline int64    `json:"finishDeadline"`
	MaxRaceAt      int64    `json:"maxRaceAt"`
	Track          Track    `json:"track"`
	Cars           []Car    `json:"cars"`
	Events         []Event  `json:"events"`
	Controls       Controls `json:"controls"`
}
type State struct {
	Options        Options         `json:"options"`
	Phase          string          `json:"phase"`
	Epoch          string          `json:"epoch"`
	ServerAt       int64           `json:"serverAt"`
	StartAt        int64           `json:"startAt"`
	FinishDeadline int64           `json:"finishDeadline"`
	MaxRaceAt      int64           `json:"maxRaceAt"`
	Track          Track           `json:"track"`
	Cars           []Car           `json:"cars"`
	Events         []Event         `json:"events"`
	EventID        int             `json:"eventId"`
	Log            []string        `json:"log"`
	Result         *gameapi.Result `json:"result,omitempty"`
}

func finite(v float64) bool           { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func angle(v float64) float64         { return math.Atan2(math.Sin(v), math.Cos(v)) }
func dist(a, b Point) float64         { return math.Hypot(a.X-b.X, a.Y-b.Y) }
func decode(m gameapi.Match) (State, error) {
	var s State
	if m.Schema != 1 {
		return s, errors.New("不支持的赛车存档版本")
	}
	if e := json.Unmarshal(m.Data, &s); e != nil {
		return s, e
	}
	if len(s.Cars) < 2 || len(s.Cars) > 6 || len(s.Track.Points) < 16 || s.Epoch == "" {
		return s, errors.New("赛车存档不完整")
	}
	return s, nil
}
func (s *State) emit(kind string, seat int, at int64, intensity float64) {
	s.EventID++
	e := Event{ID: s.EventID, Type: kind, Seat: seat, At: at, Intensity: intensity}
	if seat >= 0 && seat < len(s.Cars) {
		e.X = s.Cars[seat].X
		e.Y = s.Cars[seat].Y
	}
	s.Events = append(s.Events, e)
	if len(s.Events) > 24 {
		s.Events = s.Events[len(s.Events)-24:]
	}
}
func (s *State) match() (gameapi.Match, error) {
	b, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	m := gameapi.Match{CheckpointKey: s.Phase, Schema: 1, Data: b, Actors: []int{}, TimeoutActors: []int{}, Attention: []int{}, Log: append([]string{}, s.Log...), Result: s.Result}
	if s.Result != nil {
		return m, nil
	}
	m.SystemDeadline = s.ServerAt + frameMS
	for _, c := range s.Cars {
		if c.FinishAt != 0 {
			continue
		}
		m.Actors = append(m.Actors, c.Seat)
		if m.Deadline == 0 || c.IdleAt < m.Deadline {
			m.Deadline = c.IdleAt
		}
	}
	for _, c := range s.Cars {
		if c.FinishAt == 0 && c.IdleAt == m.Deadline {
			m.TimeoutActors = append(m.TimeoutActors, c.Seat)
		}
	}
	return m, nil
}
func rank(s *State) {
	order := make([]int, len(s.Cars))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := s.Cars[order[i]], s.Cars[order[j]]
		if a.FinishAt > 0 && b.FinishAt > 0 {
			return a.FinishAt < b.FinishAt
		}
		if a.FinishAt > 0 {
			return true
		}
		if b.FinishAt > 0 {
			return false
		}
		return a.Progress > b.Progress
	})
	for i, seat := range order {
		s.Cars[seat].Rank = i + 1
		if i > 0 {
			a, b := s.Cars[order[i-1]], s.Cars[seat]
			if a.FinishAt == b.FinishAt && math.Abs(a.Progress-b.Progress) < .01 {
				s.Cars[seat].Rank = a.Rank
			}
		}
	}
}
func (s *State) finish() {
	rank(s)
	s.Phase = "finished"
	r := &gameapi.Result{Ranks: make([]int, len(s.Cars)), Winners: []int{}, Scores: make([]int, len(s.Cars))}
	moved := false
	for _, c := range s.Cars {
		moved = moved || c.Gate >= 0
		r.Ranks[c.Seat] = c.Rank
		r.Scores[c.Seat] = int(math.Max(0, c.Progress))
		if c.Rank == 1 {
			r.Winners = append(r.Winners, c.Seat)
		}
	}
	if !moved || len(r.Winners) == len(s.Cars) {
		r.Draw = true
		r.Winners = []int{}
		for i := range r.Ranks {
			r.Ranks[i] = 1
			s.Cars[i].Rank = 1
		}
	}
	r.Details, _ = json.Marshal(map[string]any{"track": s.Options.Track, "laps": s.Options.Laps, "cars": s.Cars, "reason": func() string {
		if s.ServerAt >= s.MaxRaceAt {
			return "race_limit"
		}
		for _, c := range s.Cars {
			if c.FinishAt == 0 {
				return "finish_deadline"
			}
		}
		return "all_finished"
	}()})
	s.Result = r
	s.Log = append(s.Log, "比赛结束，按过线先后及有效赛道进度结算")
}
