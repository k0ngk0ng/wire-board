package game

import (
	"fmt"
	"reflect"
	"testing"
)

func tradersHelperRecipe(n int, scene string, city bool) (*State, error) {
	switch scene {
	case "rivers":
		if city {
			return NewCatanRiversCitiesKnights(n, CatanOptions{FiveSix: n > 4})
		}
		if n == 2 {
			return NewCatanTwoRivers(n, CatanOptions{})
		}
		return NewCatanRivers(n, CatanOptions{FiveSix: n > 4})
	case "caravans":
		if city {
			return NewCatanCaravansCitiesKnights(n, CatanOptions{FiveSix: n > 4})
		}
		if n == 2 {
			return NewCatanTwoCaravans(n, CatanOptions{})
		}
		return NewCatanCaravans(n, CatanOptions{FiveSix: n > 4})
	case "attack":
		if city {
			if n == 2 {
				return NewCatanTwoAttackCitiesKnights(n, CatanOptions{})
			}
			return NewCatanAttackCitiesKnights(n)
		}
		if n == 2 {
			return NewCatanTwoAttack(n, CatanOptions{})
		}
		return NewCatanAttack(n)
	case "transport":
		if city {
			return NewCatanTransportCitiesKnights(n)
		}
		return NewCatanTransport(n)
	case "rivers-attack":
		return NewCatanRiversAttack(n, city)
	case "rivers-caravans":
		return NewCatanRiversCaravans(n, CatanOptions{FiveSix: n > 4}, city)
	case "rivers-transport":
		return NewCatanRiversTransport(n, city)
	case "caravans-attack":
		return NewCatanCaravansAttack(n, city)
	case "caravans-transport":
		return NewCatanCaravansTransport(n, city)
	case "attack-transport":
		return NewCatanAttackTransport(n, city)
	case "rivers-shores":
		return NewCatanRiversSeafarers(n, CatanRiversSeafarersSetup{Scenario: "shores"}, nil)
	case "attack-shores":
		return NewCatanAttackShores(n)
	case "attack-pirates":
		return NewCatanAttackPirates(n)
	case "attack-wonders":
		return NewCatanAttackWonders(n)
	case "attack-tribe":
		return NewCatanAttackTribe(n)
	case "transport-shores":
		return NewCatanTransportSeafarers(n, "shores")
	case "caravans-shores":
		return NewCatanCaravansShoresSeafarers(n)
	}
	panic(scene)
}

