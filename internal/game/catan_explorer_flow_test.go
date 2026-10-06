package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerStateRestore(t *testing.T, s *State) *State {
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
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, &next) {
		t.Fatal("whole explorer state differs after restart")
	}
	return &next
}

func TestCatanExplorerNaturalMatches(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := newCatanExplorerState(n)
			if err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_roll" || s.Turn != s.Catan.StartPlayer || s.Catan.TurnSerial != 1 || s.Catan.Explorer.Economy.GoldBank != 148-2*n {
				t.Fatal("must start with official initial state")
			}
			phases, actions := map[string]int{}, map[string]int{}
			steps := 0
			for ; steps < 2400 && !s.Finished; steps++ {
				actor := s.Turn
				if s.Phase == "catan_discard" {
					for p, due := range s.Catan.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				phases[s.Phase]++
				a, e := s.BotAction(actor)
				if e == nil {
					e = s.Apply(actor, a)
				}
				if e != nil {
					t.Fatalf("step%d round%d phase%s actor%d action%+v: %v", steps, s.Round, s.Phase, actor, a, e)
				}
				actions[a.Type]++
				if steps%23 == 0 {
					s = explorerStateRestore(t, s)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 8 || s.Catan.LongestOwner != -1 || s.Catan.ArmyOwner != -1 {
				t.Fatal("no proper 8-point finish", steps, s.Round, phases, actions)
			}
			if actions["catan_explorer_sail"] == 0 || actions["catan_explorer_settle"] == 0 || actions["catan_explorer_bank"] == 0 {
				t.Fatal("game must exercise ships, settlers and economy", actions)
			}
			s = explorerStateRestore(t, s)
			if err = s.Apply(s.Turn, Action{Type: "catan_end", Prompt: int(s.Catan.TurnSerial)}); err == nil {
				t.Fatal("cannot act after winning")
			}
			t.Logf("%d steps, round%d winner%d score%d actions%v", steps, s.Round, s.Winners[0], s.Catan.Players[s.Winners[0]].Score, actions)
		})
	}
}

