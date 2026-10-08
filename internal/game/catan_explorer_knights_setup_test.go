package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

func explorerKnightsSetupFixture(t *testing.T, n, start int, scenario string) explorerSetupFixture {
	t.Helper()
	g, b, f, c, e, s, err := newCatanExplorerMissionSetupVariant(n, scenario, "variable", start, true)
	if err != nil {
		t.Fatal(err)
	}
	return explorerSetupFixture{g, b, f, c, e, s}
}

func TestCatanExplorerKnightsSetupCompletedLegalOpenings(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			for start := 0; start < n; start++ {
				t.Run(fmt.Sprintf("%d/%s/start%d", n, scenario, start), func(t *testing.T) {
					q := explorerKnightsSetupFixture(t, n, start, scenario)
					rng := rand.New(rand.NewSource(int64(41*n + start)))
					opening := explorerKnightsCompletableOpening(t, q, rng)
					hidden, _ := json.Marshal(q.B.Hidden)
					for i := 0; i < 4*n; i++ {
						step := q.S.current(n)
						wantKind, wantOwner := "city", (start+i)%n
						if i >= n && i < 2*n {
							wantKind, wantOwner = "harbor", (start+2*n-1-i)%n
						}
						if i >= 2*n {
							wantKind = "road"
							if i%2 == 1 {
								wantKind = "ship"
							}
							wantOwner = (start + (i-2*n)/2) % n
						}
						if step == nil || step.Kind != wantKind || step.Player != wantOwner || step.Owner != wantOwner {
							t.Fatal("wrong combo placement order", i, step, wantKind, wantOwner)
						}
						choices := q.S.choices(q.G, q.B, q.F)
						if len(choices) == 0 {
							t.Fatal("placement stranded", i, step)
						}
						target := opening[i]
						if !slices.Contains(choices, target) {
							t.Fatal("planned legal placement missing", i, target)
						}
						before := q.bytes()
						if err := q.place((step.Player+1)%n, i+1, step.Kind, target); err == nil || q.bytes() != before {
							t.Fatal("foreign placement accepted or mutated")
						}
						if err := q.place(step.Player, i, step.Kind, target); err == nil || q.bytes() != before {
							t.Fatal("stale placement accepted or mutated")
						}
						if err := q.place(step.Player, i+1, step.Kind, target); err != nil {
							t.Fatal(i, err)
						}
						q.restore(t)
						if i+1 < 4*n {
							if q.E.Turn != nil {
								t.Fatal("early production")
							}
							for _, p := range q.G.Players {
								if sum(p.Resources) != 0 {
									t.Fatal("early resources")
								}
							}
						}
					}
					if q.S.current(n) != nil || q.E.Turn.Player != start || q.E.Turn.Sequence != 1 || q.E.Turn.Phase != "roll" {
						t.Fatal("wrong completed setup")
					}
					if n > 4 && (q.G.Paired.Primary != start || q.G.Paired.Secondary != (start+3)%n || q.G.Paired.Second) {
						t.Fatal("wrong paired markers")
					}
					for p, hand := range q.G.Players {
						v, h := q.S.Settlements[p], q.S.Harbors[p]
						if !q.G.cityAt(v) || !q.G.harborAt(h) || hand.Score != 4 || q.E.Gold[p] != 2 {
							t.Fatal("city/harbor or score", p)
						}
						roads, villages, cities := q.G.pieces(p)
						if roads != 1 || villages != 0 || cities != 1 {
							t.Fatal("wrong opening pieces", p, roads, villages, cities)
						}
						want := make([]int, 8)
						for _, tile := range q.G.Tiles {
							if tile.Resource < 5 && slices.Contains(tile.Vertices, v) {
								want[tile.Resource]++
							}
						}
						if !slices.Equal(hand.Resources, want) {
							t.Fatal("city starting resources must be single ordinary cards", p, hand.Resources, want)
						}
						if q.C.Units[p*11] != (catanExplorerCargoLocation{"ship", p * 3}) || q.F.Positions[p*3] < 0 {
							t.Fatal("missing loaded ship")
						}
					}
					ckSupply(t, q.G)
					after, _ := json.Marshal(q.B.Hidden)
					if string(hidden) != string(after) {
						t.Fatal("setup altered hidden terrain")
					}
					// This stage stops before dice/event integration. Do not let the five-card
					// standalone production controller process an eight-card combined state.
					before := q.bytes()
					if _, err := q.E.resolveProduction(q.G, q.F, q.C, start, 1, [2]int{3, 3}); err == nil || q.bytes() != before {
						t.Fatal("unfinished combination production accepted or changed state")
					}
					x := catanExplorer{Board: q.B, Fleet: q.F, Cargo: q.C, Economy: q.E, Setup: q.S}
					if err := x.validate(q.G); err == nil {
						t.Fatal("unfinished full combination accepted")
					}
				})
			}
		}
	}
}

