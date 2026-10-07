package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// A production-controller fixture built through legal complete setup. Mission
// controllers are deliberately not installed: full combined Apply remains gated.
func explorerCityProductionFixture(t *testing.T, n int) *State {
	t.Helper()
	return explorerCityScenarioFixture(t, n, "pirate-lairs")
}
func explorerCityScenarioFixture(t *testing.T, n int, scenario string) *State {
	t.Helper()
	q := explorerKnightsSetupFixture(t, n, 0, scenario)
	path := explorerKnightsCompletableOpening(t, q, rand.New(rand.NewSource(307)))
	for _, target := range path {
		step := q.S.current(n)
		if err := q.place(step.Player, q.S.Step+1, step.Kind, target); err != nil {
			t.Fatal(err)
		}
	}
	q.G.Explorer = &catanExplorer{Board: q.B, Fleet: q.F, Cargo: q.C, Economy: q.E, Pirate: newCatanExplorerPirate()}
	s := &State{Kind: "catan", Catan: q.G, Turn: 0, Round: 1, Phase: "catan_roll", Log: []string{}}
	if err := s.validateExplorerCityProduction(); err != nil {
		t.Fatal(err)
	}
	return s
}
func explorerCityRestore(t *testing.T, s *State) {
	t.Helper()
	before, _ := json.Marshal(s)
	var next State
	if err := json.Unmarshal(before, &next); err != nil {
		t.Fatal(err)
	}
	if err := next.validateExplorerCityProduction(); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(&next)
	if string(before) != string(after) {
		t.Fatal("production restore changed state")
	}
	*s = next
	ckProgressStock(t, s.Catan)
}
func explorerCityReject(t *testing.T, s *State, player int, a Action) {
	t.Helper()
	before := clone(*s)
	if err := s.catanExplorerCityRespond(player, a); err == nil {
		t.Fatal("illegal city response accepted", player, a)
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("rejected response changed state")
	}
}
func explorerCityClaims(g *Catan, number int) [][]int {
	out := make([][]int, len(g.Players))
	for p := range out {
		out[p] = make([]int, 8)
	}
	commodity := map[int]int{0: 5, 2: 6, 4: 7}
	for _, tile := range g.Tiles {
		if tile.Number != number || tile.Resource < 0 || tile.Resource > 4 {
			continue
		}
		for _, id := range tile.Vertices {
			v := g.Vertices[id]
			if v.Owner < 0 || v.Level == 0 {
				continue
			}
			out[v.Owner][tile.Resource]++
			if v.Level == 2 && !v.Harbor {
				if c, ok := commodity[tile.Resource]; ok {
					out[v.Owner][c]++
				} else {
					out[v.Owner][tile.Resource]++
				}
			}
		}
	}
	return out
}
func TestCatanExplorerCityProductionTerrainAndCommodity(t *testing.T) {
	for _, n := range []int{3, 6} {
		for terrain := 0; terrain < 5; terrain++ {
			t.Run(fmt.Sprintf("%d/%d", n, terrain), func(t *testing.T) {
				s := explorerCityProductionFixture(t, n)
				g := s.Catan
				city := -1
				for _, v := range g.Vertices {
					if v.Owner == 0 && g.cityAt(v.ID) {
						city = v.ID
						break
					}
				}
				tile := -1
				for _, id := range g.Explorer.Board.Starting {
					if id != g.Explorer.Board.FramePasture && slices.Contains(g.Tiles[id].Vertices, city) {
						tile = id
						break
					}
				}
				if tile < 0 {
					t.Fatal("fixture city lacks movable terrain")
				}
				for _, id := range g.Explorer.Board.Starting {
					if id != g.Explorer.Board.FramePasture && g.Tiles[id].Resource == terrain {
						g.Tiles[id].Resource, g.Tiles[tile].Resource = g.Tiles[tile].Resource, g.Tiles[id].Resource
						break
					}
				}
				if g.Tiles[tile].Resource != terrain {
					t.Fatal("missing terrain inventory")
				}
				number := g.Tiles[tile].Number
				red := min(6, number-1)
				yellow := number - red
				claims := explorerCityClaims(g, number)
				old := clone(*g)
				if err := s.catanExplorerCityRoll(red, yellow, 3); err != nil {
					t.Fatal(err)
				}
				g = s.Catan
				for p := range g.Players {
					for card, gain := range claims[p] {
						if g.Players[p].Resources[card] != old.Players[p].Resources[card]+gain {
							t.Fatal("wrong production", p, card)
						}
					}
					bonus := 0
					if sum(claims[p]) == 0 {
						bonus = 1
					}
					if g.Explorer.Economy.Gold[p] != old.Explorer.Economy.Gold[p]+bonus {
						t.Fatal("wrong no-card gold")
					}
				}
				if s.Phase != "catan_turn" || g.Explorer.Cargo.Turn == nil || g.RollID != 1 || g.CitiesKnights.BarbarianPosition != 1 {
					t.Fatal("missing action handoff")
				}
				before := clone(*s)
				if err := s.catanExplorerCityRoll(red, yellow, 3); err == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("duplicate roll accepted")
				}
				explorerCityRestore(t, s)
			})
		}
	}
}
func TestCatanExplorerCityPillageBeforeProduction(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	s.Catan.CitiesKnights.BarbarianPosition = 6
	initial := clone(*s.Catan)
	cities := []int{}
	for _, v := range s.Catan.Vertices {
		if s.Catan.cityAt(v.ID) {
			cities = append(cities, v.ID)
		}
	}
	s.Catan.CitiesKnights.Walls = slices.Clone(cities)
	if err := s.catanExplorerCityRoll(3, 3, 3); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if s.Phase != "catan_pillage" || s.Catan.Explorer.Cargo.Turn != nil {
			t.Fatal("pillage must suspend production")
		}
		for p := range s.Catan.Players {
			if !slices.Equal(initial.Players[p].Resources, s.Catan.Players[p].Resources) {
				t.Fatal("early payout")
			}
		}
		p := s.CatanPendingActor()
		city := s.Catan.pillageSites(p)[0]
		harbor := -1
		for _, v := range s.Catan.Vertices {
			if v.Owner == p && s.Catan.harborAt(v.ID) {
				harbor = v.ID
				break
			}
		}
		explorerCityReject(t, s, p, Action{Type: "catan_pillage", Vertex: harbor, Prompt: 1})
		explorerCityReject(t, s, (p+1)%3, Action{Type: "catan_pillage", Vertex: city, Prompt: 1})
		if err := s.catanExplorerCityRespond(p, Action{Type: "catan_pillage", Vertex: city, Prompt: 1}); err != nil {
			t.Fatal(err)
		}
		explorerCityRestore(t, s)
	}
	g := s.Catan
	claims := explorerCityClaims(g, 6)
	for p := range g.Players {
		if g.Players[p].Score != 3 {
			t.Fatal("pillage lost harbor points")
		}
		for r, gain := range claims[p] {
			if g.Players[p].Resources[r] != initial.Players[p].Resources[r]+gain {
				t.Fatal("produced as pre-pillage city")
			}
		}
	}
	if s.Phase != "catan_turn" || g.CitiesKnights.Invasions != 1 || len(g.CitiesKnights.Walls) != 0 || g.Robber != -1 || strings.Contains(strings.Join(s.Log, " "), "强盗进入") {
		t.Fatal("wrong post-attack state/log")
	}
}
func TestCatanExplorerCityDrawDiscardThenProductionPrivacy(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	s.Catan.CitiesKnights.Players[1].Improvements[0] = 1
	s.Catan.CitiesKnights.Players[2].Improvements[0] = 1
	ckProgressGive(t, s, 1, 3, 4, 5, 6)
	ckProgressTop(t, s, 0, 1)
	initial := clone(*s.Catan)
	if err := s.catanExplorerCityRoll(1, 5, 0); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_progress_discard" || s.CatanPendingActor() != 1 || len(s.Catan.CitiesKnights.Players[2].Progress) != 0 {
		t.Fatal("draw order")
	}
	for p := range s.Catan.Players {
		if !slices.Equal(initial.Players[p].Resources, s.Catan.Players[p].Resources) {
			t.Fatal("production before overflow response")
		}
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		v := s.View(viewer)["catan"].(map[string]any)["citiesKnights"].(map[string]any)
		if _, exists := v["progressDecks"]; exists {
			t.Fatal("deck leaked")
		}
		for p, raw := range v["players"].([]any) {
			_, seen := raw.(map[string]any)["progress"]
			if seen != (p == viewer) {
				t.Fatal("hand privacy")
			}
		}
	}
	other := clone(*s)
	slices.Reverse(other.Catan.CitiesKnights.ProgressDecks[0])
	slices.Reverse(other.Catan.CitiesKnights.Players[1].Progress)
	if !reflect.DeepEqual(s.View(-1), other.View(-1)) {
		t.Fatal("observer sees private card order")
	}
	explorerCityRestore(t, s)
	explorerCityReject(t, s, 0, Action{Type: "catan_progress_discard", Cards: []int{3}, Prompt: 1})
	explorerCityReject(t, s, 1, Action{Type: "catan_progress_discard", Cards: []int{3}, Prompt: 0})
	explorerCityReject(t, s, 1, Action{Type: "catan_progress_discard", Cards: []int{3, 3}, Prompt: 1})
	if err := s.catanExplorerCityRespond(1, Action{Type: "catan_progress_discard", Cards: []int{3}, Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || len(s.Catan.CitiesKnights.Players[1].Progress) != 4 || !slices.Equal(s.Catan.CitiesKnights.Players[2].Progress, []int{1}) {
		t.Fatal("draw/production not resumed")
	}
	if strings.Contains(strings.Join(s.Log, " "), "Alchemy") {
		t.Fatal("private draw leaked in log")
	}
	explorerCityRestore(t, s)
}
func TestCatanExplorerCityAqueductGoldAndPausedAction(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	for p := range s.Catan.Players {
		s.Catan.CitiesKnights.Players[p].Improvements[0] = 3
	}
	// Starting island has no 2: all receive one gold and queue the aqueduct.
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	for p := 0; p < 3; p++ {
		g, x := s.Catan, s.Catan.Explorer
		if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != p || x.Cargo.Turn != nil {
			t.Fatal("aqueduct did not pause action")
		}
		for _, gold := range x.Economy.Gold {
			if gold != 3 {
				t.Fatal("gold bonus absent or repeated")
			}
		}
		before := clone(*s)
		if err := x.Economy.bankTrade(g, x.Fleet, x.Cargo, 0, 1, -1, 0); err == nil || !reflect.DeepEqual(*s, before) {
			t.Fatal("trade accepted during compensation")
		}
		explorerCityReject(t, s, p, Action{Type: "catan_aqueduct", Color: 5, Prompt: 1})
		if err := s.catanExplorerCityRespond(p, Action{Type: "catan_aqueduct", Color: p, Prompt: 1}); err != nil {
			t.Fatal(err)
		}
		explorerCityRestore(t, s)
	}
	if s.Phase != "catan_turn" || s.Catan.Explorer.Cargo.Turn == nil || s.Catan.Explorer.Economy.Turn.Phase != "ready" {
		t.Fatal("no final action handoff")
	}
}
func TestCatanExplorerCitySevenIncludesCommoditiesAndWalls(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	g := s.Catan
	for p := range g.Players {
		for r, n := range g.Players[p].Resources {
			g.Bank[r] += n
			g.Players[p].Resources[r] = 0
		}
	}
	for p, amount := range []int{9, 8, 7} {
		g.Players[p].Resources[5] = 2
		g.Bank[5] -= 2
		g.Players[p].Resources[0] = amount - 2
		g.Bank[0] -= amount - 2
	}
	for _, v := range g.Vertices {
		if v.Owner == 0 && g.cityAt(v.ID) {
			g.CitiesKnights.Walls = []int{v.ID}
		}
	}
	if err := s.catanExplorerCityRoll(3, 4, 3); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	x := g.Explorer
	if s.Phase != "catan_discard" || !slices.Equal(g.DiscardDue, []int{0, 4, 0}) {
		t.Fatal("wrong city wall / commodity discard", g.DiscardDue)
	}
	for _, gold := range x.Economy.Gold {
		if gold != 2 {
			t.Fatal("seven pays gold compensation")
		}
	}
	cards := []int{2, 0, 0, 0, 0, 2, 0, 0}
	if err := x.Economy.discard(g, x.Fleet, x.Cargo, 1, 1, cards); err != nil {
		t.Fatal(err)
	}
	if err := s.catanExplorerAfterProduction(); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_explorer_pirate_place" || g.Robber != -1 {
		t.Fatal("seven did not activate Explorer pirate")
	}
	target := x.Pirate.destinations(g, x.Board)[0]
	if _, err := x.Pirate.apply(g, x.Board, x.Fleet, x.Cargo, x.Economy, 0, 1, "place", target, false, nil); err != nil {
		t.Fatal(err)
	}
	s.catanExplorerSyncPhase()
	if s.Phase != "catan_turn" {
		t.Fatal("pirate did not finish into action", s.Phase)
	}
	explorerCityRestore(t, s)
}
func TestCatanExplorerCityLastResponseLedgerOverflowIsAtomic(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	s.Catan.CitiesKnights.BarbarianPosition = 6
	e := s.Catan.Explorer.Economy
	e.GoldIssued = catanExplorerGoldLedgerLimit
	e.Gold[1] += e.GoldIssued
	e.Gold[1] += e.GoldBank
	e.GoldBank = 0
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		p := s.CatanPendingActor()
		v := s.Catan.pillageSites(p)[0]
		if err := s.catanExplorerCityRespond(p, Action{Type: "catan_pillage", Vertex: v, Prompt: 1}); err != nil {
			t.Fatal(err)
		}
	}
	p := s.CatanPendingActor()
	v := s.Catan.pillageSites(p)[0]
	explorerCityReject(t, s, p, Action{Type: "catan_pillage", Vertex: v, Prompt: 1})
	if !s.Catan.cityAt(v) || s.Catan.CitiesKnights.Invasions != 0 || s.Catan.CitiesKnights.Event == nil {
		t.Fatal("failed payout partially finished invasion")
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityCommodityOnlyProductionAvoidsCompensation(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	g := s.Catan
	city := -1
	for _, v := range g.Vertices {
		if v.Owner == 0 && g.cityAt(v.ID) {
			city = v.ID
			break
		}
	}
	tile := -1
	for _, id := range g.Explorer.Board.Starting {
		if id != g.Explorer.Board.FramePasture && slices.Contains(g.Tiles[id].Vertices, city) {
			tile = id
			break
		}
	}
	for _, id := range g.Explorer.Board.Starting {
		if g.Tiles[id].Resource == 0 {
			g.Tiles[id].Resource, g.Tiles[tile].Resource = g.Tiles[tile].Resource, g.Tiles[id].Resource
			break
		}
	}
	for p := range g.Players {
		g.CitiesKnights.Players[p].Improvements[0] = 3
	}
	for r := 0; r < 5; r++ {
		g.Players[2].Resources[r] += g.Bank[r]
		g.Bank[r] = 0
	}
	before := clone(*g)
	number := g.Tiles[tile].Number
	red := min(6, number-1)
	if err := s.catanExplorerCityRoll(red, number-red, 3); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	if g.Players[0].Resources[5] <= before.Players[0].Resources[5] || g.Explorer.Economy.Gold[0] != before.Explorer.Economy.Gold[0] {
		t.Fatal("commodity-only recipient got gold compensation")
	}
	for r := 0; r < 5; r++ {
		if g.Players[0].Resources[r] != before.Players[0].Resources[r] {
			t.Fatal("depleted ordinary supply produced cards")
		}
	}
	if s.Phase != "catan_turn" || g.CitiesKnights.Pending != nil {
		t.Fatal("empty ordinary bank blocked compensation")
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityDefenderVictoryBeforePayout(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	g := s.Catan
	for i := range g.Vertices {
		if g.Vertices[i].Owner > 0 && g.cityAt(i) {
			g.Vertices[i].Level = 1
		}
	}
	knight := -1
	for _, edge := range g.Edges {
		if edge.Owner == 0 {
			for _, v := range []int{edge.A, edge.B} {
				if g.Vertices[v].Level == 0 {
					knight = v
				}
			}
		}
	}
	if knight < 0 {
		t.Fatal("missing own road knight site")
	}
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: knight, Strength: 1, Active: true}}
	g.CitiesKnights.Players[0].DefenderPoints = 12
	g.CitiesKnights.BarbarianPosition = 6
	s.catanScores()
	if g.Players[0].Score != 16 {
		t.Fatal("wrong pre-victory score")
	}
	before := clone(*g)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	if !s.Finished || !slices.Equal(s.Winners, []int{0}) || g.Players[0].Score != 17 || g.CitiesKnights.Knights[0].Active {
		t.Fatal("defender victory did not use combo target")
	}
	for p := range g.Players {
		if !slices.Equal(g.Players[p].Resources, before.Players[p].Resources) || g.Explorer.Economy.Gold[p] != before.Explorer.Economy.Gold[p] {
			t.Fatal("production happened after immediate victory")
		}
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityProductionAcrossOrdinaryAndPairedTurns(t *testing.T) {
	for _, n := range []int{3, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityProductionFixture(t, n)
			rolls := 0
			for portion := 0; portion < 4; portion++ {
				g, x := s.Catan, s.Catan.Explorer
				priorGold := slices.Clone(x.Economy.Gold)
				claims := explorerCityClaims(g, 2)
				if n > 4 && portion%2 == 1 {
					if s.Phase != "catan_turn" || !x.Economy.Turn.NoProduction || g.RollID != rolls {
						t.Fatal("secondary repeats production")
					}
					before := clone(*s)
					if err := s.catanExplorerCityRoll(1, 1, 3); err == nil || !reflect.DeepEqual(*s, before) {
						t.Fatal("secondary roll accepted")
					}
				} else {
					if s.Phase != "catan_roll" {
						t.Fatal("next primary missing roll")
					}
					if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
						t.Fatal(err)
					}
					rolls++
					g, x = s.Catan, s.Catan.Explorer
					for p, gold := range x.Economy.Gold {
						bonus := 0
						if sum(claims[p]) == 0 {
							bonus = 1
						}
						if gold != priorGold[p]+bonus {
							t.Fatal("one bonus per actual production")
						}
					}
				}
				if g.RollID != rolls || g.CitiesKnights.BarbarianPosition != rolls {
					t.Fatal("event die advanced without production")
				}
				handoffGold := slices.Clone(x.Economy.Gold)
				g.CitiesKnights.TradePowers = &CatanTradePowers{}
				if err := x.Cargo.beginMovement(g, x.Fleet, s.Turn, g.TurnSerial, 0); err != nil {
					t.Fatal(err)
				}
				if err := x.Cargo.endMovement(g, x.Fleet, s.Turn, g.TurnSerial); err != nil {
					t.Fatal(err)
				}
				if err := s.catanExplorerNextTurn(); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(handoffGold, g.Explorer.Economy.Gold) {
					t.Fatal("handoff paid extra production compensation")
				}
				if g.CitiesKnights.ActionSerial != g.TurnSerial || g.CitiesKnights.TradePowers != nil {
					t.Fatal("city action effects did not reset at handoff")
				}
				explorerCityRestore(t, s)
			}
		})
	}
}
