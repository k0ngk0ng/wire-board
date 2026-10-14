package guandan

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
	Rounds      int `json:"rounds"`
	StartLevel  int `json:"startLevel"`
	TurnSeconds int `json:"turnSeconds"`
}
type Play struct {
	Seat  int   `json:"seat"`
	Cards []int `json:"cards"`
	Pass  bool  `json:"pass"`
	Combination
}
type Exchange struct {
	From     int `json:"from"`
	To       int `json:"to"`
	Given    int `json:"given"`
	Returned int `json:"returned"`
}
type Reveal struct {
	Seat   int    `json:"seat"`
	Cards  []int  `json:"cards"`
	Reason string `json:"reason"`
}
type Preview struct {
	Seat            int           `json:"seat"`
	Cards           []int         `json:"cards"`
	Interpretations []Combination `json:"interpretations"`
}
type Event struct {
	Value   int    `json:"value,omitempty"`
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Seat    int    `json:"seat"`
	Target  int    `json:"target"`
	Cards   []int  `json:"cards,omitempty"`
	Private bool   `json:"private,omitempty"`
	Kind    string `json:"kind,omitempty"`
	At      int64  `json:"at"`
}
type RoundResult struct {
	Round        int   `json:"round"`
	Level        int   `json:"level"`
	Team         int   `json:"team"`
	Head         int   `json:"head"`
	PartnerPlace int   `json:"partnerPlace"`
	Promotion    int   `json:"promotion"`
	Order        []int `json:"order"`
	Ranks        []int `json:"ranks"`
	DoubleDown   bool  `json:"doubleDown"`
	Levels       []int `json:"levels"`
}
type State struct {
	Ruleset      string        `json:"ruleset"`
	Options      Options       `json:"options"`
	Hands        [][]int       `json:"hands"`
	Played       []int         `json:"played"`
	Phase        string        `json:"phase"`
	Turn         int           `json:"turn"`
	Window       int           `json:"window"`
	Round        int           `json:"round"`
	DealSerial   int           `json:"dealSerial"`
	DealtAt      int64         `json:"dealtAt"`
	Levels       []int         `json:"levels"`
	Level        int           `json:"level"`
	DealerTeam   int           `json:"dealerTeam"`
	FinishOrder  []int         `json:"finishOrder"`
	LastPlay     *Play         `json:"lastPlay"`
	Preview      *Preview      `json:"preview,omitempty"`
	Table        []Play        `json:"table"`
	Passes       int           `json:"passes"`
	Exchanges    []Exchange    `json:"exchanges"`
	Revealed     []Reveal      `json:"revealed"`
	AntiTribute  bool          `json:"antiTribute"`
	OpeningSeat  int           `json:"openingSeat"`
	Ready        []bool        `json:"ready"`
	Due          []int64       `json:"due"`
	Results      []RoundResult `json:"roundResults"`
	TieBreak     bool          `json:"tieBreak"`
	WinnerTeam   int           `json:"winnerTeam"`
	FinishReason string        `json:"finishReason"`
	Events       []Event       `json:"events"`
	EventSerial  int           `json:"eventSerial"`
	Logs         []string      `json:"logs"`
}

