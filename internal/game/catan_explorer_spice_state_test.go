package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerSpiceStarted(t *testing.T, n int) *State {
	t.Helper()
	s, err := newCatanExplorerSpiceState(n)
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.Explorer.Setup != nil {
		actions := s.catanExplorerChoices(s.Turn)
		if len(actions) == 0 {
			t.Fatal("spice setup stalled")
		}
		if err = s.Apply(s.Turn, actions[0]); err != nil {
			t.Fatal(err)
		}
		s = explorerStateRestore(t, s)
	}
	if s.Catan.Explorer.Lairs != nil || s.Catan.Explorer.Spice == nil {
		t.Fatal("wrong mission components")
	}
	return s
}

func explorerSpiceApply(t *testing.T, s *State, a Action) {
	t.Helper()
	a.Prompt = int(s.Catan.TurnSerial)
	raw, err := json.Marshal(catanExplorerChoiceView([]Action{a})[0])
	if err != nil {
		t.Fatal(err)
	}
	var wire Action
	if err = json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(s.Turn, wire); err != nil {
		t.Fatal(a, err)
	}
	*s = *explorerStateRestore(t, s)
}

// Boundary fixtures below deliberately reveal tiles and position ships/crew.
// They verify State.Apply and restoration, not natural full-game bot strategy.
func explorerSpiceRevealed(t *testing.T) *State {
	t.Helper()
	s := explorerSpiceStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerFishRevealExcept(t, s, -1)
	return s
}
func explorerSpiceDock(t *testing.T, s *State, ship int, accepts func(CatanEdge) bool) {
	t.Helper()
	g, x := s.Catan, s.Catan.Explorer
	for _, e := range g.Edges {
		if catanExplorerSeaEdge(g, e.ID) && !slices.Contains(x.Fleet.Positions, e.ID) && accepts(e) {
			x.Fleet.Positions[ship] = e.ID
			return
		}
	}
	t.Fatal("no fixture berth")
}
func explorerSpiceClaimFixture(t *testing.T, s *State, player, tile, crew int, at catanExplorerCargoLocation) int {
	t.Helper()
	x := s.Catan.Explorer
	for id, sack := range x.Cargo.Spice {
		if sack.Origin == tile && sack.Owner == -1 {
			x.Cargo.Spice[id].Owner = player
			x.Cargo.Spice[id].At = at
			x.Cargo.Units[player*11+crew] = catanExplorerCargoLocation{"farm", tile}
			return id
		}
	}
	t.Fatal("no unclaimed sack")
	return -1
}

