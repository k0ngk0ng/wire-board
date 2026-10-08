package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func fishingGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanFishing(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func fishGameReject(t *testing.T, s *State, player int, a Action) {
	t.Helper()
	before, _ := json.Marshal(s)
	if err := s.Apply(player, a); err == nil {
		t.Fatal("invalid fishing action accepted")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("invalid action changed state")
	}
}

func fishingProductionFixture(t *testing.T, n int) *State {
	t.Helper()
	s := fishingGame(t, n)
	g := s.Catan
	g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
	s.Turn, s.Phase = 0, "catan_roll"
	v := g.Tiles[g.Fishing.Map.Lakes[0].Tile].Vertices
	g.Vertices[v[0]].Owner, g.Vertices[v[0]].Level = 1, 2
	g.Vertices[v[3]].Owner, g.Vertices[v[3]].Level = 2, 1
	fishOwn(&g.Fishing.Tokens, 1, 0, 1, 2, 3, 4, 5, 6)
	fishOwn(&g.Fishing.Tokens, 2, 11, 12, 13, 14, 15, 16, 17)
	fishTop(&g.Fishing.Tokens, 21, 22, 23)
	if err := s.catanRollProduction(2); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanFishingStartingSettlementAndBootGoal(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, site := range []string{"lake", "coast"} {
			for _, second := range []bool{false, true} {
				s := fishingGame(t, n)
				g := s.Catan
				vertex := g.Fishing.Map.Grounds[0].Vertices[1]
				if site == "lake" {
					vertex = g.Tiles[g.Fishing.Map.Lakes[0].Tile].Vertices[0]
				}
				if second {
					g.SetupStep = n
				}
				s.Turn = 0
				fishTop(&g.Fishing.Tokens, 0)
				if err := s.Apply(0, Action{Type: "catan_settlement", Vertex: vertex}); err != nil {
					t.Fatal(err)
				}
				g = s.Catan
				want := 0
				if second {
					want = 1
				}
				if len(g.Fishing.Tokens.Hands[0]) != want || g.Fishing.Started[0] != second || s.Phase != "catan_setup_road" || g.SetupVertex != vertex {
					t.Fatal("starting fish", site, second)
				}
				fishGameReject(t, s, 0, Action{Type: "catan_settlement", Vertex: vertex})
				copy := clone(*s)
				if !reflect.DeepEqual(*s, copy) {
					t.Fatal("starting award restore")
				}
			}
		}
	}
	s := fishingGame(t, 3)
	g := s.Catan
	fishTop(&g.Fishing.Tokens, catanFishBoot)
	g.SetupStep, s.Turn = 3, 0
	vertex := g.Fishing.Map.Grounds[0].Vertices[0]
	if err := s.Apply(0, Action{Type: "catan_settlement", Vertex: vertex}); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	if g.Fishing.Tokens.BootOwner != 0 || len(g.Fishing.Tokens.Hands[0]) != 0 || g.victoryTargetFor(0) != 11 || g.victoryTargetFor(1) != 10 {
		t.Fatal("starting boot")
	}
	g.SetupStep = g.SetupLimit()
	g.Players[0].Score = 10
	s.catanVictory()
	if s.Finished {
		t.Fatal("boot owner won at base threshold")
	}
	g.Players[0].Score = 11
	s.catanVictory()
	if !s.Finished || g.Players[0].Score != 11 {
		t.Fatal("boot changes goal, not score")
	}
}

func TestCatanFishingProductionResponseRestoreAndNoRepeat(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := fishingProductionFixture(t, n)
		if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != 1 || s.Turn != 0 {
			t.Fatal("first responder")
		}
		resources := clone(s.Catan.Players)
		fishGameReject(t, s, 0, Action{Type: "catan_end"})
		fishGameReject(t, s, 2, Action{Type: "catan_fish_keep"})
		fishGameReject(t, s, 1, Action{Type: "catan_fish_replace", Card: 29})
		if err := s.Apply(1, Action{Type: "catan_fish_replace", Card: 0}); err != nil {
			t.Fatal(err)
		}
		if s.CatanPendingActor() != 2 || len(s.Catan.Fishing.Tokens.Hands[1]) != 7 || !slices.Contains(s.Catan.Fishing.Tokens.Hands[1], 21) {
			t.Fatal("second responder/replacement limit")
		}
		restored := clone(*s)
		s = &restored
		if err := s.Apply(2, Action{Type: "catan_fish_keep"}); err != nil {
			t.Fatal(err)
		}
		if s.Phase != "catan_turn" || s.Turn != 0 || s.CatanPendingActor() != -1 || s.Catan.Fishing.Pending != nil || !reflect.DeepEqual(resources, s.Catan.Players) {
			t.Fatal("response repeated resource production or changed active turn")
		}
		before, _ := json.Marshal(s)
		if err := s.catanRollProduction(2); err == nil {
			t.Fatal("repeated roll production accepted")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("repeat mutated state")
		}
		fishGameReject(t, s, 2, Action{Type: "catan_fish_keep"})
		if n > 4 {
			hands := clone(s.Catan.Fishing.Tokens)
			s.Catan.Paired.Primary, s.Catan.Paired.Secondary, s.Catan.Paired.Second = 0, 3, false
			if err := s.Apply(0, Action{Type: "catan_end"}); err != nil {
				t.Fatal(err)
			}
			if s.Turn != 3 || s.Phase != "catan_turn" || !reflect.DeepEqual(hands, s.Catan.Fishing.Tokens) {
				t.Fatal("paired second action produced fish again")
			}
		}
	}
}

