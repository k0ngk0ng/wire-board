package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var twoRiversSeaCases = []struct{ scenario, layout string }{{"shores", ""}, {"fog", ""}, {"desert", "rivers-across"}, {"desert", "desert-belt"}, {"tribe", ""}}

func TestCatanTwoRiversSeaNatural(t *testing.T) {
	for _, tc := range twoRiversSeaCases {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/events%t", tc.scenario, tc.layout, events), func(t *testing.T) {
				s, err := newCatanTwoRiversSeafarers(tc.scenario, tc.layout)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				diagnostic := twoRiversSeaDiagnostic{Test: t.Name(), State: s, Actions: map[string]int{}}
				if path := os.Getenv("WIRE_BOARD_TWO_RIVERS_REPLAY"); path != "" {
					raw, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if err = json.Unmarshal(raw, &diagnostic); err != nil {
						t.Fatal(err)
					}
					if diagnostic.Test != t.Name() || diagnostic.State == nil {
						t.Fatal("replay artifact does not match this test")
					}
					s = diagnostic.State
				}
				// Capture action errors, failed restore/conservation checks and a
				// long-game stall alike. No production rules or limits are changed.
				t.Cleanup(func() {
					if t.Failed() {
						diagnostic.save(t)
					}
				})
				limit := 20000
				if value := os.Getenv("WIRE_BOARD_TWO_RIVERS_STEPS"); value != "" {
					limit, err = strconv.Atoi(value)
					if err != nil || limit < 1 {
						t.Fatal("WIRE_BOARD_TWO_RIVERS_STEPS must be a positive integer")
					}
				}
				for step := 0; step < limit && !s.Finished; step++ {
					diagnostic.DoubleRollSeen = diagnostic.DoubleRollSeen || len(s.Catan.Two.Rolls) == 2
					diagnostic.NeutralBuildSeen = diagnostic.NeutralBuildSeen || s.Phase == "catan_two_build"
					actor := twoFullActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal("bot action", s.Phase, actor, err)
					}
					diagnostic.Steps++
					diagnostic.Actions[a.Type]++
					diagnostic.Recent = append(diagnostic.Recent, twoRiversSeaAction{actor, s.Phase, a})
					if len(diagnostic.Recent) > 32 {
						diagnostic.Recent = diagnostic.Recent[1:]
					}
					if err = s.Apply(actor, a); err != nil {
						t.Fatal("apply", actor, a, err)
					}
					if step%79 == 0 {
						twoSeaRestore(t, s)
						riversSeaRestore(t, s)
						riverConserved(t, s)
						s.View(-1)
					}
				}
				if !s.Finished || !diagnostic.DoubleRollSeen || !diagnostic.NeutralBuildSeen {
					t.Fatal("incomplete", s.Round, s.Phase, diagnostic.DoubleRollSeen, diagnostic.NeutralBuildSeen)
				}
				if err = s.validateTwoRiversSea(); err != nil {
					t.Fatal(err)
				}
				// A winner below the printed target is only legal when the
				// logged dead-end site rule finished a frozen table.
				if score := s.Catan.Players[s.Winners[0]].Score; score < s.Catan.victoryTargetFor(s.Winners[0]) {
					last := s.Log[len(s.Log)-1]
					if !strings.Contains(last, CatanTwoRiversSeaDeadEndRules) {
						t.Fatalf("winner scored %d below the target without the dead-end rule: %s", score, last)
					}
				}
				twoSeaRestore(t, s)
				riversSeaRestore(t, s)
				t.Log("round", s.Round, "winners", s.Winners)
			})
		}
	}
}

type twoRiversSeaAction struct {
	Actor  int    `json:"actor"`
	Phase  string `json:"phase"`
	Action Action `json:"action"`
}

type twoRiversSeaDiagnostic struct {
	DoubleRollSeen   bool                 `json:"doubleRollSeen"`
	NeutralBuildSeen bool                 `json:"neutralBuildSeen"`
	Test             string               `json:"test"`
	Steps            int                  `json:"steps"`
	State            *State               `json:"state"`
	Actions          map[string]int       `json:"actions"`
	Recent           []twoRiversSeaAction `json:"recent"`
}

