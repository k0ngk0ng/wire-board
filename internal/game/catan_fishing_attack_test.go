package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func fishingAttackRestore(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err = restored.validateCatanAttack(); err != nil {
		t.Fatal(err)
	}
	if err = restored.Catan.validateFishing(); err != nil {
		t.Fatal(err)
	}
	if err = restored.validateCatanTwo(); err != nil {
		t.Fatal(err)
	}
	if err = restored.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	*s = restored
}
func TestCatanFishingAttackNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanFishingAttack(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				actions := map[string]int{}
				for step := 0; step < 16000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					actions[a.Type]++
					if step%83 == 0 {
						fishingAttackRestore(t, s)
						for viewer := -1; viewer < n; viewer++ {
							s.View(viewer)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round, s.Phase)
				}
				t.Log("rounds", s.Round, "fish dev", actions["catan_fish_dev"], "fish roads", actions["catan_fish_road"])
			})
		}
	}
}
func fishingAttackAfterSetup(t *testing.T, n int) *State {
	t.Helper()
	s, e := newCatanFishingAttack(n)
	if e != nil {
		t.Fatal(e)
	}
	for s.Catan.setup() {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	return s
}
func fishingAttackHand(t *testing.T, s *State, p, amount int) {
	t.Helper()
	f := &s.Catan.Fishing.Tokens
	for i := range f.Hands {
		f.DrawPile = append(f.DrawPile, f.Hands[i]...)
		f.Hands[i] = nil
	}
	for amount > 0 {
		i := slices.IndexFunc(f.DrawPile, func(id int) bool { return catanFishValue(id) > 0 && catanFishValue(id) <= amount })
		if i < 0 {
			t.Fatal("missing fish")
		}
		id := f.DrawPile[i]
		f.DrawPile = slices.Delete(f.DrawPile, i, i+1)
		f.Hands[p] = append(f.Hands[p], id)
		amount -= catanFishValue(id)
	}
}
func TestCatanFishingAttackCardResumesRollAndNeutral(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, phase := range []string{"catan_roll", "catan_turn"} {
			s := fishingAttackAfterSetup(t, n)
			if phase == "catan_turn" {
				for s.Phase != "catan_turn" {
					actor := twoFullActor(s)
					act, e := s.BotAction(actor)
					if e != nil {
						t.Fatal(e)
					}
					if e = s.Apply(actor, act); e != nil {
						t.Fatal(e)
					}
				}
			}
			p := s.Turn
			fishingAttackHand(t, s, p, 7)
			a := s.Catan.Attack
			i := slices.Index(a.Deck, "knighthood")
			a.Deck[i], a.Deck[len(a.Deck)-1] = a.Deck[len(a.Deck)-1], a.Deck[i]
			if e := s.Apply(p, Action{Type: "catan_fish_dev", Tokens: s.Catan.fishPayment(p, s.Catan.fishActionCost(p, "catan_fish_dev"))}); e != nil {
				t.Fatal(n, phase, e)
			}
			if s.Phase != "catan_attack_card" {
				t.Fatal("not immediate", s.Phase)
			}
			for s.Phase == "catan_attack_card" {
				fishingAttackRestore(t, s)
				act, e := s.BotAction(p)
				if e != nil {
					t.Fatal(e)
				}
				if e = s.Apply(p, act); e != nil {
					t.Fatal(e)
				}
			}
			if s.Phase != phase {
				t.Fatal("lost resume", n, phase, s.Phase)
			}
			if n == 2 && len(s.Catan.Attack.Knights) != 2 {
				t.Fatal("missing shared neutral")
			}
			if len(s.Catan.DevDeck) != 0 || sum(s.Catan.Players[p].Dev) != 0 {
				t.Fatal("base deck contamination")
			}
			fishingAttackRestore(t, s)
		}
	}
}
func TestCatanFishingAttackFishMoveUndoPrivacyAndCommit(t *testing.T) {
	s := fishingAttackAfterSetup(t, 3)
	s.Phase = "catan_turn"
	p := s.Turn
	g := s.Catan
	fishingAttackHand(t, s, p, 2)
	from := catanFishingSide(g, g.Attack.Map.Castles[0], 0)
	g.Attack.Knights = []catanAttackKnight{{p, from}}
	target := -1
	for edge, d := range g.Attack.knightDestinations(g, 0, 5) {
		if d == 5 {
			target = edge
			break
		}
	}
	if target < 0 {
		t.Fatal("no long destination")
	}
	if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	before, _ := json.Marshal(s)
	q := s.Catan.Attack.EndPlan
	action := Action{Type: "catan_attack_move", Choice: "fish", Prompt: q.ID, Edge: from, Target: target}
	attackReject(t, s, (p+1)%3, action)
	if e := s.Apply(p, action); e != nil {
		t.Fatal(e)
	}
	if len(s.Catan.Fishing.Tokens.Hands[p]) == 0 {
		t.Fatal("draft spent public fish")
	}
	for viewer := -1; viewer < 3; viewer++ {
		pub := s.View(viewer)["catan"].(map[string]any)["attack"].(map[string]any)
		if viewer != p && pub["previewFish"] != nil {
			t.Fatal("fish leaked")
		}
	}
	preview, e := s.catanAttackPlanPreview()
	if e != nil || len(preview.Catan.Fishing.Tokens.Hands[p]) != 0 {
		t.Fatal("unpaid preview", e)
	}
	fishingAttackRestore(t, s)
	if e = s.Apply(p, Action{Type: "catan_attack_move", Choice: "undo", Prompt: q.ID}); e != nil {
		t.Fatal(e)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("undo changed state")
	}
	if e = s.Apply(p, action); e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(p, Action{Type: "catan_attack_move", Choice: "confirm", Prompt: q.ID}); e != nil {
		t.Fatal(e)
	}
	if len(s.Catan.Fishing.Tokens.Hands[p]) != 0 || s.Catan.Attack.Knights[0].Edge != target {
		t.Fatal("commit lost payment/move")
	}
	fishingAttackRestore(t, s)
}

func TestCatanFishingAttackConquestAndMapIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s, e := newCatanFishingAttack(n)
		if e != nil {
			t.Fatal(e)
		}
		g := s.Catan
		if g.Fishing.Map.Lakes == nil {
			t.Fatal("no-lake map must serialize an empty array")
		}
		found := false
		for _, ground := range g.Fishing.Map.Grounds {
			for _, v := range ground.Vertices {
				coastOnly := true
				for _, tile := range g.Tiles {
					if slices.Contains(tile.Vertices, v) && !slices.Contains(g.Attack.Map.Coast, tile.ID) {
						coastOnly = false
					}
				}
				if !coastOnly {
					continue
				}
				old := g.Vertices[v]
				g.Vertices[v].Owner = 0
				g.Vertices[v].Level = 2
				before, e := g.Fishing.Map.production(g, ground.Number)
				if e != nil {
					t.Fatal(e)
				}
				for _, tile := range g.Tiles {
					if slices.Contains(tile.Vertices, v) {
						g.Attack.Barbarians[tile.ID] = 3
					}
				}
				after, e := g.Fishing.Map.production(g, ground.Number)
				if e != nil {
					t.Fatal(e)
				}
				if before[0]-after[0] != 2 {
					t.Fatal("conquered city still produces fish", before, after)
				}
				for _, tile := range g.Tiles {
					g.Attack.Barbarians[tile.ID] = 0
				}
				g.Vertices[v] = old
				found = true
				break
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatal("no coastal fixture")
		}
		for _, change := range []func(*Catan){
			func(g *Catan) { g.Fishing.Attack = "" },
			func(g *Catan) { g.Fishing.Rivers = CatanFishingRiversRules },
			func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{Tile: 0, Numbers: []int{2, 3, 11, 12}}} },
			func(g *Catan) { g.Fishing.Map.Grounds[0].Edges[0] = -1 },
			func(g *Catan) { g.Tiles[g.Attack.Map.Coast[0]].Number = 7 },
		} {
			bad := clone(*s)
			change(bad.Catan)
			if bad.Catan.validateFishing() == nil {
				t.Fatal("accepted corrupt recipe")
			}
		}
		base, e := newCatanAttackState(n, CatanOptions{FiveSix: n > 4})
		if e != nil || base.Catan.Fishing != nil || base.validateCatanAttack() != nil {
			t.Fatal("base attack changed", e)
		}
		if n == 2 && sum(base.Catan.Two.Tokens) != 10 {
			t.Fatal("base chips changed")
		}
	}
}