func TestCatanExplorerSpiceFormalSetupTurnsAndPublicChoices(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerSpiceStarted(t, n)
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

func TestCatanExplorerSpiceDiscoveryAtomicReward(t *testing.T) {
	s := explorerSpiceStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	g, x := s.Catan, s.Catan.Explorer
	// Explicit legal shuffle puts a farm on the outer coast for a one-edge voyage.
	from, to := -1, -1
	for i, h := range x.Board.Hidden {
		if h.Region == 0 && h.Farm == "swift" {
			from = i
		}
		if h.Tile == 4 {
			to = i
		}
	}
	a, b := x.Board.Hidden[from], x.Board.Hidden[to]
	a.Tile, b.Tile = b.Tile, a.Tile
	x.Board.Hidden[from], x.Board.Hidden[to] = b, a
	explorerFishRevealExcept(t, s, 4)
	start, end := -1, -1
	for _, a := range g.Edges {
		if !catanExplorerSeaEdge(g, a.ID) || catanExplorerTouches(g, a.ID, 4) || slices.Contains(x.Fleet.Positions, a.ID) {
			continue
		}
		for _, b := range g.Edges {
			if catanExplorerSeaEdge(g, b.ID) && catanExplorerAdjacentEdges(a, b) && catanExplorerTouches(g, b.ID, 4) && !slices.Contains(x.Fleet.Positions, b.ID) {
				start, end = a.ID, b.ID
				break
			}
		}
		if start >= 0 {
			break
		}
	}
	if start < 0 {
		t.Fatal("no farm approach")
	}
	ship := s.Turn * 3
	x.Fleet.Positions[ship] = start
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	g, x = s.Catan, s.Catan.Explorer
	action := Action{Type: "catan_explorer_sail", Slot: ship, Targets: []int{end}, Prompt: int(g.TurnSerial)}
	// Resource starvation must reject the complete sail/reveal transaction, not
	// partially reveal the farm or allocate sacks. Preview cannot reveal this.
	starved := clone(*s)
	q := starved.Catan.Explorer
	q.Economy.Gold[(s.Turn+1)%3] += q.Economy.GoldBank
	q.Economy.GoldBank = 0
	if err := starved.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(s.catanExplorerChoices(s.Turn))
	after, _ := json.Marshal(starved.catanExplorerChoices(s.Turn))
	if string(before) != string(after) {
		t.Fatal("reward shortage affected hidden destination preview")
	}
	explorerFishReject(t, &starved, s.Turn, action)
	gold, bank, numbers := x.Economy.Gold[s.Turn], x.Economy.GoldBank, clone(x.Board.Numbers)
	explorerSpiceApply(t, s, action)
	x = s.Catan.Explorer
	count := 0
	for _, sack := range x.Cargo.Spice {
		if sack.Origin == 4 {
			count++
		}
	}
	if count != 3 || x.Economy.Gold[s.Turn] != gold+2 || x.Economy.GoldBank != bank-2 || !reflect.DeepEqual(numbers, x.Board.Numbers) || !x.Fleet.Turn.Ships[ship].Closed {
		t.Fatal("farm discovery not atomic/complete")
	}
	explorerFishReject(t, s, s.Turn, action)
}

func TestCatanExplorerSpiceLandTransferDeliverWire(t *testing.T) {
	s := explorerSpiceRevealed(t)
	g, x := s.Catan, s.Catan.Explorer
	actor, ship := s.Turn, s.Turn*3
	x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Units[actor*11+2] = catanExplorerCargoLocation{"ship", ship}
	farm := -1
	for _, f := range x.Board.publicView().Farms {
		for _, e := range g.Edges {
			if catanExplorerSeaEdge(g, e.ID) && catanExplorerTouches(g, e.ID, f.Tile) && !slices.Contains(x.Fleet.Positions, e.ID) {
				farm = f.Tile
				x.Fleet.Positions[ship] = e.ID
				break
			}
		}
		if farm >= 0 {
			break
		}
	}
	if farm < 0 {
		t.Fatal("no accessible farm")
	}
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	var land Action
	for _, a := range s.catanExplorerChoices(actor) {
		if a.Type == "catan_explorer_spice_land" && a.Target == farm {
			land = a
			break
		}
	}
	if land.Type == "" {
		t.Fatal("no land preview")
	}
	explorerFishReject(t, s, (actor+1)%3, land)
	stale := land
	stale.Prompt++
	explorerFishReject(t, s, actor, stale)
	explorerSpiceApply(t, s, land)
	x = s.Catan.Explorer
	ids := x.Cargo.spiceContents(catanExplorerCargoLocation{"ship", ship})
	if len(ids) != 1 || x.Cargo.Units[actor*11+2] != (catanExplorerCargoLocation{"farm", farm}) {
		t.Fatal("land did not swap cargo")
	}
	sack := ids[0]
	explorerFishReject(t, s, actor, land)
	g = s.Catan
	harbor := -1
	for _, v := range g.Vertices {
		if v.Owner == actor && v.Level == 2 {
			harbor = v.ID
			break
		}
	}
	explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return e.A == harbor || e.B == harbor })
	x.Cargo.Units[actor*11+3] = catanExplorerCargoLocation{"harbor", harbor}
	var swap Action
	for _, a := range s.catanExplorerChoices(actor) {
		if a.Type == "catan_explorer_transfer" && slices.Equal(a.Give, []int{actor*11 + 3}) && slices.Equal(a.SpiceUnload, []int{sack}) {
			swap = a
			break
		}
	}
	if swap.Type == "" {
		t.Fatal("missing mixed freight swap")
	}
	explorerSpiceApply(t, s, swap)
	if s.Catan.Explorer.Cargo.Spice[sack].At != (catanExplorerCargoLocation{"harbor", harbor}) {
		t.Fatal("wire dropped spice unload")
	}
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_transfer", Slot: ship, Vertex: harbor, Take: swap.Give, SpiceLoad: []int{sack}})
	anchor := s.Catan.Explorer.Board.Council.Anchors[0]
	explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return e.A == anchor || e.B == anchor })
	var deliver Action
	for _, a := range s.catanExplorerChoices(actor) {
		if a.Type == "catan_explorer_spice_deliver" && a.Card == sack {
			deliver = a
			break
		}
	}
	if deliver.Type == "" {
		t.Fatal("missing deliver preview")
	}
	explorerSpiceApply(t, s, deliver)
	x = s.Catan.Explorer
	if x.Spice.publicView(s.Catan).Progress[actor] != 1 || s.Catan.Players[actor].Score != 5 {
		t.Fatal("wrong task score")
	}
	explorerFishReject(t, s, actor, deliver)
}

