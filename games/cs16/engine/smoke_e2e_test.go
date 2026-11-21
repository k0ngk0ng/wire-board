package engine

import "testing"

func smokeScenario() *State {
	return &State{Frame: 200, Grenades: []Grenade{{ID: 1, Kind: "smokegrenade", Active: true, ExplodesAt: 100, ExpiresAt: 1900}}}
}

func TestSmokeGrowthAndExpiryE2E(t *testing.T) {
	s := smokeScenario()
	g := s.Grenades[0]
	for _, item := range []struct {
		frame  int64
		radius int
	}{{-1, 0}, {99, 0}, {100, 0}, {101, 180 * Unit / 100}, {150, 90 * Unit}, {199, 180 * Unit * 99 / 100}, {200, 180 * Unit}, {1899, 180 * Unit}, {1900, 0}, {1901, 0}} {
		if got := SmokeRadius(g, item.frame); got != item.radius {
			t.Fatalf("frame %d radius %d want %d", item.frame, got, item.radius)
		}
	}
	g.Active = false
	if SmokeRadius(g, 200) != 0 {
		t.Fatal("inactive smoke grew")
	}
	g.Active = true
	g.Kind = "flashbang"
	if SmokeRadius(g, 200) != 0 {
		t.Fatal("flashbang became smoke")
	}
}

func TestSmokeInsideBehindAndBoundaryE2E(t *testing.T) {
	s := smokeScenario()
	centerZ := 70 * Unit
	for _, item := range []struct {
		name       string
		start, end Vec3
		blocked    bool
	}{
		{"through", Vec3{-300 * Unit, 0, centerZ}, Vec3{300 * Unit, 0, centerZ}, true},
		{"observer-inside", Vec3{0, 0, centerZ}, Vec3{300 * Unit, 0, centerZ}, true},
		{"target-inside", Vec3{-300 * Unit, 0, centerZ}, Vec3{0, 0, centerZ}, true},
		{"behind-observer", Vec3{300 * Unit, 0, centerZ}, Vec3{400 * Unit, 0, centerZ}, false},
		{"behind-target", Vec3{-400 * Unit, 0, centerZ}, Vec3{-300 * Unit, 0, centerZ}, false},
		{"tangent", Vec3{-300 * Unit, 180 * Unit, centerZ}, Vec3{300 * Unit, 180 * Unit, centerZ}, true},
		{"one-inside", Vec3{-300 * Unit, 180*Unit - 1, centerZ}, Vec3{300 * Unit, 180*Unit - 1, centerZ}, true},
		{"one-outside", Vec3{-300 * Unit, 180*Unit + 1, centerZ}, Vec3{300 * Unit, 180*Unit + 1, centerZ}, false},
		{"point-inside", Vec3{0, 0, centerZ}, Vec3{0, 0, centerZ}, true},
		{"point-outside", Vec3{300 * Unit, 0, centerZ}, Vec3{300 * Unit, 0, centerZ}, false},
		{"upper-tangent", Vec3{-300 * Unit, 0, 250 * Unit}, Vec3{300 * Unit, 0, 250 * Unit}, true},
		{"above", Vec3{-300 * Unit, 0, 250*Unit + 1}, Vec3{300 * Unit, 0, 250*Unit + 1}, false},
	} {
		if got := SmokeBlocks(s, item.start, item.end); got != item.blocked {
			t.Fatalf("%s blocked %t want %t", item.name, got, item.blocked)
		}
		if got := SmokeBlocks(s, item.end, item.start); got != item.blocked {
			t.Fatalf("%s reverse blocked %t", item.name, got)
		}
	}
	s.Frame = 1900
	if SmokeBlocks(s, Vec3{-300 * Unit, 0, centerZ}, Vec3{300 * Unit, 0, centerZ}) {
		t.Fatal("expired smoke blocked")
	}
}

func TestSmokeDiagonalAndBoundedArithmeticE2E(t *testing.T) {
	s := smokeScenario()
	for _, item := range []struct {
		start, end Vec3
		blocked    bool
	}{
		{Vec3{-300 * Unit, -300 * Unit, -230 * Unit}, Vec3{300 * Unit, 300 * Unit, 370 * Unit}, true},
		{Vec3{-32768 * Unit, -32768 * Unit, 70 * Unit}, Vec3{32768 * Unit, 32768 * Unit, 70 * Unit}, true},
		{Vec3{-32768 * Unit, -32768 * Unit, 251 * Unit}, Vec3{32768 * Unit, 32768 * Unit, 251 * Unit}, false},
		{Vec3{-(1 << 26), -(1 << 26), 70 * Unit}, Vec3{1 << 26, 1 << 26, 70 * Unit}, true},
	} {
		if got := SmokeBlocks(s, item.start, item.end); got != item.blocked {
			t.Fatalf("segment %+v %+v result %t", item.start, item.end, got)
		}
	}
	s.Grenades = append(s.Grenades, Grenade{ID: 2, Kind: "smokegrenade", Active: true, X: 1000 * Unit, ExplodesAt: 100, ExpiresAt: 1900})
	if !SmokeBlocks(s, Vec3{800 * Unit, 0, 70 * Unit}, Vec3{1200 * Unit, 0, 70 * Unit}) {
		t.Fatal("second smoke ignored")
	}
	if !SmokeBlocks(s, Vec3{-(1 << 27), 0, 70 * Unit}, Vec3{1 << 27, 0, 70 * Unit}) {
		t.Fatal("invalid ray bypassed bounded smoke")
	}
	s.Grenades = make([]Grenade, 33)
	if !SmokeBlocks(s, Vec3{}, Vec3{X: 1}) {
		t.Fatal("oversized grenade state was unbounded")
	}
}
