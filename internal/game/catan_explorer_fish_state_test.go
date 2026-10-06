package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerFishStarted(t *testing.T, n int) *State {
	t.Helper()
	// Artificial six lair numbers, explicitly private acceptance inventory.
	s, err := newCatanExplorerFishState(n, []int{2, 3, 4, 5, 6, 8})
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.Explorer.Setup != nil {
		actions := s.catanExplorerChoices(s.Turn)
		if len(actions) == 0 {
			t.Fatal("fish setup stalled")
		}
		if err = s.Apply(s.Turn, actions[0]); err != nil {
			t.Fatal(err)
		}
		s = explorerStateRestore(t, s)
	}
	return s
}
func explorerFishRevealExcept(t *testing.T, s *State, except int) {
	t.Helper()
	g, x := s.Catan, s.Catan.Explorer
	for _, h := range slices.Clone(x.Board.Hidden) {
		if h.Tile != except && !h.Revealed {
			if _, err := x.discover(g, s.Turn, []int{h.Tile}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
}
func explorerFishApply(t *testing.T, s *State, a Action) {
	t.Helper()
	a.Prompt = int(s.Catan.TurnSerial)
	beforeFish := slices.Clone(s.Catan.Explorer.Cargo.Fish)
	if err := s.Apply(s.Turn, a); err != nil {
		t.Fatal(a, err)
	}
	assertExplorerFishMotion(t, beforeFish, s, a)
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
}
func explorerFishReject(t *testing.T, s *State, player int, a Action) {
	t.Helper()
	before, _ := json.Marshal(s)
	if err := s.Apply(player, a); err == nil {
		t.Fatal("accepted invalid fish action", a)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("failed fish action mutated full state")
	}
}
func TestCatanExplorerFishFormalSetupTurnsAndPublicChoices(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerFishStarted(t, n)
			g, x := s.Catan, s.Catan.Explorer
			if x.Board.Target != 15 || len(x.Cargo.Fish) != 6 || x.Setup != nil || g.TurnSerial != 1 || s.Phase != "catan_roll" {
				t.Fatal("fish scenario did not finish official setup")
			}
			for _, p := range g.Players {
				if p.Score != 3 {
					t.Fatal("wrong initial score")
				}
			}
			start := s.Turn
			// Play two rounds using normal Apply; mandatory seven responses use the
			// existing response bot. No resources/pieces/scores are injected here.
			for turns := 0; turns < n*2; turns++ {
				serial := s.Catan.TurnSerial
				explorerFishApply(t, s, Action{Type: "catan_roll"})
				for s.Phase != "catan_turn" {
					p := s.CatanPendingActor()
					if s.Phase == "catan_discard" {
						for i, due := range s.Catan.DiscardDue {
							if due > 0 {
								p = i
								break
							}
						}
					}
					if p < 0 {
						t.Fatal("cannot answer", s.Phase)
					}
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(err)
					}
				}
				explorerFishReject(t, s, s.Turn, Action{Type: "catan_explorer_fish_roll", Prompt: int(serial)})
				explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
				action := Action{Type: "catan_explorer_fish_roll", Prompt: int(serial)}
				explorerFishReject(t, s, (s.Turn+1)%n, action)
				if !slices.ContainsFunc(s.catanExplorerChoices(s.Turn), func(a Action) bool { return a.Type == action.Type }) {
					t.Fatal("no fish roll preview")
				}
				if err := s.Apply(s.Turn, action); err != nil {
					t.Fatal(err)
				}
				if s.Catan.Explorer.Fish.LastRoll == nil || s.Catan.Explorer.Fish.LastRoll.Spawned != -1 {
					t.Fatal("unexplored shoal spawned fish")
				}
				s = explorerStateRestore(t, s)
				explorerFishReject(t, s, s.Turn, action)
				if slices.ContainsFunc(s.catanExplorerChoices(s.Turn), func(a Action) bool { return a.Type == action.Type }) {
					t.Fatal("roll still offered")
				}
				for p := -1; p < n; p++ {
					view := s.View(p)["catan"].(map[string]any)["explorer"].(map[string]any)
					if p != s.Turn && len(view["choices"].([]map[string]any)) > 0 {
						t.Fatal("fish controls leaked")
					}
					if len(view["board"].(catanExplorerBoardView).Shoals) > 0 {
						t.Fatal("unexplored fish leaked")
					}
				}
				explorerFishApply(t, s, Action{Type: "catan_end"})
				if s.Catan.TurnSerial != serial+1 {
					t.Fatal("fish roll changed turn boundary")
				}
			}
			if s.Turn != start || s.Round != 3 {
				t.Fatal("fish turn rotation incorrect")
			}
		})
	}
}
func TestCatanExplorerFishSailingDiscoveryGoldAndFiveLairs(t *testing.T) {
	s := explorerFishStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	g, x := s.Catan, s.Catan.Explorer
	target, face := -1, 0
	for _, h := range x.Board.Hidden {
		if h.Fish > 0 {
			target, face = h.Tile, h.Fish
			break
		}
	}
	explorerFishRevealExcept(t, s, target)
	if len(x.Lairs.Sites) != 5 || len(x.Lairs.Deck) != 1 {
		t.Fatal("fish scenario must discover five gold fields, leaving one lair token")
	}
	from, to := -1, -1
	for _, a := range g.Edges {
		if !catanExplorerSeaEdge(g, a.ID) || catanExplorerTouches(g, a.ID, target) {
			continue
		}
		for _, b := range g.Edges {
			if catanExplorerSeaEdge(g, b.ID) && catanExplorerAdjacentEdges(a, b) && catanExplorerTouches(g, b.ID, target) && !slices.Contains(x.Fleet.Positions, b.ID) {
				from, to = a.ID, b.ID
				break
			}
		}
		if from >= 0 && !slices.Contains(x.Fleet.Positions, from) {
			break
		}
		from = -1
	}
	if from < 0 {
		t.Fatal("no approach to fish shoal")
	}
	ship := s.Turn * 3
	x.Fleet.Positions[ship] = from
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
	gold := x.Economy.Gold[s.Turn]
	bank := x.Economy.GoldBank
	numbers := clone(x.Board.Numbers)
	explorerFishApply(t, s, Action{Type: "catan_explorer_sail", Slot: ship, Targets: []int{to}})
	x = s.Catan.Explorer
	if x.Economy.Gold[s.Turn] != gold+2 || x.Economy.GoldBank != bank-2 || !reflect.DeepEqual(numbers, x.Board.Numbers) || !x.Fleet.Turn.Ships[ship].Closed {
		t.Fatal("shoal discovery reward/mandatory stop")
	}
	if !slices.ContainsFunc(x.Board.publicView().Shoals, func(h catanExplorerShoal) bool { return h.Tile == target && h.Number == face }) {
		t.Fatal("discovered fish die face missing")
	}
	explorerStateRestore(t, s)
}
func TestCatanExplorerFishStateLoadTransferAndDeliverPreviews(t *testing.T) {
	s := explorerFishStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerFishRevealExcept(t, s, -1)
	g, x := s.Catan, s.Catan.Explorer
	ship := s.Turn * 3
	shoal := x.Board.publicView().Shoals[0]
	for _, e := range g.Edges {
		if catanExplorerSeaEdge(g, e.ID) && catanExplorerTouches(g, e.ID, shoal.Tile) && !slices.Contains(x.Fleet.Positions, e.ID) {
			x.Fleet.Positions[ship] = e.ID
			break
		}
	}
	x.Cargo.Units[s.Turn*11] = catanExplorerCargoLocation{"supply", -1}
	explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
	g, x = s.Catan, s.Catan.Explorer
	// Trusted die source exercises real spawning; user-facing actions never
	// accept a client-supplied die. Loading then goes through State.Apply.
	if err := x.Fish.apply(g, x.Board, x.Fleet, x.Cargo, s.Turn, g.TurnSerial, "roll", 0, 0, -1, func(int) int { return shoal.Number - 1 }); err != nil {
		t.Fatal(err)
	}
	applyPreview := func(kind string) {
		t.Helper()
		for _, a := range s.catanExplorerChoices(s.Turn) {
			if a.Type != kind {
				continue
			}
			raw, _ := json.Marshal(catanExplorerChoiceView([]Action{a})[0])
			var wire Action
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			beforeFish := slices.Clone(s.Catan.Explorer.Cargo.Fish)
			if err := s.Apply(s.Turn, wire); err != nil {
				t.Fatal("preview cannot execute", a, err)
			}
			assertExplorerFishMotion(t, beforeFish, s, wire)
			return
		}
		t.Fatal("missing preview", kind)
	}
	applyPreview("catan_explorer_fish_load")
	g, x = s.Catan, s.Catan.Explorer
	if x.Cargo.Fish[0] != (catanExplorerCargoLocation{"ship", ship}) {
		t.Fatal("fish not loaded")
	}
	// Explicit berth fixture verifies wire projection of mixed-unit exchanges.
	harbor := -1
	for _, v := range g.Vertices {
		if v.Owner == s.Turn && v.Level == 2 {
			harbor = v.ID
			break
		}
	}
	for _, e := range g.Edges {
		if (e.A == harbor || e.B == harbor) && catanExplorerSeaEdge(g, e.ID) {
			x.Fleet.Positions[ship] = e.ID
			break
		}
	}
	x.Cargo.Units[s.Turn*11+2], x.Cargo.Units[s.Turn*11+3] = catanExplorerCargoLocation{"harbor", harbor}, catanExplorerCargoLocation{"harbor", harbor}
	var swap Action
	for _, a := range s.catanExplorerChoices(s.Turn) {
		if a.Type == "catan_explorer_transfer" && len(a.Give) == 2 && slices.Equal(a.Targets, []int{0}) {
			swap = a
			break
		}
	}
	if swap.Type == "" {
		t.Fatal("full fish/crew swap absent")
	}
	raw, _ := json.Marshal(catanExplorerChoiceView([]Action{swap})[0])
	var wire Action
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	beforeFish := slices.Clone(s.Catan.Explorer.Cargo.Fish)
	if err := s.Apply(s.Turn, wire); err != nil {
		t.Fatal(err)
	}
	assertExplorerFishMotion(t, beforeFish, s, wire)
	if s.Catan.Explorer.Cargo.Fish[0] != (catanExplorerCargoLocation{"harbor", harbor}) {
		t.Fatal("wire lost fish field")
	}
	// Swap back and dock at the actual north council anchor.
	explorerFishApply(t, s, Action{Type: "catan_explorer_transfer", Slot: ship, Vertex: harbor, Take: swap.Give, Cards: []int{0}})
	g, x = s.Catan, s.Catan.Explorer
	anchor := x.Board.Council.Anchors[0]
	for _, e := range g.Edges {
		if (e.A == anchor || e.B == anchor) && catanExplorerSeaEdge(g, e.ID) {
			x.Fleet.Positions[ship] = e.ID
			break
		}
	}
	before := g.Players[s.Turn].Score
	applyPreview("catan_explorer_fish_deliver")
	if s.Catan.Players[s.Turn].Score != before+2 || len(s.Catan.Explorer.Fish.Deliveries) != 1 {
		t.Fatal("fish score missing from total")
	}
	explorerStateRestore(t, s)
}

