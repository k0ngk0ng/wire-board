package game

import (
	"reflect"
	"testing"
)

// Compare animation metadata with the actual committed physical inventory.
// Called from real Apply tests covering natural fishing, port swaps, pirate
// clearing and immediate victory, including JSON/wire action roundtrips.
func assertExplorerFishMotion(t *testing.T, before []catanExplorerCargoLocation, s *State, a Action) {
	t.Helper()
	x := s.Catan.Explorer
	changed := map[int]bool{}
	for id, loc := range x.Cargo.Fish {
		if loc != before[id] {
			changed[id] = true
		}
	}
	m := x.Motion
	if len(changed) == 0 && a.Type != "catan_explorer_fish_roll" {
		return
	}
	if m == nil || m.ID != x.ActionID || m.Kind != a.Type || len(m.Fish) != len(changed) {
		t.Fatal("fish animation does not match committed action", a, m)
	}
	for _, f := range m.Fish {
		if !changed[f.Fish] || f.From != before[f.Fish] || f.To != x.Cargo.Fish[f.Fish] {
			t.Fatal("missing, duplicate, or incorrect fish endpoint", f)
		}
		delete(changed, f.Fish)
	}
	if a.Type == "catan_explorer_fish_roll" && !reflect.DeepEqual(m.FishRoll, x.Fish.LastRoll) {
		t.Fatal("animation changed authoritative die result")
	}
	for viewer := -1; viewer < len(s.Catan.Players); viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
		if !reflect.DeepEqual(v["motion"], m) {
			t.Fatal("opponent or observer missing public fish motion")
		}
	}
}