func TestCatanExplorerSpiceAbilitiesPersistAcrossTurns(t *testing.T) {
	s := explorerSpiceRevealed(t)
	actor := s.Turn
	x := s.Catan.Explorer
	crew := 2
	for _, farm := range x.Board.publicView().Farms {
		explorerSpiceClaimFixture(t, s, actor, farm.Tile, crew, catanExplorerCargoLocation{"supply", -1})
		crew++
	}
	// Six claimed/lost sacks retain all abilities; no delivery points invented.
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		g := s.Catan
		x = g.Explorer
		g.Bank[0] -= 3
		g.Players[actor].Resources[0] += 3
		for i := 0; i < 2; i++ {
			if !slices.ContainsFunc(s.catanExplorerChoices(actor), func(a Action) bool { return a.Type == "catan_explorer_spice_gold" && a.Card == 0 }) {
				t.Fatal("gold preview missing")
			}
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_spice_gold", Card: 0})
		}
		explorerFishReject(t, s, actor, Action{Type: "catan_explorer_spice_gold", Card: 0, Prompt: int(s.Catan.TurnSerial)})
		explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
		x = s.Catan.Explorer
		if x.Fleet.Turn.Speed != 2 {
			t.Fatal("swift ability lost at new movement")
		}
		explorerFishReject(t, s, actor, Action{Type: "catan_explorer_spice_gold", Card: 0, Prompt: int(s.Catan.TurnSerial)})
		for i := 0; i < 3; i++ {
			explorerPassOrdinaryTurn(t, s)
		}
		if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
			t.Fatal(err)
		}
		*s = *explorerStateRestore(t, s)
	}
}

func TestCatanExplorerSpiceMandatoryPirateWithoutLairs(t *testing.T) {
	s := explorerSpiceStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{3, 4}); err != nil {
		t.Fatal(err)
	}
	steps := 0
	for s.Phase != "catan_turn" {
		if steps > 8 {
			t.Fatal("mandatory response stalled")
		}
		steps++
		actor := s.CatanPendingActor()
		if s.Phase == "catan_discard" {
			for p, n := range s.Catan.DiscardDue {
				if n > 0 {
					actor = p
					break
				}
			}
		} else {
			for _, kind := range []string{"catan_explorer_begin_move", "catan_explorer_spice_gold", "catan_explorer_fish_roll", "catan_end"} {
				explorerFishReject(t, s, s.Turn, Action{Type: kind, Prompt: int(s.Catan.TurnSerial)})
			}
		}
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(actor, a); err != nil {
			t.Fatal(err)
		}
		*s = *explorerStateRestore(t, s)
	}
	if steps == 0 {
		t.Fatal("seven skipped mandatory response")
	}
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	explorerFishReject(t, s, s.Turn, Action{Type: "catan_explorer_land", Prompt: int(s.Catan.TurnSerial)})
	explorerSpiceApply(t, s, Action{Type: "catan_end"})
}

