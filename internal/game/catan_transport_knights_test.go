package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func transportKnightRestore(t *testing.T, s *State) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(data, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	*s = next
}
func TestCatanTransportKnightsNaturalGames(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := NewCatanTransportCitiesKnights(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 18000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(step, s.Phase, e)
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, s.Phase, a, e)
					}
					if step%89 == 0 {
						transportKnightRestore(t, s)
					}
				}
				if !s.Finished || s.Catan.victoryTarget() != 15 {
					t.Fatal("unfinished", s.Round, s.Phase)
				}
				transportKnightRestore(t, s)
			})
		}
	}
}

func transportKnightReady(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanTransportCitiesKnights(n)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 100 && s.Phase != "catan_turn"; step++ {
		twoSeaStep(t, s)
	}
	if s.Phase != "catan_turn" {
		t.Fatal("setup incomplete")
	}
	return s
}

func TestCatanTransportKnightsChaseAndRestore(t *testing.T) {
	s := transportKnightReady(t, 3)
	g := s.Catan
	p, other := s.Turn, (s.Turn+1)%3
	vertex, edge := -1, -1
	for _, e := range g.Edges {
		if g.Vertices[e.A].Level == 0 && g.Transport.Map.siteAt(e.A) < 0 {
			vertex, edge = e.A, e.ID
			break
		}
	}
	g.Transport.Barbarians[0] = edge
	for i := 1; i < 3; i++ {
		for _, e := range g.Edges {
			if !slices.Contains(g.Transport.Barbarians[:i], e.ID) {
				g.Transport.Barbarians[i] = e.ID
				break
			}
		}
	}
	g.CitiesKnights.Knights = []CatanKnight{{Owner: p, Vertex: vertex, Strength: 1, Active: true}}
	target := -1
	for _, e := range g.Edges {
		if !slices.Contains(g.Transport.Barbarians[:], e.ID) {
			target = e.ID
			break
		}
	}
	g.Edges[target].Owner = other
	for c, n := range g.Players[other].Resources {
		g.Bank[c] += n
		g.Players[other].Resources[c] = 0
	}
	g.Bank[6]--
	g.Players[other].Resources[6] = 1
	transportKnightRestore(t, s)
	a := Action{Type: "catan_transport_knight_chase", Vertex: vertex, Card: 0, Edge: target}
	for _, bad := range []Action{
		{Type: a.Type, Vertex: vertex, Card: 0, Edge: edge},
		{Type: a.Type, Vertex: vertex, Card: 3, Edge: target},
		{Type: a.Type, Vertex: -1, Card: 0, Edge: target},
	} {
		before := clone(*s)
		if s.Apply(p, bad) == nil || !reflect.DeepEqual(before, *s) {
			t.Fatal("invalid chase mutated state")
		}
	}
	if s.Apply(other, a) == nil {
		t.Fatal("wrong player")
	}
	own := g.Players[p].Resources[6]
	if err := s.Apply(p, a); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || s.Catan.CitiesKnights.Knights[0].Active || s.Catan.Players[p].Resources[6] != own+1 || s.Catan.Players[other].Resources[6] != 0 {
		t.Fatal("chase/commodity theft")
	}
	if s.Apply(p, a) == nil {
		t.Fatal("inactive knight reused")
	}
	transportKnightRestore(t, s)
	// Action is harmless in an ordinary room, where the new controller is absent.
	base := transportState(t, true)
	if base.Apply(base.Turn, a) == nil {
		t.Fatal("base accepted city action")
	}
}

func TestCatanTransportKnightsDiceAndInvalidSaves(t *testing.T) {
	s := transportKnightReady(t, 3)
	s.Phase = "catan_roll"
	pair := [][2]int{{1, 1}, {6, 6}, {2, 3}}
	rolls := 0
	pos := s.Catan.CitiesKnights.BarbarianPosition
	if err := s.catanTransportCityRoll(func() [2]int { p := pair[rolls]; rolls++; return p }, 3); err != nil {
		t.Fatal(err)
	}
	if rolls != 3 || s.Catan.CitiesKnights.BarbarianPosition != pos+1 || !slices.Equal(s.Catan.Dice, []int{2, 3}) {
		t.Fatal("rerolled event die or kept unusable production")
	}
	transportKnightRestore(t, s)
	for _, mutate := range []func(*State){
		func(s *State) { s.Phase = "unknown" },
		func(s *State) { s.Catan.Bank = s.Catan.Bank[:5] },
		func(s *State) { s.Catan.Players[0].Resources = s.Catan.Players[0].Resources[:5] },
		func(s *State) { s.Catan.Transport.Knights = "unknown" },
		func(s *State) { s.Phase = "catan_pillage" },
		func(s *State) {
			s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: s.Turn, Vertex: s.Catan.Transport.Map.Sites[0].Center, Strength: 1}}
		},
	} {
		bad := clone(*s)
		mutate(&bad)
		if bad.validateCatanTransport() == nil {
			t.Fatal("invalid save accepted")
		}
	}
}

