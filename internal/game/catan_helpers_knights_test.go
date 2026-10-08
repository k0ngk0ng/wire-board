package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func helperCityRestore(t *testing.T, s *State) {
	t.Helper()
	b, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var q State
	if e = json.Unmarshal(b, &q); e != nil {
		t.Fatal(e)
	}
	if e = q.validateCityHelpers(); e != nil {
		t.Fatal(s.Phase, e)
	}
	*s = q
}
func helperCityFixture(t *testing.T, id int) *State {
	t.Helper()
	s, e := NewCatanCitiesKnights(3, CatanOptions{Helpers: true, AllHelpers: true})
	if e != nil {
		t.Fatal(e)
	}
	for s.Catan.setup() {
		a, e := s.BotAction(s.Turn)
		if e != nil {
			t.Fatal(e)
		}
		helperApply(t, s, s.Turn, a)
	}
	g := s.Catan
	s.Turn = 0
	s.Phase = "catan_turn"
	g.TurnSerial = 10
	old := g.Players[0].Helper.ID
	if at := slices.Index(g.HelperDisplay, id); at >= 0 {
		g.HelperDisplay[at] = old
	} else {
		for p := 1; p < len(g.Players); p++ {
			if g.Players[p].Helper.ID == id {
				g.Players[p].Helper.ID = old
			}
		}
	}
	g.Players[0].Helper = &CatanHelperSeat{ID: id, AcquiredTurn: 1}
	helperCityRestore(t, s)
	return s
}
func TestCatanHelpersKnightsNaturalGames(t *testing.T) {
	for _, recipe := range []struct {
		n      int
		sea    string
		events bool
	}{{3, "", false}, {6, "", true}, {3, "shores", true}, {4, "tribe", false}, {3, "islands", false}, {5, "fog", true}, {6, "desert", false}, {3, "cloth", true}, {3, "pirate_islands", false}, {4, "wonders", true}, {5, "new_world", false}} {
		t.Run(fmt.Sprint(recipe), func(t *testing.T) {
			o := CatanOptions{Helpers: true, AllHelpers: true, FiveSix: recipe.n > 4}
			var s *State
			var e error
			if recipe.sea == "" {
				s, e = NewCatanCitiesKnights(recipe.n, o)
			} else {
				var world *CatanNewWorldMap
				if recipe.sea == "new_world" {
					world, e = GenerateCatanNewWorldMap(recipe.n)
					if e != nil {
						t.Fatal(e)
					}
				}
				s, e = NewCatanCitiesKnightsSeafarers(recipe.n, o, CatanSeafarersSetup{Scenario: recipe.sea}, world)
			}
			if e != nil {
				t.Fatal(e)
			}
			if recipe.events {
				if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
					t.Fatal(e)
				}
			}
			seen := map[string]int{}
			for step := 0; step < 14000 && !s.Finished; step++ {
				actor := s.CatanPendingActor()
				if actor < 0 {
					actor = s.Turn
				}
				if s.Phase == "catan_discard" {
					for p, n := range s.Catan.DiscardDue {
						if n > 0 {
							actor = p
							break
						}
					}
				}
				a, e := s.BotAction(actor)
				if e != nil {
					t.Fatal(step, s.Phase, e)
				}
				if a.Skill == "helper" || a.Type == "catan_helper" {
					seen[fmt.Sprint(s.Catan.Players[actor].Helper.ID)]++
				}
				if e = s.Apply(actor, a); e != nil {
					t.Fatal(step, s.Phase, a, e)
				}
				if step%31 == 0 {
					helperCityRestore(t, s)
				}
			}
			if !s.Finished {
				t.Fatal("did not naturally finish", s.Round, seen)
			}
			t.Log(s.Round, seen)
		})
	}
}
func TestCatanHelpersKnightsProgressChoicePrivacyAndSwap(t *testing.T) {
	for track := 0; track < 3; track++ {
		t.Run(fmt.Sprint(track), func(t *testing.T) {
			s := helperCityFixture(t, 6)
			helperGrant(s, 0, []int{0, 0, 1, 1, 1})
			helperReject(t, s, 0, Action{Type: "catan_helper", Choice: "progress_buy", Color: 3})
			helperApply(t, s, 0, Action{Type: "catan_helper", Choice: "progress_buy", Color: track})
			helperCityRestore(t, s)
			q := s.Catan.HelperPending
			if q == nil || q.Kind != "progress" || len(q.Cards) != 3 {
				t.Fatal("no private progress choice")
			}
			for _, viewer := range []int{1, 2, -1} {
				v := s.View(viewer)["catan"].(map[string]any)["helperPending"].(map[string]any)
				if v["cards"] != nil {
					t.Fatal("private candidates leaked")
				}
			}
			helperReject(t, s, 1, Action{Type: "catan_helper_choice", Card: q.Cards[0]})
			card := q.Cards[0]
			helperApply(t, s, 0, Action{Type: "catan_helper_choice", Card: card})
			helperCityRestore(t, s)
			if s.Catan.HelperPending.Kind != "exchange" {
				t.Fatal("missing helper flip")
			}
			helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "exchange", Card: 12})
			s.Catan.TurnSerial++
			if !catanProgressRules[card].Victory {
				helperApply(t, s, 0, Action{Type: "catan_helper", Card: card})
				helperCityRestore(t, s)
				if s.Catan.HelperPending.Kind != "exchange" {
					t.Fatal("swap did not consume helper")
				}
			}
		})
	}
}
func TestCatanHelpersKnightsHildaAfterAqueductAndThorolfWalls(t *testing.T) {
	s := helperCityFixture(t, 3)
	g := s.Catan
	k := g.CitiesKnights
	g.Dice = []int{1, 1}
	g.RollID = 1
	k.Players[0].Improvements[0] = 3
	before := sum(g.Players[0].Resources)
	s.catanAfterProduction([]int{0, 1, 1})
	if s.Phase != "catan_aqueduct" || g.HelperPending != nil || k.Helpers.Production == nil {
		t.Fatal("aqueduct did not precede Hilda")
	}
	helperCityRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_aqueduct", Color: 0})
	if s.Catan.HelperPending == nil || s.Catan.HelperPending.Player != 0 {
		t.Fatal("Hilda lost after aqueduct")
	}
	helperCityRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Color: 1})
	if sum(s.Catan.Players[0].Resources) != before+2 {
		t.Fatal("independent compensation")
	}
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	s = helperCityFixture(t, 5)
	g = s.Catan
	k = g.CitiesKnights
	for _, v := range g.Vertices {
		if v.Owner == 0 && v.Level == 2 {
			k.Walls = append(k.Walls, v.ID)
			break
		}
	}
	attackHand(s, 0, []int{0, 0, 0, 0, 0, 8, 0, 0})
	g.Dice = []int{3, 4}
	g.RollID = 1
	if e := s.catanRollProductionEffect(7, false); e != nil {
		t.Fatal(e)
	}
	if g.DiscardDue[0] != 0 || g.HelperPending == nil || g.HelperPending.Kind != "resource" {
		t.Fatal("wall threshold ignored")
	}
	helperCityRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Color: 2})
}
func TestCatanHelpersKnightsGregorRealKnightAndCommodityBoundaries(t *testing.T) {
	s := helperCityFixture(t, 8)
	g := s.Catan
	k := g.CitiesKnights
	vertex := -1
	knight := -1
	for _, v := range g.Vertices {
		if v.Owner == 0 && v.Level == 1 {
			vertex = v.ID
		}
		if v.Level == 0 && knight < 0 {
			knight = v.ID
		}
	}
	k.Knights = append(k.Knights, CatanKnight{Owner: 0, Vertex: knight, Strength: 2, Active: true})
	attackHand(s, 0, []int{0, 0, 0, 1, 2, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_city", Vertex: vertex, Skill: "helper", Target: -1})
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: vertex, Skill: "helper", Target: knight})
	helperCityRestore(t, s)
	if s.Catan.knightAt(knight) != nil || s.Catan.Vertices[vertex].Level != 2 || sum(s.Catan.Players[0].Resources) != 0 || len(s.Catan.HelperExile) != 0 {
		t.Fatal("wrong entity knight payment")
	}
	s = helperCityFixture(t, 7)
	g = s.Catan
	g.Players[1].Score = 9
	attackHand(s, 1, []int{0, 0, 0, 0, 0, 1, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_helper", Target: 1})
	attackHand(s, 1, []int{1, 0, 0, 0, 0, 1, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_helper", Target: 1})
	v := s.View(0)["catan"].(map[string]any)["helperPending"].(map[string]any)
	if len(v["resources"].([]int)) != 5 {
		t.Fatal("commodity hand revealed")
	}
	helperReject(t, s, 0, Action{Type: "catan_helper_choice", Color: 5})
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Color: 0})
	helperCityRestore(t, s)
}

func TestCatanHelpersKnightsAlchemyProductionUsesDice(t *testing.T) {
	s := helperCityFixture(t, 5)
	if e := s.EnableCatanEvents(CatanEventCatalogue); e != nil {
		t.Fatal(e)
	}
	s.Phase = "catan_roll"
	if e := s.catanCityRoll(3, 4, 0); e != nil {
		t.Fatal(e)
	}
	if s.Catan.RevealedEvent != nil || s.Catan.HelperPending == nil || s.Catan.HelperPending.Kind != "resource" {
		t.Fatal("alchemy helper not reached")
	}
	if e := s.validateCatanEventSession(); e != nil {
		t.Fatal(e)
	}
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Color: 0})
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	helperCityRestore(t, s)
}

