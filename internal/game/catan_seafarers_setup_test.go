package game

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCatanSeafarersSetupValidation(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, info := range CatanSeafarersScenarios(n) {
			setup, err := NormalizeCatanSeafarersSetup(n, CatanSeafarersSetup{Scenario: info.ID})
			if err != nil || setup.Rules != CatanSeafarersRules || setup.Layout != info.Layouts[0] {
				t.Fatal(n, info.ID, setup, err)
			}
			for _, layout := range []string{"fixed", "variable", "prepared", "unknown"} {
				allowed := layout == "fixed" || (n <= 4 && info.ID != "pirate_islands" && layout == "variable")
				if n > 4 && info.ID == "shores" {
					allowed = layout == "variable"
				}
				if info.ID == "new_world" {
					allowed = layout == "prepared"
				}
				_, err = NormalizeCatanSeafarersSetup(n, CatanSeafarersSetup{Scenario: info.ID, Layout: layout})
				if (err == nil) != allowed {
					t.Fatal(n, info.ID, layout, err)
				}
			}
		}
	}
	for _, setup := range []CatanSeafarersSetup{{}, {Scenario: "unknown"}, {Scenario: "fog", Rules: "2022"}} {
		if _, err := NormalizeCatanSeafarersSetup(3, setup); err == nil {
			t.Fatal("invalid configuration accepted", setup)
		}
	}
	for _, n := range []int{0, 2, 7} {
		if CatanSeafarersScenarios(n) != nil {
			t.Fatal("catalog offered unsupported seats")
		}
		if _, err := NormalizeCatanSeafarersSetup(n, CatanSeafarersSetup{Scenario: "shores"}); err == nil {
			t.Fatal("invalid seats accepted")
		}
	}
	if _, err := NewCatanSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "new_world"}, nil); err == nil {
		t.Fatal("New World silently generated an unapproved map")
	}
	world, err := GenerateCatanNewWorldMap(3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCatanSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "fog"}, world); err == nil {
		t.Fatal("stray approved map silently ignored")
	}
	if _, err := NewCatanSeafarers(5, CatanOptions{}, CatanSeafarersSetup{Scenario: "fog"}, nil); err == nil {
		t.Fatal("missing five/six extension accepted")
	}
	if _, err := NewCatanSeafarers(3, CatanOptions{AllHelpers: true}, CatanSeafarersSetup{Scenario: "fog"}, nil); err == nil {
		t.Fatal("invalid Helpers combination accepted")
	}
	list := CatanSeafarersScenarios(3)
	list[0].Layouts[0] = "corrupt"
	if CatanSeafarersScenarios(3)[0].Layouts[0] != "fixed" {
		t.Fatal("catalog caller can mutate rules")
	}
}

// Exercise every recipe through the shared entry point and finish all mandatory
// setup using legal bot actions, including ports, gold and helper exchanges.
func TestCatanSeafarersSetupAllRecipesReachFirstRoll(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, info := range CatanSeafarersScenarios(n) {
			for _, layout := range info.Layouts {
				for _, helpers := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d/%s/%s/helpers=%v", n, info.ID, layout, helpers), func(t *testing.T) {
						var world *CatanNewWorldMap
						var err error
						if info.ID == "new_world" {
							world, err = GenerateCatanNewWorldMap(n)
							if err != nil {
								t.Fatal(err)
							}
						}
						s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}, CatanSeafarersSetup{Scenario: info.ID, Layout: layout}, world)
						if err != nil {
							t.Fatal(err)
						}
						g := s.Catan
						actualScenario := info.ID
						if n > 4 && info.ID == "islands" {
							actualScenario = "six_islands"
						}
						if g.Seafarers.Scenario != actualScenario || g.Seafarers.Layout != layout || g.Seafarers.Rules != CatanSeafarersRules || g.Seafarers.VictoryPoints != info.VictoryPoints {
							t.Fatal("wrong scenario/rules/target", g.Seafarers)
						}
						phase := "catan_setup_settlement"
						switch info.ID {
						case "cloth":
							if layout == "variable" {
								phase = "catan_cloth_start"
							}
						case "wonders":
							phase = "catan_wonders_start"
						case "new_world":
							phase = "catan_world_ports"
							if !reflect.DeepEqual(g.NewWorldMap(), world) {
								t.Fatal("approved map changed")
							}
						case "pirate_islands":
							for _, p := range g.Players {
								if p.Score != 1 {
									t.Fatal("preset settlement absent from score")
								}
							}
						}
						if s.Phase != phase {
							t.Fatal("skipped required initial choice", s.Phase, phase)
						}
						for step := 0; step < 120 && s.Phase != "catan_roll"; step++ {
							actor := s.Turn
							if p := s.CatanPendingActor(); p >= 0 {
								actor = p
							}
							a, err := s.BotAction(actor)
							if err != nil {
								t.Fatal(step, s.Phase, err)
							}
							helperApply(t, s, actor, a)
							fleetSupply(t, s.Catan)
							if step%7 == 0 {
								restored := clone(*s)
								s = &restored
							}
						}
						g = s.Catan
						if s.Phase != "catan_roll" || g.SetupStep != g.SetupLimit() || s.Turn != g.StartPlayer {
							t.Fatal("setup did not reach original first player", s.Phase, g.SetupStep)
						}
						wantBuildings := 2
						if info.ID == "cloth" || info.ID == "pirate_islands" {
							wantBuildings = 3
						}
						for p, seat := range g.Players {
							count := 0
							for _, v := range g.Vertices {
								if v.Owner == p {
									count++
								}
							}
							if count != wantBuildings || (seat.Helper != nil) != helpers {
								t.Fatal("wrong initial pieces/helpers", p, count, seat.Helper)
							}
						}
						if info.ID == "new_world" && g.newWorld().Index != len(g.newWorld().Ports) {
							t.Fatal("incomplete ports")
						}
						view := s.View(-1)["catan"].(map[string]any)
						sea := view["seafarers"].(map[string]any)
						if sea["rules"] != CatanSeafarersRules || sea["layout"] != layout {
							t.Fatal("public rules missing")
						}
						for _, raw := range view["players"].([]any) {
							if _, ok := raw.(map[string]any)["resources"]; ok {
								t.Fatal("spectator resource leak")
							}
						}
					})
				}
			}
		}
	}
}
