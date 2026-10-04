package game

import (
	"reflect"
	"slices"
	"testing"
)

func tribeGame(t *testing.T) (*State, int) {
	t.Helper()
	s := seaTestGame(t)
	g := s.Catan
	g.Seafarers.Scenario = "tribe"
	g.Seafarers.VictoryPoints = 13
	g.Seafarers.Tribe = &CatanTribeState{Tokens: []int{}, Development: []CatanTribeDevelopment{}, Ports: []CatanPort{}, Points: make([]int, 3), HeldPorts: make([][]int, 3)}
	edge := seaEdge(t, g, 0, 1)
	v := g.Edges[edge].A
	g.Vertices[v].Owner = 0
	g.Vertices[v].Level = 1
	g.ResumePhase = "catan_turn"
	helperGrant(s, 0, []int{8, 8, 8, 8, 8})
	return s, edge
}
func tribeCard(t *testing.T, g *Catan, edge, card int) {
	t.Helper()
	i := slices.Index(g.DevDeck, card)
	if i < 0 {
		t.Fatal("fixture deck")
	}
	g.DevDeck = append(g.DevDeck[:i], g.DevDeck[i+1:]...)
	g.tribe().Development = append(g.tribe().Development, CatanTribeDevelopment{Edge: edge, Card: card})
}
func TestCatanTribeShipRewardsPortAndDevelopmentAge(t *testing.T) {
	s, edge := tribeGame(t)
	g := s.Catan
	g.tribe().Tokens = []int{edge}
	g.tribe().Ports = []CatanPort{{Edge: edge, Resource: 0}}
	tribeCard(t, g, edge, 0)
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	g = s.Catan
	tr := g.tribe()
	if tr.Points[0] != 1 || g.Players[0].Dev[0] != 1 || g.Players[0].NewDev[0] != 1 || len(tr.Tokens)+len(tr.Development)+len(tr.Ports) != 0 {
		t.Fatal("edge rewards not consumed exactly once")
	}
	if s.Phase != "catan_port" || s.CatanPendingActor() != 0 || tr.Pending.AfterRoute == nil || !reflect.DeepEqual(tr.HeldPorts[0], []int{0}) {
		t.Fatal("port response not persisted")
	}
	helperReject(t, s, 1, Action{Type: "catan_port", Edge: edge})
	helperReject(t, s, 0, Action{Type: "catan_end"})
	helperReject(t, s, 0, Action{Type: "catan_port", Edge: edge, Slot: -1})
	helperReject(t, s, 0, Action{Type: "catan_port", Edge: edge, Slot: 1})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_port", Edge: edge})
	g = s.Catan
	if s.Phase != "catan_turn" || g.tribe().Pending != nil || g.rates(0)[0] != 2 {
		t.Fatal("port did not resume action / apply rate")
	}
	before := append([]int{}, g.Players[0].Resources...)
	helperApply(t, s, 0, Action{Type: "catan_bank", Give: []int{2, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}})
	if s.Catan.Players[0].Resources[0] != before[0]-2 || s.Catan.Players[0].Resources[1] != before[1]+1 {
		t.Fatal("port could not be used immediately")
	}
	helperReject(t, s, 0, Action{Type: "catan_dev", Card: 0})
	for i := 0; i < 3; i++ {
		s.Phase = "catan_turn"
		helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	}
	helperApply(t, s, 0, Action{Type: "catan_dev", Card: 0})
	if s.Phase != "catan_robber" {
		t.Fatal("reward development did not unlock next action phase")
	}
}
func TestCatanTribeRoadDoesNotCollectAndMovingShipDoes(t *testing.T) {
	s, edge := tribeGame(t)
	s.Catan.tribe().Tokens = []int{edge}
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: edge})
	if s.Catan.tribe().Points[0] != 0 || len(s.Catan.tribe().Tokens) != 1 {
		t.Fatal("road collected sea reward")
	}
	s, edge = tribeGame(t)
	g := s.Catan
	g.Edges[edge].Owner = 0
	g.Edges[edge].Ship = true
	to := g.shipDestinations(0, edge)
	if len(to) == 0 {
		t.Fatal("fixture has no move")
	}
	g.tribe().Tokens = []int{to[0]}
	before := append([]int{}, g.Players[0].Resources...)
	helperApply(t, s, 0, Action{Type: "catan_move_ship", Edge: edge, Target: to[0]})
	if s.Catan.tribe().Points[0] != 1 || len(s.Catan.tribe().Tokens) != 0 || !reflect.DeepEqual(s.Catan.Players[0].Resources, before) {
		t.Fatal("move reward/payment")
	}
	if err := s.catanCollectTribe(0, to[0]); err != nil || s.Catan.tribe().Points[0] != 1 {
		t.Fatal("repeated reward")
	}
}
func TestCatanTribeRewardErrorsAreAtomicAndCanWinImmediately(t *testing.T) {
	s, edge := tribeGame(t)
	g := s.Catan
	g.tribe().Tokens = []int{edge}
	g.tribe().Development = []CatanTribeDevelopment{{Edge: edge, Card: 99}}
	helperReject(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	g.tribe().Development = nil
	g.tribe().Points[0] = 11
	g.tribe().Ports = []CatanPort{{Edge: edge, Resource: -1}}
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if !s.Finished || s.Catan.Players[0].Score != 13 || s.Catan.tribe().Pending != nil {
		t.Fatal("winning reward forced another response")
	}
	s, edge = tribeGame(t)
	g = s.Catan
	g.tribe().Points[0] = 11
	tribeCard(t, g, edge, 4)
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if !s.Finished || s.Catan.Players[0].Score != 13 {
		t.Fatal("new victory card not counted immediately")
	}
}
func TestCatanTribeHeldPortWaitsForSettlementAndSpacing(t *testing.T) {
	s, edge := tribeGame(t)
	g := s.Catan
	v := g.Edges[edge].A
	g.Vertices[v].Owner = -1
	g.Vertices[v].Level = 0
	g.Edges[edge].Owner = 0
	g.tribe().HeldPorts[0] = []int{-1, 2}
	if s.catanAskTribePort(0, s.Phase, nil, false) || s.Phase != "catan_turn" {
		t.Fatal("forced placement without coastal building")
	}
	helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: v})
	g = s.Catan
	if s.Phase != "catan_port" || len(g.tribe().HeldPorts[0]) != 2 {
		t.Fatal("building did not trigger held port")
	}
	helperApply(t, s, 0, Action{Type: "catan_port", Slot: 1, Edge: edge})
	g = s.Catan
	if s.Phase != "catan_turn" || len(g.tribe().HeldPorts[0]) != 1 || g.tribe().HeldPorts[0][0] != -1 || len(g.tribePortEdges(0)) != 0 {
		t.Fatal("adjacent ports allowed or unplaceable port lost")
	}
	for _, candidate := range g.Edges {
		if candidate.ID != edge && (candidate.A == g.Edges[edge].A || candidate.B == g.Edges[edge].A || candidate.A == g.Edges[edge].B || candidate.B == g.Edges[edge].B) {
			if slices.Contains(g.tribePortEdges(0), candidate.ID) {
				t.Fatal("port spacing")
			}
		}
	}
	if g.rates(0)[2] != 2 || g.rates(0)[0] != 4 {
		t.Fatal("held general port granted trade rate")
	}
}
func TestCatanTribePortResumesFreeRouteSetupAndHelper(t *testing.T) {
	t.Run("free route", func(t *testing.T) {
		s, edge := tribeGame(t)
		g := s.Catan
		s.Phase = "catan_roads"
		g.FreeRoads = 2
		g.tribe().Ports = []CatanPort{{Edge: edge, Resource: -1}}
		before := append([]int{}, g.Players[0].Resources...)
		helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
		if s.Catan.FreeRoads != 2 {
			t.Fatal("consumed free route before port response")
		}
		restored := clone(*s)
		s = &restored
		s.AutoCatanPending()
		g = s.Catan
		if s.Phase != "catan_roads" || g.FreeRoads != 1 || g.tribe().Pending != nil || !reflect.DeepEqual(before, g.Players[0].Resources) {
			t.Fatal("free route continuation")
		}
		a, e := s.BotAction(0)
		if e != nil {
			t.Fatal(e)
		}
		helperApply(t, s, 0, a)
		if s.Phase != "catan_turn" || s.Catan.FreeRoads != 0 {
			t.Fatal("second free route failed")
		}
	})
	t.Run("setup", func(t *testing.T) {
		s, edge := tribeGame(t)
		g := s.Catan
		s.Phase = "catan_setup_road"
		g.SetupStep = 0
		g.StartPlayer = 0
		g.SetupVertex = g.Edges[edge].A
		g.tribe().Ports = []CatanPort{{Edge: edge, Resource: -1}}
		helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
		if g = s.Catan; g.SetupStep != 0 || g.tribe().Pending.AfterRoute == nil {
			t.Fatal("setup advanced before port")
		}
		s.AutoCatanPending()
		if s.Catan.SetupStep != 1 || s.Turn != 1 || s.Phase != "catan_setup_settlement" {
			t.Fatal("setup continuation")
		}
	})
	t.Run("helper exchange", func(t *testing.T) {
		s, edge := tribeGame(t)
		g := s.Catan
		g.Options.Helpers = true
		g.TurnSerial = 2
		g.Players[0].Helper = &CatanHelperSeat{ID: 8}
		g.HelperDisplay = []int{1, 2}
		g.tribe().HeldPorts[0] = []int{0}
		v := g.Edges[edge].A
		g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
		g.Edges[edge].Owner = 0
		knight := slices.Index(g.DevDeck, 0)
		g.DevDeck = append(g.DevDeck[:knight], g.DevDeck[knight+1:]...)
		g.DevDiscard = append(g.DevDiscard, 0)
		g.Players[0].Knights = 1
		before := append([]int{}, g.Players[0].Resources...)
		helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: v, Skill: "helper"})
		g = s.Catan
		before[0]--
		before[1]--
		if s.Phase != "catan_port" || g.HelperPending != nil || !g.tribe().Pending.Helper || g.Vertices[v].Level != 1 || g.Players[0].Knights != 0 || !reflect.DeepEqual(g.Players[0].Resources, before) {
			t.Fatal("helper construction did not open port choice before helper exchange")
		}
		if err := s.EliminateCatan(0); err == nil {
			t.Fatal("eliminated pending player")
		}
		helperApply(t, s, 0, Action{Type: "catan_port", Edge: edge})
		if s.Phase != "catan_helper" || s.Catan.HelperPending.Kind != "exchange" {
			t.Fatal("helper exchange lost")
		}
		helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
		if s.Phase != "catan_turn" || !s.Catan.Players[0].Helper.Moon {
			t.Fatal("helper could not finish")
		}
	})
}
func TestCatanTribeNumberedSettlementsRobberAndPrivacy(t *testing.T) {
	s, edge := tribeGame(t)
	g := s.Catan
	g.Tiles[7].Number = 0
	g.Tiles[7].Resource = CatanGold
	for _, v := range g.Tiles[7].Vertices {
		if g.canSettlement(0, v, true) {
			t.Fatal("building on unnumbered outer island")
		}
	}
	s.Phase = "catan_robber"
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 7})
	g.Tiles[7].Resource = CatanDesert
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 7})
	legal := s.View(0)["catan"].(map[string]any)["legal"].(map[string][]int)["robber"]
	if slices.Contains(legal, 7) || !slices.Contains(legal, 3) {
		t.Fatal("robber public hints")
	}
	helperApply(t, s, 0, Action{Type: "catan_robber", Tile: 3})
	g = s.Catan
	tribeCard(t, g, edge, 0)
	s.Phase = "catan_turn"
	before := []map[string]any{}
	for viewer := -1; viewer < 3; viewer++ {
		before = append(before, s.View(viewer))
	}
	bot, e := s.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	g.tribe().Development[0].Card = 4
	for viewer := -1; viewer < 3; viewer++ {
		if !reflect.DeepEqual(before[viewer+1], s.View(viewer)) {
			t.Fatal("hidden edge card exposed", viewer)
		}
	}
	next, e := s.BotAction(0)
	if e != nil || !reflect.DeepEqual(bot, next) {
		t.Fatal("bot reads hidden edge card", bot, next, e)
	}
}