func TestCatanFishingAttackTwoCompensationAndNeutralPayment(t *testing.T) {
	s := fishingAttackAfterSetup(t, 2)
	for s.Phase != "catan_turn" {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	g, a, p := s.Catan, s.Catan.Attack, s.Turn
	tile, one, two := -1, -1, -1
	for _, id := range a.Map.Coast {
		for _, e := range g.Edges {
			for _, f := range g.Edges {
				if e.ID != f.ID && !a.castleEdge(g, e.ID) && !a.castleEdge(g, f.ID) && slices.Contains(e.Tiles, id) && slices.Contains(f.Tiles, id) && a.Map.edgeOrientation(g, e.ID) == a.Map.edgeOrientation(g, f.ID) {
					tile, one, two = id, e.ID, f.ID
					break
				}
			}
			if tile >= 0 {
				break
			}
		}
		if tile >= 0 {
			break
		}
	}
	if tile < 0 {
		t.Fatal("missing battle fixture")
	}
	loss := 0
	for d := 1; d <= 6; d++ {
		if catanAttackLossOrientation(d) == a.Map.edgeOrientation(g, one) {
			loss = d
			break
		}
	}
	a.Barbarians = make([]int, len(g.Tiles))
	a.Barbarians[tile] = 1
	a.Knights = []catanAttackKnight{{p, one}, {catanAttackNeutral, two}}
	fishingAttackHand(t, s, p, 2)
	// A neutral cannot spend the active player's fish, even with a valid path.
	for target := range a.knightDestinations(g, 1, 5) {
		if target != two {
			attackEndReject(t, s, []catanAttackMove{{From: two, To: target, Fish: true}}, attackDice(t))
			break
		}
	}
	beforeGold := a.Gold[p]
	if e := s.catanAttackResolveEnd(nil, attackDice(t, 3, 2, loss)); e != nil {
		t.Fatal(e)
	}
	a, g = s.Catan.Attack, s.Catan
	if a.NeutralPrisoners != 1 || a.Gold[p] != beforeGold+6 || sum(g.Two.Tokens) != 0 || g.Two.TokensIssued != 0 || g.Two.Bank != 0 || len(a.End.Battles[0].Tokens) != 0 {
		t.Fatal("fish compensation issued trade chips", a.End, g.Two)
	}
	if len(a.Knights) != 1 || a.Knights[0].Player != catanAttackNeutral {
		t.Fatal("neutral casualty")
	}
	fishingAttackRestore(t, s)
}

func TestCatanFishingAttackPaymentRejectsWithoutMutation(t *testing.T) {
	s := fishingAttackAfterSetup(t, 3)
	s.Phase = "catan_turn"
	g := s.Catan
	p := s.Turn
	fishingAttackHand(t, s, p, 0)
	from := catanFishingSide(g, g.Attack.Map.Castles[0], 0)
	g.Attack.Knights = []catanAttackKnight{{p, from}}
	target := -1
	for edge, d := range g.Attack.knightDestinations(g, 0, 5) {
		if d == 5 {
			target = edge
			break
		}
	}
	if target < 0 {
		t.Fatal("no destination")
	}
	attackEndReject(t, s, []catanAttackMove{{From: from, To: target, Fish: true}}, attackDice(t))
	fishingAttackHand(t, s, p, 2)
	attackEndReject(t, s, []catanAttackMove{{From: from, To: target, Fish: true, Wheat: true}}, attackDice(t))
	attackEndReject(t, s, []catanAttackMove{{From: from, To: -1, Fish: true}}, attackDice(t))
	attackReject(t, s, p, Action{Type: "catan_fish_knight"})
	if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	q := s.Catan.Attack.EndPlan
	attackReject(t, s, p, Action{Type: "catan_attack_move", Choice: "fish", Prompt: q.ID - 1, Edge: from, Target: target})
}
