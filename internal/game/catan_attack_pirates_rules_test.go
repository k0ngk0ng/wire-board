package game

import (
	"reflect"
	"slices"
	"testing"
)

func pirateAttackCardFixture(t *testing.T, n int, card string) *State {
	t.Helper()
	s, err := NewCatanAttackPirates(n)
	if err != nil {
		t.Fatal(err)
	}
	finishPirateAttackSetup(t, s)
	g := s.Catan
	s.Phase = "catan_turn"
	if n == 2 {
		g.Two.Rolls = []int{6, 8}
	}
	at := slices.Index(g.Attack.Deck, card)
	if at < 0 {
		t.Fatal("card missing")
	}
	last := len(g.Attack.Deck) - 1
	g.Attack.Deck[at], g.Attack.Deck[last] = g.Attack.Deck[last], g.Attack.Deck[at]
	if err = s.catanAttackDrawCard(s.Turn); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanAttackPiratesWarshipChoiceAndKnightIsolation(t *testing.T) {
	s := pirateAttackCardFixture(t, 2, "knighthood")
	g := s.Catan
	actor := s.Turn
	id := g.pirateNextWarship(actor)
	if id < 0 {
		t.Fatal("no initial ship")
	}
	a := Action{Type: "catan_attack_card", Prompt: g.Attack.Pending.ID, Choice: "warship", Edge: id}
	before := clone(*s)
	bad := a
	bad.Edge = id + 1
	if s.Apply(actor, bad) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("wrong ship mutated state")
	}
	if err := s.Apply(actor, a); err != nil {
		t.Fatal(err)
	}
	if !s.Catan.Edges[id].Warship || len(s.Catan.Attack.Knights) != 0 || s.Catan.Attack.Pending == nil || !s.Catan.Attack.Pending.WarshipUsed {
		t.Fatal("optional upgrade lost mandatory recruitment")
	}
	// Reload after the optional effect: the same card cannot upgrade twice.
	reloaded := clone(*s)
	s = &reloaded
	before = clone(*s)
	if s.Apply(actor, a) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("second upgrade mutated state")
	}
	if err := s.validateCatanAttack(); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	actor = s.Turn
	edges := g.Attack.recruitEdges(g, actor, "knighthood")
	a = Action{Type: "catan_attack_card", Prompt: g.Attack.Pending.ID, Choice: "knighthood", Edge: edges[0]}
	if err := s.Apply(actor, a); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Attack.Pending == nil || !s.Catan.Attack.Pending.Neutral {
		t.Fatal("neutral knight not queued")
	}
	g = s.Catan
	bad = a
	bad.Choice = "warship"
	bad.Edge = g.pirateNextWarship(actor)
	before = clone(*s)
	if s.Apply(actor, bad) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("neutral response upgraded ship")
	}
	b := clone(*s)
	s = &b
	a, err := s.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	if a.Choice == "warship" {
		t.Fatal("bot upgraded during neutral placement")
	}
	if err = s.Apply(actor, a); err != nil {
		t.Fatal(err)
	}
	if len(s.Catan.Attack.Knights) != 2 || s.Catan.Attack.Knights[1].Player != catanAttackNeutral {
		t.Fatal("shared knight missing")
	}
	if s.Catan.Attack.Pending != nil || !s.Catan.Edges[id].Warship {
		t.Fatal("upgrade and recruitment failed to finish together")
	}
	// Recruitment remains legal without accepting the optional upgrade.
	s = pirateAttackCardFixture(t, 3, "knighthood")
	g = s.Catan
	actor, id = s.Turn, g.pirateNextWarship(s.Turn)
	edges = g.Attack.recruitEdges(g, actor, "knighthood")
	if err := s.Apply(actor, Action{Type: "catan_attack_card", Prompt: g.Attack.Pending.ID, Choice: "knighthood", Edge: edges[0]}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Edges[id].Warship || s.Catan.Attack.Pending != nil || len(s.Catan.Attack.Knights) != 1 {
		t.Fatal("direct recruitment did not skip upgrade")
	}
}

