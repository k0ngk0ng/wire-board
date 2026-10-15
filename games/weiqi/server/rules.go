package weiqi

import (
	"crypto/sha256"
	"fmt"
	"sort"
)

// PositionKey implements positional superko: colors only, never the next seat.
func PositionKey(board []int) string {
	b := make([]byte, len(board))
	for i, v := range board {
		b[i] = byte(v)
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}
func neighbours(pos, n int) []int {
	r := make([]int, 0, 4)
	if pos%n > 0 {
		r = append(r, pos-1)
	}
	if pos%n < n-1 {
		r = append(r, pos+1)
	}
	if pos >= n {
		r = append(r, pos-n)
	}
	if pos+n < n*n {
		r = append(r, pos+n)
	}
	return r
}
func group(board []int, pos, n int) ([]int, []int) {
	color := board[pos]
	seen := make([]bool, len(board))
	lib := make([]bool, len(board))
	stones := []int{pos}
	seen[pos] = true
	for i := 0; i < len(stones); i++ {
		for _, p := range neighbours(stones[i], n) {
			if board[p] == 0 {
				lib[p] = true
			} else if board[p] == color && !seen[p] {
				seen[p] = true
				stones = append(stones, p)
			}
		}
	}
	libs := []int{}
	for p, v := range lib {
		if v {
			libs = append(libs, p)
		}
	}
	sort.Ints(stones)
	return stones, libs
}
func play(board []int, pos, color, n int, history map[string]bool) ([]int, []int, bool) {
	if pos < 0 || pos >= len(board) || board[pos] != 0 {
		return nil, nil, false
	}
	next := append([]int(nil), board...)
	next[pos] = color
	removed := []int{}
	for _, p := range neighbours(pos, n) {
		if next[p] == 3-color {
			stones, libs := group(next, p, n)
			if len(libs) == 0 {
				for _, q := range stones {
					next[q] = 0
				}
				removed = append(removed, stones...)
			}
		}
	}
	_, libs := group(next, pos, n)
	if len(libs) == 0 {
		return nil, nil, false
	}
	if history != nil && history[PositionKey(next)] {
		return nil, nil, false
	}
	sort.Ints(removed)
	return next, removed, true
}
func keySet(keys []string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, key := range keys {
		m[key] = true
	}
	return m
}

type Score struct {
	Black     float64 `json:"black"`
	White     float64 `json:"white"`
	Neutral   int     `json:"neutral"`
	Stones    [2]int  `json:"stones"`
	Territory [2]int  `json:"territory"`
	Ownership []int   `json:"ownership"`
}

func score(board []int, n int, komi float64, dead []int) Score {
	b := append([]int(nil), board...)
	for _, p := range dead {
		b[p] = 0
	}
	s := Score{Ownership: make([]int, len(b))}
	seen := make([]bool, len(b))
	for p, v := range b {
		if v != 0 {
			s.Stones[v-1]++
			s.Ownership[p] = v
			continue
		}
		if seen[p] {
			continue
		}
		area := []int{p}
		seen[p] = true
		colors := [3]bool{}
		for i := 0; i < len(area); i++ {
			for _, q := range neighbours(area[i], n) {
				if b[q] != 0 {
					colors[b[q]] = true
				} else if !seen[q] {
					seen[q] = true
					area = append(area, q)
				}
			}
		}
		owner := 0
		if colors[1] && !colors[2] {
			owner = 1
		}
		if colors[2] && !colors[1] {
			owner = 2
		}
		if owner == 0 {
			s.Neutral += len(area)
		} else {
			s.Territory[owner-1] += len(area)
			for _, q := range area {
				s.Ownership[q] = owner
			}
		}
	}
	s.Black = float64(s.Stones[0] + s.Territory[0])
	s.White = float64(s.Stones[1]+s.Territory[1]) + komi
	return s
}