func TestCatanExplorerKnightsMapInventoryAndRestore(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			g, b, err := newCatanExplorerBoardVariant(n, scenario, "variable", true)
			if err != nil {
				t.Fatal(err)
			}
			base, ordinary, err := newCatanExplorerBoard(n, scenario, "variable")
			if err != nil {
				t.Fatal(err)
			}
			count := func(g *Catan, b *catanExplorerBoard) [5]int {
				var out [5]int
				for _, id := range b.Starting {
					out[g.Tiles[id].Resource]++
				}
				return out
			}
			want := count(base, ordinary)
			want[0]--
			want[3]++
			if count(g, b) != want || b.Target != ordinary.Target+5 || !b.CitiesKnights {
				t.Fatal("wrong combo terrain/target", n, scenario)
			}
			view := b.publicView()
			if !view.CitiesKnights || view.Target != b.Target {
				t.Fatal("public board lost combined rules")
			}
			other := clone(*b)
			slices.Reverse(other.Hidden)
			slices.Reverse(other.Numbers[0])
			public, _ := json.Marshal(view)
			privateChanged, _ := json.Marshal(other.publicView())
			if string(public) != string(privateChanged) {
				t.Fatal("public combination view exposes hidden deck order")
			}
			if g.Tiles[b.FramePasture].Resource != 2 || g.Tiles[b.FramePasture].Number != 6 || len(g.Tiles) != len(base.Tiles) {
				t.Fatal("combo moved printed frame")
			}
			raw, _ := json.Marshal(struct {
				G *Catan
				B *catanExplorerBoard
			}{g, b})
			var restored struct {
				G *Catan
				B *catanExplorerBoard
			}
			if err = json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			if err = restored.B.validate(restored.G); err != nil {
				t.Fatal(err)
			}
			for _, mutate := range []func(*Catan, *catanExplorerBoard){
				func(_ *Catan, b *catanExplorerBoard) { b.CitiesKnights = false },
				func(_ *Catan, b *catanExplorerBoard) { b.Target -= 5 },
				func(g *Catan, b *catanExplorerBoard) {
					for _, id := range b.Starting {
						if g.Tiles[id].Resource == 3 {
							g.Tiles[id].Resource = 0
							break
						}
					}
				},
				func(g *Catan, b *catanExplorerBoard) { g.Tiles[b.FramePasture].Resource = 3 },
			} {
				candidateG, candidateB := clone(*g), clone(*b)
				mutate(&candidateG, &candidateB)
				if err = candidateB.validate(&candidateG); err == nil {
					t.Fatal("corrupt combination map accepted")
				}
			}
		}
	}
	for _, n := range []int{1, 7} {
		if _, _, err := newCatanExplorerBoardVariant(n, "pirate-lairs", "variable", true); err == nil {
			t.Fatal("unsupported player count accepted")
		}
	}
	if _, _, err := newCatanExplorerBoardVariant(3, "land-ho", "fixed", true); err == nil {
		t.Fatal("unverified printed combo accepted")
	}
}

