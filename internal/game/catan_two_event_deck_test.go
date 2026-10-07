package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func twoReferenceGame(t *testing.T, setup bool) *State {
	t.Helper()
	s, err := newCatanTwoReferenceEvents()
	if err != nil {
		t.Fatal(err)
	}
	if setup {
		for s.Catan.setup() {
			a, err := s.BotAction(s.Turn)
			if err != nil {
				t.Fatal(err)
			}
			helperApply(t, s, s.Turn, a)
		}
	}
	return s
}

func twoReferenceTop(t *testing.T, s *State, ids ...int) {
	t.Helper()
	d := &s.Catan.EventDeck.Deck
	for offset, id := range ids {
		at := slices.Index(d.DrawPile, id)
		if at < 0 || id == catanEventNewYear {
			t.Fatal("invalid reference fixture card", id)
		}
		top := len(d.DrawPile) - 1 - offset
		d.DrawPile[at], d.DrawPile[top] = d.DrawPile[top], d.DrawPile[at]
	}
}

func resolveTwoReferenceProduction(t *testing.T, s *State) *State {
	t.Helper()
	for steps := 0; s.Phase != "catan_roll" && s.Phase != "catan_turn" && steps < 30; steps++ {
		s = referenceEventRestore(t, s)
		twoCoreRestore(t, s)
		actor := ckActor(s)
		helperReject(t, s, actor, Action{Type: "catan_roll"})
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	if s.Phase != "catan_roll" && s.Phase != "catan_turn" {
		t.Fatal("production stalled", s.Phase)
	}
	return s
}

func TestCatanTwoEventDeckSameProductionTwice(t *testing.T) {
	for _, ids := range [][]int{{22, 23}, {15, 16}} { // Two eights, or two sevens.
		t.Run(fmt.Sprint(ids), func(t *testing.T) {
			s := twoReferenceGame(t, true)
			if ids[0] == 15 {
				for p := range 2 {
					for c := range 5 {
						catanGive(s.Catan, p, c, 2-s.Catan.Players[p].Resources[c])
					}
				}
			}
			twoReferenceTop(t, s, ids...)
			total := catanEventReferenceFaces[ids[0]].Production
			owner := s.Turn
			expected := [][]int{slices.Clone(s.Catan.Players[0].Resources), slices.Clone(s.Catan.Players[1].Resources)}
			if total == 8 {
				for _, tile := range s.Catan.Tiles {
					if tile.Number == 8 && tile.ID != s.Catan.Robber && tile.Resource < 5 {
						for _, v := range tile.Vertices {
							vertex := s.Catan.Vertices[v]
							if vertex.Owner >= 0 && vertex.Level > 0 {
								expected[vertex.Owner][tile.Resource] += 2 * vertex.Level
							}
						}
					}
				}
			}
			for draw := 1; draw <= 2; draw++ {
				helperApply(t, s, owner, Action{Type: "catan_roll"})
				if total == 7 && draw == 1 && s.Phase != "catan_discard" {
					t.Fatal("first seven skipped simultaneous discards")
				}
				s = resolveTwoReferenceProduction(t, s)
				if s.Catan.RollID != draw || len(s.Catan.Two.Rolls) != draw || s.Catan.Two.Rolls[draw-1] != total || !slices.Equal(s.Catan.EventDeck.Deck.Discard, ids[:draw]) {
					t.Fatal("duplicate number caused a reroll or skipped production")
				}
				if draw == 1 {
					if s.Phase != "catan_roll" || s.Turn != owner {
						t.Fatal("first event did not return to second production")
					}
					helperReject(t, s, owner, Action{Type: "catan_end"})
				} else if s.Phase != "catan_turn" {
					t.Fatal("second event did not enter action phase")
				}
				twoCoreRestore(t, s)
				catanCheck(t, s)
			}
			if total == 8 {
				for p, want := range expected {
					if !slices.Equal(want, s.Catan.Players[p].Resources) {
						t.Fatal("same-number production skipped or duplicated", p, want, s.Catan.Players[p].Resources)
					}
				}
			}
			helperReject(t, s, owner, Action{Type: "catan_roll"})
			helperApply(t, s, owner, Action{Type: "catan_end"})
			if s.Turn == owner || len(s.Catan.Two.Rolls) != 0 || s.Catan.RollID != 2 {
				t.Fatal("next turn did not clear only current production history")
			}
		})
	}
}

func TestCatanTwoEventDeckAllFacesAndRestarts(t *testing.T) {
	for kind := range catanCardEventNames {
		t.Run(kind, func(t *testing.T) {
			s := twoReferenceGame(t, true)
			referenceEventTop(t, s, kind)
			helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
			if s.Catan.RevealedEvent.Kind != kind {
				t.Fatal("wrong face")
			}
			s = resolveTwoReferenceProduction(t, s)
			if s.Catan.RollID != 1 || s.Phase != "catan_roll" || !s.Catan.RevealedEvent.ProductionStarted {
				t.Fatal("first event did not complete exactly once")
			}
			for _, edge := range s.Catan.Edges {
				if edge.Owner < 0 && edge.Damaged {
					t.Fatal("neutral road was damaged")
				}
			}
			helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
			s = resolveTwoReferenceProduction(t, s)
			if s.Phase != "catan_turn" || s.Catan.RollID != 2 {
				t.Fatal("second production stalled")
			}
		})
	}
}

func TestCatanTwoEventDeckEarthquakeRepairIsNotNeutralConstruction(t *testing.T) {
	s := twoReferenceGame(t, true)
	twoReferenceTop(t, s, 11, 22) // Earthquake, then a plain production.
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	s = resolveTwoReferenceProduction(t, s)
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	s = resolveTwoReferenceProduction(t, s)
	owner := s.Turn
	edge := -1
	for _, e := range s.Catan.Edges {
		if e.Owner == owner && e.Damaged {
			edge = e.ID
		}
	}
	if edge < 0 {
		t.Fatal("earthquake did not damage active player's road")
	}
	before := slices.Clone(s.Catan.Edges)
	for _, c := range []int{0, 1} {
		if s.Catan.Players[owner].Resources[c] == 0 {
			catanGive(s.Catan, owner, c, 1)
		}
	}
	sequence := s.Catan.Two.Sequence
	helperApply(t, s, owner, Action{Type: "catan_repair_road", Edge: edge})
	if s.Phase != "catan_turn" || s.Catan.Two.Pending != nil || s.Catan.Two.Sequence != sequence {
		t.Fatal("repair triggered neutral construction")
	}
	before[edge].Damaged = false
	if !reflect.DeepEqual(before, s.Catan.Edges) {
		t.Fatal("repair changed unrelated routes")
	}
	// A genuine new road still requires exactly one neutral road.
	for _, c := range []int{0, 1} {
		if s.Catan.Players[owner].Resources[c] == 0 {
			catanGive(s.Catan, owner, c, 1)
		}
	}
	roads := s.View(owner)["catan"].(map[string]any)["legal"].(map[string][]int)["roads"]
	if len(roads) == 0 {
		t.Fatal("no construction fixture")
	}
	helperApply(t, s, owner, Action{Type: "catan_road", Edge: roads[0]})
	if s.Catan.Two.Pending == nil || s.Phase != "catan_two_build" {
		t.Fatal("new road lost neutral construction")
	}
	s = resolveTwoReferenceProduction(t, s)
	if s.Catan.RollID != 2 {
		t.Fatal("neutral construction changed production")
	}
}

func TestCatanTwoEventDeckNewYearSecondProduction(t *testing.T) {
	s := twoReferenceGame(t, true)
	for draw := 1; draw <= 34; draw++ {
		helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
		s = resolveTwoReferenceProduction(t, s)
		if s.Catan.EventDeck.Deck.Cycle != uint64((draw-1)/31+1) || s.Catan.RollID != draw {
			t.Fatal("wrong new year boundary", draw)
		}
		if draw%2 == 0 {
			if s.Phase != "catan_turn" || len(s.Catan.Two.Rolls) != 2 {
				t.Fatal("new year consumed or skipped a production")
			}
			helperApply(t, s, s.Turn, Action{Type: "catan_end"})
		} else if s.Phase != "catan_roll" {
			t.Fatal("first production did not continue")
		}
		twoCoreRestore(t, s)
		s = referenceEventRestore(t, s)
	}
}

func TestCatanTwoEventDeckNaturalMatch(t *testing.T) {
	s := twoReferenceGame(t, false)
	for step := 0; !s.Finished && step < 6000; step++ {
		actor := ckActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(step, err)
		}
		if err = s.Apply(actor, a); err != nil {
			t.Fatalf("step=%d phase=%s action=%+v: %v", step, s.Phase, a, err)
		}
		if err = s.validateCatanTwo(); err != nil {
			t.Fatal(err)
		}
		if err = s.validateCatanEventSession(); err != nil {
			t.Fatal(err)
		}
		catanCheck(t, s)
		if step%19 == 0 || s.Catan.CardEvent != nil {
			twoCoreRestore(t, s)
			s = referenceEventRestore(t, s)
		}
	}
	if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 10 {
		t.Fatal("no natural victory", s.Round)
	}
	t.Logf("%d production draws / %d cycles / %d rounds", s.Catan.RollID, s.Catan.EventDeck.Deck.Cycle, s.Round)
}

