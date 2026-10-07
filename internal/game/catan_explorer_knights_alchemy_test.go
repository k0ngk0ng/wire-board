package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerAlchemyReject(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	before := clone(*s)
	calls := 0
	if err := s.catanExplorerCityRollAction(p, a, func(n int) int { calls++; return 3 }); err == nil {
		t.Fatal("invalid alchemy accepted")
	}
	if calls != 0 || !reflect.DeepEqual(*s, before) {
		t.Fatal("invalid request consumed randomness/card or changed state")
	}
}

func TestCatanExplorerCityAlchemyDiceAndIndependentEvent(t *testing.T) {
	for _, n := range []int{3, 6} {
		for face := range 6 {
			t.Run(fmt.Sprintf("%d/face%d", n, face), func(t *testing.T) {
				s := explorerCityProductionFixture(t, n)
				ckProgressGive(t, s, 0, 0)
				claims := explorerCityClaims(s.Catan, 2)
				a := Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: 1, Color: 5, Target: 5}
				calls := 0
				if err := s.catanExplorerCityRollAction(0, a, func(sides int) int {
					if sides != 6 {
						t.Fatal("wrong event die")
					}
					calls++
					return face
				}); err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				barbarian := 0
				if face >= 3 {
					barbarian = 1
				}
				if calls != 1 || !slices.Equal(g.Dice, []int{1, 1}) || g.Explorer.Economy.Turn.Dice != [2]int{1, 1} || g.CitiesKnights.EventDie != face || g.CitiesKnights.BarbarianPosition != barbarian || g.RollID != 1 || s.Phase != "catan_turn" || slices.Contains(g.CitiesKnights.Players[0].Progress, 0) {
					t.Fatal("Alchemy controlled event die, skipped event or failed to consume card")
				}
				for p, gold := range g.Explorer.Economy.Gold {
					want := 2
					if sum(claims[p]) == 0 {
						want++
					}
					if gold != want {
						t.Fatal("missing empty-production gold")
					}
				}
				explorerCityRestore(t, s)
				explorerCityActionReject(t, s, 0, a)
			})
		}
	}
	for red := 1; red <= 6; red++ {
		for yellow := 1; yellow <= 6; yellow++ {
			s := explorerCityProductionFixture(t, 3)
			ckProgressGive(t, s, 0, 0)
			if err := s.catanExplorerCityRollAction(0, Action{Type: "catan_progress", Card: 0, Tokens: []int{red, yellow}, Prompt: 1}, func(int) int { return 3 }); err != nil {
				t.Fatal(red, yellow, err)
			}
			if !slices.Equal(s.Catan.Dice, []int{red, yellow}) {
				t.Fatal("selected dice changed")
			}
			if red+yellow == 7 && s.Phase != "catan_explorer_pirate_place" {
				t.Fatal("chosen seven did not activate explorer pirate")
			}
			explorerCityRestore(t, s)
		}
	}
}

