package shengji

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

type Rules struct {
	Decks int `json:"decks"`
	Level int `json:"level"`
	Trump int `json:"trump"`
}
type Part struct {
	Kind  string `json:"kind"`
	Cards []int  `json:"cards"`
	High  int    `json:"high"`
}
type Shape struct {
	Suit  int    `json:"suit"`
	Count int    `json:"count"`
	Four  bool   `json:"four"`
	Parts []Part `json:"parts"`
}

func rank(id int) int {
	b := id % 54
	if b >= 52 {
		return b - 36
	}
	return b%13 + 2
}
func printedSuit(id int) int {
	if id%54 >= 52 {
		return -1
	}
	return id % 54 / 13
}
func suitOf(id int, r Rules) int {
	if rank(id) >= 16 || rank(id) == r.Level || printedSuit(id) == r.Trump {
		return 4
	}
	return printedSuit(id)
}
func power(id int, r Rules) int {
	n := rank(id)
	if r.Trump < 0 {
		if n == 17 {
			return 14
		}
		if n == 16 {
			return 13
		}
		if n == r.Level {
			return 12
		}
	}
	if n == 17 {
		return 15
	}
	if n == 16 {
		return 14
	}
	if n == r.Level {
		if printedSuit(id) == r.Trump {
			return 13
		}
		return 12
	}
	p := n - 2
	if n > r.Level {
		p--
	}
	return p
}
func points(ids []int) int {
	n := 0
	for _, id := range ids {
		switch rank(id) {
		case 5:
			n += 5
		case 10, 13:
			n += 10
		}
	}
	return n
}
func sortHand(ids []int, r Rules) {
	sort.Slice(ids, func(i, j int) bool {
		a, b := ids[i], ids[j]
		if suitOf(a, r) != suitOf(b, r) {
			return suitOf(a, r) > suitOf(b, r)
		}
		if power(a, r) != power(b, r) {
			return power(a, r) > power(b, r)
		}
		if a%54 != b%54 {
			return a%54 < b%54
		}
		return a < b
	})
}
func validIDs(ids []int, decks int) bool {
	seen := map[int]bool{}
	for _, id := range ids {
		if id < 0 || id >= 54*decks || seen[id] {
			return false
		}
		seen[id] = true
	}
	return len(ids) > 0
}
func owns(h, ids []int) bool {
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] || !slices.Contains(h, id) {
			return false
		}
		seen[id] = true
	}
	return len(ids) > 0
}
func isFour(ids []int, r Rules) bool {
	if r.Decks != 1 || len(ids) != 4 || rank(ids[0]) >= 16 {
		return false
	}
	for _, id := range ids {
		if rank(id) != rank(ids[0]) {
			return false
		}
	}
	return validIDs(ids, 1)
}

type pairNode struct {
	IDs   []int
	Power int
}