func TestCatanExplorerStatePrivacyAndIllegalActions(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	actor := s.Turn
	for _, a := range []Action{{Type: "catan_roll"}, {Type: "catan_roll", Prompt: 2}, {Type: "catan_city", Prompt: 1}, {Type: "catan_buy_dev", Prompt: 1}, {Type: "catan_end", Prompt: 1}} {
		before, _ := json.Marshal(s)
		if err = s.Apply(actor, a); err == nil {
			t.Fatal("invalid initial action", a)
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("rejected state action mutated snapshot")
		}
	}
	for viewer := -1; viewer < 3; viewer++ {
		view := s.View(viewer)
		v := view["catan"].(map[string]any)
		x := v["explorer"].(map[string]any)
		data, _ := json.Marshal(x["board"])
		var board map[string]any
		_ = json.Unmarshal(data, &board)
		if board["hidden"] != nil || board["numbers"] != nil {
			t.Fatal("secret map leaked")
		}
		for p, raw := range v["players"].([]any) {
			public := raw.(map[string]any)
			if (public["resources"] != nil) != (p == viewer) {
				t.Fatal("hand leaked or missing", viewer, p)
			}
		}
		if v["victoryTarget"] != 8 || len(v["legal"].(map[string][]int)) != 0 {
			t.Fatal("wrong rules or base-game choices")
		}
		other := clone(*s)
		for i, h := range other.Catan.Explorer.Board.Hidden {
			for j, q := range other.Catan.Explorer.Board.Hidden {
				if i != j && h.Region == q.Region && h.Resource != q.Resource {
					other.Catan.Explorer.Board.Hidden[i].Resource, other.Catan.Explorer.Board.Hidden[j].Resource = q.Resource, h.Resource
					break
				}
			}
			break
		}
		before, _ := json.Marshal(view)
		after, _ := json.Marshal(other.View(viewer))
		if string(before) != string(after) {
			t.Fatal("hidden terrain changes public view")
		}
	}
	if err = s.Apply((actor+1)%3, Action{Type: "catan_roll", Prompt: 1}); err == nil {
		t.Fatal("wrong seat may not roll")
	}
	if err = s.Apply(actor, Action{Type: "catan_roll", Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Phase == "catan_discard" {
		t.Fatal("initial hand below discard threshold")
	}
	if err = s.Apply(actor, Action{Type: "catan_end", Prompt: 1}); err == nil {
		t.Fatal("cannot skip movement phase")
	}
	if err = s.Apply(actor, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(actor, Action{Type: "catan_end", Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Turn != (actor+1)%3 || s.Catan.TurnSerial != 2 || s.Phase != "catan_roll" || !slices.Equal(s.Catan.DiscardDue, []int{0, 0, 0}) {
		t.Fatal("next round state")
	}
}

func explorerFlowResources(s *State, hands map[int][]int) {
	g := s.Catan
	g.Bank = []int{19, 19, 19, 19, 19}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
		if hand, ok := hands[p]; ok {
			g.Players[p].Resources = slices.Clone(hand)
		}
		for r, n := range g.Players[p].Resources {
			g.Bank[r] -= n
		}
	}
}

func TestCatanExplorerGoldPlayerTradeConsentAndRevalidation(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.catanExplorerRoll([2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	actor, other := s.Turn, (s.Turn+1)%3
	explorerFlowResources(s, map[int][]int{actor: {2, 0, 0, 0, 0}, other: {0, 1, 0, 0, 0}})
	offer := Action{Type: "catan_trade_offer", Prompt: 1, Give: []int{2, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}, GoldGive: 1}
	if err = s.Apply(actor, offer); err != nil {
		t.Fatal(err)
	}
	id := s.Catan.Trade.ID
	complete := Action{Type: "catan_trade_complete", Prompt: 1, Offer: id, Target: other}
	before, _ := json.Marshal(s)
	if err = s.Apply(actor, complete); err == nil {
		t.Fatal("cannot transfer without acceptance")
	}
	after, _ := json.Marshal(s)
	if string(after) != string(before) {
		t.Fatal("unaccepted trade mutated state")
	}
	if err = s.Apply(other, Action{Type: "catan_trade_accept", Prompt: 1, Offer: id}); err != nil {
		t.Fatal(err)
	}
	s = explorerStateRestore(t, s)
	stale := clone(*s)
	stale.Catan.Players[other].Resources[1]--
	stale.Catan.Bank[1]++
	before, _ = json.Marshal(stale)
	if err = stale.Apply(actor, complete); err == nil {
		t.Fatal("must recheck current resources after acceptance")
	}
	after, _ = json.Marshal(stale)
	if string(after) != string(before) {
		t.Fatal("stale trade partially transferred")
	}
	gold := slices.Clone(s.Catan.Explorer.Economy.Gold)
	bank := slices.Clone(s.Catan.Bank)
	if err = s.Apply(actor, complete); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	if !slices.Equal(g.Players[actor].Resources, []int{0, 1, 0, 0, 0}) || !slices.Equal(g.Players[other].Resources, []int{2, 0, 0, 0, 0}) || g.Explorer.Economy.Gold[actor] != gold[actor]-1 || g.Explorer.Economy.Gold[other] != gold[other]+1 || !slices.Equal(g.Bank, bank) || g.Trade != nil {
		t.Fatal("resources/gold must move only between consenting players")
	}
	if err = s.Apply(actor, complete); err == nil {
		t.Fatal("trade cannot be replayed")
	}
}

func TestCatanExplorerSevenAutoResponsesAndImmediateVictory(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	a, b := s.Turn, (s.Turn+1)%3
	explorerFlowResources(s, map[int][]int{a: {8, 0, 0, 0, 0}, b: {0, 9, 0, 0, 0}})
	gold := slices.Clone(s.Catan.Explorer.Economy.Gold)
	if err = s.catanExplorerRoll([2]int{3, 4}); err != nil {
		t.Fatal(err)
	}
	s = explorerStateRestore(t, s)
	s.AutoCatanPending()
	if s.Phase != "catan_turn" || s.Turn != a || sum(s.Catan.DiscardDue) != 0 || sum(s.Catan.Players[a].Resources) != 4 || sum(s.Catan.Players[b].Resources) != 5 || !slices.Equal(gold, s.Catan.Explorer.Economy.Gold) {
		t.Fatal("parallel auto discard must resume same turn without pirate or gold bonus")
	}
	s = explorerStateRestore(t, s)
	// Explicit legal seven-point midgame fixture, not part of natural matches.
	g := s.Catan
	vertices := make([]int, len(g.Vertices))
	for i := range vertices {
		vertices[i] = i
	}
	slices.SortStableFunc(vertices, func(a, b int) int {
		if catanExplorerCoast(g, a) == catanExplorerCoast(g, b) {
			return 0
		}
		if catanExplorerCoast(g, a) {
			return -1
		}
		return 1
	})
	vertex := -1
	for _, id := range vertices {
		v := g.Vertices[id]
		if g.Players[a].Score >= 7 {
			break
		}
		if catanExplorerBotSite(g, a, v.ID, false) {
			g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level = a, 1
			g.Players[a].Score++
			if catanExplorerCoast(g, v.ID) {
				vertex = v.ID
			}
		}
	}
	if g.Players[a].Score != 7 || vertex < 0 {
		t.Fatal("could not place legal score fixture")
	}
	explorerFlowResources(s, map[int][]int{a: {0, 0, 0, 2, 2}})
	if err = s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(a, Action{Type: "catan_explorer_harbor", Prompt: 1, Vertex: vertex}); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || s.Phase != "finished" || !slices.Equal(s.Winners, []int{a}) || s.Catan.Players[a].Score != 8 || sum(s.Catan.Players[a].Resources) != 0 || s.Catan.Explorer.Cargo.Turn.Phase != "action" {
		t.Fatal("harbor upgrade must win immediately for exact 2 wheat/2 ore, before movement")
	}
	explorerStateRestore(t, s)
}

func TestCatanExplorerBotIgnoresHiddenTerrainAndOtherHands(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.catanExplorerRoll([2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	actor := s.Turn
	explorerFlowResources(s, map[int][]int{actor: {2, 2, 2, 2, 2}, (actor + 1) % 3: {3, 0, 0, 0, 0}, (actor + 2) % 3: {0, 3, 0, 0, 0}})
	first, err := s.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	next := clone(*s)
	other := (actor + 1) % 3
	next.Catan.Players[other].Resources = []int{0, 3, 0, 0, 0}
	next.Catan.Players[(actor+2)%3].Resources = []int{3, 0, 0, 0, 0}
	for region := 0; region < 2; region++ {
		slices.Reverse(next.Catan.Explorer.Board.Numbers[region])
	}
	for i, h := range next.Catan.Explorer.Board.Hidden {
		for j, q := range next.Catan.Explorer.Board.Hidden {
			if h.Region == q.Region && h.Resource != q.Resource {
				next.Catan.Explorer.Board.Hidden[i].Resource, next.Catan.Explorer.Board.Hidden[j].Resource = q.Resource, h.Resource
				break
			}
		}
		break
	}
	second, err := next.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("bot changed paid decision because of hidden information", first, second)
	}
	for _, state := range []*State{s, &next} {
		if err = state.Apply(actor, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
			t.Fatal(err)
		}
	}
	first, err = s.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	second, err = next.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("bot navigated using secret map or number order", first, second)
	}
}
