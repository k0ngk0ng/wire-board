package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func twoCoreFixture(t *testing.T) *State {
	t.Helper()
	s, err := newCatanTwoCore()
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, s.Turn, a)
	}
	return s
}

func twoCoreRestore(t *testing.T, s *State) {
	t.Helper()
	b, _ := json.Marshal(s)
	var next State
	if err := json.Unmarshal(b, &next); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(next)
	if string(b) != string(a) {
		t.Fatal("two-player restore changed state")
	}
	if err := next.validateCatanTwo(); err != nil {
		t.Fatal(err)
	}
	*s = next
}

// Deterministic dice fixtures still use the production resolver and the same
// atomic post-action transition as Apply. No forced-dice client field exists.
func twoCoreRoll(t *testing.T, s *State, a, b int) error {
	t.Helper()
	next := clone(*s)
	if err := next.catanTwoRoll(a, b); err != nil {
		return err
	}
	if err := next.catanTwoAfterAction(s, Action{Type: "catan_roll"}); err != nil {
		return err
	}
	if err := next.validateCatanTwo(); err != nil {
		return err
	}
	*s = next
	return nil
}

func twoCoreActionPhase(t *testing.T, s *State) {
	t.Helper()
	if err := twoCoreRoll(t, s, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := twoCoreRoll(t, s, 6, 6); err != nil {
		t.Fatal(err)
	}
}

func TestCatanTwoCoreDoubleProductionAndSevenResume(t *testing.T) {
	for _, firstSeven := range []bool{true, false} {
		t.Run(fmt.Sprint(firstSeven), func(t *testing.T) {
			s := twoCoreFixture(t)
			p := s.Turn
			if !firstSeven {
				if err := twoCoreRoll(t, s, 1, 1); err != nil {
					t.Fatal(err)
				}
			}
			for i := range 2 {
				catanGive(s.Catan, i, 0, 8-s.Catan.Players[i].Resources[0])
			}
			if err := twoCoreRoll(t, s, 3, 4); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_discard" {
				t.Fatal("seven skipped discard")
			}
			helperReject(t, s, p, Action{Type: "catan_roll"})
			twoCoreRestore(t, s)
			for i := range 2 {
				due := s.Catan.DiscardDue[i]
				give := make([]int, 5)
				left := due
				for c, n := range s.Catan.Players[i].Resources {
					give[c] = min(left, n)
					left -= give[c]
				}
				helperApply(t, s, i, Action{Type: "catan_discard", Tokens: give})
				twoCoreRestore(t, s)
			}
			if s.Phase != "catan_robber" {
				t.Fatal("seven skipped robber")
			}
			// Choose an opponent's producing tile to exercise actual random theft.
			tile := -1
			for _, candidate := range s.Catan.Tiles {
				for _, id := range candidate.Vertices {
					if s.Catan.Vertices[id].Owner == 1-p && s.Catan.robberAllowed(candidate.ID) {
						tile = candidate.ID
						break
					}
				}
				if tile >= 0 {
					break
				}
			}
			if tile < 0 {
				t.Fatal("robber fixture")
			}
			before := sum(s.Catan.Players[1-p].Resources)
			helperApply(t, s, p, Action{Type: "catan_robber", Tile: tile})
			if sum(s.Catan.Players[1-p].Resources) != before-1 {
				t.Fatal("missing theft")
			}
			want := "catan_turn"
			if firstSeven {
				want = "catan_roll"
			}
			if s.Phase != want {
				t.Fatal("wrong production continuation", s.Phase, want)
			}
			twoCoreRestore(t, s)
			if firstSeven {
				old, _ := json.Marshal(s)
				if twoCoreRoll(t, s, 4, 3) == nil {
					t.Fatal("same second total accepted")
				}
				b, _ := json.Marshal(s)
				if string(old) != string(b) {
					t.Fatal("rejected repeated roll mutated state")
				}
				if err := twoCoreRoll(t, s, 6, 6); err != nil {
					t.Fatal(err)
				}
			}
			if s.Phase != "catan_turn" || len(s.Catan.Two.Rolls) != 2 || s.Catan.RollID != 2 {
				t.Fatal("two completed production phases")
			}
			helperReject(t, s, p, Action{Type: "catan_roll"})
			helperApply(t, s, p, Action{Type: "catan_end"})
			if s.Turn != 1-p || s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 0 {
				t.Fatal("next player inherited production")
			}
		})
	}
}

func TestCatanTwoCoreActualDiceNeverRepeatTotal(t *testing.T) {
	s := twoCoreFixture(t)
	for turn := 0; turn < 40; turn++ {
		p := s.Turn
		for step := 0; step < 12 && s.Phase != "catan_turn"; step++ {
			actor := p
			if s.Phase == "catan_discard" {
				for i, due := range s.Catan.DiscardDue {
					if due > 0 {
						actor = i
						break
					}
				}
			}
			a, err := s.BotAction(actor)
			if err != nil {
				t.Fatal(err)
			}
			helperApply(t, s, actor, a)
		}
		rolls := s.Catan.Two.Rolls
		if s.Phase != "catan_turn" || len(rolls) != 2 || rolls[0] == rolls[1] {
			t.Fatal("actual production loop", s.Phase, rolls)
		}
		twoCoreRestore(t, s)
		helperApply(t, s, p, Action{Type: "catan_end"})
	}
}

