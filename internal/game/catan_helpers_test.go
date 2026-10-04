package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func helperTestGame(t *testing.T, id int) *State {
	t.Helper()
	s, err := NewCatan(3, CatanOptions{Helpers: true, AllHelpers: true})
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = 6
	g.TurnSerial = 10
	s.Turn = 0
	s.Phase = "catan_turn"
	g.Players[0].Helper = &CatanHelperSeat{ID: id, AcquiredTurn: 1}
	g.HelperDisplay = nil
	for i := 1; i <= 12; i++ {
		if i != id {
			g.HelperDisplay = append(g.HelperDisplay, i)
		}
	}
	return s
}
func helperApply(t *testing.T, s *State, player int, a Action) {
	t.Helper()
	if err := s.Apply(player, a); err != nil {
		t.Fatalf("phase %s player %d action %+v: %v", s.Phase, player, a, err)
	}
}
func helperReject(t *testing.T, s *State, player int, a Action) {
	t.Helper()
	before, _ := json.Marshal(s)
	if err := s.Apply(player, a); err == nil {
		t.Fatalf("accepted invalid %+v", a)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("rejected helper action mutated state")
	}
}
func helperGrant(s *State, player int, resources []int) {
	catanMove(s.Catan.Bank, s.Catan.Players[player].Resources, resources)
}

func TestCatanHelpersSetupDisplayAndStartingStack(t *testing.T) {
	for _, all := range []bool{false, true} {
		s, err := NewCatan(4, CatanOptions{Helpers: true, AllHelpers: all})
		if err != nil {
			t.Fatal(err)
		}
		want := 4
		if all {
			want = 8
		}
		if len(s.Catan.HelperDisplay) != want {
			t.Fatal("incorrect random helper display")
		}
		for s.Catan.setup() {
			a, err := s.BotAction(s.Turn)
			if err != nil {
				t.Fatal(err)
			}
			helperApply(t, s, s.Turn, a)
		}
		seen := map[int]bool{}
		for i, p := range s.Catan.Players {
			if p.Helper == nil || p.Helper.ID != i+1 || p.Helper.Moon || !s.Catan.helperReady(i, i+1) {
				t.Fatal("starting stack not taken in reverse setup order", i, p.Helper)
			}
			seen[p.Helper.ID] = true
		}
		for _, id := range s.Catan.HelperDisplay {
			if seen[id] || id <= 4 || id > 12 {
				t.Fatal("helper duplicate")
			}
			seen[id] = true
		}
	}
}

func TestCatanHelpersFlipExchangeAndSameTurnLock(t *testing.T) {
	s := helperTestGame(t, 11)
	helperApply(t, s, 0, Action{Type: "catan_helper", Color: 0})
	if s.Phase != "catan_helper" || s.Catan.HelperPending.Kind != "exchange" {
		t.Fatal("use did not prompt flip/exchange")
	}
	helperReject(t, s, 1, Action{Type: "catan_helper_choice", Choice: "flip"})
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	if !s.Catan.Players[0].Helper.Moon {
		t.Fatal("not flipped to moon")
	}
	helperReject(t, s, 0, Action{Type: "catan_helper", Color: 0})
	s.Catan.TurnSerial++
	helperApply(t, s, 0, Action{Type: "catan_helper", Color: 0})
	helperReject(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	helperReject(t, s, 0, Action{Type: "catan_helper_choice", Choice: "exchange", Card: 11})
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "exchange", Card: 10})
	if !slices.Contains(s.Catan.HelperDisplay, 11) || s.Catan.Players[0].Helper.Moon {
		t.Fatal("exchange did not reset sun side")
	}
	helperReject(t, s, 0, Action{Type: "catan_helper"})
}

