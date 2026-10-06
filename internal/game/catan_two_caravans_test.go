package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func twoCaravanFixture(t *testing.T) *State {
	t.Helper()
	s, e := NewCatanTwoCaravans(2, CatanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s.Turn, s.Catan.StartPlayer = 0, 0
	s.Catan.SetupStep = 4
	s.Phase = "catan_turn"
	s.Catan.Two.Rolls = []int{2, 12}
	return s
}
func twoCaravanCheck(t *testing.T, s *State) {
	t.Helper()
	if e := s.validateCatanTwo(); e != nil {
		t.Fatal(e)
	}
	caravanRestore(t, s)
	if e := s.validateCatanTwo(); e != nil {
		t.Fatal(e)
	}
}
func twoCaravanBid(t *testing.T, s *State, bids []int) {
	t.Helper()
	for p, n := range bids {
		catanGive(s.Catan, p, 2, n)
	}
	s.Catan.Caravans.Built = true
	helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	for i := 0; i < 2; i++ {
		p := s.CatanPendingActor()
		helperApply(t, s, p, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, bids[p], 0, 0}})
		twoCaravanCheck(t, s)
	}
}
func TestCatanTwoCaravansSetupAndOffboardRetreat(t *testing.T) {
	for i := 0; i < 24; i++ {
		s, e := NewCatanTwoCaravans(2, CatanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if s.Catan.Robber != -1 || len(s.Catan.Players) != 2 || s.Catan.victoryTarget() != 12 || s.Catan.Caravans.Map.Supply != 22 {
			t.Fatal("initial combination")
		}
		for s.Catan.setup() {
			a, e := s.BotAction(s.Turn)
			if e != nil {
				t.Fatal(e)
			}
			helperApply(t, s, s.Turn, a)
			twoCaravanCheck(t, s)
		}
		if s.Catan.Caravans.Built || len(s.Catan.Caravans.Wagons) != 0 || s.Catan.Caravans.Pending != nil || s.Catan.Robber != -1 {
			t.Fatal("setup triggered vote")
		}
		// First pre-roll token window: productive blocker moves off board, no theft.
		g := s.Catan
		g.Robber = 0
		p := s.Turn
		before := clone(*g)
		cost := g.twoTokenCost(p)
		if !slices.Equal(g.twoRetreatTiles(), []int{-1}) {
			t.Fatal("offboard target")
		}
		helperApply(t, s, p, Action{Type: "catan_two_robber", Tile: -1})
		if s.Catan.Robber != -1 || s.Catan.Two.Tokens[p] != before.Two.Tokens[p]-cost || len(s.Catan.twoRetreatTiles()) != 0 || !strings.Contains(s.Log[len(s.Log)-1], "棋盘外") {
			t.Fatal("retreat")
		}
		for seat := range 2 {
			if !slices.Equal(before.Players[seat].Resources, s.Catan.Players[seat].Resources) {
				t.Fatal("retreat stole resource")
			}
		}
		helperReject(t, s, p, Action{Type: "catan_two_robber", Tile: -1})
		twoCaravanCheck(t, s)
	}
	for _, n := range []int{1, 3, 4, 5, 6} {
		if _, e := NewCatanTwoCaravans(n, CatanOptions{}); e == nil {
			t.Fatal("bad count")
		}
	}
	if _, e := NewCatanTwoCaravans(2, CatanOptions{Helpers: true}); e == nil {
		t.Fatal("unverified combination")
	}
	if _, e := NewCatanCaravans(2, CatanOptions{}); e == nil {
		t.Fatal("ordinary constructor exposed two")
	}
}
func TestCatanTwoCaravansBidPlacementRestorePrivacy(t *testing.T) {
	for _, bids := range [][]int{{0, 0}, {2, 2}, {3, 1}, {1, 3}} {
		for active := 0; active < 2; active++ {
			t.Run(fmt.Sprint(bids, active), func(t *testing.T) {
				s := twoCaravanFixture(t)
				s.Turn = active
				twoCaravanBid(t, s, bids)
				leader := -1
				if bids[0] > bids[1] {
					leader = 0
				}
				if bids[1] > bids[0] {
					leader = 1
				}
				chooser := leader
				if chooser < 0 {
					chooser = active
				}
				if s.CatanPendingActor() != chooser || s.Phase != "catan_caravan_place" {
					t.Fatal("wrong first chooser")
				}
				first := s.Catan.Caravans.Map.Starts[0]
				helperReject(t, s, 1-chooser, Action{Type: "catan_caravan_place", Edge: first.Edge, Vertex: first.From})
				helperApply(t, s, chooser, Action{Type: "catan_caravan_place", Edge: first.Edge, Vertex: first.From})
				twoCaravanCheck(t, s)
				g, c := s.Catan, s.Catan.Caravans
				if len(c.Wagons) != 1 || c.Pending == nil || g.Bank[2] != 19-bids[0]-bids[1] || s.Turn != active {
					t.Fatal("first finished/paid back early")
				}
				next := chooser
				if leader < 0 {
					next = 1 - active
				}
				if s.CatanPendingActor() != next {
					t.Fatal("wrong second chooser")
				}
				for viewer := -1; viewer < 2; viewer++ {
					view := s.View(viewer)["catan"].(map[string]any)
					pub := view["caravans"].(map[string]any)
					if pub["canAct"] != (viewer == next) {
						t.Fatal("response permission")
					}
					for p, v := range view["players"].([]any) {
						if p != viewer && v.(map[string]any)["resources"] != nil {
							t.Fatal("resource leak")
						}
					}
				}
				var same catanCaravanWagon
				found := false
				for _, w := range c.choices(g) {
					if c.trainOrigins(g, w.From) == c.trainOrigins(g, first.From) {
						same = w
						found = true
						break
					}
				}
				if !found {
					t.Fatal("no continued train")
				}
				second := same
				if leader >= 0 {
					helperReject(t, s, next, Action{Type: "catan_caravan_place", Edge: same.Edge, Vertex: same.From})
					second = s.Catan.Caravans.responseChoices(s.Catan)[0]
				}
				helperApply(t, s, next, Action{Type: "catan_caravan_place", Edge: second.Edge, Vertex: second.From})
				if len(s.Catan.Caravans.Wagons) != 2 || s.Catan.Caravans.Pending != nil || s.Catan.Bank[2] != 19 || s.Turn != 1-active || s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 0 {
					t.Fatal("pair did not complete")
				}
				helperReject(t, s, next, Action{Type: "catan_caravan_place", Edge: second.Edge, Vertex: second.From})
				twoCaravanCheck(t, s)
			})
		}
	}
}
func TestCatanTwoCaravansRejectCorruptProgressAndUsePublicBotData(t *testing.T) {
	s := twoCaravanFixture(t)
	twoCaravanBid(t, s, []int{2, 1})
	a, e := s.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	helperApply(t, s, 0, a)
	for _, bad := range []func(*State){
		func(s *State) { s.Catan.Caravans.Pending.Two = nil },
		func(s *State) { s.Catan.Caravans.Pending.Two.Start++ },
		func(s *State) { s.Catan.Caravans.Pending.Two.First.Edge++ },
		func(s *State) { s.Catan.Caravans.Pending.Two.Train = 7 },
		func(s *State) { s.Catan.Caravans.Pending.Chooser = 1 },
		func(s *State) { s.Catan.Caravans.Pending.Kind = "vote"; s.Phase = "catan_caravan_vote" },
		func(s *State) { s.Catan.Caravans.Pending.Bids[0][2]++ },
	} {
		trial := clone(*s)
		bad(&trial)
		before, _ := json.Marshal(trial)
		if trial.Apply(0, Action{Type: "catan_caravan_place", Edge: 0, Vertex: 0}) == nil {
			t.Fatal("corrupt response accepted")
		}
		after, _ := json.Marshal(trial)
		if string(before) != string(after) {
			t.Fatal("bad response mutated")
		}
	}
	a, e = s.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	alt := clone(*s)
	catanGive(alt.Catan, 1, 0, 3)
	alt.Catan.Players[1].Dev = []int{1, 2, 3, 4, 1}
	slices.Reverse(alt.Catan.DevDeck)
	b, e := alt.BotAction(0)
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("bot read opponent secrets", a, b, e)
	}
	helperApply(t, s, 0, a)
	twoCaravanCheck(t, s)
}

