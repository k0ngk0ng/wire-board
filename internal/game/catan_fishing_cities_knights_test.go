package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishCityGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanFishingCitiesKnights(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func fishCityActionGame(t *testing.T, n int, phase string) *State {
	t.Helper()
	s := fishCityGame(t, n)
	g := s.Catan
	s.Turn, s.Phase = 0, phase
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	if g.Paired != nil {
		g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 3, 0, true
	}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	fishOwn(&g.Fishing.Tokens, 0, 0, 1, 11, 12, 21, 22, 23)
	s.catanScores()
	return s
}
func fishCityProgressTop(t *testing.T, s *State, card int) {
	t.Helper()
	k := s.Catan.CitiesKnights
	track := catanProgressRules[card].Track
	at := slices.Index(k.ProgressDecks[track], card)
	if at < 0 {
		t.Fatal("missing progress fixture card")
	}
	deck := k.ProgressDecks[track]
	deck[at], deck[len(deck)-1] = deck[len(deck)-1], deck[at]
}
func TestCatanFishingCitiesKnightsSetupAndGoal(t *testing.T) {
	for n := 3; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishCityGame(t, n)
			g := s.Catan
			if g.victoryTargetFor(0) != 13 || len(g.Bank) != 8 || len(g.DevDeck) != 0 || g.Robber != -1 || g.CitiesKnights.RobberStart != -1 {
				t.Fatal("combination setup")
			}
			if err := g.validateFishing(); err != nil {
				t.Fatal(err)
			}
			// The second starting building is a city, but setup awards one fish token.
			p := s.Turn
			g.SetupStep = n
			s.Phase = "catan_setup_city"
			v := g.Tiles[g.Fishing.Map.Lakes[0].Tile].Vertices[0]
			fishTop(&g.Fishing.Tokens, 0)
			if err := s.Apply(p, Action{Type: "catan_city", Vertex: v}); err != nil {
				t.Fatal(err)
			}
			g = s.Catan
			if g.Vertices[v].Level != 2 || len(g.Fishing.Tokens.Hands[p]) != 1 || s.Phase != "catan_setup_road" || sum(g.Players[p].Resources[5:]) != 0 {
				t.Fatal("starting city fish/resource award")
			}
			fishTop(&g.Fishing.Tokens, 29)
			due := make([]int, n)
			due[p] = 1
			if err := s.catanStartFishing(due, nil, "catan_setup_road"); err != nil {
				t.Fatal(err)
			}
			if g.victoryTargetFor(p) != 14 || g.Players[p].Score != 2 {
				t.Fatal("boot target changed score")
			}
		})
	}
	if s, err := NewCatanFishingCitiesKnights(3, CatanOptions{Helpers: true}); err != nil || !s.Catan.cityHelpers() || !s.Catan.fishingHelpers() {
		t.Fatal("missing helper combination", err)
	}
}
func TestCatanFishingCitiesKnightsReplacementThenAqueduct(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishCityGame(t, n)
			g := s.Catan
			s.Turn, s.Phase = 0, "catan_roll"
			g.SetupStep, g.RollID = g.SetupLimit(), 1
			for i := range g.Tiles {
				if g.Tiles[i].Resource < 5 {
					g.Tiles[i].Number = 6
				}
			}
			lake := g.Fishing.Map.Lakes[0].Tile
			g.Vertices[g.Tiles[lake].Vertices[0]].Owner, g.Vertices[g.Tiles[lake].Vertices[0]].Level = 1, 2
			g.CitiesKnights.Players[1].Improvements[CatanScience] = 3
			fishOwn(&g.Fishing.Tokens, 1, 0, 1, 2, 3, 4, 5, 6)
			fishTop(&g.Fishing.Tokens, 21, 22)
			if err := s.catanRollProduction(2); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != 1 || g.CitiesKnights.Pending != nil {
				t.Fatal("aqueduct ran before fish replacement")
			}
			saved := clone(*s)
			s = &saved
			if err := s.Apply(1, Action{Type: "catan_fish_replace", Card: 0}); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != 1 || s.Catan.Fishing.Pending != nil || len(s.Catan.Fishing.Tokens.Hands[1]) != 7 {
				t.Fatal("fish-only receiver lost aqueduct")
			}
			fishGameReject(t, s, 0, Action{Type: "catan_aqueduct", Color: 0})
			fishGameReject(t, s, 1, Action{Type: "catan_fish_resource", Tokens: []int{21, 1}, Color: 0})
			saved = clone(*s)
			s = &saved
			if err := s.Apply(1, Action{Type: "catan_aqueduct", Color: 0}); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_turn" || s.Turn != 0 || s.Catan.Players[1].Resources[0] != 1 {
				t.Fatal("aqueduct continuation")
			}
			fishGameReject(t, s, 0, Action{Type: "catan_fish_keep"})
		})
	}
}
func TestCatanFishingCitiesKnightsPaymentsAndPrivacy(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		t.Run(phase, func(t *testing.T) {
			s := fishCityActionGame(t, 3, phase)
			g := s.Catan
			g.Bank[CatanCoin]--
			g.Players[1].Resources[CatanCoin]++
			fishGameReject(t, s, 0, Action{Type: "catan_fish_resource", Color: CatanCoin, Tokens: []int{0, 21}})
			fishGameReject(t, s, 0, Action{Type: "catan_fish_dev", Tokens: []int{0, 21, 22}})
			fishGameReject(t, s, 0, Action{Type: "catan_fish_progress", Color: 3, Tokens: []int{0, 21, 22}})
			if err := s.Apply(0, Action{Type: "catan_fish_steal", Target: 1, Tokens: []int{23}}); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Players[0].Resources[CatanCoin] != 1 || s.Catan.CitiesKnights.Invasions != 0 {
				t.Fatal("fish theft before first attack must allow commodity")
			}
			fishCityProgressTop(t, s, 0)
			before := append([]int{}, s.Catan.CitiesKnights.ProgressDecks[0]...)
			if err := s.Apply(0, Action{Type: "catan_fish_progress", Color: 0, Tokens: []int{0, 21, 22}}); err != nil {
				t.Fatal(err)
			}
			g = s.Catan
			if !slices.Equal(g.CitiesKnights.Players[0].Progress, []int{0}) || !slices.Equal(g.CitiesKnights.ProgressDecks[0], before[:len(before)-1]) || s.Phase != phase {
				t.Fatal("progress purchase or phase")
			}
			ckProgressStock(t, g)
			for _, viewer := range []int{-1, 0, 1, 2} {
				v := s.View(viewer)["catan"].(map[string]any)
				b, _ := json.Marshal(v)
				var public map[string]any
				json.Unmarshal(b, &public)
				k := public["citiesKnights"].(map[string]any)
				if _, ok := k["progressDecks"]; ok {
					t.Fatal("progress deck order leaked")
				}
				for p, raw := range k["players"].([]any) {
					_, visible := raw.(map[string]any)["progress"]
					if visible != (p == viewer) {
						t.Fatal("private progress face leaked")
					}
				}
				f := public["fishing"].(map[string]any)["tokens"].(map[string]any)
				for p, raw := range f["players"].([]any) {
					_, visible := raw.(map[string]any)["tokens"]
					if visible != (p == viewer && len(g.Fishing.Tokens.Hands[p]) > 0) {
						t.Fatal("private fish face leaked")
					}
				}
			}
			if phase == "catan_roll" && !slices.Contains(s.View(0)["catan"].(map[string]any)["progressPlayable"].([]int), 0) {
				t.Fatal("new progress should be immediately playable")
			}
		})
	}
}
func TestCatanFishingCitiesKnightsProgressStockCapAndVictory(t *testing.T) {
	s := fishCityActionGame(t, 3, "catan_turn")
	g := s.Catan
	k := g.CitiesKnights
	// Move the selected stack to another player's private hand to make it empty.
	k.Players[1].Progress = append(k.Players[1].Progress, k.ProgressDecks[0]...)
	k.ProgressDecks[0] = nil
	fishGameReject(t, s, 0, Action{Type: "catan_fish_progress", Color: 0, Tokens: []int{0, 21, 22}})
	legal := s.catanFishLegal(0)
	if slices.Contains(legal["progressTracks"].([]int), 0) || slices.Contains(legal["actions"].([]string), "catan_fish_dev") {
		t.Fatal("empty stack/base development offered")
	}
	s = fishCityActionGame(t, 6, "catan_turn")
	g = s.Catan
	k = g.CitiesKnights
	for range 4 {
		s.catanDrawProgress(0, 1)
	}
	if err := s.Apply(0, Action{Type: "catan_fish_progress", Color: 1, Tokens: []int{0, 21, 22}}); err != nil {
		t.Fatal(err)
	}
	if len(s.Catan.CitiesKnights.Players[0].Progress) != 5 || s.Phase != "catan_turn" {
		t.Fatal("active player should keep excess progress until turn end")
	}
	if err := s.Apply(0, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_progress_end" {
		t.Fatal("missing end-of-turn progress discard")
	}
	for _, boot := range []bool{false, true} {
		s = fishCityActionGame(t, 3, "catan_roll")
		g = s.Catan
		k = g.CitiesKnights
		k.Players[0].DefenderPoints = 11 // one settlement + eleven defender points = twelve
		if boot {
			fishOwn(&g.Fishing.Tokens, 0, 29)
			g.Fishing.Tokens.Hands[0] = g.Fishing.Tokens.Hands[0][:7]
			g.Fishing.Tokens.BootOwner = 0
		}
		fishCityProgressTop(t, s, 9)
		s.catanScores()
		if err := s.Apply(0, Action{Type: "catan_fish_progress", Color: 0, Tokens: []int{0, 21, 22}}); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Players[0].Score != 13 || s.Finished == boot {
			t.Fatal("progress point must respect individual boot target")
		}
		if !slices.Equal(s.Catan.CitiesKnights.Players[0].PublicProgress, []int{9}) {
			t.Fatal("victory progress not public")
		}
	}
}
func TestCatanFishingCitiesKnightsBotsFinishAndConserve(t *testing.T) {
	for n := 3; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishCityGame(t, n)
			paid, draws := 0, 0
			for step := 0; step < 8000 && !s.Finished; step++ {
				actor := s.Turn
				if pending := s.CatanPendingActor(); pending >= 0 {
					actor = pending
				} else if s.Phase == "catan_discard" {
					for p, due := range s.Catan.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if _, ok := catanFishCosts[a.Type]; ok {
					paid++
				}
				if a.Type == "catan_fish_progress" {
					draws++
				}
				if err := s.Apply(actor, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				g := s.Catan
				if err := g.validateFishing(); err != nil {
					t.Fatal(err)
				}
				ckProgressStock(t, g)
				ckKnightStock(t, g)
				for color, bank := range g.Bank {
					amount := bank
					for _, p := range g.Players {
						amount += p.Resources[color]
					}
					want := 19
					if n > 4 {
						want = 24
					}
					if color >= 5 {
						want = 12
						if n > 4 {
							want = 18
						}
					}
					if amount != want {
						t.Fatal("card supply", color, amount, want)
					}
				}
				for p := range g.Players {
					roads, _, _ := g.pieces(p)
					if roads > 15 || g.settlementPiecesLeft(p) < 0 || g.cityPiecesLeft(p) < 0 {
						t.Fatal("physical pieces")
					}
				}
				if step%41 == 0 {
					saved := clone(*s)
					if !reflect.DeepEqual(*s, saved) {
						t.Fatal("restore differs")
					}
					s = &saved
				}
			}
			if !s.Finished || paid == 0 || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
				t.Fatal("incomplete combined game", s.Round, paid, draws)
			}
			t.Logf("round=%d paid=%d fish-progress=%d", s.Round, paid, draws)
		})
	}
}