func TestCatanHelpersKnightsConfigurationsAndOldBaseIsolation(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, scene := range []string{"", "shores", "islands", "fog", "desert", "tribe", "cloth", "pirate_islands", "wonders", "new_world"} {
			for _, all := range []bool{false, true} {
				o := CatanOptions{Helpers: true, AllHelpers: all, FiveSix: n > 4}
				var s *State
				var e error
				if scene == "" {
					s, e = NewCatanCitiesKnights(n, o)
				} else {
					var world *CatanNewWorldMap
					if scene == "new_world" {
						world, e = GenerateCatanNewWorldMap(n)
						if e != nil {
							t.Fatal(e)
						}
					}
					s, e = NewCatanCitiesKnightsSeafarers(n, o, CatanSeafarersSetup{Scenario: scene}, world)
				}
				if e != nil {
					t.Fatal(n, scene, all, e)
				}
				if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
					t.Fatal(n, scene, all, e)
				}
				if e = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); e != nil {
					t.Fatal(e)
				}
				if e = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); e != nil {
					t.Fatal(e)
				}
				helperCityRestore(t, s)
			}
		}
	}
	s, e := NewCatanCitiesKnights(3, CatanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if s.Catan.CitiesKnights.Helpers != nil || s.Catan.Options.Helpers {
		t.Fatal("adaptation leaked to original knights")
	}
	b, e := NewCatan(3, CatanOptions{Helpers: true})
	if e != nil {
		t.Fatal(e)
	}
	if b.Catan.CitiesKnights != nil || len(b.Catan.DevDeck) != 25 {
		t.Fatal("base helper deck changed")
	}
}
func TestCatanHelpersKnightsProgressVictoryAndCorruptRestore(t *testing.T) {
	s := helperCityFixture(t, 6)
	g := s.Catan
	k := g.CitiesKnights
	at := slices.Index(k.ProgressDecks[0], 9)
	k.ProgressDecks[0][at], k.ProgressDecks[0][len(k.ProgressDecks[0])-1] = k.ProgressDecks[0][len(k.ProgressDecks[0])-1], 9
	k.Players[0].DefenderPoints = g.victoryTarget() - g.Players[0].Score - 1
	s.catanScores()
	helperGrant(s, 0, []int{0, 0, 1, 1, 1})
	helperApply(t, s, 0, Action{Type: "catan_helper", Choice: "progress_buy", Color: 0})
	for _, mutate := range []func(*State){
		func(q *State) { q.Catan.CitiesKnights.Helpers.Rules = "bad" },
		func(q *State) { q.Catan.Options.Helpers = false },
		func(q *State) { q.Catan.HelperPending.Cards[0] = 99 },
		func(q *State) { q.Catan.HelperPending.Target = 2 },
		func(q *State) { q.Catan.CitiesKnights.ProgressDecks[0] = q.Catan.CitiesKnights.ProgressDecks[0][1:] },
	} {
		q := clone(*s)
		mutate(&q)
		if q.validateCityHelpers() == nil {
			t.Fatal("corrupt progress inventory accepted")
		}
	}
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Card: 9})
	helperCityRestore(t, s)
	if !s.Finished || s.Catan.HelperPending != nil || !slices.Contains(s.Catan.CitiesKnights.Players[0].PublicProgress, 9) {
		t.Fatal("helper point did not win immediately")
	}
}
func TestCatanHelpersKnightsOrdinaryAbilitiesAndRoadConnections(t *testing.T) {
	for _, id := range []int{1, 2, 4, 9, 10, 11} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			s := helperCityFixture(t, id)
			g := s.Catan
			attackHand(s, 0, []int{4, 4, 4, 4, 4, 1, 1, 1})
			var a Action
			switch id {
			case 1:
				attackHand(s, 1, []int{0, 0, 1, 0, 0, 1, 0, 0})
				a = Action{Type: "catan_helper", Color: 2, Targets: []int{1}, Cards: []int{0}}
				helperReject(t, s, 0, Action{Type: "catan_helper", Color: 5, Targets: []int{1}, Cards: []int{0}})
			case 2:
				edge := -1
				for _, e := range g.Edges {
					if g.canRoad(0, e.ID) {
						edge = e.ID
						break
					}
				}
				a = Action{Type: "catan_road", Edge: edge, Skill: "helper", Tokens: []int{0, 1, 1, 0, 0}}
			case 4:
				from, to := -1, -1
				for _, e := range g.Edges {
					if !g.helperEndRoad(0, e.ID) {
						continue
					}
					copy := *g
					copy.Edges = slices.Clone(g.Edges)
					copy.Edges[e.ID].Owner = -1
					for _, f := range copy.Edges {
						if f.ID != e.ID && copy.canRoad(0, f.ID) {
							from, to = e.ID, f.ID
							break
						}
					}
					if from >= 0 {
						break
					}
				}
				if from < 0 {
					t.Fatal("fixture lacks end road")
				}
				edge := g.Edges[from]
				tip := edge.A
				if g.Vertices[tip].Level > 0 {
					tip = edge.B
				}
				g.CitiesKnights.Knights = append(g.CitiesKnights.Knights, CatanKnight{Owner: 0, Vertex: tip, Strength: 1})
				helperReject(t, s, 0, Action{Type: "catan_helper", Edge: from, Target: to})
				g.CitiesKnights.Knights = nil
				a = Action{Type: "catan_helper", Edge: from, Target: to}
			case 9:
				helperReject(t, s, 0, Action{Type: "catan_helper", Give: []int{0, 0, 0, 0, 0, 2, 0, 0}, Take: []int{1, 0, 0, 0, 0, 0, 0, 0}})
				a = Action{Type: "catan_helper", Give: []int{2, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}}
			case 10, 11:
				helperReject(t, s, 0, Action{Type: "catan_helper", Color: 0})
				for _, tile := range g.Tiles {
					if tile.Resource < 5 {
						g.Robber = tile.ID
						break
					}
				}
				g.CitiesKnights.Invasions = 1
				a = Action{Type: "catan_helper", Color: 0}
			}
			helperApply(t, s, 0, a)
			helperCityRestore(t, s)
			if s.Catan.HelperPending == nil || s.Catan.HelperPending.Kind != "exchange" {
				t.Fatal("helper did not complete")
			}
			helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
			helperCityRestore(t, s)
		})
	}
}

