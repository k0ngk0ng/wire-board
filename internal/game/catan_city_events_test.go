package game

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func ckProgressStock(t *testing.T, g *Catan) {
	t.Helper()
	counts := make([]int, len(catanProgressRules))
	for track, deck := range g.CitiesKnights.ProgressDecks {
		for _, c := range deck {
			if c < 0 || c >= len(counts) || catanProgressRules[c].Track != track {
				t.Fatal("invalid progress deck")
			}
			counts[c]++
		}
	}
	for _, p := range g.CitiesKnights.Players {
		for _, c := range p.Progress {
			if catanProgressRules[c].Victory {
				t.Fatal("VP in private hand")
			}
			counts[c]++
		}
		for _, c := range p.PublicProgress {
			if !catanProgressRules[c].Victory {
				t.Fatal("non-VP in public stack")
			}
			counts[c]++
		}
	}
	if tribe := g.tribe(); tribe != nil && tribe.ProgressRules == CatanTribeProgressRules {
		for _, reward := range tribe.Development {
			if reward.Card < 0 || reward.Card >= len(counts) {
				t.Fatal("invalid tribe progress")
			}
			counts[reward.Card]++
		}
	}
	for c, n := range counts {
		if n != catanProgressRules[c].Count {
			t.Fatal("progress conservation", c, n)
		}
	}
	ckSupply(t, g)
}
func ckProgressGive(t *testing.T, s *State, player int, cards ...int) {
	t.Helper()
	k := s.Catan.CitiesKnights
	for _, c := range cards {
		track := catanProgressRules[c].Track
		at := slices.Index(k.ProgressDecks[track], c)
		if at < 0 {
			t.Fatal("fixture card unavailable", c)
		}
		k.ProgressDecks[track] = slices.Delete(k.ProgressDecks[track], at, at+1)
		k.Players[player].Progress = append(k.Players[player].Progress, c)
	}
}
func ckProgressTop(t *testing.T, s *State, cards ...int) {
	t.Helper()
	for i := len(cards) - 1; i >= 0; i-- {
		c := cards[i]
		track := catanProgressRules[c].Track
		deck := s.Catan.CitiesKnights.ProgressDecks[track]
		at := slices.Index(deck, c)
		if at < 0 {
			t.Fatal("fixture top card unavailable")
		}
		deck = slices.Delete(deck, at, at+1)
		s.Catan.CitiesKnights.ProgressDecks[track] = append(deck, c)
	}
}
func ckEvent(t *testing.T) *State {
	s := ckEmptyTurn(t)
	s.Phase = "catan_roll"
	for i := range s.Catan.Tiles {
		s.Catan.Tiles[i].Resource = CatanDesert
		s.Catan.Tiles[i].Number = 0
	}
	s.Catan.CitiesKnights.RobberStart = 1
	return s
}
func TestCatanCityEventsProgressInventoryEligibilityAndOrder(t *testing.T) {
	for n := 3; n <= 6; n++ {
		s := ckGame(t, n)
		for _, deck := range s.Catan.CitiesKnights.ProgressDecks {
			if len(deck) != 18 {
				t.Fatal("each progress category requires 18 cards")
			}
		}
		ckProgressStock(t, s.Catan)
	}
	s := ckEvent(t)
	s.Turn = 2
	for i := range 3 {
		s.Catan.CitiesKnights.Players[i].Improvements[0] = 1
	}
	ckProgressTop(t, s, 9, 0, 1)
	if err := s.catanCityRoll(2, 4, 0); err != nil {
		t.Fatal(err)
	}
	k := s.Catan.CitiesKnights
	if !reflect.DeepEqual(k.Players[2].PublicProgress, []int{9}) || !reflect.DeepEqual(k.Players[0].Progress, []int{0}) || !reflect.DeepEqual(k.Players[1].Progress, []int{1}) || k.Players[2].ProgressPoints != 1 || s.Catan.Players[2].Score != 1 {
		t.Fatal("clockwise draws or immediate public VP")
	}
	if strings.Contains(strings.Join(s.Log, " "), "Alchemy") || strings.Contains(strings.Join(s.Log, " "), "Crane") {
		t.Fatal("draw log leaked hidden progress kind")
	}
	if s.Phase != "catan_turn" || k.Event != nil || s.Catan.RollID != 1 {
		t.Fatal("event production continuation")
	}
	ckProgressStock(t, s.Catan)
	s = ckEvent(t)
	s.Catan.CitiesKnights.Players[0].Improvements[0] = 1
	s.Catan.CitiesKnights.Players[1].Improvements[0] = 2
	s.Catan.CitiesKnights.Players[2].Improvements[0] = 0
	ckProgressTop(t, s, 0)
	if err := s.catanCityRoll(3, 3, 0); err != nil {
		t.Fatal(err)
	}
	k = s.Catan.CitiesKnights
	if len(k.Players[0].Progress) != 0 || len(k.Players[1].Progress) != 1 || len(k.Players[2].Progress) != 0 {
		t.Fatal("red die threshold or zero-level eligibility")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanCityEventsProgressDiscardBeforeProductionAndPrivacy(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	k := g.CitiesKnights
	g.Tiles[0].Resource, g.Tiles[0].Number, g.Tiles[0].Vertices = 0, 6, []int{0}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	k.Players[1].Improvements[0], k.Players[2].Improvements[0] = 1, 1
	ckProgressGive(t, s, 1, 3, 4, 5, 6)
	ckProgressTop(t, s, 0, 1)
	if err := s.catanCityRoll(1, 5, 0); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_progress_discard" || s.CatanPendingActor() != 1 || sum(g.Players[0].Resources) != 0 || len(k.Players[2].Progress) != 0 {
		t.Fatal("event response must precede production and subsequent draw")
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		view := s.View(viewer)["catan"].(map[string]any)["citiesKnights"].(map[string]any)
		if _, ok := view["event"]; ok {
			t.Fatal("internal production queue exposed to viewer", viewer)
		}
		if view["pending"] == nil || view["eventDie"] != float64(0) {
			t.Fatal("public response or rolled event die missing")
		}
		if _, ok := view["progressDecks"]; ok {
			t.Fatal("progress deck leaked")
		}
		for p, raw := range view["players"].([]any) {
			_, visible := raw.(map[string]any)["progress"]
			if visible != (p == viewer) {
				t.Fatal("hidden progress hand leaked")
			}
		}
	}
	helperReject(t, s, 0, Action{Type: "catan_progress_discard", Cards: []int{3}})
	helperReject(t, s, 1, Action{Type: "catan_progress_discard", Cards: []int{3, 3}})
	helperReject(t, s, 1, Action{Type: "catan_progress_discard", Cards: []int{9}})
	restored := clone(*s)
	s = &restored
	s.AutoCatanPending()
	k = s.Catan.CitiesKnights
	if s.Phase != "catan_turn" || k.Event != nil || len(k.Players[1].Progress) != 4 || !reflect.DeepEqual(k.Players[2].Progress, []int{1}) || k.ProgressDecks[0][0] != 3 || s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[0].Resources[5] != 1 {
		t.Fatal("discard/bottom return/remaining draw/production chain")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanCityEventsActiveHandLimitEndsTurnAfterChoice(t *testing.T) {
	s := ckEvent(t)
	s.Catan.CitiesKnights.Players[0].Improvements[0] = 1
	ckProgressGive(t, s, 0, 3, 4, 5, 6)
	ckProgressTop(t, s, 0)
	if err := s.catanCityRoll(1, 1, 0); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || len(s.Catan.CitiesKnights.Players[0].Progress) != 5 {
		t.Fatal("active player cannot defer discard")
	}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	if s.Phase != "catan_progress_end" || s.Turn != 0 {
		t.Fatal("end exceeded hand limit")
	}
	s.AutoCatanPending()
	if s.Turn != 1 || s.Phase != "catan_roll" || len(s.Catan.CitiesKnights.Players[0].Progress) != 4 {
		t.Fatal("discard did not end turn")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanCityEventsBarbarianDistanceAndPillageBeforeProduction(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	k := g.CitiesKnights
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.Tiles[0].Resource, g.Tiles[0].Number, g.Tiles[0].Vertices = 0, 6, []int{0}
	k.Walls = []int{0}
	for step := 1; step < 7; step++ {
		s.Phase = "catan_roll"
		if err := s.catanCityRoll(1, 1, 3+(step%3)); err != nil {
			t.Fatal(err)
		}
		if k.BarbarianPosition != step || k.Invasions != 0 || g.Robber != -1 || g.Vertices[0].Level != 2 {
			t.Fatal("premature barbarian attack", step)
		}
	}
	s.Phase = "catan_roll"
	if err := s.catanCityRoll(1, 5, 3); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_pillage" || s.CatanPendingActor() != 0 || sum(g.Players[0].Resources) != 0 {
		t.Fatal("pillage timing")
	}
	helperReject(t, s, 0, Action{Type: "catan_pillage", Vertex: 1})
	helperApply(t, s, 0, Action{Type: "catan_pillage", Vertex: 0})
	g = s.Catan
	k = g.CitiesKnights
	if g.Vertices[0].Level != 1 || g.Players[0].Resources[0] != 1 || g.Players[0].Resources[5] != 0 || k.Invasions != 1 || k.BarbarianPosition != 0 || g.Robber != 1 || len(k.Walls) != 0 {
		t.Fatal("pillage/city production/first robber order")
	}
	ckProgressStock(t, g)
}
func TestCatanCityEventsPillageWallBeforeSevenAndImmuneMetropolis(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	k := g.CitiesKnights
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	k.Walls = []int{0}
	k.BarbarianPosition = 6
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 8, 0, 0})
	if err := s.catanCityRoll(1, 6, 3); err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, 0, Action{Type: "catan_pillage", Vertex: 0})
	if s.Phase != "catan_discard" || s.Catan.DiscardDue[0] != 4 {
		t.Fatal("destroyed wall still protected hand")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_robber" {
		t.Fatal("first invasion did not activate robber for same roll's seven")
	}
	ckProgressStock(t, s.Catan)
	s = ckEvent(t)
	g = s.Catan
	k = g.CitiesKnights
	for p := range 3 {
		g.Vertices[p].Owner, g.Vertices[p].Level = p, 2
	}
	k.Metropolises[0] = 0
	k.Knights = []CatanKnight{{Owner: 1, Vertex: 10, Strength: 1, Active: true}, {Owner: 2, Vertex: 11, Strength: 1, Active: true}}
	k.BarbarianPosition = 6
	if err := s.catanCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	if s.CatanPendingActor() != 1 {
		t.Fatal("metropolis owner incorrectly pillaged")
	}
	helperApply(t, s, 1, Action{Type: "catan_pillage", Vertex: 1})
	if s.CatanPendingActor() != 2 {
		t.Fatal("tied weakest defender not pillaged")
	}
	helperApply(t, s, 2, Action{Type: "catan_pillage", Vertex: 2})
	if s.Catan.Vertices[0].Level != 2 || s.Catan.Vertices[1].Level != 1 || s.Catan.Vertices[2].Level != 1 {
		t.Fatal("wrong pillage victims")
	}
	for _, n := range s.Catan.CitiesKnights.Knights {
		if n.Active {
			t.Fatal("knight active after attack")
		}
	}
}
func TestCatanCityEventsDefenderWinAndTiedRewardDiscardChain(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	k := g.CitiesKnights
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	k.BarbarianPosition = 6
	k.Players[0].DefenderPoints = 10
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 10, Strength: 1, Active: true}, {Owner: 1, Vertex: 11, Strength: 3}}
	if err := s.catanCityRoll(1, 5, 3); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || k.Players[0].DefenderPoints != 11 || !reflect.DeepEqual(s.Winners, []int{0}) {
		t.Fatal("defender VP victory or inactive strength counted")
	}
	s = ckEvent(t)
	g = s.Catan
	k = g.CitiesKnights
	s.Turn = 1
	for _, p := range []int{0, 2} {
		g.Vertices[p].Owner, g.Vertices[p].Level = p, 2
	}
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 10, Strength: 1, Active: true}, {Owner: 2, Vertex: 11, Strength: 1, Active: true}}
	k.BarbarianPosition = 6
	ckProgressGive(t, s, 0, 3, 4, 5, 6)
	ckProgressTop(t, s, 23, 0)
	if err := s.catanCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	if s.CatanPendingActor() != 2 {
		t.Fatal("reward clockwise order")
	}
	helperApply(t, s, 2, Action{Type: "catan_defender_reward", Color: 2})
	if s.CatanPendingActor() != 0 || s.Catan.CitiesKnights.Players[2].ProgressPoints != 1 {
		t.Fatal("immediate reward VP")
	}
	helperApply(t, s, 0, Action{Type: "catan_defender_reward", Color: 0})
	if s.Phase != "catan_progress_discard" || s.Catan.CitiesKnights.Invasions != 0 {
		t.Fatal("reward overflow must settle before finishing attack")
	}
	restored := clone(*s)
	s = &restored
	s.AutoCatanPending()
	if s.Phase != "catan_turn" || s.Catan.CitiesKnights.Invasions != 1 {
		t.Fatal("reward discard continuation")
	}
	for _, p := range s.Catan.CitiesKnights.Players {
		if p.DefenderPoints != 0 {
			t.Fatal("ties awarded defender points")
		}
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanCityEventsFallenCityRetainsPieceAndUpgradePriority(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	k := g.CitiesKnights
	for v := range 9 {
		g.Vertices[v].Owner = 0
		g.Vertices[v].Level = 2
		if v >= 1 && v <= 5 {
			g.Vertices[v].Level = 1
		}
	}
	k.BarbarianPosition = 6
	k.Walls = []int{0}
	helperGrant(s, 0, []int{0, 0, 0, 2, 3, 0, 0, 0})
	if err := s.catanCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, 0, Action{Type: "catan_pillage", Vertex: 0})
	g = s.Catan
	k = g.CitiesKnights
	if !reflect.DeepEqual(k.FallenCities, []int{0}) || g.cityPiecesLeft(0) != 0 || g.settlementPiecesLeft(0) != 0 || !g.canCityUpgrade(0, 0) || g.canCityUpgrade(0, 1) {
		t.Fatal("fallen city physical piece accounting")
	}
	helperReject(t, s, 0, Action{Type: "catan_city", Vertex: 1})
	restored := clone(*s)
	s = &restored
	legal := s.View(0)["catan"].(map[string]any)["legal"].(map[string][]int)
	if !reflect.DeepEqual(legal["cities"], []int{0}) {
		t.Fatal("fallen city legal upgrades", legal["cities"])
	}
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: 0})
	if len(s.Catan.CitiesKnights.FallenCities) != 0 || s.Catan.Vertices[0].Level != 2 || s.Catan.cityPiecesLeft(0) != 0 || s.Catan.settlementPiecesLeft(0) != 0 {
		t.Fatal("restored city piece accounting")
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanCityEventsEmptyDeckAndHiddenOrderDoNotAffectBot(t *testing.T) {
	s := ckGame(t, 6)
	s.Catan.SetupStep = 12
	s.Phase = "catan_roll"
	s.Turn = 0
	k := s.Catan.CitiesKnights
	// The entire trade deck can legally be held among six players (three each).
	for len(k.ProgressDecks[1]) > 0 {
		p := (18 - len(k.ProgressDecks[1])) / 3
		c := k.ProgressDecks[1][len(k.ProgressDecks[1])-1]
		ckProgressGive(t, s, p, c)
	}
	k.Players[0].Improvements[1] = 1
	if err := s.catanCityRoll(1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if len(k.Players[0].Progress) != 3 || k.Event != nil || s.Phase != "catan_turn" {
		t.Fatal("empty event deck stalled or invented card")
	}
	ckProgressStock(t, s.Catan)
	k.Event = &CatanCityEvent{Red: 1, Yellow: 1, Face: 3, Attack: true, Tasks: []CatanCityEventTask{{Kind: "defender_reward", Player: 0}}}
	k.Pending = &CatanCityPending{Kind: "defender_reward", Players: []int{0}}
	s.Phase = "catan_defender_reward"
	helperReject(t, s, 0, Action{Type: "catan_defender_reward", Color: 1})
	a, err := s.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	other := clone(*s)
	slices.Reverse(other.Catan.CitiesKnights.ProgressDecks[0])
	slices.Reverse(other.Catan.CitiesKnights.ProgressDecks[2])
	other.Catan.CitiesKnights.Players[1].Progress = []int{0, 1, 2}
	b, err := other.BotAction(0)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("reward bot reads hidden cards")
	}
	originalView := s.View(-1)["catan"].(map[string]any)["citiesKnights"]
	changedView := other.View(-1)["catan"].(map[string]any)["citiesKnights"]
	if !reflect.DeepEqual(originalView, changedView) {
		t.Fatal("deck ordering/opponent kinds exposed in spectator view")
	}
}
func TestCatanCityEventsOwnTurnVictoryAndDepartedProgressReturn(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	k := g.CitiesKnights
	g.Vertices[1].Owner, g.Vertices[1].Level = 1, 2
	k.Players[1].DefenderPoints = 10
	k.Knights = []CatanKnight{{Owner: 1, Vertex: 10, Strength: 1, Active: true}}
	k.BarbarianPosition = 6
	if err := s.catanCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	if s.Finished || k.Players[1].DefenderPoints != 11 || g.Players[1].Score != 13 {
		t.Fatal("nonactive defender won before their own turn")
	}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{1}) {
		t.Fatal("13-point defender did not win on own turn")
	}
	s = ckEvent(t)
	s.Phase = "catan_turn"
	ckProgressGive(t, s, 0, 0, 10, 16)
	if err := s.EliminateCatan(0); err != nil {
		t.Fatal(err)
	}
	if len(s.Catan.CitiesKnights.Players[0].Progress) != 0 {
		t.Fatal("departed progress cards held forever")
	}
	ckProgressStock(t, s.Catan)
}