func (d *twoRiversSeaDiagnostic) save(t *testing.T) {
	t.Helper()
	dir := filepath.Join("..", "..", ".local", "catan-two-rivers-failures")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Error("create diagnostic directory", err)
		return
	}
	f, err := os.CreateTemp(dir, "failure-*.json")
	if err != nil {
		t.Error("create diagnostic artifact", err)
		return
	}
	err = json.NewEncoder(f).Encode(d)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Error("write diagnostic artifact", err, closeErr)
		return
	}
	t.Logf("saved state and last 32 actions: %s; actions=%v", f.Name(), d.Actions)
}
func TestCatanTwoRiversSeaRecipes(t *testing.T) {
	for _, tc := range twoRiversSeaCases {
		s, err := newCatanTwoRiversSeafarers(tc.scenario, tc.layout)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if len(g.Players) != 2 || len(g.Rivers.Gold) != 2 || len(g.Seafarers.Seats) != 2 || len(g.Two.SeaStarts) != 2 || g.Paired != nil || g.Rivers.Bank != 100 {
			t.Fatal("recipe seats")
		}
		if tr := g.tribe(); tr != nil && (len(tr.Points) != 2 || len(tr.HeldPorts) != 2) {
			t.Fatal("tribe live seats")
		}
		for _, rules := range []string{"", CatanTwoRiversWorldRules, "unknown"} {
			c := clone(*s)
			c.Catan.Two.RiversSea = rules
			if c.validateCatanTwo() == nil || c.Catan.validateRivers() == nil {
				t.Fatal("invalid version", tc, rules)
			}
		}
		c := clone(*s)
		c.Catan.Rivers = nil
		if c.validateCatanTwo() == nil {
			t.Fatal("orphan marker")
		}
	}
	for _, scenario := range []string{"islands", "cloth", "pirate_islands", "wonders", "new_world"} {
		if _, err := newCatanTwoRiversSeafarers(scenario, ""); err == nil {
			t.Fatal("wrong scenario", scenario)
		}
	}
}

func TestCatanTwoRiversSeaNeutralExploration(t *testing.T) {
	s, err := newCatanTwoRiversSeafarers("fog", "")
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	g.Two.Rolls = []int{6, 8}
	edge := -1
	for _, e := range g.Edges {
		if len(g.fogAtRoute(e.ID)) > 0 {
			edge = e.ID
			break
		}
	}
	if edge < 0 {
		t.Fatal("no fog edge")
	}
	bank, gold, tokens := slices.Clone(g.Bank), slices.Clone(g.Rivers.Gold), slices.Clone(g.Two.Tokens)
	players := clone(g.Players)
	before := len(g.Seafarers.Fog.Terrain)
	reward, err := s.catanDiscover(-2, edge)
	if err != nil {
		t.Fatal(err)
	}
	if reward != 0 || g.GoldPending != nil || len(g.Seafarers.Fog.Terrain) >= before || !slices.Equal(bank, g.Bank) || !slices.Equal(gold, g.Rivers.Gold) || !slices.Equal(tokens, g.Two.Tokens) || !reflect.DeepEqual(players, g.Players) {
		t.Fatal("neutral discovery paid reward")
	}
	twoSeaRestore(t, s)
	riversSeaRestore(t, s)
}

func TestCatanTwoRiversSeaTribeRewards(t *testing.T) {
	for _, kind := range []string{"point", "card", "port"} {
		t.Run(kind, func(t *testing.T) {
			s, err := newCatanTwoRiversSeafarers("tribe", "")
			if err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			tr := g.tribe()
			edge := tr.Tokens[0]
			if kind == "card" {
				edge = tr.Development[0].Edge
			}
			if kind == "port" {
				edge = tr.Ports[0].Edge
			}
			e := g.Edges[edge]
			g.Vertices[e.A].Owner, g.Vertices[e.A].Level = -2, 1
			if !g.canShip(-2, edge) {
				t.Fatal("fixture does not reach reward")
			}
			for _, c := range g.twoNeutralShipChoices() {
				if c.Edge == edge {
					t.Fatal("neutral can block reward")
				}
			}
			g.Vertices[e.A].Owner, g.Vertices[e.A].Level = -1, 0
			p := s.Turn
			g.SetupStep = g.SetupLimit()
			s.Phase = "catan_turn"
			g.Two.Rolls = []int{6, 8}
			g.Edges[edge].Owner, g.Edges[edge].Ship = p, true
			points, cards, ports := tr.Points[p], sum(g.Players[p].Dev), len(tr.HeldPorts[p])
			if err = s.catanCollectTribe(p, edge); err != nil {
				t.Fatal(err)
			}
			if kind == "point" && tr.Points[p] != points+1 || kind == "card" && sum(g.Players[p].Dev) != cards+1 || kind == "port" && len(tr.HeldPorts[p]) != ports+1 {
				t.Fatal("human reward missing")
			}
			twoSeaRestore(t, s)
			riversSeaRestore(t, s)
		})
	}
}

