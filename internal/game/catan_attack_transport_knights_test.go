package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func attackTransportCityStarted(t *testing.T, n int) *State {
	t.Helper()
	s, err := newCatanAttackTransportKnights(n)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 100 && (s.Catan.setup() || s.Phase != "catan_turn"); step++ {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(s.Phase, a, e)
		}
	}
	if s.Catan.setup() || s.Phase != "catan_turn" {
		t.Fatal("not started")
	}
	return s
}
func TestCatanAttackTransportCityInventionRestore(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s := attackTransportCityStarted(t, n)
		g := s.Catan
		ids := g.attackCityInventionTiles()
		left, right := ids[0], -1
		for _, id := range ids {
			if g.Tiles[id].Number != g.Tiles[left].Number {
				right = id
				break
			}
		}
		if right < 0 {
			t.Fatal("no different numbers")
		}
		if err := s.catanAttackCityInvention(left, right); err != nil {
			t.Fatal(err)
		}
		if err := s.validateCatanTransport(); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(s)
		var restored State
		if err := json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if err := restored.validateCatanTransport(); err != nil {
			t.Fatal(err)
		}
		if err := restored.catanTransportBeginTravel(restored.Turn); err != nil {
			t.Fatal("travel after invention", err)
		}
		if err := restored.validateCatanTransport(); err != nil {
			t.Fatal(err)
		}
		restored.Catan.Attack.City.NumberSwaps = nil
		if restored.validateCatanTransport() == nil {
			t.Fatal("missing invention record accepted")
		}
	}
}
func TestCatanAttackTransportCityIssuedBarbarianIdentities(t *testing.T) {
	for _, n := range []int{2, 6} {
		s := attackTransportCityStarted(t, n)
		g := s.Catan
		p := &g.AttackTransport.Pieces
		// All printed pieces are captives, so a landing must issue fresh stable IDs.
		for i := range p.Barbarians {
			p.Barbarians[i] = catanAttackTransportBarbarian{-1, -1, 0}
		}
		g.syncAttackTransportCounts()
		before := len(p.Barbarians)
		targets, err := s.catanAttackCityLanding([2]int{6, 6})
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if n > 4 {
			want = 2
		}
		if len(targets) != want || g.Attack.City.Issued != want || len(p.Barbarians) != before+want || g.Attack.supply()+g.Attack.City.Issued != 0 {
			t.Fatal("issued ledger", targets, g.Attack.City.Issued)
		}
		for id := before; id < len(p.Barbarians); id++ {
			if p.Barbarians[id].Edge < 0 || p.blocking(p.Barbarians[id].Edge) != id {
				t.Fatal("new piece not blocking")
			}
		}
		if err = g.validateAttackTransportPieces(); err != nil {
			t.Fatal(err)
		}
		// The newly issued ID must also work in wagon drive-off and survive restore.
		id := before
		edge := p.Barbarians[id].Edge
		board := g.attackTransportBoard()
		travel, err := newCatanAttackTransportTravel(g, board, p, s.Turn, g.Edges[edge].A, 1)
		if err != nil {
			t.Fatal(err)
		}
		ok, err := travel.driveOff(g, board, p, g.Transport.Gold, id, 6)
		if err != nil || !ok || travel.Travel.Pending != id {
			t.Fatal("issued piece drive-off", err)
		}
		raw, err := json.Marshal(travel)
		if err != nil {
			t.Fatal(err)
		}
		var restored catanAttackTransportTravel
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if err = restored.validate(g, board, p, g.Transport.Gold); err != nil {
			t.Fatal(err)
		}
		destination := 9
		if n > 4 {
			destination = 18
		}
		destinationEdge := p.edges(g, destination, id)[0]
		if err = restored.relocate(g, board, p, g.Transport.Gold, destination, destinationEdge); err != nil {
			t.Fatal(err)
		}
		g.syncAttackTransportCounts()
		if p.blocking(edge) != -1 || p.blocking(destinationEdge) != id {
			t.Fatal("issued piece relocation")
		}
		copy := clone(*s)
		copy.Catan.Attack.City.Issued--
		if copy.validateCatanTransport() == nil {
			t.Fatal("missing issue record")
		}
	}
}
func TestCatanAttackTransportCityExtremeDiceOnce(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, face := range []int{0, 3} {
			s := attackTransportCityStarted(t, n)
			g := s.Catan
			s.Phase = "catan_roll"
			before := sum(g.AttackTransport.Pieces.counts(g))
			if err := s.catanCityRoll(6, 6, face); err != nil {
				t.Fatal(err)
			}
			want := 1
			if n > 4 {
				want = 2
			}
			if sum(g.AttackTransport.Pieces.counts(g)) != before+want || g.CitiesKnights.BarbarianPosition != 0 || g.CitiesKnights.Invasions != 0 {
				t.Fatal("missing or double invasion")
			}
			if err := s.validateCatanTransport(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestCatanAttackTransportCityInlandBattleAndIntrigue(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s := attackTransportCityStarted(t, n)
		g := s.Catan
		actor := s.Turn
		tile := 9
		if n > 4 {
			tile = 18
		}
		pieces := &g.AttackTransport.Pieces
		for i := range pieces.Barbarians {
			pieces.Barbarians[i] = catanAttackTransportBarbarian{-1, -1, -1}
		}
		edges := pieces.edges(g, tile, -1)
		pieces.Barbarians[0] = catanAttackTransportBarbarian{tile, edges[0], -1}
		g.syncAttackTransportCounts()
		if err := s.catanAttackCityIntrigue(actor, tile); err != nil {
			t.Fatal(err)
		}
		if pieces.blocking(edges[0]) != -1 || g.Attack.Prisoners[actor] != 1 {
			t.Fatal("intrigue desync")
		}
		pieces.Barbarians[1] = catanAttackTransportBarbarian{tile, edges[0], -1}
		g.syncAttackTransportCounts()
		edge := catanFishingSide(g, tile, 0)
		g.Attack.City.Knights = []catanAttackCityKnight{{Owner: actor, Edge: edge, Strength: 2, Active: true}}
		die := g.Attack.Map.edgeOrientation(g, edge) + 1
		tr := g.Transport
		other := (actor + 1) % n
		tr.Gold[other] += tr.GoldBank
		tr.GoldBank = 0
		before := tr.Gold[actor]
		battle, err := s.catanAttackCityBattle(tile, func() int { return die })
		if err != nil {
			t.Fatal(err)
		}
		g = s.Catan
		if battle == nil || len(battle.Downgraded) != 1 || g.Attack.Prisoners[actor] != 2 || g.AttackTransport.Pieces.blocking(edges[0]) != -1 || g.Transport.Gold[actor] != before+3 || g.Transport.GoldIssued != 3 {
			t.Fatal("battle result", battle)
		}
		if err = s.validateCatanTransport(); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCatanAttackTransportCityProgressDiscardThenWagon(t *testing.T) {
	s := attackTransportCityStarted(t, 3)
	actor := s.Turn
	// Obtain five non-victory cards from their real decks. Victory cards are
	// revealed immediately and must never be injected into the private hand.
	ckProgressGive(t, s, actor, 0, 1, 2, 3, 4)
	if err := s.Apply(actor, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_progress_end" {
		t.Fatal("missing discard", s.Phase)
	}
	for step := 0; step < 40 && s.Phase != "catan_transport_move"; step++ {
		p := twoFullActor(s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(p, a); err != nil {
			t.Fatal(err)
		}
	}
	if s.Phase != "catan_transport_move" || s.Turn != actor {
		t.Fatal("discard did not resume knight/wagon")
	}
	raw, _ := json.Marshal(s)
	if err := s.Apply(actor, Action{Type: "catan_transport_stop", Offer: int(s.Catan.Transport.Sequence) + 1}); err == nil {
		t.Fatal("stale wagon sequence")
	}
	after, _ := json.Marshal(s)
	if string(raw) != string(after) {
		t.Fatal("rejected wagon mutated")
	}
	for step := 0; step < 30 && s.Turn == actor; step++ {
		s.AutoCatanPending()
	}
	if s.Turn == actor {
		t.Fatal("autoplay stuck")
	}
	if err := s.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
}
func TestCatanAttackTransportCityCorruptionAndOrdinaryIsolation(t *testing.T) {
	s := attackTransportCityStarted(t, 3)
	for _, bad := range []func(*Catan){func(g *Catan) { g.Transport.Knights = "" }, func(g *Catan) { g.Transport.Knights = CatanTransportKnightsRules }, func(g *Catan) { g.Attack.Gold = []int{0, 0, 0} }, func(g *Catan) { g.CitiesKnights.BarbarianPosition = 1 }, func(g *Catan) { g.Attack.City.Issued = 1 }} {
		copy := clone(*s)
		bad(copy.Catan)
		if copy.validateCatanTransport() == nil {
			t.Fatal("bad save accepted")
		}
	}
	before := clone(*s)
	if _, err := s.catanAttackCityLanding([2]int{0, 1}); err == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("invalid landing mutated")
	}
	for _, makeState := range []func(int) (*State, error){NewCatanTransport, NewCatanTransportCitiesKnights, NewCatanAttackCitiesKnights} {
		ordinary, err := makeState(3)
		if err != nil {
			t.Fatal(err)
		}
		if ordinary.Catan.AttackTransport != nil {
			t.Fatal("combined controller leaked")
		}
		if ordinary.Catan.Transport != nil && !slices.Contains(ordinary.Catan.Transport.Barbarians[:], ordinary.Catan.Transport.Map.Barbarians[0]) {
			t.Fatal("ordinary transport changed")
		}
	}
}

func TestCatanAttackTransportCityCargoProduction(t *testing.T) {
	for _, n := range []int{3, 6} {
		base := attackTransportCityStarted(t, n)
		for _, site := range base.Catan.Transport.Map.Sites {
			for _, level := range []int{1, 2} {
				s := clone(*base)
				g := s.Catan
				for i := range g.Vertices {
					g.Vertices[i].Owner = -1
					g.Vertices[i].Level = 0
				}
				for i := range g.Tiles {
					g.Tiles[i].Number = 0
				}
				for p := range g.Players {
					catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
				}
				for i := range g.AttackTransport.Pieces.Barbarians {
					g.AttackTransport.Pieces.Barbarians[i] = catanAttackTransportBarbarian{-1, -1, -1}
				}
				g.syncAttackTransportCounts()
				v := g.Tiles[site.Tile].Vertices[0]
				g.Vertices[v].Owner, g.Vertices[v].Level = 0, level
				g.Tiles[site.Tile].Number = 6
				want := make([]int, 8)
				switch site.Kind {
				case "glassworks":
					want[0] = 1
					if level == 2 {
						want[CatanPaper] = 1
					}
				case "castle":
					want[2] = 1
					if level == 2 {
						want[CatanCommodityCloth] = 1
					}
				}
				if err := s.catanRollProduction(6); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(g.Players[0].Resources, want) {
					t.Fatalf("n%d %s level%d: got%v want%v", n, site.Kind, level, g.Players[0].Resources, want)
				}
				if g.Tiles[site.Tile].Resource != catanTransportTerrain {
					t.Fatal("terrain changed")
				}
				catanMove(g.Players[0].Resources, g.Bank, slices.Clone(g.Players[0].Resources))
				for i := 0; i < 3; i++ {
					g.AttackTransport.Pieces.Barbarians[i] = catanAttackTransportBarbarian{site.Tile, -1, -1}
				}
				g.syncAttackTransportCounts()
				if err := s.catanRollProduction(6); err != nil {
					t.Fatal(err)
				}
				if sum(g.Players[0].Resources) != 0 {
					t.Fatal("conquered depot produced")
				}
			}
		}
	}
}

func TestCatanAttackTransportCityCardNumberLanding(t *testing.T) {
	for _, face := range []int{0, 3} {
		for _, production := range []int{2, 12} {
			s := attackTransportCityStarted(t, 3)
			g := s.Catan
			s.Phase = "catan_roll"
			g.RollID++
			before := sum(g.AttackTransport.Pieces.counts(g))
			// Event-card production is independent of the two ordinary dice (total 8).
			if err := s.catanStartCityDiceEvent(4, 4, face, production, false); err != nil {
				t.Fatal(err)
			}
			if got := sum(g.AttackTransport.Pieces.counts(g)); got != before+1 {
				t.Fatalf("face%d production%d: got%d want%d", face, production, got, before+1)
			}
			if g.CitiesKnights.BarbarianPosition != 0 {
				t.Fatal("sea barbarians moved")
			}
		}
	}
}

func TestCatanAttackTransportCityVictoryBeforeWagon(t *testing.T) {
	s := attackTransportCityStarted(t, 6)
	g := s.Catan
	actor := s.Turn
	tile := 18
	pieces := &g.AttackTransport.Pieces
	for i := range pieces.Barbarians {
		pieces.Barbarians[i] = catanAttackTransportBarbarian{-1, -1, -1}
	}
	g.syncAttackTransportCounts()
	s.catanScores()
	captives := (14-g.Players[actor].Score)*3 - 1
	if captives+1 > len(pieces.Barbarians) {
		t.Fatal("fixture lacks pieces")
	}
	for i := 0; i < captives; i++ {
		pieces.Barbarians[i].Captor = actor
	}
	edges := pieces.edges(g, tile, -1)
	pieces.Barbarians[captives] = catanAttackTransportBarbarian{tile, edges[0], -1}
	g.syncAttackTransportCounts()
	edge := catanFishingSide(g, tile, 0)
	g.Attack.City.Knights = []catanAttackCityKnight{{Owner: actor, Edge: edge, Strength: 2, Active: true}}
	s.catanScores()
	if g.Players[actor].Score != 13 {
		t.Fatal("fixture score", g.Players[actor].Score)
	}
	calls := 0
	result, err := s.catanAttackCityResolveEnd(nil, func() int { calls++; return 1 })
	if err != nil {
		t.Fatal(err)
	}
	if !s.Finished || s.Catan.Players[actor].Score != 14 || calls != 0 || s.Catan.Transport.Travel != nil || len(result.Battles) != 1 {
		t.Fatal("victory waited for casualties or wagon", calls, result)
	}
	if err := s.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanAttackTransportCityProgressPrivacyAndOwnership(t *testing.T) {
	s := attackTransportCityStarted(t, 3)
	actor := s.Turn
	ckProgressGive(t, s, actor, 12)
	for viewer := -1; viewer < 3; viewer++ {
		view := s.View(viewer)["catan"].(map[string]any)
		k := view["citiesKnights"].(map[string]any)
		if k["progressDecks"] != nil || k["event"] != nil {
			t.Fatal("private deck or event queue leaked")
		}
		for seat, raw := range k["players"].([]any) {
			hand := raw.(map[string]any)
			if (hand["progress"] != nil) != (seat == viewer) {
				t.Fatal("private progress hand", viewer, seat)
			}
		}
	}
	before := clone(*s)
	if err := s.Apply((actor+1)%3, Action{Type: "catan_progress", Card: 12, Tile: 0}); err == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("foreign progress action accepted or mutated state")
	}
	tiles := s.Catan.merchantTiles(actor)
	if len(tiles) == 0 {
		t.Fatal("no merchant target")
	}
	if err := s.Apply(actor, Action{Type: "catan_progress", Card: 12, Tile: tiles[0]}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.CitiesKnights.Merchant == nil || s.Catan.CitiesKnights.Merchant.Owner != actor {
		t.Fatal("merchant not applied")
	}
	if err := s.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanAttackTransportCityDeparture(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s := attackTransportCityStarted(t, n)
		actor := s.Turn
		ckProgressGive(t, s, actor, 12)
		before := sum(s.Catan.Transport.Gold) + s.Catan.Transport.GoldBank
		if err := s.EliminateCatan(actor); err != nil {
			t.Fatal(n, err)
		}
		g := s.Catan
		if !g.Players[actor].Eliminated || g.Transport.Gold[actor] != 0 || sum(g.Transport.Gold)+g.Transport.GoldBank != before || len(g.CitiesKnights.Players[actor].Progress) != 0 {
			t.Fatal("departure inventory", n)
		}
		if err := s.validateCatanTransport(); err != nil {
			t.Fatal(n, err)
		}
	}
}
