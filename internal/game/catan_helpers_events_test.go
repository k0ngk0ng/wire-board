package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func helperEventGame(t *testing.T, n int, all bool) *State {
	t.Helper()
	s, err := newCatanHelperReferenceEvents(n, all)
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

func eventAssignHelper(t *testing.T, s *State, player, id int) {
	t.Helper()
	g := s.Catan
	old := g.Players[player].Helper.ID
	if old == id {
		return
	}
	if i := slices.Index(g.HelperDisplay, id); i >= 0 {
		g.HelperDisplay[i] = old
	} else {
		found := false
		for p := range g.Players {
			if g.Players[p].Helper.ID == id {
				g.Players[p].Helper = &CatanHelperSeat{ID: old}
				found = true
				break
			}
		}
		if !found {
			t.Fatal("requested helper outside this game's pool")
		}
	}
	g.Players[player].Helper = &CatanHelperSeat{ID: id}
}

func TestCatanHelperEventsAllFacesAndPairedHandoff(t *testing.T) {
	for _, n := range []int{3, 6} {
		for kind := range catanCardEventNames {
			t.Run(fmt.Sprintf("%d/%s", n, kind), func(t *testing.T) {
				s := helperEventGame(t, n, true)
				id := referenceEventTop(t, s, kind)
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				for step := 0; s.Phase != "catan_turn" && step < 40; step++ {
					s = referenceEventRestore(t, s)
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					helperApply(t, s, actor, a)
				}
				if s.Phase != "catan_turn" || s.Catan.RollID != 1 || !slices.Equal(s.Catan.EventDeck.Deck.Discard, []int{id}) || s.Catan.HelperPending != nil {
					t.Fatal("event/helper chain did not finish once")
				}
				s = referenceEventRestore(t, s)
				if n == 6 {
					secondary := s.Catan.Paired.Secondary
					eventAssignHelper(t, s, secondary, 11)
					for _, tile := range s.Catan.Tiles {
						if tile.Resource < 5 {
							s.Catan.Robber = tile.ID
							break
						}
					}
					helperApply(t, s, s.Turn, Action{Type: "catan_end"})
					if s.Turn != secondary || !s.Catan.Paired.Second {
						t.Fatal("paired handoff failed")
					}
					helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
					helperApply(t, s, s.Turn, Action{Type: "catan_helper"})
					helperApply(t, s, s.Turn, Action{Type: "catan_helper_choice", Choice: "flip"})
					if s.Catan.RollID != 1 {
						t.Fatal("secondary helper redrew event")
					}
				}
			})
		}
	}
}

func TestCatanHelperEventsHildaSeparatesRewardsFromProduction(t *testing.T) {
	for _, production := range []bool{false, true} {
		t.Run(fmt.Sprint(production), func(t *testing.T) {
			s := helperEventGame(t, 3, true)
			g := s.Catan
			owner := (s.Turn + 1) % 3
			eventAssignHelper(t, s, owner, 3)
			// Keep the physical map, but isolate whether this seat actually
			// produces on the card's 2: a single explicit settlement fixture.
			for i := range g.Vertices {
				if g.Vertices[i].Owner == owner {
					g.Vertices[i].Owner = -1
					g.Vertices[i].Level = 0
				}
			}
			color := -1
			if production {
				for _, tile := range g.Tiles {
					if tile.Number == 2 {
						v := tile.Vertices[0]
						g.Vertices[v].Owner, g.Vertices[v].Level = owner, 1
						color = tile.Resource
						break
					}
				}
			}
			before := slices.Clone(g.Players[owner].Resources)
			referenceEventTop(t, s, "plentiful_year")
			helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
			for s.Catan.CardEvent != nil {
				actor := s.CatanPendingActor()
				helperApply(t, s, actor, Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}})
			}
			g = s.Catan
			before[0]++ // This is an event reward, never production income.
			if production {
				before[color]++
				if g.HelperPending != nil {
					t.Fatal("productive Hilda got compensation")
				}
			} else {
				if g.HelperPending == nil || g.HelperPending.Player != owner || !g.HelperPending.Optional {
					t.Fatal("event reward incorrectly suppressed Hilda")
				}
				s = referenceEventRestore(t, s)
				helperApply(t, s, owner, Action{Type: "catan_helper_choice", Color: 1})
				before[1]++
				helperApply(t, s, owner, Action{Type: "catan_helper_choice", Choice: "flip"})
			}
			if !slices.Equal(before, s.Catan.Players[owner].Resources) || s.Catan.RollID != 1 {
				t.Fatal("incorrect event/production/helper quantities")
			}
		})
	}
}

