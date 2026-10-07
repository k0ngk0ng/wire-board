package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func forceAttackCard(t *testing.T, s *State, card string) {
	t.Helper()
	a := s.Catan.Attack
	i := slices.Index(a.Deck, card)
	if i < 0 {
		t.Fatal("card not in deck", card)
	}
	a.Deck[i], a.Deck[len(a.Deck)-1] = a.Deck[len(a.Deck)-1], a.Deck[i]
}

func buyAttackCard(t *testing.T, s *State, card string) {
	t.Helper()
	forceAttackCard(t, s, card)
	attackHand(s, s.Turn, []int{0, 0, 1, 1, 1})
	if e := s.Apply(s.Turn, Action{Type: "catan_buy_dev"}); e != nil {
		t.Fatal(e)
	}
	assertAttackRestored(t, s)
}

func TestCatanAttackCardsImmediatePlayGuardsPersistenceAndPrivacy(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, card := range []string{"capture", "knighthood", "swift_knight", "treason"} {
			for _, auto := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/auto=%v", n, card, auto), func(t *testing.T) {
					s := newAttackState(t, n, false)
					p := s.Turn
					buyAttackCard(t, s, card)
					a := s.Catan.Attack
					if s.Phase != "catan_attack_card" || s.CatanPendingActor() != p || a.CardSequence != 1 || a.Pending.Card != card || len(a.Deck) != 25 || len(a.Discard) != 0 || sum(s.Catan.Players[p].Resources) != 0 {
						t.Fatal("purchase did not enter mandatory immediate use")
					}
					choice, e := s.BotAction(p)
					if e != nil {
						t.Fatal(e)
					}
					for _, bad := range []Action{{Type: "catan_end"}, {Type: "catan_roll"}, {Type: "catan_buy_dev"}, {Type: "catan_trade_offer"}, {Type: "catan_dev"}, {Type: "catan_discard"}} {
						attackReject(t, s, p, bad)
					}
					attackReject(t, s, (p+1)%n, choice)
					bad := choice
					bad.Prompt++
					attackReject(t, s, p, bad)
					bad = choice
					bad.Choice = "different_card"
					attackReject(t, s, p, bad)
					if e := s.EliminateCatan(p); e == nil {
						t.Fatal("pending responder was eliminated instead of auto-resolving")
					}
					for viewer := -1; viewer < n; viewer++ {
						v := s.View(viewer)["catan"].(map[string]any)
						pub := v["attack"].(map[string]any)
						if pub["deck"] != nil || pub["canAct"] != (viewer == p) || pub["pending"].(map[string]any)["card"] != card {
							t.Fatal("public card or control/deck privacy")
						}
						if viewer != p && (pub["targets"] != nil || pub["edges"] != nil || pub["sources"] != nil || pub["destinations"] != nil) {
							t.Fatal("nonresponder received choice controls")
						}
						for owner, raw := range v["players"].([]any) {
							if owner != viewer && raw.(map[string]any)["resources"] != nil {
								t.Fatal("opponent hand leaked")
							}
						}
					}
					other := clone(*s)
					slices.Reverse(other.Catan.Attack.Deck)
					// Redistribute an opponent's resource types without changing totals.
					attackHand(&other, (p+1)%n, []int{0, 0, 0, 3, 2})
					otherChoice, e := other.BotAction(p)
					if e != nil || !reflect.DeepEqual(choice, otherChoice) {
						t.Fatal("card bot used an opponent's hand or future deck")
					}
					if auto {
						s.AutoCatanPending()
					} else if e = s.Apply(p, choice); e != nil {
						t.Fatal(e)
					}
					a = s.Catan.Attack
					if s.Phase != "catan_turn" || s.Turn != p || s.CatanPendingActor() != -1 || a.Pending != nil || !slices.Equal(a.Discard, []string{card}) || sum(s.Catan.Players[p].Dev) != 0 {
						t.Fatal("card did not resolve exactly once and resume action")
					}
					switch card {
					case "capture":
						if a.Prisoners[p] != 1 || sum(a.Barbarians) != 1 {
							t.Fatal("capture effect")
						}
					case "knighthood", "swift_knight":
						if len(a.Knights) != 1 || a.Knights[0].Player != p || a.Knights[0].Edge != choice.Edge || s.Catan.Players[p].Knights != 0 {
							t.Fatal("independent knight effect")
						}
					case "treason":
						if a.Gold[p] != 2 || sum(a.Barbarians) != 2 {
							t.Fatal("treason effect")
						}
					}
					attackReject(t, s, p, choice)
					assertAttackRestored(t, s)
				})
			}
		}
	}
}

