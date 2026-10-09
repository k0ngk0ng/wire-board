package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanTradersVariantsAdmission(t *testing.T) {
	for _, scene := range []string{"rivers", "caravans", "attack", "transport", "rivers-attack", "rivers-caravans", "rivers-transport", "caravans-attack", "caravans-transport", "attack-transport", "rivers-shores", "attack-shores", "attack-pirates", "attack-wonders", "attack-tribe", "transport-shores", "caravans-shores"} {
		for _, n := range []int{2, 3, 6} {
			cities := []bool{false}
			if slices.Contains([]string{"rivers", "caravans", "attack", "transport", "rivers-attack", "rivers-caravans", "rivers-transport", "caravans-attack", "caravans-transport", "attack-transport"}, scene) {
				cities = append(cities, true)
			}
			for _, city := range cities {
				for _, bits := range []int{1, 2, 3} {
					t.Run(fmt.Sprintf("%s/%d/%t/%d", scene, n, city, bits), func(t *testing.T) {
						s, err := tradersHelperRecipe(n, scene, city)
						if err != nil {
							t.Fatal(err)
						}
						base := s.Catan.victoryTarget()
						if bits&1 != 0 {
							if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
								t.Fatal(err)
							}
						}
						if bits&2 != 0 {
							if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
								t.Fatal(err)
							}
							base++
						}
						if s.Catan.victoryTarget() != base {
							t.Fatal("goal drift")
						}
						if err = s.EnableCatanTradersHelpers(true); err != nil {
							t.Fatal(err)
						}
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
						cp := clone(*s)
						if err = cp.validateCatanEventSession(); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

func TestCatanTradersVariantsNatural(t *testing.T) {
	for _, c := range []struct {
		scene string
		n     int
		city  bool
	}{{"rivers", 3, false}, {"attack-pirates", 2, false}, {"attack-wonders", 6, false}, {"transport", 3, true}, {"rivers-caravans", 2, true}, {"transport-shores", 6, false}, {"attack-transport", 3, true}} {
		t.Run(fmt.Sprintf("%s/%d/%t", c.scene, c.n, c.city), func(t *testing.T) {
			s, err := tradersHelperRecipe(c.n, c.scene, c.city)
			if err != nil {
				t.Fatal(err)
			}
			base := s.Catan.victoryTarget()
			if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if err = s.EnableCatanTradersHelpers(true); err != nil {
				t.Fatal(err)
			}
			if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 16000 && !s.Finished; step++ {
				p := ckActor(s)
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(step, s.Phase, e)
				}
				if e = s.Apply(p, a); e != nil {
					t.Fatal(step, s.Phase, a, e)
				}
				if step%137 == 0 {
					cp := clone(*s)
					s = &cp
				}
			}
			if !s.Finished || s.Catan.victoryTarget() != base+1 {
				t.Fatal("incomplete match", s.Round, s.Phase)
			}
			t.Logf("finished round %d, goal %d", s.Round, base+1)
		})
	}
}

func TestCatanTradersFriendlySevenAndEventIsolation(t *testing.T) {
	s, err := NewCatanAttack(3)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		p := ckActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	g := s.Catan
	p := s.Turn
	protected := (p + 1) % 3
	other := (p + 2) % 3
	for _, id := range []int{protected, other} {
		helperGrant(s, id, []int{1, 0, 0, 0, 0})
	}
	g.Players[protected].Score = 2
	g.Players[other].Score = 3
	g.ResumePhase = "catan_turn"
	before := clone(g.Players[protected].Resources)
	s.catanAfterSevenDiscards()
	if !slices.Equal(g.Players[protected].Resources, before) {
		t.Fatal("stole protected resources")
	}
	if !slices.Contains(g.cardTheftTargets(p), protected) {
		t.Fatal("friendly changed card-event theft")
	}
	if s.Phase != "catan_turn" {
		t.Fatal("protected victim blocked seven")
	}
	for _, mutate := range []func(*Catan){func(g *Catan) { g.TradersVariants = "" }, func(g *Catan) { g.FriendlyRobber.Rules = "bad" }} {
		cp := clone(*s)
		mutate(cp.Catan)
		if cp.validateEventVariants() == nil {
			t.Fatal("bad variant accepted")
		}
	}
}

func TestCatanTradersVariantsRejectedSetupIsAtomic(t *testing.T) {
	s, _ := NewCatanTransport(3)
	s.Catan.Bank[0] = -1
	before := clone(*s)
	if s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("invalid variant setup mutated state")
	}
}

func TestCatanTradersHarborConquestAndLiberation(t *testing.T) {
	s, err := NewCatanAttack(3)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	city, other := -1, -1
	for _, port := range g.Ports {
		edge := g.Edges[port.Edge]
		for _, id := range []int{edge.A, edge.B} {
			eligible := true
			for _, tile := range g.Tiles {
				if slices.Contains(tile.Vertices, id) && !slices.Contains(g.Attack.Map.Coast, tile.ID) {
					eligible = false
				}
			}
			if city < 0 && eligible {
				city = id
			}
			if id != city {
				other = id
			}
		}
	}
	if city < 0 || other < 0 {
		t.Fatal("harbor fixture lacks conquerable city")
	}
	g.Vertices[city].Owner, g.Vertices[city].Level = 0, 2
	g.Vertices[other].Owner, g.Vertices[other].Level = 0, 1
	s.catanScores()
	if g.harborPoints()[0] != 3 || g.Harbors.Owner != 0 {
		t.Fatal("initial award")
	}
	touched := []int{}
	for _, tile := range g.Tiles {
		if slices.Contains(tile.Vertices, city) {
			g.Attack.Barbarians[tile.ID] = 3
			touched = append(touched, tile.ID)
		}
	}
	s.catanScores()
	if !g.Attack.conqueredBuilding(g, city) || g.harborPoints()[0] >= 3 || g.Harbors.Owner == 0 {
		t.Fatal("conquered city retained harbor award")
	}
	for _, id := range touched {
		g.Attack.Barbarians[id] = 2
	}
	s.catanScores()
	if g.harborPoints()[0] != 3 || g.Harbors.Owner != 0 {
		t.Fatal("liberation did not restore award")
	}
	if g.victoryTarget() != 13 {
		t.Fatal("harbor target")
	}
}

func TestCatanTradersTransportFriendlyMovesWithoutTheft(t *testing.T) {
	s, err := NewCatanTransport(3)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		p := ckActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	g := s.Catan
	p := s.Turn
	victim := (p + 1) % 3
	g.Players[victim].Score = 2
	helperGrant(s, victim, []int{1, 0, 0, 0, 0})
	edge := -1
	for _, e := range g.Edges {
		if !slices.Contains(g.Transport.Barbarians[:], e.ID) {
			edge = e.ID
			break
		}
	}
	g.Edges[edge].Owner = victim
	g.ResumePhase = "catan_turn"
	s.catanTransportBeginBarbarian()
	before := slices.Clone(g.Players[victim].Resources)
	if err = s.catanTransportBarbarian(p, Action{Type: "catan_transport_barbarian", Offer: int(g.Transport.BarbarianSequence), Card: 0, Edge: edge}); err != nil {
		t.Fatal(err)
	}
	if g.Transport.Barbarians[0] != edge || !slices.Equal(before, g.Players[victim].Resources) || s.Phase != "catan_turn" {
		t.Fatal("friendly protection prevented movement or stole cards")
	}
}
