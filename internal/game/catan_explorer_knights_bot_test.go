package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerCityBotStep(t *testing.T, s *State, p int) Action {
	t.Helper()
	before := clone(*s)
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(s.Phase, p, err)
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("choosing a bot action mutated the live game")
	}
	if a.Prompt != int(s.Catan.TurnSerial) {
		t.Fatal("bot lost current sequence", a)
	}
	explorerCityStateAct(t, s, p, a)
	return a
}

func TestCatanExplorerCityBotCommodityDiscardAndTimeout(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityStateStarted(t, n, "explorers-and-pirates")
			for p := range s.Catan.Players {
				explorerDevelopmentGrant(t, s, p, 5+p%3, 8)
			}
			if err := s.catanExplorerCityRoll(3, 4, 0); err != nil {
				t.Fatal(err)
			}
			explorerCityStateRestore(t, s)
			for p, due := range s.Catan.DiscardDue {
				if due == 0 {
					t.Fatal("expected each player to discard")
				}
				a, err := s.BotAction(p)
				if err != nil || len(a.Tokens) != 8 || sum(a.Tokens) != due || sum(a.Tokens[5:]) == 0 {
					t.Fatal("commodity discard missing", p, a, err)
				}
			}
			s.AutoCatanPending()
			explorerCityStateRestore(t, s)
			if s.Phase != "catan_explorer_pirate_place" || sum(s.Catan.DiscardDue) != 0 {
				t.Fatal("timeout did not finish all simultaneous discards")
			}
			for step := 0; s.CatanPendingActor() >= 0; step++ {
				if step > 2 {
					t.Fatal("pirate response stalled")
				}
				explorerCityBotStep(t, s, s.CatanPendingActor())
			}
		})
	}
}

func TestCatanExplorerCityBotRequiredResponses(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityStateStarted(t, n, "pirate-lairs")
			s.Catan.CitiesKnights.BarbarianPosition = 6
			if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
				t.Fatal(err)
			}
			for count := 0; s.CatanPendingActor() >= 0; count++ {
				if count >= n {
					t.Fatal("pillage queue stalled")
				}
				before := s.Catan.Explorer.ActionID
				s.AutoCatanPending()
				if s.Catan.Explorer.ActionID != before+1 {
					t.Fatal("timeout failed to apply city response")
				}
				explorerCityStateRestore(t, s)
			}
			p, other := s.Turn, (s.Turn+1)%n
			ckProgressGive(t, s, p, 10, 0, 1, 2, 3, 4, 5)
			explorerDevelopmentGrant(t, s, p, 0, 1)
			explorerDevelopmentGrant(t, s, other, 5, 1)
			explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 10})
			explorerCityStateAct(t, s, p, Action{Type: "catan_commercial_offer", Target: other, Color: 0})
			if a := explorerCityBotStep(t, s, other); a.Type != "catan_commercial_harbor" {
				t.Fatal("off-turn response missing", a)
			}
			explorerCityStateAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
			if a := explorerCityBotStep(t, s, p); a.Type != "catan_progress_discard" || s.Phase != "catan_explorer_move" {
				t.Fatal("progress overflow did not resume sailing", a, s.Phase)
			}
		})
	}
}

func TestCatanExplorerCityBotAlchemyRoadsAndDevelopment(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "explorers-and-pirates")
	p := s.Turn
	ckProgressGive(t, s, p, 0)
	if a := explorerCityBotStep(t, s, p); a.Type != "catan_progress" || a.Card != 0 {
		t.Fatal("owned Alchemy ignored", a)
	}
	explorerCityStateReady(t, s)
	ckProgressGive(t, s, p, 7)
	if a := explorerCityBotStep(t, s, p); a.Type != "catan_progress" || a.Card != 7 {
		t.Fatal("owned Road Building ignored", a)
	}
	for steps := 0; s.Phase == "catan_roads"; steps++ {
		if steps >= 2 {
			t.Fatal("free roads stalled")
		}
		explorerCityBotStep(t, s, p)
	}
	// Give only science commodities, from the physical bank. The bot should
	// develop a city, not spend gold trying to buy an unavailable commodity.
	for c, n := range s.Catan.Players[p].Resources {
		s.Catan.Bank[c] += n
		s.Catan.Players[p].Resources[c] = 0
	}
	explorerDevelopmentGrant(t, s, p, 5, 1)
	if a := explorerCityBotStep(t, s, p); a.Type != "catan_improvement" || a.Color != CatanScience {
		t.Fatal("city development absent from mission bot", a)
	}
}

