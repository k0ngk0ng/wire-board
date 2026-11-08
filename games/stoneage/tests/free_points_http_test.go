package stoneage_test

import (
	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
)

// New requested behavior: equal starting budgets, freely distributed four
// attributes, no preset-only allocation, and atomic rejection of bad totals.
func TestStoneageFreePointsHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "stoneage")
	h.Start(map[string]any{})
	for _, c := range h.People {
		if gametest.Data(c.Room())["pointBudget"] != float64(480) {
			t.Fatal("starting attribute budgets differ")
		}
	}
	a := httpSuggestion(t, h.People[0])
	l := a["loadout"].(map[string]any)
	for _, points := range [][]int{{0, 0, 0, 0}, {481, 0, 0, 0}, {120, 120, 120, 121}, {-1, 161, 160, 160}, {160, 160, 160}, {120, 120, 120, 120, 480}} {
		l["points"] = points
		before := gametest.Data(h.People[0].Room())
		h.People[0].Action(a, 400)
		if !reflect.DeepEqual(before, gametest.Data(h.People[0].Room())) {
			t.Fatal("rejected allocation mutated match")
		}
	}
	l["points"] = []int{233, 107, 81, 59}
	h.People[0].Action(a, 200)
	own := gametest.Data(h.People[0].Room())["self"].(map[string]any)["loadout"].(map[string]any)
	if !reflect.DeepEqual(own["points"], []any{float64(233), float64(107), float64(81), float64(59)}) {
		t.Fatal("custom allocation lost", own)
	}
	b := httpSuggestion(t, h.People[1])
	b["loadout"].(map[string]any)["points"] = []int{480, 0, 0, 0}
	h.People[1].Action(b, 200)
	h.Restart()
	if !reflect.DeepEqual(own, gametest.Data(h.People[0].Room())["self"].(map[string]any)["loadout"]) {
		t.Fatal("restart changed chosen points")
	}
	h.Record("equal budgets, arbitrary valid allocation and invalid totals")
	report(t, "free-points-http", h.People[0].Room())
}