func TestCatanTransportKnightsMapCommerceAndTravel(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := transportKnightReady(t, n)
			g := s.Catan
			p := s.Turn
			counts := [5]int{}
			for _, tile := range g.Tiles {
				if tile.Resource >= 0 && tile.Resource < 5 {
					counts[tile.Resource]++
				}
			}
			expected := catanTransportBoardRecipe(n > 4).resources
			expected[0]--
			expected[3]++
			if counts != expected || g.victoryTarget() != 15 || len(g.DevDeck) != 0 {
				t.Fatal("combined map/deck")
			}
			rate := g.rates(p)[5]
			g.Bank[5] -= rate
			g.Players[p].Resources[5] += rate
			gold := g.Transport.Gold[p]
			if err := s.Apply(p, Action{Type: "catan_coin_sell", Color: 5}); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Transport.Gold[p] != gold+1 {
				t.Fatal("commodity sale")
			}
			if s.Apply(p, Action{Type: "catan_coin_buy", Color: 5}) == nil {
				t.Fatal("coin bought commodity")
			}
			if err := s.Apply(p, Action{Type: "catan_end"}); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_transport_move" || s.Catan.Transport.Moves != 1 {
				t.Fatal("missing wagon travel")
			}
			transportKnightRestore(t, s)
		})
	}
}

func TestCatanTransportKnightsEndDiscardBeforeWagon(t *testing.T) {
	s := transportKnightReady(t, 3)
	g := s.Catan
	p := s.Turn
	k := g.CitiesKnights
	for len(k.Players[p].Progress) < 5 {
		for i, id := range k.ProgressDecks[0] {
			if !catanProgressRules[id].Victory {
				k.Players[p].Progress = append(k.Players[p].Progress, id)
				k.ProgressDecks[0] = slices.Delete(k.ProgressDecks[0], i, i+1)
				break
			}
		}
	}
	transportKnightRestore(t, s)
	if err := s.Apply(p, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_progress_end" || s.Catan.Transport.Travel != nil {
		t.Fatal("wagon started before discard")
	}
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(p, a); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_transport_move" || s.Catan.Transport.Moves != 1 || len(s.Catan.CitiesKnights.Players[p].Progress) != 4 {
		t.Fatal("discard continuation")
	}
	transportKnightRestore(t, s)
}

func TestCatanTransportKnightsInventionAndActivationLock(t *testing.T) {
	for _, n := range []int{2, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := transportKnightReady(t, n)
			p := s.Turn
			choices := s.Catan.inventionNumbers()
			left, right := choices[0], choices[1]
			for _, c := range choices {
				if c.Number != left.Number {
					right = c
					break
				}
			}
			ckProgressGive(t, s, p, 3)
			if err := s.Apply(p, Action{Type: "catan_progress", Card: 3, Tile: left.Tile, Target: right.Tile}); err != nil {
				t.Fatal(err)
			}
			transportKnightRestore(t, s)
			bad := clone(*s)
			bad.Catan.Transport.Map.NumberSwaps = nil
			if bad.validateCatanTransport() == nil {
				t.Fatal("unrecorded map change")
			}
			g := s.Catan
			knight := CatanKnight{Owner: p, Vertex: g.Edges[g.Transport.Barbarians[0]].A, Strength: 1, Active: true, ActivatedAt: g.CitiesKnights.ActionSerial}
			if len(g.transportKnightBarbarians(&knight)) != 0 {
				t.Fatal("new activation acted")
			}
			knight.ActivatedAt = 0
			if len(g.transportKnightBarbarians(&knight)) == 0 {
				t.Fatal("prior activation unavailable")
			}
		})
	}
}
