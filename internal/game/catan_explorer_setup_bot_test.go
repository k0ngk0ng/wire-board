package game

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestCatanExplorerCityBotOpening(t *testing.T) {
	var slowest time.Duration
	for n := 3; n <= 6; n++ {
		for start := 0; start < n; start++ {
			for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
				t.Run(fmt.Sprintf("%d/%d/%s", n, start, scenario), func(t *testing.T) {
					// Explicit synthetic lair tokens, never a public release recipe.
					var numbers []int
					if catanExplorerMissionScenario(scenario) {
						numbers = []int{3, 4, 5, 9, 10, 11}
						if n > 4 {
							numbers = append(numbers, 3, 4)
						}
					}
					s, err := newCatanExplorerCityState(n, scenario, numbers)
					if err != nil {
						t.Fatal(err)
					}
					g, x := s.Catan, s.Catan.Explorer
					s.Turn, g.StartPlayer, x.Setup.Start = start, start, start
					if g.Paired != nil {
						g.Paired.Primary, g.Paired.Secondary = start, (start+3)%n
					}
					explorerCityStateRestore(t, s)
					steps := 0
					for s.Catan.Explorer.Setup != nil {
						g, x = s.Catan, s.Catan.Explorer
						before := clone(*s)
						began := time.Now()
						target, ok := x.Setup.botOpening(g, x.Board, x.Fleet, 4096)
						slowest = max(slowest, time.Since(began))
						if !ok {
							t.Fatal("bot opening needed fallback", x.Setup.Step)
						}
						a, err := s.BotAction(s.Turn)
						if err != nil || a.Target != target || a.Prompt != x.Setup.Step+1 || !reflect.DeepEqual(*s, before) {
							t.Fatal("bot integration, prompt, or read-only failure", a, err)
						}
						choices, _ := s.catanExplorerSpecialChoices(s.Turn)
						if !slices.ContainsFunc(choices, func(v Action) bool { return reflect.DeepEqual(v, a) }) {
							t.Fatal("bot chose outside human legal actions")
						}
						// Permuting every hidden deck must not affect setup selection.
						private := clone(*s)
						for track := range private.Catan.CitiesKnights.ProgressDecks {
							slices.Reverse(private.Catan.CitiesKnights.ProgressDecks[track])
						}
						if lairs := private.Catan.Explorer.Lairs; lairs != nil {
							slices.Reverse(lairs.Deck)
						}
						board := private.Catan.Explorer.Board
						for region := 0; region < 2; region++ {
							slices.Reverse(board.Numbers[region])
							ids := []int{}
							for id, h := range board.Hidden {
								if !h.Revealed && h.Region == region {
									ids = append(ids, id)
								}
							}
							for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
								a, b := ids[i], ids[j]
								board.Hidden[a].Resource, board.Hidden[b].Resource = board.Hidden[b].Resource, board.Hidden[a].Resource
								board.Hidden[a].Fish, board.Hidden[b].Fish = board.Hidden[b].Fish, board.Hidden[a].Fish
								board.Hidden[a].Farm, board.Hidden[b].Farm = board.Hidden[b].Farm, board.Hidden[a].Farm
								board.Hidden[a].PirateDie, board.Hidden[b].PirateDie = board.Hidden[b].PirateDie, board.Hidden[a].PirateDie
							}
						}
						if err := private.validateCatanExplorer(); err != nil {
							t.Fatal("private permutation invalid", err)
						}
						other, err := private.BotAction(private.Turn)
						if err != nil || !reflect.DeepEqual(other, a) || !reflect.DeepEqual(private.View(private.Turn), s.View(s.Turn)) {
							t.Fatal("hidden information influenced setup", err)
						}
						explorerCityStateAct(t, s, s.Turn, a)
						steps++
					}
					if steps != 4*n || s.Phase != "catan_roll" || s.Turn != start {
						t.Fatal("wrong completed opening", steps, s.Phase, s.Turn)
					}
				})
			}
		}
	}
	t.Log("slowest opening search", slowest)
}

func TestCatanExplorerCityBotOpeningBoundaries(t *testing.T) {
	for _, fixture := range []struct{ n, start, step int }{{4, 2, 7}, {6, 0, 11}} {
		q := explorerKnightsSetupFixture(t, fixture.n, fixture.start, "pirate-lairs")
		before := q.bytes()
		for _, budget := range []int{0, 1} {
			if _, ok := q.S.botOpening(q.G, q.B, q.F, budget); ok || before != q.bytes() {
				t.Fatal("exhausted search changed state or claimed a completed opening")
			}
		}
		var lastCompletable explorerSetupFixture
		rng := rand.New(rand.NewSource(int64(41*fixture.n + fixture.start)))
		for q.S.Step < fixture.step {
			if _, ok := q.S.botOpening(q.G, q.B, q.F, 4096); ok {
				lastCompletable = clone(q)
			}
			step := q.S.current(fixture.n)
			options := q.S.choices(q.G, q.B, q.F)
			if err := q.place(step.Player, q.S.Step+1, step.Kind, options[rng.Intn(len(options))]); err != nil {
				t.Fatal(err)
			}
		}
		before = q.bytes()
		if _, ok := q.S.botOpening(q.G, q.B, q.F, 4096); ok || before != q.bytes() {
			t.Fatal("stranded manual opening silently repaired or declared completable")
		}
		// A player switches to autoplay during the recorded manual path, before
		// the irreversible coast-blocking choice. Finish using real placement.
		q = lastCompletable
		if q.S == nil || q.S.Step == 0 {
			t.Fatal("missing partial human opening for takeover")
		}
		for step := q.S.current(fixture.n); step != nil; step = q.S.current(fixture.n) {
			target, ok := q.S.botOpening(q.G, q.B, q.F, 4096)
			if !ok {
				t.Fatal("takeover failed to preserve a completable opening")
			}
			if err := q.place(step.Player, q.S.Step+1, step.Kind, target); err != nil {
				t.Fatal(err)
			}
			q.restore(t)
		}
	}
}