func TestCatanFishingCitiesKnightsBotSelectsProgressWithoutPeeking(t *testing.T) {
	s := fishCityActionGame(t, 3, "catan_turn")
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_fish_progress" {
		t.Fatal("bot did not consider fish progress", a, err)
	}
	other := clone(*s)
	for i := range other.Catan.CitiesKnights.ProgressDecks {
		slices.Reverse(other.Catan.CitiesKnights.ProgressDecks[i])
	}
	slices.Reverse(other.Catan.Fishing.Tokens.DrawPile)
	b, err := other.BotAction(0)
	if err != nil || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(s.View(0), other.View(0)) {
		t.Fatal("bot or view uses hidden deck order", a, b, err)
	}
	if err := s.Apply(0, a); err != nil {
		t.Fatal(err)
	}
}

func TestCatanFishingCitiesKnightsProgressCannotTakeFish(t *testing.T) {
	s := fishCityActionGame(t, 3, "catan_turn")
	g := s.Catan
	fishOwn(&g.Fishing.Tokens, 1, 2, 13)
	g.Players[1].Resources[0] = 3
	g.Bank[0] -= 3
	ckProgressGive(t, s, 0, 14)
	before := g.Fishing.Tokens.copy()
	if err := s.Apply(0, Action{Type: "catan_progress", Card: 14, Color: 0}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, s.Catan.Fishing.Tokens) || s.Catan.Players[0].Resources[0] != 2 || s.Catan.Players[1].Resources[0] != 1 {
		t.Fatal("resource monopoly affected fish")
	}
	ckProgressGive(t, s, 0, 14)
	fishGameReject(t, s, 0, Action{Type: "catan_progress", Card: 14, Color: catanLake})
	ckProgressStock(t, s.Catan)
}