func TestCatanTwoRiversSeaFogGoldContinuation(t *testing.T) {
	for _, free := range []bool{false, true} {
		t.Run(fmt.Sprint(free), func(t *testing.T) {
			s, err := newCatanTwoRiversSeafarers("fog", "")
			if err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			p := s.Turn
			g.SetupStep = g.SetupLimit()
			g.Two.Rolls = []int{6, 8}
			s.Phase = "catan_turn"
			target := -1
			for _, e := range g.Edges {
				if len(g.fogAtRoute(e.ID)) != 1 || !g.edgeTerrain(e.ID, true) {
					continue
				}
				for _, v := range []int{e.A, e.B} {
					if !g.canSettlement(p, v, true) {
						continue
					}
					g.Vertices[v].Owner, g.Vertices[v].Level = p, 1
					if g.canShip(p, e.ID) {
						target = e.ID
						break
					}
					g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
				}
				if target >= 0 {
					break
				}
			}
			if target < 0 {
				t.Fatal("no fog approach")
			}
			fog := g.Seafarers.Fog
			i := slices.Index(fog.Terrain, CatanGold)
			if i < 0 {
				t.Fatal("no gold")
			}
			fog.Terrain[i], fog.Terrain[len(fog.Terrain)-1] = fog.Terrain[len(fog.Terrain)-1], fog.Terrain[i]
			resume := "catan_turn"
			if free {
				s.Phase = "catan_roads"
				g.FreeRoads = 2
				g.ResumePhase = "catan_turn"
				resume = "catan_roads"
			} else {
				helperGrant(s, p, catanPrices["catan_ship"])
			}
			helperApply(t, s, p, Action{Type: "catan_ship", Edge: target})
			if s.Phase != "catan_gold" || s.Catan.Two.AfterRoute != "ship" || s.Catan.Two.Pending != nil {
				t.Fatal("lost deferred neutral")
			}
			twoSeaRestore(t, s)
			riversSeaRestore(t, s)
			helperApply(t, s, p, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
			if s.Phase != "catan_two_build" || s.Catan.Two.Pending.Kind != "ship" {
				t.Fatal("gold skipped neutral")
			}
			twoSeaStep(t, s)
			if s.Phase != resume || s.Catan.Two.Pending != nil || s.Catan.Two.AfterRoute != "" {
				t.Fatal("wrong resume", s.Phase)
			}
			twoSeaRestore(t, s)
			riversSeaRestore(t, s)
			riverConserved(t, s)
		})
	}
}

func TestCatanTwoRiversSeaPrivacyAndCoast(t *testing.T) {
	for _, scenario := range []string{"fog", "tribe"} {
		s, err := newCatanTwoRiversSeafarers(scenario, "")
		if err != nil {
			t.Fatal(err)
		}
		c := clone(*s)
		if scenario == "fog" {
			slices.Reverse(c.Catan.Seafarers.Fog.Terrain)
			slices.Reverse(c.Catan.Seafarers.Fog.Numbers)
		} else {
			tr := c.Catan.tribe()
			for i, j := 0, len(tr.Development)-1; i < j; i, j = i+1, j-1 {
				tr.Development[i].Card, tr.Development[j].Card = tr.Development[j].Card, tr.Development[i].Card
			}
		}
		for _, viewer := range []int{-1, 0, 1} {
			a, _ := json.Marshal(s.View(viewer))
			b, _ := json.Marshal(c.View(viewer))
			if string(a) != string(b) {
				t.Fatal("hidden cards leaked", scenario, viewer)
			}
		}
		for _, v := range s.Catan.Two.SeaStarts {
			want := 1
			for _, id := range s.Catan.Rivers.Map.Swamps {
				if slices.Contains(s.Catan.Tiles[id].Vertices, v) {
					want = 3
				}
			}
			if s.Catan.twoSettlementTokens(0, v) != want || s.Catan.twoSettlementTokens(-2, v) != 0 {
				t.Fatal("coastal/swamp tokens")
			}
		}
	}
}

func TestCatanTwoRiversSeaSwampRetreat(t *testing.T) {
	cases := append(slices.Clone(twoRiversSeaCases), struct{ scenario, layout string }{"new_world", ""})
	for _, tc := range cases {
		var s *State
		var err error
		if tc.scenario == "new_world" {
			s, err = newCatanTwoRiversWorld(nil)
		} else {
			s, err = newCatanTwoRiversSeafarers(tc.scenario, tc.layout)
		}
		if err != nil {
			t.Fatal(err)
		}
		for s.Phase == "catan_world_ports" {
			twoSeaStep(t, s)
		}
		g := s.Catan
		p := s.Turn
		g.SetupStep = g.SetupLimit()
		g.Two.Rolls = []int{6, 8}
		s.Phase = "catan_turn"
		for _, tile := range g.Tiles {
			if tile.Resource < 5 {
				g.Robber = tile.ID
				break
			}
		}
		before := clone(g.Players)
		gold := slices.Clone(g.Rivers.Gold)
		pirate := g.Seafarers.Pirate
		if !slices.Equal(g.twoRetreatTiles(), g.Rivers.Map.Swamps) {
			t.Fatal("swamp overridden", tc)
		}
		helperReject(t, s, p, Action{Type: "catan_two_robber", Tile: -1})
		target := g.Rivers.Map.Swamps[0]
		helperApply(t, s, p, Action{Type: "catan_two_robber", Tile: target})
		g = s.Catan
		if g.Robber != target || g.Seafarers.Pirate != pirate || !slices.Equal(gold, g.Rivers.Gold) {
			t.Fatal("retreat target/ledger")
		}
		for i := range before {
			if !slices.Equal(before[i].Resources, g.Players[i].Resources) {
				t.Fatal("retreat stole")
			}
		}
		twoSeaRestore(t, s)
		riversSeaRestore(t, s)
	}
}
