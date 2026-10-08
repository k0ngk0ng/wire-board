package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Controlled valid midgames: use real spice maps and cargo inventories, then
// relocate public pieces and resources to exercise full recruitment berths.
// No artificial lair numbers are needed by this scenario.
func explorerRecruitFreightFixture(t *testing.T, n int, kind, destination string, knights ...bool) (*State, Action) {
	t.Helper()
	var s *State
	if len(knights) > 0 && knights[0] {
		var err error
		s, err = NewCatanExplorerCitiesKnights(n, "spices-for-catan")
		if err != nil {
			t.Fatal(err)
		}
		for s.Catan.Explorer.Setup != nil {
			a, err := s.BotAction(s.Turn)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Apply(s.Turn, a); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.catanExplorerCityRoll(1, 2, 0); err != nil {
			t.Fatal(err)
		}
	} else {
		s = explorerSpiceStarted(t, n)
		if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
			t.Fatal(err)
		}
	}
	explorerFishRevealExcept(t, s, -1)
	g, x, p := s.Catan, s.Catan.Explorer, s.Turn
	for i := range g.Players {
		for r, v := range g.Players[i].Resources {
			g.Bank[r] += v
		}
		g.Players[i].Resources = make([]int, len(g.Bank))
	}
	for r := range g.Bank {
		g.Players[p].Resources[r] = 5
		g.Bank[r] -= 5
	}
	harbor := -1
	for _, v := range g.Vertices {
		if v.Owner == p && catanExplorerHarborAt(g, v.ID) {
			harbor = v.ID
		}
	}
	if harbor < 0 {
		t.Fatal("no initial harbor")
	}
	ship := p * 3
	if !x.Cargo.docked(g, x.Fleet, p, ship, harbor) {
		t.Fatal("initial ship not docked")
	}
	h, b := catanExplorerCargoLocation{"harbor", harbor}, catanExplorerCargoLocation{"ship", ship}
	target, other := h, b
	if destination == "ship" {
		target, other = b, h
	}
	// Fill every build location: the other berth has the initial settler.
	x.Cargo.Units[p*11] = other
	a := Action{Type: "catan_explorer_unit", Prompt: int(g.TurnSerial), Card: p*11 + 4, Choice: target.Kind, Target: target.Index}
	if kind == "fish" {
		x.Cargo.Fish[0] = target
		a.Targets = []int{0}
	} else {
		farm := x.Board.publicView().Farms[0].Tile
		sack := explorerSpiceClaimFixture(t, s, p, farm, 2, target)
		x.Cargo.Units[p*11+3] = target
		a.SpiceUnload = []int{sack}
	}
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	return explorerStateRestore(t, s), a
}

