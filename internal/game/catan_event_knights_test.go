package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func eventKnightsGame(t *testing.T, n int, scenario string) *State {
	t.Helper()
	var s *State
	var err error
	if scenario == "" {
		s, err = NewCatanCitiesKnights(n, CatanOptions{FiveSix: n > 4})
	} else {
		s, err = NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: scenario}, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		p := ckActor(s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, p, a)
	}
	return referenceEventRestore(t, s)
}

func finishEventKnightsResponses(t *testing.T, s *State) *State {
	t.Helper()
	for step := 0; !s.Finished && s.Phase != "catan_turn" && step < 80; step++ {
		s = referenceEventRestore(t, s)
		p := ckActor(s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err, s.Phase)
		}
		helperApply(t, s, p, a)
	}
	if !s.Finished && s.Phase != "catan_turn" {
		t.Fatal("unresolved responses", s.Phase)
	}
	return referenceEventRestore(t, s)
}

func TestCatanEventKnightsAllFacesAndMaps(t *testing.T) {
	for _, scenario := range []string{"", "shores", "islands", "fog", "desert"} {
		for n := 3; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				for kind := range catanCardEventNames {
					s := eventKnightsGame(t, n, scenario)
					id := referenceEventTop(t, s, kind)
					if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
						t.Fatal(kind, err)
					}
					s = finishEventKnightsResponses(t, s)
					r := s.Catan.RevealedEvent
					if r.Kind != kind || r.Production != catanEventReferenceFaces[id].Production || r.Red != 3 || r.Face != 2 || !r.ProductionStarted || s.Catan.RollID != 1 {
						t.Fatal("wrong independent production/dice", r)
					}
					for viewer := -1; viewer < n; viewer++ {
						v := s.View(viewer)["catan"].(map[string]any)["eventDeck"]
						b, _ := json.Marshal(v)
						var deck map[string]any
						json.Unmarshal(b, &deck)
						if deck["knights"] != CatanEventKnightsRules || deck["deck"] != nil || deck["drawPile"] != nil || deck["alchemyRolls"] != nil {
							t.Fatal("deck privacy/version", deck)
						}
					}
				}
			})
		}
	}
}

func TestCatanEventKnightsAlchemyAcrossNewYear(t *testing.T) {
	s := eventKnightsGame(t, 3, "")
	for roll := 1; roll <= 40; roll++ {
		s.Phase = "catan_roll" // Isolate successive production windows; no free resources or deck reset.
		if roll == 1 || roll == 10 || roll == 34 || roll == 35 {
			before := clone(s.Catan.EventDeck.Deck)
			ckProgressGive(t, s, s.Turn, 0)
			helperReject(t, s, s.Turn, Action{Type: "catan_progress", Card: 0, Tokens: []int{7, 1}})
			helperApply(t, s, s.Turn, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 4}})
			if !reflect.DeepEqual(before, s.Catan.EventDeck.Deck) || s.Catan.RevealedEvent != nil || !s.Catan.EventDeck.alchemyLatest(roll) {
				t.Fatal("alchemy consumed or displayed an event card", roll)
			}
		} else {
			if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
				t.Fatal(roll, err)
			}
		}
		s = finishEventKnightsResponses(t, s)
		if s.Catan.RollID != roll {
			t.Fatal("double production")
		}
	}
	d := s.Catan.EventDeck
	if d.AlchemyRolls != 4 || d.LastAlchemyRoll != 35 || d.Deck.Cycle != 2 || len(d.Deck.Discard) != 5 {
		t.Fatal("wrong draw accounting", d)
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanEventKnightsTradeIncludesCommodities(t *testing.T) {
	s := ckEvent(t)
	if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	s.Catan.LongestOwner = 0
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 1, 0, 0})
	referenceEventTop(t, s, "trade_advantage")
	if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
		t.Fatal(err)
	}
	s = referenceEventRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_event_steal", Target: 1})
	if s.Catan.Players[0].Resources[5] != 1 || s.Catan.Players[1].Resources[5] != 0 {
		t.Fatal("commodity-only opponent not eligible")
	}
	referenceEventRestore(t, s)
}

func TestCatanEventKnightsCorruptRestore(t *testing.T) {
	s := eventKnightsGame(t, 3, "")
	ckProgressGive(t, s, s.Turn, 0)
	helperApply(t, s, s.Turn, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 4}})
	s = finishEventKnightsResponses(t, s)
	for name, corrupt := range map[string]func(*State){
		"version":         func(s *State) { s.Catan.EventDeck.Knights = "unknown" },
		"missing-version": func(s *State) { s.Catan.EventDeck.Knights = "" },
		"negative":        func(s *State) { s.Catan.EventDeck.AlchemyRolls = -1 },
		"too-many":        func(s *State) { s.Catan.EventDeck.AlchemyRolls++ },
		"missing-last":    func(s *State) { s.Catan.EventDeck.LastAlchemyRoll = 0 },
		"future-last":     func(s *State) { s.Catan.EventDeck.LastAlchemyRoll++ },
		"dice":            func(s *State) { s.Catan.Dice[1] = 0 },
		"stale-card":      func(s *State) { s.Catan.RevealedEvent = &CatanRevealedEvent{} },
		"detached":        func(s *State) { s.Catan.CitiesKnights = nil },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			corrupt(&bad)
			if err := bad.validateCatanEventSession(); err == nil {
				t.Fatal("corrupt save accepted")
			}
		})
	}
}

