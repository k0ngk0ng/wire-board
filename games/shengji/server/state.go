package shengji

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Options struct {
	Decks        int    `json:"decks"`
	Rounds       int    `json:"rounds"`
	StartLevel   int    `json:"startLevel"`
	TurnSeconds  int    `json:"turnSeconds"`
	ThrowPenalty int    `json:"throwPenalty"`
	BottomMode   string `json:"bottomMode"`
}
type Declaration struct {
	Seat     int   `json:"seat"`
	Cards    []int `json:"cards"`
	Trump    int   `json:"trump"`
	Strength int   `json:"strength"`
}
type Reveal struct {
	Seat   int    `json:"seat"`
	Cards  []int  `json:"cards"`
	Reason string `json:"reason"`
}
type Play struct {
	Seat  int   `json:"seat"`
	Cards []int `json:"cards"`
}
type Trick struct {
	Plays  []Play `json:"plays"`
	Winner int    `json:"winner"`
	Points int    `json:"points"`
	Shape  Shape  `json:"shape"`
}
type Event struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Seat  int    `json:"seat"`
	Cards []int  `json:"cards,omitempty"`
	At    int64  `json:"at"`
	Label string `json:"label,omitempty"`
}
type RoundResult struct {
	Round        int   `json:"round"`
	Level        int   `json:"level"`
	Trump        int   `json:"trump"`
	Dealer       int   `json:"dealer"`
	AttackPoints int   `json:"attackPoints"`
	TrickPoints  int   `json:"trickPoints"`
	Penalty      int   `json:"penalty"`
	BottomPoints int   `json:"bottomPoints"`
	BottomFactor int   `json:"bottomFactor"`
	Team         int   `json:"team"`
	Promotion    int   `json:"promotion"`
	NextDealer   int   `json:"nextDealer"`
	Levels       []int `json:"levels"`
	Bottom       []int `json:"bottom"`
}
type State struct {
	Ruleset       string        `json:"ruleset"`
	Options       Options       `json:"options"`
	Hands         [][]int       `json:"hands"`
	Bottom        []int         `json:"bottom"`
	Played        []int         `json:"played"`
	Phase         string        `json:"phase"`
	Turn          int           `json:"turn"`
	Window        int           `json:"window"`
	Round         int           `json:"round"`
	DealSerial    int           `json:"dealSerial"`
	DealtAt       int64         `json:"dealtAt"`
	Levels        []int         `json:"levels"`
	Level         int           `json:"level"`
	Trump         int           `json:"trump"`
	Dealer        int           `json:"dealer"`
	NextDealer    int           `json:"nextDealer"`
	StartSeat     int           `json:"startSeat"`
	Declaration   *Declaration  `json:"declaration"`
	Revealed      []Reveal      `json:"revealed"`
	Passed        []bool        `json:"passed"`
	Rebellion     bool          `json:"rebellion"`
	Trick         []Play        `json:"trick"`
	Lead          *Shape        `json:"lead"`
	ChallengeSeat int           `json:"challengeSeat"`
	LastTrick     *Trick        `json:"lastTrick"`
	TrickNumber   int           `json:"trickNumber"`
	Captured      []int         `json:"captured"`
	Penalty       int           `json:"penalty"`
	Ready         []bool        `json:"ready"`
	Due           []int64       `json:"due"`
	Results       []RoundResult `json:"roundResults"`
	TieBreak      bool          `json:"tieBreak"`
	WinnerTeam    int           `json:"winnerTeam"`
	FinishReason  string        `json:"finishReason"`
	Events        []Event       `json:"events"`
	EventSerial   int           `json:"eventSerial"`
	Logs          []string      `json:"logs"`
}