func TestCatanExplorerSpiceDualMissionImmediateVictory(t *testing.T) {
	for _, kind := range []string{"spice", "fish"} {
		t.Run(kind, func(t *testing.T) {
			s := explorerSpiceRevealed(t)
			actor, ship := s.Turn, s.Turn*3
			x := s.Catan.Explorer
			sacks := []int{}
			for i, farm := range x.Board.publicView().Farms {
				sacks = append(sacks, explorerSpiceClaimFixture(t, s, actor, farm.Tile, i+2, catanExplorerCargoLocation{"supply", -1}))
			}
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
			g, x := s.Catan, s.Catan.Explorer
			for _, id := range sacks[:2] {
				x.Spice.Deliveries = append(x.Spice.Deliveries, catanExplorerSpiceDelivery{actor, g.TurnSerial, id})
			}
			x.Fish.Deliveries = []catanExplorerFishDelivery{{Player: actor, Sequence: g.TurnSerial, Fish: 0}, {Player: actor, Sequence: g.TurnSerial, Fish: 0}}
			s.catanExplorerMissionScore()
			explorerVictoryBuildings(t, s, actor, 10)
			if g.Players[actor].Score != 14 {
				t.Fatal("dual mission fixture score")
			}
			x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
			anchor := x.Board.Council.Anchors[0]
			explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return e.A == anchor || e.B == anchor })
			card := sacks[2]
			if kind == "spice" {
				x.Cargo.Spice[card].At = catanExplorerCargoLocation{"ship", ship}
			} else {
				card = 0
				x.Cargo.Fish[0] = catanExplorerCargoLocation{"ship", ship}
			}
			serial := g.TurnSerial
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_" + kind + "_deliver", Slot: ship, Card: card})
			if !s.Finished || s.Catan.Players[actor].Score != 15 || !slices.Equal(s.Winners, []int{actor}) || s.Catan.TurnSerial != serial {
				t.Fatal("delivery did not win immediately", s.Catan.Players[actor].Score)
			}
			if len(s.catanExplorerChoices(actor)) != 0 || s.CatanPendingActor() != -1 {
				t.Fatal("finished game kept responding")
			}
			explorerFishReject(t, s, actor, Action{Type: "catan_end", Prompt: int(serial)})
		})
	}
}