func TestCatanAttackCaptureLiberationAndImmediateVictory(t *testing.T) {
	s := newAttackState(t, 3, false)
	g, p := s.Catan, s.Turn
	tile := g.Attack.Map.Coast[0]
	g.Attack.Barbarians[tile] = 3
	g.Attack.Prisoners[p] = 23 // 11 points; the next prisoner reaches 12.
	s.catanScores()
	if g.Players[p].Score != 11 {
		t.Fatal("fixture score")
	}
	buyAttackCard(t, s, "capture")
	q := s.Catan.Attack.Pending
	if e := s.Apply(p, Action{Type: "catan_attack_card", Prompt: q.ID, Choice: q.Card, Tile: tile}); e != nil {
		t.Fatal(e)
	}
	g = s.Catan
	if !s.Finished || s.Phase != "finished" || !slices.Equal(s.Winners, []int{p}) || g.Attack.Barbarians[tile] != 2 || g.Players[p].Score != 12 || g.Attack.Pending != nil || !g.tileProduces(g.Tiles[tile], g.Tiles[tile].Number) {
		t.Fatal("capture did not liberate, score and end immediately")
	}
	assertAttackRestored(t, s)
}

func TestCatanAttackCaptureRedrawShuffleAndRepeatedSameTurnPurchases(t *testing.T) {
	s := newAttackState(t, 3, false)
	a := s.Catan.Attack
	clear(a.Barbarians)
	// Four captures at the end of the pile, followed by a knighthood.
	slices.SortFunc(a.Deck, func(one, two string) int {
		weight := func(card string) int {
			if card == "capture" {
				return 2
			}
			if card == "knighthood" {
				return 1
			}
			return 0
		}
		return weight(one) - weight(two)
	})
	buyAttackCard(t, s, "capture")
	a = s.Catan.Attack
	if a.Pending.Card != "knighthood" || a.CardSequence != 1 || len(a.Deck) != 21 || !slices.Equal(a.Discard, []string{"capture", "capture", "capture", "capture"}) || sum(s.Catan.Players[s.Turn].Resources) != 0 {
		t.Fatal("empty board capture did not redraw for free")
	}
	s.AutoCatanPending()
	// Recycling the complete discard also supports a new purchase in this turn.
	a = s.Catan.Attack
	a.Discard = append(a.Discard, a.Deck...)
	a.Deck = []string{}
	beforeSeq, beforeRound, beforeTurn := a.CardSequence, s.Round, s.Turn
	attackHand(s, s.Turn, []int{0, 0, 1, 1, 1})
	if e := s.Apply(s.Turn, Action{Type: "catan_buy_dev"}); e != nil {
		t.Fatal(e)
	}
	if s.Catan.Attack.CardSequence != beforeSeq+1 || s.Round != beforeRound || s.Turn != beforeTurn || s.Catan.Attack.Pending == nil {
		t.Fatal("repeated purchase or shuffle failed")
	}
	s.AutoCatanPending()
	assertAttackRestored(t, s)
}

