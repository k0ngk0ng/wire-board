package game

import (
	"math/bits"
	"math/rand"
	"reflect"
	"testing"
)

// Independent exhaustive oracle: enumerate subsets of physical ground and port
// pieces, checking shared slots and adjacent ports directly on the cycle.
func bruteFishingWorldCapacity(c fishingWorldCoast, ports, grounds int) []bool {
	n, width := len(c.port), grounds+1
	result := make([]bool, (ports+1)*width)
	for fish := 0; fish < 1<<n; fish++ {
		f, occupied, valid := bits.OnesCount(uint(fish)), 0, true
		if f > grounds {
			continue
		}
		for i := range n {
			if fish&(1<<i) != 0 {
				mask := 1<<i | 1<<((i+1)%n)
				if !c.ground[i] || mask&occupied != 0 {
					valid = false
					break
				}
				occupied |= mask
			}
		}
		if !valid {
			continue
		}
		for port := 0; port < 1<<n; port++ {
			p := bits.OnesCount(uint(port))
			if p > ports || port&occupied != 0 {
				continue
			}
			valid = true
			for i := range n {
				if port&(1<<i) != 0 && (!c.port[i] || port&(1<<((i+1)%n)) != 0) {
					valid = false
					break
				}
			}
			if valid {
				result[p*width+f] = true
			}
		}
	}
	return result
}

func TestCatanFishingWorldCapacityMatchesExhaustivePieces(t *testing.T) {
	rng := rand.New(rand.NewSource(20261006))
	for n := 3; n <= 10; n++ {
		for sample := 0; sample < 48; sample++ {
			coast := fishingWorldCoast{port: make([]bool, n), ground: make([]bool, n)}
			for i := range n {
				coast.port[i] = sample == 0 || rng.Intn(3) != 0
				coast.ground[i] = sample == 0 || rng.Intn(3) != 0
			}
			want := bruteFishingWorldCapacity(coast, 6, 6)
			if got := coast.capacities(6, 6); !reflect.DeepEqual(got, want) {
				t.Fatalf("cycle %d sample %d: got %v want %v coast=%+v", n, sample, got, want, coast)
			}
		}
	}
}

func TestCatanFishingWorldCapacityIncludesSharedSlotsAndCycleSeam(t *testing.T) {
	// Each inventory fits alone, but they compete for the very same edges.
	c := fishingWorldCoast{port: []bool{true, true, true, true}, ground: []bool{true, true, true, true}}
	capacity := c.capacities(2, 2)
	if !capacity[2*3] || !capacity[2] || capacity[2*3+2] || !capacity[1*3+1] {
		t.Fatal("separate counts incorrectly substituted for joint packing")
	}
	// The only possible ground crosses the array cut; the only port sits in
	// the middle, adjacent to that ground, which is allowed.
	c = fishingWorldCoast{port: []bool{false, false, true, false, false}, ground: []bool{false, false, false, false, true}}
	if !c.capacities(1, 1)[3] {
		t.Fatal("cut-crossing ground or touching ground/port rejected")
	}
	c = fishingWorldCoast{port: []bool{true, false, false, true}, ground: make([]bool, 4)}
	if c.capacities(2, 0)[2] {
		t.Fatal("ports across the cycle cut touch")
	}
}

func TestCatanFishingWorldCapacityActualMapsAndConstructiveCompletion(t *testing.T) {
	for _, n := range []int{3, 4} {
		accepted, rejected := 0, 0
		for sample := 0; sample < 24; sample++ {
			s := newWorldGame(t, n, false)
			g := s.Catan
			before := clone(*g)
			coasts, valid := g.fishingWorldCoastlines()
			if !valid || len(coasts) == 0 || !g.fishingWorldCanFinish(10, 0) {
				t.Fatal("ordinary New World topology/port capacity rejected")
			}
			fits := g.fishingWorldCanFinish(10, 6)
			if !reflect.DeepEqual(before, *g) {
				t.Fatal("capacity check mutated approved map")
			}
			if !fits {
				rejected++
				continue // The constructor will report this, never reroll the map.
			}
			accepted++
			// Construct all pieces through capacity-preserving choices. This is
			// a geometric witness, not a claim that the action flow is wired yet.
			for i := 0; i < 10; i++ {
				found := false
				for _, edge := range g.worldPortEdges() {
					g.Ports = append(g.Ports, CatanPort{Edge: edge, Resource: -1})
					if g.fishingWorldCanFinish(9-i, 6) {
						found = true
						break
					}
					g.Ports = g.Ports[:len(g.Ports)-1]
				}
				if !found {
					t.Fatal("joint feasibility lost before all ports placed", i)
				}
			}
			g.Fishing = &CatanFishing{}
			for i, number := range []int{4, 5, 6, 8, 9, 10} {
				found := false
				for _, coast := range g.fishingCoasts(g.findIslands()) {
					g.Fishing.Map.Grounds = append(g.Fishing.Map.Grounds, fishingSeaGround(number, coast))
					if g.fishingWorldCanFinish(0, 5-i) {
						found = true
						break
					}
					g.Fishing.Map.Grounds = g.Fishing.Map.Grounds[:len(g.Fishing.Map.Grounds)-1]
				}
				if !found {
					t.Fatal("joint feasibility lost before all grounds placed", i)
				}
			}
			if err := g.Fishing.Map.validateCoastalGrounds(g.fishingCoasts(g.findIslands()), len(g.Players)); err != nil {
				t.Fatal("witness did not produce six real nonoverlapping grounds", err)
			}
			if !reflect.DeepEqual(before.Tiles, g.Tiles) || !reflect.DeepEqual(before.Edges, g.Edges) || !reflect.DeepEqual(before.Vertices, g.Vertices) {
				t.Fatal("constructive completion modified map geometry")
			}
		}
		t.Logf("players=%d joint-layout-witnesses=%d insufficient-maps=%d", n, accepted, rejected)
	}
}

func TestCatanFishingWorldCapacityRejectsOverlapsAndIgnoresHiddenOrder(t *testing.T) {
	s := newWorldGame(t, 3, false)
	g := s.Catan
	before := g.fishingWorldCanFinish(10, 6)
	for i := range g.newWorld().Ports {
		g.newWorld().Ports[i] = 99 // only public geometry participates
	}
	if g.fishingWorldCanFinish(10, 6) != before {
		t.Fatal("hidden port identities influence capacity")
	}
	for _, count := range [][2]int{{-1, 0}, {12, 0}, {0, -1}, {0, 9}} {
		if g.fishingWorldCanFinish(count[0], count[1]) {
			t.Fatal("invalid piece inventory")
		}
	}
	coasts := g.fishingCoasts(g.findIslands())
	if len(coasts) == 0 {
		t.Fatal("no concave coast in fixture")
	}
	c := coasts[0]
	g.Ports = []CatanPort{{Edge: c.Edges[0]}, {Edge: c.Edges[1]}}
	if g.fishingWorldCanFinish(0, 0) {
		t.Fatal("touching existing ports accepted")
	}
	g.Ports = g.Ports[:1]
	g.Fishing = &CatanFishing{Map: catanFishingMap{Grounds: []catanFishingGround{fishingSeaGround(4, c)}}}
	if g.fishingWorldCanFinish(0, 0) {
		t.Fatal("port and ground overlap accepted")
	}
	g.Ports = nil
	g.Fishing.Map.Grounds = append(g.Fishing.Map.Grounds, fishingSeaGround(5, c))
	if g.fishingWorldCanFinish(0, 0) {
		t.Fatal("overlapping existing grounds accepted")
	}
}
