package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestCatanExplorerEventsCorruptionAndHiddenOrder(t *testing.T) {
	for _, city := range []bool{false, true} {
		s := explorerEventsGame(t, 3, "explorers-and-pirates", city)
		referenceEventTop(t, s, "earthquake")
		if err := s.catanDrawEventRandom(func(int) int { return 0 }); err != nil {
			t.Fatal(err)
		}
		explorerEventReady(t, s)
		for name, corrupt := range map[string]func(*State){
			"missing-economy":  func(s *State) { s.Catan.Explorer.Economy = nil },
			"missing-rule":     func(s *State) { s.Catan.EventDeck.Explorer = "" },
			"unknown-rule":     func(s *State) { s.Catan.EventDeck.Explorer = "other" },
			"wrong-production": func(s *State) { s.Catan.Explorer.Economy.Turn.Production++ },
			"invented-dice": func(s *State) {
				s.Catan.Explorer.Economy.Turn.Production = 0
				s.Catan.Dice = []int{3, 3}
				s.Catan.Explorer.Economy.Turn.Dice = [2]int{3, 3}
			},
			"event-text": func(s *State) {
				s.Catan.CardEvent = &CatanCardEvent{Kind: "earthquake", Production: 6, Players: []int{s.Turn}}
			},
			"draw-count":     func(s *State) { s.Catan.RollID++ },
			"duplicate-face": func(s *State) { s.Catan.EventDeck.Deck.DrawPile[0] = s.Catan.EventDeck.Deck.DrawPile[1] },
		} {
			t.Run(fmt.Sprintf("city=%v/%s", city, name), func(t *testing.T) {
				bad := clone(*s)
				corrupt(&bad)
				before, _ := json.Marshal(bad)
				if bad.Apply(bad.Turn, Action{Type: "catan_explorer_begin_move", Prompt: int(bad.Catan.TurnSerial)}) == nil {
					t.Fatal("accepted corrupt save")
				}
				after, _ := json.Marshal(bad)
				if string(before) != string(after) {
					t.Fatal("rejection mutated save")
				}
			})
		}
		for viewer := -1; viewer < 3; viewer++ {
			raw, _ := json.Marshal(s.View(viewer))
			var public map[string]any
			if err := json.Unmarshal(raw, &public); err != nil {
				t.Fatal(err)
			}
			g := public["catan"].(map[string]any)
			d := g["eventDeck"].(map[string]any)
			if d["explorer"] != CatanEventExplorerRules || d["drawPile"] != nil || d["deck"] != nil {
				t.Fatal("private deck exposed or public rule missing")
			}
			board := g["explorer"].(map[string]any)["board"].(map[string]any)
			if board["hidden"] != nil || board["numbers"] != nil {
				t.Fatal("hidden world exposed")
			}
		}
		if err := s.catanBeginCardEvent("earthquake", 6, 0, 0); err == nil {
			t.Fatal("Explorer executed card text")
		}
	}
}

func explorerEventsGame(t *testing.T, n int, scenario string, city bool) *State {
	t.Helper()
	var s *State
	var err error
	if city {
		s, err = NewCatanExplorerCitiesKnights(n, scenario)
	} else if scenario == "land-ho" {
		s, err = NewCatanExplorerLandHo(n)
	} else if scenario == "spices-for-catan" {
		s, err = NewCatanExplorerSpices(n)
	} else {
		s, err = NewCatanExplorerMission(n, scenario)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	for i := 0; s.Catan.Explorer.Setup != nil && i < 80; i++ {
		a, e := s.BotAction(s.Turn)
		if e != nil {
			t.Fatal(e)
		}
		helperApply(t, s, s.Turn, a)
	}
	if s.Catan.Explorer.Setup != nil {
		t.Fatal("opening did not complete")
	}
	return s
}

func explorerEventReady(t *testing.T, s *State) {
	t.Helper()
	for i := 0; s.Phase != "catan_turn" && !s.Finished && i < 80; i++ {
		p := ckActor(s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err, s.Phase)
		}
		helperApply(t, s, p, a)
		*s = *referenceEventRestore(t, s)
	}
	if !s.Finished && s.Phase != "catan_turn" {
		t.Fatal("production stuck", s.Phase)
	}
}

func explorerEventEnd(t *testing.T, s *State) {
	t.Helper()
	for _, typ := range []string{"catan_explorer_begin_move", "catan_end"} {
		helperApply(t, s, s.Turn, Action{Type: typ, Prompt: int(s.Catan.TurnSerial)})
	}
	*s = *referenceEventRestore(t, s)
}

func TestCatanExplorerEventsDeparturePreservesDeck(t *testing.T) {
	for _, city := range []bool{false, true} {
		for _, n := range []int{3, 6} {
			for _, after := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/city=%v/after=%v", n, city, after), func(t *testing.T) {
					s := explorerEventsGame(t, n, "explorers-and-pirates", city)
					if after {
						helperApply(t, s, s.Turn, Action{Type: "catan_roll", Prompt: int(s.Catan.TurnSerial)})
						explorerEventReady(t, s)
					}
					deck := clone(s.Catan.EventDeck)
					if err := s.EliminateCatan(s.Turn); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(deck, s.Catan.EventDeck) {
						t.Fatal("departure consumed a card")
					}
					for i := 0; i < 50 && s.Catan.RollID < 3; i++ {
						p := ckActor(s)
						a, err := s.BotAction(p)
						if err != nil {
							t.Fatal(err)
						}
						helperApply(t, s, p, a)
					}
					referenceEventRestore(t, s)
					if s.Catan.RollID < 3 {
						t.Fatal("survivors stalled")
					}
				})
			}
		}
	}
}