func TestCatanTwoEventDeckBetweenDrawsFreeRepairAndTrade(t *testing.T) {
	s := twoReferenceGame(t, true)
	twoReferenceTop(t, s, 11, 22)
	owner := s.Turn
	helperApply(t, s, owner, Action{Type: "catan_roll"})
	s = resolveTwoReferenceProduction(t, s)
	edge := -1
	for _, e := range s.Catan.Edges {
		if e.Owner == owner && e.Damaged {
			edge = e.ID
		}
	}
	if edge < 0 {
		t.Fatal("missing damaged road")
	}
	catanCard(s.Catan, owner, 1)
	helperApply(t, s, owner, Action{Type: "catan_dev", Card: 1})
	serial := s.Catan.Two.Sequence
	helperApply(t, s, owner, Action{Type: "catan_repair_road", Edge: edge})
	if s.Phase != "catan_roads" || s.Catan.FreeRoads != 1 || s.Catan.Two.Sequence != serial {
		t.Fatal("free repair consumed wrong construction allowance")
	}
	roads := s.View(owner)["catan"].(map[string]any)["legal"].(map[string][]int)["roads"]
	if len(roads) == 0 {
		t.Fatal("missing free road site")
	}
	helperApply(t, s, owner, Action{Type: "catan_road", Edge: roads[0]})
	if s.Phase != "catan_two_build" {
		t.Fatal("free new road did not require neutral road")
	}
	a, err := s.BotAction(owner)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, owner, a)
	if s.Phase != "catan_roll" || !s.catanTwoTokenWindow(owner) || s.Catan.RollID != 1 {
		t.Fatal("did not restore second production window")
	}
	for _, p := range []int{owner, 1 - owner} {
		catanGive(s.Catan, p, 0, 2-s.Catan.Players[p].Resources[0])
	}
	helperApply(t, s, owner, Action{Type: "catan_two_trade"})
	s = referenceEventRestore(t, s)
	helperReject(t, s, owner, Action{Type: "catan_roll"})
	a, err = s.BotAction(owner)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, owner, a)
	if s.Phase != "catan_roll" || s.Catan.RollID != 1 || !s.Catan.Two.Spent {
		t.Fatal("trade lost second production")
	}
	helperApply(t, s, owner, Action{Type: "catan_roll"})
	if s.Phase != "catan_turn" || !slices.Equal(s.Catan.Two.Rolls, []int{6, 8}) {
		t.Fatal("second actual draw lost")
	}
	catanCheck(t, s)
}

func TestCatanTwoEventDeckRejectsCorruptProductionAndDiceFallback(t *testing.T) {
	seed := twoReferenceGame(t, true)
	twoReferenceTop(t, seed, 22, 23)
	helperApply(t, seed, seed.Turn, Action{Type: "catan_roll"})
	for _, damage := range []func(*State){
		func(s *State) { s.Catan.Two.Rolls = nil },
		func(s *State) { s.Catan.Two.Rolls = []int{7} },
		func(s *State) { s.Catan.Two.Rolls = []int{8, 8} },
		func(s *State) { s.Catan.Options.FiveSix = true },
		func(s *State) { s.Catan.EventDeck.Catalogue = "unknown" },
	} {
		bad := clone(*seed)
		damage(&bad)
		helperReject(t, &bad, bad.Turn, Action{Type: "catan_roll"})
	}
	before := clone(*seed)
	if err := seed.catanTwoRoll(1, 1); err == nil || !reflect.DeepEqual(*seed, before) {
		t.Fatal("dice bypassed event deck")
	}
}
