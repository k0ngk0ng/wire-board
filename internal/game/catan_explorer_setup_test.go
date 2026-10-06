package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

type explorerSetupFixture struct {
	G *Catan
	B *catanExplorerBoard
	F *catanExplorerSailing
	C *catanExplorerCargo
	E *catanExplorerEconomy
	S *catanExplorerSetup
}

func newExplorerSetupFixture(t *testing.T, n int, layout string, start int) explorerSetupFixture {
	t.Helper()
	g, b, f, c, e, s, err := newCatanExplorerLairsSetup(n, layout, start)
	if err != nil {
		t.Fatal(err)
	}
	return explorerSetupFixture{g, b, f, c, e, s}
}
func (q explorerSetupFixture) bytes() string { data, _ := json.Marshal(q); return string(data) }
func (q explorerSetupFixture) place(player, prompt int, kind string, target int) error {
	return q.S.place(q.G, q.B, q.F, q.C, q.E, player, prompt, kind, target)
}
func (q explorerSetupFixture) restore(t *testing.T) {
	t.Helper()
	before := q.bytes()
	for _, v := range []any{q.G, q.B, q.F, q.C, q.E, q.S} {
		data, _ := json.Marshal(v)
		if err := json.Unmarshal(data, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.S.validate(q.G, q.B, q.F, q.C, q.E); err != nil {
		t.Fatal(err)
	}
	if before != q.bytes() {
		t.Fatal("setup restore changed state")
	}
}
func TestCatanExplorerLairsOfficialSetupAllSeatsAndLayouts(t *testing.T) {
	for n := 2; n <= 4; n++ {
		for start := 0; start < n; start++ {
			for _, layout := range []string{"fixed", "variable"} {
				for seed := int64(0); seed < 5; seed++ {
					t.Run(fmt.Sprintf("%d/start%d/%s/%d", n, start, layout, seed), func(t *testing.T) {
						q := newExplorerSetupFixture(t, n, layout, start)
						rng := rand.New(rand.NewSource(seed))
						hidden, _ := json.Marshal(q.B.Hidden)
						for step := q.S.current(n); step != nil; step = q.S.current(n) {
							options := q.S.choices(q.G, q.B, q.F)
							if len(options) == 0 {
								t.Fatal("valid setup choices stranded later placement", q.S.Step, step)
							}
							oldTurn := q.E.Turn
							if err := q.place(step.Player, q.S.Step+1, step.Kind, options[rng.Intn(len(options))]); err != nil {
								t.Fatal(q.S.Step, err)
							}
							if oldTurn != nil {
								t.Fatal("production started before all placement")
							}
							if q.S.current(n) != nil {
								for _, p := range q.G.Players {
									if sum(p.Resources) != 0 {
										t.Fatal("early starting resources")
									}
								}
							}
							if q.S.Step%3 == 0 {
								q.restore(t)
							}
						}
						q.restore(t)
						after, _ := json.Marshal(q.B.Hidden)
						if string(hidden) != string(after) {
							t.Fatal("setup revealed hidden terrain")
						}
						if q.E.Turn.Player != start || q.E.Turn.Phase != "roll" || q.G.TurnSerial != 1 || q.G.SetupStep != 2*n {
							t.Fatal("wrong first production turn")
						}
						for p, player := range q.G.Players {
							want := make([]int, 5)
							for _, tile := range q.G.Tiles {
								if tile.Resource < 5 && slices.Contains(tile.Vertices, q.S.Settlements[p]) {
									want[tile.Resource]++
								}
							}
							if !slices.Equal(want, player.Resources) || player.Score != 3 || q.E.Gold[p] != 2 || q.F.Positions[p*3] < 0 || q.C.Units[p*11] != (catanExplorerCargoLocation{"ship", p * 3}) {
								t.Fatal("wrong opening inventory", p)
							}
						}
						neutralBuildings, neutralRoads := 0, 0
						for _, v := range q.G.Vertices {
							if v.Owner < -1 {
								neutralBuildings++
							}
						}
						for _, e := range q.G.Edges {
							if e.Owner < -1 {
								neutralRoads++
							}
						}
						wantNeutral := 0
						if n == 2 {
							wantNeutral = 4
						}
						if neutralBuildings != wantNeutral || neutralRoads != 0 {
							t.Fatal("neutral pieces must be buildings only")
						}
						// Consume the initialized economic turn directly, proving handoff into the
						// existing ordinary production/construction/movement controllers.
						if _, err := q.E.resolveProduction(q.G, q.F, q.C, start, 1, [2]int{1, 2}); err != nil {
							t.Fatal(err)
						}
						if err := q.C.beginMovement(q.G, q.F, start, 1, 0); err != nil {
							t.Fatal(err)
						}
						if err := q.C.endMovement(q.G, q.F, start, 1); err != nil {
							t.Fatal(err)
						}
						if err := q.E.beginProduction(q.G, q.F, q.C, (start+1)%n, 2); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
func TestCatanExplorerSetupOrderAndNeutralOwnership(t *testing.T) {
	got := catanExplorerSetupPlan(2, 1)
	want := []catanExplorerSetupStep{{1, 1, "harbor"}, {0, 0, "harbor"}, {1, -2, "harbor"}, {0, -3, "harbor"}, {0, 0, "settlement"}, {1, 1, "settlement"}, {0, -3, "settlement"}, {1, -2, "settlement"}, {1, 1, "road"}, {1, 1, "ship"}, {0, 0, "road"}, {0, 0, "ship"}}
	if !slices.Equal(got, want) {
		t.Fatal("wrong two-player setup order", got)
	}
	got = catanExplorerSetupPlan(3, 2)
	for i, player := range []int{2, 0, 1, 1, 0, 2, 2, 2, 0, 0, 1, 1} {
		if got[i].Player != player {
			t.Fatal("non-host order", got)
		}
	}
}
func TestCatanExplorerSetupRejectsIllegalAndStalePlacementsAtomically(t *testing.T) {
	q := newExplorerSetupFixture(t, 4, "fixed", 2)
	for step := q.S.current(4); step != nil; step = q.S.current(4) {
		options := q.S.choices(q.G, q.B, q.F)
		if len(options) == 0 {
			t.Fatal("no choices")
		}
		before := q.bytes()
		requests := []struct {
			p, prompt int
			kind      string
			target    int
		}{{(step.Player + 1) % 4, q.S.Step + 1, step.Kind, options[0]}, {step.Player, q.S.Step, step.Kind, options[0]}, {step.Player, q.S.Step + 1, "not-setup", options[0]}, {step.Player, q.S.Step + 1, step.Kind, -1}}
		for _, r := range requests {
			if err := q.place(r.p, r.prompt, r.kind, r.target); err == nil {
				t.Fatal("accepted illegal setup", r)
			}
			if q.bytes() != before {
				t.Fatal("failed placement changed state")
			}
		}
		target := options[0]
		if err := q.place(step.Player, q.S.Step+1, step.Kind, target); err != nil {
			t.Fatal(err)
		}
	}
	if len(q.S.choices(q.G, q.B, q.F)) != 0 {
		t.Fatal("completed setup still offers choices")
	}
	before := q.bytes()
	if err := q.place(2, 1, "harbor", q.S.Harbors[2]); err == nil || q.bytes() != before {
		t.Fatal("replayed completed setup")
	}
}
func TestCatanExplorerSetupRejectsCorruptRecordedBuildings(t *testing.T) {
	q := newExplorerSetupFixture(t, 2, "fixed", 0)
	if err := q.place(0, 1, "harbor", q.S.choices(q.G, q.B, q.F)[0]); err != nil {
		t.Fatal(err)
	}
	for _, damage := range []func(*catanExplorerSetup){func(s *catanExplorerSetup) { s.Step++ }, func(s *catanExplorerSetup) { s.Harbors[0] = -1 }, func(s *catanExplorerSetup) { s.Harbors[1] = s.Harbors[0] }, func(s *catanExplorerSetup) { s.Start = 1 }} {
		s := clone(*q.S)
		damage(&s)
		if err := s.validate(q.G, q.B, q.F, q.C, q.E); err == nil {
			t.Fatal("accepted corrupt setup")
		}
	}
	q.G.Players[0].Resources[0]++
	q.G.Bank[0]--
	if err := q.S.validate(q.G, q.B, q.F, q.C, q.E); err == nil {
		t.Fatal("accepted early resources")
	}
}
