package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCatanPublicEventsEnableRestoreAndIsolation(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, fixed := range []bool{false, true} {
			if fixed && n < 5 {
				continue
			}
			t.Run(fmt.Sprintf("%d/fixed=%t", n, fixed), func(t *testing.T) {
				layout := "variable"
				if fixed {
					layout = "fixed"
				}
				s, err := NewCatanConfigured(n, CatanOptions{FiveSix: n > 4}, CatanBaseConfiguration{Layout: layout})
				if err != nil {
					t.Fatal(err)
				}
				if s.Catan.EventDeck != nil {
					t.Fatal("base unexpectedly has events")
				}
				before, _ := json.Marshal(s)
				if s.EnableCatanEvents("legacy-reference-v1") == nil {
					t.Fatal("legacy deck accepted as public")
				}
				after, _ := json.Marshal(s)
				if string(before) != string(after) {
					t.Fatal("invalid enable mutated base")
				}
				if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
				s = referenceEventRestore(t, s)
				if s.Catan.EventDeck.Catalogue != CatanEventCatalogue {
					t.Fatal("lost site catalogue")
				}
				before, _ = json.Marshal(s)
				if s.EnableCatanEvents(CatanEventCatalogue) == nil {
					t.Fatal("repeat enable shuffled deck")
				}
				after, _ = json.Marshal(s)
				if string(before) != string(after) {
					t.Fatal("repeat enable changed deck")
				}
				for s.Catan.setup() {
					p := ckActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(e)
					}
					helperApply(t, s, p, a)
				}
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				if s.Catan.RollID != 1 || s.Catan.RevealedEvent == nil {
					t.Fatal("production did not draw")
				}
				referenceEventRestore(t, s)
			})
		}
	}
}

func TestCatanPublicEventsRejectUnsupportedAtomically(t *testing.T) {
	constructors := map[string]func() (*State, error){
		"fishing":   func() (*State, error) { return NewCatanFishing(3, CatanOptions{}) },
		"explorer":  func() (*State, error) { return NewCatanExplorerLandHo(3) },
		"transport": func() (*State, error) { return NewCatanTransport(3) },
		"already-rolled": func() (*State, error) {
			s, e := NewCatan(3, CatanOptions{})
			if e == nil {
				s.Catan.RollID = 1
			}
			return s, e
		},
	}
	for name, create := range constructors {
		t.Run(name, func(t *testing.T) {
			s, e := create()
			if e != nil {
				t.Fatal(e)
			}
			before, _ := json.Marshal(s)
			if s.EnableCatanEvents(CatanEventCatalogue) == nil {
				t.Fatal("unsupported enabled")
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("rejected enable mutated state")
			}
		})
	}
}