func TestCatanExplorerSpiceDeparturePreservesHistoryAndContinues(t *testing.T) {
	s := explorerSpiceRevealed(t)
	actor, other := s.Turn, (s.Turn+1)%3
	x := s.Catan.Explorer
	farms := x.Board.publicView().Farms
	ids := []int{}
	for i, farm := range farms {
		ids = append(ids, explorerSpiceClaimFixture(t, s, actor, farm.Tile, i+2, catanExplorerCargoLocation{"supply", -1}))
	}
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	x = s.Catan.Explorer
	x.Spice.Deliveries = []catanExplorerSpiceDelivery{{actor, s.Catan.TurnSerial, ids[0]}}
	s.catanExplorerMissionScore()
	explorerPassOrdinaryTurn(t, s)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	otherSack := explorerSpiceClaimFixture(t, s, other, farms[0].Tile, 2, catanExplorerCargoLocation{"supply", -1})
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	x = s.Catan.Explorer
	x.Spice.Deliveries = append(x.Spice.Deliveries, catanExplorerSpiceDelivery{other, s.Catan.TurnSerial, otherSack})
	s.catanExplorerMissionScore()
	explorerPassOrdinaryTurn(t, s)
	explorerPassOrdinaryTurn(t, s)
	g, x := s.Catan, s.Catan.Explorer
	if s.Turn != actor || x.Spice.publicView(g).Leader != actor {
		t.Fatal("tie leader changed too early")
	}
	x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Spice[ids[1]].At = catanExplorerCargoLocation{"ship", actor * 3}
	for _, v := range g.Vertices {
		if v.Owner == actor && v.Level == 2 {
			x.Cargo.Spice[ids[2]].At = catanExplorerCargoLocation{"harbor", v.ID}
			break
		}
	}
	history := clone(x.Spice.Deliveries)
	beforeUnowned := []catanExplorerSpiceSack{}
	for _, sack := range x.Cargo.Spice {
		if sack.Owner < 0 {
			beforeUnowned = append(beforeUnowned, sack)
		}
	}
	if err := s.EliminateCatan(actor); err != nil {
		t.Fatal(err)
	}
	*s = *explorerStateRestore(t, s)
	x = s.Catan.Explorer
	if !reflect.DeepEqual(history, x.Spice.Deliveries) || x.Spice.publicView(s.Catan).Leader != other {
		t.Fatal("departure lost history/leader")
	}
	afterUnowned := []catanExplorerSpiceSack{}
	for _, sack := range x.Cargo.Spice {
		if sack.Owner == actor && sack.At != (catanExplorerCargoLocation{"supply", -1}) {
			t.Fatal("departed sack not returned")
		}
		if sack.Owner < 0 {
			afterUnowned = append(afterUnowned, sack)
		}
	}
	if !reflect.DeepEqual(beforeUnowned, afterUnowned) {
		t.Fatal("departure changed unclaimed farm stock")
	}
	for i := 0; i < 4; i++ {
		explorerPassOrdinaryTurn(t, s)
		*s = *explorerStateRestore(t, s)
	}
}

func TestCatanExplorerSpicePrivacyAndCorruptState(t *testing.T) {
	s := explorerSpiceStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	other := clone(*s)
	x := other.Catan.Explorer
	ids := []int{}
	for i, h := range x.Board.Hidden {
		if h.Region == 0 {
			ids = append(ids, i)
		}
	}
	for i := 0; i < len(ids)/2; i++ {
		a, b := ids[i], ids[len(ids)-1-i]
		ha, hb := x.Board.Hidden[a], x.Board.Hidden[b]
		ha.Tile, hb.Tile = hb.Tile, ha.Tile
		x.Board.Hidden[a], x.Board.Hidden[b] = hb, ha
	}
	if err := other.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	for viewer := -1; viewer < 3; viewer++ {
		if !reflect.DeepEqual(s.View(viewer), other.View(viewer)) {
			t.Fatal("hidden farms affected public view", viewer)
		}
	}
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Explorer.Spice = nil },
		func(s *State) { s.Catan.Explorer.Fish = nil },
		func(s *State) { s.Catan.Explorer.Pirate = nil },
		func(s *State) { s.Catan.Explorer.Lairs, _ = newCatanExplorerLairs(3, []int{2, 3, 4, 5, 6, 8}) },
		func(s *State) { s.Catan.Explorer.Spice.Deliveries = []catanExplorerSpiceDelivery{{99, 1, 0}} },
		func(s *State) { s.Catan.Explorer.Spice.Deliveries = []catanExplorerSpiceDelivery{{s.Turn, 1, 24}} },
		func(s *State) { s.Catan.Explorer.Fleet.Turn.Speed = 2 },
		func(s *State) { s.Catan.Players[s.Turn].Score++ },
	} {
		bad := clone(*s)
		mutate(&bad)
		if err := bad.validateCatanExplorer(); err == nil {
			t.Fatal("corrupt spice state accepted")
		}
	}
}

func TestCatanExplorerSpicePiratePlacementClearsOnlyShoalAndBlocksFishing(t *testing.T) {
	s := explorerSpiceStarted(t, 3)
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
