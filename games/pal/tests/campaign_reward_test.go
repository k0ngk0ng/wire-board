package pal_test

import (
	"sort"
	"strings"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

// Observe the original victory panel. Zero cash is a valid original reward;
// identities, hidden enemy HP and native experience memory are never consulted.
func campaignBattleReward(v pal.View) (int, int, bool) {
	if v.Raster == nil {
		return 0, 0, false
	}
	seen := map[int]bool{}
	digits := map[[2]int]int{}
	var label strings.Builder
	var visit func(int)
	visit = func(id int) {
		if id <= 0 || id > len(v.Raster.Nodes) || seen[id] {
			return
		}
		seen[id] = true
		n := v.Raster.Nodes[id-1]
		visit(n[1])
		visit(n[2])
		if n[0] == 2 && n[7] == 70 && n[8] == 0 {
			label.WriteRune(rune(n[5]))
		}
		if n[0] == 1 && n[5] == 2 && n[6] == 0 && n[7] >= 19 && n[7] <= 28 && (n[9] == 74 || n[9] == 119) {
			digits[[2]int{n[8], n[9]}] = n[7] - 19
		}
	}
	visit(v.Raster.Root)
	if !strings.Contains(label.String(), "经验") {
		return 0, 0, false
	}
	number := func(y int) (int, bool) {
		xs := []int{}
		for p := range digits {
			if p[1] == y {
				xs = append(xs, p[0])
			}
		}
		sort.Ints(xs)
		n := 0
		for _, x := range xs {
			n = n*10 + digits[[2]int{x, y}]
		}
		return n, len(xs) > 0
	}
	xp, haveXP := number(74)
	cash, haveCash := number(119)
	return xp, cash, haveXP && haveCash
}
