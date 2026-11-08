package stoneage_test

import (
	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
)

// Saving and locking must be one transaction: a lost/failed save must never
// lock an older loadout. Invalid fields cannot leave a partially saved choice.
func TestStoneageAtomicLoadoutLockHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "stoneage")
	h.Start(map[string]any{})
	a := httpSuggestion(t, h.People[0])
	a["type"] = "lock"
	a["loadout"].(map[string]any)["points"] = []int{233, 107, 81, 59}
	h.People[0].Action(a, 200)
	current := gametest.Data(h.People[0].Room())
	if current["players"].([]any)[0].(map[string]any)["locked"] != true {
		t.Fatal("atomic lock did not lock")
	}
	if current["self"].(map[string]any)["loadout"].(map[string]any)["points"].([]any)[0] != float64(233) {
		t.Fatal("locked old allocation")
	}
	b := httpSuggestion(t, h.People[1])
	b["type"] = "lock"
	b["loadout"].(map[string]any)["weapon"] = 999999
	before := gametest.Data(h.People[1].Room())
	h.People[1].Action(b, 400)
	if !reflect.DeepEqual(before, gametest.Data(h.People[1].Room())) {
		t.Fatal("invalid lock saved partial state")
	}
	h.Record("atomic configure-and-lock; rejected lock leaves no partial configuration")
	report(t, "atomic-loadout-lock-http", h.People[0].Room())
}
