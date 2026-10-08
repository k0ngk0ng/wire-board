package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func tribeProgressFixture(t *testing.T, card int) (*State, int) {
	t.Helper()
	s, edge := tribeGame(t)
	g := s.Catan
	s.enableCitiesKnights()
	s.Turn = 0
	s.Phase = "catan_turn"
	g.Seafarers.VictoryPoints = 15
	g.Seafarers.Pirate = -1
	g.initTribeProgress()
	track := catanProgressRules[card].Track
	at := slices.Index(g.CitiesKnights.ProgressDecks[track], card)
	g.CitiesKnights.ProgressDecks[track] = slices.Delete(g.CitiesKnights.ProgressDecks[track], at, at+1)
	g.tribe().Development = []CatanTribeDevelopment{{Edge: edge, Card: card}}
	return s, edge
}

func TestCatanTribeProgressRewardsAndPrivacy(t *testing.T) {
	for _, card := range []int{0, 7, 9, 13, 16, 23} {
		t.Run(fmt.Sprint(card), func(t *testing.T) {
			s, edge := tribeProgressFixture(t, card)
			if err := s.Catan.validateTribeProgress(); err != nil {
				t.Fatal(err)
			}
			for _, viewer := range []int{-1, 0, 1} {
				v := s.View(viewer)["catan"].(map[string]any)
				tr := v["seafarers"].(map[string]any)["tribe"].(map[string]any)
				b, _ := json.Marshal(tr["development"])
				want, _ := json.Marshal([]map[string]int{{"edge": edge}})
				if string(b) != string(want) {
					t.Fatal("hidden reward leaked", string(b))
				}
			}
			helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
			g := s.Catan
			if len(g.tribe().Development) != 0 || sum(g.Players[0].Dev) != 0 || s.Phase != "catan_turn" {
				t.Fatal("reward continuation")
			}
			p := g.CitiesKnights.Players[0]
			if catanProgressRules[card].Victory {
				if !slices.Equal(p.PublicProgress, []int{card}) || len(p.Progress) != 0 || p.ProgressPoints != 1 {
					t.Fatal("public victory reward")
				}
			} else {
				if !slices.Equal(p.Progress, []int{card}) {
					t.Fatal("private reward")
				}
				v := s.View(1)["catan"].(map[string]any)["citiesKnights"].(map[string]any)["players"].([]any)[0].(map[string]any)
				if _, leak := v["progress"]; leak {
					t.Fatal("opponent reward leaked")
				}
				event := g.CitiesKnights.ProgressEvents[len(g.CitiesKnights.ProgressEvents)-1]
				if event.Card != nil {
					t.Fatal("animation reward leaked")
				}
				if card != 0 {
					helperApply(t, s, 0, Action{Type: "catan_progress", Card: card, Choice: "skip"})
				}
			}
			before := clone(*s)
			if err := s.catanCollectTribe(0, edge); err != nil || !reflect.DeepEqual(before, *s) {
				t.Fatal("reward duplicated", err)
			}
			ckProgressStock(t, s.Catan)
			restored := clone(*s)
			if err := restored.Catan.validateTribeProgress(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatanTribeProgressFreeRoutePortOverflowAndVictory(t *testing.T) {
	s, edge := tribeProgressFixture(t, 13)
	ckProgressGive(t, s, 0, 1, 2, 3, 4)
	s.Catan.tribe().Ports = []CatanPort{{Edge: edge, Resource: 0}}
	s.Catan.FreeRoads = 2
	s.Catan.ResumePhase = "catan_turn"
	s.Phase = "catan_roads"
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if s.Phase != "catan_port" || len(s.Catan.CitiesKnights.Players[0].Progress) != 5 || s.Catan.FreeRoads != 2 {
		t.Fatal("port interrupted free ship")
	}
	s = referenceEventRestore(t, s)
	helperReject(t, s, 1, Action{Type: "catan_port", Edge: edge})
	helperApply(t, s, 0, Action{Type: "catan_port", Edge: edge})
	if s.Catan.FreeRoads != 1 || s.Phase != "catan_roads" {
		t.Fatal("port lost second free route", s.Phase)
	}
	a, err := s.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, 0, a)
	if s.Phase != "catan_turn" {
		t.Fatal("free route failed to finish", s.Phase)
	}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	if s.Phase != "catan_progress_end" {
		t.Fatal("own-turn fifth reward must discard at end", s.Phase)
	}
	s = referenceEventRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_progress_discard", Cards: []int{13}})
	if len(s.Catan.CitiesKnights.Players[0].Progress) != 4 {
		t.Fatal("end discard")
	}
	ckProgressStock(t, s.Catan)
	s, edge = tribeProgressFixture(t, 9)
	s.Catan.tribe().Points[0] = 13
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if !s.Finished || !slices.Equal(s.Winners, []int{0}) || s.Catan.Players[0].Score != 15 {
		t.Fatal("reward public VP must win immediately", s.Catan.Players[0].Score)
	}
}

func TestCatanTribeProgressCorruptAndOrdinaryIsolation(t *testing.T) {
	s, edge := tribeProgressFixture(t, 13)
	for _, mutate := range []func(*Catan){
		func(g *Catan) { g.tribe().ProgressRules = "unknown" },
		func(g *Catan) { g.tribe().Development[0].Card = 99 },
		func(g *Catan) { g.tribe().Development[0].Edge = -1 },
		func(g *Catan) { g.CitiesKnights.ProgressDecks[0] = g.CitiesKnights.ProgressDecks[0][1:] },
		func(g *Catan) { g.Players[0].Dev[0] = 1 },
		func(g *Catan) { g.tribe().Development = append(g.tribe().Development, g.tribe().Development[0]) },
	} {
		bad := clone(*s)
		mutate(bad.Catan)
		if bad.Catan.validateTribeProgress() == nil {
			t.Fatal("corruption accepted")
		}
		helperReject(t, &bad, 0, Action{Type: "catan_ship", Edge: edge})
	}
	ordinary, _ := tribeGame(t)
	if ordinary.Catan.tribe().ProgressRules != "" || ordinary.Catan.validateTribeProgress() != nil {
		t.Fatal("ordinary changed")
	}
	ordinary.Catan.tribe().ProgressRules = CatanTribeProgressRules
	if ordinary.Catan.validateTribeProgress() == nil {
		t.Fatal("ordinary accepted progress")
	}
}

func TestCatanTribeKnightsEventsAllFaces(t *testing.T) {
	for n := 3; n <= 6; n++ {
		layouts := []string{"fixed"}
		if n <= 4 {
			layouts = append(layouts, "variable")
		}
		for _, layout := range layouts {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				seed, err := NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "tribe", Layout: layout}, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err = seed.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				if err = seed.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				if err = seed.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
				for step := 0; seed.Catan.setup() && step < 100; step++ {
					a, e := seed.BotAction(ckActor(seed))
					if e != nil {
						t.Fatal(e)
					}
					helperApply(t, seed, ckActor(seed), a)
				}
				if seed.Catan.setup() {
					t.Fatal("opening stalled")
				}
				for kind := range catanCardEventNames {
					cp := clone(*seed)
					s := &cp
					referenceEventTop(t, s, kind)
					if err = s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
						t.Fatal(kind, err)
					}
					s = finishEventKnightsResponses(t, s)
					ckProgressStock(t, s.Catan)
					if !s.Catan.RevealedEvent.ProductionStarted || s.Catan.tribe().ProgressRules != CatanTribeProgressRules {
						t.Fatal("production/reward recipe")
					}
					if n > 4 {
						before := clone(s.Catan.EventDeck)
						helperApply(t, s, s.Turn, Action{Type: "catan_end"})
						if !s.Catan.Paired.Second || !reflect.DeepEqual(before, s.Catan.EventDeck) {
							t.Fatal("paired redrew")
						}
					}
				}
			})
		}
	}
}

func TestCatanTribeProgressMovingShipAndAlchemy(t *testing.T) {
	s, edge := tribeProgressFixture(t, 0)
	g := s.Catan
	g.Edges[edge].Owner, g.Edges[edge].Ship = 0, true
	destinations := g.shipDestinations(0, edge)
	if len(destinations) == 0 {
		t.Fatal("no fixture destination")
	}
	target := destinations[0]
	g.tribe().Development[0].Edge = target
	helperApply(t, s, 0, Action{Type: "catan_move_ship", Edge: edge, Target: target})
	if !slices.Equal(s.Catan.CitiesKnights.Players[0].Progress, []int{0}) {
		t.Fatal("moving ship lost reward")
	}
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 3}})
	s.Phase = "catan_roll"
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 3}})
	if s.Catan.RollID != 1 || !slices.Equal(s.Catan.Dice, []int{2, 3}) || len(s.Catan.CitiesKnights.Players[0].Progress) != 0 {
		t.Fatal("alchemy reward timing")
	}
	ckProgressStock(t, s.Catan)
}