func TestCatanEventKnightsTextBeforeBarbariansAndProduction(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 1, 1, 2, 2})
	s.Phase = "catan_roll"
	g := s.Catan
	g.Tiles[0].Vertices = []int{0, 3, 6}
	for p, v := range []int{0, 3, 6} {
		g.Vertices[v].Owner, g.Vertices[v].Level = p, 2
	}
	g.CitiesKnights.RobberStart = -1
	g.CitiesKnights.BarbarianPosition = 6
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 1, Strength: 1, Active: true}}
	if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	referenceEventTop(t, s, "earthquake")
	calls := 0
	if err := s.catanDrawEventRandom(func(int) int {
		calls++
		if calls == 1 {
			return 1
		}
		return 3
	}); err != nil {
		t.Fatal(err)
	}
	for p := range 3 {
		s = referenceEventRestore(t, s)
		if s.Catan.CitiesKnights.BarbarianPosition != 6 || s.Catan.CitiesKnights.Event != nil {
			t.Fatal("city dice ran before card responses")
		}
		helperApply(t, s, p, Action{Type: "catan_earthquake", Edge: p * 2})
	}
	s = referenceEventRestore(t, s)
	if s.Phase != "catan_pillage" || s.Catan.RevealedEvent.ProductionStarted {
		t.Fatal("pillage not preserved before production")
	}
	if sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("early production")
	}
	s = finishEventKnightsResponses(t, s)
	if s.Catan.CitiesKnights.Invasions != 1 || s.Catan.RollID != 1 || !s.Catan.RevealedEvent.ProductionStarted || s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[0].Resources[5] != 1 {
		t.Fatal("wrong production after invasion")
	}
}

func TestCatanEventKnightsEpidemicAndAlchemyProgressResponse(t *testing.T) {
	for _, alchemy := range []bool{false, true} {
		t.Run(fmt.Sprint(alchemy), func(t *testing.T) {
			s := ckKnightGraph(t, []int{0, 1, 2})
			s.Phase = "catan_roll"
			g := s.Catan
			g.Tiles[0].Vertices = []int{0}
			g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
			g.CitiesKnights.Players[1].Improvements[0] = 1
			ckProgressGive(t, s, 1, 3, 4, 5, 6)
			if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
			if alchemy {
				ckProgressGive(t, s, 0, 0)
				ckProgressTop(t, s, 1)
				if err := s.catanPlayProgressRandom(0, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 4}}, func(int) int { return 0 }); err != nil {
					t.Fatal(err)
				}
			} else {
				ckProgressTop(t, s, 0)
				id := referenceEventTop(t, s, "epidemic")
				s.Catan.Tiles[0].Number = catanEventReferenceFaces[id].Production
				calls := 0
				if err := s.catanDrawEventRandom(func(int) int {
					calls++
					if calls == 1 {
						return 1
					}
					return 0
				}); err != nil {
					t.Fatal(err)
				}
			}
			s = referenceEventRestore(t, s)
			if s.Phase != "catan_progress_discard" || sum(s.Catan.Players[0].Resources) != 0 {
				t.Fatal("production before progress discard")
			}
			helperApply(t, s, 1, Action{Type: "catan_progress_discard", Cards: []int{3}})
			s = referenceEventRestore(t, s)
			commodity := 0
			if alchemy {
				commodity = 1
			}
			if s.Phase != "catan_turn" || s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[0].Resources[5] != commodity {
				t.Fatal("lost epidemic/dice on restore")
			}
			ckProgressStock(t, s.Catan)
		})
	}
}

func TestCatanEventKnightsVictoryBeforeProduction(t *testing.T) {
	s := ckEvent(t)
	s.Catan.CitiesKnights.Players[0].DefenderPoints = 12
	s.Catan.CitiesKnights.Players[0].Improvements[0] = 1
	ckProgressTop(t, s, 9)
	if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	referenceEventTop(t, s, "beautiful_day")
	if err := s.catanDrawEventRandom(func(int) int { return 0 }); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || s.Catan.RevealedEvent.ProductionStarted {
		t.Fatal("victory did not stop production")
	}
	referenceEventRestore(t, s)
}
