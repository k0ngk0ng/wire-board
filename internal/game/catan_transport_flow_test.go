package game

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

func transportState(t *testing.T, setup bool) *State {
	t.Helper()
	s, err := newCatanTransportState(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if setup {
		for i := 0; s.Catan.setup() && i < 30; i++ {
			a, e := s.BotAction(s.Turn)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Apply(s.Turn, a); e != nil {
				t.Fatal(a, e)
			}
		}
	}
	return s
}
func transportRestoreState(t *testing.T, s *State) *State {
	t.Helper()
	data, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var next State
	if e = json.Unmarshal(data, &next); e != nil {
		t.Fatal(e)
	}
	if e = next.validateCatanTransport(); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s, &next) {
		t.Fatal("restore differs")
	}
	return &next
}
func TestCatanTransportNaturalMatches(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, e := newCatanTransportState(n, CatanOptions{})
			if e != nil {
				t.Fatal(e)
			}
			phases := map[string]int{}
			steps := 0
			for ; steps < 4000 && !s.Finished; steps++ {
				actor := s.CatanPendingActor()
				if actor < 0 {
					actor = s.Turn
				}
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
				if e = s.validateCatanTransport(); e != nil {
					t.Fatal(steps, e)
				}
				g := s.Catan
				for p, pl := range g.Players {
					w := g.Transport.Wagons[p]
					score := len(w.Delivered) + pl.Dev[4]
					if w.Level == 4 {
						score++
					}
					if g.ArmyOwner == p {
						score += 2
					}
					for _, v := range g.Vertices {
						if v.Owner == p {
							score += v.Level
						}
					}
					if pl.Score != score {
						t.Fatal("score drift", steps, p, score, pl.Score)
					}
				}
				if steps%31 == 0 {
					s = transportRestoreState(t, s)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 13 {
				t.Fatal("no points finish", steps, s.Round, phases)
			}
			t.Logf("%d steps, round%d winner%d score%d phases%v", steps, s.Round, s.Winners[0], s.Catan.Players[s.Winners[0]].Score, phases)
		})
	}
}
func TestCatanTransportSetupProductionAndDiscard(t *testing.T) {
	s := transportState(t, true)
	g := s.Catan
	for p, w := range g.Transport.Wagons {
		roads, villages, cities := g.pieces(p)
		if roads != 2 || villages != 1 || cities != 1 || g.Vertices[w.Position].Owner != p || g.Vertices[w.Position].Level != 2 {
			t.Fatal("official starting city/wagon", p, w)
		}
	}
	if g.LongestOwner != -1 || g.Robber != -1 || g.victoryTarget() != 13 {
		t.Fatal("scenario awards")
	}
	index := 0
	rolls := [][2]int{{1, 1}, {6, 6}, {2, 3}}
	if e := s.catanTransportRoll(func() [2]int { r := rolls[index]; index++; return r }); e != nil {
		t.Fatal(e)
	}
	if index != 3 || g.RollID != 1 || !slices.Equal(g.Dice, []int{2, 3}) || s.Phase != "catan_turn" {
		t.Fatal("reroll must not create extra production")
	}
	// Separate valid seven fixture: resources are transferred, not created.
	s = transportState(t, true)
	g = s.Catan
	for p := range g.Players {
		catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
	}
	transportGive(g, 1, []int{8, 0, 0, 0, 0})
	if e := s.catanTransportRoll(func() [2]int { return [2]int{3, 4} }); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_discard" || g.DiscardDue[1] != 4 {
		t.Fatal("seven discard")
	}
	if e := s.Apply(1, Action{Type: "catan_discard", Tokens: []int{4, 0, 0, 0, 0}}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_transport_barbarian" || s.CatanPendingActor() != s.Turn {
		t.Fatal("seven must move barbarian")
	}
	s = transportRestoreState(t, s)
	a, e := s.BotAction(s.Turn)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(s.Turn, a); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_turn" {
		t.Fatal("seven did not resume")
	}
}
func transportGiveDev(t *testing.T, s *State, p, kind int) {
	t.Helper()
	g := s.Catan
	i := slices.Index(g.DevDeck, kind)
	if i < 0 {
		t.Fatal("missing card")
	}
	g.DevDeck = append(g.DevDeck[:i], g.DevDeck[i+1:]...)
	g.Players[p].Dev[kind]++
}
func TestCatanTransportKnightAndSwift(t *testing.T) {
	s := transportState(t, true)
	p := s.Turn
	transportGiveDev(t, s, p, 0)
	if e := s.Apply(p, Action{Type: "catan_dev", Card: 0}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_transport_barbarian" || s.Catan.Players[p].Knights != 1 {
		t.Fatal("knight effect")
	}
	g := s.Catan
	other := (p + 1) % 3
	edge := -1
	for _, e := range g.Edges {
		if e.Owner == other && !slices.Contains(g.Transport.Barbarians[:], e.ID) {
			edge = e.ID
			break
		}
	}
	catanMove(g.Players[other].Resources, g.Bank, slices.Clone(g.Players[other].Resources))
	transportGive(g, other, []int{0, 1, 0, 0, 0})
	gold := g.Transport.Gold[other]
	own := slices.Clone(g.Players[p].Resources)
	seq := int(g.Transport.BarbarianSequence)
	before, _ := json.Marshal(s)
	if e := s.Apply(other, Action{Type: "catan_transport_barbarian", Offer: seq, Card: 0, Edge: edge}); e == nil {
		t.Fatal("other responder")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("failed response mutated game")
	}
	if e := s.Apply(p, Action{Type: "catan_transport_barbarian", Offer: seq, Card: 0, Edge: edge}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_roll" || s.Catan.Players[p].Resources[1] != own[1]+1 || s.Catan.Transport.Gold[other] != gold {
		t.Fatal("knight must steal resource, preserve gold and resume preroll")
	}
	s = transportState(t, true)
	p = s.Turn
	transportGiveDev(t, s, p, 2)
	if e := s.Apply(p, Action{Type: "catan_dev", Card: 2}); e != nil {
		t.Fatal(e)
	}
	if e := s.catanTransportRoll(func() [2]int { return [2]int{2, 3} }); e != nil {
		t.Fatal(e)
	}
	transportGive(s.Catan, p, []int{0, 0, 0, 1, 0})
	if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	first := s.Catan.Transport.Sequence
	if e := s.Apply(p, Action{Type: "catan_transport_wheat", Offer: int(first)}); e != nil {
		t.Fatal(e)
	}
	if e := s.Apply(p, Action{Type: "catan_transport_stop", Offer: int(first)}); e != nil {
		t.Fatal(e)
	}
	tr := s.Catan.Transport
	if s.Turn != p || tr.Moves != 2 || tr.Sequence != first+1 || tr.Travel.Points != 4 || !tr.Travel.WheatUsed {
		t.Fatal("swift reset per-turn restrictions or added leftover MP")
	}
	before, _ = json.Marshal(s)
	if e := s.Apply(p, Action{Type: "catan_transport_wheat", Offer: int(tr.Sequence)}); e == nil {
		t.Fatal("second grain during same turn")
	}
	after, _ = json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("second wheat mutated state")
	}
	if e := s.Apply(p, Action{Type: "catan_transport_stop", Offer: int(tr.Sequence)}); e != nil {
		t.Fatal(e)
	}
	if s.Turn == p || s.Phase != "catan_roll" || s.Catan.Transport.Swift || s.Catan.Transport.Travel != nil {
		t.Fatal("next player after second move")
	}
}

