package game

import (
	"reflect"
	"testing"
)

func assertExplorerSpiceMotion(t *testing.T, before []catanExplorerSpiceSack, s *State, a Action) {
	t.Helper()
	x := s.Catan.Explorer
	changed := map[int]bool{}
	for id, sack := range x.Cargo.Spice {
		if sack.At != before[id].At {
			changed[id] = true
		}
	}
	if len(changed) == 0 {
		return
	}
	m := x.Motion
	if m == nil || m.Kind != a.Type || m.ID != x.ActionID || len(m.Spice) != len(changed) {
		t.Fatal("spice motion disagrees with committed freight", a, m)
	}
	for _, c := range m.Spice {
		if !changed[c.Sack] || c.From != before[c.Sack].At || c.To != x.Cargo.Spice[c.Sack].At {
			t.Fatal("incorrect or duplicate spice endpoint", c)
		}
		delete(changed, c.Sack)
	}
	for viewer := -1; viewer < len(s.Catan.Players); viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
		if !reflect.DeepEqual(v["motion"], m) {
			t.Fatal("public spice motion missing for viewer", viewer)
		}
	}
}