func randomN(n int) (int, error) {
	v, e := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if e != nil {
		return 0, e
	}
	return int(v.Int64()), nil
}
func (s *State) rules() Rules { return Rules{s.Options.Decks, s.Level, s.Trump} }
func (s *State) bottomCount() int {
	if s.Options.Decks == 1 {
		return 6
	}
	return 8
}
func (s *State) handCount() int {
	if s.Options.Decks == 1 {
		return 12
	}
	return 25
}
func (s *State) note(format string, args ...any) {
	s.Logs = append(s.Logs, fmt.Sprintf(format, args...))
	if len(s.Logs) > 60 {
		s.Logs = s.Logs[len(s.Logs)-60:]
	}
}
func (s *State) event(kind string, seat int, ids []int, label string, now time.Time) {
	s.EventSerial++
	s.Events = append(s.Events, Event{s.EventSerial, kind, seat, append([]int{}, ids...), now.UnixMilli(), label})
	if len(s.Events) > 60 {
		s.Events = s.Events[len(s.Events)-60:]
	}
}
func (s *State) actors() []int {
	out := []int{}
	switch s.Phase {
	case "declare":
		for i := 0; i < 4; i++ {
			p := (s.StartSeat + i) % 4
			if !s.Passed[p] {
				out = append(out, p)
			}
		}
	case "bury":
		out = append(out, s.Dealer)
	case "play":
		out = append(out, s.Turn)
	case "round_end":
		for p := 0; p < 4; p++ {
			if !s.Ready[p] {
				out = append(out, p)
			}
		}
	}
	return out
}
func (s *State) canAct(p int) bool { return p >= 0 && p < 4 && slices.Contains(s.actors(), p) }
func (s *State) nextWindow(now time.Time) {
	s.Window++
	s.Due = make([]int64, 4)
	for _, p := range s.actors() {
		s.Due[p] = now.UnixMilli() + int64(s.Options.TurnSeconds)*1000
	}
}
func (s *State) deal(now time.Time) error {
	deck := make([]int, s.Options.Decks*54)
	for i := range deck {
		deck[i] = i
	}
	for i := len(deck) - 1; i > 0; i-- {
		j, e := randomN(i + 1)
		if e != nil {
			return e
		}
		deck[i], deck[j] = deck[j], deck[i]
	}
	s.Level = s.Options.StartLevel
	if s.Dealer >= 0 {
		s.Level = s.Levels[s.Dealer%2]
	}
	s.Trump = -1
	s.Hands = make([][]int, 4)
	n := s.handCount()
	for p := 0; p < 4; p++ {
		s.Hands[p] = append([]int{}, deck[p*n:(p+1)*n]...)
		sortHand(s.Hands[p], s.rules())
	}
	s.Bottom = append([]int{}, deck[4*n:]...)
	s.Played = []int{}
	s.Declaration = nil
	s.Revealed = []Reveal{}
	s.Passed = make([]bool, 4)
	s.Ready = make([]bool, 4)
	s.Trick = []Play{}
	s.Lead = nil
	s.LastTrick = nil
	s.ChallengeSeat = -1
	s.TrickNumber = 1
	s.Captured = []int{0, 0}
	s.Penalty = 0
	s.Phase = "declare"
	s.Turn = s.StartSeat
	s.DealSerial++
	s.DealtAt = now.UnixMilli()
	s.event("deal", -1, nil, "", now)
	s.note("第 %d 副，%d 副牌，打 %s", s.Round, s.Options.Decks, levelName(s.Level))
	s.nextWindow(now)
	return nil
}
func (s *State) declarationFor(p int, ids []int) (Declaration, error) {
	bad := errors.New("亮主须使用级牌；反主须比当前声明更强，不能自反或同强度反主")
	if !owns(s.Hands[p], ids) || len(ids) > s.Options.Decks {
		return Declaration{}, bad
	}
	level := s.Level
	if s.Rebellion {
		level = s.Levels[p%2]
	}
	d := Declaration{Seat: p, Cards: append([]int{}, ids...), Trump: printedSuit(ids[0]), Strength: len(ids)}
	if len(ids) == 1 {
		if rank(ids[0]) != level {
			return d, bad
		}
	} else if len(ids) == 2 {
		if ids[0]%54 != ids[1]%54 {
			return d, bad
		}
		if rank(ids[0]) >= 16 {
			d.Trump = -1
			d.Strength = 3
		} else if rank(ids[0]) != level {
			return d, bad
		}
	} else {
		return d, bad
	}
	if s.Declaration != nil && (p == s.Declaration.Seat || d.Strength <= s.Declaration.Strength) {
		return d, bad
	}
	return d, nil
}
func (s *State) declarations(p int) []Declaration {
	out := []Declaration{}
	for _, id := range s.Hands[p] {
		if d, e := s.declarationFor(p, []int{id}); e == nil {
			out = append(out, d)
		}
	}
	for _, pair := range pairsIn(s.Hands[p], s.rules()) {
		if d, e := s.declarationFor(p, pair.IDs); e == nil {
			out = append(out, d)
		}
	}
	return out
}
func (s *State) canRebel(p int) bool {
	if s.Options.Decks != 1 || s.Phase != "declare" || !s.canAct(p) || points(s.Hands[p]) != 0 {
		return false
	}
	return p != s.Dealer || points(s.Bottom) == 0
}
func (s *State) finishDeclaration(now time.Time) error {
	if s.Declaration == nil && s.Dealer < 0 && s.Options.Decks == 2 && !s.Rebellion {
		s.note("首副无人亮主，重新发牌")
		return s.deal(now)
	}
	if s.Dealer < 0 {
		s.Dealer = s.StartSeat
		if s.Declaration != nil {
			s.Dealer = s.Declaration.Seat
		}
	}
	if s.Rebellion && s.Declaration != nil {
		s.Dealer = s.Declaration.Seat
	}
	s.Level = s.Levels[s.Dealer%2]
	if s.Declaration == nil {
		exposed := []int{}
		best := -1
		s.Trump = -1
		for _, id := range s.Bottom {
			exposed = append(exposed, id)
			if rank(id) == s.Level {
				s.Trump = printedSuit(id)
				break
			}
			if rank(id) < 16 && (best < 0 || rank(id) > rank(best)) {
				best = id
			}
		}
		if s.Trump < 0 && best >= 0 {
			s.Trump = printedSuit(best)
		}
		s.Revealed = append(s.Revealed, Reveal{-1, exposed, "无人亮主，翻底定主"})
		s.event("reveal_bottom", -1, exposed, "翻底定主", now)
	}
	s.Rebellion = false
	s.Hands[s.Dealer] = append(s.Hands[s.Dealer], s.Bottom...)
	s.Bottom = []int{}
	for p := range s.Hands {
		sortHand(s.Hands[p], s.rules())
	}
	s.Phase = "bury"
	s.Turn = s.Dealer
	s.note("玩家 %d 坐庄，请扣 %d 张底牌", s.Dealer+1, s.bottomCount())
	s.event("take_bottom", s.Dealer, nil, "", now)
	s.nextWindow(now)
	return nil
}
func (s *State) finishTrick(now time.Time) {
	winner := s.Trick[0]
	all := []int{}
	for i, p := range s.Trick {
		all = append(all, p.Cards...)
		if i > 0 && beatsTrick(p.Cards, winner.Cards, *s.Lead, s.rules()) {
			winner = p
		}
	}
	value := points(all)
	s.Captured[winner.Seat%2] += value
	s.LastTrick = &Trick{append([]Play{}, s.Trick...), winner.Seat, value, *s.Lead}
	s.event("trick", winner.Seat, nil, fmt.Sprintf("收取 %d 分", value), now)
	s.Trick = []Play{}
	shape := *s.Lead
	s.Lead = nil
	s.ChallengeSeat = -1
	s.Turn = winner.Seat
	if len(s.Hands[0]) == 0 {
		s.finishRound(winner.Seat, shape, now)
	} else {
		s.TrickNumber++
		s.nextWindow(now)
	}
}
func (s *State) finishRound(lastWinner int, shape Shape, now time.Time) {
	attacking := 1 - s.Dealer%2
	factor := 0
	bottomPoints := 0
	if lastWinner%2 == attacking {
		factor = bottomFactor(shape, s.Options.BottomMode, s.Options.Decks)
		bottomPoints = points(s.Bottom) * factor
	}
	captured := s.Captured[attacking]
	score := max(0, captured+s.Penalty+bottomPoints)
	attack, promotion := scoreRound(score, s.Options.Decks)
	team := s.Dealer % 2
	s.NextDealer = (s.Dealer + 2) % 4
	if attack {
		team = attacking
		s.NextDealer = (s.Dealer + 1) % 4
	}
	passedAce := team == s.Dealer%2 && s.Level == 14 && promotion > 0
	s.Levels[team] = min(14, s.Levels[team]+promotion)
	s.Results = append(s.Results, RoundResult{Round: s.Round, Level: s.Level, Trump: s.Trump, Dealer: s.Dealer, AttackPoints: score, TrickPoints: captured, Penalty: s.Penalty, BottomPoints: bottomPoints, BottomFactor: factor, Team: team, Promotion: promotion, NextDealer: s.NextDealer, Levels: append([]int{}, s.Levels...), Bottom: append([]int{}, s.Bottom...)})
	s.note("抓分方 %d 分，%s方升 %d 级", score, map[bool]string{true: "抓分", false: "庄家"}[attack], promotion)
	s.event("round_end", lastWinner, s.Bottom, "", now)
	s.WinnerTeam = -1
	s.FinishReason = ""
	if passedAce {
		s.WinnerTeam = team
		s.FinishReason = "passed_ace"
	} else if s.TieBreak {
		s.WinnerTeam = team
		s.FinishReason = "tie_break"
	} else if s.Options.Rounds > 0 && s.Round >= s.Options.Rounds {
		if s.Levels[0] == s.Levels[1] {
			s.TieBreak = true
			s.note("双方级数相同，加打一副决胜")
		} else {
			s.WinnerTeam = 0
			if s.Levels[1] > s.Levels[0] {
				s.WinnerTeam = 1
			}
			s.FinishReason = "round_limit"
		}
	}
	s.Phase = "round_end"
	s.Ready = make([]bool, 4)
	if s.WinnerTeam >= 0 {
		s.Phase = "finished"
		s.event("finish", lastWinner, nil, s.FinishReason, now)
	}
	s.nextWindow(now)
}
func (s *State) envelope() (gameapi.Match, error) {
	if e := s.valid(); e != nil {
		return gameapi.Match{}, e
	}
	data, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	actors := s.actors()
	deadline := int64(0)
	for _, p := range actors {
		if deadline == 0 || s.Due[p] < deadline {
			deadline = s.Due[p]
		}
	}
	timed := []int{}
	for _, p := range actors {
		if s.Due[p] == deadline {
			timed = append(timed, p)
		}
	}
	m := gameapi.Match{Schema: 1, Data: data, Actors: actors, TimeoutActors: timed, Deadline: deadline, Attention: append([]int{}, actors...), Log: append([]string{}, s.Logs...)}
	if s.Phase == "finished" {
		ranks, scores := make([]int, 4), make([]int, 4)
		winners := []int{}
		for p := 0; p < 4; p++ {
			ranks[p] = 3
			scores[p] = s.Levels[p%2]
			if p%2 == s.WinnerTeam {
				ranks[p] = 1
				winners = append(winners, p)
			}
		}
		details, _ := json.Marshal(map[string]any{"ruleset": s.Ruleset, "reason": s.FinishReason, "roundResults": s.Results})
		m.Result = &gameapi.Result{Ranks: ranks, Scores: scores, Winners: winners, Team: true, Details: details}
	}
	return m, m.Validate(4)
}
func decode(m gameapi.Match) (*State, error) {
	if m.Schema != 1 {
		return nil, errors.New("不支持的升级存档版本")
	}
	var s State
	if e := strict(m.Data, &s); e != nil {
		return nil, e
	}
	if e := s.valid(); e != nil {
		return nil, e
	}
	return &s, nil
}
func (s *State) valid() error {
	bad := func() error { return errors.New("升级存档不完整或不一致") }
	normalized, e := configure(4, mustJSON(s.Options))
	if e != nil {
		return bad()
	}
	var opts Options
	_ = json.Unmarshal(normalized, &opts)
	if opts != s.Options || s.Ruleset != ruleset(s.Options.Decks) || s.Round < 1 || s.Round > 10000 || s.Window < 1 || s.DealSerial < 1 || len(s.Hands) != 4 || len(s.Levels) != 2 || s.Level < 2 || s.Level > 14 || s.Trump < -1 || s.Trump > 3 || s.Dealer < -1 || s.Dealer > 3 || s.StartSeat < 0 || s.StartSeat > 3 || s.Turn < 0 || s.Turn > 3 || len(s.Ready) != 4 || len(s.Passed) != 4 || len(s.Due) != 4 || len(s.Captured) != 2 || len(s.Events) > 60 || len(s.Logs) > 60 || s.ChallengeSeat < -1 || s.ChallengeSeat > 3 {
		return bad()
	}
	for _, l := range s.Levels {
		if l < 2 || l > 14 {
			return bad()
		}
	}
	if !slices.Contains([]string{"declare", "bury", "play", "round_end", "finished"}, s.Phase) {
		return bad()
	}
	if s.Phase != "declare" && s.Dealer < 0 {
		return bad()
	}
	seen := make([]bool, s.Options.Decks*54)
	consume := func(ids []int) bool {
		for _, id := range ids {
			if id < 0 || id >= len(seen) || seen[id] {
				return false
			}
			seen[id] = true
		}
		return true
	}
	for _, h := range s.Hands {
		if len(h) > s.handCount()+s.bottomCount() || !consume(h) {
			return bad()
		}
	}
	if !consume(s.Bottom) || !consume(s.Played) {
		return bad()
	}
	for _, v := range seen {
		if !v {
			return bad()
		}
	}
	if s.Phase == "bury" {
		if len(s.Bottom) != 0 || len(s.Hands[s.Dealer]) != s.handCount()+s.bottomCount() {
			return bad()
		}
	} else if len(s.Bottom) != s.bottomCount() {
		return bad()
	}
	if len(s.Trick) > 3 || (s.Lead == nil) != (len(s.Trick) == 0) {
		return bad()
	}
	if len(s.Trick) > 0 {
		expected, e := leadShape(s.Trick[0].Cards, s.rules())
		if e != nil || !reflect.DeepEqual(expected, *s.Lead) {
			return bad()
		}
		played := map[int]bool{}
		for i, p := range s.Trick {
			if p.Seat != (s.Trick[0].Seat+i)%4 || len(p.Cards) != s.Lead.Count {
				return bad()
			}
			for _, id := range p.Cards {
				if played[id] || !slices.Contains(s.Played, id) {
					return bad()
				}
				played[id] = true
			}
		}
		if s.Turn != (s.Trick[0].Seat+len(s.Trick))%4 {
			return bad()
		}
	}
	if s.Phase == "play" && len(s.Hands[s.Turn]) == 0 {
		return bad()
	}
	expected := s.Round - 1
	if s.Phase == "round_end" || s.Phase == "finished" {
		expected = s.Round
		for _, h := range s.Hands {
			if len(h) != 0 {
				return bad()
			}
		}
	}
	if len(s.Results) != expected {
		return bad()
	}
	for _, p := range s.actors() {
		if s.Due[p] <= 0 {
			return bad()
		}
	}
	if s.Phase == "finished" && (s.WinnerTeam < 0 || s.WinnerTeam > 1 || s.FinishReason == "") {
		return bad()
	}
	return nil
}
func ruleset(decks int) string {
	if decks == 1 {
		return "shengji-shanghai-single-v1"
	}
	return "shengji-shanghai-double-v1"
}
func levelName(level int) string {
	if level >= 11 {
		return map[int]string{11: "J", 12: "Q", 13: "K", 14: "A"}[level]
	}
	return fmt.Sprint(level)
}
func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