func TestCatanExplorerCityBotCommercialOfferStable(t *testing.T) {
	s := explorerCityStateStarted(t, 6, "explorers-and-pirates")
	explorerCityStateReady(t, s)
	p := s.Turn
	ckProgressGive(t, s, p, 10, 10)
	explorerDevelopmentGrant(t, s, p, 0, 2)
	explorerDevelopmentGrant(t, s, (p+1)%6, 5, 1)
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 10})
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 10})
	before := clone(*s)
	for range 20 {
		a, err := s.BotAction(p)
		if err != nil || a.Type != "catan_commercial_offer" || a.Card != 0 || !reflect.DeepEqual(*s, before) {
			t.Fatal("duplicate Commercial Harbors gave unstable offers", a, err)
		}
	}
	explorerCityBotStep(t, s, p)
	if actor := s.CatanPendingActor(); actor >= 0 {
		explorerCityBotStep(t, s, actor)
	}
}

func TestCatanExplorerCityBotBankChoices(t *testing.T) {
	base := explorerCityStateStarted(t, 3, "explorers-and-pirates")
	explorerCityStateReady(t, base)
	p := base.Turn
	for c, n := range base.Catan.Players[p].Resources {
		base.Catan.Bank[c] += n
		base.Catan.Players[p].Resources[c] = 0
	}
	seenBank := false
	for color := 0; color < 8; color++ {
		for _, count := range []int{2, 3, 4, 6} {
			s := clone(*base)
			explorerDevelopmentGrant(t, &s, p, color, count)
			a := explorerCityBotStep(t, &s, p)
			if a.Type == "catan_explorer_bank" {
				seenBank = true
				if a.Color == -1 && a.Target >= 5 {
					t.Fatal("gold bought a commodity", a)
				}
			}
		}
	}
	if !seenBank {
		t.Fatal("bank planning not exercised")
	}
}

func TestCatanExplorerCityBotCommodityBankRate(t *testing.T) {
	base := explorerCityStateStarted(t, 3, "explorers-and-pirates")
	explorerCityStateReady(t, base)
	p := base.Turn
	// Reach science level five through real payments and metropolis choices,
	// so paper can no longer be spent on a cheaper science improvement.
	for level := 1; level <= 5; level++ {
		explorerDevelopmentGrant(t, base, p, 5, level)
		explorerCityStateAct(t, base, p, Action{Type: "catan_improvement", Color: CatanScience})
		if actor := base.CatanPendingActor(); actor >= 0 {
			explorerCityBotStep(t, base, actor)
		}
	}
	for c, n := range base.Catan.Players[p].Resources {
		base.Catan.Bank[c] += n
		base.Catan.Players[p].Resources[c] = 0
	}
	x := base.Catan.Explorer
	x.Economy.GoldBank += x.Economy.Gold[p]
	x.Economy.Gold[p] = 0
	for _, count := range []int{3, 4} {
		s := clone(*base)
		explorerDevelopmentGrant(t, &s, p, 5, count)
		a := explorerCityBotStep(t, &s, p)
		if count == 3 && a.Type == "catan_explorer_bank" {
			t.Fatal("three commodities used at the ordinary resource rate", a)
		}
		if count == 4 && (a.Type != "catan_explorer_bank" || a.Color != 5 || s.Catan.Players[p].Resources[5] != 0) {
			t.Fatal("four surplus commodities not exchanged legally", a)
		}
	}
}

