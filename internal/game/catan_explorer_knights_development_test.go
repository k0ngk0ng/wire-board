package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerDevelopmentCity(t *testing.T, s *State, p int) int {
	t.Helper()
	for _, v := range s.Catan.Vertices {
		if v.Owner == p && s.Catan.cityAt(v.ID) {
			return v.ID
		}
	}
	t.Fatal("fixture city missing", p)
	return -1
}
func explorerDevelopmentGrant(t *testing.T, s *State, p, card, amount int) {
	t.Helper()
	if amount > s.Catan.Bank[card] {
		t.Fatal("fixture exhausted card", card)
	}
	s.Catan.Bank[card] -= amount
	s.Catan.Players[p].Resources[card] += amount
}
func explorerDevelopmentImprove(t *testing.T, s *State, track int, crane bool) {
	t.Helper()
	a := Action{Type: "catan_improvement", Color: track, Prompt: int(s.Catan.TurnSerial)}
	if crane {
		a.Type, a.Card = "catan_progress", 1
		ckProgressGive(t, s, s.Turn, 1)
	}
	before := clone(*s)
	cost := s.Catan.CitiesKnights.Players[s.Turn].Improvements[track] + 1
	if crane {
		cost--
	}
	if err := s.catanExplorerCityAction(s.Turn, a); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Players[s.Turn].Resources[5+track] != before.Catan.Players[s.Turn].Resources[5+track]-cost || s.Catan.Bank[5+track] != before.Catan.Bank[5+track]+cost || s.Catan.CitiesKnights.Players[s.Turn].Improvements[track] != before.Catan.CitiesKnights.Players[s.Turn].Improvements[track]+1 {
		t.Fatal("wrong improvement cost/level")
	}
	explorerCityRestore(t, s)
}
func explorerDevelopmentChoose(t *testing.T, s *State, city int) {
	t.Helper()
	if err := s.catanExplorerCityRespond(s.Turn, Action{Type: "catan_metropolis", Vertex: city, Prompt: int(s.Catan.TurnSerial)}); err != nil {
		t.Fatal(err)
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityDevelopmentTracksAndMetropolis(t *testing.T) {
	for _, n := range []int{3, 6} {
		for track := range 3 {
			t.Run(fmt.Sprintf("%d/track%d", n, track), func(t *testing.T) {
				s := explorerCityProductionFixture(t, n)
				if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
					t.Fatal(err)
				}
				city := explorerDevelopmentCity(t, s, 0)
				start := s.Catan.Players[0].Score
				for level := 1; level <= 5; level++ {
					explorerDevelopmentGrant(t, s, 0, 5+track, level)
					explorerDevelopmentImprove(t, s, track, false)
					if level == 4 {
						if s.Phase != "catan_metropolis" || s.CatanPendingActor() != 0 || s.Catan.Explorer.Cargo.Turn.Phase != "action" {
							t.Fatal("metropolis did not pause active segment")
						}
						for _, v := range s.Catan.Vertices {
							if v.Harbor && v.Owner == 0 {
								explorerCityReject(t, s, 0, Action{Type: "catan_metropolis", Vertex: v.ID, Prompt: int(s.Catan.TurnSerial)})
							}
						}
						explorerCityReject(t, s, 1, Action{Type: "catan_metropolis", Vertex: city, Prompt: int(s.Catan.TurnSerial)})
						explorerCityReject(t, s, 0, Action{Type: "catan_metropolis", Vertex: city, Prompt: 0})
						explorerCityActionReject(t, s, 0, Action{Type: "catan_improvement", Color: track, Prompt: int(s.Catan.TurnSerial)})
						production := clone(*s.Catan.Explorer.Economy)
						cargo := clone(*s.Catan.Explorer.Cargo)
						explorerDevelopmentChoose(t, s, city)
						if !reflect.DeepEqual(production, *s.Catan.Explorer.Economy) || !reflect.DeepEqual(cargo, *s.Catan.Explorer.Cargo) {
							t.Fatal("selection restarted production/cargo")
						}
					}
					if s.Phase != "catan_turn" || s.Catan.CitiesKnights.Pending != nil {
						t.Fatal("did not return to action")
					}
				}
				if s.Catan.cityMetropolisOwner(track) != 0 || s.Catan.Players[0].Score != start+2 {
					t.Fatal("metropolis ownership/points")
				}
				explorerCityActionReject(t, s, 0, Action{Type: "catan_improvement", Color: track, Prompt: int(s.Catan.TurnSerial)})
				explorerCityReject(t, s, 0, Action{Type: "catan_metropolis", Vertex: city, Prompt: int(s.Catan.TurnSerial)})
			})
		}
	}
}

func TestCatanExplorerCityDevelopmentCraneAndEngineering(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	city := explorerDevelopmentCity(t, s, 0)
	// First improvement is free with Crane, subsequent levels save one card.
	explorerDevelopmentImprove(t, s, 0, true)
	explorerDevelopmentGrant(t, s, 0, 5, 1)
	explorerDevelopmentImprove(t, s, 0, true)
	explorerDevelopmentGrant(t, s, 0, 5, 3)
	explorerDevelopmentImprove(t, s, 0, false)
	explorerDevelopmentGrant(t, s, 0, 5, 3)
	explorerDevelopmentImprove(t, s, 0, true)
	explorerDevelopmentChoose(t, s, city)
	ckProgressGive(t, s, 0, 2)
	before := clone(*s.Catan)
	a := Action{Type: "catan_progress", Card: 2, Vertex: city, Prompt: int(s.Catan.TurnSerial)}
	if err := s.catanExplorerCityAction(0, a); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Catan.CitiesKnights.Walls, []int{city}) || !slices.Equal(s.Catan.Players[0].Resources, before.Players[0].Resources) || s.Catan.catanDiscardLimit(0) != 9 {
		t.Fatal("Engineering wall was not free")
	}
	explorerCityRestore(t, s)
	explorerCityActionReject(t, s, 0, a)
	// Failed construction returns neither a card nor its benefit partially.
	ckProgressGive(t, s, 0, 2)
	explorerCityActionReject(t, s, 0, a)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_improvement", Color: 1, Skill: "crane", Prompt: a.Prompt})
}

