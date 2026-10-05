package game

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCatanProgressTradeMerchantOwnershipRatesAndVictory(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Ports = nil
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Vertices: []int{0, 1}}, {ID: 1, Resource: CatanDesert, Vertices: []int{0}}, {ID: 2, Resource: CatanGold, Vertices: []int{0}}, {ID: 3, Resource: CatanSea, Vertices: []int{0}}, {ID: 4, Resource: 4, Vertices: []int{2}}}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	g.Vertices[1].Owner, g.Vertices[1].Level = 1, 1
	g.Robber = 0
	ckProgressGive(t, s, 0, 12, 12)
	ckProgressGive(t, s, 1, 12)
	for _, tile := range []int{-1, 2, 3, 4, 5} {
		helperReject(t, s, 0, Action{Type: "catan_progress", Card: 12, Tile: tile})
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 12, Tile: 0})
	if s.Catan.Players[0].Score != 2 || s.Catan.rates(0)[0] != 2 || s.Catan.rates(0)[5] != 4 || s.Catan.rates(1)[0] != 4 {
		t.Fatal("merchant point or resource-only trading wrong")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 12, Tile: 1})
	if s.Catan.Players[0].Score != 2 || s.Catan.rates(0)[0] != 4 {
		t.Fatal("desert merchant must retain VP, not old trade power")
	}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	s.Phase = "catan_turn"
	helperApply(t, s, 1, Action{Type: "catan_progress", Card: 12, Tile: 0})
	if s.Catan.Players[0].Score != 1 || s.Catan.Players[1].Score != 2 || s.Catan.rates(1)[0] != 2 {
		t.Fatal("merchant theft did not transfer VP and rate")
	}
	restored := clone(*s)
	if !reflect.DeepEqual(restored.Catan.CitiesKnights.Merchant, s.Catan.CitiesKnights.Merchant) {
		t.Fatal("merchant did not persist")
	}
	if err := s.EliminateCatan(1); err != nil {
		t.Fatal(err)
	}
	if s.Catan.CitiesKnights.Merchant != nil {
		t.Fatal("departed player retained merchant")
	}
	ckProgressStock(t, s.Catan)
	s = ckEmptyTurn(t)
	s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 0, 1
	s.Catan.Tiles[0].Vertices = []int{0}
	s.Catan.Tiles[0].Resource = 0
	s.Catan.CitiesKnights.Players[0].DefenderPoints = 11
	ckProgressGive(t, s, 0, 12)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 12, Tile: 0})
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) {
		t.Fatal("merchant 13th VP did not end game")
	}
}
func TestCatanProgressTradeFleetMultipleKindsBanksAndPairedExpiry(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := ckGame(t, n)
		g := s.Catan
		g.SetupStep = 2 * n
		g.TurnSerial = 1
		g.Ports = nil
		s.Turn = 0
		s.Phase = "catan_turn"
		if g.Paired != nil {
			g.Paired = &CatanPairedTurn{Primary: 0, Secondary: 3}
		}
		ckProgressGive(t, s, 0, 13, 13)
		helperGrant(s, 0, []int{4, 0, 0, 0, 0, 4, 0, 0})
		for _, c := range []int{-1, 8} {
			helperReject(t, s, 0, Action{Type: "catan_progress", Card: 13, Color: c})
		}
		helperApply(t, s, 0, Action{Type: "catan_progress", Card: 13, Color: 0})
		helperApply(t, s, 0, Action{Type: "catan_progress", Card: 13, Color: 5})
		if s.Catan.rates(0)[0] != 2 || s.Catan.rates(0)[5] != 2 || s.Catan.rates(1)[5] != 4 {
			t.Fatal("multiple fleets or owner restriction")
		}
		restored := clone(*s)
		s = &restored
		for range 2 {
			helperApply(t, s, 0, Action{Type: "catan_bank", Give: []int{2, 0, 0, 0, 0, 2, 0, 0}, Take: []int{0, 1, 0, 0, 0, 0, 1, 0}})
		}
		if sum(s.Catan.Players[0].Resources) != 4 {
			t.Fatal("fleet may make repeated mixed trades")
		}
		helperApply(t, s, 0, Action{Type: "catan_end"})
		if s.Catan.CitiesKnights.TradePowers != nil || s.Catan.rates(0)[0] != 4 || s.Catan.rates(0)[5] != 4 {
			t.Fatal("fleet persisted into next action phase")
		}
		if n == 6 && (s.Turn != 3 || s.Catan.TurnSerial != 1) {
			t.Fatal("not checked paired phase")
		}
		ckProgressStock(t, s.Catan)
	}
}
func TestCatanProgressTradeMonopoliesLimitsAndKinds(t *testing.T) {
	for _, card := range []int{14, 15} {
		for _, n := range []int{3, 6} {
			s := ckGame(t, n)
			g := s.Catan
			g.SetupStep = 2 * n
			g.TurnSerial = 1
			s.Turn = 0
			s.Phase = "catan_turn"
			ckProgressGive(t, s, 0, card)
			color, limit := 0, 2
			if card == 15 {
				color, limit = 5, 1
			}
			want := 0
			for p := 1; p < n; p++ {
				count := p
				if p == 2 {
					count = 4
				}
				if p > 2 {
					count = 0
				}
				cards := make([]int, 8)
				cards[color] = count
				helperGrant(s, p, cards)
				want += min(count, limit)
			}
			wrong := 5
			if card == 15 {
				wrong = 0
			}
			for _, c := range []int{-1, wrong, 8} {
				helperReject(t, s, 0, Action{Type: "catan_progress", Card: card, Color: c})
			}
			bank := append([]int{}, s.Catan.Bank...)
			helperApply(t, s, 0, Action{Type: "catan_progress", Card: card, Color: color})
			if s.Catan.Players[0].Resources[color] != want || !reflect.DeepEqual(bank, s.Catan.Bank) || s.Catan.Players[2].Resources[color] != 4-limit {
				t.Fatal("monopoly should transfer capped cards from each player, not the supply")
			}
			ckProgressStock(t, s.Catan)
		}
	}
	s := ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 14)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 14, Color: 0})
	ckProgressStock(t, s.Catan) // May be played for zero gain.
}
func TestCatanProgressTradeGuildDuesPrivateChoiceAndSmallHands(t *testing.T) {
	for _, count := range []int{0, 1, 4} {
		s := ckEmptyTurn(t)
		s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 1, 2
		s.Catan.Vertices[1].Owner, s.Catan.Vertices[1].Level = 0, 1
		s.catanScores()
		ckProgressGive(t, s, 0, 11)
		hand := []int{0, 0, 0, 0, 0, 0, 0, 0}
		hand[0] = min(count, 1)
		hand[5] = max(0, count-1)
		helperGrant(s, 1, hand)
		for _, p := range []int{-1, 0, 2, 3} {
			helperReject(t, s, 0, Action{Type: "catan_progress", Card: 11, Target: p})
		}
		helperApply(t, s, 0, Action{Type: "catan_progress", Card: 11, Target: 1})
		if count == 0 {
			if s.Phase != "catan_turn" || s.CatanPendingActor() != -1 {
				t.Fatal("empty hand blocked play")
			}
			ckProgressStock(t, s.Catan)
			continue
		}
		if s.CatanPendingActor() != 0 || s.Phase != "catan_guild_dues" {
			t.Fatal("missing private follow-up")
		}
		for _, viewer := range []int{-1, 0, 1, 2} {
			view := s.View(viewer)["catan"].(map[string]any)
			pending := view["citiesKnights"].(map[string]any)["pending"].(map[string]any)
			revealed, ok := pending["resources"]
			if ok != (viewer == 0) || (ok && !reflect.DeepEqual(revealed, hand)) {
				t.Fatal("hand reveal to wrong viewer")
			}
			for p, raw := range view["players"].([]any) {
				_, visible := raw.(map[string]any)["resources"]
				if visible != (viewer == p) {
					t.Fatal("main hand privacy changed")
				}
			}
		}
		helperReject(t, s, 0, Action{Type: "catan_end"})
		take := make([]int, 8)
		take[0] = 1
		if count > 1 {
			take[5] = 1
		}
		for _, p := range []int{1, 2} {
			helperReject(t, s, p, Action{Type: "catan_guild_dues", Take: take})
		}
		helperReject(t, s, 0, Action{Type: "catan_guild_dues", Take: []int{0, 0, 0, 0, 0, 0, 0, 0}})
		helperReject(t, s, 0, Action{Type: "catan_guild_dues", Take: []int{0, 2, 0, 0, 0, 0, 0, 0}})
		restored := clone(*s)
		s = &restored
		helperApply(t, s, 0, Action{Type: "catan_guild_dues", Take: take})
		if !reflect.DeepEqual(s.Catan.Players[0].Resources, take) || s.CatanPendingActor() != -1 || s.Phase != "catan_turn" {
			t.Fatal("private pick failed")
		}
		for _, log := range s.Log {
			if strings.Contains(log, "纸张") || strings.Contains(log, "木材") {
				t.Fatal("private choice leaked in log")
			}
		}
		ckProgressStock(t, s.Catan)
	}
}
func TestCatanProgressTradeCommercialHarborInterleavingRepeatAndMandatoryChoice(t *testing.T) {
	s := ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 10, 10)
	helperGrant(s, 0, []int{5, 0, 0, 0, 0, 0, 0, 0})
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 2, 1, 0})
	helperGrant(s, 2, []int{0, 0, 0, 0, 1, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_commercial_offer", Card: 0, Target: 1, Color: 0})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 10})
	for _, a := range []Action{{Type: "catan_commercial_offer", Card: 1, Target: 1}, {Type: "catan_commercial_offer", Target: 0}, {Type: "catan_commercial_offer", Target: 8}, {Type: "catan_commercial_offer", Target: 1, Color: 5}, {Type: "catan_commercial_offer", Target: 1, Color: 1}} {
		helperReject(t, s, 0, a)
	}
	helperApply(t, s, 0, Action{Type: "catan_commercial_offer", Target: 1, Color: 0})
	if s.CatanPendingActor() != 1 || s.Catan.Players[0].Resources[0] != 5 {
		t.Fatal("harbor must await target's choice")
	}
	for _, p := range []int{0, 2} {
		helperReject(t, s, p, Action{Type: "catan_commercial_harbor", Color: 5})
	}
	for _, c := range []int{-1, 0, 7, 8} {
		helperReject(t, s, 1, Action{Type: "catan_commercial_harbor", Color: c})
	}
	helperReject(t, s, 1, Action{Type: "catan_commercial_harbor", Choice: "skip", Color: 5})
	helperReject(t, s, 0, Action{Type: "catan_end"})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 1, Action{Type: "catan_commercial_harbor", Color: 6})
	if s.Catan.Players[0].Resources[0] != 4 || s.Catan.Players[0].Resources[6] != 1 || s.Catan.Players[1].Resources[0] != 1 {
		t.Fatal("harbor did not exchange chosen cards")
	}
	helperReject(t, s, 0, Action{Type: "catan_commercial_offer", Target: 1, Color: 0})
	helperApply(t, s, 0, Action{Type: "catan_bank", Give: []int{4, 0, 0, 0, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0, 0, 0, 0}})
	helperApply(t, s, 0, Action{Type: "catan_commercial_offer", Target: 2, Color: 1})
	if s.CatanPendingActor() != -1 || s.Catan.Players[0].Resources[1] != 1 {
		t.Fatal("no commodities should return offered resource immediately")
	}
	helperReject(t, s, 0, Action{Type: "catan_commercial_offer", Target: 2, Color: 1})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 10})
	helperApply(t, s, 0, Action{Type: "catan_commercial_offer", Card: 1, Target: 1, Color: 1})
	s.AutoCatanPending()
	if s.CatanPendingActor() != -1 || s.Catan.Players[0].Resources[5] != 1 {
		t.Fatal("second card must permit a new offer to same opponent")
	}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	if s.Catan.CitiesKnights.TradePowers != nil {
		t.Fatal("harbor did not expire")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressTradeBotsHiddenIndependenceAndResponses(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	g.Vertices[1].Owner, g.Vertices[1].Level = 1, 2
	s.catanScores()
	ckProgressGive(t, s, 0, 10, 11, 12, 13, 14, 15)
	helperGrant(s, 0, []int{3, 0, 0, 0, 0, 2, 0, 0})
	helperGrant(s, 1, []int{1, 0, 0, 0, 0, 0, 3, 0})
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_progress" {
		t.Fatal("missing trade bot", a, err)
	}
	other := clone(*s)
	other.Catan.Players[1].Resources = []int{0, 0, 0, 4, 0, 0, 0, 0}
	slices.Reverse(other.Catan.CitiesKnights.ProgressDecks[1])
	b, err := other.BotAction(0)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("trade bot read hidden composition/order", a, b, err)
	}
	if !reflect.DeepEqual(g.tradeProgressBotChoices(0), other.Catan.tradeProgressBotChoices(0)) {
		t.Fatal("trade candidates depend on hidden cards")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 11, Target: 1})
	a, err = s.BotAction(0)
	if err != nil || a.Type != "catan_guild_dues" || sum(a.Take) != 2 {
		t.Fatal("bot did not use authorized revealed hand", a, err)
	}
	helperApply(t, s, 0, a)
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressTradeSkipAndPhaseGuards(t *testing.T) {
	s := ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 10, 11, 12, 13, 14, 15)
	for _, card := range []int{10, 11, 12, 13, 14, 15} {
		helperReject(t, s, 1, Action{Type: "catan_progress", Card: card})
		s.Phase = "catan_roll"
		helperReject(t, s, 0, Action{Type: "catan_progress", Card: card})
		s.Phase = "catan_turn"
		helperReject(t, s, 0, Action{Type: "catan_progress", Card: card, Choice: "unknown"})
		helperApply(t, s, 0, Action{Type: "catan_progress", Card: card, Choice: "skip"})
	}
	if s.Catan.CitiesKnights.TradePowers != nil || s.Catan.CitiesKnights.Merchant != nil || s.Catan.CitiesKnights.Pending != nil {
		t.Fatal("waived card applied effect")
	}
	ckProgressStock(t, s.Catan)
}