func TestCatanHelpersKnightsCommodityProductionAndEmptyBank(t *testing.T) {
	s := helperCityFixture(t, 3)
	g := s.Catan
	g.Dice = []int{2, 3}
	g.RollID = 1
	// Receiving a commodity is receiving production; gold choices have already
	// contributed to this same count. Neither a helper nor aqueduct is due.
	g.CitiesKnights.Players[0].Improvements[0] = 3
	s.catanAfterProduction([]int{1, 0, 0})
	if g.HelperPending != nil || g.CitiesKnights.Helpers.Production != nil || g.CitiesKnights.Pending != nil {
		t.Fatal("commodity earner received compensation")
	}
	// When the aqueduct exhausts the last ordinary card, Hilda must not block.
	for c := 0; c < 5; c++ {
		g.Players[2].Resources[c] += g.Bank[c]
		g.Bank[c] = 0
	}
	g.Players[2].Resources[0]--
	g.Bank[0] = 1
	s.catanAfterProduction([]int{0, 1, 1})
	helperCityRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_aqueduct", Color: 0})
	if s.Phase != "catan_turn" || s.Catan.HelperPending != nil || s.Catan.CitiesKnights.Helpers.Production != nil {
		t.Fatal("empty bank stalled helper")
	}
	helperCityRestore(t, s)
}