func TestCatanTwoCoreCompulsoryRoadsAndFreeRoads(t *testing.T) {
	for _, preRoll := range []bool{true, false} {
		t.Run(fmt.Sprint(preRoll), func(t *testing.T) {
			s := twoCoreFixture(t)
			p := s.Turn
			if !preRoll {
				twoCoreActionPhase(t, s)
			}
			resume := s.Phase
			catanCard(s.Catan, p, 1)
			helperApply(t, s, p, Action{Type: "catan_dev", Card: 1})
			built := 0
			for step := 0; step < 2; step++ {
				if s.Phase != "catan_roads" {
					break
				}
				edge := -1
				for _, e := range s.Catan.Edges {
					if s.Catan.canRoad(p, e.ID) {
						edge = e.ID
						break
					}
				}
				if edge < 0 {
					t.Fatal("free road fixture")
				}
				bank := slices.Clone(s.Catan.Bank)
				helperApply(t, s, p, Action{Type: "catan_road", Edge: edge})
				if s.Phase != "catan_two_build" || s.CatanPendingActor() != p || s.Catan.Two.Sequence != step+1 {
					t.Fatal("missing mandatory neutral response")
				}
				if !slices.Equal(bank, s.Catan.Bank) {
					t.Fatal("free road paid resources")
				}
				helperReject(t, s, 1-p, Action{Type: "catan_two_build", Target: 0, Vertex: -1, Edge: 0})
				helperReject(t, s, p, Action{Type: "catan_end"})
				helperReject(t, s, p, Action{Type: "catan_roll"})
				if err := s.EliminateCatan(p); err == nil {
					t.Fatal("removed mandatory responder")
				}
				twoCoreRestore(t, s)
				s.AutoCatanPending()
				if s.Catan.Two.Pending != nil {
					t.Fatal("automatic neutral choice stalled")
				}
				built++
				twoCoreRestore(t, s)
			}
			if built != 2 || s.Phase != resume || s.Catan.FreeRoads != 0 {
				t.Fatal("free road continuation", built, s.Phase)
			}
			neutral := 0
			for _, e := range s.Catan.Edges {
				if e.Owner < -1 {
					neutral++
				}
			}
			if neutral != 2 {
				t.Fatal("neutral roads duplicated", neutral)
			}
		})
	}
}

func TestCatanTwoCorePaidBuildsAndPrivateChoices(t *testing.T) {
	s := twoCoreFixture(t)
	twoCoreActionPhase(t, s)
	p := s.Turn
	// Natural existing roads; grant only the explicit cost for this action fixture.
	for attempt := 0; attempt < 10; attempt++ {
		vertex := -1
		for _, v := range s.Catan.Vertices {
			if s.Catan.canSettlement(p, v.ID, false) {
				vertex = v.ID
				break
			}
		}
		a := Action{Type: "catan_settlement", Vertex: vertex}
		if vertex < 0 {
			a.Type = "catan_road"
			for _, e := range s.Catan.Edges {
				if s.Catan.canRoad(p, e.ID) {
					a.Edge = e.ID
					break
				}
			}
		}
		for c, n := range catanPrices[a.Type] {
			catanGive(s.Catan, p, c, max(0, n-s.Catan.Players[p].Resources[c]))
		}
		before := slices.Clone(s.Catan.Players[p].Resources)
		tokens := slices.Clone(s.Catan.Two.Tokens)
		reward := 0
		if a.Type == "catan_settlement" {
			reward = s.Catan.twoSettlementTokens(p, a.Vertex)
		}
		helperApply(t, s, p, a)
		if s.Catan.Two.Tokens[p] != tokens[p]+reward || s.Catan.Two.Tokens[1-p] != tokens[1-p] {
			t.Fatal("paid construction token reward")
		}
		if s.Catan.Two.Pending == nil {
			t.Fatal("paid construction did not require neutral")
		}
		for c, n := range catanPrices[a.Type] {
			if s.Catan.Players[p].Resources[c] != before[c]-n {
				t.Fatal("construction cost")
			}
		}
		for viewer := -1; viewer < 2; viewer++ {
			v := s.View(viewer)["catan"].(map[string]any)
			q := v["two"].(map[string]any)
			if q["canAct"] != (viewer == p) || q["actor"] != p {
				t.Fatal("neutral responder permission")
			}
			for other, raw := range v["players"].([]any) {
				seat := raw.(map[string]any)
				if other != viewer && (seat["resources"] != nil || seat["dev"] != nil) {
					t.Fatal("private hand leak")
				}
			}
		}
		bot, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		changed := clone(*s)
		changed.Catan.Players[1-p].Resources = []int{2, 0, 4, 1, 3}
		slices.Reverse(changed.Catan.DevDeck)
		other, err := changed.BotAction(p)
		if err != nil || !reflect.DeepEqual(bot, other) {
			t.Fatal("neutral bot used rival hand/deck")
		}
		helperApply(t, s, p, bot)
		if s.Catan.Two.Tokens[p] != tokens[p]+reward || s.Catan.Two.Tokens[1-p] != tokens[1-p] {
			t.Fatal("neutral build awarded tokens")
		}
		helperReject(t, s, p, bot)
		twoCoreRestore(t, s)
		if vertex >= 0 {
			// Upgrade does not build another neutral village or road.
			sequence := s.Catan.Two.Sequence
			for c, n := range catanPrices["catan_city"] {
				catanGive(s.Catan, p, c, max(0, n-s.Catan.Players[p].Resources[c]))
			}
			helperApply(t, s, p, Action{Type: "catan_city", Vertex: vertex})
			if s.Catan.Two.Tokens[p] != tokens[p]+reward {
				t.Fatal("city awarded settlement tokens again")
			}
			if s.Catan.Two.Pending != nil || s.Catan.Two.Sequence != sequence {
				t.Fatal("city triggered neutral build")
			}
			return
		}
	}
	t.Fatal("did not reach real settlement")
}

