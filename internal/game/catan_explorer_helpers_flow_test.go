package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerHelpersOpening(t *testing.T, city bool) *State {
	t.Helper()
	s := explorerHelpersNew(t, 3, "pirate-lairs", city, true, false, true)
	for i := 0; i < 100 && s.Catan.Explorer.Setup != nil; i++ {
		explorerHelpersStep(t, s)
	}
	if s.Phase != "catan_roll" {
		t.Fatal("opening")
	}
	return s
}

func explorerHelpersHand(t *testing.T, s *State, p int, hand []int) {
	t.Helper()
	g := s.Catan
	if len(hand) != len(g.Bank) {
		t.Fatal("hand length")
	}
	for r, n := range g.Players[p].Resources {
		g.Bank[r] += n
		g.Players[p].Resources[r] = 0
	}
	for r, n := range hand {
		if n < 0 || n > g.Bank[r] {
			t.Fatal("fixture bank")
		}
		g.Bank[r] -= n
		g.Players[p].Resources[r] = n
	}
}

func explorerHelpersRoll(t *testing.T, s *State, number int) {
	t.Helper()
	red := min(6, number-1)
	var err error
	if s.Catan.CitiesKnights != nil {
		err = s.catanExplorerCityRoll(red, number-red, 3)
	} else {
		err = s.catanExplorerRoll([2]int{red, number - red})
	}
	if err != nil {
		t.Fatal(err)
	}
	explorerFishingRestore(t, s)
}

func TestCatanExplorerHelpersSevenAndProduction(t *testing.T) {
	for _, city := range []bool{false, true} {
		for _, large := range []bool{false, true} {
			t.Run(fmt.Sprintf("city%t/large%t", city, large), func(t *testing.T) {
				s := explorerHelpersOpening(t, city)
				p, other := (s.Turn+1)%3, (s.Turn+2)%3
				explorerHelpersAssign(t, s, p, 5)
				hand := make([]int, len(s.Catan.Bank))
				hand[0] = 3
				if large {
					hand[0] = 8
					if city {
						hand[0] = 3
						hand[5] = 5
					}
				}
				explorerHelpersHand(t, s, p, hand)
				otherHand := make([]int, len(hand))
				otherHand[1] = 8
				explorerHelpersHand(t, s, other, otherHand)
				explorerHelpersRoll(t, s, 7)
				g := s.Catan
				if s.Phase != "catan_helper" || s.CatanPendingActor() != p || g.DiscardDue[p] != 0 || g.DiscardDue[other] != 4 {
					t.Fatal("Thorolf priority or protection")
				}
				if large {
					if g.HelperPending.Kind != "exchange" || !reflect.DeepEqual(hand, g.Players[p].Resources) {
						t.Fatal("protection gave bonus")
					}
				} else {
					if g.HelperPending.Kind != "resource" {
						t.Fatal("small hand resource")
					}
					explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Color: 2})
					if s.Catan.Players[p].Resources[2] != 1 {
						t.Fatal("small hand bonus missing")
					}
				}
				explorerHelpersReject(t, s, other, Action{Type: "catan_discard", Tokens: []int{0, 4, 0, 0, 0}, Prompt: int(g.TurnSerial)})
				explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
				if s.Phase != "catan_discard" {
					t.Fatal("did not resume discards")
				}
				for i := 0; i < 10 && s.Phase != "catan_turn"; i++ {
					explorerHelpersStep(t, s)
					explorerFishingRestore(t, s)
				}
				if s.Phase != "catan_turn" {
					t.Fatal("seven continuation")
				}
			})
		}
		for _, skip := range []bool{false, true} {
			t.Run(fmt.Sprintf("city%t/HildaSkip%t", city, skip), func(t *testing.T) {
				s := explorerHelpersOpening(t, city)
				p := (s.Turn + 1) % 3
				explorerHelpersAssign(t, s, p, 3)
				if city {
					s.Catan.CitiesKnights.Players[p].Improvements[0] = 3
				}
				number := 0
				for n := 2; n <= 12; n++ {
					if n != 7 && sum(explorerCityClaims(s.Catan, n)[p]) == 0 {
						number = n
						break
					}
				}
				if number == 0 {
					t.Fatal("fixture no non-producing number")
				}
				before := sum(s.Catan.Players[p].Resources)
				explorerHelpersRoll(t, s, number)
				if s.Phase != "catan_helper" || s.CatanPendingActor() != p || s.Catan.HelperPending.Kind != "resource" {
					t.Fatal("Hilda response")
				}
				if skip {
					explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Choice: "skip"})
				} else {
					explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Color: 0})
					explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
				}
				bonus := 0
				if !skip {
					bonus = 1
				}
				if sum(s.Catan.Players[p].Resources) != before+bonus {
					t.Fatal("Hilda income")
				}
				if city {
					if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != p {
						t.Fatal("Hilda changed Aqueduct qualification", s.Phase)
					}
					explorerHelpersStep(t, s)
				}
				if s.Phase != "catan_turn" || s.Catan.Explorer.Helpers.Production != nil {
					t.Fatal("production continuation")
				}
				explorerFishingRestore(t, s)
			})
		}
	}
}