func TestCatanHelpersKnightsGregorOpensOpponentsClothRoute(t *testing.T) {
	s := ckClothKnightGraph(t)
	g := s.Catan
	s.Turn = 1
	g.TurnSerial = 10
	g.Options, _ = NormalizeCatanOptions(CatanOptions{Helpers: true, AllHelpers: true})
	g.CitiesKnights.Helpers = &catanHelpersKnights{Rules: CatanHelpersKnightsRules}
	for p, id := range []int{1, 8, 3} {
		g.Players[p].Helper = &CatanHelperSeat{ID: id}
	}
	for id := 1; id <= 12; id++ {
		if id != 1 && id != 8 && id != 3 {
			g.HelperDisplay = append(g.HelperDisplay, id)
		}
	}
	g.Vertices[7].Owner = 1
	g.Vertices[7].Level = 1
	helperGrant(s, 1, []int{0, 0, 0, 1, 2})
	s.catanClothTrade(0)
	if g.cloth().Held[0] != 0 {
		t.Fatal("fixture not blocked by knight")
	}
	helperApply(t, s, 1, Action{Type: "catan_city", Vertex: 7, Skill: "helper", Target: 2})
	if s.Catan.cloth().Held[0] != 2 || s.Catan.Players[0].Score != 2 {
		t.Fatal("removing helper knight did not reopen opponents' cloth routes")
	}
	helperCityRestore(t, s)
}

