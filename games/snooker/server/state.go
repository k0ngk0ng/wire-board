package snooker

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Options struct {
	BestOf    int    `json:"bestOf"`
	Teams     []int  `json:"teams"`
	AimAssist string `json:"aimAssist"`
}
type Flight struct {
	Integrity string      `json:"integrity"`
	ID        int         `json:"id"`
	Start     int64       `json:"start"`
	End       int64       `json:"end"`
	Seat      int         `json:"seat"`
	Context   RuleContext `json:"context"`
	Strike    Strike      `json:"strike"`
	Motion    Motion      `json:"motion"`
	Resolved  bool        `json:"resolved"`
}
type Foul struct {
	Offender      int         `json:"offender"`
	OffenderIndex int         `json:"offenderIndex"`
	Next          int         `json:"next"`
	Ruling        Ruling      `json:"ruling"`
	Before        RuleContext `json:"before"`
}
type MissSequence struct {
	Offender int  `json:"offender"`
	Count    int  `json:"count"`
	Warned   bool `json:"warned"`
}
type FrameResult struct {
	Frame  int    `json:"frame"`
	Winner int    `json:"winner"`
	Scores [2]int `json:"scores"`
	Reason string `json:"reason"`
}
type State struct {
	Due          []int64       `json:"due,omitempty"`
	Concede      []bool        `json:"concede"`
	OfferTeam    int           `json:"offerTeam"`
	OfferTurn    int           `json:"offerTurn"`
	FinishReason string        `json:"finishReason,omitempty"`
	Ruleset      string        `json:"ruleset"`
	Options      Options       `json:"options"`
	Players      int           `json:"players"`
	Teams        []int         `json:"teams"`
	Order        []int         `json:"order"`
	NextFirst    [2]int        `json:"nextFirst"`
	TurnIndex    int           `json:"turnIndex"`
	Turn         int           `json:"turn"`
	Phase        string        `json:"phase"`
	Window       int           `json:"window"`
	Balls        []Ball        `json:"balls"`
	Target       string        `json:"target"`
	Clear        int           `json:"clear"`
	InHand       bool          `json:"inHand"`
	FreeBall     bool          `json:"freeBall"`
	Scores       [2]int        `json:"scores"`
	FramesWon    [2]int        `json:"framesWon"`
	Frame        int           `json:"frame"`
	OpeningTeam  int           `json:"openingTeam"`
	OpeningSeat  int           `json:"openingSeat"`
	Break        int           `json:"break"`
	BestBreak    []int         `json:"bestBreak"`
	Flight       *Flight       `json:"flight,omitempty"`
	ShotID       int           `json:"shotId"`
	Foul         *Foul         `json:"foul,omitempty"`
	MissRun      *MissSequence `json:"missRun,omitempty"`
	BlackDecider bool          `json:"blackDecider"`
	BlackStarter int           `json:"blackStarter"`
	Ready        []bool        `json:"ready"`
	Resign       []bool        `json:"resign"`
	Rerack       []bool        `json:"rerack"`
	Results      []FrameResult `json:"results"`
	Winner       int           `json:"winner"`
	Logs         []string      `json:"logs"`
}