func TestCatanExplorerCityBotPrivateProgress(t *testing.T) {
	base := explorerCityStateStarted(t, 3, "explorers-and-pirates")
	explorerCityStateReady(t, base)
	p := base.Turn
	for color := 0; color < 8; color++ {
		explorerDevelopmentGrant(t, base, p, color, 2)
	}
	for other := range base.Catan.Players {
		if other != p {
			explorerDevelopmentGrant(t, base, other, 5+other%3, 2)
			ckProgressGive(t, base, other, 0, 1)
		}
	}
	for _, card := range []int{1, 2, 3, 4, 5, 6, 7, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 24} {
		t.Run(fmt.Sprint(card), func(t *testing.T) {
			s := clone(*base)
			// Card 1's two physical copies are held by the opponents above.
			if card == 1 {
				other := (p + 1) % 3
				s.Catan.CitiesKnights.Players[other].Progress = slices.DeleteFunc(s.Catan.CitiesKnights.Players[other].Progress, func(id int) bool { return id == 1 })
				s.Catan.CitiesKnights.ProgressDecks[0] = append(s.Catan.CitiesKnights.ProgressDecks[0], 1)
			}
			ckProgressGive(t, &s, p, card)
			s.Catan.CitiesKnights.Invasions = 1
			explorerCityStateRestore(t, &s)
			before := clone(s)
			first, err := s.BotAction(p)
			if err != nil || !reflect.DeepEqual(before, s) {
				t.Fatal("bot mutated live game", err)
			}
			// Existing mission helper swaps fog, lair tokens and opponents' card
			// composition while preserving public counts and complete validity.
			assertExplorerFullBotPrivate(t, &s, p, first)
			next := clone(s)
			k := next.Catan.CitiesKnights
			for track := range k.ProgressDecks {
				slices.Reverse(k.ProgressDecks[track])
			}
			for other := range k.Players {
				if other == p {
					continue
				}
				for i, id := range k.Players[other].Progress {
					track := catanProgressRules[id].Track
					for j, replacement := range k.ProgressDecks[track] {
						if replacement != id && !catanProgressRules[replacement].Victory {
							k.Players[other].Progress[i], k.ProgressDecks[track][j] = replacement, id
							break
						}
					}
				}
			}
			explorerCityStateRestore(t, &next)
			if !reflect.DeepEqual(s.View(p), next.View(p)) {
				t.Fatal("private card permutation changed view")
			}
			second, err := next.BotAction(p)
			if err != nil || !reflect.DeepEqual(first, second) {
				t.Fatal("bot read hidden progress cards", first, second, err)
			}
			explorerCityStateAct(t, &s, p, first)
		})
	}
}

func TestCatanExplorerCityBotNaturalTurnRotations(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				// Legal opening is selected by the existing completion-search
				// fixture; everything afterwards uses actual BotAction/Apply.
				// No extra cards, pieces, resources or die outcomes are injected.
				s := explorerCityStateStarted(t, n, scenario)
				seen := map[string]int{}
				for steps := 0; s.Catan.RollID < 2*n; steps++ {
					if steps >= 800 || s.Finished {
						t.Fatal("bot rotations stalled", s.Phase, s.Round, seen)
					}
					p := s.CatanPendingActor()
					if p < 0 {
						p = s.Turn
						if s.Phase == "catan_discard" {
							for other, due := range s.Catan.DiscardDue {
								if due > 0 {
									p = other
									break
								}
							}
						}
					}
					a := explorerCityBotStep(t, s, p)
					seen[a.Type]++
				}
				if seen["catan_end"] < 2*n-1 || seen["catan_explorer_sail"] == 0 {
					t.Fatal("missing real turn/movement coverage", seen)
				}
				t.Log("two production rotations", seen)
			})
		}
	}
}
