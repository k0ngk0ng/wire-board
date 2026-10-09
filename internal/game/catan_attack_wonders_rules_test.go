package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanAttackWondersBalancedLandingAndRestore(t *testing.T) {
	s, err := NewCatanAttackWonders(3)
	if err != nil {
		t.Fatal(err)
	}
	finishAttackSetup(t, s)
	s.Phase = "catan_turn"
	g := s.Catan
	dice := [][2]int{{1, 1}, {6, 6}, {1, 2}}
	i := 0
	if err = s.startWonderLandings(1, func() [2]int { d := dice[i]; i++; return d }); err != nil {
		t.Fatal(err)
	}
	sources, targets := g.wonderLandingChoices()
	if len(sources) != 3 || len(targets) != 1 || targets[0] != 46 || g.Attack.supply() != 0 || !g.Attack.conquered(sources[0]) {
		t.Fatal("initial reserve or printed dual disc", sources, targets)
	}
	b := clone(*s)
	s = &b
	g = s.Catan
	if err = s.validateCatanAttack(); err != nil {
		t.Fatal(err)
	}
	a := Action{Type: "catan_attack_landing", Prompt: g.Attack.WonderLanding.ID, Slot: 0, Tile: sources[2], Target: targets[0]}
	before := clone(*s)
	if s.Apply((s.Turn+1)%3, a) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("wrong player mutated landing")
	}
	if err = s.Apply(s.Turn, a); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	sources, targets = g.wonderLandingChoices()
	if slices.Contains(sources, a.Tile) || g.Attack.Barbarians[46] != 1 || len(targets) != 1 || targets[0] != 46 {
		t.Fatal("reserve must be balanced", sources, targets)
	}
	a.Slot = 1
	before = clone(*s)
	if s.Apply(s.Turn, a) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("lower reserve accepted")
	}
	a.Tile = sources[0]
	if err = s.Apply(s.Turn, a); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Attack.Barbarians[46] != 2 {
		t.Fatal("2/12 production site not used for both landings")
	}
	// Invalid saved dice or the progress cursor cannot silently skip responses.
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Attack.WonderLanding.Cursor = 99 }, func(g *Catan) { g.Attack.WonderLanding.Dice[0] = [2]int{0, 2} }} {
		b := clone(*s)
		mutate(b.Catan)
		if b.validateCatanAttack() == nil {
			t.Fatal("corrupt landing accepted")
		}
	}
}

func TestCatanAttackWondersRechecksRequirementsAndVictory(t *testing.T) {
	s, err := NewCatanAttackWonders(3)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Turn = 0
	s.Phase = "catan_turn"
	w := g.wonders()
	marker := w.Markers[0]
	vertex := marker.Vertex
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 0, 1
	w.Cards[marker.Card].Owner = 0
	// Conserve pieces while making every land adjacent to this building conquered.
	for _, tile := range g.Tiles {
		if tile.Resource == CatanSea || !slices.Contains(tile.Vertices, vertex) {
			continue
		}
		if !slices.Contains(g.Attack.Map.Coast, tile.ID) {
			t.Fatal("fixture nonproductive marker")
		}
		g.Attack.Barbarians[tile.ID] = 3
		g.Attack.Barbarians[g.Attack.Map.Reserves[0]] -= 3
	}
	cost := catanWonderRules[marker.Card].Cost[:]
	catanMove(g.Bank, g.Players[0].Resources, cost)
	before := clone(*s)
	if !g.Attack.conqueredBuilding(g, vertex) || g.wonderRequirements(0, marker.Card) || s.catanWonderAction(0, Action{Type: "catan_wonder_build", Card: marker.Card}) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("conquered condition paid or built")
	}
	public := s.View(0)["catan"].(map[string]any)
	if len(public["wonderBuilds"].([]int)) != 0 {
		t.Fatal("invalid build offered to UI")
	}
	// Liberating the adjacent land permits building again with the same cost.
	for _, tile := range g.Tiles {
		if slices.Contains(tile.Vertices, vertex) && g.Attack.Barbarians[tile.ID] > 0 {
			g.Attack.Barbarians[tile.ID]--
			g.Attack.Prisoners[0]++
			break
		}
	}
	if !g.wonderRequirements(0, marker.Card) {
		t.Fatal("liberation did not restore condition")
	}
	if err = s.catanWonderAction(0, Action{Type: "catan_wonder_build", Card: marker.Card}); err != nil {
		t.Fatal(err)
	}
	g.Players[0].Score = 12
	w.Cards[marker.Card].Level = 1
	w.Cards[0].Owner = 1
	w.Cards[0].Level = 1
	if g.wonderVictory(0) {
		t.Fatal("tie won")
	}
	w.Cards[marker.Card].Level = 2
	if !g.wonderVictory(0) {
		t.Fatal("twelve points and higher level did not win")
	}
	g.Players[0].Score = 0
	w.Cards[marker.Card].Level = 4
	if !g.wonderVictory(0) {
		t.Fatal("four levels did not win")
	}
}

