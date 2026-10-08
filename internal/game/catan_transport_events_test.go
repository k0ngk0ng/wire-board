package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func transportEventsGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanTransport(n)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		p := ckActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		helperApply(t, s, p, a)
	}
	return s
}

func transportEventResponses(t *testing.T, s *State) *State {
	t.Helper()
	for i := 0; s.Phase != "catan_turn" && s.Phase != "catan_roll" && i < 80; i++ {
		s = referenceEventRestore(t, s)
		if err := s.validateCatanTransport(); err != nil {
			t.Fatal(err)
		}
		p := ckActor(s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err, s.Phase)
		}
		helperApply(t, s, p, a)
	}
	if s.Phase != "catan_turn" && s.Phase != "catan_roll" {
		t.Fatal("unresolved transport event", s.Phase)
	}
	return referenceEventRestore(t, s)
}

func TestCatanTransportEventsAllFaces(t *testing.T) {
	for n := 2; n <= 6; n++ {
		seed := transportEventsGame(t, n)
		for kind := range catanCardEventNames {
			t.Run(fmt.Sprintf("%d/%s", n, kind), func(t *testing.T) {
				cp := clone(*seed)
				s := &cp
				before := clone(s.Catan.Transport)
				id := referenceEventTop(t, s, kind)
				helperReject(t, s, (s.Turn+1)%n, Action{Type: "catan_roll"})
				helperApply(t, s, s.Turn, Action{Type: "catan_roll", Card: 999})
				s = transportEventResponses(t, s)
				if s.Catan.RollID != 1 || len(s.Catan.EventDeck.Deck.Discard) != 1 || s.Catan.RevealedEvent.Kind != kind || s.Catan.RevealedEvent.Production != catanEventReferenceFaces[id].Production || !s.Catan.RevealedEvent.ProductionStarted {
					t.Fatal("event draw or continuation mismatch")
				}
				if s.Catan.Robber != -1 || s.Catan.Transport.Travel != nil || !reflect.DeepEqual(s.Catan.Transport.Wagons, before.Wagons) {
					t.Fatal("event moved robber or wagon")
				}
				if kind == "robber_flees" && s.Catan.Transport.Barbarians != before.Barbarians {
					t.Fatal("robber flees moved barbarians")
				}
				if kind == "earthquake" {
					for p := range s.Catan.Players {
						if !s.Catan.hasDamagedRoad(p) {
							t.Fatal("player did not damage a road", p)
						}
					}
				}
				if n == 2 {
					if s.Phase != "catan_roll" || !slices.Equal(s.Catan.Two.Rolls, []int{catanEventReferenceFaces[id].Production}) {
						t.Fatal("first card must resume second draw")
					}
					helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
					s = transportEventResponses(t, s)
				}
				if s.Phase != "catan_turn" {
					t.Fatal("not ready to build")
				}
				deck := clone(s.Catan.EventDeck)
				helperApply(t, s, s.Turn, Action{Type: "catan_end"})
				if s.Phase != "catan_transport_move" || s.Catan.Transport.Travel == nil {
					t.Fatal("no wagon phase after production")
				}
				helperApply(t, s, s.Turn, Action{Type: "catan_transport_stop", Offer: int(s.Catan.Transport.Sequence)})
				if !reflect.DeepEqual(deck, s.Catan.EventDeck) {
					t.Fatal("end movement drew a card")
				}
				if n > 4 && (!s.Catan.Paired.Second || s.Phase != "catan_turn") {
					t.Fatal("second paired player should build without drawing")
				}
				if err := s.validateCatanTransport(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCatanTransportEventsTwoDrawsAndEndpoints(t *testing.T) {
	for _, ids := range [][]int{{22, 23}, {15, 16}, {0, 35}} {
		t.Run(fmt.Sprint(ids), func(t *testing.T) {
			s := transportEventsGame(t, 2)
			twoReferenceTop(t, s, ids...)
			if ids[0] == 15 {
				for p := range s.Catan.Players {
					for c := range 5 {
						catanGive(s.Catan, p, c, 2-s.Catan.Players[p].Resources[c])
					}
				}
			}
			owner := s.Turn
			for i, id := range ids {
				helperApply(t, s, owner, Action{Type: "catan_roll"})
				if i == 0 && id == 15 && s.Phase != "catan_discard" {
					t.Fatal("seven missed discards")
				}
				if id == 15 || id == 16 {
					for s.Phase == "catan_discard" {
						p := ckActor(s)
						a, err := s.BotAction(p)
						if err != nil {
							t.Fatal(err)
						}
						helperApply(t, s, p, a)
					}
					if s.Phase != "catan_transport_barbarian" {
						t.Fatal("seven must move barbarian before second card/building")
					}
					helperReject(t, s, owner, Action{Type: "catan_roll"})
				}
				s = transportEventResponses(t, s)
				if s.Catan.RollID != i+1 || s.Catan.Two.Rolls[i] != catanEventReferenceFaces[id].Production || !slices.Equal(s.Catan.EventDeck.Deck.Discard, ids[:i+1]) {
					t.Fatal("duplicate or endpoint caused skipped draw")
				}
				if i == 0 {
					if s.Phase != "catan_roll" {
						t.Fatal("first card did not resume second draw")
					}
					helperReject(t, s, owner, Action{Type: "catan_end"})
				}
			}
			if s.Phase != "catan_turn" || s.Turn != owner {
				t.Fatal("two cards did not resume building")
			}
		})
	}
}

func TestCatanTransportEventsEpidemicAndCurrencyIsolation(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := transportEventsGame(t, n)
		g := s.Catan
		for p := range g.Players {
			catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
		}
		id := referenceEventTop(t, s, "epidemic")
		production := catanEventReferenceFaces[id].Production
		// Keep the official map; place one explicit test city on a producing
		// corner so the epidemic assertion cannot pass without city output.
		chosen := -1
		for i, tile := range g.Tiles {
			if tile.Number == production && tile.Resource < 5 {
				chosen = i
				break
			}
		}
		if chosen < 0 {
			t.Fatal("no matching production tile")
		}
		vertex := g.Tiles[chosen].Vertices[0]
		g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 0, 2
		want := make([][]int, n)
		for p := range want {
			want[p] = make([]int, 5)
		}
		for _, tile := range g.Tiles {
			if tile.Number != production || tile.Resource >= 5 {
				continue
			}
			for _, vertex := range tile.Vertices {
				v := g.Vertices[vertex]
				if v.Owner >= 0 && v.Level > 0 {
					want[v.Owner][tile.Resource]++
				}
			}
		}
		gold, wagons := slices.Clone(g.Transport.Gold), clone(g.Transport.Wagons)
		helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
		for p := range want {
			if !slices.Equal(s.Catan.Players[p].Resources, want[p]) {
				t.Fatal("epidemic must produce one per city", n, p)
			}
		}
		if !slices.Equal(gold, s.Catan.Transport.Gold) || !reflect.DeepEqual(wagons, s.Catan.Transport.Wagons) {
			t.Fatal("production changed gold/cargo")
		}
	}
}

func TestCatanTransportEventsNewYearAndHiddenOrder(t *testing.T) {
	for _, n := range []int{2, 6} {
		s := transportEventsGame(t, n)
		for s.Catan.RollID < 33 {
			if s.Phase == "catan_roll" {
				other := clone(*s)
				pile := other.Catan.EventDeck.Deck.DrawPile
				pile[0], pile[1] = pile[1], pile[0]
				for p := -1; p < n; p++ {
					if !reflect.DeepEqual(s.View(p), other.View(p)) {
						t.Fatal("hidden event order leaked")
					}
				}
				a, err := s.BotAction(s.Turn)
				if err != nil {
					t.Fatal(err)
				}
				b, err := other.BotAction(other.Turn)
				if err != nil || !reflect.DeepEqual(a, b) {
					t.Fatal("bot consulted hidden order")
				}
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				s = transportEventResponses(t, s)
			} else {
				helperApply(t, s, s.Turn, Action{Type: "catan_end"})
				helperApply(t, s, s.Turn, Action{Type: "catan_transport_stop", Offer: int(s.Catan.Transport.Sequence)})
			}
		}
		if s.Catan.EventDeck.Deck.Cycle != 2 || len(s.Catan.EventDeck.Deck.Discard) != 2 {
			t.Fatal("New Year cycle lost")
		}
		s = referenceEventRestore(t, s)
		bad := clone(*s)
		bad.Catan.RevealedEvent.Production++
		helperReject(t, &bad, bad.Turn, Action{Type: "catan_roll"})
		if bad.validateCatanEventSession() == nil {
			t.Fatal("transport accepted corrupt event continuation")
		}
	}
}

func TestCatanTransportEventsDamagedRoadTravel(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s := transportEventsGame(t, n)
		referenceEventTop(t, s, "earthquake")
		helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
		s = transportEventResponses(t, s)
		if n == 2 {
			referenceEventTop(t, s, "beautiful_day")
			helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
			s = transportEventResponses(t, s)
		}
		g := s.Catan
		id := -1
		for _, e := range g.Edges {
			if e.Owner >= 0 && e.Owner != s.Turn && e.Damaged {
				id = e.ID
				break
			}
		}
		if id < 0 {
			t.Fatal("event did not damage opponent road")
		}
		edge := g.Edges[id]
		// Explicit movement fixture: put the wagon at the damaged edge after
		// resolving the real event; don't fake the event or movement cost.
		g.Transport.Wagons[s.Turn].Position = edge.A
		gold := slices.Clone(g.Transport.Gold)
		helperApply(t, s, s.Turn, Action{Type: "catan_end"})
		g = s.Catan
		tr := g.Transport
		quote, err := tr.Travel.quote(g, tr.Map, tr.Barbarians, tr.Gold, id)
		wantMP := 2
		if slices.Contains(tr.Barbarians[:], id) {
			wantMP += 2
		}
		if err != nil || quote.MP != wantMP || quote.Toll != 1 || quote.Pay != edge.Owner {
			t.Fatal("damaged road did not retain toll / lose road speed", quote, err)
		}
		points, actor := tr.Travel.Points, s.Turn
		helperApply(t, s, actor, Action{Type: "catan_transport_step", Edge: id, Offer: int(tr.Sequence)})
		tr = s.Catan.Transport
		if tr.Wagons[actor].Position != edge.B || tr.Travel.Points != points-wantMP || tr.Gold[actor] != gold[actor]-1 || tr.Gold[edge.Owner] != gold[edge.Owner]+1 {
			t.Fatal("movement did not apply authoritative quote")
		}
		referenceEventRestore(t, s)
	}
}
