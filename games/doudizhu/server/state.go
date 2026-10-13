package doudizhu

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Options struct {
	Mode             string `json:"mode"`
	Rounds           int    `json:"rounds"`
	TurnSeconds      int    `json:"turnSeconds"`
	PersonalDoubling bool   `json:"personalDoubling"`
	AllowStraight    bool   `json:"allowStraight"`
}
type Play struct {
	Seat  int   `json:"seat"`
	Cards []int `json:"cards"`
	Pass  bool  `json:"pass"`
	Combination
}
type Event struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Seat  int    `json:"seat"`
	Cards []int  `json:"cards,omitempty"`
	Kind  string `json:"kind,omitempty"`
	At    int64  `json:"at"`
}
type RoundResult struct {
	Round      int    `json:"round"`
	Landlord   int    `json:"landlord"`
	FirstOut   int    `json:"firstOut"`
	Winners    []int  `json:"winners"`
	Bid        int    `json:"bid"`
	Multiplier int    `json:"multiplier"`
	Spring     string `json:"spring"`
	Scores     []int  `json:"scores"`
}
type State struct {
	Options        Options       `json:"options"`
	Ruleset        string        `json:"ruleset"`
	Players        int           `json:"players"`
	Hands          [][]int       `json:"hands"`
	Bottom         []int         `json:"bottom"`
	Played         []int         `json:"played"`
	Phase          string        `json:"phase"`
	Turn           int           `json:"turn"`
	Window         int           `json:"window"`
	Round          int           `json:"round"`
	DealSerial     int           `json:"dealSerial"`
	DealtAt        int64         `json:"dealtAt"`
	BidStart       int           `json:"bidStart"`
	BidSteps       int           `json:"bidSteps"`
	Bid            int           `json:"bid"`
	Bids           []int         `json:"bids"`
	BidDone        []bool        `json:"bidDone"`
	Landlord       int           `json:"landlord"`
	LastPlay       *Play         `json:"lastPlay"`
	Table          []Play        `json:"table"`
	Passes         int           `json:"passes"`
	PlayCounts     []int         `json:"playCounts"`
	Multiplier     int           `json:"multiplier"`
	DoubleFactors  []int         `json:"doubleFactors"`
	DoubleReady    []bool        `json:"doubleReady"`
	LandlordFactor int           `json:"landlordFactor"`
	Ready          []bool        `json:"ready"`
	Due            []int64       `json:"due"`
	Scores         []int         `json:"scores"`
	RoundResults   []RoundResult `json:"roundResults"`
	Events         []Event       `json:"events"`
	EventSerial    int           `json:"eventSerial"`
	Logs           []string      `json:"logs"`
}