func tradersHelpersTurn(t *testing.T, n int, scene string, city bool) *State {
	t.Helper()
	s, err := tradersHelperRecipe(n, scene, city)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.EnableCatanTradersHelpers(true); err != nil {
		t.Fatal(err)
	}
	for step := 0; s.Phase != "catan_turn" && step < 150; step++ {
		p := ckActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	if s.Phase != "catan_turn" {
		t.Fatal("no turn")
	}
	return s
}

func TestCatanTradersHelpersKnightBuildLandingAndNeutral(t *testing.T) {
	for _, n := range []int{2, 3} {
		s := tradersHelpersTurn(t, n, "attack-wonders", false)
		g := s.Catan
		p := s.Turn
		eventAssignHelper(t, s, p, 8)
		edges := g.Attack.recruitEdges(g, p, "knighthood")
		g.Attack.Knights = append(g.Attack.Knights, catanAttackKnight{Player: p, Edge: edges[0]})
		if n == 2 {
			g.Attack.Knights = append(g.Attack.Knights, catanAttackKnight{Player: catanAttackNeutral, Edge: edges[1]})
		}
		vertex := -1
		for _, v := range g.Vertices {
			if v.Owner == p && v.Level == 1 {
				vertex = v.ID
				break
			}
		}
		if vertex < 0 {
			t.Fatal("missing initial village")
		}
		helperGrant(s, p, []int{0, 0, 0, 1, 2})
		action := Action{Type: "catan_city", Skill: "helper", Vertex: vertex, Target: edges[0]}
		bad := action
		bad.Target = edges[1]
		before := clone(*s)
		if s.Apply(p, bad) == nil || !reflect.DeepEqual(before, *s) {
			t.Fatal("foreign knight consumed")
		}
		if err := s.Apply(p, action); err != nil {
			t.Fatal(err)
		}
		if s.Catan.HelperPending == nil || !s.Catan.TradersHelpers.BuildLanding || s.Catan.Attack.WonderLanding != nil {
			t.Fatal("landing before helper exchange")
		}
		cp := clone(*s)
		s = &cp
		if err := s.Apply(p, Action{Type: "catan_helper_choice", Choice: "flip"}); err != nil {
			t.Fatal(err)
		}
		for s.Phase == "catan_attack_landing" {
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Apply(p, a); e != nil {
				t.Fatal(e)
			}
		}
		if s.Catan.TradersHelpers.BuildLanding || s.Catan.Vertices[vertex].Level != 2 {
			t.Fatal("construction continuation lost")
		}
		if n == 2 && (len(s.Catan.Attack.Knights) != 1 || s.Catan.Attack.Knights[0].Player != catanAttackNeutral) {
			t.Fatal("neutral knight removed")
		}
	}
}

func TestCatanTradersHelpersSupplyAndResourceSwap(t *testing.T) {
	for _, scene := range []string{"attack", "transport"} {
		for _, id := range []int{10, 11, 12} {
			if id == 12 && scene == "transport" {
				continue
			}
			s := tradersHelpersTurn(t, 3, scene, false)
			p := s.Turn
			g := s.Catan
			eventAssignHelper(t, s, p, id)
			gold := g.tradeGold()[p]
			hand := sum(g.Players[p].Resources)
			a := Action{Type: "catan_helper", Color: 1, Card: 0}
			if id == 12 {
				helperGrant(s, p, []int{1, 0, 0, 0, 0})
				hand++
			}
			if err := s.Apply(p, a); err != nil {
				t.Fatal(scene, id, err)
			}
			if id == 10 && s.Catan.tradeGold()[p] != gold+1 {
				t.Fatal("missing gold")
			}
			if id == 11 && sum(s.Catan.Players[p].Resources) != hand+1 {
				t.Fatal("missing resource")
			}
			if id == 12 && sum(s.Catan.Players[p].Resources) != hand {
				t.Fatal("swap changed hand size")
			}
			if s.Catan.HelperPending == nil || s.Catan.HelperPending.Kind != "exchange" {
				t.Fatal("ability not spent")
			}
		}
	}
}

func TestCatanTradersHelpersRiverMoveRefundAndBridgeGuard(t *testing.T) {
	s := tradersHelpersTurn(t, 3, "rivers", false)
	g := s.Catan
	player, from, to := -1, -1, -1
	for p := range g.Players {
		for _, e := range g.Edges {
			if !g.helperEndRoad(p, e.ID) || !g.riverEdge(e.ID) {
				continue
			}
			temp := clone(*g)
			temp.Edges[e.ID].Owner = -1
			for _, target := range temp.Edges {
				if target.ID != e.ID && temp.canRoad(p, target.ID) {
					player, from, to = p, e.ID, target.ID
					break
				}
			}
			if from >= 0 {
				break
			}
		}
		if from >= 0 {
			break
		}
	}
	if from < 0 {
		t.Skip("random starting roads lack a movable river road")
	}
	s.Turn = player
	eventAssignHelper(t, s, player, 4)
	gold := g.riverGold()[player]
	bank := g.Rivers.Bank
	bad := Action{Type: "catan_helper", Edge: from, Target: -1}
	before := clone(*s)
	if s.Apply(player, bad) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("failed move charged refund")
	}
	if err := s.Apply(player, Action{Type: "catan_helper", Edge: from, Target: to}); err != nil {
		t.Fatal(err)
	}
	want := gold - 1
	if g.riverEdge(to) {
		want++
	}
	if s.Catan.riverGold()[player] != want || s.Catan.Rivers.Bank != bank+gold-want {
		t.Fatal("river road move changed gold supply")
	}
	cp := clone(*s)
	edge := &cp.Catan.Edges[to]
	edge.Bridge = true
	if cp.Catan.helperEndRoad(player, to) {
		t.Fatal("bridge advertised as a movable road")
	}
}

