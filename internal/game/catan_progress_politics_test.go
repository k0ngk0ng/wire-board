package game

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCatanProgressPoliticsEncouragementPreservesActiveLocks(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 1})
	k := s.Catan.CitiesKnights
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1, Active: true, ActivatedAt: 2}, {Owner: 0, Vertex: 1, Strength: 2}, {Owner: 1, Vertex: 3, Strength: 3}}
	ckProgressGive(t, s, 0, 17, 17)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 17})
	g := s.Catan
	if !g.knightCanAct(g.knightAt(0)) || g.knightCanAct(g.knightAt(1)) || !g.knightAt(1).Active || g.knightAt(3).Active {
		t.Fatal("Encouragement changed existing locks or opponent")
	}
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 17}) // No new activations, still consumed.
	if !s.Catan.knightCanAct(s.Catan.knightAt(0)) {
		t.Fatal("second encouragement reset existing lock")
	}
	ckProgressStock(t, s.Catan)
	ckKnightStock(t, s.Catan)
}
func TestCatanProgressPoliticsEspionagePrivacyTransferAndEndLimit(t *testing.T) {
	s := ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 18, 0, 1, 4, 6)
	ckProgressGive(t, s, 1, 7, 8)
	for _, p := range []int{-1, 0, 3} {
		helperReject(t, s, 0, Action{Type: "catan_progress", Card: 18, Target: p})
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 18, Target: 1})
	for _, viewer := range []int{-1, 0, 1, 2} {
		v := s.View(viewer)["catan"].(map[string]any)
		k := v["citiesKnights"].(map[string]any)
		hand, ok := k["pending"].(map[string]any)["progress"]
		if ok != (viewer == 0) || (ok && !reflect.DeepEqual(hand, []int{7, 8})) {
			t.Fatal("espionage leaked hand")
		}
		for p, raw := range k["players"].([]any) {
			_, visible := raw.(map[string]any)["progress"]
			if visible != (p == viewer) {
				t.Fatal("ordinary hand privacy loosened")
			}
		}
	}
	helperReject(t, s, 1, Action{Type: "catan_espionage", Card: 7})
	for _, card := range []int{-1, 9, 23, 24, 99} {
		helperReject(t, s, 0, Action{Type: "catan_espionage", Card: card})
	}
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 4})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_espionage", Card: 7})
	if len(s.Catan.CitiesKnights.Players[0].Progress) != 5 || !reflect.DeepEqual(s.Catan.CitiesKnights.Players[1].Progress, []int{8}) {
		t.Fatal("spy transfer incorrect")
	}
	for _, log := range s.Log {
		if strings.Contains(log, "道路建设") {
			t.Fatal("stolen card identity exposed")
		}
	}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	if s.Phase != "catan_progress_end" {
		t.Fatal("spy bypassed end-of-turn limit")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress_discard", Cards: []int{7}})
	ckProgressStock(t, s.Catan)
	s = ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 18, 18)
	ckProgressGive(t, s, 1, 4)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 18, Target: 1})
	helperApply(t, s, 0, Action{Type: "catan_espionage", Choice: "skip"})
	if len(s.Catan.CitiesKnights.Players[1].Progress) != 1 {
		t.Fatal("optional spy take wasn't optional")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 18, Target: 1})
	helperApply(t, s, 0, Action{Type: "catan_espionage", Card: 4})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 4}) // Newly stolen cards are immediately usable.
	ckProgressStock(t, s.Catan)
	s = ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 18)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 18, Target: 2})
	if s.CatanPendingActor() != -1 {
		t.Fatal("empty spy target blocked play")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressPoliticsWeddingSabotageQueuesAndWalls(t *testing.T) {
	for _, card := range []int{20, 24} {
		s := ckGame(t, 6)
		g := s.Catan
		g.SetupStep = 12
		g.TurnSerial = 1
		s.Turn = 0
		s.Phase = "catan_turn"
		for p, score := range []int{3, 3, 4, 2, 5, 3} {
			g.CitiesKnights.Players[p].DefenderPoints = score
		}
		// A wall changes seven's threshold, never Sabotage's half-hand requirement.
		g.Vertices[0].Owner, g.Vertices[0].Level = 1, 2
		g.CitiesKnights.Walls = []int{0}
		g.CitiesKnights.Players[1].DefenderPoints = 1
		s.catanScores()
		ckProgressGive(t, s, 0, card)
		for p, n := range []int{5, 3, 5, 4, 1, 0} {
			cards := make([]int, 8)
			cards[p%5] = n
			helperGrant(s, p, cards)
		}
		// Swap one card in a victim's hand to a commodity, preserving inventories.
		g.Players[2].Resources[2]--
		g.Bank[2]++
		g.Players[2].Resources[5]++
		g.Bank[5]--
		helperApply(t, s, 0, Action{Type: "catan_progress", Card: card})
		kind := "sabotage"
		want := []int{1, 2}
		if card == 24 {
			kind = "wedding"
			want = []int{2, 4}
		}
		if !reflect.DeepEqual(s.Catan.CitiesKnights.Pending.Players, want) {
			t.Fatal("wrong score/hand eligibility", card, s.Catan.CitiesKnights.Pending.Players)
		}
		helperReject(t, s, 0, Action{Type: "catan_" + kind, Give: make([]int, 8)})
		helperReject(t, s, want[0], Action{Type: "catan_" + kind, Give: make([]int, 8)})
		for _, p := range want {
			if s.CatanPendingActor() != p {
				t.Fatal("wrong response order")
			}
			restored := clone(*s)
			s = &restored
			a, err := s.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			helperApply(t, s, p, a)
			ckProgressStock(t, s.Catan)
		}
		if s.CatanPendingActor() != -1 || s.Phase != "catan_turn" {
			t.Fatal("politics queue stuck")
		}
		if card == 20 && (sum(s.Catan.Players[0].Resources) != 5 || sum(s.Catan.Players[1].Resources) != 2 || sum(s.Catan.Players[2].Resources) != 3) {
			t.Fatal("sabotage affected caster or used seven's discard rule")
		}
		if card == 24 && (sum(s.Catan.Players[0].Resources) != 8 || sum(s.Catan.Players[4].Resources) != 0 || sum(s.Catan.Players[1].Resources) != 3) {
			t.Fatal("wedding strict VP/shortage handling")
		}
	}
}
func TestCatanProgressPoliticsIntrigueRetreatAndNoRoute(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 1, 1})
	s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: 1, Vertex: 1, Strength: 3, Active: true, ActivatedAt: 2, PromotedAt: 3}}
	ckProgressGive(t, s, 0, 19)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 19, Vertex: 2})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 19, Vertex: 1})
	if s.CatanPendingActor() != 1 || s.Catan.knightAt(1) != nil {
		t.Fatal("intrigue did not trigger defender retreat")
	}
	helperReject(t, s, 0, Action{Type: "catan_knight_retreat", Vertex: 2})
	helperReject(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: 1})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: 3})
	n := s.Catan.knightAt(3)
	if n == nil || !n.Active || n.Strength != 3 || n.ActivatedAt != 2 || n.PromotedAt != 3 {
		t.Fatal("intrigue lost retreat state")
	}
	ckProgressStock(t, s.Catan)
	ckKnightStock(t, s.Catan)
	s = ckKnightGraph(t, []int{0, 0})
	s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: 1, Vertex: 1, Strength: 3}}
	ckProgressGive(t, s, 0, 19)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 19, Vertex: 1})
	if s.CatanPendingActor() != -1 || len(s.Catan.CitiesKnights.Knights) != 0 {
		t.Fatal("no-route displaced knight must return to stock")
	}
	ckKnightStock(t, s.Catan)
}
func TestCatanProgressPoliticsTaxationUniqueVictimsAndFirstInvasion(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	ckProgressGive(t, s, 0, 21)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: 1, Choice: "skip"})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: 1})
	g = s.Catan
	g.CitiesKnights.Invasions = 1
	g.Robber = 0
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Vertices: []int{0}}, {ID: 1, Resource: 1, Vertices: []int{0, 1, 2, 3}}, {ID: 2, Resource: CatanSea}}
	for v, p := range []int{0, 1, 1, 2} {
		g.Vertices[v].Owner, g.Vertices[v].Level = p, 1
	}
	helperGrant(s, 0, []int{2, 0, 0, 0, 0, 0, 0, 0})
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 3, 0, 0})
	helperGrant(s, 2, []int{0, 0, 0, 0, 2, 0, 0, 0})
	ckProgressGive(t, s, 1, 4)
	for _, tile := range []int{-1, 0, 2, 3} {
		helperReject(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: tile})
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: 1})
	g = s.Catan
	if !reflect.DeepEqual(g.Players[0].Resources, []int{2, 0, 0, 0, 1, 1, 0, 0}) || g.Players[1].Resources[5] != 2 || len(g.CitiesKnights.Players[1].Progress) != 1 || s.Phase != "catan_turn" || g.Robber != 1 {
		t.Fatal("taxation duplicates victims or stole wrong card zone")
	}
	for _, log := range s.Log {
		if strings.Contains(log, "纸张") || strings.Contains(log, "矿石") {
			t.Fatal("random theft revealed private type")
		}
	}
	ckProgressStock(t, g)
}
func TestCatanProgressPoliticsTreasonChoicesInventoryAndImmediateAction(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 1, 1})
	k := s.Catan.CitiesKnights
	k.Knights = []CatanKnight{{Owner: 1, Vertex: 3, Strength: 3, Active: true, ActivatedAt: 2}, {Owner: 1, Vertex: 5, Strength: 1}}
	ckProgressGive(t, s, 0, 22)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 22, Target: 2})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 22, Target: 1})
	helperReject(t, s, 0, Action{Type: "catan_treason_remove", Vertex: 3})
	helperReject(t, s, 1, Action{Type: "catan_treason_remove", Vertex: 0})
	helperReject(t, s, 1, Action{Type: "catan_treason_remove", Choice: "skip"})
	helperApply(t, s, 1, Action{Type: "catan_treason_remove", Vertex: 3})
	if s.Catan.knightCount(1, 3) != 0 {
		t.Fatal("Treason reference must not reserve the removed opponent piece")
	}
	if s.CatanPendingActor() != 0 || s.Phase != "catan_treason_place" {
		t.Fatal("missing separate optional placement")
	}
	helperReject(t, s, 1, Action{Type: "catan_treason_place", Vertex: 1, Color: 3})
	helperReject(t, s, 0, Action{Type: "catan_treason_place", Vertex: 4, Color: 3})
	helperReject(t, s, 0, Action{Type: "catan_treason_place", Vertex: 1, Color: 4})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_treason_place", Vertex: 1, Color: 3})
	if s.Catan.CitiesKnights.Players[0].Improvements[CatanPolitics] != 0 || !s.Catan.knightCanAct(s.Catan.knightAt(1)) {
		t.Fatal("deserted mighty knight requires neither fortress nor new activation")
	}
	helperApply(t, s, 0, Action{Type: "catan_knight_move", Vertex: 1, Target: 2})
	ckKnightStock(t, s.Catan)
	ckProgressStock(t, s.Catan)
	for _, decline := range []bool{false, true} {
		s = ckKnightGraph(t, []int{0, 0, 0, 1, 1})
		s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1}, {Owner: 0, Vertex: 1, Strength: 1}, {Owner: 1, Vertex: 5, Strength: 1}}
		if decline {
			s.Catan.CitiesKnights.Knights = s.Catan.CitiesKnights.Knights[1:]
		}
		ckProgressGive(t, s, 0, 22)
		helperApply(t, s, 0, Action{Type: "catan_progress", Card: 22, Target: 1})
		helperApply(t, s, 1, Action{Type: "catan_treason_remove", Vertex: 5})
		if decline {
			helperApply(t, s, 0, Action{Type: "catan_treason_place", Choice: "skip"})
		}
		if s.CatanPendingActor() != -1 || s.Catan.knightAt(5) != nil {
			t.Fatal("opponent removal must hold despite unavailable/declined placement")
		}
		ckKnightStock(t, s.Catan)
		ckProgressStock(t, s.Catan)
	}
}
func TestCatanProgressPoliticsBotAndHiddenIndependence(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 1, 1})
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	g.Vertices[4].Owner, g.Vertices[4].Level = 1, 2
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 1, Strength: 1}, {Owner: 1, Vertex: 2, Strength: 2}}
	s.catanScores()
	ckProgressGive(t, s, 0, 16, 17, 18, 19, 20, 21, 22, 24)
	ckProgressGive(t, s, 1, 0, 1)
	helperGrant(s, 1, []int{1, 1, 1, 1, 0, 0, 0, 0})
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_progress" {
		t.Fatal("politics bot", a, err)
	}
	other := clone(*s)
	other.Catan.Players[1].Resources = []int{0, 0, 0, 0, 0, 0, 4, 0}
	other.Catan.CitiesKnights.Players[1].Progress = []int{7, 8}
	slices.Reverse(other.Catan.CitiesKnights.ProgressDecks[2])
	b, err := other.BotAction(0)
	if err != nil || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(g.politicsBotChoices(0), other.Catan.politicsBotChoices(0)) {
		t.Fatal("politics bot inspected hidden cards")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 18, Target: 1})
	a, err = s.BotAction(0)
	if err != nil || a.Type != "catan_espionage" || !slices.Contains([]int{0, 1}, a.Card) {
		t.Fatal("spy response bot")
	}
	helperApply(t, s, 0, a)
	ckProgressStock(t, s.Catan)
}
