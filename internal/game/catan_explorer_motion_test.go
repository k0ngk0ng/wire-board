package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerMotionPublicTransactionalAndRestored(t *testing.T) {
	s, err := newCatanExplorerState(2)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for step := 0; step < 1600 && !s.Finished; step++ {
		actor := s.Turn
		if s.Phase == "catan_discard" {
			for p, n := range s.Catan.DiscardDue {
				if n > 0 {
					actor = p
					break
				}
			}
		}
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		before := clone(*s)
		if err = s.Apply(actor, a); err != nil {
			t.Fatal(err)
		}
		x := s.Catan.Explorer
		if x.ActionID != before.Catan.Explorer.ActionID+1 {
			t.Fatal("action ID not advanced")
		}
		if m := x.Motion; m != nil {
			seen[m.Kind] = true
			if m.ID != x.ActionID || m.Player != actor {
				t.Fatal("wrong event identity")
			}
			if m.Kind == "catan_explorer_sail" && !slices.Equal(m.Path, append([]int{before.Catan.Explorer.Fleet.Positions[a.Slot]}, a.Targets...)) {
				t.Fatal("not the submitted path")
			}
			for _, tile := range m.Revealed {
				if before.Catan.Tiles[tile].Resource != 8 || s.Catan.Tiles[tile].Resource == 8 {
					t.Fatal("revealed hidden/unmodified tile")
				}
			}
			for _, c := range m.Cargo {
				if c.From != before.Catan.Explorer.Cargo.Units[c.Unit] || c.To != x.Cargo.Units[c.Unit] || c.From == c.To {
					t.Fatal("wrong cargo flight")
				}
			}
			for viewer := -1; viewer < 2; viewer++ {
				v := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
				if !reflect.DeepEqual(v["motion"], m) {
					t.Fatal("motion differs by viewer")
				}
			}
			s = explorerStateRestore(t, s)
		} else if a.Type == "catan_explorer_sail" {
			t.Fatal("missing sailing event")
		}
		bytesBefore, _ := json.Marshal(s)
		if err := s.Apply(actor, Action{Type: "catan_explorer_sail", Prompt: -1}); err == nil {
			t.Fatal("accepted invalid action")
		}
		bytesAfter, _ := json.Marshal(s)
		if !slices.Equal(bytesBefore, bytesAfter) {
			t.Fatal("rejected action consumed event")
		}
	}
	if !seen["catan_explorer_sail"] || !seen["catan_explorer_settle"] {
		t.Fatal("missing natural motion coverage", seen)
	}
}

func TestCatanExplorerMotionLoadUnload(t *testing.T) {
	s, err := newCatanExplorerState(2)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(s.Turn, Action{Type: "catan_explorer_begin_move", Prompt: int(s.Catan.TurnSerial)}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"harbor", "ship"} {
		found := false
		for _, a := range s.catanExplorerChoices(s.Turn) {
			if a.Type != "catan_explorer_transfer" || kind == "harbor" && len(a.Take) == 0 || kind == "ship" && len(a.Give) == 0 {
				continue
			}
			if err = s.Apply(s.Turn, a); err != nil {
				t.Fatal(err)
			}
			m := s.Catan.Explorer.Motion
			if m == nil || len(m.Cargo) != 1 || m.Cargo[0].To.Kind != kind || m.Vertex != a.Vertex || m.Ship != a.Slot {
				t.Fatal("wrong loading/unloading anchors", m)
			}
			found = true
			break
		}
		if !found {
			t.Fatal("missing initial harbor transfer", kind)
		}
	}
	if err = s.Apply(s.Turn, Action{Type: "catan_end", Prompt: int(s.Catan.TurnSerial)}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Explorer.Motion != nil {
		t.Fatal("non-motion action retained old effect")
	}
}

func TestCatanExplorerMissionMotionUsesOnlyConfirmedPublicChanges(t *testing.T) {
	s, err := newCatanExplorerLairsState(3, "fixed", []int{2, 3, 4, 5, 6, 8})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for step := 0; step < 6000 && !s.Finished; step++ {
		actor := s.Turn
		if s.Phase == "catan_discard" {
			for p, n := range s.Catan.DiscardDue {
				if n > 0 {
					actor = p
					break
				}
			}
		}
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		before := clone(*s)
		if err = s.Apply(actor, a); err != nil {
			t.Fatal(err)
		}
		x, old := s.Catan.Explorer, before.Catan.Explorer
		m := x.Motion
		if m == nil {
			continue
		}
		seen[m.Kind] = true
		for _, c := range m.Cargo {
			if c.From != old.Cargo.Units[c.Unit] || c.To != x.Cargo.Units[c.Unit] {
				t.Fatal("cargo motion differs from actual piece change")
			}
		}
		if m.Lair != nil {
			l := m.Lair
			site := x.Lairs.Sites[x.Lairs.site(l.Tile)]
			prior := old.Lairs.Sites[old.Lairs.site(l.Tile)]
			if l.Ready != (site.Ready > 0 && prior.Ready == 0) || l.Resolved != (site.Resolved > 0 && prior.Resolved == 0) || l.Hero != site.Hero {
				t.Fatal("wrong lair transition")
			}
			if len(l.Dice) > 0 && (!slices.Equal(l.Dice, site.Rounds[len(site.Rounds)-1]) || len(site.Rounds) != len(prior.Rounds)+1) {
				t.Fatal("dice event replayed old round")
			}
			if l.Resolved {
				seen["resolved"] = true
			}
			raw, _ := json.Marshal(l)
			var public map[string]any
			_ = json.Unmarshal(raw, &public)
			for _, key := range []string{"number", "deck", "inventory", "contributions"} {
				if _, ok := public[key]; ok {
					t.Fatal("private or redundant mission data copied", key)
				}
			}
		}
		if p := m.Pirate; p != nil {
			if p.FromOwner != old.Pirate.Owner || p.FromTile != old.Pirate.Tile || p.ToOwner != x.Pirate.Owner || p.ToTile != x.Pirate.Tile {
				t.Fatal("pirate placement event incorrect")
			}
		}
		if m.Chase != nil && !reflect.DeepEqual(m.Chase, x.Pirate.LastChase) {
			t.Fatal("wrong chase die")
		}
		for viewer := -1; viewer < 3; viewer++ {
			view := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
			if !reflect.DeepEqual(view["motion"], m) {
				t.Fatal("motion differs by viewer")
			}
		}
		saved, _ := json.Marshal(s)
		if err = s.Apply(actor, Action{Type: a.Type, Prompt: -1}); err == nil {
			t.Fatal("stale action accepted")
		}
		unchanged, _ := json.Marshal(s)
		if !slices.Equal(saved, unchanged) {
			t.Fatal("failed action changed animation")
		}
		s = explorerStateRestore(t, s)
	}
	if !s.Finished || !seen["catan_explorer_land"] || !seen["catan_explorer_pickup"] || !seen["resolved"] || !seen["catan_explorer_pirate_place"] {
		t.Fatal("mission motion not exercised", seen)
	}
}
