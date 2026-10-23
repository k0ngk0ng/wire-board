package doudizhu

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"
)

const huanleRuleset = "doudizhu-shanghai-huanle-v2"

func (s *State) isHuanle() bool { return s.Ruleset == huanleRuleset }
func naturalRank(id int) int {
	b := id % 54
	if b >= 52 {
		return b - 36
	}
	return b%13 + 2
}
func runHigh(rs []int) (int, bool) {
	if len(rs) < 2 {
		return 0, false
	}
	rs = append([]int{}, rs...)
	slices.Sort(rs)
	adjacent := func(a []int) bool {
		for i := 1; i < len(a); i++ {
			if a[i] != a[i-1]+1 {
				return false
			}
		}
		return true
	}
	if adjacent(rs) {
		return rs[len(rs)-1], true
	}
	if rs[len(rs)-1] == 14 {
		rs = append([]int{1}, rs[:len(rs)-1]...)
		if adjacent(rs) {
			return rs[len(rs)-1], true
		}
	}
	return 0, false
}
func classifyHuanle(ids []int) (Combination, error) {
	bad := func() (Combination, error) {
		return Combination{}, errors.New("这些牌不符合上海斗地主牌型；飞机带的对子也须连续")
	}
	if len(ids) == 0 || len(ids) > 33 {
		return bad()
	}
	seen := map[int]bool{}
	counts := map[int]int{}
	for _, id := range ids {
		if id < 0 || id >= 108 || seen[id] {
			return bad()
		}
		seen[id] = true
		counts[naturalRank(id)]++
	}
	if c, e := classify(ids, 4, false); e == nil && slices.Contains([]string{"single", "pair", "triple", "trio_pair", "bomb", "rocket"}, c.Kind) {
		return c, nil
	}
	keys := []int{}
	for r := range counts {
		if r >= 16 {
			return bad()
		}
		keys = append(keys, r)
	}
	if high, ok := runHigh(keys); ok {
		for size, kind := range map[int]string{1: "straight", 2: "pairs", 3: "triples"} {
			minimum := 3
			if size == 1 {
				minimum = 5
			} else if size == 3 {
				minimum = 2
			}
			if len(keys) < minimum {
				continue
			}
			all := true
			for _, r := range keys {
				all = all && counts[r] == size
			}
			if all {
				return Combination{kind, high, len(ids), len(keys)}, nil
			}
		}
	}
	body, wings := []int{}, []int{}
	for r, n := range counts {
		switch n {
		case 3:
			body = append(body, r)
		case 2:
			wings = append(wings, r)
		default:
			return bad()
		}
	}
	if len(body) < 2 || len(body) != len(wings) {
		return bad()
	}
	hi, ok := runHigh(body)
	_, wingOK := runHigh(wings)
	if !ok || !wingOK {
		return bad()
	}
	return Combination{"plane_pair", hi, len(ids), len(body)}, nil
}
func (s *State) classify(ids []int) (Combination, error) {
	if s.isHuanle() {
		return classifyHuanle(ids)
	}
	return classify(ids, s.Players, s.Options.AllowStraight)
}
func orderedHuanlePlay(ids []int, c Combination) []int {
	if !slices.Contains([]string{"straight", "pairs", "triples", "plane_pair"}, c.Kind) {
		return orderedPlay(ids, c)
	}
	out := append([]int{}, ids...)
	counts := map[int]int{}
	for _, id := range ids {
		counts[naturalRank(id)]++
	}
	wings := []int{}
	for r, n := range counts {
		if n == 2 {
			wings = append(wings, r)
		}
	}
	wingHi, _ := runHigh(wings)
	body := func(id int) bool { return c.Kind != "plane_pair" || counts[naturalRank(id)] == 3 }
	value := func(id int) int {
		n := naturalRank(id)
		hi := c.High
		if !body(id) {
			hi = wingHi
		}
		if n == 14 && hi < 14 {
			return 1
		}
		return n
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if body(a) != body(b) {
			return body(a)
		}
		if value(a) != value(b) {
			return value(a) > value(b)
		}
		if a%54 != b%54 {
			return a%54 < b%54
		}
		return a < b
	})
	return out
}
func (s *State) ordered(ids []int, c Combination) []int {
	if s.isHuanle() {
		return orderedHuanlePlay(ids, c)
	}
	return orderedPlay(ids, c)
}
func movesForHuanle(hand []int) []Move {
	result := []Move{}
	seen := map[string]bool{}
	groups := map[int][]int{}
	for _, id := range hand {
		groups[naturalRank(id)] = append(groups[naturalRank(id)], id)
	}
	for r := range groups {
		slices.Sort(groups[r])
	}
	add := func(ids []int) {
		copy := append([]int{}, ids...)
		slices.Sort(copy)
		key := fmt.Sprint(copy)
		if seen[key] {
			return
		}
		c, e := classifyHuanle(copy)
		if e != nil {
			return
		}
		seen[key] = true
		result = append(result, Move{orderedHuanlePlay(copy, c), c})
	}
	for _, m := range movesFor(hand, 4, false) {
		add(m.Cards)
	}
	build := func(start, units, count int) ([]int, bool) {
		out := []int{}
		for v := start; v < start+units; v++ {
			r := v
			if r == 1 {
				r = 14
			}
			if len(groups[r]) < count {
				return nil, false
			}
			out = append(out, groups[r][:count]...)
		}
		return out, true
	}
	for size := 1; size <= 3; size++ {
		minimum := 3
		if size == 1 {
			minimum = 5
		} else if size == 3 {
			minimum = 2
		}
		for units := minimum; units <= min(13, len(hand)/size); units++ {
			for start := 1; start+units-1 <= 14; start++ {
				if ids, ok := build(start, units, size); ok {
					add(ids)
				}
			}
		}
	}
	for units := 2; units <= len(hand)/5; units++ {
		for start := 1; start+units-1 <= 14; start++ {
			body, ok := build(start, units, 3)
			if !ok {
				continue
			}
			for wingStart := 1; wingStart+units-1 <= 14; wingStart++ {
				wing, ok := build(wingStart, units, 2)
				if ok {
					add(append(append([]int{}, body...), wing...))
				}
			}
		}
	}
	return result
}
func (s *State) legalMoves(p int) []Move {
	candidates := []Move{}
	if s.isHuanle() {
		candidates = movesForHuanle(s.Hands[p])
	} else {
		candidates = movesFor(s.Hands[p], s.Players, s.Options.AllowStraight)
	}
	out := []Move{}
	for _, m := range candidates {
		if (m.Kind == "bomb" || m.Kind == "rocket") && !s.canBomb(p) {
			continue
		}
		if s.LastPlay == nil || beats(m.Combination, s.LastPlay.Combination) {
			out = append(out, m)
		}
	}
	sortHints(out, s.Hands[p], s.Players)
	return out
}
func (s *State) bombLimit(p int) int {
	if !s.isHuanle() || p == s.Landlord || !s.BidDone[p] {
		return -1
	}
	return max(1, s.Bids[p])
}
func (s *State) bombsUsed(p int) int {
	if len(s.BombCounts) != s.Players {
		return 0
	}
	return s.BombCounts[p]
}
func (s *State) canBomb(p int) bool {
	limit := s.bombLimit(p)
	return limit < 0 || s.bombsUsed(p) < limit
}
func (s *State) reportCandidates() [][]int {
	out := [][]int{}
	if s.Landlord < 0 {
		return out
	}
	byRank := map[int][]int{}
	jokers := []int{}
	for _, id := range s.Hands[s.Landlord] {
		r := rank(id)
		if r >= 16 {
			jokers = append(jokers, id)
		} else {
			byRank[r] = append(byRank[r], id)
		}
	}
	if len(jokers) == 4 {
		sortHand(jokers)
		out = append(out, jokers)
	}
	for r := 15; r >= 3; r-- {
		if len(byRank[r]) >= 7 {
			sortHand(byRank[r])
			out = append(out, byRank[r])
		}
	}
	return out
}
func (s *State) huanleTransfer(lordWins bool) int {
	head := 1
	if s.TouLiao && lordWins {
		head = 4
	}
	reports := len(s.Reports)
	if reports > 0 && !s.ReportPlay {
		return s.Bid * reports * head
	}
	if lordWins {
		return s.Bid * (s.Multiplier + reports) * head
	}
	return s.Bid * (reports - s.Multiplier)
}
func (s *State) chooseLandlord(a Action, now time.Time) error {
	if a.OpenHand && !s.TouLiao {
		return errors.New("只有头撩的地主可以摊打")
	}
	candidates := s.reportCandidates()
	if len(a.Reports) > 2 {
		return errors.New("最多选择双报到")
	}
	seen := map[int]bool{}
	chosen := [][]int{}
	for _, index := range a.Reports {
		if index < 0 || index >= len(candidates) || seen[index] {
			return errors.New("请选择手中不同的四王或七张以上炸弹报到")
		}
		seen[index] = true
		chosen = append(chosen, append([]int{}, candidates[index]...))
	}
	s.OpenHand = a.OpenHand
	s.Reports = chosen
	s.ReportPlay = a.Continue && len(chosen) > 0
	if s.OpenHand {
		s.Multiplier *= 2
		s.event("open_hand", s.Landlord, s.Hands[s.Landlord], "", now)
		s.note("玩家 %d 选择摊打，手牌公开", s.Landlord+1)
	}
	for _, cards := range chosen {
		s.event("report", s.Landlord, cards, "", now)
	}
	if len(chosen) > 0 {
		s.note("玩家 %d %s报到，选择%s", s.Landlord+1, map[bool]string{true: "双", false: "单"}[len(chosen) == 2], map[bool]string{true: "继续打", false: "不打"}[s.ReportPlay])
	}
	if len(chosen) > 0 && !s.ReportPlay {
		s.finishRound(s.Landlord, now)
	} else {
		s.startPlay(now)
	}
	return nil
}