func TestCatanExplorerHelpersLeaderPrivacy(t *testing.T) {
	s := explorerHelpersFixture(t, true, 7)
	p, target, other := s.Turn, (s.Turn+1)%3, (s.Turn+2)%3
	explorerVictoryBuildings(t, s, target, s.Catan.Players[p].Score+1)
	explorerHelpersHand(t, s, target, []int{0, 0, 0, 0, 0, 3, 0, 0})
	before := s.catanExplorerChoices(p)
	changed := clone(*s)
	explorerHelpersHand(t, &changed, target, []int{3, 0, 0, 0, 0, 0, 0, 0})
	if !reflect.DeepEqual(before, changed.catanExplorerChoices(p)) {
		t.Fatal("previews reveal ordinary/commodity composition")
	}
	explorerHelpersAct(t, s, p, Action{Type: "catan_helper", Target: target})
	for _, viewer := range []int{p, target, other, -1} {
		v := s.View(viewer)["catan"].(map[string]any)
		q := v["helperPending"].(map[string]any)
		_, has := q["resources"]
		if has != (viewer == p) {
			t.Fatal("helper hand privacy", viewer)
		}
	}
	if a, e := s.BotAction(p); e != nil || a.Choice != "skip" {
		t.Fatal("empty ordinary hand must complete", a, e)
	}
	explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Choice: "skip"})
	explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
	if sum(s.Catan.Players[target].Resources) != 3 {
		t.Fatal("helper took commodities")
	}
	// A separate fixture exercises the resource choice and its hidden color.
	s = &changed
	explorerHelpersAct(t, s, p, Action{Type: "catan_helper", Target: target})
	explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Color: 0})
	if s.Catan.Players[target].Resources[0] != 2 {
		t.Fatal("resource not taken")
	}
	explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Choice: "exchange", Card: s.Catan.HelperDisplay[0]})
	if s.Catan.helperReady(p, s.Catan.Players[p].Helper.ID) {
		t.Fatal("new helper immediately ready")
	}
}

func TestCatanExplorerHelpersDepartureAndCorruption(t *testing.T) {
	for _, city := range []bool{false, true} {
		s := explorerHelpersOpening(t, city)
		p := s.Turn
		id := s.Catan.Players[p].Helper.ID
		if err := s.EliminateCatan(p); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Players[p].Helper != nil || !slices.Contains(s.Catan.HelperDisplay, id) {
			t.Fatal("departing helper lost")
		}
		explorerFishingRestore(t, s)
		serial := s.Catan.TurnSerial
		for i := 0; i < 200 && s.Catan.TurnSerial < serial+4; i++ {
			explorerHelpersStep(t, s)
		}
		if s.Catan.TurnSerial < serial+4 {
			t.Fatal("departure stopped game")
		}
	}
	s := explorerHelpersFixture(t, false, 11)
	for name, damage := range map[string]func(*State){
		"version":   func(s *State) { s.Catan.Explorer.Helpers.Rules = "unknown" },
		"disabled":  func(s *State) { s.Catan.Options.Helpers = false },
		"duplicate": func(s *State) { s.Catan.HelperDisplay = append(s.Catan.HelperDisplay, s.Catan.Players[0].Helper.ID) },
		"missing":   func(s *State) { s.Catan.Explorer.Helpers = nil },
		"future":    func(s *State) { s.Catan.Players[0].Helper.AcquiredTurn = s.Catan.TurnSerial + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			next := clone(*s)
			damage(&next)
			if next.validateCatanExplorer() == nil {
				t.Fatal("corrupt helpers restored")
			}
		})
	}
	before := clone(*s)
	if err := s.enableCatanExplorerHelpers(false); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("late enable not atomic")
	}
}

