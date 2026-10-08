package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerFishingGame(t *testing.T, n int, scenario string, cities, lakes, events bool) *State {
	t.Helper()
	s, err := NewCatanExplorerFishing(n, scenario, cities, lakes)
	if err != nil {
		t.Fatal(err)
	}
	if events {
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; s.Catan.Explorer.Setup != nil && i < 80; i++ {
		if s.CatanExplorerSetupBlocked() {
			if err = s.ResetCatanExplorerSetup(); err != nil {
				t.Fatal(err)
			}
		}
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(s.Turn, a); err != nil {
			t.Fatal("setup", a, err)
		}
	}
	if s.Catan.Explorer.Setup != nil {
		t.Fatal("opening did not complete")
	}
	return s
}

func explorerFishingRestore(t *testing.T, s *State) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(data, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanExplorer(); err != nil {
		t.Fatal("restored fishing explorer", err)
	}
	if err = next.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*s, next) {
		t.Fatal("saved fishing state changed")
	}
	*s = next
}

func TestCatanExplorerFishingStartsAndNaturalTurns(t *testing.T) {
	for _, cfg := range explorerFishingMapConfigs() {
		if cfg.layout == "fixed" && cfg.scenario != "land-ho" {
			continue
		}
		t.Run(fmt.Sprintf("%d/%s/cities%t/lakes%t", cfg.n, cfg.scenario, cfg.cities, cfg.lakes), func(t *testing.T) {
			s := explorerFishingGame(t, cfg.n, cfg.scenario, cfg.cities, cfg.lakes, true)
			g := s.Catan
			if g.Fishing.LastRollID != 0 || slices.Contains(g.Fishing.Started, false) || g.RollID != 0 || g.Two != nil || s.Phase != "catan_roll" {
				t.Fatal("initial fish state")
			}
			if cfg.n == 2 {
				for _, hand := range g.Fishing.Tokens.Hands {
					values := []int{}
					for _, id := range hand {
						values = append(values, catanFishValue(id))
					}
					slices.Sort(values)
					if !slices.Equal(values, []int{1, 1, 2, 2, 3}) {
						t.Fatal("native two-player fish setup", values)
					}
				}
				if g.Fishing.Tokens.BootOwner != -1 {
					t.Fatal("fixed initial tokens included boot")
				}
			}
			explorerFishingRestore(t, s)
			for i := 0; i < 160 && s.Catan.TurnSerial < 5 && !s.Finished; i++ {
				actor := s.CatanPendingActor()
				if actor < 0 {
					actor = s.Turn
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal("bot", s.Phase, err)
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatal("action", s.Phase, a, err)
				}
				explorerFishingRestore(t, s)
			}
			if s.Catan.TurnSerial < 5 {
				t.Fatal("could not complete four turns")
			}
		})
	}
}

func TestCatanExplorerFishingBootTargetAndViews(t *testing.T) {
	s := explorerFishingGame(t, 2, "land-ho", false, true, false)
	f := &s.Catan.Fishing.Tokens
	fishTop(f, catanFishBoot)
	if err := f.beginDraw(s.Turn, []int{1, 1}); err != nil {
		t.Fatal(err)
	}
	for player := range s.Catan.Players {
		want := s.Catan.Explorer.Board.Target
		if f.BootOwner == player {
			want++
		}
		if s.Catan.victoryTargetFor(player) != want || catanExplorerFishVictoryTarget(s.Catan, s.Catan.Explorer.Board, player) != want {
			t.Fatal("boot target")
		}
	}
	for viewer := -1; viewer < 2; viewer++ {
		data, _ := json.Marshal(s.View(viewer))
		var v map[string]any
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatal(err)
		}
		public := v["catan"].(map[string]any)["fishing"].(map[string]any)
		if public["pending"] != nil || public["started"] != nil {
			t.Fatal("private fishing continuation exposed")
		}
		tokens := public["tokens"].(map[string]any)
		if tokens["drawPile"] != nil || tokens["hands"] != nil {
			t.Fatal("private fish order/hands exposed")
		}
		for p, raw := range tokens["players"].([]any) {
			faces := raw.(map[string]any)["tokens"]
			if p != viewer && faces != nil {
				t.Fatal("opponent fish faces exposed", viewer, p)
			}
		}
		copy := clone(*s)
		slices.Reverse(copy.Catan.Fishing.Tokens.DrawPile)
		other, _ := json.Marshal(copy.View(viewer))
		if string(data) != string(other) {
			t.Fatal("view depends on private fish draw order")
		}
	}
	explorerFishingRestore(t, s)
}