func TestCatanTransportVictoryStopsBeforeLoadingAndSwift(t *testing.T) {
	s := transportState(t, true)
	g := s.Catan
	tpt := g.Transport
	p := s.Turn
	s.Phase = "catan_turn"
	// Explicit near-win fixture, conserving physical commodity and dev cards.
	for i := 0; i < 9; i++ {
		id := tpt.Stacks[0][0]
		tpt.Stacks[0] = tpt.Stacks[0][1:]
		tpt.Wagons[p].Delivered = append(tpt.Wagons[p].Delivered, id)
	}
	found := false
	for stack, ids := range tpt.Stacks {
		for i, id := range ids {
			token, _ := tpt.token(id)
			if tpt.Map.accepts(0, token.Cargo) {
				tpt.Wagons[p].Cargo = id
				tpt.Stacks[stack] = append(ids[:i], ids[i+1:]...)
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("cargo fixture")
	}
	path := tpt.Map.Sites[0].Paths[0]
	tpt.Wagons[p].Position = g.Edges[path].A
	s.catanScores()
	if g.Players[p].Score != 12 {
		t.Fatal("fixture not twelve points", g.Players[p].Score)
	}
	transportGiveDev(t, s, p, 2)
	if e := s.Apply(p, Action{Type: "catan_dev", Card: 2}); e != nil {
		t.Fatal(e)
	}
	if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	seq := int(s.Catan.Transport.Sequence)
	if e := s.Apply(p, Action{Type: "catan_transport_step", Offer: seq, Edge: path}); e != nil {
		t.Fatal(e)
	}
	before := len(s.Catan.Transport.Stacks[0])
	s = transportRestoreState(t, s)
	if e := s.Apply(p, Action{Type: "catan_transport_arrival", Offer: seq, Choice: "deliver"}); e != nil {
		t.Fatal(e)
	}
	if !s.Finished || s.Catan.Players[p].Score != 13 || !slices.Equal(s.Winners, []int{p}) || len(s.Catan.Transport.Stacks[0]) != before || s.Catan.Transport.Wagons[p].Cargo != 0 || s.Catan.Transport.Moves != 1 {
		t.Fatal("winning delivery must stop before loading or second travel")
	}
	transportRestoreState(t, s)
}
func TestCatanTransportPublicViewTimeoutAndDeparture(t *testing.T) {
	s := transportState(t, true)
	for _, p := range []int{-1, 0, 1, 2} {
		before, _ := json.Marshal(s.View(p))
		s.Catan.Transport.Stacks[0][0], s.Catan.Transport.Stacks[0][1] = s.Catan.Transport.Stacks[0][1], s.Catan.Transport.Stacks[0][0]
		after, _ := json.Marshal(s.View(p))
		if string(before) != string(after) {
			t.Fatal("view leaks hidden commodity order", p)
		}
		var view map[string]any
		json.Unmarshal(after, &view)
		g := view["catan"].(map[string]any)
		tr := g["transport"].(map[string]any)
		if _, ok := tr["stacks"]; ok {
			t.Fatal("raw transport state escaped")
		}
		if _, ok := g["devDeck"]; ok {
			t.Fatal("dev deck escaped")
		}
		if p < 0 {
			for _, raw := range g["players"].([]any) {
				if _, ok := raw.(map[string]any)["dev"]; ok {
					t.Fatal("observer sees dev identities")
				}
			}
		}
	}
	p := s.Turn
	if e := s.catanTransportRoll(func() [2]int { return [2]int{3, 4} }); e != nil {
		t.Fatal(e)
	}
	if s.Phase == "catan_discard" {
		s.AutoCatanPending()
	}
	if s.Phase != "catan_transport_barbarian" {
		t.Fatal("seven fixture")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_turn" {
		t.Fatal("barbarian timeout did not resume")
	}
	if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	if e := s.EliminateCatan(p); e == nil {
		t.Fatal("cannot kick during mandatory movement response")
	}
	for i := 0; i < 30 && s.CatanPendingActor() >= 0; i++ {
		before, _ := json.Marshal(s)
		s.AutoCatanPending()
		after, _ := json.Marshal(s)
		if string(before) == string(after) {
			t.Fatal("timeout response made no progress")
		}
	}
	if s.Turn == p || s.CatanPendingActor() >= 0 {
		t.Fatal("timeout movement did not finish")
	}
	p = s.Turn
	gold := s.Catan.Transport.Gold[p]
	bank := s.Catan.Transport.GoldBank
	if e := s.EliminateCatan(p); e != nil {
		t.Fatal(e)
	}
	if s.Turn == p || s.Catan.Transport.Gold[p] != 0 || s.Catan.Transport.GoldBank != bank+gold || s.Catan.Transport.Active != s.Turn {
		t.Fatal("departure did not hand off")
	}
	if e := s.validateCatanTransport(); e != nil {
		t.Fatal(e)
	}
	// The survivor can immediately take the next normal action after restoring.
	s = transportRestoreState(t, s)
	a, e := s.BotAction(s.Turn)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(s.Turn, a); e != nil {
		t.Fatal(e)
	}
}
func TestCatanTransportGoldPlayerTradeAndRoadRestrictions(t *testing.T) {
	s := transportState(t, true)
	g := s.Catan
	p := s.Turn
	target := (p + 1) % 3
	s.Phase = "catan_turn"
	for _, port := range g.Transport.Map.Sites {
		if g.canSettlement(p, port.Center, true) {
			t.Fatal("can settle plaza")
		}
		for _, edge := range port.Blocked {
			if g.canRoad(p, edge) {
				t.Fatal("can road X")
			}
		}
	}
	catanMove(g.Players[target].Resources, g.Bank, slices.Clone(g.Players[target].Resources))
	transportGive(g, target, []int{1, 0, 0, 0, 0})
	offer := Action{Type: "catan_trade_offer", Give: []int{0, 0, 0, 0, 0}, Take: []int{1, 0, 0, 0, 0}, GoldGive: 2}
	if e := s.Apply(p, offer); e != nil {
		t.Fatal(e)
	}
	id := s.Catan.Trade.ID
	if e := s.Apply(target, Action{Type: "catan_trade_accept", Offer: id}); e != nil {
		t.Fatal(e)
	}
	if e := s.Apply(p, Action{Type: "catan_trade_complete", Offer: id, Target: target}); e != nil {
		t.Fatal(e)
	}
	if s.Catan.Transport.Gold[p] != 3 || s.Catan.Transport.Gold[target] != 7 {
		t.Fatal("player gold transfer")
	}
	if e := s.validateCatanTransport(); e != nil {
		t.Fatal(e)
	}
	for _, n := range []int{2, 5, 6} {
		if _, e := newCatanTransportState(n, CatanOptions{FiveSix: n > 4}); e == nil {
			t.Fatal("unverified whole-game recipe opened", n)
		}
	}
}

func TestCatanTransportCorruptContinuationRejected(t *testing.T) {
	s := transportState(t, true)
	if e := s.catanTransportRoll(func() [2]int { return [2]int{3, 4} }); e != nil {
		t.Fatal(e)
	}
	if s.Phase == "catan_discard" {
		s.AutoCatanPending()
	}
	s.Catan.ResumePhase = "finished"
	before, _ := json.Marshal(s)
	if e := s.validateCatanTransport(); e == nil {
		t.Fatal("barbarian cannot resume at a fake finished phase")
	}
	if e := s.Apply(s.Turn, Action{Type: "catan_transport_barbarian", Offer: int(s.Catan.Transport.BarbarianSequence), Edge: 0}); e == nil {
		t.Fatal("corrupt continuation applied")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("corrupt response mutated state")
	}
	s = transportState(t, true)
	s.Phase = "catan_turn"
	if e := s.Apply(s.Turn, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	s.Phase = "catan_turn"
	if e := s.validateCatanTransport(); e == nil {
		t.Fatal("pending travel bypassed into ordinary actions")
	}
}

func TestCatanTransportChoicesUseAuthoritativeCostsAndPrivacy(t *testing.T) {
	s := transportState(t, true)
	s.Phase = "catan_turn"
	g := s.Catan
	transportGive(g, s.Turn, []int{2, 0, 1, 1, 1})
	view := s.catanTransportChoices(s.Turn)
	if view["canUpgrade"] != true || len(view["buy"].([]int)) != 5 {
		t.Fatal("action choices", view)
	}
	for _, p := range []int{-1, (s.Turn + 1) % 3} {
		if len(s.catanTransportChoices(p)) != 0 {
			t.Fatal("private action choices leaked")
		}
	}
	if err := s.Apply(s.Turn, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	choices := s.catanTransportChoices(s.Turn)
	steps := choices["steps"].([]catanTransportStep)
	if len(steps) == 0 || choices["canWheat"] != true || choices["canStop"] != true {
		t.Fatal("missing movement choices", choices)
	}
	for _, quote := range steps {
		copy := clone(*s)
		old := copy.Catan.Transport.Travel.Points
		if err := copy.Apply(copy.Turn, Action{Type: "catan_transport_step", Offer: int(copy.Catan.Transport.Sequence), Edge: quote.Edge}); err != nil {
			t.Fatal("advertised move rejected", err)
		}
		q := copy.Catan.Transport.Travel
		if q.Position != quote.To || q.Arrived < 0 && q.Points != old-quote.MP {
			t.Fatal("advertised move disagrees with Apply")
		}
	}
	for _, p := range []int{-1, (s.Turn + 1) % 3} {
		if len(s.catanTransportChoices(p)) != 0 {
			t.Fatal("private movement choices leaked")
		}
	}
	// The arrival window must never advertise another step, wheat or stop.
	for i := 0; i < 20 && s.Phase == "catan_transport_move"; i++ {
		if q := s.Catan.Transport.Travel; q.Arrived >= 0 {
			c := s.catanTransportChoices(s.Turn)
			if c["steps"] != nil || c["canWheat"] != nil || c["canStop"] != nil {
				t.Fatal("arrival has travel choices")
			}
			return
		}
		a, e := s.BotAction(s.Turn)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(s.Turn, a); e != nil {
			t.Fatal(e)
		}
	}
}

func TestCatanTransportHTTPFixturesAreFreshAndValid(t *testing.T) {
	for _, n := range []int{3, 4} {
		data, e := os.ReadFile(fmt.Sprintf("../server/testdata/catan_transport_%d.json", n))
		if e != nil {
			t.Fatal(e)
		}
		var s State
		if e = json.Unmarshal(data, &s); e != nil {
			t.Fatal(e)
		}
		if e = s.validateCatanTransport(); e != nil {
			t.Fatal(e)
		}
		if s.Catan.SetupStep != 0 || s.Phase != "catan_setup_settlement" || len(s.Catan.Players) != n || len(s.Catan.DevDeck) != 25 {
			t.Fatal("not a fresh setup")
		}
		for p, pl := range s.Catan.Players {
			if sum(pl.Resources) != 0 || sum(pl.Dev) != 0 || s.Catan.Transport.Wagons[p].Position != -1 || s.Catan.Transport.Gold[p] != 5 {
				t.Fatal("fixture starts with injected advantage")
			}
		}
	}
}