func toss() int {
	n, e := rand.Int(rand.Reader, big.NewInt(2))
	if e != nil {
		return 0
	}
	return int(n.Int64())
}
func (s *State) context() RuleContext {
	return RuleContext{Balls: append([]Ball{}, s.Balls...), Target: s.Target, Clear: s.Clear, FreeBall: s.FreeBall, InHand: s.InHand, Scores: s.Scores, Team: s.Teams[s.Turn]}
}
func (s *State) note(f string, a ...any) {
	s.Logs = append(s.Logs, fmt.Sprintf(f, a...))
	if len(s.Logs) > 60 {
		s.Logs = s.Logs[len(s.Logs)-60:]
	}
}
func (s *State) first(team int) int {
	for p, v := range s.Teams {
		if v == team {
			return p
		}
	}
	return 0
}
func (s *State) setTurn(seat int) { s.Turn = seat; s.TurnIndex = slices.Index(s.Order, seat) }
func (s *State) nextSeat() int    { return s.Order[(s.TurnIndex+1)%s.Players] }
func (s *State) setOrder() {
	lists := [2][]int{}
	for p, team := range s.Teams {
		lists[team] = append(lists[team], p)
	}
	for team := 0; team < 2; team++ {
		if len(lists[team]) == 2 && lists[team][1] == s.NextFirst[team] {
			slices.Reverse(lists[team])
		}
	}
	s.Order = []int{}
	for i := 0; i < len(lists[0]); i++ {
		s.Order = append(s.Order, lists[0][i], lists[1][i])
	}
}
func (s *State) rack() {
	s.Due = nil
	s.Balls = InitialBalls()
	s.Scores = [2]int{}
	s.Target = "red"
	s.Clear = 2
	s.InHand = true
	s.FreeBall = false
	s.Break = 0
	s.Foul = nil
	s.MissRun = nil
	s.Flight = nil
	s.BlackDecider = false
	s.BlackStarter = -1
	s.Ready = make([]bool, s.Players)
	s.Concede = make([]bool, s.Players)
	s.OfferTeam = -1
	s.OfferTurn = -1
	s.Resign = make([]bool, s.Players)
	s.Rerack = make([]bool, s.Players)
	s.setOrder()
	s.setTurn(s.NextFirst[s.OpeningTeam])
	s.OpeningSeat = s.Turn
	s.Phase = "aim"
}
func (s *State) actors() []int {
	switch s.Phase {
	case "rolling", "finished":
		return []int{}
	case "start_choice", "foul", "black_choice", "black_player", "concede":
		return []int{s.Turn}
	default:
		out := []int{s.Turn}
		for p := 0; p < s.Players; p++ {
			if p != s.Turn {
				out = append(out, p)
			}
		}
		return out
	}
}
func (s *State) match() (gameapi.Match, error) {
	if e := s.valid(); e != nil {
		return gameapi.Match{}, e
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	m := gameapi.Match{Schema: 1, Data: raw, Actors: s.actors(), Attention: []int{}, Log: append([]string{}, s.Logs...)}
	if s.Phase == "rolling" {
		m.SystemDeadline = s.Flight.End
	} else if s.Phase != "finished" {
		if s.Phase == "frame_end" {
			for p := 0; p < s.Players; p++ {
				if !s.Ready[p] {
					m.Attention = append(m.Attention, p)
				}
			}
		} else {
			m.Attention = []int{s.Turn}
		}
	}
	for _, p := range m.Attention {
		if len(s.Due) == s.Players && s.Due[p] > 0 {
			due := s.Due[p]
			if m.Deadline == 0 || due < m.Deadline {
				m.Deadline = due
				m.TimeoutActors = []int{p}
			} else if due == m.Deadline {
				m.TimeoutActors = append(m.TimeoutActors, p)
			}
		}
	}
	if s.Phase == "finished" {
		r := &gameapi.Result{Ranks: make([]int, s.Players), Scores: make([]int, s.Players), Team: s.Players == 4}
		for p, team := range s.Teams {
			r.Ranks[p] = s.Players/2 + 1
			r.Scores[p] = s.FramesWon[team]
			if team == s.Winner {
				r.Winners = append(r.Winners, p)
				r.Ranks[p] = 1
			}
		}
		r.Details, _ = json.Marshal(map[string]any{"ruleset": s.Ruleset, "frames": s.Results, "framesWon": s.FramesWon, "bestBreak": s.BestBreak, "finishReason": s.FinishReason})
		m.Result = r
	}
	return m, nil
}
func decode(m gameapi.Match) (*State, error) {
	if m.Schema != 1 {
		return nil, errors.New("不支持的斯诺克存档版本")
	}
	var s State
	if json.Unmarshal(m.Data, &s) != nil {
		return nil, errors.New("无效斯诺克存档")
	}
	if e := s.valid(); e != nil {
		return nil, e
	}
	return &s, nil
}
func (s *State) valid() error {
	bad := func() error { return errors.New("斯诺克存档不完整或不一致") }
	if len(s.Due) != 0 && len(s.Due) != s.Players {
		return bad()
	}
	for _, due := range s.Due {
		if due < 0 {
			return bad()
		}
	}
	if s.Ruleset != "snooker-wpbsa-2024-v1" || (s.Players != 2 && s.Players != 4) || s.Window < 1 || s.Frame < 1 || len(s.Balls) != 22 || len(s.Teams) != s.Players || len(s.Order) != s.Players || len(s.BestBreak) != s.Players || len(s.Ready) != s.Players || len(s.Resign) != s.Players || len(s.Concede) != s.Players || len(s.Rerack) != s.Players || s.Turn < 0 || s.Turn >= s.Players || s.TurnIndex < 0 || s.TurnIndex >= s.Players || s.Order[s.TurnIndex] != s.Turn {
		return bad()
	}
	if !slices.Contains([]string{"start_choice", "aim", "rolling", "foul", "black_choice", "black_player", "frame_end", "finished", "concede"}, s.Phase) || !slices.Contains([]string{"red", "colour", "clear"}, s.Target) || s.Clear < 2 || s.Clear > 8 || s.OpeningTeam < 0 || s.OpeningTeam > 1 {
		return bad()
	}
	normalized, e := configure(s.Players, mustJSON(s.Options))
	if e != nil || string(normalized) != string(mustJSON(s.Options)) {
		return bad()
	}
	seen := map[int]bool{}
	teams := [2]int{}
	for i, p := range s.Order {
		if p < 0 || p >= s.Players || seen[p] || s.Teams[p] < 0 || s.Teams[p] > 1 || s.Teams[p] != i%2 || s.Options.Teams[p] != s.Teams[p] {
			return bad()
		}
		seen[p] = true
		teams[s.Teams[p]]++
	}
	if teams[0] != s.Players/2 || teams[1] != s.Players/2 {
		return bad()
	}
	for team := 0; team < 2; team++ {
		if s.NextFirst[team] < 0 || s.NextFirst[team] >= s.Players || s.Teams[s.NextFirst[team]] != team || s.Scores[team] < 0 || s.Scores[team] > 100000 || s.FramesWon[team] < 0 || s.FramesWon[team] > s.Options.BestOf {
			return bad()
		}
	}
	seen = map[int]bool{}
	for i, b := range s.Balls {
		if b.ID < 0 || b.ID > 21 || seen[b.ID] || !finite(b.Pos.X) || !finite(b.Pos.Y) {
			return bad()
		}
		seen[b.ID] = true
		if b.Down {
			continue
		}
		if b.Pos.X < -.001 || b.Pos.X > TableWidth+.001 || b.Pos.Y < -.001 || b.Pos.Y > TableHeight+.001 {
			return bad()
		}
		for _, o := range s.Balls[:i] {
			if !o.Down && b.Pos.sub(o.Pos).length() < 2*BallRadius-.00002 {
				return bad()
			}
		}
	}
	if s.Phase == "rolling" && (s.Flight == nil || s.Flight.Resolved || s.Flight.End <= s.Flight.Start) {
		return bad()
	}
	if s.Phase == "foul" && (s.Foul == nil || s.Foul.Offender < 0 || s.Foul.Offender >= s.Players || s.Foul.Next != s.Turn) {
		return bad()
	}
	if s.Phase == "concede" && (s.OfferTeam < 0 || s.OfferTeam > 1 || s.OfferTurn < 0 || s.OfferTurn >= s.Players || s.Teams[s.Turn] == s.OfferTeam) {
		return bad()
	}
	if s.Phase == "finished" && (s.Winner < 0 || s.Winner > 1) {
		return bad()
	}
	if s.Frame > s.Options.BestOf {
		return bad()
	}
	wins := [2]int{}
	for i, r := range s.Results {
		if r.Frame != i+1 || r.Winner < 0 || r.Winner > 1 || r.Scores[0] < 0 || r.Scores[1] < 0 {
			return bad()
		}
		wins[r.Winner]++
	}
	if wins != s.FramesWon {
		return bad()
	}
	expected := s.Frame - 1
	if s.Phase == "frame_end" || (s.Phase == "finished" && s.FinishReason != "resigned") {
		expected = s.Frame
	}
	if len(s.Results) != expected && !(s.Phase == "finished" && s.FinishReason == "resigned" && len(s.Results) == s.Frame) {
		return bad()
	}
	if s.MissRun != nil && (s.MissRun.Offender < 0 || s.MissRun.Offender >= s.Players || s.MissRun.Count < 1 || s.MissRun.Count > 2 || s.MissRun.Warned != (s.MissRun.Count == 2)) {
		return bad()
	}
	if s.Phase != "finished" && s.Phase != "frame_end" {
		if s.Target == "red" && redsLeft(s.Balls) == 0 || s.Target == "clear" && (redsLeft(s.Balls) > 0 || s.Clear > 7) {
			return bad()
		}
		for _, b := range s.Balls {
			if b.ID < 16 {
				continue
			}
			shouldBeOn := s.Target != "clear" || BallValue(b.ID) >= s.Clear
			if b.Down == shouldBeOn {
				return bad()
			}
		}
	}
	if f := s.Flight; f != nil {
		if f.ID < 1 || f.ID > s.ShotID || f.Seat < 0 || f.Seat >= s.Players || f.Context.Team != s.Teams[f.Seat] || !validStrike(f.Strike) || !finite(f.Motion.Duration) || f.Motion.Duration <= 0 || f.Motion.Duration > 30 || f.End != f.Start+int64(math.Ceil(f.Motion.Duration*1000)) || len(f.Motion.Balls) != 22 || len(f.Motion.Frames) < 2 || f.Integrity != flightHash(f) {
			return bad()
		}
		if s.Phase == "rolling" && (f.Seat != s.Turn || !bytes.Equal(mustJSON(f.Context), mustJSON(s.context()))) {
			return bad()
		}
	}
	return nil
}
func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func (s *State) finishFrame(team int, reason string) {
	s.Concede = make([]bool, s.Players)
	s.OfferTeam = -1
	s.OfferTurn = -1
	s.FramesWon[team]++
	s.Results = append(s.Results, FrameResult{s.Frame, team, s.Scores, reason})
	s.Break = 0
	s.FreeBall = false
	s.Foul = nil
	s.MissRun = nil
	s.note("%s", s.finishExplanation(reason))
	s.note("第 %d 局结束，%s 队获胜（%d : %d）", s.Frame, []string{"A", "B"}[team], s.Scores[0], s.Scores[1])
	if s.FramesWon[team] > s.Options.BestOf/2 {
		s.Phase = "finished"
		s.Winner = team
		s.FinishReason = reason
	} else {
		s.Phase = "frame_end"
		s.Ready = make([]bool, s.Players)
	}
}
func (s *State) settle(now time.Time) error {
	f := s.Flight
	if f == nil || f.Resolved || now.UnixMilli() < f.End {
		return nil
	}
	r := Judge(f.Context, f.Strike, f.Motion)
	s.Balls = append([]Ball{}, f.Motion.Balls...)
	f.Resolved = true
	oldTurn := s.Turn
	oldIndex := s.TurnIndex
	team := s.Teams[oldTurn]
	s.Target = r.Target
	s.Clear = r.Clear
	s.InHand = r.InHand
	s.FreeBall = false
	s.Rerack = make([]bool, s.Players)
	s.Resign = make([]bool, s.Players)
	s.Concede = make([]bool, s.Players)
	onlyBlack := f.Context.Target == "clear" && f.Context.Clear == 7
	for _, b := range f.Context.Balls {
		if b.ID > 0 && b.ID != 21 && !b.Down {
			onlyBlack = false
		}
	}
	if r.Foul > 0 {
		s.Scores[1-team] += r.Foul
		s.Break = 0
		s.note("玩家 %d 犯规，%s 队加 %d 分：%s", oldTurn+1, []string{"A", "B"}[1-team], r.Foul, joinReasons(r.Reasons))
	} else {
		s.Scores[team] += r.Points
		if r.Points > 0 {
			s.Break += r.Points
			s.BestBreak[oldTurn] = max(s.BestBreak[oldTurn], s.Break)
			s.note("玩家 %d 得 %d 分，单杆 %d", oldTurn+1, r.Points, s.Break)
		} else {
			s.Break = 0
		}
	}
	if onlyBlack && (r.Points > 0 || r.Foul > 0) {
		if s.Scores[0] == s.Scores[1] && !s.BlackDecider {
			var e error
			s.Balls, e = SpotColours(s.Balls, []int{21})
			if e != nil {
				return e
			}
			for i := range s.Balls {
				if s.Balls[i].ID == 0 {
					s.Balls[i].Down = true
				}
			}
			s.Clear = 7
			s.Target = "clear"
			s.InHand = true
			s.Break = 0
			s.BlackDecider = true
			s.Phase = "black_choice"
			s.setTurn(s.first(toss()))
			s.note("双方平分，黑球复位；抽签胜方选择先打的一队")
		} else {
			winner := 0
			if s.Scores[1] > s.Scores[0] {
				winner = 1
			}
			s.finishFrame(winner, "black")
		}
		s.Window++
		return nil
	}
	var e error
	s.Balls, e = SpotColours(s.Balls, r.Respot)
	if e != nil {
		return e
	}
	if r.Foul > 0 {
		// Once a full-ball miss has started a restored-position sequence, later
		// failures keep that sequence even if accumulating penalties change the gap.
		if r.FullBall && s.MissRun != nil && s.MissRun.Offender == oldTurn && len(r.Reasons) > 0 {
			for _, why := range r.Reasons {
				if why == "未先击中目标球" || why == "先击中非目标球" || why == "同时先碰多个不同目标球" {
					r.Miss = true
				}
			}
		}
		if r.Miss && r.FullBall {
			warned := s.MissRun != nil && s.MissRun.Offender == oldTurn && s.MissRun.Warned
			if s.MissRun == nil || s.MissRun.Offender != oldTurn {
				s.MissRun = &MissSequence{Offender: oldTurn}
			}
			s.MissRun.Count++
			if s.MissRun.Count >= 3 && warned {
				s.finishFrame(1-team, "three_misses")
				s.Window++
				return nil
			}
			if s.MissRun.Count >= 2 {
				s.MissRun.Warned = true
				s.note("警告：再次要求原位重打后仍未先击中目标球，将判负本局")
			}
		} else {
			s.MissRun = nil
		}
		next := s.nextSeat()
		s.Foul = &Foul{oldTurn, oldIndex, next, r, f.Context}
		s.setTurn(next)
		s.Phase = "foul"
	} else {
		s.MissRun = nil
		s.Foul = nil
		s.Phase = "aim"
		if !r.KeepTurn {
			s.setTurn(s.nextSeat())
		}
	}
	s.Window++
	return nil
}
func joinReasons(reasons []string) string {
	out := ""
	for i, s := range reasons {
		if i > 0 {
			out += "；"
		}
		out += s
	}
	return out
}

// Integrity detects damaged/replaced persisted stroke data; it is not an
// authorization mechanism. Seats and commands are authenticated by the host.
func flightHash(f *Flight) string {
	copy := *f
	copy.Integrity = ""
	copy.Resolved = false
	sum := sha256.Sum256(mustJSON(copy))
	return fmt.Sprintf("%x", sum)
}