func TestCatanExplorerCityAlchemyGuardsAndFailedPayout(t *testing.T) {
	for _, reason := range []string{"missing", "foreign", "stale", "short", "zero", "seven", "extra", "skip", "skill", "other_card", "ordinary_selected"} {
		t.Run(reason, func(t *testing.T) {
			s := explorerCityProductionFixture(t, 3)
			ckProgressGive(t, s, 0, 0)
			p := 0
			a := Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: 1}
			switch reason {
			case "missing":
				s.Catan.CitiesKnights.Players[0].Progress = nil
				s.Catan.CitiesKnights.returnProgress([]int{0})
			case "foreign":
				p = 1
			case "stale":
				a.Prompt = 0
			case "short":
				a.Tokens = []int{1}
			case "zero":
				a.Tokens = []int{0, 1}
			case "seven":
				a.Tokens = []int{7, 1}
			case "extra":
				a.Tokens = []int{1, 1, 1}
			case "skip":
				a.Choice = "skip"
			case "skill":
				a.Skill = "alchemy"
			case "other_card":
				a.Card = 1
			case "ordinary_selected":
				a.Type = "catan_roll"
			}
			explorerAlchemyReject(t, s, p, a)
		})
	}
	s := explorerCityProductionFixture(t, 3)
	ckProgressGive(t, s, 0, 0)
	x := s.Catan.Explorer
	// Exhaust the safety bound, not the physical coin supply: ordinary coin
	// shortages now use the explicit supplemental ledger rule.
	x.Economy.GoldIssued = catanExplorerGoldLedgerLimit
	x.Economy.Gold[1] += x.Economy.GoldIssued
	x.Economy.Gold[1] += x.Economy.GoldBank
	x.Economy.GoldBank = 0
	before := clone(*s)
	a := Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: 1}
	if err := s.catanExplorerCityRollAction(0, a, func(int) int { return 3 }); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("failed downstream payout consumed Alchemy/event/roll")
	}
	if err := s.catanExplorerCityRollAction(0, a, func(int) int { return 6 }); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("invalid server random result changed state")
	}
	if err := s.catanExplorerCityRollAction(0, a, nil); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("nil source accepted")
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityAlchemyOverflowAndPillageResume(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	ckProgressGive(t, s, 0, 0)
	ckProgressGive(t, s, 1, 3, 4, 5, 6)
	ckProgressTop(t, s, 1)
	s.Catan.CitiesKnights.Players[1].Improvements[0] = 1
	a := Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: 1}
	if err := s.catanExplorerCityRollAction(0, a, func(int) int { return 0 }); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_progress_discard" || s.CatanPendingActor() != 1 || s.Catan.Explorer.Economy.Gold[0] != 2 || s.Catan.Explorer.Cargo.Turn != nil {
		t.Fatal("production ran before progress overflow response")
	}
	explorerCityRestore(t, s)
	if err := s.catanExplorerCityRespond(1, Action{Type: "catan_progress_discard", Cards: []int{3}, Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || s.Catan.Explorer.Economy.Gold[0] != 3 || s.Catan.RollID != 1 || len(s.Catan.CitiesKnights.Players[1].Progress) != 4 {
		t.Fatal("overflow failed to resume one production")
	}
	explorerCityRestore(t, s)

	s = explorerCityProductionFixture(t, 3)
	ckProgressGive(t, s, 0, 0)
	s.Catan.CitiesKnights.BarbarianPosition = 6
	x := s.Catan.Explorer
	x.Economy.Gold[1] += x.Economy.GoldBank
	x.Economy.GoldBank = 0
	if err := s.catanExplorerCityRollAction(0, a, func(int) int { return 3 }); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 3; step++ {
		if s.Phase != "catan_pillage" {
			t.Fatal("Alchemy bypassed barbarian pillage")
		}
		p := s.CatanPendingActor()
		v := s.Catan.pillageSites(p)[0]
		response := Action{Type: "catan_pillage", Vertex: v, Prompt: 1}
		if step == 2 {
			if slices.Contains(s.Catan.CitiesKnights.Players[0].Progress, 0) {
				t.Fatal("pillage refunded already-played Alchemy")
			}
		}
		if err := s.catanExplorerCityRespond(p, response); err != nil {
			t.Fatal(err)
		}
		explorerCityRestore(t, s)
	}
	if s.Phase != "catan_turn" || s.Catan.RollID != 1 || s.Catan.CitiesKnights.Invasions != 1 || s.Catan.Explorer.Economy.GoldIssued != 3 {
		t.Fatal("pillage did not resume Alchemy production")
	}
}

func TestCatanExplorerCityOrdinaryRollActionAndSecondaryCannotRoll(t *testing.T) {
	s := explorerCityProductionFixture(t, 6)
	values := []int{0, 1, 5}
	calls := 0
	if err := s.catanExplorerCityRollAction(0, Action{Type: "catan_roll", Prompt: 1}, func(sides int) int {
		if sides != 6 || calls >= len(values) {
			t.Fatal("wrong ordinary roll draws")
		}
		v := values[calls]
		calls++
		return v
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || !slices.Equal(s.Catan.Dice, []int{1, 2}) || s.Catan.CitiesKnights.EventDie != 5 {
		t.Fatal("ordinary dice were not server-selected")
	}
	explorerCityRestore(t, s)
	explorerCityFinishAction(t, s)
	if !s.Catan.Explorer.Economy.Turn.NoProduction || s.Turn != 3 {
		t.Fatal("missing secondary segment")
	}
	ckProgressGive(t, s, 3, 0)
	explorerCityActionReject(t, s, 3, Action{Type: "catan_roll", Prompt: int(s.Catan.TurnSerial)})
	explorerCityActionReject(t, s, 3, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: int(s.Catan.TurnSerial)})
}