func TestCatanAttackKnightPlacementLimitsAndOccupiedEdges(t *testing.T) {
	for _, card := range []string{"knighthood", "swift_knight"} {
		s := newAttackState(t, 6, false)
		g, p := s.Catan, s.Turn
		edges := g.Attack.recruitEdges(g, p, card)
		occupied := edges[0]
		g.Attack.Knights = append(g.Attack.Knights, catanAttackKnight{Player: (p + 1) % 6, Edge: occupied})
		// A road does not block either type of physical knight placement.
		chosen := edges[1]
		g.Edges[chosen].Owner = (p + 1) % 6
		buyAttackCard(t, s, card)
		q := s.Catan.Attack.Pending
		action := Action{Type: "catan_attack_card", Prompt: q.ID, Choice: q.Card, Edge: occupied}
		attackReject(t, s, p, action)
		if card == "knighthood" {
			for _, edge := range g.Edges {
				if !g.Attack.castleEdge(g, edge.ID) {
					action.Edge = edge.ID
					attackReject(t, s, p, action)
					break
				}
			}
		}
		action.Edge = chosen
		if e := s.Apply(p, action); e != nil {
			t.Fatal(e)
		}
		g = s.Catan
		if g.Edges[chosen].Owner != (p+1)%6 || len(g.Attack.Knights) != 2 {
			t.Fatal("road overwritten")
		}
		for len(g.Attack.Knights) < 7 {
			edges = g.Attack.recruitEdges(g, p, "swift_knight")
			g.Attack.Knights = append(g.Attack.Knights, catanAttackKnight{Player: p, Edge: edges[0]})
		}
		buyAttackCard(t, s, card)
		if s.Catan.Attack.Pending != nil || s.Phase != "catan_turn" || len(s.Catan.Attack.Knights) != 7 || len(s.Catan.Attack.Discard) != 2 {
			t.Fatal("exhausted knight supply created a seventh piece or redrew")
		}
		assertAttackRestored(t, s)
	}
	// Swift Knight really means ANY empty edge, including a castle edge.
	s := newAttackState(t, 3, false)
	castle := s.Catan.Attack.recruitEdges(s.Catan, s.Turn, "knighthood")[0]
	buyAttackCard(t, s, "swift_knight")
	q := s.Catan.Attack.Pending
	if e := s.Apply(s.Turn, Action{Type: "catan_attack_card", Prompt: q.ID, Choice: q.Card, Edge: castle}); e != nil {
		t.Fatal(e)
	}
	assertAttackRestored(t, s)
	// Occupied castle edges prevent Knighthood even if the player has pieces;
	// placement must not displace another player's knight or move it early.
	s = newAttackState(t, 3, false)
	for _, edge := range s.Catan.Attack.recruitEdges(s.Catan, s.Turn, "knighthood") {
		s.Catan.Attack.Knights = append(s.Catan.Attack.Knights, catanAttackKnight{Player: (s.Turn + 1) % 3, Edge: edge})
	}
	before := slices.Clone(s.Catan.Attack.Knights)
	buyAttackCard(t, s, "knighthood")
	if s.Catan.Attack.Pending != nil || !reflect.DeepEqual(before, s.Catan.Attack.Knights) || len(s.Catan.Attack.Discard) != 1 {
		t.Fatal("full castle created a knight, displaced a piece or redrew")
	}
	assertAttackRestored(t, s)
}

func TestCatanAttackCardBotCanBuyWithoutOrdinaryDeck(t *testing.T) {
	s := newAttackState(t, 3, false)
	attackHand(s, s.Turn, []int{0, 0, 1, 1, 1})
	if len(s.Catan.DevDeck) != 0 {
		t.Fatal("fixture ordinary deck")
	}
	before, _ := json.Marshal(s)
	a, e := s.BotAction(s.Turn)
	if e != nil || a.Type != "catan_buy_dev" {
		t.Fatal("bot cannot buy the independent deck", a, e)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("bot's legality simulation consumed the real deck")
	}
	if e = s.Apply(s.Turn, a); e != nil {
		t.Fatal(e)
	}
	s.AutoCatanPending()
	if s.Catan.Attack.Pending != nil || s.Phase != "catan_turn" {
		t.Fatal("bot did not complete a purchased card")
	}
	assertAttackRestored(t, s)
}