// Controlled, legal geometry isolates fish-only production. Initial ships
// and their cargo stay intact; ordinary buildings are replaced by two lake
// buildings so the expected production does not depend on a random opening.
func explorerFishingLakeFixture(t *testing.T, cities, events bool) *State {
	t.Helper()
	s := explorerFishingGame(t, 3, "explorers-and-pirates", cities, true, events)
	g := s.Catan
	for i := range g.Vertices {
		g.Vertices[i].Owner, g.Vertices[i].Level, g.Vertices[i].Harbor = -1, 0, false
	}
	for i := range g.Edges {
		g.Edges[i].Owner = -1
	}
	lake := g.Tiles[g.Fishing.Map.Lakes[0].Tile]
	for p, corner := range []int{0, 3} {
		v := &g.Vertices[lake.Vertices[corner]]
		v.Owner, v.Level = p, 1
		if cities {
			v.Level = 2
			g.CitiesKnights.Players[p].Improvements[CatanScience] = 3
		}
	}
	s.catanScores()
	tokens, _ := newCatanFishingTokens(3)
	g.Fishing.Tokens = *tokens
	fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
	fishOwn(&g.Fishing.Tokens, 1, 7, 8, 9, 10, 11, 12, 13)
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal("lake fixture", err)
	}
	return s
}

func TestCatanExplorerFishingProductionQueueAqueductAndReplay(t *testing.T) {
	for _, cities := range []bool{false, true} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("cities%t/events%t", cities, events), func(t *testing.T) {
				s := explorerFishingLakeFixture(t, cities, events)
				old := clone(*s)
				var err error
				if events {
					// Pin a real 2-production card; never fabricate a deck face.
					deck := &s.Catan.EventDeck.Deck
					found := false
					for i, id := range deck.DrawPile {
						if id == catanEventNewYear {
							continue
						}
						card := catanEventReferenceFaces[id]
						if card.Production == 2 {
							deck.DrawPile[i], deck.DrawPile[len(deck.DrawPile)-1] = deck.DrawPile[len(deck.DrawPile)-1], id
							found = true
							break
						}
					}
					if !found {
						t.Fatal("catalogue has no 2 card")
					}
					err = s.catanDrawEventRandom(func(int) int { return 0 })
				} else if cities {
					err = s.catanExplorerCityRoll(1, 1, 3)
				} else {
					err = s.catanExplorerRoll([2]int{1, 1})
				}
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if s.Phase != "catan_fish_replace" || len(g.Fishing.Tokens.Pending) != 2 || g.Fishing.LastRollID != 1 || g.Explorer.Economy.Turn.productionNumber() != 2 {
					t.Fatal("fish queue missing", s.Phase, g.Fishing)
				}
				if cities && g.CitiesKnights.Pending != nil {
					t.Fatal("Aqueduct started before fish responses")
				}
				for p := range g.Players {
					if g.Explorer.Economy.Gold[p] != old.Catan.Explorer.Economy.Gold[p]+1 || !slices.Equal(g.Players[p].Resources, old.Catan.Players[p].Resources) {
						t.Fatal("fish-only production must give gold without resource cards")
					}
				}
				for i := 0; i < 2; i++ {
					explorerFishingRestore(t, s)
					actor := s.CatanPendingActor()
					if actor < 0 || actor > 1 {
						t.Fatal("wrong fish responder", actor)
					}
					a := Action{Type: "catan_fish_keep", Prompt: int(s.Catan.TurnSerial)}
					before, _ := json.Marshal(s)
					if err = s.Apply((actor+1)%3, a); err == nil {
						t.Fatal("foreign fish response accepted")
					}
					stale := a
					stale.Prompt--
					if err = s.Apply(actor, stale); err == nil {
						t.Fatal("stale fish response accepted")
					}
					after, _ := json.Marshal(s)
					if string(before) != string(after) {
						t.Fatal("rejection changed fish or production")
					}
					if i == 0 {
						if err = s.Apply(actor, a); err != nil {
							t.Fatal(err)
						}
					} else {
						s.AutoCatanPending()
					}
				}
				if s.Catan.Fishing.Pending != nil {
					t.Fatal("automatic fish response stalled")
				}
				if cities {
					if s.Phase != "catan_aqueduct" || len(s.Catan.CitiesKnights.Pending.Players) != 2 {
						t.Fatal("fish-only players lost Aqueduct", s.Phase)
					}
					explorerEventReady(t, s)
				}
				if s.Phase != "catan_turn" {
					t.Fatal("fish continuation did not reach action", s.Phase)
				}
				explorerFishingRestore(t, s)
				if err = s.catanExplorerFishingProduction(make([][]int, 3)); err == nil {
					t.Fatal("production replay accepted")
				}
				for p := range s.Catan.Players {
					if s.Catan.Explorer.Economy.Gold[p] != old.Catan.Explorer.Economy.Gold[p]+1 {
						t.Fatal("fish responses duplicated gold")
					}
				}
			})
		}
	}
}