func pairsIn(ids []int, r Rules) []pairNode {
	by := map[int][]int{}
	for _, id := range ids {
		by[id%54] = append(by[id%54], id)
	}
	out := []pairNode{}
	for _, v := range by {
		if len(v) == 2 {
			slices.Sort(v)
			out = append(out, pairNode{v, power(v[0], r)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Power != out[j].Power {
			return out[i].Power < out[j].Power
		}
		return out[i].IDs[0] < out[j].IDs[0]
	})
	return out
}
func chains(nodes []pairNode, mask uint64, n int) []uint64 {
	out := []uint64{}
	var walk func(uint64, int, int)
	walk = func(path uint64, last, remaining int) {
		if remaining == 0 {
			out = append(out, path)
			return
		}
		for j := last + 1; j < len(nodes); j++ {
			if mask&(1<<j) != 0 && nodes[j].Power == nodes[last].Power+1 {
				walk(path|1<<j, j, remaining-1)
			}
		}
	}
	for i := range nodes {
		if mask&(1<<i) != 0 {
			walk(1<<i, i, n-1)
		}
	}
	return out
}
func maskIDs(nodes []pairNode, mask uint64) []int {
	ids := []int{}
	for i, n := range nodes {
		if mask&(1<<i) != 0 {
			ids = append(ids, n.IDs...)
		}
	}
	return ids
}
func leadShape(ids []int, r Rules) (Shape, error) {
	bad := errors.New("请选择同一门的牌；对子必须是同花同点的两张实体牌")
	if !validIDs(ids, r.Decks) {
		return Shape{}, bad
	}
	if isFour(ids, r) {
		hi := rank(ids[0])
		if hi == r.Level {
			hi = 15
		}
		return Shape{Suit: -1, Count: 4, Four: true, Parts: []Part{{"four", append([]int{}, ids...), hi}}}, nil
	}
	s := Shape{Suit: suitOf(ids[0], r), Count: len(ids), Parts: []Part{}}
	for _, id := range ids {
		if suitOf(id, r) != s.Suit {
			return Shape{}, bad
		}
	}
	nodes := pairsIn(ids, r)
	mask := uint64(1)<<len(nodes) - 1
	used := map[int]bool{}
	for length := len(nodes); length >= 2; length-- {
		for {
			cs := chains(nodes, mask, length)
			if len(cs) == 0 {
				break
			}
			m := cs[0]
			csIDs := maskIDs(nodes, m)
			hi := 0
			for _, id := range csIDs {
				hi = max(hi, power(id, r))
				used[id] = true
			}
			s.Parts = append(s.Parts, Part{"tractor", csIDs, hi})
			mask &^= m
		}
	}
	for i, node := range nodes {
		if mask&(1<<i) != 0 {
			s.Parts = append(s.Parts, Part{"pair", node.IDs, node.Power})
			for _, id := range node.IDs {
				used[id] = true
			}
		}
	}
	for _, id := range ids {
		if !used[id] {
			s.Parts = append(s.Parts, Part{"single", []int{id}, power(id, r)})
		}
	}
	sort.Slice(s.Parts, func(i, j int) bool {
		a, b := s.Parts[i], s.Parts[j]
		if len(a.Cards) != len(b.Cards) {
			return len(a.Cards) > len(b.Cards)
		}
		if a.High != b.High {
			return a.High > b.High
		}
		return a.Cards[0] < b.Cards[0]
	})
	return s, nil
}
func demands(s Shape) []int {
	out := []int{}
	for _, p := range s.Parts {
		if len(p.Cards) >= 2 {
			out = append(out, len(p.Cards)/2)
		}
	}
	return out
}

type allocation struct {
	Profile []int
	Mask    uint64
	Cost    int
}

func compare(a, b []int) int {
	for i := 0; i < min(len(a), len(b)); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return len(a) - len(b)
}

// Maximize the compulsory structure, longest required tractor first. When a
// complete tractor is unavailable, preserve the longest partial tractors and
// then all available pairs. Memoized pair allocation avoids enumerating hands.
func allocate(nodes []pairNode, required []int) allocation {
	memo := map[string]allocation{}
	var solve func(uint64, []int) allocation
	solve = func(mask uint64, req []int) allocation {
		if len(req) == 0 {
			return allocation{Profile: []int{}}
		}
		key := fmt.Sprint(mask, "/", req)
		if v, ok := memo[key]; ok {
			return v
		}
		maxLen := min(req[0], len(nodes))
		cs := []uint64{}
		for ; maxLen > 0; maxLen-- {
			cs = chains(nodes, mask, maxLen)
			if len(cs) > 0 {
				break
			}
		}
		if maxLen == 0 {
			n := 0
			for _, v := range req {
				n += v
			}
			return allocation{Profile: make([]int, n)}
		}
		rest := append([]int{}, req[1:]...)
		if maxLen < req[0] {
			rest = append([]int{req[0] - maxLen}, rest...)
		}
		best := allocation{Cost: 1 << 30}
		for _, cm := range cs {
			next := solve(mask&^cm, rest)
			profile := append([]int{maxLen}, next.Profile...)
			cost := next.Cost
			for i, n := range nodes {
				if cm&(1<<i) != 0 {
					cost += n.Power
				}
			}
			if best.Profile == nil || compare(profile, best.Profile) > 0 || (compare(profile, best.Profile) == 0 && cost < best.Cost) {
				best = allocation{profile, cm | next.Mask, cost}
			}
		}
		memo[key] = best
		return best
	}
	return solve(uint64(1)<<len(nodes)-1, required)
}
func ofSuit(h []int, suit int, r Rules) []int {
	out := []int{}
	for _, id := range h {
		if suitOf(id, r) == suit {
			out = append(out, id)
		}
	}
	return out
}
func validateFollow(h, ids []int, lead Shape, r Rules) error {
	if !owns(h, ids) || len(ids) != lead.Count {
		return errors.New("跟牌张数须与领出相同，且只能使用自己的牌")
	}
	if lead.Four {
		for suit := 0; suit < 4; suit++ {
			held, played := false, false
			for _, id := range h {
				held = held || printedSuit(id) == suit
			}
			for _, id := range ids {
				played = played || printedSuit(id) == suit
			}
			if held && !played {
				return errors.New("四同领出须优先跟每种花色各一张")
			}
		}
		return nil
	}
	held := ofSuit(h, lead.Suit, r)
	chosen := ofSuit(ids, lead.Suit, r)
	if len(chosen) != min(len(held), lead.Count) {
		return errors.New("须先跟足领出的主牌或花色，缺门才能垫其他牌")
	}
	if len(held) < lead.Count {
		return nil
	}
	required := demands(lead)
	best := allocate(pairsIn(held, r), required)
	got := allocate(pairsIn(chosen, r), required)
	if compare(got.Profile, best.Profile) != 0 {
		return errors.New("须优先跟同门的完整拖拉机；不足时跟最长的部分拖拉机及对子")
	}
	return nil
}
func fullProfile(s Shape) []int { return demands(s) }
func contender(ids []int, lead Shape, r Rules) (int, int, bool) {
	if len(ids) != lead.Count || len(ids) == 0 {
		return 0, 0, false
	}
	if lead.Four {
		if !isFour(ids, r) {
			return 0, 0, false
		}
		v := rank(ids[0])
		if v == r.Level {
			v = 15
		}
		return 0, v, true
	}
	suit := suitOf(ids[0], r)
	if suit != lead.Suit && suit != 4 {
		return 0, 0, false
	}
	for _, id := range ids {
		if suitOf(id, r) != suit {
			return 0, 0, false
		}
	}
	req := fullProfile(lead)
	nodes := pairsIn(ids, r)
	a := allocate(nodes, req)
	if compare(a.Profile, req) != 0 {
		return 0, 0, false
	}
	high := -1
	if len(req) == 0 {
		for _, id := range ids {
			high = max(high, power(id, r))
		}
	} else {
		// Compare the highest component of the lead's largest structure. It may be
		// embedded in a longer tractor in a mixed throw.
		for _, mask := range chains(nodes, uint64(1)<<len(nodes)-1, req[0]) {
			rest := allocateRemaining(nodes, mask, req[1:])
			if !rest {
				continue
			}
			for i, n := range nodes {
				if mask&(1<<i) != 0 {
					high = max(high, n.Power)
				}
			}
		}
	}
	tier := 0
	if suit == 4 && lead.Suit != 4 {
		tier = 1
	}
	return tier, high, high >= 0
}
func allocateRemaining(nodes []pairNode, used uint64, req []int) bool {
	rest := []pairNode{}
	for i, n := range nodes {
		if used&(1<<i) == 0 {
			rest = append(rest, n)
		}
	}
	return compare(allocate(rest, req).Profile, req) == 0
}
func beatsTrick(ids, current []int, lead Shape, r Rules) bool {
	ta, ha, ok := contender(ids, lead, r)
	if !ok {
		return false
	}
	tb, hb, okb := contender(current, lead, r)
	return !okb || ta > tb || (ta == tb && ha > hb)
}
func canBeatPart(h []int, p Part, suit int, r Rules) bool {
	h = ofSuit(h, suit, r)
	if len(p.Cards) == 1 {
		for _, id := range h {
			if power(id, r) > p.High {
				return true
			}
		}
		return false
	}
	nodes := pairsIn(h, r)
	for _, m := range chains(nodes, uint64(1)<<len(nodes)-1, len(p.Cards)/2) {
		for i, n := range nodes {
			if m&(1<<i) != 0 && n.Power > p.High {
				return true
			}
		}
	}
	return false
}
func resolveThrow(ids []int, hands [][]int, actor int, r Rules) ([]int, int, error) {
	shape, e := leadShape(ids, r)
	if e != nil {
		return nil, -1, e
	}
	if shape.Four || len(shape.Parts) == 1 {
		return append([]int{}, ids...), -1, nil
	}
	parts := append([]Part{}, shape.Parts...)
	sort.Slice(parts, func(i, j int) bool {
		if len(parts[i].Cards) != len(parts[j].Cards) {
			return len(parts[i].Cards) < len(parts[j].Cards)
		}
		return parts[i].High < parts[j].High
	})
	for _, p := range parts {
		for offset := 1; offset < 4; offset++ {
			seat := (actor + offset) % 4
			if canBeatPart(hands[seat], p, shape.Suit, r) {
				return append([]int{}, p.Cards...), seat, nil
			}
		}
	}
	return append([]int{}, ids...), -1, nil
}
func bottomFactor(s Shape, mode string, decks int) int {
	if decks == 1 {
		return 2
	}
	if mode == "cards" {
		return s.Count * 2
	}
	n := 1
	for _, p := range s.Parts {
		n = max(n, len(p.Cards))
	}
	if n == 1 {
		return 2
	}
	return 1 << (n/2 + 1)
}
func scoreRound(p, decks int) (bool, int) {
	p = max(0, p)
	if decks == 1 {
		switch {
		case p == 0:
			return false, 2
		case p < 40:
			return false, 1
		case p < 80:
			return true, 0
		case p < 100:
			return true, 1
		default:
			return true, 2
		}
	}
	switch {
	case p == 0:
		return false, 3
	case p < 40:
		return false, 2
	case p < 80:
		return false, 1
	default:
		return true, (p - 80) / 40
	}
}
func shapeName(s Shape) string {
	if s.Four {
		return "四同领出"
	}
	if len(s.Parts) > 1 {
		return "甩牌"
	}
	switch s.Parts[0].Kind {
	case "single":
		return "单张"
	case "pair":
		return "对子"
	default:
		return fmt.Sprintf("%d 连对拖拉机", s.Count/2)
	}
}
func cardKey(ids []int) string {
	copy := append([]int{}, ids...)
	slices.Sort(copy)
	return strings.Trim(fmt.Sprint(copy), "[]")
}