func TestCatanExplorerFishPiratePlacementClearsOnlyShoalAndBlocksFishing(t *testing.T) {
	s := explorerFishStarted(t, 3)
	explorerFishRevealExcept(t, s, -1)
	g, x := s.Catan, s.Catan.Explorer
	actor, other := s.Turn, (s.Turn+1)%3
	shoal := x.Board.publicView().Shoals[0]
	x.Cargo.Fish[0] = catanExplorerCargoLocation{"shoal", shoal.Tile}
	for _, e := range g.Edges {
		if catanExplorerSeaEdge(g, e.ID) && slices.Contains(e.Tiles, shoal.Tile) {
			x.Fleet.Positions[other*3] = e.ID
			break
		}
	}
	x.Cargo.Units[other*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Fish[1] = catanExplorerCargoLocation{"ship", other * 3}
	// Grant one resource to make the subsequent theft response unavoidable.
	g.Bank[0]--
	g.Players[other].Resources[0]++
	if err := s.catanExplorerRoll([2]int{3, 4}); err != nil {
		t.Fatal(err)
	}
	for s.Phase == "catan_discard" {
		p := -1
		for i, due := range s.Catan.DiscardDue {
			if due > 0 {
				p = i
				break
			}
		}
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(p, a); err != nil {
			t.Fatal(err)
		}
	}
	s = explorerStateRestore(t, s)
	if s.Phase != "catan_explorer_pirate_place" {
		t.Fatal("seven skipped pirate")
	}
	explorerFishReject(t, s, actor, Action{Type: "catan_explorer_fish_roll", Prompt: int(s.Catan.TurnSerial)})
	explorerFishApply(t, s, Action{Type: "catan_explorer_pirate_place", Target: shoal.Tile})
	x = s.Catan.Explorer
	if x.Cargo.Fish[0] != (catanExplorerCargoLocation{"supply", -1}) || x.Cargo.Fish[1] != (catanExplorerCargoLocation{"ship", other * 3}) || s.Phase != "catan_explorer_pirate_steal" {
		t.Fatal("pirate did not atomically clear only shoal fish")
	}
	s = explorerStateRestore(t, s)
	explorerFishReject(t, s, actor, Action{Type: "catan_explorer_fish_load", Slot: actor * 3, Card: 1, Prompt: int(s.Catan.TurnSerial)})
	explorerFishApply(t, s, Action{Type: "catan_explorer_pirate_steal", Target: other})
	explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
	g, x = s.Catan, s.Catan.Explorer
	if err := x.Fish.apply(g, x.Board, x.Fleet, x.Cargo, actor, g.TurnSerial, "roll", 0, 0, x.Pirate.Tile, func(int) int { return shoal.Number - 1 }); err != nil {
		t.Fatal(err)
	}
	if x.Fish.LastRoll.Spawned != -1 {
		t.Fatal("pirate shoal spawned fish")
	}
	explorerStateRestore(t, s)
	bad := clone(*s)
	bad.Catan.Explorer.Cargo.Fish[0] = catanExplorerCargoLocation{"shoal", shoal.Tile}
	if err := bad.validateCatanExplorer(); err == nil {
		t.Fatal("pirate with remaining shoal fish accepted")
	}
}

func explorerFishVictoryFixture(t *testing.T) *State {
	t.Helper()
	s := explorerFishStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerFishRevealExcept(t, s, -1)
	explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
	x := s.Catan.Explorer
	x.Fish.Deliveries = []catanExplorerFishDelivery{{Player: s.Turn, Sequence: s.Catan.TurnSerial, Fish: 0}, {Player: s.Turn, Sequence: s.Catan.TurnSerial, Fish: 0}}
	s.catanExplorerMissionScore()
	return s
}
func TestCatanExplorerFishDeliveryImmediateFifteenPointVictory(t *testing.T) {
	s := explorerFishVictoryFixture(t)
	explorerVictoryBuildings(t, s, s.Turn, 12)
	g, x := s.Catan, s.Catan.Explorer
	actor, serial := s.Turn, g.TurnSerial
	ship := actor * 3
	anchor := x.Board.Council.Anchors[0]
	for _, e := range g.Edges {
		if (e.A == anchor || e.B == anchor) && catanExplorerSeaEdge(g, e.ID) && !slices.Contains(x.Fleet.Positions, e.ID) {
			x.Fleet.Positions[ship] = e.ID
			break
		}
	}
	x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Fish[0] = catanExplorerCargoLocation{"ship", ship}
	if g.Players[actor].Score != 14 {
		t.Fatal("wrong fixture total")
	}
	explorerFishApply(t, s, Action{Type: "catan_explorer_fish_deliver", Slot: ship, Card: 0})
	s = explorerStateRestore(t, s)
	if !s.Finished || !slices.Equal(s.Winners, []int{actor}) || s.Catan.Players[actor].Score != 15 || s.Catan.TurnSerial != serial || s.Catan.Explorer.Cargo.Fish[0] != (catanExplorerCargoLocation{"supply", -1}) {
		t.Fatal("delivery did not immediately win")
	}
	explorerFishReject(t, s, actor, Action{Type: "catan_end", Prompt: int(serial)})
	if s.CatanPendingActor() != -1 || len(s.catanExplorerChoices(actor)) != 0 {
		t.Fatal("finished fish game continued responding")
	}
}
func TestCatanExplorerFishPointsCountBeforeLairHeroVictory(t *testing.T) {
	s := explorerFishVictoryFixture(t)
	explorerVictoryBuildings(t, s, s.Turn, 11)
	g, x := s.Catan, s.Catan.Explorer
	actor, serial := s.Turn, g.TurnSerial
	tiles := []int{x.Lairs.Sites[0].Tile, x.Lairs.Sites[1].Tile}
	for i, tile := range tiles {
		x.Lairs.Sites[i].Ready, x.Lairs.Sites[i].Captor = serial, actor
		for j := 0; j < 3; j++ {
			x.Cargo.Units[actor*11+2+i*3+j] = catanExplorerCargoLocation{"lair", tile}
		}
	}
	if g.Players[actor].Score != 13 {
		t.Fatal("wrong combined pre-victory total")
	}
	explorerFishApply(t, s, Action{Type: "catan_end"})
	explorerFishApply(t, s, Action{Type: "catan_explorer_resolve", Target: tiles[0]})
	s = explorerStateRestore(t, s)
	x = s.Catan.Explorer
	if !s.Finished || s.Catan.Players[actor].Score != 15 || x.Lairs.RewardVictory == nil || x.Lairs.Battle != nil || x.Lairs.Progress[actor] != 1 {
		t.Fatal("lair ignored prior fish points and ran hero battle")
	}
	for _, tile := range tiles {
		site := x.Lairs.Sites[x.Lairs.site(tile)]
		if site.Resolved != 0 || site.Hero != -1 || s.Catan.Tiles[tile].Number != 0 {
			t.Fatal("post-victory lair resolved")
		}
	}
}

func TestCatanExplorerFishDepartureReturnsCargoAndTransfersLeadership(t *testing.T) {
	s := explorerFishVictoryFixture(t)
	actor, other := s.Turn, (s.Turn+1)%3
	// Give the next player an earlier legitimate turn for a separate delivery;
	// the current player already has two completed deliveries from turn one.
	explorerPassOrdinaryTurn(t, s)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
	x := s.Catan.Explorer
	x.Fish.Deliveries = append(x.Fish.Deliveries, catanExplorerFishDelivery{Player: other, Sequence: s.Catan.TurnSerial, Fish: 0})
	s.catanExplorerMissionScore()
	explorerPassOrdinaryTurn(t, s)
	explorerPassOrdinaryTurn(t, s)
	if s.Turn != actor {
		t.Fatal("wrong departure turn")
	}
	g := s.Catan
	x = g.Explorer
	ship := actor * 3
	x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Fish[0] = catanExplorerCargoLocation{"ship", ship}
	for _, v := range g.Vertices {
		if v.Owner == actor && v.Level == 2 {
			x.Cargo.Fish[1] = catanExplorerCargoLocation{"harbor", v.ID}
			break
		}
	}
	shoal := x.Board.publicView().Shoals[0]
	x.Cargo.Fish[2] = catanExplorerCargoLocation{"shoal", shoal.Tile}
	history := clone(x.Fish.Deliveries)
	if err := s.EliminateCatan(actor); err != nil {
		t.Fatal(err)
	}
	s = explorerStateRestore(t, s)
	x = s.Catan.Explorer
	if !x.Fish.Retired[actor] || x.Fish.publicView(3).Leader != other || !reflect.DeepEqual(history, x.Fish.Deliveries) {
		t.Fatal("fish history or leader not preserved")
	}
	for _, id := range []int{0, 1} {
		if x.Cargo.Fish[id] != (catanExplorerCargoLocation{"supply", -1}) {
			t.Fatal("departed cargo not returned")
		}
	}
	if x.Cargo.Fish[2] != (catanExplorerCargoLocation{"shoal", shoal.Tile}) {
		t.Fatal("unowned shoal fish removed")
	}
	for i := 0; i < 4; i++ {
		explorerPassOrdinaryTurn(t, s)
		s = explorerStateRestore(t, s)
	}
	bad := clone(*s)
	bad.Catan.Explorer.Fish.Retired = nil
	if err := bad.validateCatanExplorer(); err == nil {
		t.Fatal("missing retirement history accepted")
	}
}

func TestCatanExplorerFishStatePrivacyAndCorruptSaveRejection(t *testing.T) {
	s := explorerFishStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
	other := clone(*s)
	x := other.Catan.Explorer
	// Permute complete face-up components within one hidden region, including
	// their printed fish values. Visible terrain, hands and remaining counts stay.
	ids := []int{}
	for i, h := range x.Board.Hidden {
		if h.Region == 0 {
			ids = append(ids, i)
		}
	}
	for i := 0; i < len(ids)/2; i++ {
		a, b := ids[i], ids[len(ids)-1-i]
		x.Board.Hidden[a].Resource, x.Board.Hidden[b].Resource = x.Board.Hidden[b].Resource, x.Board.Hidden[a].Resource
		x.Board.Hidden[a].Fish, x.Board.Hidden[b].Fish = x.Board.Hidden[b].Fish, x.Board.Hidden[a].Fish
	}
	slices.Reverse(x.Board.Numbers[0])
	slices.Reverse(x.Lairs.Deck)
	if err := other.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	for p := -1; p < 3; p++ {
		if !reflect.DeepEqual(s.View(p), other.View(p)) {
			t.Fatal("secret fish components affected visible state", p)
		}
	}
	a, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	b, err := other.BotAction(other.Turn)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("existing exploration bot read hidden fish faces")
	}
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Explorer.Fish = nil },
		func(s *State) {
			s.Catan.Explorer.Fish.LastRoll = &catanExplorerFishRoll{Player: s.Turn, Sequence: s.Catan.TurnSerial + 1, Die: 6, Spawned: -1}
		},
		func(s *State) {
			s.Catan.Explorer.Fish.Deliveries = []catanExplorerFishDelivery{{Player: 99, Sequence: 1, Fish: 0}}
		},
		func(s *State) {
			s.Catan.Explorer.Fish.Deliveries = []catanExplorerFishDelivery{{Player: s.Turn, Sequence: 1, Fish: 6}}
		},
		func(s *State) { s.Catan.Explorer.Fish.Retired = []bool{false} },
		func(s *State) {
			s.Catan.Explorer.Fish.Deliveries = []catanExplorerFishDelivery{{Player: s.Turn, Sequence: 1, Fish: 0}}
		}, // Score not updated.
	} {
		bad := clone(*s)
		mutate(&bad)
		if err := bad.validateCatanExplorer(); err == nil {
			t.Fatal("corrupt fish state accepted")
		}
	}
	// Reject a forged victory with a malformed fish history before calculating
	// combined mission scores; this used to be a possible indexing panic.
	win := explorerFishVictoryFixture(t)
	explorerVictoryBuildings(t, win, win.Turn, 11)
	g := win.Catan
	x = g.Explorer
	site := &x.Lairs.Sites[0]
	site.Ready, site.Captor = g.TurnSerial, win.Turn
	for j := 0; j < 3; j++ {
		x.Cargo.Units[win.Turn*11+2+j] = catanExplorerCargoLocation{"lair", site.Tile}
	}
	tile := site.Tile
	explorerFishApply(t, win, Action{Type: "catan_end"})
	explorerFishApply(t, win, Action{Type: "catan_explorer_resolve", Target: tile})
	if !win.Finished {
		t.Fatal("fixture did not reach victory")
	}
	win.Catan.Explorer.Fish.Deliveries[0].Player = 99
	if err := win.validateCatanExplorer(); err == nil {
		t.Fatal("invalid fish scorer accepted in reward victory")
	}
}