func (s *State) actors() []int {
	out := []int{}
	switch s.Phase {
	case "bid", "play", "redouble":
		out = append(out, s.Turn)
	case "double":
		for p := 0; p < s.Players; p++ {
			if p != s.Landlord && !s.DoubleReady[p] {
				out = append(out, p)
			}
		}
	case "round_end":
		for p := 0; p < s.Players; p++ {
			if !s.Ready[p] {
				out = append(out, p)
			}
		}
	}
	return out
}
func (s *State) canAct(p int) bool { return p >= 0 && p < s.Players && slices.Contains(s.actors(), p) }
func (s *State) nextWindow(now time.Time) {
	s.Window++
	for p := range s.Due {
		s.Due[p] = 0
	}
	for _, p := range s.actors() {
		s.Due[p] = now.UnixMilli() + int64(s.Options.TurnSeconds)*1000
	}
}
func (s *State) note(format string, args ...any) {
	s.Logs = append(s.Logs, fmt.Sprintf(format, args...))
	if len(s.Logs) > 60 {
		s.Logs = s.Logs[len(s.Logs)-60:]
	}
}
func (s *State) event(kind string, p int, cards []int, combo string, now time.Time) {
	s.EventSerial++
	s.Events = append(s.Events, Event{s.EventSerial, kind, p, append([]int(nil), cards...), combo, now.UnixMilli()})
	if len(s.Events) > 48 {
		s.Events = s.Events[len(s.Events)-48:]
	}
}
func randomN(n int) (int, error) {
	v, e := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if e != nil {
		return 0, e
	}
	return int(v.Int64()), nil
}
func (s *State) deal(now time.Time) error {
	size, handSize := 54, 17
	if s.Players == 4 {
		size, handSize = 108, 25
	}
	deck := make([]int, size)
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
	s.Hands = make([][]int, s.Players)
	for p := range s.Hands {
		s.Hands[p] = append([]int{}, deck[p*handSize:(p+1)*handSize]...)
		sortHand(s.Hands[p])
	}
	s.Bottom = append([]int{}, deck[s.Players*handSize:]...)
	s.Played = []int{}
	s.Table = []Play{}
	s.LastPlay = nil
	s.Passes = 0
	s.Bid = 0
	s.BidSteps = 0
	s.Landlord = -1
	s.Bids = make([]int, s.Players)
	s.BidDone = make([]bool, s.Players)
	s.PlayCounts = make([]int, s.Players)
	s.Multiplier = 1
	s.LandlordFactor = 1
	s.DoubleFactors = make([]int, s.Players)
	for i := range s.DoubleFactors {
		s.DoubleFactors[i] = 1
	}
	s.DoubleReady = make([]bool, s.Players)
	s.Ready = make([]bool, s.Players)
	s.Due = make([]int64, s.Players)
	s.BidStart = (s.BidStart + 1) % s.Players
	s.Turn = s.BidStart
	s.Phase = "bid"
	s.DealSerial++
	s.DealtAt = now.UnixMilli()
	s.nextWindow(now)
	s.event("deal", -1, nil, "", now)
	s.note("第 %d 副：%d 人%s，开始叫分", s.Round, s.Players, map[bool]string{true: "上海斗地主", false: "经典斗地主"}[s.Players == 4])
	return nil
}
func (s *State) startPlay(now time.Time) { s.Phase = "play"; s.Turn = s.Landlord; s.nextWindow(now) }
func (s *State) awardBottom(now time.Time) {
	s.Hands[s.Landlord] = append(s.Hands[s.Landlord], s.Bottom...)
	sortHand(s.Hands[s.Landlord])
	s.event("landlord", s.Landlord, s.Bottom, "", now)
	s.note("玩家 %d 以 %d 分成为地主，取得 %d 张底牌", s.Landlord+1, s.Bid, len(s.Bottom))
	if s.Options.PersonalDoubling {
		s.Phase = "double"
		s.nextWindow(now)
	} else {
		s.startPlay(now)
	}
}
func (s *State) finishRound(first int, now time.Time) {
	spring := ""
	lordWins := first == s.Landlord
	if lordWins {
		zero := true
		for p, c := range s.PlayCounts {
			if p != s.Landlord && c != 0 {
				zero = false
			}
		}
		if zero {
			spring = "spring"
		}
	} else if s.PlayCounts[s.Landlord] == 1 {
		spring = "counter_spring"
	}
	if spring != "" {
		s.Multiplier *= 2
	}
	winners := []int{}
	delta := make([]int, s.Players)
	for p := 0; p < s.Players; p++ {
		if p == s.Landlord {
			continue
		}
		points := s.Bid * s.Multiplier * s.DoubleFactors[p] * s.LandlordFactor
		if lordWins {
			points = -points
		} else {
			winners = append(winners, p)
		}
		delta[p] = points
		delta[s.Landlord] -= points
	}
	if lordWins {
		winners = append(winners, s.Landlord)
	}
	for p, v := range delta {
		s.Scores[p] += v
	}
	s.RoundResults = append(s.RoundResults, RoundResult{s.Round, s.Landlord, first, winners, s.Bid, s.Multiplier, spring, delta})
	s.event("round_end", first, nil, spring, now)
	s.note("第 %d 副结束，%s获胜", s.Round, map[bool]string{true: "地主", false: "农民方"}[lordWins])
	s.Phase = "round_end"
	s.Ready = make([]bool, s.Players)
	if s.Round >= s.Options.Rounds {
		s.Phase = "finished"
		s.event("finish", first, nil, "", now)
	}
	s.nextWindow(now)
}
func (s *State) envelope() (gameapi.Match, error) {
	if e := s.valid(); e != nil {
		return gameapi.Match{}, e
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	actors := s.actors()
	deadline := int64(0)
	timeouts := []int{}
	for _, p := range actors {
		d := s.Due[p]
		if deadline == 0 || d < deadline {
			deadline = d
		}
	}
	for _, p := range actors {
		if s.Due[p] == deadline {
			timeouts = append(timeouts, p)
		}
	}
	m := gameapi.Match{Schema: 1, Data: raw, Actors: actors, Attention: append([]int{}, actors...), TimeoutActors: timeouts, Deadline: deadline, Log: append([]string{}, s.Logs...)}
	if s.Phase == "finished" {
		ranks := make([]int, s.Players)
		winners := []int{}
		for p, score := range s.Scores {
			rank := 1
			for _, other := range s.Scores {
				if other > score {
					rank++
				}
			}
			ranks[p] = rank
			if rank == 1 {
				winners = append(winners, p)
			}
		}
		if s.Options.Rounds == 1 {
			winners = append([]int{}, s.RoundResults[0].Winners...)
			for p := range ranks {
				ranks[p] = len(winners) + 1
				if slices.Contains(winners, p) {
					ranks[p] = 1
				}
			}
		}
		details, _ := json.Marshal(map[string]any{"mode": s.Options.Mode, "ruleset": s.Ruleset, "rounds": s.RoundResults})
		m.Result = &gameapi.Result{Ranks: ranks, Winners: winners, Scores: append([]int{}, s.Scores...), Team: s.Options.Rounds == 1, Details: details}
	}
	return m, m.Validate(s.Players)
}
func decode(m gameapi.Match) (*State, error) {
	if m.Schema != 1 {
		return nil, errors.New("不支持的斗地主存档版本")
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
	bad := func() error { return errors.New("斗地主存档不完整或不一致") }
	n := s.Players
	if (n != 3 && n != 4) || s.Options.Rounds < 1 || s.Options.Rounds > 10 || s.Options.TurnSeconds < 10 || s.Options.TurnSeconds > 120 || s.Options.Mode != modeFor(n) || s.Ruleset != rulesetFor(n) || s.Round < 1 || s.Round > s.Options.Rounds || s.Window < 1 || s.DealSerial < 1 || s.Turn < 0 || s.Turn >= n || s.BidStart < 0 || s.BidStart >= n || s.Bid < 0 || s.Bid > 3 || s.BidSteps < 0 || s.BidSteps > n || s.Multiplier < 1 || s.Multiplier > 1<<25 || s.LandlordFactor < 1 || s.LandlordFactor > 2 || s.EventSerial < 0 {
		return bad()
	}
	if !slices.Contains([]string{"bid", "double", "redouble", "play", "round_end", "finished"}, s.Phase) {
		return bad()
	}
	if len(s.Hands) != n || len(s.Bids) != n || len(s.BidDone) != n || len(s.PlayCounts) != n || len(s.Scores) != n || len(s.Ready) != n || len(s.Due) != n || len(s.DoubleFactors) != n || len(s.DoubleReady) != n || len(s.Events) > 48 || len(s.Logs) > 60 {
		return bad()
	}
	limit, bottom := 54, 3
	if n == 4 {
		limit, bottom = 108, 8
	}
	if len(s.Bottom) != bottom {
		return bad()
	}
	seen := make([]bool, limit)
	consume := func(ids []int) bool {
		for _, id := range ids {
			if id < 0 || id >= limit || seen[id] {
				return false
			}
			seen[id] = true
		}
		return true
	}
	dealt := 17
	if n == 4 {
		dealt = 25
	}
	bidCount, bestBid, bidder := 0, 0, -1
	for p, b := range s.Bids {
		if b < 0 || b > 3 || (!s.BidDone[p] && b != 0) {
			return bad()
		}
		if s.BidDone[p] {
			bidCount++
		}
		if b > bestBid {
			bestBid, bidder = b, p
		}
	}
	if bidCount != s.BidSteps || bestBid != s.Bid || bidder != s.Landlord {
		return bad()
	}
	for p, h := range s.Hands {
		maximum := dealt
		if s.Phase != "bid" && p == s.Landlord {
			maximum += bottom
		}
		if (s.Phase == "bid" && len(h) != dealt) || len(h) > maximum || !consume(h) {
			return bad()
		}
	}
	if !consume(s.Played) {
		return bad()
	}
	if s.Phase == "bid" {
		if !consume(s.Bottom) || len(s.Played) != 0 {
			return bad()
		}
		if s.Landlord < -1 || s.Landlord >= n {
			return bad()
		}
	} else {
		if s.Landlord < 0 || s.Landlord >= n || s.Bid == 0 {
			return bad()
		}
		bottomSeen := map[int]bool{}
		for _, c := range s.Bottom {
			if c < 0 || c >= limit || bottomSeen[c] {
				return bad()
			}
			bottomSeen[c] = true
		}
	}
	for _, v := range seen {
		if !v {
			return bad()
		}
	}
	sum := 0
	for p, v := range s.Scores {
		if v < -(1<<45) || v > 1<<45 || s.DoubleFactors[p] < 1 || s.DoubleFactors[p] > 2 || s.PlayCounts[p] < 0 || s.PlayCounts[p] > 108 || s.Due[p] < 0 {
			return bad()
		}
		sum += v
	}
	if sum != 0 {
		return bad()
	}
	if (s.Phase == "double" || s.Phase == "redouble") && !s.Options.PersonalDoubling {
		return bad()
	}
	if s.Phase == "play" && len(s.Hands[s.Turn]) == 0 {
		return bad()
	}
	if s.Passes < 0 || s.Passes >= n {
		return bad()
	}
	if s.LastPlay != nil {
		c, e := classify(s.LastPlay.Cards, n, s.Options.AllowStraight)
		if e != nil || c != s.LastPlay.Combination || s.LastPlay.Seat < 0 || s.LastPlay.Seat >= n {
			return bad()
		}
		for _, id := range s.LastPlay.Cards {
			if !slices.Contains(s.Played, id) {
				return bad()
			}
		}
	}
	for _, p := range s.actors() {
		if s.Due[p] <= 0 {
			return bad()
		}
	}
	expected := s.Round - 1
	if s.Phase == "round_end" || s.Phase == "finished" {
		expected = s.Round
	}
	if len(s.RoundResults) != expected {
		return bad()
	}
	if s.Phase == "finished" && s.Round != s.Options.Rounds {
		return bad()
	}
	return nil
}
func modeFor(n int) string {
	if n == 4 {
		return "shanghai4"
	}
	return "classic3"
}
func rulesetFor(n int) string {
	if n == 4 {
		return "doudizhu-shanghai-v1"
	}
	return "doudizhu-classic-v1"
}