func TestCatanHelpersKnightsGregorViewVacatedVertex(t *testing.T) {
	s := helperCityFixture(t, 8)
	g := s.Catan
	// A connected endpoint with a knight becomes a legal village site only
	// after the selected knight is returned. Keep the live map unchanged.
	g.Vertices = []CatanVertex{{ID: 0, Owner: 0, Level: 1}, {ID: 1, Owner: -1}, {ID: 2, Owner: -1}}
	g.Edges = []CatanEdge{{ID: 0, A: 0, B: 1, Owner: 0}, {ID: 1, A: 1, B: 2, Owner: 0}}
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 2, Strength: 1}}
	before, _ := json.Marshal(g)
	sites := g.helperKnightBuilds(0)
	if !slices.Contains(sites[2]["settlements"], 2) || g.canSettlement(0, 2, false) {
		t.Fatal("vacated site missing or live knight removed", sites)
	}
	after, _ := json.Marshal(g)
	if string(before) != string(after) {
		t.Fatal("view calculation mutated map")
	}
	// Use a full legal board to exercise observer privacy and actor guards.
	s = helperCityFixture(t, 8)
	g = s.Catan
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1}}
	for _, viewer := range []int{-1, 0, 1} {
		view := s.View(viewer)["catan"].(map[string]any)
		if (view["helperKnightBuilds"] != nil) != (viewer == 0) {
			t.Fatal("wrong recipient of helper build targets", viewer)
		}
	}
	s.Phase = "catan_roll"
	if s.View(0)["catan"].(map[string]any)["helperKnightBuilds"] != nil {
		t.Fatal("premature build targets")
	}
}
