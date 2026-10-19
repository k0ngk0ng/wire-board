package maps

import "testing"

func TestCatalogValid(t *testing.T) {
	list := List()
	if len(list) != 12 {
		t.Fatalf("got %d maps, want 12", len(list))
	}
	seen := map[string]bool{}
	for _, m := range list {
		if m.ID == "" || seen[m.ID] {
			t.Fatalf("bad/duplicate id %q", m.ID)
		}
		seen[m.ID] = true
		if err := Validate(m.Cells); err != nil {
			t.Fatalf("%s: %v", m.ID, err)
		}
	}
}

func TestByID(t *testing.T) {
	m, ok := ByID("classic-01")
	if !ok || m.Name == "" {
		t.Fatal("classic-01 missing")
	}
	if _, ok := ByID("nope"); ok {
		t.Fatal("unknown map accepted")
	}
}

func TestSpawnPoints(t *testing.T) {
	cases := []struct {
		size int
		team int
		want []Point
	}{
		{1, 0, []Point{{6, 2}}},
		{2, 0, []Point{{2, 2}, {10, 2}}},
		{3, 0, []Point{{2, 2}, {6, 2}, {10, 2}}},
		{1, 1, []Point{{6, 10}}},
		{2, 1, []Point{{2, 10}, {10, 10}}},
		{3, 1, []Point{{2, 10}, {6, 10}, {10, 10}}},
	}
	for _, c := range cases {
		got := SpawnPoints(c.size, c.team)
		if len(got) != len(c.want) {
			t.Fatalf("size %d team %d: %v", c.size, c.team, got)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("size %d team %d: got %v want %v", c.size, c.team, got, c.want)
			}
		}
	}
}

func TestRingAndBases(t *testing.T) {
	for team := 0; team < 2; team++ {
		bx, by := BaseCell(team)
		if bx != 6 || (team == 0 && by != 0) || (team == 1 && by != 12) {
			t.Fatalf("base %d = %d,%d", team, bx, by)
		}
		ring := Ring(team)
		if len(ring) != 5 {
			t.Fatalf("ring %d length %d", team, len(ring))
		}
	}
}