func TestCatanHelpersForcedTradeAtomicAndCanReturnRequestedResource(t *testing.T) {
	s := helperTestGame(t, 1)
	helperGrant(s, 1, []int{0, 0, 0, 0, 1})
	helperGrant(s, 2, []int{0, 0, 0, 0, 1})
	// The first exchange is legal, the second attempts an unavailable brick.
	helperReject(t, s, 0, Action{Type: "catan_helper", Color: 4, Targets: []int{1, 2}, Cards: []int{4, 1}})
	helperApply(t, s, 0, Action{Type: "catan_helper", Color: 4, Targets: []int{1, 2}, Cards: []int{4, 4}})
	if s.Catan.Players[1].Resources[4] != 1 || s.Catan.Players[2].Resources[4] != 1 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("same resource return invalid")
	}
}

func TestCatanHelpersRoadSubstitutionAndEndRoadMovement(t *testing.T) {
	s := helperTestGame(t, 2)
	g := s.Catan
	g.Vertices[0].Owner = 0
	g.Vertices[0].Level = 1
	edge := -1
	for _, e := range g.Edges {
		if g.canRoad(0, e.ID) {
			edge = e.ID
			break
		}
	}
	helperGrant(s, 0, []int{1, 0, 1, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_road", Edge: edge, Skill: "helper", Tokens: []int{0, 0, 2, 0, 0}})
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: edge, Skill: "helper", Tokens: []int{1, 0, 1, 0, 0}})
	if s.Catan.Edges[edge].Owner != 0 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("substitution did not build road")
	}
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "exchange", Card: 4})
	s.Catan.TurnSerial++
	next := -1
	s.Catan.Edges[edge].Owner = -1
	for _, e := range s.Catan.Edges {
		if e.ID != edge && s.Catan.canRoad(0, e.ID) {
			next = e.ID
			break
		}
	}
	s.Catan.Edges[edge].Owner = 0
	helperReject(t, s, 0, Action{Type: "catan_helper", Edge: edge, Target: edge})
	helperApply(t, s, 0, Action{Type: "catan_helper", Edge: edge, Target: next})
	if s.Catan.Edges[edge].Owner != -1 || s.Catan.Edges[next].Owner != 0 {
		t.Fatal("road not relocated")
	}
}

func TestCatanHelpersHildaOnlyAfterNoProductionAndDecline(t *testing.T) {
	s := helperTestGame(t, 3)
	s.Turn = 1
	s.Catan.Dice = []int{1, 1}
	s.catanRoll(2)
	if s.Catan.HelperPending == nil || s.Catan.HelperPending.Player != 0 || s.Phase != "catan_helper" {
		t.Fatal("missing out-of-turn Hilda response")
	}
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "skip"})
	if s.Phase != "catan_turn" || !s.Catan.helperReady(0, 3) {
		t.Fatal("declined helper consumed")
	}
	s.Catan.TurnSerial++
	s.catanRoll(2)
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Color: 4})
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	if s.Turn != 1 || s.Phase != "catan_turn" || s.Catan.Players[0].Resources[4] != 1 {
		t.Fatal("response changed active player")
	}
	// Actual production suppresses compensation, even for just one resource.
	s.Catan.TurnSerial++
	for _, tile := range s.Catan.Tiles {
		if tile.Resource == 5 {
			continue
		}
		s.Catan.Vertices[tile.Vertices[0]].Owner = 0
		s.Catan.Vertices[tile.Vertices[0]].Level = 1
		s.catanRoll(tile.Number)
		break
	}
	if s.Catan.HelperPending != nil {
		t.Fatal("Hilda triggered despite production")
	}
}