func TestCatanExplorerHelpersBotUsesOnlyVisibleState(t *testing.T) {
	s := explorerHelpersOpening(t, true)
	for i := 0; i < 20 && s.Phase != "catan_turn"; i++ {
		explorerHelpersStep(t, s)
	}
	if s.Phase != "catan_turn" {
		t.Fatal("fixture production did not finish")
	}
	p := s.Turn
	explorerHelpersHand(t, s, p, []int{4, 4, 4, 4, 4, 0, 0, 0})
	explorerHelpersHand(t, s, (p+1)%3, []int{2, 0, 0, 0, 0, 0, 0, 0})
	explorerHelpersHand(t, s, (p+2)%3, []int{0, 2, 0, 0, 0, 0, 0, 0})
	for _, id := range []int{1, 2, 4, 6, 7, 8, 9, 10, 11, 12} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			next := clone(*s)
			explorerHelpersAssign(t, &next, p, id)
			before := clone(next)
			a, err := next.BotAction(p)
			if err != nil || !reflect.DeepEqual(next, before) {
				t.Fatal("helper bot mutated state", err)
			}
			assertExplorerFullBotPrivate(t, &next, p, a)
		})
	}
}

func TestCatanExplorerHelpersSubstituteWithoutOriginalResource(t *testing.T) {
	for _, id := range []int{2, 6} {
		s := explorerHelpersFixture(t, true, id)
		p := s.Turn
		hand := []int{0, 2, 0, 0, 0, 0, 0, 0}
		if id == 6 {
			hand[1], hand[2] = 1, 1
		}
		explorerHelpersHand(t, s, p, hand)
		choices := s.catanExplorerHelperChoices(p)
		if len(choices) == 0 {
			t.Fatal("substitute required original wood", id)
		}
		a := choices[0]
		bad := a
		bad.Tokens = make([]int, 5)
		explorerHelpersReject(t, s, p, bad)
		explorerHelpersAct(t, s, p, a)
		if sum(s.Catan.Players[p].Resources) != 0 {
			t.Fatal("wrong substitute payment", id)
		}
	}
}

func TestCatanExplorerHelpersCrewCityAndTaskRestrictions(t *testing.T) {
	s := explorerHelpersOpening(t, true)
	for i := 0; i < 20 && s.Phase != "catan_turn"; i++ {
		explorerHelpersStep(t, s)
	}
	if s.Phase != "catan_turn" {
		t.Fatal("fixture production")
	}
	p := s.Turn
	explorerHelpersAssign(t, s, p, 8)
	explorerHelpersHand(t, s, p, []int{0, 0, 1, 1, 3, 0, 0, 0})
	harbor, city := -1, -1
	for _, v := range s.Catan.Vertices {
		if v.Owner == p {
			if v.Harbor {
				harbor = v.ID
			} else if s.Catan.cityAt(v.ID) {
				city = v.ID
			}
		}
	}
	if harbor < 0 || city < 0 {
		t.Fatal("opening buildings")
	}
	crew := p*11 + 2
	explorerHelpersAct(t, s, p, Action{Type: "catan_explorer_unit", Card: crew, Choice: "harbor", Target: harbor})
	// Explicit later-game fixture: this starting city is now an ordinary village.
	s.Catan.Vertices[city].Level = 1
	s.Catan.Players[p].Score--
	explorerFishingRestore(t, s)
	a := Action{Type: "catan_city", Vertex: city, Card: crew, Skill: "helper", Prompt: int(s.Catan.TurnSerial)}
	bad := a
	bad.Card = p * 11 // A settler cannot substitute for crew in mission games.
	explorerHelpersReject(t, s, p, bad)
	bad = a
	bad.Card = (p+1)%3*11 + 2
	explorerHelpersReject(t, s, p, bad)
	explorerHelpersAct(t, s, p, a)
	if !s.Catan.cityAt(city) || s.Catan.Explorer.Cargo.Units[crew].Kind != "supply" || sum(s.Catan.Players[p].Resources) != 0 {
		t.Fatal("crew city discount/return")
	}
}