func TestCatanTradersHelpersRejectCorruptContinuation(t *testing.T) {
	s := tradersHelpersTurn(t, 3, "attack", false)
	p := s.Turn
	eventAssignHelper(t, s, p, 6)
	helperGrant(s, p, []int{0, 0, 1, 1, 1})
	if err := s.Apply(p, Action{Type: "catan_buy_dev", Skill: "helper"}); err != nil {
		t.Fatal(err)
	}
	card := s.Catan.HelperPending.Cards[0]
	if err := s.Apply(p, Action{Type: "catan_helper_choice", Card: card}); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*State){func(s *State) { s.Turn = 99 }, func(s *State) { s.Catan.TradersHelpers.Rules = "unknown" }, func(s *State) { s.Catan.TradersHelpers.BuildLanding = true }, func(s *State) { s.Catan.TradersHelpers.AttackCard = "unknown" }, func(s *State) {
		eventAssignHelper(t, s, s.Turn, 5)
		s.Catan.Players[s.Turn].Helper.UsedTurn = s.Catan.TurnSerial
	}} {
		cp := clone(*s)
		mutate(&cp)
		if cp.validateTradersHelpers() == nil {
			t.Fatal("accepted corrupt helper continuation")
		}
	}
}

func TestCatanTradersHelpersAdmissionAndStart(t *testing.T) {
	scenes := []string{"rivers", "caravans", "attack", "transport", "rivers-attack", "rivers-caravans", "rivers-transport", "caravans-attack", "caravans-transport", "attack-transport", "rivers-shores", "attack-shores", "attack-pirates", "attack-wonders", "attack-tribe", "transport-shores", "caravans-shores"}
	for _, scene := range scenes {
		for _, n := range []int{2, 3, 6} {
			cities := []bool{false}
			if scene == "rivers" || scene == "caravans" || scene == "attack" || scene == "transport" || scene == "rivers-attack" || scene == "rivers-caravans" || scene == "rivers-transport" || scene == "caravans-attack" || scene == "caravans-transport" || scene == "attack-transport" {
				cities = append(cities, true)
			}
			if scene == "rivers-shores" || scene == "attack-shores" {
				cities = []bool{false}
			}
			for _, city := range cities {
				t.Run(fmt.Sprintf("%s/%d/%t", scene, n, city), func(t *testing.T) {
					s, err := tradersHelperRecipe(n, scene, city)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.EnableCatanTradersHelpers(true); err != nil {
						t.Fatal(err)
					}
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
					for i := 0; s.Catan.setup() && i < 100; i++ {
						p := ckActor(s)
						a, e := s.BotAction(p)
						if e != nil {
							t.Fatal(e)
						}
						if e = s.Apply(p, a); e != nil {
							t.Fatal(a, e)
						}
					}
					if s.Catan.setup() {
						t.Fatal("setup incomplete")
					}
					cp := clone(*s)
					s = &cp
					for _, p := range s.Catan.Players {
						if p.Helper == nil {
							t.Fatal("no helper after setup")
						}
					}
					if err = s.validateCatanEventSession(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestCatanTradersHelpersNatural(t *testing.T) {
	for _, c := range []struct {
		scene string
		n     int
		city  bool
	}{{"attack", 3, false}, {"attack-tribe", 2, false}, {"attack-wonders", 6, false}, {"transport", 3, true}, {"rivers-caravans", 2, true}, {"transport-shores", 6, false}, {"rivers-shores", 3, false}} {
		t.Run(fmt.Sprintf("%s/%d/%t", c.scene, c.n, c.city), func(t *testing.T) {
			s, e := tradersHelperRecipe(c.n, c.scene, c.city)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.EnableCatanTradersHelpers(true); e != nil {
				t.Fatal(e)
			}
			if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
				t.Fatal(e)
			}
			used := 0
			for step := 0; step < 16000 && !s.Finished; step++ {
				p := ckActor(s)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if a.Type == "catan_helper" || a.Skill == "helper" {
					used++
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				if s.Catan.HelperPending != nil {
					cp := clone(*s)
					s = &cp
					if err = s.validateCatanEventSession(); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !s.Finished || used == 0 {
				t.Fatal("incomplete helper match", s.Round, used)
			}
			t.Logf("completed round %d, helper actions %d", s.Round, used)
		})
	}
}