func TestCatanHelpersThorolfProtectsBeforeOtherDiscards(t *testing.T) {
	for _, held := range []int{7, 8} {
		s := helperTestGame(t, 5)
		s.Turn = 2
		helperGrant(s, 0, []int{held, 0, 0, 0, 0})
		helperGrant(s, 1, []int{0, 8, 0, 0, 0})
		s.Catan.Dice = []int{3, 4}
		s.catanRoll(7)
		if s.Catan.DiscardDue[0] != 0 || s.Catan.DiscardDue[1] != 4 || s.Catan.HelperPending.Player != 0 {
			t.Fatal("seven protection/priority incorrect")
		}
		helperReject(t, s, 1, Action{Type: "catan_discard", Tokens: []int{0, 4, 0, 0, 0}})
		if held == 7 {
			helperApply(t, s, 0, Action{Type: "catan_helper_choice", Color: 2})
		}
		helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
		if s.Phase != "catan_discard" || sum(s.Catan.Players[0].Resources) != 8 {
			t.Fatal("helper did not resume normal seven")
		}
		helperApply(t, s, 1, Action{Type: "catan_discard", Tokens: []int{0, 4, 0, 0, 0}})
		if s.Phase != "catan_robber" {
			t.Fatal("robber not resumed")
		}
	}
}

func TestCatanHelpersDevelopmentChoicePrivateRestoreAndConservation(t *testing.T) {
	s := helperTestGame(t, 6)
	s.Catan.DevDeck = []int{2, 0, 1, 4}
	helperGrant(s, 0, []int{1, 0, 1, 1, 0})
	helperApply(t, s, 0, Action{Type: "catan_buy_dev", Skill: "helper", Tokens: []int{1, 0, 1, 1, 0}})
	for _, viewer := range []int{-1, 1, 2} {
		q := s.View(viewer)["catan"].(map[string]any)["helperPending"].(map[string]any)
		if _, ok := q["cards"]; ok {
			t.Fatal("development candidates leaked")
		}
	}
	if len(s.View(0)["catan"].(map[string]any)["helperPending"].(map[string]any)["cards"].([]any)) != 3 {
		t.Fatal("owner missing private candidates")
	}
	restored := clone(*s)
	helperReject(t, &restored, 0, Action{Type: "catan_helper_choice", Card: 2})
	helperApply(t, &restored, 0, Action{Type: "catan_helper_choice", Card: 4})
	if restored.Catan.Players[0].Dev[4] != 1 || restored.Catan.Players[0].NewDev[4] != 1 || len(restored.Catan.DevDeck) != 3 {
		t.Fatal("incorrect development selection")
	}
	slices.Sort(restored.Catan.DevDeck)
	if !reflect.DeepEqual(restored.Catan.DevDeck, []int{0, 1, 2}) {
		t.Fatal("unchosen cards lost")
	}
}

func TestCatanHelpersRyanRevealsOnlyToOwnerAndUsesPublicScore(t *testing.T) {
	s := helperTestGame(t, 7)
	s.Catan.Players[0].Score = 2
	s.Catan.Players[1].Score = 4
	s.Catan.Players[1].Dev[4] = 3
	helperGrant(s, 1, []int{0, 0, 0, 1, 1})
	helperReject(t, s, 0, Action{Type: "catan_helper", Target: 1})
	s.Catan.Players[1].Score = 6
	helperApply(t, s, 0, Action{Type: "catan_helper", Target: 1})
	for _, viewer := range []int{-1, 1, 2} {
		q := s.View(viewer)["catan"].(map[string]any)["helperPending"].(map[string]any)
		if _, ok := q["resources"]; ok {
			t.Fatal("revealed hand leaked to other viewer")
		}
	}
	if _, ok := s.View(0)["catan"].(map[string]any)["helperPending"].(map[string]any)["resources"]; !ok {
		t.Fatal("helper user missing revealed hand")
	}
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Color: 4})
	if s.Catan.Players[0].Resources[4] != 1 || s.Catan.Players[1].Resources[4] != 0 {
		t.Fatal("chosen resource not transferred")
	}
}

func TestCatanHelpersKnightBuildingCanLoseLargestArmy(t *testing.T) {
	s := helperTestGame(t, 8)
	g := s.Catan
	g.Vertices[0].Owner = 0
	g.Vertices[0].Level = 1
	g.Players[0].Knights = 3
	g.ArmyOwner = 0
	g.DevDiscard = []int{0, 0, 0}
	helperGrant(s, 0, []int{0, 0, 0, 1, 2})
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: 0, Skill: "helper"})
	g = s.Catan
	if g.Vertices[0].Level != 2 || g.Players[0].Knights != 2 || g.ArmyOwner != -1 || len(g.DevDiscard) != 2 || len(g.HelperExile) != 1 {
		t.Fatal("knight cost/army recomputation incorrect")
	}
}