// Twenty-two initial wagons and complete pairs make an odd round-start
// impossible. The sole remaining wagon is legal only as the pending second.
func TestCatanTwoCaravansFinalPairAndMissingSecondSave(t *testing.T) {
	s := twoCaravanFixture(t)
	rng := rand.New(rand.NewSource(728))
	var first catanCaravanWagon
	found := false
	for attempt := 0; attempt < 100 && !found; attempt++ {
		c := s.Catan.Caravans
		c.Wagons = nil
		for len(c.Wagons) < 20 {
			choices := c.choices(s.Catan)
			if len(choices) == 0 {
				break
			}
			if e := c.place(s.Catan, choices[rng.Intn(len(choices))]); e != nil {
				t.Fatal(e)
			}
		}
		if len(c.Wagons) != 20 {
			continue
		}
		for _, w := range c.choices(s.Catan) {
			trial := clone(*c)
			if e := trial.place(s.Catan, w); e != nil {
				t.Fatal(e)
			}
			if len(trial.choices(s.Catan)) > 0 {
				first = w
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("failed final pair fixture")
	}
	twoCaravanBid(t, s, []int{0, 0})
	helperApply(t, s, 0, Action{Type: "catan_caravan_place", Edge: first.Edge, Vertex: first.From})
	twoCaravanCheck(t, s)
	if len(s.Catan.Caravans.Wagons) != 21 || s.Catan.Caravans.Pending.Two.First == nil {
		t.Fatal("missing lone pending wagon")
	}
	corrupt := clone(*s)
	corrupt.Catan.Caravans.Pending = nil
	corrupt.Phase = "catan_turn"
	before, _ := json.Marshal(corrupt)
	if e := corrupt.Apply(0, Action{Type: "catan_end"}); e == nil || !strings.Contains(e.Error(), "第二辆") {
		t.Fatal("odd saved game accepted", e)
	}
	after, _ := json.Marshal(corrupt)
	if string(before) != string(after) {
		t.Fatal("invalid save mutated")
	}
	w := s.Catan.Caravans.responseChoices(s.Catan)[0]
	helperApply(t, s, 1, Action{Type: "catan_caravan_place", Edge: w.Edge, Vertex: w.From})
	twoCaravanCheck(t, s)
	if len(s.Catan.Caravans.Wagons) != 22 || s.Catan.Caravans.Pending != nil {
		t.Fatal("last pair failed")
	}
	sequence := s.Catan.Caravans.Sequence
	s.Phase = "catan_turn"
	s.Catan.Two.Rolls = []int{2, 12}
	s.Catan.Caravans.Built = true
	helperApply(t, s, 1, Action{Type: "catan_end"})
	if s.Catan.Caravans.Pending != nil || s.Catan.Caravans.Sequence != sequence || s.Turn != 0 {
		t.Fatal("empty supply collected another bid")
	}
	twoCaravanCheck(t, s)
}

func TestCatanTwoCaravansFirstWagonVictory(t *testing.T) {
	for _, owner := range []int{0, 1} {
		s := twoCaravanFixture(t)
		g, c := s.Catan, s.Catan.Caravans
		first := c.Map.Starts[0]
		for _, w := range c.Map.Starts {
			e := g.Edges[w.Edge]
			v := e.A
			if v == w.From {
				v = e.B
			}
			if g.canSettlement(owner, v, true) {
				first = w
				break
			}
		}
		if e := c.place(g, first); e != nil {
			t.Fatal(e)
		}
		for _, other := range c.Map.Starts {
			if other != first {
				if e := c.place(g, other); e != nil {
					t.Fatal(e)
				}
				break
			}
		}
		edge := g.Edges[first.Edge]
		target := edge.A
		if target == first.From {
			target = edge.B
		}
		var second catanCaravanWagon
		for _, w := range c.choices(g) {
			if w.From == target {
				second = w
				break
			}
		}
		if g.Vertices[target].Owner != -1 {
			t.Fatal("score fixture target occupied")
		}
		g.Vertices[target].Owner, g.Vertices[target].Level = owner, 1
		count := 1
		for i := range g.Vertices {
			if count < 7 && g.canSettlement(owner, i, true) {
				g.Vertices[i].Owner = owner
				g.Vertices[i].Level = 1
				if count <= 4 {
					g.Vertices[i].Level = 2
				}
				count++
			}
		}
		s.catanScores()
		if count != 7 || g.Players[owner].Score != 11 {
			t.Fatal("score fixture", count, g.Players[owner].Score)
		}
		twoCaravanBid(t, s, []int{1, 0})
		helperApply(t, s, 0, Action{Type: "catan_caravan_place", Edge: second.Edge, Vertex: second.From})
		if s.Catan.Players[owner].Score != 12 {
			t.Fatal("missing caravan bonus")
		}
		if owner == 0 {
			if !s.Finished || len(s.Catan.Caravans.Wagons) != 3 || s.Catan.Caravans.Pending != nil || s.Catan.Bank[2] != 19 {
				t.Fatal("victory waited for extra wagon/kept escrow")
			}
		} else {
			if s.Finished || s.Catan.Caravans.Pending == nil {
				t.Fatal("non-active player prematurely won")
			}
			w := s.Catan.Caravans.responseChoices(s.Catan)[0]
			helperApply(t, s, 0, Action{Type: "catan_caravan_place", Edge: w.Edge, Vertex: w.From})
			if !s.Finished || !slices.Equal(s.Winners, []int{1}) {
				t.Fatal("next own turn failed victory")
			}
		}
		twoCaravanCheck(t, s)
	}
}
func TestCatanTwoCaravansDetectMergedOriginsWithoutGuessing(t *testing.T) {
	s := twoCaravanFixture(t)
	c, g := s.Catan.Caravans, s.Catan
	rng := rand.New(rand.NewSource(3357))
	var merged catanCaravanWagon
	found := false
	for attempt := 0; attempt < 300 && !found; attempt++ {
		c.Wagons = nil
		for len(c.Wagons) < 20 {
			choices := c.choices(g)
			if len(choices) == 0 {
				break
			}
			for _, w := range choices {
				if mask := c.trainOrigins(g, w.From); len(c.Wagons)%2 == 0 && mask > 0 && !singleCaravanOrigin(mask) {
					merged = w
					found = true
					break
				}
			}
			if found {
				break
			}
			if e := c.place(g, choices[rng.Intn(len(choices))]); e != nil {
				t.Fatal(e)
			}
		}
	}
	if !found {
		t.Fatal("no converged network")
	}
	twoCaravanBid(t, s, []int{1, 0})
	before, _ := json.Marshal(s)
	e := s.Apply(0, Action{Type: "catan_caravan_place", Edge: merged.Edge, Vertex: merged.From})
	if e == nil || !strings.Contains(e.Error(), "会合") {
		t.Fatal("guessed merged-train rule", e)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("merged guard mutated state")
	}
	twoCaravanCheck(t, s)
}

func TestCatanTwoCaravansRuleVersionCompatibility(t *testing.T) {
	for _, two := range []bool{false, true} {
		s := caravanFixture(t, 3)
		if two {
			s = twoCaravanFixture(t)
		}
		if s.Catan.Caravans.Rules != CatanCaravansRules {
			t.Fatal("constructor omitted rules")
		}
		s.Catan.Caravans.Rules = "unsupported"
		helperReject(t, s, s.Turn, Action{Type: "catan_end"})
		// Earlier internal 2025 snapshots omitted the field; map/network validation
		// still applies instead of reinterpreting them as a different edition.
		s.Catan.Caravans.Rules = ""
		raw, _ := json.Marshal(s)
		var restored State
		if e := json.Unmarshal(raw, &restored); e != nil {
			t.Fatal(e)
		}
		helperApply(t, &restored, restored.Turn, Action{Type: "catan_end"})
		if two {
			twoCaravanCheck(t, &restored)
		} else {
			caravanConserved(t, &restored)
		}
		restored.Catan.Caravans.Map.Supply++
		helperReject(t, &restored, restored.Turn, Action{Type: "catan_roll"})
	}
}