func TestCatanHelperEventsThorolfSevenAndRestore(t *testing.T) {
	for _, count := range []int{7, 8} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := helperEventGame(t, 3, true)
			g := s.Catan
			owner := (s.Turn + 1) % 3
			eventAssignHelper(t, s, owner, 5)
			for p := range g.Players {
				catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
			}
			catanGive(g, owner, 0, count)
			catanGive(g, s.Turn, 1, 8)
			referenceEventTop(t, s, "robber_attacks")
			helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
			if s.Catan.DiscardDue[owner] != 0 || s.Catan.DiscardDue[s.Turn] != 4 || s.Catan.HelperPending.Player != owner {
				t.Fatal("seven ignored Thorolf or skipped other player's discard")
			}
			s = referenceEventRestore(t, s)
			if count == 7 {
				helperApply(t, s, owner, Action{Type: "catan_helper_choice", Color: 2})
			}
			helperApply(t, s, owner, Action{Type: "catan_helper_choice", Choice: "flip"})
			if s.Phase != "catan_discard" || sum(s.Catan.Players[owner].Resources) != 8 {
				t.Fatal("helper did not resume frozen discard")
			}
			helperApply(t, s, s.Turn, Action{Type: "catan_discard", Tokens: []int{0, 4, 0, 0, 0}})
			if s.Phase != "catan_robber" || s.Catan.RollID != 1 {
				t.Fatal("discard did not resume robber once")
			}
			s = referenceEventRestore(t, s)
		})
	}
}

func TestCatanHelperEventsDamagedRoadCannotBeMovedOrDiscountRepaired(t *testing.T) {
	s := helperEventGame(t, 3, true)
	owner := s.Turn
	eventAssignHelper(t, s, owner, 4)
	referenceEventTop(t, s, "earthquake")
	helperApply(t, s, owner, Action{Type: "catan_roll"})
	for s.Phase != "catan_turn" {
		actor := ckActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	g := s.Catan
	from := -1
	for _, edge := range g.Edges {
		if edge.Owner == owner && edge.Damaged {
			from = edge.ID
			break
		}
	}
	if from < 0 {
		t.Fatal("no actual damaged road")
	}
	if g.helperEndRoad(owner, from) {
		t.Fatal("damaged road offered to move helper")
	}
	view := s.View(owner)["catan"].(map[string]any)
	if len(view["helperRoadMoves"].(map[int][]int)) != 0 {
		t.Fatal("damaged road move shown in legal hints")
	}
	helperReject(t, s, owner, Action{Type: "catan_helper", Edge: from, Target: (from + 1) % len(g.Edges)})
	eventAssignHelper(t, s, owner, 2)
	helperReject(t, s, owner, Action{Type: "catan_repair_road", Edge: from, Skill: "helper", Tokens: []int{2, 0, 0, 0, 0}})
	catanGive(s.Catan, owner, 0, 1)
	catanGive(s.Catan, owner, 1, 1)
	helperApply(t, s, owner, Action{Type: "catan_repair_road", Edge: from})
	if !s.Catan.helperReady(owner, 2) || s.Catan.Edges[from].Damaged {
		t.Fatal("repair used a helper or left road damaged")
	}
	s = referenceEventRestore(t, s)
}

func TestCatanHelperEventsRejectCorruptResponses(t *testing.T) {
	s := helperEventGame(t, 3, true)
	referenceEventTop(t, s, "plentiful_year")
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	for _, bad := range []func(*State){
		func(s *State) { s.Catan.HelperDisplay = append(s.Catan.HelperDisplay, s.Catan.HelperDisplay[0]) },
		func(s *State) { s.Catan.Players[0].Helper = nil },
		func(s *State) { s.Catan.Players[0].Helper.UsedTurn = s.Catan.TurnSerial + 1 },
		func(s *State) {
			s.Catan.HelperPending = &CatanHelperPending{Player: 0, Kind: "resource", Resume: "catan_turn"}
		},
	} {
		trial := clone(*s)
		bad(&trial)
		helperReject(t, &trial, trial.CatanPendingActor(), Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}})
	}
}