func TestCatanAttackPiratesMapSetupAndLanding(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s, err := NewCatanAttackPirates(n)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if sum(g.Attack.Barbarians) != 0 || g.Seafarers.VictoryPoints != 12 {
			t.Fatal("initial supply or goal")
		}
		finishPirateAttackSetup(t, s)
		g = s.Catan
		for p := range g.Players {
			_, villages, cities := g.pieces(p)
			if villages != 4 || cities != 0 {
				t.Fatal("setup built city", p, villages, cities)
			}
		}
		if n <= 4 {
			if g.Tiles[27].Resource != catanCastle || g.Tiles[18].Number != 2 || g.Attack.Map.ExtraNumbers[0].Number != 12 {
				t.Fatal("printed map")
			}
		}
		if n == 2 {
			if len(g.pirateIslands().Fortresses) != 2 || len(g.Attack.Gold) != 2 || len(g.Two.SeaStarts) != 2 {
				t.Fatal("extra player leak")
			}
		}
		for _, mutate := range []func(*Catan){func(g *Catan) { g.pirateIslands().Fortresses[0].Strength = 4 }, func(g *Catan) { g.pirateIslands().FleetPath[0] = 0 }, func(g *Catan) { g.Attack.Map.Castles[0] = 0 }, func(g *Catan) { g.Seafarers.Pirate = 0 }} {
			b := clone(*s)
			mutate(b.Catan)
			if b.validateCatanAttack() == nil {
				t.Fatal("bad pirate state")
			}
		}
	}
	s, err := NewCatanAttackPirates(3)
	if err != nil {
		t.Fatal(err)
	}
	finishPirateAttackSetup(t, s)
	g := s.Catan
	s.Phase = "catan_turn"
	rolls := [][2]int{{5, 5}, {3, 3}, {2, 2}}
	at := 0
	if err = s.startWonderLandings(1, func() [2]int { d := rolls[at]; at++; return d }); err != nil {
		t.Fatal(err)
	}
	source, targets := g.wonderLandingChoices()
	if !slices.Equal(source, []int{-1}) || len(targets) != 2 {
		t.Fatal("public supply or target tie", source, targets)
	}
	id := targets[0]
	g.Attack.Barbarians[id] = 1
	_, next := g.wonderLandingChoices()
	if slices.Contains(next, id) {
		t.Fatal("more occupied target admitted")
	}
}

func TestCatanAttackPiratesFortressGateAndPostCaptureShip(t *testing.T) {
	s, err := NewCatanAttackPirates(3)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	s.Turn = 0
	g.Players[0].Score = 12
	s.catanVictory()
	if s.Finished {
		t.Fatal("unconquered fortress won")
	}
	f := &g.pirateIslands().Fortresses[0]
	f.Strength = 0
	g.Vertices[f.Vertex].Owner, g.Vertices[f.Vertex].Level = 0, 1
	if g.pirateNextWarship(0) != -1 {
		t.Fatal("post-capture warship upgrade")
	}
	for _, e := range g.Edges {
		if g.canShip(0, e.ID) {
			t.Fatal("post-capture new ship")
		}
	}
	s.catanVictory()
	if !s.Finished {
		t.Fatal("captured fortress and twelve points not winning")
	}
}

func finishPirateAttackSetup(t *testing.T, s *State) {
	t.Helper()
	for step := 0; s.Catan.setup() && step < 160; step++ {
		a, e := s.BotAction(s.Turn)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(s.Turn, a); e != nil {
			t.Fatal(e)
		}
	}
	if s.Catan.setup() {
		t.Fatal("setup unfinished")
	}
}

func TestCatanAttackPiratesFleetAndOrdinaryDiceContinue(t *testing.T) {
	s, err := NewCatanAttackPirates(3)
	if err != nil {
		t.Fatal(err)
	}
	finishPirateAttackSetup(t, s)
	g := s.Catan
	p := g.pirateIslands()
	// Force a defense reward using public fleet geometry and a converted own ship.
	tile := -1
	vertex := -1
	for _, id := range p.FleetPath {
		for _, v := range g.Tiles[id].Vertices {
			if g.Vertices[v].Level == 0 {
				tile, vertex = id, v
				break
			}
		}
		if tile >= 0 {
			break
		}
	}
	actor := s.Turn
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = actor, 1
	g.Edges[p.Fortresses[actor].StartShip].Warship = true
	// Strength is one: use two legal expedition ships as defense.
	ship := -1
	for _, e := range g.Edges {
		if e.Owner != -1 {
			continue
		}
		_, route, ok := g.pirateShipPlan(actor, e.ID)
		if ok && len(route) == len(p.Fortresses[actor].Route)+1 {
			ship = e.ID
			break
		}
	}
	if ship < 0 {
		t.Fatal("no extension")
	}
	g.Edges[ship].Owner, g.Edges[ship].Ship, g.Edges[ship].Warship = actor, true, true
	if !g.pirateCommitShip(actor, ship) {
		t.Fatal("extension route")
	}
	at := slices.Index(p.FleetPath, tile)
	g.Seafarers.Pirate = p.FleetPath[(at-1+len(p.FleetPath))%len(p.FleetPath)]
	g.Dice = []int{1, 1}
	g.RollID = 1
	s.Phase = "catan_roll"
	pending, err := s.catanRaidFleet(2)
	if err != nil || !pending {
		t.Fatal("no defense response", err)
	}
	if err = s.validateCatanAttack(); err != nil {
		t.Fatal("defense restore", err)
	}
	b := clone(*s)
	s = &b
	color := 0
	for s.Catan.Bank[color] == 0 {
		color++
	}
	if err = s.Apply(actor, Action{Type: "catan_fleet_reward", Color: color}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || s.Catan.pirateIslands().Raid != nil {
		t.Fatal("defense production did not continue")
	}
}
