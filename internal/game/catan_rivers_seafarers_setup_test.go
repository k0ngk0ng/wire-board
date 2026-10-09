package game

import (
	"reflect"
	"testing"
)

func TestCatanRiversSeafarersSetupAdmission(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, scenario := range []string{"shores", "fog", "desert", "tribe", "new_world"} {
			setup, err := NormalizeCatanRiversSeafarersSetup(n, CatanRiversSeafarersSetup{Scenario: scenario})
			unsupported := n > 4 && (scenario == "desert" || scenario == "tribe")
			if unsupported {
				if err == nil {
					t.Fatal("admitted unfinished recipe", n, scenario)
				}
				continue
			}
			if err != nil {
				t.Fatal(n, scenario, err)
			}
			again, err := NormalizeCatanRiversSeafarersSetup(n, setup)
			if err != nil || again != setup {
				t.Fatal("unstable normalization")
			}
			s, err := NewCatanRiversSeafarers(n, setup, nil)
			if err != nil {
				t.Fatal(n, scenario, err)
			}
			if len(s.Catan.Players) != n || s.Catan.Seafarers.Scenario != scenario || !s.Catan.riversSea() {
				t.Fatal("wrong game")
			}
			if n == 2 && s.validateCatanTwo() != nil {
				t.Fatal("two-player configuration")
			}
			riversSeaRestore(t, s)
		}
	}
	for _, setup := range []CatanRiversSeafarersSetup{
		{Scenario: "shores", Rules: "unknown"}, {Scenario: "cloth"}, {Scenario: "shores", Layout: "prepared"}, {Scenario: "new_world", Layout: "fixed"},
	} {
		if _, err := NewCatanRiversSeafarers(3, setup, nil); err == nil {
			t.Fatal("invalid accepted", setup)
		}
	}
	for _, n := range []int{0, 1, 7} {
		if _, err := NormalizeCatanRiversSeafarersSetup(n, CatanRiversSeafarersSetup{Scenario: "shores"}); err == nil {
			t.Fatal("invalid seats")
		}
	}
}
func TestCatanRiversSeafarersPreparedSetup(t *testing.T) {
	m := preparedRiverWorldFixture(t, 4)
	before := clone(*m)
	for _, n := range []int{2, 4} {
		s, err := NewCatanRiversSeafarers(n, CatanRiversSeafarersSetup{Scenario: "new_world", Layout: "prepared"}, m)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(s.Catan.RiversWorldMap(), m) || !reflect.DeepEqual(*m, before) {
			t.Fatal("map mutated")
		}
		riversSeaRestore(t, s)
	}
	for _, scenario := range []string{"shores", "new_world"} {
		if _, err := NewCatanRiversSeafarers(4, CatanRiversSeafarersSetup{Scenario: scenario}, m); err == nil {
			t.Fatal("ignored supplied map")
		}
	}
	if _, err := NewCatanRiversSeafarers(4, CatanRiversSeafarersSetup{Scenario: "new_world", Layout: "prepared"}, nil); err == nil {
		t.Fatal("rerolled missing prepared map")
	}
}
