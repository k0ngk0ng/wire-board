package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerChoicesAreExecutablePrivateAndReadOnly(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	actor := s.Turn
	if err = s.catanExplorerRoll([2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	// Resource-rich controlled fixture, not a natural game. Each advertised
	// construction/bank action must independently execute through real Apply.
	explorerFlowResources(s, map[int][]int{actor: {4, 4, 4, 4, 4}})
	before, _ := json.Marshal(s)
	choices := s.catanExplorerChoices(actor)
	kinds := map[string]int{}
	for _, a := range choices {
		kinds[a.Type]++
		next := clone(*s)
		if err = next.Apply(actor, a); err != nil {
			t.Fatal("advertised action rejected", a, err)
		}
	}
	for _, kind := range []string{"catan_road", "catan_explorer_ship", "catan_explorer_unit", "catan_explorer_bank", "catan_explorer_begin_move"} {
		if kinds[kind] == 0 {
			t.Fatal("missing initial construction", kind)
		}
	}
	for _, p := range []int{-1, (actor + 1) % 3, 3} {
		if len(s.catanExplorerChoices(p)) != 0 {
			t.Fatal("choices leaked to nonactor")
		}
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("probes mutated state")
	}
	// The compact wire representation must preserve zero-valued IDs and all
	// action fields relevant to execution, including unit replacement lists.
	data, _ := json.Marshal(catanExplorerChoiceView(choices))
	var decoded []Action
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(choices, decoded) {
		t.Fatal("wire actions differ")
	}
	if err = s.Apply(actor, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	before, _ = json.Marshal(s)
	moves := s.catanExplorerChoices(actor)
	voyages := 0
	for _, a := range moves {
		if a.Type == "catan_explorer_sail" {
			voyages++
		}
		next := clone(*s)
		if err = next.Apply(actor, a); err != nil {
			t.Fatal("movement choice rejected", a, err)
		}
	}
	after, _ = json.Marshal(s)
	if voyages == 0 || string(before) != string(after) {
		t.Fatal("missing/read-write movement choices")
	}
}

func TestCatanExplorerChoicesDoNotOracleHiddenDiscovery(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	actor := s.Turn
	if err = s.catanExplorerRoll([2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	explorerFlowResources(s, map[int][]int{actor: {3, 3, 3, 3, 3}, (actor + 1) % 3: {2, 0, 0, 0, 0}, (actor + 2) % 3: {0, 2, 0, 0, 0}})
	for _, movement := range []bool{false, true} {
		if movement {
			if err = s.Apply(actor, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
				t.Fatal(err)
			}
		}
		changed := clone(*s)
		hidden := changed.Catan.Explorer.Board.Hidden
		for i := range hidden {
			for j := i + 1; j < len(hidden); j++ {
				if hidden[i].Region == hidden[j].Region {
					hidden[i].Resource, hidden[j].Resource = hidden[j].Resource, hidden[i].Resource
					break
				}
			}
		}
		for i := range changed.Catan.Explorer.Board.Numbers {
			slices.Reverse(changed.Catan.Explorer.Board.Numbers[i])
		}
		p, q := (actor+1)%3, (actor+2)%3
		changed.Catan.Players[p].Resources, changed.Catan.Players[q].Resources = changed.Catan.Players[q].Resources, changed.Catan.Players[p].Resources
		if err = changed.validateCatanExplorer(); err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(s.View(actor))
		after, _ := json.Marshal(changed.View(actor))
		if string(before) != string(after) {
			t.Fatal("view choices reveal hidden map or opponent resource types")
		}
	}
	// Exhaust rewards while conserving supplies. Movement previews may not hide
	// a destination according to whether its unknown terrain reward is payable.
	dry := clone(*s)
	g := dry.Catan
	x := g.Explorer
	p := (actor + 1) % 3
	for r, n := range g.Bank {
		g.Players[p].Resources[r] += n
		g.Bank[r] = 0
	}
	x.Economy.Gold[p] += x.Economy.GoldBank
	x.Economy.GoldBank = 0
	if err = dry.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.catanExplorerChoices(actor), dry.catanExplorerChoices(actor)) {
		t.Fatal("hidden discovery reward influences route previews")
	}
}

func TestCatanExplorerDestinationsCrossFullEdgeAndStopAtFog(t *testing.T) {
	g, f, _, _ := explorerSailingFixture(t, 3)
	f.Positions[1], f.Positions[2] = -1, -1
	f.Positions[3], f.Positions[4] = 1, 1
	f.Turn = nil
	if err := f.begin(g, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	paths := f.destinations(g, 0, 1, 0)
	found := false
	for _, path := range paths {
		if path[len(path)-1] == 1 {
			t.Fatal("full edge advertised as a stop")
		}
		if slices.Equal(path, []int{1, 2}) {
			found = true
		}
	}
	if !found {
		t.Fatal("full edge blocks transit")
	}
	// Independently enumerate all non-looping routes on a two-hex graph and
	// compare minimum legal lengths with the destination planner.
	g, f, _, _ = explorerSailingFixture(t, 3, CatanHexSpec{Resource: CatanSea}, CatanHexSpec{Q: 1, Resource: CatanFog})
	expected := map[int]int{}
	var enumerate func(int, []int)
	enumerate = func(from int, path []int) {
		if len(path) >= 4 {
			return
		}
		for _, edge := range g.Edges {
			if edge.ID == f.Positions[0] || slices.Contains(path, edge.ID) || !catanExplorerAdjacentEdges(g.Edges[from], edge) {
				continue
			}
			next := append(slices.Clone(path), edge.ID)
			quote, err := f.quote(g, 0, 1, 0, next, -1, -1)
			if err == nil {
				if old := expected[edge.ID]; old == 0 || len(next) < old {
					expected[edge.ID] = len(next)
				}
			}
			// A quote can reject a full destination but allow transit. Fog is an
			// unconditional stop; continuing is checked by quote in deeper searches.
			if len(quote.Exploring) == 0 {
				enumerate(edge.ID, next)
			}
		}
	}
	enumerate(f.Positions[0], nil)
	actual := map[int]int{}
	for _, path := range f.destinations(g, 0, 1, 0) {
		actual[path[len(path)-1]] = len(path)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatal("shortest reachable map differs", expected, actual)
	}
}

func TestCatanExplorerChoicesFollowCargoAndNewSequence(t *testing.T) {
	s, err := newCatanExplorerState(2)
	if err != nil {
		t.Fatal(err)
	}
	actor := s.Turn
	if err = s.catanExplorerRoll([2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(actor, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	// Printed settler ship begins beside its harbor. Unload then reload using
	// exactly the offered choices, checking the live cargo positions in between.
	transferred := 0
	for step := 0; step < 2; step++ {
		for _, a := range s.catanExplorerChoices(actor) {
			if a.Type != "catan_explorer_transfer" || step == 0 && len(a.Take) == 0 || step == 1 && len(a.Give) == 0 {
				continue
			}
			if err = s.Apply(actor, a); err != nil {
				t.Fatal(err)
			}
			transferred++
			break
		}
	}
	if transferred != 2 {
		t.Fatal("printed ship cannot unload/reload", transferred)
	}
	old := s.catanExplorerChoices(actor)
	if err = s.Apply(actor, Action{Type: "catan_end", Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	if len(s.catanExplorerChoices(actor)) != 0 {
		t.Fatal("old actor still offered choices")
	}
	next := s.catanExplorerChoices(s.Turn)
	if len(next) != 1 || next[0].Type != "catan_roll" || next[0].Prompt != 2 {
		t.Fatal("new prompt missing")
	}
	for _, a := range old {
		if err = s.Apply(actor, a); err == nil {
			t.Fatal("stale choice accepted")
		}
	}
}

func TestCatanExplorerChoicesFullCargoReplacementAndEnd(t *testing.T) {
	s, err := newCatanExplorerState(2)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Turn
	if err = s.catanExplorerRoll([2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	explorerFlowResources(s, map[int][]int{p: {2, 2, 2, 2, 2}})
	x := s.Catan.Explorer
	harbor := x.Board.Opening[p].Harbor
	// Both printed containers are full: initial settler aboard, second settler
	// in its harbor. The allowed one-piece replacement must be explicit.
	x.Cargo.Units[p*11+1] = catanExplorerCargoLocation{"harbor", harbor}
	if err = s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	replacements := 0
	for _, a := range s.catanExplorerChoices(p) {
		if a.Type != "catan_explorer_unit" {
			continue
		}
		if len(a.Cards) != 1 || a.Card != a.Cards[0] {
			t.Fatal("full cargo build omitted required return", a)
		}
		next := clone(*s)
		if err = next.Apply(p, a); err != nil {
			t.Fatal("replacement option failed", a, err)
		}
		replacements++
	}
	if replacements != 2 {
		t.Fatal("missing ship or harbor replacement", replacements)
	}
	if err = s.EliminateCatan(p); err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []int{-1, 0, 1} {
		if len(s.catanExplorerChoices(viewer)) > 0 {
			t.Fatal("finished game offers actions")
		}
	}
}

func BenchmarkCatanExplorerChoiceView(b *testing.B) {
	for _, phase := range []string{"construction", "movement"} {
		b.Run(phase, func(b *testing.B) {
			s, err := newCatanExplorerState(4)
			if err != nil {
				b.Fatal(err)
			}
			if err = s.catanExplorerRoll([2]int{1, 1}); err != nil {
				b.Fatal(err)
			}
			explorerFlowResources(s, map[int][]int{s.Turn: {4, 4, 4, 4, 4}})
			if phase == "movement" {
				if err = s.Apply(s.Turn, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.View(s.Turn)
			}
		})
	}
}