func TestCatanAttackTreasonSourcesConquestAndSupply(t *testing.T) {
	for _, boardSources := range []int{0, 1, 2} {
		s := newAttackState(t, 3, false)
		a := s.Catan.Attack
		clear(a.Barbarians)
		from := []int{-1, -1}
		for i := range boardSources {
			from[i] = a.Map.Coast[i]
			a.Barbarians[from[i]] = 3
		}
		buyAttackCard(t, s, "treason")
		a = s.Catan.Attack
		to := []int{a.Map.Coast[2], a.Map.Coast[3]}
		choice := Action{Type: "catan_attack_card", Prompt: a.Pending.ID, Choice: "treason", Give: from, Take: to}
		for _, badTo := range [][]int{{to[0], to[0]}, {to[0]}, {to[0], -1}, {to[0], a.Map.Castles[0]}} {
			bad := choice
			bad.Take = badTo
			attackReject(t, s, s.Turn, bad)
		}
		if boardSources > 0 {
			bad := choice
			bad.Give = []int{-1, -1}
			attackReject(t, s, s.Turn, bad)
			bad = choice
			bad.Give = []int{from[0], from[0]}
			attackReject(t, s, s.Turn, bad)
			bad = choice
			bad.Take = []int{to[0], from[0]}
			attackReject(t, s, s.Turn, bad)
		}
		before := a.supply()
		if e := s.Apply(s.Turn, choice); e != nil {
			t.Fatal(e)
		}
		a = s.Catan.Attack
		if a.supply() != before-(2-boardSources) || a.Gold[s.Turn] != 2 || a.Barbarians[to[0]] != 1 || a.Barbarians[to[1]] != 1 {
			t.Fatal("treason supply")
		}
		for _, id := range from {
			if id >= 0 && a.Barbarians[id] != 2 {
				t.Fatal("source did not liberate")
			}
		}
		assertAttackRestored(t, s)
	}
	// Both destinations become conquered; no source may double as a destination.
	s := newAttackState(t, 6, false)
	a := s.Catan.Attack
	clear(a.Barbarians)
	from, to := a.Map.Coast[:2], a.Map.Coast[2:4]
	for _, id := range from {
		a.Barbarians[id] = 3
	}
	for _, id := range to {
		a.Barbarians[id] = 2
	}
	buyAttackCard(t, s, "treason")
	q := s.Catan.Attack.Pending
	if e := s.Apply(s.Turn, Action{Type: "catan_attack_card", Prompt: q.ID, Choice: q.Card, Give: from, Take: to}); e != nil {
		t.Fatal(e)
	}
	for _, id := range to {
		if !s.Catan.Attack.conquered(id) {
			t.Fatal("treason failed to conquer")
		}
	}
	assertAttackRestored(t, s)
}

func TestCatanAttackCardPublicBoundaryGatesAndCorruptPending(t *testing.T) {
	overflow := newAttackState(t, 3, false)
	a := overflow.Catan.Attack
	attackCoins(overflow, (overflow.Turn+1)%3, a.Map.Gold-1)
	a.GoldIssued = catanGoldLedgerLimit
	a.Gold[(overflow.Turn+1)%3] += a.GoldIssued
	attackHand(overflow, overflow.Turn, []int{0, 0, 1, 1, 1})
	for _, card := range []string{"capture", "knighthood", "swift_knight", "treason"} {
		forceAttackCard(t, overflow, card)
		attackReject(t, overflow, overflow.Turn, Action{Type: "catan_buy_dev"})
	}
	s := newAttackState(t, 3, false)
	buyAttackCard(t, s, "capture")
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Attack.Pending.ID++ },
		func(s *State) { s.Catan.Attack.Pending.Player = (s.Turn + 1) % 3 },
		func(s *State) { s.Catan.Attack.Pending.Card = "unknown" },
		func(s *State) { s.Catan.Attack.Pending = nil },
		func(s *State) { s.Phase = "catan_turn" },
		func(s *State) { s.Catan.Attack.Deck = append(s.Catan.Attack.Deck, "capture") },
	} {
		bad := clone(*s)
		mutate(&bad)
		before, _ := json.Marshal(&bad)
		if e := bad.validateCatanAttack(); e == nil {
			t.Fatal("invalid pending accepted")
		}
		bad.AutoCatanPending()
		after, _ := json.Marshal(&bad)
		if string(before) != string(after) {
			t.Fatal("timeout changed corrupted state")
		}
	}
}
