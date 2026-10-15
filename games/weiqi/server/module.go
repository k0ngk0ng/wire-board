package weiqi

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"time"
)

type Options struct {
	Size        int     `json:"size"`
	Komi        float64 `json:"komi"`
	Difficulty  string  `json:"difficulty"`
	TurnSeconds int     `json:"turnSeconds"`
}
type Event struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Seat    int    `json:"seat"`
	Pos     int    `json:"pos"`
	Removed []int  `json:"removed"`
	At      int64  `json:"at"`
}
type State struct {
	Options   Options  `json:"options"`
	Round     string   `json:"round"`
	Board     []int    `json:"board"`
	Black     int      `json:"black"`
	Turn      int      `json:"turn"`
	Ply       int      `json:"ply"`
	Revision  int      `json:"revision"`
	Sequence  int      `json:"sequence"`
	Phase     string   `json:"phase"`
	Winner    int      `json:"winner"`
	Reason    string   `json:"reason"`
	Passes    int      `json:"passes"`
	Keys      []string `json:"keys"`
	Captured  [2]int   `json:"captured"`
	Dead      []int    `json:"dead"`
	Confirmed [2]bool  `json:"confirmed"`
	Deadline  int64    `json:"deadline"`
	Deadlines [2]int64 `json:"deadlines"`
	Score     Score    `json:"score"`
	Last      Event    `json:"last"`
	Log       []string `json:"log"`
}
type View struct {
	State
	Legal     []int `json:"legal"`
	CanPlay   bool  `json:"canPlay"`
	CanScore  bool  `json:"canScore"`
	CanResign bool  `json:"canResign"`
}
type Action struct {
	Type     string `json:"type"`
	Round    string `json:"round"`
	Ply      int    `json:"ply"`
	Revision int    `json:"revision"`
	Pos      *int   `json:"pos,omitempty"`
}