func TestCatanExplorerEventsOnlyProductionAllScenarios(t *testing.T) {
	for _, scenario := range []string{"land-ho", "pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
		for n := 2; n <= 6; n++ {
			for _, city := range []bool{false, true} {
				if scenario == "land-ho" && (n > 4 || city) || city && n == 2 {
					continue
				}
				seed := explorerEventsGame(t, n, scenario, city)
				for kind := range catanCardEventNames {
					t.Run(fmt.Sprintf("%s/%d/city=%v/%s", scenario, n, city, kind), func(t *testing.T) {
						s := clone(*seed)
						id := referenceEventTop(t, &s, kind)
						number := catanEventReferenceFaces[id].Production
						baseline := clone(s)
						baseline.Catan.EventDeck = nil
						red := max(1, number-6)
						var err error
						if city {
							err = baseline.catanExplorerCityRoll(red, number-red, 0)
						} else {
							err = baseline.catanExplorerRoll([2]int{red, number - red})
						}
						if err != nil {
							t.Fatal(err)
						}
						calls := 0
						err = s.catanDrawEventRandom(func(int) int {
							calls++
							if calls == 1 {
								return red - 1
							}
							return 0
						})
						if err != nil {
							t.Fatal(err)
						}
						g := s.Catan
						if city && calls != 2 || !city && calls != 0 {
							t.Fatal("incorrect independent dice count", calls)
						}
						if !reflect.DeepEqual(g.Players, baseline.Catan.Players) || !reflect.DeepEqual(g.Bank, baseline.Catan.Bank) || !reflect.DeepEqual(g.Explorer.Economy.Gold, baseline.Catan.Explorer.Economy.Gold) || !reflect.DeepEqual(g.Edges, baseline.Catan.Edges) || !reflect.DeepEqual(g.Vertices, baseline.Catan.Vertices) || s.Phase != baseline.Phase {
							t.Fatal("ignored card effect changed ordinary production")
						}
						if g.CardEvent != nil || g.RollID != 1 || g.Dice[1] != 0 || g.Explorer.Economy.Turn.Production != number {
							t.Fatal("fabricated dice or card response")
						}
						if err = s.validateCatanExplorer(); err != nil {
							t.Fatal(err)
						}
						s = *referenceEventRestore(t, &s)
						explorerEventReady(t, &s)
						if !s.Catan.RevealedEvent.ProductionStarted {
							t.Fatal("missing production marker")
						}
						if n == 2 && s.Catan.Two != nil {
							t.Fatal("Explorer must not use base two-player double draws")
						}
						deck := clone(s.Catan.EventDeck)
						explorerEventEnd(t, &s)
						if !reflect.DeepEqual(deck, s.Catan.EventDeck) {
							t.Fatal("movement/end consumed a card")
						}
						if n > 4 && (s.Phase != "catan_turn" || !s.Catan.Paired.Second) {
							t.Fatal("second player did not skip production")
						}
					})
				}
			}
		}
	}
}

func TestCatanExplorerEventsNewYearAlchemyAndPrivateDeck(t *testing.T) {
	for _, city := range []bool{false, true} {
		for _, n := range []int{3, 6} {
			t.Run(fmt.Sprintf("%d/city=%v", n, city), func(t *testing.T) {
				s := explorerEventsGame(t, n, "explorers-and-pirates", city)
				for roll := 1; roll <= 36; roll++ {
					alchemy := city && (roll == 1 || roll == 33)
					if alchemy {
						ckProgressGive(t, s, s.Turn, 0)
						deck := clone(s.Catan.EventDeck.Deck)
						helperApply(t, s, s.Turn, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: int(s.Catan.TurnSerial)})
						if !reflect.DeepEqual(deck, s.Catan.EventDeck.Deck) || s.Catan.RevealedEvent != nil {
							t.Fatal("alchemy changed card deck")
						}
					} else {
						helperApply(t, s, s.Turn, Action{Type: "catan_roll", Prompt: int(s.Catan.TurnSerial)})
					}
					explorerEventReady(t, s)
					for viewer := -1; viewer < n; viewer++ {
						v := s.View(viewer)["catan"].(map[string]any)
						if v["eventDeck"] == nil {
							t.Fatal("public event deck missing")
						}
					}
					if s.Catan.RollID != roll {
						t.Fatal("double production")
					}
					explorerEventEnd(t, s)
					if n > 4 {
						explorerEventEnd(t, s)
					}
				}
				if s.Catan.EventDeck.Deck.Cycle != 2 {
					t.Fatal("New Year did not reshuffle")
				}
			})
		}
	}
}