func TestCatanExplorerRecruitReturnsFishAndSpice(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, kind := range []string{"fish", "spice"} {
			for _, dest := range []string{"harbor", "ship"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, kind, dest), func(t *testing.T) {
					s, a := explorerRecruitFreightFixture(t, n, kind, dest)
					p := s.Turn
					// A fish can also be exchanged for a large settler. One spice only frees
					// one slot, so its valid replacement is a crew, never a large settler.
					if kind == "fish" && dest == "ship" {
						a.Card = p*11 + 1
					}
					offered := false
					for _, choice := range s.catanExplorerChoices(p) {
						if reflect.DeepEqual(catanExplorerChoiceView([]Action{choice}), catanExplorerChoiceView([]Action{a})) {
							offered = true
						}
					}
					if !offered {
						t.Fatal("return not offered", a)
					}
					raw, err := json.Marshal(catanExplorerChoiceView([]Action{a})[0])
					if err != nil {
						t.Fatal(err)
					}
					var wire Action
					if err = json.Unmarshal(raw, &wire); err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(wire.Targets, a.Targets) || !slices.Equal(wire.SpiceUnload, a.SpiceUnload) {
						t.Fatal("return lost on wire")
					}
					before := clone(*s)
					explorerFishReject(t, s, (p+1)%n, wire)
					stale := wire
					stale.Prompt--
					explorerFishReject(t, s, p, stale)
					if err = s.Apply(p, wire); err != nil {
						t.Fatal(err)
					}
					x := s.Catan.Explorer
					supply := catanExplorerCargoLocation{"supply", -1}
					if kind == "fish" && x.Cargo.Fish[0] != supply {
						t.Fatal("fish not returned")
					}
					if kind == "spice" {
						id := a.SpiceUnload[0]
						sack := x.Cargo.Spice[id]
						prior := before.Catan.Explorer.Cargo.Spice[id]
						if sack.At != supply || sack.Origin != prior.Origin || sack.Owner != prior.Owner || !x.Cargo.farmFriend(p, sack.Origin) || x.Cargo.Units[p*11+2] != (catanExplorerCargoLocation{"farm", sack.Origin}) {
							t.Fatal("return lost farm entitlement or permanent crew")
						}
					}
					if x.Cargo.Units[a.Card] != (catanExplorerCargoLocation{a.Choice, a.Target}) {
						t.Fatal("recruitment destination")
					}
					for r, cost := range catanExplorerUnitCost(a.Card) {
						if s.Catan.Players[p].Resources[r] != before.Catan.Players[p].Resources[r]-cost || s.Catan.Bank[r] != before.Catan.Bank[r]+cost {
							t.Fatal("wrong construction payment")
						}
					}
					if !reflect.DeepEqual(x.Fish, before.Catan.Explorer.Fish) || !reflect.DeepEqual(x.Spice, before.Catan.Explorer.Spice) || s.Catan.Players[p].Score != before.Catan.Players[p].Score {
						t.Fatal("discard must not deliver or score")
					}
					assertExplorerFishMotion(t, before.Catan.Explorer.Cargo.Fish, s, wire)
					assertExplorerSpiceMotion(t, before.Catan.Explorer.Cargo.Spice, s, wire)
					if x.Motion == nil || len(x.Motion.Cargo) != 1 || x.Motion.Cargo[0].Unit != a.Card {
						t.Fatal("recruitment motion absent")
					}
					for viewer := -1; viewer < n; viewer++ {
						if viewer != p && len(s.catanExplorerChoices(viewer)) != 0 {
							t.Fatal("other viewer can recruit")
						}
					}
					s = explorerStateRestore(t, s)
					explorerFishReject(t, s, p, wire) // returned freight is no longer in the berth
				})
			}
		}
	}
}

func TestCatanExplorerRecruitFreightRejectsInvalidReturnsAtomically(t *testing.T) {
	for _, kind := range []string{"fish", "spice"} {
		t.Run(kind, func(t *testing.T) {
			s, a := explorerRecruitFreightFixture(t, 3, kind, "harbor")
			p := s.Turn
			cases := map[string]func(*State, *Action){
				"negative": func(_ *State, a *Action) {
					if kind == "fish" {
						a.Targets = []int{-1}
					} else {
						a.SpiceUnload = []int{-1}
					}
				},
				"out-of-range": func(_ *State, a *Action) {
					if kind == "fish" {
						a.Targets = []int{999}
					} else {
						a.SpiceUnload = []int{999}
					}
				},
				"duplicate": func(_ *State, a *Action) {
					if kind == "fish" {
						a.Targets = append(a.Targets, a.Targets[0])
					} else {
						a.SpiceUnload = append(a.SpiceUnload, a.SpiceUnload[0])
					}
				},
				"multiple-kinds": func(_ *State, a *Action) { a.Cards = []int{p*11 + 3} },
				"other-berth":    func(s *State, a *Action) { a.Choice = "ship"; a.Target = p * 3 },
				"empty-berth-available": func(s *State, _ *Action) {
					s.Catan.Explorer.Cargo.Units[p*11] = catanExplorerCargoLocation{"supply", -1}
				},
				"unaffordable": func(s *State, _ *Action) {
					s.Catan.Bank[2] += s.Catan.Players[p].Resources[2]
					s.Catan.Players[p].Resources[2] = 0
				},
				"wrong-phase": func(s *State, _ *Action) { explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"}) },
			}
			if kind == "spice" {
				cases["too-large"] = func(_ *State, a *Action) { a.Card = p*11 + 1 }
			}
			for name, mutate := range cases {
				t.Run(name, func(t *testing.T) {
					copy, act := clone(*s), clone(a)
					mutate(&copy, &act)
					if err := copy.validateCatanExplorer(); err != nil {
						t.Fatal("invalid precondition", err)
					}
					explorerFishReject(t, &copy, p, act)
				})
			}
		})
	}
}