func strict(raw []byte, v any) error {
	if len(raw) > 4096 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func configure(players int, raw json.RawMessage) (json.RawMessage, error) {
	if players != 2 {
		return nil, errors.New("围棋需要两名玩家")
	}
	o := Options{Size: 19, Komi: 7.5, Difficulty: "standard", TurnSeconds: 120}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效围棋设置")
		}
	}
	if (o.Size != 9 && o.Size != 13 && o.Size != 19) || (o.Komi != 0 && o.Komi != 7.5) || (o.Difficulty != "easy" && o.Difficulty != "standard") || o.TurnSeconds < 30 || o.TurnSeconds > 300 {
		return nil, errors.New("请选择 9／13／19 路、白贴 7.5 目或不贴目，以及支持的难度和 30～300 秒时限")
	}
	return json.Marshal(o)
}
func decode(m gameapi.Match) (*State, error) {
	var s State
	if m.Schema != 1 || json.Unmarshal(m.Data, &s) != nil || s.Round == "" || s.Black < 0 || s.Black > 1 || s.Turn < 0 || s.Turn > 1 || s.Ply < 0 || s.Revision < 0 || s.Passes < 0 || s.Passes > 2 {
		return nil, errors.New("无效围棋存档")
	}
	o, _ := json.Marshal(s.Options)
	if _, e := configure(2, o); e != nil {
		return nil, e
	}
	if len(s.Board) != s.Options.Size*s.Options.Size || len(s.Keys) == 0 {
		return nil, errors.New("无效棋盘")
	}
	for _, v := range s.Board {
		if v < 0 || v > 2 {
			return nil, errors.New("无效棋子")
		}
	}
	if s.Phase != "playing" && s.Phase != "scoring" && s.Phase != "finished" {
		return nil, errors.New("无效阶段")
	}
	if s.Phase == "playing" && s.Deadline <= 0 || s.Phase == "scoring" && (s.Deadlines[0] <= 0 || s.Deadlines[1] <= 0) {
		return nil, errors.New("无效时限")
	}
	seen := map[int]bool{}
	for _, p := range s.Dead {
		if p < 0 || p >= len(s.Board) || s.Phase != "finished" && s.Board[p] == 0 || seen[p] {
			return nil, errors.New("无效死子提案")
		}
		seen[p] = true
	}
	return &s, nil
}
func (s *State) colour(seat int) int {
	if seat == s.Black {
		return 1
	}
	return 2
}
func (s *State) label(seat int) string {
	if seat == s.Black {
		return "黑方"
	}
	return "白方"
}
func (s *State) note(line string) {
	s.Log = append(s.Log, line)
	if len(s.Log) > 80 {
		s.Log = append([]string(nil), s.Log[len(s.Log)-80:]...)
	}
}
func (s *State) event(c gameapi.Context, kind string, pos int, removed []int) {
	s.Sequence++
	s.Last = Event{ID: s.Sequence, Type: kind, Seat: c.Actor, Pos: pos, Removed: removed, At: c.Now.UnixMilli()}
}
func (s *State) end(winner int, reason string) {
	s.Phase = "finished"
	s.Winner = winner
	s.Reason = reason
	s.Deadline = 0
	s.Deadlines = [2]int64{}
}
func (s *State) envelope() (gameapi.Match, error) {
	raw, e := json.Marshal(s)
	m := gameapi.Match{Schema: 1, Data: raw, Log: append([]string(nil), s.Log...)}
	if s.Phase == "playing" {
		m.Actors = []int{0, 1}
		m.Attention = []int{s.Turn}
		m.TimeoutActors = []int{s.Turn}
		m.Deadline = s.Deadline
	} else if s.Phase == "scoring" {
		m.Actors = []int{0, 1}
		m.Attention = []int{}
		m.TimeoutActors = []int{}
		for seat := 0; seat < 2; seat++ {
			if !s.Confirmed[seat] {
				m.Attention = append(m.Attention, seat)
				if m.Deadline == 0 || s.Deadlines[seat] < m.Deadline {
					m.Deadline = s.Deadlines[seat]
				}
			}
		}
		for seat := 0; seat < 2; seat++ {
			if !s.Confirmed[seat] && s.Deadlines[seat] == m.Deadline {
				m.TimeoutActors = append(m.TimeoutActors, seat)
			}
		}
	} else {
		r := &gameapi.Result{Ranks: []int{1, 1}, Winners: []int{}}
		if s.Winner < 0 {
			r.Draw = true
		} else {
			r.Ranks[1-s.Winner] = 2
			r.Winners = []int{s.Winner}
		}
		details := map[string]any{"reason": s.Reason, "plies": s.Ply, "komi": s.Options.Komi, "size": s.Options.Size}
		if s.Reason == "score" {
			details["score"] = s.Score
		}
		r.Details, _ = json.Marshal(details)
		m.Result = r
	}
	return m, e
}
func Module() gameapi.Module {
	return gameapi.Module{ID: "weiqi", Name: "围棋", MinPlayers: 2, MaxPlayers: 2, Configure: configure, Start: start, Apply: apply, View: view, BotAction: bot, Reclaim: reclaim, AcceptStale: acceptStale, BotSearch: search}
}
func start(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	o, e := configure(c.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var opts Options
	_ = json.Unmarshal(o, &opts)
	id := make([]byte, 16)
	if _, e = rand.Read(id); e != nil {
		return gameapi.Match{}, e
	}
	s := State{Options: opts, Round: hex.EncodeToString(id), Board: make([]int, opts.Size*opts.Size), Black: int(id[0] & 1), Phase: "playing", Winner: -1, Dead: []int{}, Log: []string{fmt.Sprintf("%d 路围棋，面积计分、禁自杀、全局同形禁着；白贴 %g 目", opts.Size, opts.Komi)}}
	s.Turn = s.Black
	s.Keys = []string{PositionKey(s.Board)}
	s.Deadline = c.Now.Add(time.Duration(opts.TurnSeconds) * time.Second).UnixMilli()
	s.Score = score(s.Board, opts.Size, opts.Komi, nil)
	return s.envelope()
}
func validToken(s *State, a Action) bool {
	return a.Round == s.Round && a.Ply == s.Ply && a.Revision == s.Revision
}
func acceptStale(c gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil || c.Actor < 0 || c.Actor > 1 {
		return false
	}
	var a Action
	return strict(raw, &a) == nil && a.Type == "confirm_score" && a.Pos == nil && s.Phase == "scoring" && validToken(s, a) && !s.Confirmed[c.Actor]
}
func apply(c gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if c.Players != 2 || c.Actor < 0 || c.Actor > 1 || s.Phase == "finished" || strict(raw, &a) != nil {
		return m, errors.New("当前不能操作")
	}
	if !validToken(s, a) {
		return m, errors.New("棋盘或计分提案已更新，请重新操作")
	}
	if (a.Type == "place" || a.Type == "mark_dead") != (a.Pos != nil) {
		return m, errors.New("请检查操作的落点")
	}
	until := c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
	switch a.Type {
	case "place":
		if s.Phase != "playing" || s.Turn != c.Actor {
			return m, errors.New("请等待自己的落子回合")
		}
		next, removed, ok := play(s.Board, *a.Pos, s.colour(c.Actor), s.Options.Size, keySet(s.Keys))
		if !ok {
			return m, errors.New("此处不能落子：已占用、无气自杀或重复历史局面")
		}
		s.Board = next
		s.Keys = append(s.Keys, PositionKey(next))
		s.Captured[c.Actor] += len(removed)
		s.Ply++
		s.Passes = 0
		s.Turn = 1 - s.Turn
		s.Deadline = until
		s.event(c, "place", *a.Pos, removed)
		s.note(fmt.Sprintf("%s落子 %c%d，提 %d 子", s.label(c.Actor), 'A'+*a.Pos%s.Options.Size, s.Options.Size-*a.Pos/s.Options.Size, len(removed)))
	case "pass":
		if s.Phase != "playing" || s.Turn != c.Actor {
			return m, errors.New("请等待自己的回合")
		}
		s.Ply++
		s.Passes++
		s.Turn = 1 - s.Turn
		s.Deadline = until
		s.event(c, "pass", -1, nil)
		s.note(s.label(c.Actor) + "停着")
		if s.Passes == 2 {
			s.Phase = "scoring"
			s.Revision++
			s.Dead = []int{}
			s.Confirmed = [2]bool{}
			s.Deadlines = [2]int64{until, until}
			s.Deadline = 0
			s.Score = score(s.Board, s.Options.Size, s.Options.Komi, nil)
			s.note("双方连续停着，请标记死子并确认计分；有争议可继续对弈")
		}
	case "mark_dead":
		if s.Phase != "scoring" || *a.Pos < 0 || *a.Pos >= len(s.Board) || s.Board[*a.Pos] == 0 {
			return m, errors.New("请在计分阶段选择一块棋子")
		}
		stones, _ := group(s.Board, *a.Pos, s.Options.Size)
		marked := make(map[int]bool, len(s.Dead))
		for _, p := range s.Dead {
			marked[p] = true
		}
		remove := marked[*a.Pos]
		for _, p := range stones {
			if remove {
				delete(marked, p)
			} else {
				marked[p] = true
			}
		}
		s.Dead = []int{}
		for p := range s.Board {
			if marked[p] {
				s.Dead = append(s.Dead, p)
			}
		}
		s.Revision++
		s.Confirmed = [2]bool{}
		s.Score = score(s.Board, s.Options.Size, s.Options.Komi, s.Dead)
		s.note(s.label(c.Actor) + "修改死子提案，双方须重新确认")
	case "confirm_score":
		if s.Phase != "scoring" || s.Confirmed[c.Actor] {
			return m, errors.New("当前没有待确认的计分提案")
		}
		s.Confirmed[c.Actor] = true
		s.note(s.label(c.Actor) + "确认当前计分提案")
		if s.Confirmed[0] && s.Confirmed[1] {
			s.Score = score(s.Board, s.Options.Size, s.Options.Komi, s.Dead)
			for _, p := range s.Dead {
				s.Board[p] = 0
			}
			winner := -1
			if s.Score.Black > s.Score.White {
				winner = s.Black
			}
			if s.Score.White > s.Score.Black {
				winner = 1 - s.Black
			}
			s.end(winner, "score")
			s.event(c, "score", -1, append([]int(nil), s.Dead...))
			s.note(fmt.Sprintf("确认计分：黑 %g，白 %g（含贴目）", s.Score.Black, s.Score.White))
		}
	case "resume":
		if s.Phase != "scoring" {
			return m, errors.New("当前不在计分确认阶段")
		}
		s.Phase = "playing"
		s.Revision++
		s.Passes = 0
		s.Dead = []int{}
		s.Confirmed = [2]bool{}
		s.Deadlines = [2]int64{}
		s.Deadline = until
		s.Score = score(s.Board, s.Options.Size, s.Options.Komi, nil)
		s.event(c, "resume", -1, nil)
		s.note(s.label(c.Actor) + "选择继续对弈，保留棋盘和重复历史")
	case "resign":
		s.end(1-c.Actor, "resign")
		s.event(c, "resign", -1, nil)
		s.note(s.label(c.Actor) + "认输")
	default:
		return m, errors.New("未知围棋操作")
	}
	return s.envelope()
}
func view(m gameapi.Match, seat int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	active := seat >= 0 && seat < 2 && s.Phase != "finished"
	v := View{State: *s, Legal: []int{}, CanPlay: active && s.Phase == "playing" && seat == s.Turn, CanScore: active && s.Phase == "scoring", CanResign: active}
	if v.CanPlay {
		history := keySet(s.Keys)
		for p, c := range s.Board {
			if c == 0 {
				if _, _, ok := play(s.Board, p, s.colour(seat), s.Options.Size, history); ok {
					v.Legal = append(v.Legal, p)
				}
			}
		}
	}
	return v, nil
}
func reclaim(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Actor < 0 || c.Actor > 1 {
		return m, errors.New("无效席位")
	}
	until := c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
	changed := false
	if s.Phase == "playing" && c.Actor == s.Turn && s.Deadline <= c.Now.UnixMilli() {
		s.Deadline = until
		changed = true
	}
	if s.Phase == "scoring" && !s.Confirmed[c.Actor] && s.Deadlines[c.Actor] <= c.Now.UnixMilli() {
		s.Deadlines[c.Actor] = until
		changed = true
	}
	if changed {
		return s.envelope()
	}
	return m, nil
}