func TestCatanExplorerKnightsSetupRejectsCorruptComponents(t *testing.T) {
	for _, mutate := range []func(explorerSetupFixture){
		func(q explorerSetupFixture) { q.S.CitiesKnights = false },
		func(q explorerSetupFixture) { q.B.CitiesKnights = false },
		func(q explorerSetupFixture) { q.G.CitiesKnights = nil },
		func(q explorerSetupFixture) { q.G.Bank[5]-- },
		func(q explorerSetupFixture) { q.G.Players[0].Resources[5]++ },
		func(q explorerSetupFixture) { q.G.CitiesKnights.Players = q.G.CitiesKnights.Players[:2] },
		func(q explorerSetupFixture) {
			q.G.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1}}
		},
		func(q explorerSetupFixture) {
			q.G.CitiesKnights.ProgressDecks[0] = q.G.CitiesKnights.ProgressDecks[0][1:]
		},
		func(q explorerSetupFixture) { q.G.CitiesKnights.Players[0].Improvements[0] = 1 },
	} {
		q := explorerKnightsSetupFixture(t, 3, 0, "pirate-lairs")
		mutate(q)
		before := q.bytes()
		if err := q.S.validate(q.G, q.B, q.F, q.C, q.E); err == nil {
			t.Fatal("corrupt setup accepted")
		}
		if err := q.place(0, 1, "city", 0); err == nil || before != q.bytes() {
			t.Fatal("corrupt setup mutated")
		}
	}
}

// Look ahead using geometry only to select one legally completable opening.
// This is a test path planner, not a restriction on human placement choices.
// The stranded manual path is preserved separately below as a release issue.
func explorerKnightsCompletableOpening(t *testing.T, q explorerSetupFixture, rng *rand.Rand) []int {
	t.Helper()
	var search func(explorerSetupFixture, int) ([]int, bool)
	nodes := 0
	search = func(q explorerSetupFixture, depth int) ([]int, bool) {
		nodes++
		if nodes > 100000 {
			t.Fatal("opening search exceeded budget")
		}
		step := q.S.current(len(q.G.Players))
		if step == nil {
			return []int{}, true
		}
		options := q.S.choices(q.G, q.B, q.F)
		rng.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
		for _, target := range options {
			next := clone(q)
			// Plan geometry without dealing resources or invoking the economy. Actual
			// tests replay every selected step through validated atomic place().
			switch step.Kind {
			case "city":
				next.G.Vertices[target].Owner = step.Owner
				next.G.Vertices[target].Level = 2
				next.S.Settlements[step.Owner] = target
			case "harbor":
				next.G.Vertices[target].Owner = step.Owner
				next.G.Vertices[target].Level = 2
				next.G.Vertices[target].Harbor = true
				next.S.Harbors[step.Owner] = target
			case "road":
				next.G.Edges[target].Owner = step.Owner
			case "ship":
				next.F.Positions[step.Owner*3] = target
			}
			next.S.Step++
			if suffix, ok := search(next, depth+1); ok {
				return append([]int{target}, suffix...), true
			}
		}
		return nil, false
	}
	path, ok := search(q, 0)
	if !ok {
		t.Fatal("no legal combined opening")
	}
	return path
}

func TestCatanExplorerKnightsSetupPreservesStrandedCoastEvidence(t *testing.T) {
	// The published city-first rule plus unrestricted starting-island city sites
	// can consume the coast needed for later harbors. Do not silently forbid
	// legal city sites, swap placement order, or place a harbor across a building.
	for _, fixture := range []struct{ n, start, step int }{{4, 2, 7}, {6, 0, 11}} {
		q := explorerKnightsSetupFixture(t, fixture.n, fixture.start, "pirate-lairs")
		rng := rand.New(rand.NewSource(int64(41*fixture.n + fixture.start)))
		for q.S.Step < fixture.step {
			step := q.S.current(fixture.n)
			options := q.S.choices(q.G, q.B, q.F)
			if len(options) == 0 {
				t.Fatal("unexpected earlier dead end")
			}
			if err := q.place(step.Player, q.S.Step+1, step.Kind, options[rng.Intn(len(options))]); err != nil {
				t.Fatal(err)
			}
		}
		step := q.S.current(fixture.n)
		if step.Kind != "harbor" || len(q.S.choices(q.G, q.B, q.F)) != 0 {
			t.Fatal("recorded coastal dead end changed")
		}
		q.restore(t)
		before := q.bytes()
		for _, target := range q.B.HarborStarts {
			if err := q.place(step.Player, q.S.Step+1, "harbor", target); err == nil || q.bytes() != before {
				t.Fatal("stranded setup allowed overlapping harbor")
			}
		}
	}
}