func TestCatanFishingViewsAndBotIgnoreOthersSecrets(t *testing.T) {
	s := fishingProductionFixture(t, 3)
	for viewer := -1; viewer < 3; viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)["fishing"].(map[string]any)
		b, _ := json.Marshal(v)
		var public map[string]any
		_ = json.Unmarshal(b, &public)
		tokens := public["tokens"].(map[string]any)
		if tokens["drawPile"] != nil || tokens["hands"] != nil || public["pending"] != nil || public["lastRollId"] != nil || public["started"] != nil {
			t.Fatal("private fishing save exposed")
		}
		for p, raw := range tokens["players"].([]any) {
			_, faces := raw.(map[string]any)["tokens"]
			if faces != (p == viewer && len(s.Catan.Fishing.Tokens.Hands[p]) > 0) {
				t.Fatal("fish faces privacy")
			}
		}
		if public["canReplace"] != (viewer == 1) {
			t.Fatal("wrong actionable seat")
		}
	}
	bot, err := s.BotAction(1)
	if err != nil {
		t.Fatal(err)
	}
	other := clone(*s)
	f := &other.Catan.Fishing.Tokens
	slices.Reverse(f.DrawPile)
	f.Hands[2][0], f.DrawPile[0] = f.DrawPile[0], f.Hands[2][0]
	a, err := other.BotAction(1)
	if err != nil || !reflect.DeepEqual(a, bot) || !reflect.DeepEqual(s.View(1), other.View(1)) {
		t.Fatal("bot/view depends on others' faces or draw order", err)
	}
	s.AutoCatanPending()
	if s.CatanPendingActor() != 2 {
		t.Fatal("timeout consumed all responders")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_turn" {
		t.Fatal("timeout failed to continue")
	}
}

func TestCatanFishingSevenRobberAndRemoval(t *testing.T) {
	s := fishingGame(t, 3)
	g := s.Catan
	g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
	s.Turn, s.Phase = 0, "catan_roll"
	fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
	fishTop(&g.Fishing.Tokens, catanFishBoot)
	if err := g.Fishing.Tokens.beginDraw(1, []int{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	before := clone(g.Fishing.Tokens)
	if err := s.catanRollProduction(7); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_robber" || sum(g.DiscardDue) != 0 || !reflect.DeepEqual(before, g.Fishing.Tokens) {
		t.Fatal("fish counted as discard resources")
	}
	lake := g.Fishing.Map.Lakes[0].Tile
	if err := s.Apply(0, Action{Type: "catan_robber", Tile: lake}); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	if g.Robber != lake || s.Phase != "catan_turn" {
		t.Fatal("robber cannot enter lake")
	}
	if err := s.EliminateCatan(0); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	if len(g.Fishing.Tokens.Hands[0]) != 0 || len(g.Fishing.Tokens.Discard) != 7 {
		t.Fatal("departing fish not returned")
	}
	s.Turn, s.Phase = 1, "catan_turn"
	if err := s.EliminateCatan(1); err != nil {
		t.Fatal(err)
	}
	if g.Fishing.Tokens.BootOwner != -1 || !slices.Contains(g.Fishing.Tokens.DrawPile, catanFishBoot) {
		t.Fatal("boot stranded on absent player")
	}
	if err := g.Fishing.Tokens.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanFishingFullSetupAndAtomicInvalidState(t *testing.T) {
	for n := 3; n <= 6; n++ {
		s := fishingGame(t, n)
		for steps := 0; s.Catan.setup() && steps < 4*n+2; steps++ {
			a, err := s.BotAction(s.Turn)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Apply(s.Turn, a); err != nil {
				t.Fatal(err)
			}
			if err := s.Catan.validateFishing(); err != nil {
				t.Fatal(err)
			}
			copy := clone(*s)
			if !reflect.DeepEqual(copy, *s) {
				t.Fatal("setup restore")
			}
			s = &copy
		}
		if s.Catan.setup() || s.Phase != "catan_roll" {
			t.Fatal("setup did not finish")
		}
		for _, awarded := range s.Catan.Fishing.Started {
			if !awarded {
				t.Fatal("missing initial entitlement check")
			}
		}
		// Invalid fishing state must be rejected before JSON cloning or an
		// ordinary roll/build action changes resources, dice or the turn.
		s.Catan.Fishing.Map.Grounds[0].Number = 7
		fishGameReject(t, s, s.Turn, Action{Type: "catan_roll"})
	}
	if _, err := NewCatanFishing(3, CatanOptions{AllHelpers: true}); err == nil {
		t.Fatal("unverified helper combination opened")
	}
	s := fishingGame(t, 3)
	s.Catan.SetupStep = s.Catan.SetupLimit()
	s.Phase = "catan_roll"
	before, _ := json.Marshal(s)
	if err := s.catanBeginCardEvent("beautiful_day", 4, 0, 0); err == nil {
		t.Fatal("unverified event combination opened")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("unsupported combination mutated state")
	}
}