func TestCatanHelperEventsNaturalGames(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := newCatanHelperReferenceEvents(n, n%2 == 0)
			if err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 5000 && !s.Finished; step++ {
				actor := ckActor(s)
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				helperApply(t, s, actor, a)
				if step%23 == 0 {
					s = referenceEventRestore(t, s)
				}
				for c, total := range s.Catan.Bank {
					for _, p := range s.Catan.Players {
						total += p.Resources[c]
					}
					want := 19
					if n > 4 {
						want = 24
					}
					if total != want {
						t.Fatal("resource conservation", c, total)
					}
				}
				g := s.Catan
				dev := len(g.DevDeck) + len(g.DevDiscard) + len(g.HelperExile)
				if q := g.HelperPending; q != nil && q.Kind == "development" {
					dev += len(q.Cards)
				}
				for _, p := range g.Players {
					dev += sum(p.Dev)
				}
				want := 25
				if n > 4 {
					want = 34
				}
				if dev != want {
					t.Fatal("development cards lost or duplicated through helpers", dev)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 10 || s.Catan.RollID == 0 {
				t.Fatal("event/helpers natural game did not finish")
			}
			s = referenceEventRestore(t, s)
		})
	}
}

func TestCatanHelperEventsPreDrawAndPrivateDevelopment(t *testing.T) {
	s := helperEventGame(t, 3, true)
	owner := s.Turn
	eventAssignHelper(t, s, owner, 10)
	for _, tile := range s.Catan.Tiles {
		if tile.Resource < 5 {
			s.Catan.Robber = tile.ID
			break
		}
	}
	helperApply(t, s, owner, Action{Type: "catan_helper"})
	s = referenceEventRestore(t, s)
	helperReject(t, s, owner, Action{Type: "catan_roll"})
	helperApply(t, s, owner, Action{Type: "catan_helper_choice", Choice: "flip"})
	if s.Phase != "catan_roll" || s.Catan.RollID != 0 || len(s.Catan.EventDeck.Deck.Discard) != 0 {
		t.Fatal("pre-draw helper consumed an event")
	}
	referenceEventTop(t, s, "beautiful_day")
	helperApply(t, s, owner, Action{Type: "catan_roll"})
	for s.Phase != "catan_turn" {
		actor := ckActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	eventAssignHelper(t, s, owner, 6)
	for _, color := range []int{2, 3, 4} {
		catanGive(s.Catan, owner, color, 1)
	}
	helperApply(t, s, owner, Action{Type: "catan_buy_dev", Skill: "helper"})
	s = referenceEventRestore(t, s)
	q := s.Catan.HelperPending
	if q.Kind != "development" || len(q.Cards) != 3 {
		t.Fatal("missing private development choice")
	}
	for viewer := -1; viewer < 3; viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)
		pending := v["helperPending"].(map[string]any)
		if (pending["cards"] != nil) != (viewer == owner) {
			t.Fatal("candidate cards privacy")
		}
		raw, _ := json.Marshal(v["eventDeck"])
		var deck map[string]any
		if err := json.Unmarshal(raw, &deck); err != nil {
			t.Fatal(err)
		}
		if deck["deck"] != nil || deck["drawPile"] != nil {
			t.Fatal("event deck leak during private helper")
		}
	}
	bad := clone(*s)
	bad.Catan.HelperPending.Cards[0] = 99
	helperReject(t, &bad, owner, Action{Type: "catan_helper_choice", Card: q.Cards[0]})
	helperApply(t, s, owner, Action{Type: "catan_helper_choice", Card: q.Cards[0]})
	helperApply(t, s, owner, Action{Type: "catan_helper_choice", Choice: "flip"})
	if s.Catan.RollID != 1 || s.Phase != "catan_turn" {
		t.Fatal("private helper repeated production")
	}
	s = referenceEventRestore(t, s)
}