func TestCatanTwoCoreCorruptionRejectedAtomically(t *testing.T) {
	for _, bad := range []func(*State){
		func(s *State) { s.Catan.Two.Rolls = []int{6, 6} },
		func(s *State) { s.Catan.Two.Rolls = []int{1} },
		func(s *State) { s.Catan.Bank[0]++ },
		func(s *State) { s.Catan.Vertices[0].Owner = 7 },
		func(s *State) { s.Catan.Two.Pending = &CatanTwoPending{Kind: "city", Resume: "catan_turn"} },
		func(s *State) { s.Catan.Options.Helpers = true },
	} {
		s := twoCoreFixture(t)
		bad(s)
		before, _ := json.Marshal(s)
		if s.Apply(s.Turn, Action{Type: "catan_roll"}) == nil {
			t.Fatal("corrupt two-player state accepted")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("corrupt action changed state")
		}
	}
}

func TestCatanTwoCoreNeutralAwardUsesNeutralName(t *testing.T) {
	s := twoCoreFixture(t)
	g := s.Catan
	// Connected neutral construction reaches a genuine five-segment route.
	for step := 0; step < 15 && g.roadLength(-2) < 5; step++ {
		choices := g.twoNeutralChoices("road")
		found := false
		for _, choice := range choices {
			if choice.Owner == -2 {
				if err := g.placeTwoNeutral("road", choice); err != nil {
					t.Fatal(err)
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatal("neutral route fixture")
		}
	}
	s.catanScores()
	if g.LongestOwner != -2 || g.Players[0].Score != 2 || g.Players[1].Score != 2 || !strings.Contains(strings.Join(s.Log, "\n"), "中立势力 1获得最长路线") {
		t.Fatal("neutral award counted as a player or unclaimed", g.LongestOwner)
	}
}

func TestCatanTwoCoreNeutralTakesRealLongestPoints(t *testing.T) {
	s := twoNeutralFixture(t)
	g := s.Catan
	g.Two = &CatanTwo{Rules: CatanTwoRules, Tokens: []int{5, 5}, Bank: 10, Rolls: []int{2, 12}, Sequence: 1, Pending: &CatanTwoPending{Kind: "road", Resume: "catan_turn"}}
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_two_build"
	v := g.Tiles[0].Vertices[4]
	g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
	neutral, real := twoSixEdgePath(t, g, -2), twoSixEdgePath(t, g, 0)
	g.Edges[neutral[5]].Owner = -1
	g.Edges[real[5]].Owner = -1
	g.LongestOwner = 0
	s.catanScores()
	if g.Players[0].Score != 3 || g.LongestOwner != 0 {
		t.Fatal("real longest route fixture")
	}
	twoCoreRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_two_build", Target: 0, Edge: neutral[5], Vertex: -1})
	if s.Catan.LongestOwner != -2 || s.Catan.Players[0].Score != 1 || s.Phase != "catan_turn" || s.Finished {
		t.Fatal("neutral did not remove real player's two points")
	}
}

func TestCatanTwoCoreRejectsImpossibleResponseResume(t *testing.T) {
	for _, kind := range []string{"road", "settlement"} {
		s := twoCoreFixture(t)
		twoCoreActionPhase(t, s)
		s.Phase = "catan_two_build"
		s.Catan.Two.Sequence = 1
		s.Catan.Two.Pending = &CatanTwoPending{Kind: kind, Resume: "catan_roll"}
		before, _ := json.Marshal(s)
		if s.Apply(s.Turn, Action{Type: "catan_two_build", Target: 0, Vertex: -1, Edge: 0}) == nil {
			t.Fatal("response permitted a third production")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("bad continuation mutated state")
		}
	}
}
