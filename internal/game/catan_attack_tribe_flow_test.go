package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// One response chain: reward card -> free neutral knight -> port -> neutral ship.
func TestCatanAttackTribeTwoRewardPortNeutralChain(t *testing.T) {
	s, err := NewCatanAttackTribe(2)
	if err != nil {
		t.Fatal(err)
	}
	finishAttackSetup(t, s)
	g := s.Catan
	actor := s.Turn
	clear(g.Attack.Barbarians)
	g.Attack.Sequence = 0
	g.Attack.Landing = nil
	g.Attack.Knights = nil
	g.Two.Rolls = []int{6, 8}
	s.Phase = "catan_turn"
	reward := g.tribe().Development[0]
	// Exchange a reserved card with the draw pile without changing inventory.
	for i, card := range g.Attack.Deck {
		if card == "knighthood" {
			g.Attack.Deck[i] = catanAttackTribeCards[reward.Card]
			g.tribe().Development[0].Card = 1
			break
		}
	}
	g.Edges[reward.Edge].Owner, g.Edges[reward.Edge].Ship = actor, true
	// A held port must be placed only after the card and shared neutral knight.
	port := g.tribe().Ports[0]
	g.tribe().Ports = g.tribe().Ports[1:]
	g.tribe().HeldPorts[actor] = append(g.tribe().HeldPorts[actor], port.Resource)
	if len(g.tribePortEdges(actor)) == 0 {
		t.Fatal("fixture lacks port position")
	}
	before := clone(*s)
	if err = s.catanAfterRoute(CatanRouteCompletion{Player: actor, Edge: reward.Edge}); err != nil {
		t.Fatal(err)
	}
	if err = s.catanTwoAfterAction(&before, Action{Type: "catan_ship"}); err != nil {
		t.Fatal(err)
	}
	if g.Attack.TribeRoute == nil || g.Two.AfterRoute != "ship" || g.Two.Pending != nil || g.tribe().Pending != nil {
		t.Fatal("reward was bypassed")
	}
	for phase := 0; phase < 2; phase++ {
		b := clone(*s)
		s = &b
		g = s.Catan
		if err = s.validateCatanAttack(); err != nil {
			t.Fatal("reward restore", err)
		}
		if err = s.validateCatanTwo(); err != nil {
			t.Fatal("neutral restore", err)
		}
		before = clone(*s)
		a, e := s.catanAttackCardBot(actor)
		if e != nil {
			t.Fatal(e)
		}
		if err = s.catanAttackCardChoice(actor, a); err != nil {
			t.Fatal(err)
		}
		if err = s.catanTwoAfterAction(&before, a); err != nil {
			t.Fatal(err)
		}
		if phase == 0 && (g.Attack.Pending == nil || !g.Attack.Pending.Neutral || g.Two.Pending != nil) {
			t.Fatal("neutral knight missing or neutral ship started early")
		}
	}
	if s.Phase != "catan_port" || g.Attack.TribeRoute != nil || g.Two.AfterRoute != "ship" || g.Two.Pending != nil {
		t.Fatal("port did not precede neutral build", s.Phase)
	}
	b := clone(*s)
	s = &b
	g = s.Catan
	if err = s.validateCatanAttack(); err != nil {
		t.Fatal("port restore", err)
	}
	for step := 0; s.Phase == "catan_port" && step < 10; step++ {
		before = clone(*s)
		a, e := s.catanTribePortBot(actor)
		if e != nil {
			t.Fatal(e)
		}
		if err = s.catanPlaceTribePort(actor, a); err != nil {
			t.Fatal(err)
		}
		if err = s.catanTwoAfterAction(&before, a); err != nil {
			t.Fatal(err)
		}
	}
	if s.Phase != "catan_two_build" || g.Two.Pending == nil || g.Two.Pending.Kind != "ship" || g.Two.AfterRoute != "" {
		t.Fatal("neutral ship did not start", s.Phase)
	}
	if err = s.validateCatanTwo(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanAttackTribeMapRewardsAndShortage(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s, err := NewCatanAttackTribe(n)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if n <= 4 {
			if g.Tiles[21].Resource != catanCastle || g.Tiles[26].Resource != CatanDesert || g.Tiles[26].Number != 0 {
				t.Fatal("printed castle/desert")
			}
		}
		for _, id := range g.Attack.Map.Coast {
			want := 0
			if g.Tiles[id].Number == 2 || g.Tiles[id].Number == 12 {
				want = 1
			}
			if g.Attack.Barbarians[id] != want {
				t.Fatal("initial landing")
			}
		}
		raw, _ := json.Marshal(s.View(-1))
		var view map[string]any
		json.Unmarshal(raw, &view)
		// Inspect just the reward deck, excluding the public last-played card.
		sea := view["catan"].(map[string]any)["seafarers"].(map[string]any)
		cards, _ := json.Marshal(sea["tribe"].(map[string]any)["development"])
		if strings.Contains(string(cards), "card") {
			t.Fatal("reserved face leaked")
		}
		for _, mutate := range []func(*Catan){func(g *Catan) { g.tribe().AttackRules = "" }, func(g *Catan) { g.tribe().Development[0].Card = 99 }, func(g *Catan) { g.tribe().Points[0]++ }, func(g *Catan) { g.Attack.Map.Castles[0] = 0 }} {
			b := clone(*s)
			mutate(b.Catan)
			if b.validateCatanAttack() == nil {
				t.Fatal("corrupt tribe accepted")
			}
		}
		finishAttackSetup(t, s)
		g = s.Catan
		s.Phase = "catan_turn"
		clear(g.Attack.Barbarians)
		g.Attack.Prisoners[0] = g.Attack.Map.Barbarians - 1
		if err = s.catanAttackLanding(func() [2]int { return [2]int{1, 1} }, func(int) int { return 1 }); err != nil {
			t.Fatal(err)
		}
		if !g.Attack.Landing.Rolls[0].Shortage || sum(g.Attack.Barbarians) != 1 {
			t.Fatal("last-piece policy")
		}
		if err = s.validateCatanAttack(); err != nil {
			t.Fatal(err)
		}
	}
	// Claimed rewards never return on replay or when unrelated turn state changes.
	s, err := NewCatanAttackTribe(3)
	if err != nil {
		t.Fatal(err)
	}
	b := clone(*s)
	b.Catan.tribe().Development[0].Edge = b.Catan.tribe().Development[1].Edge
	if _, err = b.Catan.attackTribeReservedCards(); err == nil {
		t.Fatal("duplicate edge")
	}
	if !reflect.DeepEqual(s.Catan.Attack.Map.Coast, []int{13, 14, 15, 16, 17, 18, 33, 32, 31, 30, 29, 28}) {
		t.Fatal("battle order")
	}
}
