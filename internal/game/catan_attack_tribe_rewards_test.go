package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCatanAttackTribeReservedDeck(t *testing.T) {
	s, e := newCatanAttackShores(4)
	if e != nil {
		t.Fatal(e)
	}
	g := s.Catan
	g.Seafarers.Tribe = &CatanTribeState{Development: []CatanTribeDevelopment{{Edge: 0}, {Edge: 1}, {Edge: 2}, {Edge: 3}}, Points: make([]int, 4), HeldPorts: make([][]int, 4)}
	if e = s.reserveAttackTribeRewards(); e != nil {
		t.Fatal(e)
	}
	if len(g.Attack.Deck) != 22 || len(g.DevDeck) != 0 {
		t.Fatal("deck counts")
	}
	counts, e := g.attackTribeReservedCards()
	if e != nil {
		t.Fatal(e)
	}
	for _, card := range g.Attack.Deck {
		counts[card]++
	}
	if !reflect.DeepEqual(counts, catanAttackCardCounts()) {
		t.Fatal("card conservation")
	}
	before := clone(*s)
	if s.reserveAttackTribeRewards() == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("repeat is not atomic")
	}
	raw, e := json.Marshal(s.View(-1))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(raw), `"card":`) {
		t.Fatal("hidden tribe reward leaked")
	}
	b := clone(*s)
	b.Catan.tribe().Development[0].Card = 99
	if _, e = b.Catan.attackTribeReservedCards(); e == nil {
		t.Fatal("corrupt reward accepted")
	}
}

func TestCatanAttackTribeRewardUsesScenarioEffect(t *testing.T) {
	s, e := newCatanAttackShores(4)
	if e != nil {
		t.Fatal(e)
	}
	g := s.Catan
	g.Seafarers.Tribe = &CatanTribeState{Development: []CatanTribeDevelopment{{Edge: 0}}, Points: make([]int, 4), HeldPorts: make([][]int, 4)}
	// Deterministically reserve knighthood, retaining the original inventory.
	for i, card := range g.Attack.Deck {
		if card == "knighthood" {
			last := len(g.Attack.Deck) - 1
			g.Attack.Deck[i], g.Attack.Deck[last] = g.Attack.Deck[last], g.Attack.Deck[i]
			break
		}
	}
	if e = s.reserveAttackTribeRewards(); e != nil {
		t.Fatal(e)
	}
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	if e = s.claimAttackTribeReward(s.Turn, 0); e != nil {
		t.Fatal(e)
	}
	if len(g.tribe().Development) != 0 || g.Attack.Pending == nil || g.Attack.Pending.Card != "knighthood" || s.Phase != "catan_attack_card" {
		t.Fatal("scenario reward response")
	}
	for _, p := range g.Players {
		if sum(p.Dev) != 0 || sum(p.NewDev) != 0 {
			t.Fatal("basic card leaked")
		}
	}
	counts, e := g.attackTribeReservedCards()
	if e != nil {
		t.Fatal(e)
	}
	counts[g.Attack.Pending.Card]++
	for _, c := range g.Attack.Deck {
		counts[c]++
	}
	if !reflect.DeepEqual(counts, catanAttackCardCounts()) {
		t.Fatal("claimed card duplicated")
	}
}

func TestCatanAttackTribeRouteWaitsForCard(t *testing.T) {
	s, e := newCatanAttackShores(4)
	if e != nil {
		t.Fatal(e)
	}
	g := s.Catan
	edge := 0
	g.Edges[edge].Owner, g.Edges[edge].Ship = s.Turn, true
	g.Seafarers.Tribe = &CatanTribeState{Development: []CatanTribeDevelopment{{Edge: edge}}, Points: make([]int, 4), HeldPorts: make([][]int, 4)}
	for i, c := range g.Attack.Deck {
		if c == "knighthood" {
			last := len(g.Attack.Deck) - 1
			g.Attack.Deck[i], g.Attack.Deck[last] = g.Attack.Deck[last], g.Attack.Deck[i]
			break
		}
	}
	if e = s.reserveAttackTribeRewards(); e != nil {
		t.Fatal(e)
	}
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	if e = s.catanAfterRoute(CatanRouteCompletion{Player: s.Turn, Edge: edge}); e != nil {
		t.Fatal(e)
	}
	if g.Attack.TribeRoute == nil || s.Phase != "catan_attack_card" {
		t.Fatal("route did not wait")
	}
	b := clone(*s)
	s = &b
	g = s.Catan
	a, e := s.catanAttackCardBot(s.Turn)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.catanAttackCardChoice(s.Turn, a); e != nil {
		t.Fatal(e)
	}
	if g.Attack.TribeRoute != nil || g.Attack.Pending != nil || s.Phase != "catan_turn" {
		t.Fatal("route did not resume")
	}
}
