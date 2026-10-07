package game

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func attackLastPieceFixture(t *testing.T, n, number int) (*State, []int) {
	t.Helper()
	s := newAttackState(t, n, false)
	g, a := s.Catan, s.Catan.Attack
	targets := []int{}
	for _, tile := range a.Map.Coast {
		if g.Tiles[tile].Number == number {
			targets = append(targets, tile)
		}
	}
	if len(targets) != 2 {
		t.Fatal("expected double coastal number")
	}
	// Controlled late-game stock: conserved prisoners leave exactly one piece.
	for i, total := 0, a.supply()-1; i < total; i++ {
		a.Prisoners[i%n]++
	}
	s.catanScores()
	assertAttackRestored(t, s)
	return s, targets
}

func TestCatanAttackLastPieceRandomLanding(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, number := range []int{5, 9} {
			for pick := 0; pick < 2; pick++ {
				t.Run(fmt.Sprintf("%d/%d/%d", n, number, pick), func(t *testing.T) {
					s, targets := attackLastPieceFixture(t, n, number)
					a := s.Catan.Attack
					before := slices.Clone(a.Barbarians)
					prisoners := slices.Clone(a.Prisoners)
					dice, rollID := slices.Clone(s.Catan.Dice), s.Catan.RollID
					calls := 0
					err := s.catanAttackLanding(scriptedAttackDice(t, [2]int{number / 2, number - number/2}), func(limit int) int {
						if limit != 2 {
							t.Fatal("random selection must have exactly two legal targets")
						}
						calls++
						return pick
					})
					if err != nil {
						t.Fatal(err)
					}
					before[targets[pick]]++
					if calls != 1 || a.supply() != 0 || !slices.Equal(before, a.Barbarians) || !slices.Equal(prisoners, a.Prisoners) {
						t.Fatal("last-piece allocation changed stock, prisoners, or wrong hex")
					}
					if a.Sequence != 1 || len(a.Landing.Rolls) != 1 || !a.Landing.Rolls[0].Shortage || !slices.Equal(a.Landing.Rolls[0].Tiles, []int{targets[pick]}) {
						t.Fatal("missing durable shortage outcome")
					}
					if !slices.Equal(dice, s.Catan.Dice) || rollID != s.Catan.RollID {
						t.Fatal("landing changed production dice")
					}
					if !slices.ContainsFunc(s.Log, func(line string) bool { return strings.Contains(line, "本站补充规则") }) {
						t.Fatal("supplemental rule missing from log")
					}
					for viewer := -1; viewer < n; viewer++ {
						pub := s.View(viewer)["catan"].(map[string]any)["attack"].(map[string]any)
						if pub["landingSupplyRule"] != "random-last" || pub["deck"] != nil {
							t.Fatal("public rule or deck privacy")
						}
					}
					assertAttackRestored(t, s)
					// Empty supply must not reroll, choose, or alter the previous placement.
					if err := s.catanAttackLanding(scriptedAttackDice(t), nil); err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(before, a.Barbarians) || len(a.Landing.Rolls) != 0 {
						t.Fatal("empty stock placed another piece")
					}
					assertAttackRestored(t, s)
				})
			}
		}
	}
}

func TestCatanAttackLastPieceConquestAndAtomicErrors(t *testing.T) {
	s, targets := attackLastPieceFixture(t, 6, 5)
	a := s.Catan.Attack
	// Move two captured pieces back onto each target, preserving one in stock.
	for _, tile := range targets {
		a.Barbarians[tile] = 2
		a.Prisoners[0] -= 2
	}
	s.catanScores()
	assertAttackRestored(t, s)
	for _, pick := range []int{-1, 2} {
		before := clone(*s)
		if err := s.catanAttackLanding(scriptedAttackDice(t, [2]int{2, 3}), func(int) int { return pick }); err == nil || !reflect.DeepEqual(before, *s) {
			t.Fatal("bad random result mutated state")
		}
	}
	if err := s.catanAttackLanding(scriptedAttackDice(t, [2]int{2, 3}), func(int) int { return 1 }); err != nil {
		t.Fatal(err)
	}
	if a.conquered(targets[0]) || !a.conquered(targets[1]) {
		t.Fatal("last piece conquered wrong target")
	}
	assertAttackRestored(t, s)
	for _, damage := range []func(*catanAttackLandingRoll){
		func(r *catanAttackLandingRoll) { r.Tiles = nil },
		func(r *catanAttackLandingRoll) { r.Tiles = slices.Clone(targets) },
		func(r *catanAttackLandingRoll) { r.Dice = [2]int{1, 1} },
	} {
		next := clone(*s)
		damage(&next.Catan.Attack.Landing.Rolls[0])
		if next.validateCatanAttack() == nil {
			t.Fatal("corrupt shortage record accepted")
		}
	}
}

func TestCatanAttackLastPieceSkipsConqueredTargets(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, number := range []int{5, 9} {
			s, targets := attackLastPieceFixture(t, n, number)
			a := s.Catan.Attack
			a.Barbarians[targets[0]] = 3
			a.Prisoners[0] -= 3
			s.catanScores()
			assertAttackRestored(t, s)
			// Only one eligible target: no random tie-breaking is needed.
			if err := s.catanAttackLanding(scriptedAttackDice(t, [2]int{number / 2, number - number/2}), nil); err != nil {
				t.Fatal(err)
			}
			if a.Barbarians[targets[0]] != 3 || a.Barbarians[targets[1]] != 1 || a.Landing.Rolls[0].Shortage || a.supply() != 0 {
				t.Fatal("conquered target took last piece")
			}
			assertAttackRestored(t, s)
		}
	}
}

func TestCatanAttackLastPieceAfterEarlierLandingIsAtomic(t *testing.T) {
	s, targets := attackLastPieceFixture(t, 6, 9)
	a := s.Catan.Attack
	a.Prisoners[0]-- // Two in supply: a single target consumes the first one.
	s.catanScores()
	assertAttackRestored(t, s)
	before := clone(*s)
	dice := func() func() [2]int { return scriptedAttackDice(t, [2]int{1, 1}, [2]int{4, 5}) }
	if err := s.catanAttackLanding(dice(), func(int) int { return 2 }); err == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("invalid choice partially committed an earlier landing")
	}
	if err := s.catanAttackLanding(dice(), func(int) int { return 0 }); err != nil {
		t.Fatal(err)
	}
	if len(a.Landing.Rolls) != 2 || a.Landing.Rolls[0].Shortage || !a.Landing.Rolls[1].Shortage || a.Barbarians[targets[0]] != 1 || a.supply() != 0 {
		t.Fatal("earlier placement did not consume stock before random allocation")
	}
	assertAttackRestored(t, s)
}