func TestCatanExplorerCityDevelopmentMetropolisTransferAndVictory(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityProductionFixture(t, n)
			if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
				t.Fatal(err)
			}
			first := explorerDevelopmentCity(t, s, 0)
			// Controlled level-three midgame, then real purchases and handoff.
			s.Catan.CitiesKnights.Players[0].Improvements[1] = 3
			explorerDevelopmentGrant(t, s, 0, 6, 4)
			explorerDevelopmentImprove(t, s, 1, false)
			explorerDevelopmentChoose(t, s, first)
			explorerCityFinishAction(t, s)
			if s.Phase == "catan_roll" {
				if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
					t.Fatal(err)
				}
			}
			p := s.Turn
			second := explorerDevelopmentCity(t, s, p)
			s.Catan.CitiesKnights.Players[p].Improvements[1] = 3
			explorerDevelopmentGrant(t, s, p, 6, 4)
			explorerDevelopmentImprove(t, s, 1, false)
			if s.Catan.cityMetropolisOwner(1) != 0 || s.Phase != "catan_turn" {
				t.Fatal("fourth-level tie stole metropolis")
			}
			explorerDevelopmentGrant(t, s, p, 6, 5)
			explorerDevelopmentImprove(t, s, 1, false)
			// Explicit defender-point fixture makes the +2 exactly win at the
			// scenario's 17-point combined target, including secondary actor.
			s.Catan.CitiesKnights.Players[p].DefenderPoints = 11
			s.catanScores()
			if s.Catan.Players[p].Score != 15 {
				t.Fatal("wrong victory setup")
			}
			explorerDevelopmentChoose(t, s, second)
			if !s.Finished || !slices.Equal(s.Winners, []int{p}) || s.Catan.Players[p].Score != 17 || s.Catan.Players[0].Score != 4 || s.Catan.cityMetropolisOwner(1) != p {
				t.Fatal("wrong metropolis transfer/victory")
			}
		})
	}
}

func TestCatanExplorerCityDevelopmentWallsAndNoCityGuards(t *testing.T) {
	s := explorerCityProductionFixture(t, 6)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	cities := []int{explorerDevelopmentCity(t, s, 0)}
	// Four physically distinct city pieces in a controlled six-player midgame.
	for p := 1; p <= 3; p++ {
		v := explorerDevelopmentCity(t, s, p)
		s.Catan.Vertices[v].Owner = 0
		cities = append(cities, v)
	}
	s.catanScores()
	explorerCityRestore(t, s)
	for _, v := range cities[:3] {
		explorerDevelopmentGrant(t, s, 0, 1, 2)
		before := s.Catan.Players[0].Resources[1]
		if err := s.catanExplorerCityAction(0, Action{Type: "catan_wall", Vertex: v, Prompt: 1}); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Players[0].Resources[1] != before-2 {
			t.Fatal("wall price")
		}
		explorerCityRestore(t, s)
	}
	if s.Catan.catanDiscardLimit(0) != 13 {
		t.Fatal("three-wall hand limit")
	}
	explorerDevelopmentGrant(t, s, 0, 1, 2)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_wall", Vertex: cities[3], Prompt: 1})
	ckProgressGive(t, s, 0, 2)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_progress", Card: 2, Vertex: cities[3], Prompt: 1})
	for _, reason := range []string{"harbor", "no_city", "no_spare_city", "wrong_card", "missing_progress"} {
		t.Run(reason, func(t *testing.T) {
			q := explorerCityProductionFixture(t, 3)
			if err := q.catanExplorerCityRoll(1, 1, 3); err != nil {
				t.Fatal(err)
			}
			g := q.Catan
			city := explorerDevelopmentCity(t, q, 0)
			a := Action{Type: "catan_improvement", Color: 0, Prompt: 1}
			switch reason {
			case "harbor":
				a.Type = "catan_wall"
				for _, v := range g.Vertices {
					if v.Owner == 0 && v.Harbor {
						a.Vertex = v.ID
					}
				}
			case "no_city":
				g.Vertices[city].Level = 1
			case "no_spare_city":
				g.CitiesKnights.Players[0].Improvements = [3]int{4, 3, 0}
				g.CitiesKnights.Metropolises[0] = city
				a.Color = 1
				explorerDevelopmentGrant(t, q, 0, 6, 4)
			case "wrong_card":
				explorerDevelopmentGrant(t, q, 0, 6, 1)
			case "missing_progress":
				a.Type, a.Card = "catan_progress", 1
			}
			q.catanScores()
			explorerCityRestore(t, q)
			explorerCityActionReject(t, q, 0, a)
		})
	}
}

