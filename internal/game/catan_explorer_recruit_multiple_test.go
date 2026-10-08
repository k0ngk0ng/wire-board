package game

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// A controlled full-berth situation on a real, legally initialized mission.
// Permanent farm claims remain intact when their sacks are returned.
func explorerRecruitMultipleFixture(t *testing.T, n int, kind, destination string, knights bool) (*State, Action) {
	t.Helper()
	s, a := explorerRecruitFreightFixture(t, n, "spice", destination, knights)
	x, p := s.Catan.Explorer, s.Turn
	target := catanExplorerCargoLocation{a.Choice, a.Target}
	a.Card = p*11 + 1
	a.Cards = []int{p*11 + 3}
	if kind == "crew" {
		x.Cargo.Spice[a.SpiceUnload[0]].At = catanExplorerCargoLocation{"supply", -1}
		a.SpiceUnload = nil
		x.Cargo.Units[p*11+4] = target
		a.Cards = append(a.Cards, p*11+4)
	} else if kind == "spice" {
		x.Cargo.Units[p*11+3] = catanExplorerCargoLocation{"supply", -1}
		a.Cards = nil
		farm := x.Board.publicView().Farms[1].Tile
		sack := explorerSpiceClaimFixture(t, s, p, farm, 4, target)
		a.SpiceUnload = append(a.SpiceUnload, sack)
	}
	return explorerStateRestore(t, s), a
}

func TestCatanExplorerRecruitTwoSmallCargo(t *testing.T) {
	for _, knights := range []bool{false, true} {
		for n := 2; n <= 6; n++ {
			if knights && n == 2 {
				continue
			}
			for _, kind := range []string{"crew", "mixed", "spice"} {
				for _, dest := range []string{"ship", "harbor"} {
					t.Run(fmt.Sprintf("%t/%d/%s/%s", knights, n, kind, dest), func(t *testing.T) {
						s, a := explorerRecruitMultipleFixture(t, n, kind, dest, knights)
						p := s.Turn
						found := false
						for _, q := range s.catanExplorerChoices(p) {
							if reflect.DeepEqual(catanExplorerChoiceView([]Action{q}), catanExplorerChoiceView([]Action{a})) {
								found = true
							}
						}
						if !found {
							t.Fatal("missing explicit two-cargo choice")
						}
						before := clone(*s)
						explorerFishReject(t, s, (p+1)%n, a)
						if err := s.Apply(p, a); err != nil {
							t.Fatal(err)
						}
						x := s.Catan.Explorer
						supply := catanExplorerCargoLocation{"supply", -1}
						for _, id := range a.Cards {
							if x.Cargo.Units[id] != supply {
								t.Fatal("crew not returned")
							}
						}
						for _, id := range a.SpiceUnload {
							sack, old := x.Cargo.Spice[id], before.Catan.Explorer.Cargo.Spice[id]
							if sack.At != supply || sack.Owner != old.Owner || sack.Origin != old.Origin || !x.Cargo.farmFriend(p, sack.Origin) {
								t.Fatal("farm entitlement changed")
							}
						}
						if x.Cargo.Units[a.Card] != (catanExplorerCargoLocation{a.Choice, a.Target}) {
							t.Fatal("missing settler")
						}
						for r, cost := range catanExplorerUnitCost(a.Card) {
							if s.Catan.Bank[r] != before.Catan.Bank[r]+cost || s.Catan.Players[p].Resources[r] != before.Catan.Players[p].Resources[r]-cost {
								t.Fatal("wrong payment")
							}
						}
						if !reflect.DeepEqual(x.Fish, before.Catan.Explorer.Fish) || !reflect.DeepEqual(x.Spice, before.Catan.Explorer.Spice) || s.Catan.Players[p].Score != before.Catan.Players[p].Score {
							t.Fatal("return delivered or scored")
						}
						if len(x.Motion.Cargo) != len(a.Cards)+1 || len(x.Motion.Spice) != len(a.SpiceUnload) {
							t.Fatal("missing movement events")
						}
						if !strings.Contains(strings.Join(s.Log, "\n"), "本站补充规则") {
							t.Fatal("unlabelled supplement")
						}
						s = explorerStateRestore(t, s)
						explorerFishReject(t, s, p, a)
					})
				}
			}
		}
	}
}

func TestCatanExplorerRecruitTwoSmallCargoRejectsAtomically(t *testing.T) {
	for _, kind := range []string{"crew", "mixed", "spice"} {
		s, a := explorerRecruitMultipleFixture(t, 3, kind, "harbor", false)
		p := s.Turn
		cases := map[string]func(*State, *Action){
			"duplicate": func(_ *State, a *Action) {
				if len(a.Cards) == 2 {
					a.Cards[1] = a.Cards[0]
				} else {
					a.SpiceUnload = append(a.SpiceUnload, a.SpiceUnload[0])
				}
			},
			"wrong-target": func(_ *State, a *Action) { a.Choice = "ship"; a.Target = p * 3 },
			"other-unit":   func(_ *State, a *Action) { a.Card = (p + 1) % 3 * 11 },
			"stale":        func(_ *State, a *Action) { a.Prompt-- },
			"over-return":  func(_ *State, a *Action) { a.Card = p*11 + 9 },
			"under-return": func(_ *State, a *Action) {
				if len(a.Cards) > 0 {
					a.Cards = a.Cards[1:]
				} else {
					a.SpiceUnload = a.SpiceUnload[1:]
				}
			},
			"unaffordable": func(s *State, _ *Action) {
				g := s.Catan
				g.Bank[0] += g.Players[p].Resources[0]
				g.Players[p].Resources[0] = 0
			},
			"not-all-full": func(s *State, _ *Action) {
				s.Catan.Explorer.Cargo.Units[p*11] = catanExplorerCargoLocation{"supply", -1}
			},
		}
		for name, mutate := range cases {
			t.Run(kind+"/"+name, func(t *testing.T) { q, b := clone(*s), clone(a); mutate(&q, &b); explorerFishReject(t, &q, p, b) })
		}
	}
}

func TestCatanExplorerRetiredCrewRecruitmentPlan(t *testing.T) {
	s, a := explorerRecruitMultipleFixture(t, 3, "crew", "ship", false)
	g, x, p := s.Catan, s.Catan.Explorer, s.Turn
	// Finish all of this player's farm contacts, without fabricating deliveries.
	for _, farm := range x.Board.publicView().Farms {
		if x.Cargo.farmFriend(p, farm.Tile) {
			continue
		}
		for id := p*11 + 2; id < (p+1)*11; id++ {
			if x.Cargo.Units[id].Kind == "supply" {
				sack := explorerSpiceClaimFixture(t, s, p, farm.Tile, id%11, catanExplorerCargoLocation{"supply", -1})
				if sack < 0 {
					t.Fatal("claim")
				}
				break
			}
		}
	}
	// Keep the other berth full with a fish; returning it is not a bot plan.
	old := x.Cargo.Units[p*11]
	x.Cargo.Units[p*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Fish[0] = old
	s = explorerStateRestore(t, s)
	found := false
	for _, plan := range s.catanExplorerMissionPlans(p) {
		q := plan.action
		if q.Type == "catan_explorer_unit" && q.Card%11 < 2 && slices.Equal(q.Cards, a.Cards) {
			found = true
			if err := s.Apply(p, q); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if !found {
		t.Fatal("bot cannot replace retired crew", g.Explorer.Board.Scenario)
	}
}