func randomN(n int) (int, error) {
	x, e := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if e != nil {
		return 0, e
	}
	return int(x.Int64()), nil
}
func (s *State) note(format string, args ...any) {
	s.Logs = append(s.Logs, fmt.Sprintf(format, args...))
	if len(s.Logs) > 60 {
		s.Logs = s.Logs[len(s.Logs)-60:]
	}
}
func (s *State) event(kind string, seat, target int, cards []int, private bool, combo string, now time.Time) {
	s.EventSerial++
	value := 0
	if kind == "play" && s.LastPlay != nil {
		value = s.LastPlay.High
		if value == 15 {
			value = s.Level
		}
	}
	s.Events = append(s.Events, Event{ID: s.EventSerial, Type: kind, Seat: seat, Target: target, Cards: append([]int{}, cards...), Private: private, Kind: combo, At: now.UnixMilli(), Value: value})
	if len(s.Events) > 60 {
		s.Events = s.Events[len(s.Events)-60:]
	}
}
func (s *State) actors() []int {
	out := []int{}
	switch s.Phase {
	case "play":
		out = append(out, s.Turn)
	case "tribute":
		for _, e := range s.Exchanges {
			if e.Given < 0 {
				out = append(out, e.From)
			}
		}
	case "return":
		for _, e := range s.Exchanges {
			if e.Returned < 0 {
				out = append(out, e.To)
			}
		}
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
	s.Preview = nil
	s.Window++
	for p := range s.Due {
		s.Due[p] = 0
	}
	for _, p := range s.actors() {
		s.Due[p] = now.UnixMilli() + int64(s.Options.TurnSeconds)*1000
	}
}
func (s *State) nextLive(after int) int {
	for i := 1; i <= 4; i++ {
		p := (after + i) % 4
		if len(s.Hands[p]) > 0 {
			return p
		}
	}
	return -1
}
func (s *State) responders() int {
	n := 0
	for p, h := range s.Hands {
		if len(h) > 0 && (s.LastPlay == nil || p != s.LastPlay.Seat) {
			n++
		}
	}
	return n
}
func (s *State) deal(now time.Time) error {
	deck := make([]int, 108)
	for i := range deck {
		deck[i] = i
	}
	for i := 107; i > 0; i-- {
		j, e := randomN(i + 1)
		if e != nil {
			return e
		}
		deck[i], deck[j] = deck[j], deck[i]
	}
	if s.DealerTeam >= 0 {
		s.Level = s.Levels[s.DealerTeam]
	} else {
		s.Level = s.Options.StartLevel
	}
	s.Hands = make([][]int, 4)
	for p := 0; p < 4; p++ {
		s.Hands[p] = append([]int{}, deck[p*27:(p+1)*27]...)
		sortHand(s.Hands[p], s.Level)
	}
	s.Played = []int{}
	s.FinishOrder = []int{}
	s.LastPlay = nil
	s.Table = []Play{}
	s.Passes = 0
	s.Exchanges = []Exchange{}
	s.Revealed = []Reveal{}
	s.AntiTribute = false
	s.Ready = make([]bool, 4)
	s.Due = make([]int64, 4)
	s.DealSerial++
	s.DealtAt = now.UnixMilli()
	s.event("deal", -1, -1, nil, false, "", now)
	s.note("第 %d 副，打 %s，红心%s为配牌", s.Round, levelName(s.Level), levelName(s.Level))
	if s.Round == 1 {
		eligible := []int{}
		for _, id := range deck {
			if natural(id) < 16 && !wild(id, s.Level) {
				eligible = append(eligible, id)
			}
		}
		index, e := randomN(len(eligible))
		if e != nil {
			return e
		}
		flipped := eligible[index]
		owner := 0
		for p, h := range s.Hands {
			if slices.Contains(h, flipped) {
				owner = p
			}
		}
		s.OpeningSeat = owner
		s.Turn = owner
		s.Phase = "play"
		s.Revealed = append(s.Revealed, Reveal{owner, []int{flipped}, "首副亮牌"})
		s.nextWindow(now)
		return nil
	}
	last := s.Results[len(s.Results)-1]
	givers := []int{}
	if last.DoubleDown {
		for p := 0; p < 4; p++ {
			if p%2 != last.Team {
				givers = append(givers, p)
			}
		}
	} else {
		for p, r := range last.Ranks {
			if r == 4 {
				givers = append(givers, p)
			}
		}
	}
	red := []Reveal{}
	redCount := 0
	for _, p := range givers {
		ids := []int{}
		for _, id := range s.Hands[p] {
			if natural(id) == 17 {
				ids = append(ids, id)
				redCount++
			}
		}
		if len(ids) > 0 {
			red = append(red, Reveal{p, ids, "抗贡明示"})
		}
	}
	if redCount == 2 {
		s.AntiTribute = true
		s.Revealed = red
		s.OpeningSeat = last.Head
		s.Turn = last.Head
		s.Phase = "play"
		for _, r := range red {
			s.event("anti_tribute", r.Seat, -1, r.Cards, false, "", now)
		}
		s.note("应贡方持有两个大王，抗贡，由上游先出")
		s.nextWindow(now)
		return nil
	}
	for _, p := range givers {
		s.Exchanges = append(s.Exchanges, Exchange{From: p, To: -1, Given: -1, Returned: -1})
	}
	s.Phase = "tribute"
	s.Turn = givers[0]
	s.nextWindow(now)
	return nil
}
func (s *State) tributeChoices(p int) []int {
	best := -1
	out := []int{}
	for _, id := range s.Hands[p] {
		if wild(id, s.Level) {
			continue
		}
		r := strength(natural(id), s.Level)
		if r > best {
			best = r
			out = []int{id}
		} else if r == best {
			out = append(out, id)
		}
	}
	return out
}
func (s *State) returnChoices(p int) []int {
	incoming := -1
	for _, x := range s.Exchanges {
		if x.To == p {
			incoming = x.Given
		}
	}
	out := []int{}
	fallback := []int{}
	least := 100
	for _, id := range s.Hands[p] {
		if id == incoming || wild(id, s.Level) {
			continue
		}
		r := strength(natural(id), s.Level)
		if r <= 10 {
			out = append(out, id)
		}
		if r < least {
			least = r
			fallback = []int{id}
		} else if r == least {
			fallback = append(fallback, id)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
func (s *State) settleTributes(now time.Time) {
	last := s.Results[len(s.Results)-1]
	if len(s.Exchanges) == 1 {
		s.Exchanges[0].To = last.Head
		s.OpeningSeat = s.Exchanges[0].From
	} else {
		a, b := &s.Exchanges[0], &s.Exchanges[1]
		ra, rb := strength(natural(a.Given), s.Level), strength(natural(b.Given), s.Level)
		if ra == rb {
			a.To = (a.From + 3) % 4
			b.To = (b.From + 3) % 4
		} else if ra > rb {
			a.To = last.Head
			b.To = (last.Head + 2) % 4
		} else {
			b.To = last.Head
			a.To = (last.Head + 2) % 4
		}
		if a.To == last.Head {
			s.OpeningSeat = a.From
		} else {
			s.OpeningSeat = b.From
		}
	}
	for _, x := range s.Exchanges {
		s.Hands[x.From] = slices.DeleteFunc(s.Hands[x.From], func(id int) bool { return id == x.Given })
		s.Hands[x.To] = append(s.Hands[x.To], x.Given)
		sortHand(s.Hands[x.To], s.Level)
		s.event("tribute", x.From, x.To, []int{x.Given}, false, "", now)
		s.note("玩家 %d 向玩家 %d 进贡", x.From+1, x.To+1)
	}
	s.Phase = "return"
	s.nextWindow(now)
}
func (s *State) roundComplete() bool {
	if len(s.FinishOrder) >= 3 {
		return true
	}
	return len(s.FinishOrder) == 2 && s.FinishOrder[0]%2 == s.FinishOrder[1]%2
}
func (s *State) finishRound(now time.Time) {
	head := s.FinishOrder[0]
	team := head % 2
	mate := (head + 2) % 4
	ranks := make([]int, 4)
	for i, p := range s.FinishOrder {
		ranks[p] = i + 1
	}
	double := len(s.FinishOrder) == 2
	for p := range ranks {
		if ranks[p] == 0 {
			if double {
				ranks[p] = 3
			} else {
				ranks[p] = 4
			}
		}
	}
	partnerPlace := ranks[mate]
	promotion := 4 - partnerPlace + 1 // 2 -> 3, 3 -> 2, 4 -> 1
	passedAce := s.Level == 14 && (s.DealerTeam == team || s.DealerTeam < 0) && partnerPlace <= 3
	s.Levels[team] = min(14, s.Levels[team]+promotion)
	s.Results = append(s.Results, RoundResult{s.Round, s.Level, team, head, partnerPlace, promotion, append([]int{}, s.FinishOrder...), ranks, double, append([]int{}, s.Levels...)})
	s.event("round_end", head, mate, nil, false, "", now)
	s.note("上游玩家 %d，同伴第 %d 位，己方升 %d 级", head+1, partnerPlace, promotion)
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
	s.DealerTeam = team
	s.Ready = make([]bool, 4)
	s.Phase = "round_end"
	if s.WinnerTeam >= 0 {
		s.Phase = "finished"
		s.event("finish", head, -1, nil, false, s.FinishReason, now)
	}
	s.nextWindow(now)
}
func (s *State) envelope() (gameapi.Match, error) {
	if e := s.valid(); e != nil {
		return gameapi.Match{}, e
	}
	b, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	a := s.actors()
	deadline := int64(0)
	for _, p := range a {
		if deadline == 0 || s.Due[p] < deadline {
			deadline = s.Due[p]
		}
	}
	ta := []int{}
	for _, p := range a {
		if s.Due[p] == deadline {
			ta = append(ta, p)
		}
	}
	m := gameapi.Match{Schema: 1, Data: b, Actors: a, Attention: append([]int{}, a...), TimeoutActors: ta, Deadline: deadline, Log: append([]string{}, s.Logs...)}
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
		m.Result = &gameapi.Result{Ranks: ranks, Winners: winners, Scores: scores, Team: true, Details: details}
	}
	return m, m.Validate(4)
}
func decode(m gameapi.Match) (*State, error) {
	if m.Schema != 1 {
		return nil, errors.New("不支持的掼蛋存档版本")
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
	bad := func() error { return errors.New("掼蛋存档不完整或不一致") }
	if s.Ruleset != "guandan-jiangsu-2017-v1" || s.Options.Rounds < 0 || s.Options.Rounds > 24 || s.Options.StartLevel < 2 || s.Options.StartLevel > 14 || s.Options.TurnSeconds < 10 || s.Options.TurnSeconds > 120 || s.Round < 1 || s.Round > 10000 || s.Window < 1 || s.DealSerial < 1 || len(s.Hands) != 4 || len(s.Levels) != 2 || s.Level < 2 || s.Level > 14 || s.DealerTeam < -1 || s.DealerTeam > 1 || s.Turn < 0 || s.Turn > 3 || s.OpeningSeat < 0 || s.OpeningSeat > 3 || len(s.Ready) != 4 || len(s.Due) != 4 || len(s.Events) > 60 || len(s.Logs) > 60 || s.Passes < 0 || s.Passes > 3 {
		return bad()
	}
	if !slices.Contains([]string{"play", "tribute", "return", "round_end", "finished"}, s.Phase) {
		return bad()
	}
	for _, l := range s.Levels {
		if l < 2 || l > 14 {
			return bad()
		}
	}
	seen := make([]bool, 108)
	consume := func(ids []int) bool {
		for _, id := range ids {
			if id < 0 || id >= 108 || seen[id] {
				return false
			}
			seen[id] = true
		}
		return true
	}
	for _, h := range s.Hands {
		if len(h) > 28 || !consume(h) {
			return bad()
		}
	}
	if !consume(s.Played) {
		return bad()
	}
	for _, v := range seen {
		if !v {
			return bad()
		}
	}
	finish := map[int]bool{}
	for _, p := range s.FinishOrder {
		if p < 0 || p > 3 || finish[p] || len(s.Hands[p]) != 0 {
			return bad()
		}
		finish[p] = true
	}
	if len(s.FinishOrder) > 3 {
		return bad()
	}
	if s.Phase == "play" && (len(s.Hands[s.Turn]) == 0 || s.roundComplete()) {
		return bad()
	}
	expected := s.Round - 1
	if s.Phase == "round_end" || s.Phase == "finished" {
		expected = s.Round
	}
	if len(s.Results) != expected {
		return bad()
	}
	if (s.Phase == "tribute" || s.Phase == "return") && (s.Round == 1 || len(s.Exchanges) < 1 || len(s.Exchanges) > 2 || len(s.Played) != 0) {
		return bad()
	}
	froms, tos := map[int]bool{}, map[int]bool{}
	for _, x := range s.Exchanges {
		if x.From < 0 || x.From > 3 || x.To < -1 || x.To > 3 || x.Given < -1 || x.Given > 107 || x.Returned < -1 || x.Returned > 107 || froms[x.From] {
			return bad()
		}
		froms[x.From] = true
		if s.Phase == "tribute" {
			if x.Given >= 0 && !slices.Contains(s.Hands[x.From], x.Given) {
				return bad()
			}
		} else {
			if x.To < 0 || x.To == x.From || x.Given < 0 || tos[x.To] {
				return bad()
			}
			tos[x.To] = true
			if s.Phase == "return" && x.Returned < 0 && !slices.Contains(s.Hands[x.To], x.Given) {
				return bad()
			}
		}
	}
	if s.LastPlay != nil {
		if s.LastPlay.Seat < 0 || s.LastPlay.Seat > 3 {
			return bad()
		}
		cs, e := interpret(s.LastPlay.Cards, s.Level)
		if e != nil {
			return bad()
		}
		ok := false
		for _, c := range cs {
			if reflect.DeepEqual(c, s.LastPlay.Combination) {
				ok = true
			}
		}
		if !ok {
			return bad()
		}
		for _, id := range s.LastPlay.Cards {
			if !slices.Contains(s.Played, id) {
				return bad()
			}
		}
	}
	if s.Preview != nil {
		v := s.Preview
		if s.Phase != "play" || v.Seat != s.Turn || v.Seat < 0 || v.Seat > 3 {
			return bad()
		}
		for _, id := range v.Cards {
			if !slices.Contains(s.Hands[v.Seat], id) {
				return bad()
			}
		}
		cs, e := interpret(v.Cards, s.Level)
		if e != nil {
			return bad()
		}
		legal := []Combination{}
		for _, c := range cs {
			if s.LastPlay == nil || beats(c, s.LastPlay.Combination) {
				legal = append(legal, c)
			}
		}
		if len(legal) == 0 || !reflect.DeepEqual(legal, v.Interpretations) {
			return bad()
		}
	}
	for _, p := range s.actors() {
		if s.Due[p] <= 0 {
			return bad()
		}
	}
	if s.Phase == "finished" {
		if s.WinnerTeam < 0 || s.WinnerTeam > 1 || s.FinishReason == "" {
			return bad()
		}
	}
	return nil
}
func levelName(level int) string {
	if level >= 11 {
		return map[int]string{11: "J", 12: "Q", 13: "K", 14: "A"}[level]
	}
	return fmt.Sprint(level)
}