func TestCatanExplorerCityDevelopmentCorruptRestore(t *testing.T) {
	base := explorerCityProductionFixture(t, 3)
	if err := base.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	city := explorerDevelopmentCity(t, base, 0)
	base.Catan.CitiesKnights.Players[0].Improvements[0] = 3
	explorerDevelopmentGrant(t, base, 0, 5, 4)
	explorerDevelopmentImprove(t, base, 0, false)
	for _, reason := range []string{"track", "actor", "level", "phase", "no_city", "duplicate_wall", "bad_wall", "bad_metropolis", "harbor_metropolis", "duplicate_metropolis"} {
		t.Run(reason, func(t *testing.T) {
			s := clone(*base)
			g, k := s.Catan, s.Catan.CitiesKnights
			switch reason {
			case "track":
				k.Pending.Track = 3
			case "actor":
				k.Pending.Players = []int{1}
			case "level":
				k.Players[0].Improvements[0] = 6
			case "phase":
				g.Explorer.Cargo.Turn.Phase = "movement"
			case "no_city":
				g.Vertices[city].Level = 1
			case "duplicate_wall":
				k.Walls = []int{city, city}
			case "bad_wall":
				k.Walls = []int{-1}
			case "bad_metropolis":
				k.Metropolises[1] = -2
			case "harbor_metropolis":
				for _, v := range g.Vertices {
					if v.Owner == 1 && v.Harbor {
						k.Players[1].Improvements[1] = 4
						k.Metropolises[1] = v.ID
						break
					}
				}
			case "duplicate_metropolis":
				other := explorerDevelopmentCity(t, &s, 1)
				k.Players[1].Improvements = [3]int{0, 4, 4}
				k.Metropolises[1], k.Metropolises[2] = other, other
			}
			if err := s.validateExplorerCityProduction(); err == nil {
				t.Fatal("corrupt development restored")
			}
			explorerCityReject(t, &s, 0, Action{Type: "catan_metropolis", Vertex: city, Prompt: 1})
		})
	}
}

func TestCatanExplorerCityDevelopmentAqueductAfterEarnedUpgrade(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	for level := 1; level <= 3; level++ {
		explorerDevelopmentGrant(t, s, 0, CatanPaper, level)
		explorerDevelopmentImprove(t, s, CatanScience, false)
	}
	explorerCityFinishAction(t, s)
	before := clone(*s.Catan)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	if s.CatanPendingActor() != 0 || s.Phase != "catan_aqueduct" || s.Turn != 1 {
		t.Fatal("earned aqueduct not offered on another player's production")
	}
	explorerCityActionReject(t, s, 1, Action{Type: "catan_improvement", Color: 1, Prompt: int(s.Catan.TurnSerial)})
	if err := s.catanExplorerCityRespond(0, Action{Type: "catan_aqueduct", Color: 0, Prompt: int(s.Catan.TurnSerial)}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Players[0].Resources[0] != before.Players[0].Resources[0]+1 || s.Catan.Explorer.Economy.Gold[0] != before.Explorer.Economy.Gold[0]+1 || s.Turn != 1 || s.Phase != "catan_turn" {
		t.Fatal("earned aqueduct did not preserve combined compensation or actor")
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityDevelopmentTwoMetropolisesNeedTwoCities(t *testing.T) {
	s, village := explorerCityVillageFixture(t, 3)
	first := explorerDevelopmentCity(t, s, 0)
	s.Catan.CitiesKnights.Players[0].Improvements[0] = 3
	s.Catan.CitiesKnights.Players[0].Improvements[1] = 3
	explorerDevelopmentGrant(t, s, 0, CatanPaper, 4)
	explorerDevelopmentImprove(t, s, CatanScience, false)
	explorerDevelopmentChoose(t, s, first)
	explorerDevelopmentGrant(t, s, 0, CatanCommodityCloth, 4)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_improvement", Color: CatanCommerce, Prompt: 1})
	if err := s.catanExplorerCityAction(0, Action{Type: "catan_city", Vertex: village, Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	explorerDevelopmentImprove(t, s, CatanCommerce, false)
	explorerCityReject(t, s, 0, Action{Type: "catan_metropolis", Vertex: first, Prompt: 1})
	explorerDevelopmentChoose(t, s, village)
	if s.Catan.CitiesKnights.Metropolises != [3]int{first, village, -1} || s.Catan.Players[0].Score != 8 {
		t.Fatal("two distinct metropolises not scored on actual cities")
	}
	explorerCityRestore(t, s)
}
