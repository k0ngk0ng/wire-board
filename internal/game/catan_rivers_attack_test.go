package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func riversAttackRestore(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanAttack(); err != nil {
		t.Fatal(err)
	}
	if err = next.Catan.validateRivers(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanTwo(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	*s = next
}
func TestCatanRiversAttackNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := NewCatanRiversAttack(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 10000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					if step%37 == 0 {
						riversAttackRestore(t, s)
						for viewer := -1; viewer < n; viewer++ {
							s.View(viewer)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				riversAttackRestore(t, s)
				t.Log("round", s.Round, "winner", s.Winners, "landings", s.Catan.Attack.Sequence)
			})
		}
	}
}

func riversAttackPlaying(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanRiversAttack(n)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; s.Catan.setup() && step < 80; step++ {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e == nil {
			e = s.Apply(p, a)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if s.Catan.setup() {
		t.Fatal("setup unfinished")
	}
	s.Phase = "catan_turn"
	return s
}

func TestCatanRiversAttackSharedGold(t *testing.T) {
	s := riversAttackPlaying(t, 3)
	g := s.Catan
	p := s.Turn
	a := g.Attack
	before := a.Gold[p]
	if err := s.catanRiverReward(p, 5); err != nil {
		t.Fatal(err)
	}
	if g.tradeGold()[p] != before+5 || g.riverGold()[p] != before+5 || len(g.Rivers.Gold) != 0 {
		t.Fatal("separate river balance")
	}
	if err := s.Apply(p, Action{Type: "catan_coin_buy", Color: 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(p, Action{Type: "catan_coin_buy", Color: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(p, Action{Type: "catan_coin_buy", Color: 2}); err == nil {
		t.Fatal("bypassed shared buy limit")
	}
	g = s.Catan
	a = g.Attack
	if a.Bought != 2 || g.Rivers.Bought != 0 || a.Gold[p] != before+1 {
		t.Fatal("wrong shared ledger")
	}
	// Exhaust the physical bank without destroying conservation, then issue
	// river construction rewards through the same attack ledger.
	a.Gold[p] += a.GoldBank
	a.GoldBank = 0
	if err := s.catanRiverReward(p, 3); err != nil {
		t.Fatal(err)
	}
	if a.GoldIssued != 3 || a.GoldBank != 0 {
		t.Fatal("river reward did not issue in shared ledger")
	}
	riversAttackRestore(t, s)
	s.View(p)
	for q := range s.Catan.Players {
		if s.Catan.riverPoints(q) < 0 {
			t.Fatal("poverty deducted points")
		}
	}
	broken := clone(*s)
	broken.Catan.Rivers.Gold = make([]int, 3)
	if broken.Catan.validateRivers() == nil {
		t.Fatal("duplicate ledger accepted")
	}
	base, err := NewCatanRivers(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if base.Catan.riverPoints(0) != -2 {
		t.Fatal("ordinary river poverty changed")
	}
}

func TestCatanRiversAttackBridgeConquest(t *testing.T) {
	s := riversAttackPlaying(t, 3)
	g := s.Catan
	p := s.Turn
	id := g.Rivers.Map.Bridges[len(g.Rivers.Map.Bridges)-1]
	e := g.Edges[id]
	g.Vertices[e.A].Owner = p
	g.Vertices[e.A].Level = 1
	if !g.canBridge(p, id) {
		t.Fatal("unoccupied river mouth cannot be bridged")
	}
	// The outlet itself is a swamp; test a productive coastal bridge instead.
	for _, edge := range g.Rivers.Map.Bridges {
		for _, tile := range g.Edges[edge].Tiles {
			if slices.Contains(g.Attack.Map.Coast, tile) {
				g.Edges[edge].Owner = -1
				g.Edges[edge].Bridge = false
				g.Vertices[g.Edges[edge].A].Owner = p
				g.Vertices[g.Edges[edge].A].Level = 1
				g.Attack.Barbarians[tile] = 0
				if !g.canBridge(p, edge) {
					t.Fatal("productive coast cannot be bridged before conquest")
				}
				g.Attack.Barbarians[tile] = 3
				if g.canBridge(p, edge) {
					t.Fatal("bridge allowed beside conquest")
				}
				return
			}
		}
	}
	t.Fatal("missing coastal bridge fixture")
}

func TestCatanRiversAttackBlockedCastleContinues(t *testing.T) {
	s := riversAttackPlaying(t, 6)
	s.Turn = 1
	g := s.Catan
	attackHand(s, 1, []int{0, 0, 0, 0, 0})
	g.Attack.Knights = []catanAttackKnight{{0, 11}, {1, 86}, {0, 42}, {1, 31}, {3, 16}, {2, 54}, {2, 89}, {5, 33}, {5, 88}, {0, 0}, {5, 37}, {0, 27}, {5, 56}, {2, 58}, {5, 75}, {0, 35}, {4, 17}, {1, 49}, {1, 50}}
	for i := range g.Attack.Barbarians {
		g.Attack.Barbarians[i] = 0
	}
	if err := s.Apply(1, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	for step := 0; s.Phase == "catan_attack_end" && step < 15; step++ {
		a, err := s.BotAction(1)
		if err == nil {
			err = s.Apply(1, a)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.Phase == "catan_attack_end" {
		t.Fatal("castle still blocks turn")
	}
	found := false
	for _, line := range s.Log {
		found = found || strings.Contains(line, "本回合暂留城堡")
	}
	if !found {
		t.Fatal("supplement not logged")
	}
	riversAttackRestore(t, s)
}

func TestCatanRiversAttackBlockedOwnKnightAllowsNeutral(t *testing.T) {
	s := riversAttackPlaying(t, 2)
	g := s.Catan
	p := s.Turn
	attackHand(s, p, []int{0, 0, 0, 0, 0})
	// Surround one castle knight with occupied destinations. Passing through
	// knights is legal, but ending on one is not. Keep each army within six.
	for _, edge := range g.Edges {
		if !g.Attack.castleEdge(g, edge.ID) {
			continue
		}
		g.Attack.Knights = []catanAttackKnight{{p, edge.ID}}
		blocked := g.Attack.knightDestinations(g, 0, 3)
		if len(blocked) > 16 {
			continue
		}
		ids := []int{}
		for id := range blocked {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for i, id := range ids {
			owner := 1 - p
			if i >= 6 {
				owner = catanAttackNeutral
			}
			if i >= 12 {
				owner = p
			}
			g.Attack.Knights = append(g.Attack.Knights, catanAttackKnight{owner, id})
		}
		if !g.riverAttackCastleBlocked(0, p) {
			t.Fatal("fixture did not block castle knight")
		}
		g.Attack.EndPlan = &catanAttackEndPlan{ID: 1, Player: p}
		choices, _ := s.catanAttackPlanChoices(s)
		for _, choice := range choices {
			neutral := false
			for _, k := range g.Attack.Knights {
				neutral = neutral || k.Edge == choice.From && k.Player == catanAttackNeutral
			}
			if neutral && len(choice.Normal) > 0 {
				if err := s.catanAttackMoveKnights([]catanAttackMove{{From: choice.From, To: choice.Normal[0]}}, false); err != nil {
					t.Fatal("blocked own knight prevented neutral movement:", err)
				}
				return
			}
		}
	}
	t.Fatal("no legal neutral choice alongside blocked castle knight")
}