func TestCatanAttackWondersSourceAndTargetMinimum(t *testing.T) {
	s, err := NewCatanAttackWonders(3)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	// 4 occurs at two mainland hexes. Only the less occupied one is eligible.
	g.Attack.Barbarians[22] = 1
	g.Attack.Barbarians[g.Attack.Map.Reserves[0]]--
	d := [][2]int{{2, 2}, {2, 3}, {3, 3}}
	i := 0
	if err = s.startWonderLandings(1, func() [2]int { v := d[i]; i++; return v }); err != nil {
		t.Fatal(err)
	}
	_, targets := g.wonderLandingChoices()
	if slices.Contains(targets, 22) || !slices.Equal(targets, []int{25}) {
		t.Fatal("uneven fill options", targets)
	}
	// Both choices are offered when the two hexes have equal occupancy.
	g.Attack.Barbarians[25] = 1
	g.Attack.Barbarians[g.Attack.Map.Reserves[1]]--
	_, targets = g.wonderLandingChoices()
	if !slices.Equal(targets, []int{22, 25}) {
		t.Fatal("equal fill tie", targets)
	}
}

func TestCatanAttackWondersReserveCombatAndPrivacy(t *testing.T) {
	s, err := NewCatanAttackWonders(3)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	for _, id := range g.Attack.Map.Reserves {
		if g.Attack.Barbarians[id] != 12 || !g.Attack.conquered(id) {
			t.Fatal("printed reserve")
		}
	}
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	qdice := [][2]int{{2, 2}, {2, 3}, {3, 3}}
	i := 0
	if err = s.startWonderLandings(1, func() [2]int { d := qdice[i]; i++; return d }); err != nil {
		t.Fatal(err)
	}
	view := s.View(-1)["catan"].(map[string]any)["attack"].(map[string]any)
	if _, ok := view["wonderLanding"].(map[string]any)["dice"]; ok {
		t.Fatal("future landing dice leaked")
	}
	g.Attack.WonderLanding = nil
	s.Phase = "catan_turn"
	// The capture/treason cards and ordinary knight battles still apply to the
	// desert hexes, including liberation once fewer than three barbarians remain.
	desert := g.Attack.Map.Reserves[0]
	g.Attack.Prisoners[0] += 9
	g.Attack.Barbarians[desert] = 3
	if !slices.Contains(g.attackCaptureTargets(), desert) || !slices.Contains(g.attackBattleTiles(), desert) {
		t.Fatal("reserve omitted from card or battle")
	}
	if err = g.attackCapture(desert, 0); err != nil {
		t.Fatal(err)
	}
	if g.Attack.conquered(desert) {
		t.Fatal("reserve remains conquered after capture")
	}
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Attack.Barbarians[desert]++ }, func(g *Catan) { g.wonders().Cards[0].Owner = 99 }, func(g *Catan) { g.Attack.Map.Reserves[0] = 0 }, func(g *Catan) { g.wonders().Cards[0].Level = 1 }} {
		b := clone(*s)
		mutate(b.Catan)
		if b.validateCatanAttack() == nil {
			t.Fatal("corrupt reserve or wonder accepted")
		}
	}
}