func TestCatanHelpersTradeFrenzyOneResourceAndRobberAbilities(t *testing.T) {
	s := helperTestGame(t, 9)
	helperGrant(s, 0, []int{6, 2, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_helper", Give: []int{2, 2, 0, 0, 0}, Take: []int{0, 0, 1, 1, 0}})
	helperApply(t, s, 0, Action{Type: "catan_helper", Give: []int{6, 0, 0, 0, 0}, Take: []int{0, 0, 1, 1, 1}})
	if !reflect.DeepEqual(s.Catan.Players[0].Resources, []int{0, 2, 1, 1, 1}) {
		t.Fatal("batch two-for-one trade incorrect")
	}
	s = helperTestGame(t, 10)
	s.Phase = "catan_roll"
	resource := -1
	for _, tile := range s.Catan.Tiles {
		if tile.Resource < 5 {
			s.Catan.Robber = tile.ID
			resource = tile.Resource
			break
		}
	}
	helperApply(t, s, 0, Action{Type: "catan_helper"})
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	if s.Phase != "catan_roll" || s.Catan.Tiles[s.Catan.Robber].Resource != 5 || s.Catan.Players[0].Resources[resource] != 1 {
		t.Fatal("pre-roll robber chase incorrect")
	}
	s.Catan.TurnSerial++
	helperReject(t, s, 0, Action{Type: "catan_helper"})
}

func TestCatanHelpersSwapDevelopmentBottomAndNewCardRestriction(t *testing.T) {
	s := helperTestGame(t, 12)
	s.Catan.Players[0].Dev[0] = 1
	s.Catan.DevDeck = []int{4, 1}
	helperApply(t, s, 0, Action{Type: "catan_helper", Card: 0})
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	if !reflect.DeepEqual(s.Catan.DevDeck, []int{0, 4}) || s.Catan.Players[0].Dev[1] != 1 || s.Catan.Players[0].NewDev[1] != 1 {
		t.Fatal("swap changed deck order or old/new state")
	}
	helperReject(t, s, 0, Action{Type: "catan_dev", Card: 1})
}

func TestCatanHelpersTimeoutResolvesOtherPlayerAndSetup(t *testing.T) {
	s := helperTestGame(t, 3)
	s.Turn = 2
	s.catanRoll(2)
	s.AutoCatanPending()
	if s.Catan.HelperPending != nil || s.Turn != 2 || s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 1 {
		t.Fatal("helper timeout did not resolve owning player")
	}
	s, err := NewCatan(3, CatanOptions{Helpers: true})
	if err != nil {
		t.Fatal(err)
	}
	s.AutoCatanPending()
	if s.Catan.SetupStep != 1 {
		t.Fatal("helper game setup timeout did not complete exactly one seat")
	}
}

func TestCatanHelpersBotsCompleteAndConserveResources(t *testing.T) {
	s, err := NewCatan(3, CatanOptions{Helpers: true, AllHelpers: true})
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 4000 && !s.Finished; step++ {
		actor := s.Turn
		if q := s.Catan.HelperPending; q != nil {
			actor = q.Player
		} else if s.Phase == "catan_discard" {
			for i, n := range s.Catan.DiscardDue {
				if n > 0 {
					actor = i
					break
				}
			}
		}
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatalf("step %d phase %s: %v", step, s.Phase, err)
		}
		helperApply(t, s, actor, a)
		for color, n := range s.Catan.Bank {
			for _, p := range s.Catan.Players {
				n += p.Resources[color]
			}
			if n != 19 {
				t.Fatalf("resource %d conservation: %d", color, n)
			}
		}
	}
	if !s.Finished {
		t.Fatal("helpers bot game did not finish")
	}
}
